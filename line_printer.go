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

package nin

import (
	"fmt"
	"os"
	"runtime"
)

// Prints lines of text, possibly overprinting previously printed lines
// if the terminal supports it.
type LinePrinter struct {
	// Whether we can do fancy terminal control codes.
	smartTerminal bool

	// Whether we can use ISO 6429 (ANSI) color sequences.
	supportsColor bool

	// Whether the caret is at the beginning of a blank line.
	haveBlankLine bool

	// Whether console is locked.
	consoleLocked bool

	// Buffered current line while console is locked.
	lineBuffer string

	// Buffered line type while console is locked.
	elide bool

	// Buffered console output while console is locked.
	outputBuffer string

	//console *void
}

func (l *LinePrinter) isSmartTerminal() bool {
	return l.smartTerminal
}
func (l *LinePrinter) setSmartTerminal(smart bool) {
	l.smartTerminal = smart
}

func NewLinePrinter() LinePrinter {
	l := LinePrinter{
		haveBlankLine: true,
	}
	/*
		if os.Getenv("TERM") != "dumb" {
			if runtime.GOOS != "windows" {
				// Don't panic for now.
				//l.smartTerminal = isatty(1)
			} else {
				// Don't panic for now.
				//console = GetStdHandle(STD_OUTPUT_HANDLE)
				//var csbi CONSOLE_SCREEN_BUFFER_INFO
				//smartTerminal = GetConsoleScreenBufferInfo(console, &csbi)
			}
		}
	*/
	l.supportsColor = l.smartTerminal
	if !l.supportsColor {
		f := os.Getenv("CLICOLOR_FORCE")
		l.supportsColor = f != "" && f != "0"
	}
	// Try enabling ANSI escape sequence support on Windows 10 terminals.
	if runtime.GOOS == "windows" {
		if l.supportsColor {
			panic("TODO")
			/*
				var mode DWORD
				if GetConsoleMode(console, &mode) {
					if !SetConsoleMode(console, mode|ENABLE_VIRTUAL_TERMINAL_PROCESSING) {
						supportsColor = false
					}
				}
			*/
		}
	}
	return l
}

// Overprints the current line. If type is ELIDE, elides toPrint to fit on
// one line.
func (l *LinePrinter) Print(toPrint string, elide bool) {
	if l.consoleLocked {
		l.lineBuffer = toPrint
		l.elide = elide
		return
	}

	if l.smartTerminal {
		fmt.Printf("\r") // Print over previous line, if any.
		// On Windows, calling a C library function writing to stdout also handles
		// pausing the executable when the "Pause" key or Ctrl-S is pressed.
	}

	if l.smartTerminal && elide {
		l.haveBlankLine = false
		if runtime.GOOS == "windows" {
			panic("TODO")
			/*
				var csbi CONSOLE_SCREEN_BUFFER_INFO
				GetConsoleScreenBufferInfo(l.console, &csbi)
				toPrint = ElideMiddle(toPrint, csbi.dwSize.X)
				if l.supportsColor {
					// this means ENABLE_VIRTUAL_TERMINAL_PROCESSING
					// succeeded
					fmt.Printf("%s\x1B[K", toPrint) // Clear to end of line.
					fflush(stdout)
				} else {
					// We don't want to have the cursor spamming back and forth, so instead of
					// printf use WriteConsoleOutput which updates the contents of the buffer,
					// but doesn't move the cursor position.
					bufSize := COORD{csbi.dwSize.X, 1}
					zeroZero := COORD{0, 0}
					target := SMALL_RECT{csbi.dwCursorPosition.X, csbi.dwCursorPosition.Y,
						csbi.dwCursorPosition.X + csbi.dwSize.X - 1,
						csbi.dwCursorPosition.Y}
					charData := make([]CHAR_INFO, csbi.dwSize.X)
					for i := 0; i < csbi.dwSize.X; i++ {
						if i < len(toPrint) {
							charData[i].Char.AsciiChar = toPrint[i]
						} else {
							charData[i].Char.AsciiChar = ' '
						}
						charData[i].Attributes = csbi.wAttributes
					}
					WriteConsoleOutput(l.console, &charData[0], bufSize, zeroZero, &target)
				}
			*/
		} else {
			panic("TODO")
			/*
				// Limit output to width of the terminal if provided so we don't cause
				// line-wrapping.
				var size winsize
				if ioctl(STDOUT_FILENO, TIOCGWINSZ, &size) == 0 && size.wsCol {
					toPrint = ElideMiddle(toPrint, size.wsCol)
				}
				fmt.Printf("%s", toPrint)
				fmt.Printf("\x1B[K") // Clear to end of line.
				fflush(stdout)
			*/
		}
	} else {
		fmt.Printf("%s\n", toPrint)
	}
}

// Print the given data to the console, or buffer it if it is locked.
func (l *LinePrinter) PrintOrBuffer(data string) {
	if l.consoleLocked {
		l.outputBuffer += data
	} else {
		// Avoid printf and C strings, since the actual output might contain null
		// bytes like UTF-16 does (yuck).
		_, _ = os.Stdout.WriteString(data)
	}
}

// Prints a string on a new line, not overprinting previous output.
func (l *LinePrinter) PrintOnNewLine(toPrint string) {
	if l.consoleLocked && len(l.lineBuffer) != 0 {
		l.outputBuffer += l.lineBuffer
		l.outputBuffer += "\n"
		l.lineBuffer = ""
	}
	if !l.haveBlankLine {
		l.PrintOrBuffer("\n")
	}
	if len(toPrint) != 0 {
		l.PrintOrBuffer(toPrint)
	}
	l.haveBlankLine = len(toPrint) == 0 || toPrint[0] == '\n'
}

// Lock or unlock the console.  Any output sent to the LinePrinter while the
// console is locked will not be printed until it is unlocked.
func (l *LinePrinter) SetConsoleLocked(locked bool) {
	if locked == l.consoleLocked {
		return
	}

	if locked {
		l.PrintOnNewLine("")
	}

	l.consoleLocked = locked

	if !locked {
		l.PrintOnNewLine(l.outputBuffer)
		if len(l.lineBuffer) != 0 {
			l.Print(l.lineBuffer, l.elide)
		}
		l.outputBuffer = ""
		l.lineBuffer = ""
	}
}
