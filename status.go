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
	config *BuildConfig

	startedEdges, finishedEdges, totalEdges, runningEdges int
	timeMillis                                            int32

	// Prints progress output.
	printer LinePrinter

	// The custom progress status format to use.
	progressStatusFormat string
	currentRate          slidingRateInfo
}

type slidingRateInfo struct {
	rate       float64
	N          int
	times      []float64
	lastUpdate int
}

func (s *slidingRateInfo) updateRate(updateHint int, timeMillis int32) {
	if updateHint == s.lastUpdate {
		return
	}
	s.lastUpdate = updateHint

	if len(s.times) == s.N {
		s.times = s.times[:len(s.times)-1]
	}
	s.times = append(s.times, float64(timeMillis))
	back := s.times[0]
	front := s.times[len(s.times)-1]
	if back != front {
		s.rate = float64(len(s.times)) / ((back - front) / 1e3)
	}
}

func NewStatusPrinter(config *BuildConfig) *StatusPrinter {
	s := &StatusPrinter{
		config:  config,
		printer: NewLinePrinter(),
		currentRate: slidingRateInfo{
			rate:       -1,
			N:          config.parallelism,
			lastUpdate: -1,
		},
	}
	// Don't do anything fancy in verbose mode.
	if s.config.verbosity != Normal {
		s.printer.setSmartTerminal(false)
	}

	s.progressStatusFormat = os.Getenv("NINJA_STATUS")
	if s.progressStatusFormat == "" {
		s.progressStatusFormat = "[%f/%t] "
	}
	return s
}

func (s *StatusPrinter) PlanHasTotalEdges(total int) {
	s.totalEdges = total
}

func (s *StatusPrinter) BuildEdgeStarted(edge *Edge, startTimeMillis int32) {
	s.startedEdges++
	s.runningEdges++
	s.timeMillis = startTimeMillis
	if edge.Pool == ConsolePool || s.printer.isSmartTerminal() {
		s.PrintStatus(edge, startTimeMillis)
	}

	if edge.Pool == ConsolePool {
		s.printer.SetConsoleLocked(true)
	}
}

func (s *StatusPrinter) BuildEdgeFinished(edge *Edge, endTimeMillis int32, success bool, output string) {
	s.timeMillis = endTimeMillis
	s.finishedEdges++

	if edge.Pool == ConsolePool {
		s.printer.SetConsoleLocked(false)
	}

	if s.config.verbosity == Quiet {
		return
	}

	if edge.Pool != ConsolePool {
		s.PrintStatus(edge, endTimeMillis)
	}

	s.runningEdges--

	// Print the command that is spewing before printing its output.
	if !success {
		outputs := ""
		for _, o := range edge.Outputs {
			outputs += o.Path + " "
		}
		if s.printer.supportsColor {
			s.printer.PrintOnNewLine("\x1B[31mFAILED: \x1B[0m" + outputs + "\n")
		} else {
			s.printer.PrintOnNewLine("FAILED: " + outputs + "\n")
		}
		s.printer.PrintOnNewLine(edge.EvaluateCommand(false) + "\n")
	}

	if len(output) != 0 {
		// ninja sets stdout and stderr of subprocesses to a pipe, to be able to
		// check if the output is empty. Some compilers, e.g. clang, check
		// isatty(stderr) to decide if they should print colored output.
		// To make it possible to use colored output with ninja, subprocesses should
		// be run with a flag that forces them to always print color escape codes.
		// To make sure these escape codes don't show up in a file if ninja's output
		// is piped to a file, ninja strips ansi escape codes again if it's not
		// writing to a |smartTerminal|.
		// (Launching subprocesses in pseudo ttys doesn't work because there are
		// only a few hundred available on some systems, and ninja can launch
		// thousands of parallel compile commands.)
		finalOutput := ""
		if !s.printer.supportsColor {
			finalOutput = StripAnsiEscapeCodes(output)
		} else {
			finalOutput = output
		}

		// TODO(maruel): Use an existing Go package.
		// Fix extra CR being added on Windows, writing out CR CR LF (#773)
		//Setmode(Fileno(stdout), _O_BINARY) // Begin Windows extra CR fix

		s.printer.PrintOnNewLine(finalOutput)

		//Setmode(Fileno(stdout), _O_TEXT) // End Windows extra CR fix

	}
}

func (s *StatusPrinter) BuildLoadDyndeps() {
	// The DependencyScan calls Explain() to print lines explaining why
	// it considers a portion of the graph to be out of date.  Normally
	// this is done before the build starts, but our caller is about to
	// load a dyndep file during the build.  Doing so may generate more
	// explanation lines (via fprintf directly to stderr), but in an
	// interactive console the cursor is currently at the end of a status
	// line.  Start a new line so that the first explanation does not
	// append to the status line.  After the explanations are done a
	// new build status line will appear.
	if gExplaining {
		s.printer.PrintOnNewLine("")
	}
}

func (s *StatusPrinter) BuildStarted() {
	s.startedEdges = 0
	s.finishedEdges = 0
	s.runningEdges = 0
}

func (s *StatusPrinter) BuildFinished() {
	s.printer.SetConsoleLocked(false)
	s.printer.PrintOnNewLine("")
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
				out += strconv.Itoa(s.startedEdges)

				// Total edges.
			case 't':
				out += strconv.Itoa(s.totalEdges)

				// Running edges.
			case 'r':
				out += strconv.Itoa(s.runningEdges)

				// Unstarted edges.
			case 'u':
				out += strconv.Itoa(s.totalEdges - s.startedEdges)

				// Finished edges.
			case 'f':
				out += strconv.Itoa(s.finishedEdges)

				// Overall finished edges per second.
			case 'o':
				rate := float64(s.finishedEdges) / float64(s.timeMillis) * 1000.
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}

				// Current rate, average over the last '-j' jobs.
			case 'c':
				s.currentRate.updateRate(s.finishedEdges, s.timeMillis)
				rate := s.currentRate.rate
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}

				// Percentage
			case 'p':
				percent := (100 * s.finishedEdges) / s.totalEdges
				out += fmt.Sprintf("%3d%%", percent)

			case 'e':
				out += fmt.Sprintf("%.3f", float64(s.timeMillis)*0.001)

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
	if s.config.verbosity == Quiet || s.config.verbosity == NoStatusUpdate {
		return
	}

	forceFullCommand := s.config.verbosity == Verbose

	toPrint := edge.GetBinding("description")
	if toPrint == "" || forceFullCommand {
		toPrint = edge.GetBinding("command")
	}

	toPrint = s.FormatProgressStatus(s.progressStatusFormat, timeMillis) + toPrint
	s.printer.Print(toPrint, !forceFullCommand)
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
