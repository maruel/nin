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

package ginja

import (
	"fmt"
	"os"
	"strconv"
)

// Abstract interface to object that tracks the status of a build:
// completion fraction, printing updates.
type Status interface {
	PlanHasTotalEdges(total int)
	BuildEdgeStarted(edge *Edge, start_time_millis int64)
	BuildEdgeFinished(edge *Edge, end_time_millis int64, success bool, output string)
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

	started_edges_, finished_edges_, total_edges_, running_edges_ int
	time_millis_                                                  int64

	// Prints progress output.
	printer_ LinePrinter

	// The custom progress status format to use.
	progress_status_format_ string
	current_rate_           slidingRateInfo
}

type slidingRateInfo struct {
	rate_        float64
	N            int
	times_       []float64
	last_update_ int
}

func (s *slidingRateInfo) updateRate(update_hint int, time_millis int64) {
	if update_hint == s.last_update_ {
		return
	}
	s.last_update_ = update_hint

	if len(s.times_) == s.N {
		s.times_ = s.times_[:len(s.times_)-1]
	}
	s.times_ = append(s.times_, float64(time_millis))
	back := s.times_[0]
	front := s.times_[len(s.times_)-1]
	if back != front {
		s.rate_ = float64(len(s.times_)) / ((back - front) / 1e3)
	}
}

func NewStatusPrinter(config *BuildConfig) StatusPrinter {
	s := StatusPrinter{
		config_: config,
		current_rate_: slidingRateInfo{
			rate_:        -1,
			N:            config.parallelism,
			last_update_: -1,
		},
	}
	// Don't do anything fancy in verbose mode.
	if s.config_.verbosity != NORMAL {
		s.printer_.set_smart_terminal(false)
	}

	s.progress_status_format_ = os.Getenv("NINJA_STATUS")
	if s.progress_status_format_ == "" {
		s.progress_status_format_ = "[%f/%t] "
	}
	return s
}

func (s *StatusPrinter) PlanHasTotalEdges(total int) {
	s.total_edges_ = total
}

func (s *StatusPrinter) BuildEdgeStarted(edge *Edge, start_time_millis int64) {
	s.started_edges_++
	s.running_edges_++
	s.time_millis_ = start_time_millis
	if edge.use_console() || s.printer_.is_smart_terminal() {
		s.PrintStatus(edge, start_time_millis)
	}

	if edge.use_console() {
		s.printer_.SetConsoleLocked(true)
	}
}

func (s *StatusPrinter) BuildEdgeFinished(edge *Edge, end_time_millis int64, success bool, output string) {
	panic("TODO")
	/*
		s.time_millis_ = end_time_millis
		s.finished_edges_++

		if edge.use_console() {
			s.printer_.SetConsoleLocked(false)
		}

		if s.config_.verbosity == QUIET {
			return
		}

		if !edge.use_console() {
			s.PrintStatus(edge, end_time_millis)
		}

		s.running_edges_--

		// Print the command that is spewing before printing its output.
		if !success {
			outputs := ""
			for _, o := range edge {
				outputs += o.path() + " "
			}
			if s.printer_.supports_color() {
				s.printer_.PrintOnNewLine("\x1B[31mFAILED: \x1B[0m" + outputs + "\n")
			} else {
				s.printer_.PrintOnNewLine("FAILED: " + outputs + "\n")
			}
			s.printer_.PrintOnNewLine(edge.EvaluateCommand() + "\n")
		}

		if len(output) != 0 {
			// ninja sets stdout and stderr of subprocesses to a pipe, to be able to
			// check if the output is empty. Some compilers, e.g. clang, check
			// isatty(stderr) to decide if they should print colored output.
			// To make it possible to use colored output with ninja, subprocesses should
			// be run with a flag that forces them to always print color escape codes.
			// To make sure these escape codes don't show up in a file if ninja's output
			// is piped to a file, ninja strips ansi escape codes again if it's not
			// writing to a |smart_terminal_|.
			// (Launching subprocesses in pseudo ttys doesn't work because there are
			// only a few hundred available on some systems, and ninja can launch
			// thousands of parallel compile commands.)
			final_output := ""
			if !s.printer_.supports_color() {
				final_output = StripAnsiEscapeCodes(output)
			} else {
				final_output = output
			}

			// TODO(maruel): Use an existing Go package.
				// Fix extra CR being added on Windows, writing out CR CR LF (#773)
				//_setmode(_fileno(stdout), _O_BINARY) // Begin Windows extra CR fix

				s.printer_.PrintOnNewLine(final_output)

				//_setmode(_fileno(stdout), _O_TEXT) // End Windows extra CR fix

		}
	*/
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
	if g_explaining {
		s.printer_.PrintOnNewLine("")
	}
}

func (s *StatusPrinter) BuildStarted() {
	s.started_edges_ = 0
	s.finished_edges_ = 0
	s.running_edges_ = 0
}

func (s *StatusPrinter) BuildFinished() {
	s.printer_.SetConsoleLocked(false)
	s.printer_.PrintOnNewLine("")
}

// Format the progress status string by replacing the placeholders.
// See the user manual for more information about the available
// placeholders.
// @param progress_status_format The format of the progress status.
// @param status The status of the edge.
func (s *StatusPrinter) FormatProgressStatus(progress_status_format string, time_millis int64) string {
	out := ""
	// TODO(maruel): Benchmark to optimize memory usage and performance
	// especially when GC is disabled.
	for i := 0; i < len(progress_status_format); i++ {
		c := progress_status_format[i]
		if c == '%' {
			i++
			c := progress_status_format[i]
			switch c {
			case '%':
				out += "%"
				break

				// Started edges.
			case 's':
				out += strconv.Itoa(s.started_edges_)
				break

				// Total edges.
			case 't':
				out += strconv.Itoa(s.total_edges_)
				break

				// Running edges.
			case 'r':
				{
					out += strconv.Itoa(s.running_edges_)
					break
				}

				// Unstarted edges.
			case 'u':
				out += strconv.Itoa(s.total_edges_ - s.started_edges_)
				break

				// Finished edges.
			case 'f':
				out += strconv.Itoa(s.finished_edges_)
				break

				// Overall finished edges per second.
			case 'o':
				rate := float64(s.finished_edges_) / float64(s.time_millis_) * 1000.
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}
				break

				// Current rate, average over the last '-j' jobs.
			case 'c':
				s.current_rate_.updateRate(s.finished_edges_, s.time_millis_)
				rate := s.current_rate_.rate_
				if rate == -1 {
					out += "?"
				} else {
					out += fmt.Sprintf("%.1f", rate)
				}
				break

				// Percentage
			case 'p':
				percent := (100 * s.finished_edges_) / s.total_edges_
				out += fmt.Sprintf("%3d%%", percent)
				break

			case 'e':
				out += fmt.Sprintf("%.3f", float64(s.time_millis_)*0.001)
				break

			default:
				Fatal("unknown placeholder '%%%c' in $NINJA_STATUS", c)
				return ""
			}
		} else {
			out += string(c)
		}
	}
	return out
}

func (s *StatusPrinter) PrintStatus(edge *Edge, time_millis int64) {
	if s.config_.verbosity == QUIET || s.config_.verbosity == NO_STATUS_UPDATE {
		return
	}

	force_full_command := s.config_.verbosity == VERBOSE

	to_print := edge.GetBinding("description")
	if to_print == "" || force_full_command {
		to_print = edge.GetBinding("command")
	}

	to_print = s.FormatProgressStatus(s.progress_status_format_, time_millis) + to_print

	l := FULL
	if force_full_command {
		l = ELIDE
	}
	s.printer_.Print(to_print, l)
}

func (s *StatusPrinter) Warning(msg string, i ...interface{}) {
	Warning(msg, i...)
}

func (s *StatusPrinter) Error(msg string, i ...interface{}) {
	Error(msg, i...)
}

func (s *StatusPrinter) Info(msg string, i ...interface{}) {
	Info(msg, i...)
}
