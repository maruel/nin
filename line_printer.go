// Copyright 2013 Google Inc. All Rights Reserved.
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
	"os"
	"runtime"
)

// Prints lines of text, possibly overprinting previously printed lines
// if the terminal supports it.
type LinePrinter struct {
	// Whether we can do fancy terminal control codes.
	smart_terminal_ bool

	// Whether we can use ISO 6429 (ANSI) color sequences.
	supports_color_ bool

	// Whether the caret is at the beginning of a blank line.
	have_blank_line_ bool

	// Whether console is locked.
	console_locked_ bool

	// Buffered current line while console is locked.
	line_buffer_ string

	// Buffered line type while console is locked.
	line_type_ LineType

	// Buffered console output while console is locked.
	output_buffer_ string

	//console_ *void
}

func (l *LinePrinter) is_smart_terminal() bool {
	return l.smart_terminal_
}
func (l *LinePrinter) set_smart_terminal(smart bool) {
	l.smart_terminal_ = smart
}
func (l *LinePrinter) supports_color() bool {
	return l.supports_color_
}

type LineType int

const (
	FULL LineType = iota
	ELIDE
)

func NewLinePrinter() LinePrinter {
	l := LinePrinter{
		have_blank_line_: true,
	}
	/*
		if os.Getenv("TERM") != "dumb" {
			if runtime.GOOS != "windows" {
				// Don't panic for now.
				//l.smart_terminal_ = isatty(1)
			} else {
				// Don't panic for now.
				//console_ = GetStdHandle(STD_OUTPUT_HANDLE)
				//var csbi CONSOLE_SCREEN_BUFFER_INFO
				//smart_terminal_ = GetConsoleScreenBufferInfo(console_, &csbi)
			}
		}
	*/
	l.supports_color_ = l.smart_terminal_
	if !l.supports_color_ {
		l.supports_color_ = os.Getenv("CLICOLOR_FORCE") != "0"
	}
	// Try enabling ANSI escape sequence support on Windows 10 terminals.
	if runtime.GOOS == "windows" {
		if l.supports_color_ {
			panic("TODO")
			/*
				var mode DWORD
				if GetConsoleMode(console_, &mode) {
					if !SetConsoleMode(console_, mode|ENABLE_VIRTUAL_TERMINAL_PROCESSING) {
						supports_color_ = false
					}
				}
			*/
		}
	}
	return l
}

// Overprints the current line. If type is ELIDE, elides to_print to fit on
// one line.
func (l *LinePrinter) Print(to_print string, t LineType) {
	if l.console_locked_ {
		l.line_buffer_ = to_print
		l.line_type_ = t
		return
	}

	if l.smart_terminal_ {
		printf("\r") // Print over previous line, if any.
		// On Windows, calling a C library function writing to stdout also handles
		// pausing the executable when the "Pause" key or Ctrl-S is pressed.
	}

	if l.smart_terminal_ && t == ELIDE {
		l.have_blank_line_ = false
		if runtime.GOOS == "windows" {
			panic("TODO")
			/*
				var csbi CONSOLE_SCREEN_BUFFER_INFO
				GetConsoleScreenBufferInfo(l.console_, &csbi)
				to_print = ElideMiddle(to_print, csbi.dwSize.X)
				if l.supports_color_ {
					// this means ENABLE_VIRTUAL_TERMINAL_PROCESSING
					// succeeded
					printf("%s\x1B[K", to_print) // Clear to end of line.
					fflush(stdout)
				} else {
					// We don't want to have the cursor spamming back and forth, so instead of
					// printf use WriteConsoleOutput which updates the contents of the buffer,
					// but doesn't move the cursor position.
					buf_size := COORD{csbi.dwSize.X, 1}
					zero_zero := COORD{0, 0}
					target := SMALL_RECT{csbi.dwCursorPosition.X, csbi.dwCursorPosition.Y,
						csbi.dwCursorPosition.X + csbi.dwSize.X - 1,
						csbi.dwCursorPosition.Y}
					char_data := make([]CHAR_INFO, csbi.dwSize.X)
					for i := 0; i < csbi.dwSize.X; i++ {
						if i < len(to_print) {
							char_data[i].Char.AsciiChar = to_print[i]
						} else {
							char_data[i].Char.AsciiChar = ' '
						}
						char_data[i].Attributes = csbi.wAttributes
					}
					WriteConsoleOutput(l.console_, &char_data[0], buf_size, zero_zero, &target)
				}
			*/
		} else {
			panic("TODO")
			/*
				// Limit output to width of the terminal if provided so we don't cause
				// line-wrapping.
				var size winsize
				if ioctl(STDOUT_FILENO, TIOCGWINSZ, &size) == 0 && size.ws_col {
					to_print = ElideMiddle(to_print, size.ws_col)
				}
				printf("%s", to_print)
				printf("\x1B[K") // Clear to end of line.
				fflush(stdout)
			*/
		}
	} else {
		printf("%s\n", to_print)
	}
}

// Print the given data to the console, or buffer it if it is locked.
func (l *LinePrinter) PrintOrBuffer(data string) {
	if l.console_locked_ {
		l.output_buffer_ += data
	} else {
		// Avoid printf and C strings, since the actual output might contain null
		// bytes like UTF-16 does (yuck).
		os.Stdout.WriteString(data)
	}
}

// Prints a string on a new line, not overprinting previous output.
func (l *LinePrinter) PrintOnNewLine(to_print string) {
	if l.console_locked_ && len(l.line_buffer_) != 0 {
		l.output_buffer_ += l.line_buffer_
		l.output_buffer_ += "\n"
		l.line_buffer_ = ""
	}
	if !l.have_blank_line_ {
		l.PrintOrBuffer("\n")
	}
	if len(to_print) != 0 {
		l.PrintOrBuffer(to_print)
	}
	l.have_blank_line_ = len(to_print) == 0 || to_print[0] == '\n'
}

// Lock or unlock the console.  Any output sent to the LinePrinter while the
// console is locked will not be printed until it is unlocked.
func (l *LinePrinter) SetConsoleLocked(locked bool) {
	if locked == l.console_locked_ {
		return
	}

	if locked {
		l.PrintOnNewLine("")
	}

	l.console_locked_ = locked

	if !locked {
		l.PrintOnNewLine(l.output_buffer_)
		if len(l.line_buffer_) != 0 {
			l.Print(l.line_buffer_, l.line_type_)
		}
		l.output_buffer_ = ""
		l.line_buffer_ = ""
	}
}
