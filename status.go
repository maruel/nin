// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nin

import (
	"fmt"
	"os"
	"strconv"
)

// TODO(maruel): Create a Status (or LinePrinter?) for test cases that
// redirect to testing.T.Log().

// Abstract interface to object that tracks the status of a build:
// completion fraction, printing updates.
type Status interface {
	PlanHasTotalEdges(total int)
	BuildEdgeStarted(edge *Edge, startTimeMillis int32)
	BuildEdgeFinished(edge *Edge, endTimeMillis int32, success bool, output string)
	BuildLoadDyndeps()
	BuildStarted()
	BuildFinished()

	Info(msg string, i ...interface{})
	Warning(msg string, i ...interface{})
	Error(msg string, i ...interface{})
}

// Implementation of the Status interface that prints the status as
// human-readable strings to stdout
type StatusPrinter struct {
	config_ *BuildConfig

	startedEdges_, finishedEdges_, totalEdges_, runningEdges_ int
	timeMillis_                                               int32

	// Prints progress output.
	printer_ LinePrinter

	// The custom progress status format to use.
	progressStatusFormat_ string
	currentRate_          slidingRateInfo
}

type slidingRateInfo struct {
	rate_       float64
	N           int
	times_      []float64
	lastUpdate_ int
}

func (s *slidingRateInfo) updateRate(updateHint int, timeMillis int32) {
	if updateHint == s.lastUpdate_ {
		return
	}
	s.lastUpdate_ = updateHint

	if len(s.times_) == s.N {
		s.times_ = s.times_[:len(s.times_)-1]
	}
	s.times_ = append(s.times_, float64(timeMillis))
	back := s.times_[0]
	front := s.times_[len(s.times_)-1]
	if back != front {
		s.rate_ = float64(len(s.times_)) / ((back - front) / 1e3)
	}
}

func NewStatusPrinter(config *BuildConfig) *StatusPrinter {
	s := &StatusPrinter{
		config_:  config,
		printer_: NewLinePrinter(),
		currentRate_: slidingRateInfo{
			rate_:       -1,
			N:           config.parallelism,
			lastUpdate_: -1,
		},
	}
	// Don't do anything fancy in verbose mode.
	if s.config_.verbosity != NORMAL {
		s.printer_.setSmartTerminal(false)
	}

	s.progressStatusFormat_ = os.Getenv("NINJA_STATUS")
	if s.progressStatusFormat_ == "" {
		s.progressStatusFormat_ = "[%f/%t] "
	}
	return s
}

func (s *StatusPrinter) PlanHasTotalEdges(total int) {
	s.totalEdges_ = total
}

func (s *StatusPrinter) BuildEdgeStarted(edge *Edge, startTimeMillis int32) {
	s.startedEdges_++
	s.runningEdges_++
	s.timeMillis_ = startTimeMillis
	if edge.Pool == ConsolePool || s.printer_.isSmartTerminal() {
		s.PrintStatus(edge, startTimeMillis)
	}

	if edge.Pool == ConsolePool {
		s.printer_.SetConsoleLocked(true)
	}
}

func (s *StatusPrinter) BuildEdgeFinished(edge *Edge, endTimeMillis int32, success bool, output string) {
	s.timeMillis_ = endTimeMillis
	s.finishedEdges_++

	if edge.Pool == ConsolePool {
		s.printer_.SetConsoleLocked(false)
	}

	if s.config_.verbosity == QUIET {
		return
	}

	if edge.Pool != ConsolePool {
		s.PrintStatus(edge, endTimeMillis)
	}

	s.runningEdges_--

	// Print the command that is spewing before printing its output.
	if !success {
		outputs := ""
		for _, o := range edge.Outputs {
			outputs += o.Path + " "
		}
		if s.printer_.supportsColor() {
			s.printer_.PrintOnNewLine("\x1B[31mFAILED: \x1B[0m" + outputs + "\n")
		} else {
			s.printer_.PrintOnNewLine("FAILED: " + outputs + "\n")
		}
		s.printer_.PrintOnNewLine(edge.EvaluateCommand(false) + "\n")
	}

	if len(output) != 0 {
		// ninja sets stdout and stderr of subprocesses to a pipe, to be able to
		// check if the output is empty. Some compilers, e.g. clang, check
		// isatty(stderr) to decide if they should print colored output.
		// To make it possible to use colored output with ninja, subprocesses should
		// be run with a flag that forces them to always print color escape codes.
		// To make sure these escape codes don't show up in a file if ninja's output
		// is piped to a file, ninja strips ansi escape codes again if it's not
		// writing to a |smartTerminal_|.
		// (Launching subprocesses in pseudo ttys doesn't work because there are
		// only a few hundred available on some systems, and ninja can launch
		// thousands of parallel compile commands.)
		finalOutput := ""
		if !s.printer_.supportsColor() {
			finalOutput = StripAnsiEscapeCodes(output)
		} else {
			finalOutput = output
		}

		// TODO(maruel): Use an existing Go package.
		// Fix extra CR being added on Windows, writing out CR CR LF (#773)
		//Setmode(Fileno(stdout), _O_BINARY) // Begin Windows extra CR fix

		s.printer_.PrintOnNewLine(finalOutput)

		//Setmode(Fileno(stdout), _O_TEXT) // End Windows extra CR fix

	}
}

func (s *StatusPrinter) BuildLoadDyndeps() {
	// The DependencyScan calls EXPLAIN() to print lines explaining why
	// it considers a portion of the graph to be out of date.  Normally
	// this is done before the build starts, but our caller is about to
	// load a dyndep file during the build.  Doing so may generate more
	// explanation lines (via fprintf directly to stderr), but in an
	// interactive console the cursor is currently at the end of a status
	// line.  Start a new line so that the first explanation does not
	// append to the status line.  After the explanations are done a
	// new build status line will appear.
	if gExplaining {
		s.printer_.PrintOnNewLine("")
	}
}

func (s *StatusPrinter) BuildStarted() {
	s.startedEdges_ = 0
	s.finishedEdges_ = 0
	s.runningEdges_ = 0
}

func (s *StatusPrinter) BuildFinished() {
	s.printer_.SetConsoleLocked(false)
	s.printer_.PrintOnNewLine("")
}

// Format the progress status string by replacing the placeholders.
// See the user manual for more information about the available
// placeholders.
// @param progressStatusFormat The format of the progress status.
// @param status The status of the edge.
func (s *StatusPrinter) FormatProgressStatus(progressStatusFormat string, timeMillis int32) string {
	out := ""
	// TODO(maruel): Benchmark to optimize memory usage and performance
	// especially when GC is disabled.
	for i := 0; i < len(progressStatusFormat); i++ {
		c := progressStatusFormat[i]
		if c == '%' {
			i++
			c = progressStatusFormat[i]
			switch c {
			case '%':
				out += "%"

				// Started edges.
			case 's':
				out += strconv.Itoa(s.startedEdges_)

				// Total edges.
			case 't':
				out += strconv.Itoa(s.totalEdges_)

				// Running edges.
			case 'r':
				out += strconv.Itoa(s.runningEdges_)

				// Unstarted edges.
			case 'u':
				out += strconv.Itoa(s.totalEdges_ - s.startedEdges_)

				// Finished edges.
			case 'f':
				out += strconv.Itoa(s.finishedEdges_)

				// Overall finished edges per second.
			case 'o':
				rate := float64(s.finishedEdges_) / float64(s.timeMillis_) * 1000.
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}

				// Current rate, average over the last '-j' jobs.
			case 'c':
				s.currentRate_.updateRate(s.finishedEdges_, s.timeMillis_)
				rate := s.currentRate_.rate_
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}

				// Percentage
			case 'p':
				percent := (100 * s.finishedEdges_) / s.totalEdges_
				out += fmt.Sprintf("%3d%%", percent)

			case 'e':
				out += fmt.Sprintf("%.3f", float64(s.timeMillis_)*0.001)

			default:
				fatalf("unknown placeholder '%%%c' in $NINJA_STATUS", c)
				return ""
			}
		} else {
			out += string(c)
		}
	}
	return out
}

func (s *StatusPrinter) PrintStatus(edge *Edge, timeMillis int32) {
	if s.config_.verbosity == QUIET || s.config_.verbosity == NO_STATUS_UPDATE {
		return
	}

	forceFullCommand := s.config_.verbosity == VERBOSE

	toPrint := edge.GetBinding("description")
	if toPrint == "" || forceFullCommand {
		toPrint = edge.GetBinding("command")
	}

	toPrint = s.FormatProgressStatus(s.progressStatusFormat_, timeMillis) + toPrint

	l := FULL
	if forceFullCommand {
		l = ELIDE
	}
	s.printer_.Print(toPrint, l)
}

func (s *StatusPrinter) Warning(msg string, i ...interface{}) {
	warningf(msg, i...)
}

func (s *StatusPrinter) Error(msg string, i ...interface{}) {
	errorf(msg, i...)
}

func (s *StatusPrinter) Info(msg string, i ...interface{}) {
	infof(msg, i...)
}
