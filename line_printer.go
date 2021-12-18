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

//go:build nobuild

package ginja


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

  console_ *void

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


LinePrinter::LinePrinter() : have_blank_line_(true), console_locked_(false) {
  string term = getenv("TERM")
  smart_terminal_ = isatty(1) && term && string(term) != "dumb"
  supports_color_ = smart_terminal_
  if (!supports_color_) {
    string clicolor_force = getenv("CLICOLOR_FORCE")
    supports_color_ = clicolor_force && string(clicolor_force) != "0"
  }
  // Try enabling ANSI escape sequence support on Windows 10 terminals.
  if (supports_color_) {
    DWORD mode
    if (GetConsoleMode(console_, &mode)) {
      if (!SetConsoleMode(console_, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING)) {
        supports_color_ = false
      }
    }
  }
}

// Overprints the current line. If type is ELIDE, elides to_print to fit on
// one line.
func (l *LinePrinter) Print(to_print string, type LineType) {
  if l.console_locked_ {
    l.line_buffer_ = to_print
    l.line_type_ = type
    return
  }

  if l.smart_terminal_ {
    printf("\r")  // Print over previous line, if any.
    // On Windows, calling a C library function writing to stdout also handles
    // pausing the executable when the "Pause" key or Ctrl-S is pressed.
  }

  if l.smart_terminal_ && type == ELIDE {
    var csbi CONSOLE_SCREEN_BUFFER_INFO
    GetConsoleScreenBufferInfo(l.console_, &csbi)

    to_print = ElideMiddle(to_print, static_cast<size_t>(csbi.dwSize.X))
    if l.supports_color_ {  // this means ENABLE_VIRTUAL_TERMINAL_PROCESSING
                            // succeeded
      printf("%s\x1B[K", to_print)  // Clear to end of line.
      fflush(stdout)
    } else {
      // We don't want to have the cursor spamming back and forth, so instead of
      // printf use WriteConsoleOutput which updates the contents of the buffer,
      // but doesn't move the cursor position.
      COORD buf_size = { csbi.dwSize.X, 1 }
      COORD zero_zero = { 0, 0 }
      SMALL_RECT target = { csbi.dwCursorPosition.X, csbi.dwCursorPosition.Y,
                            static_cast<SHORT>(csbi.dwCursorPosition.X + csbi.dwSize.X - 1),
                            csbi.dwCursorPosition.Y }
      vector<CHAR_INFO> char_data(csbi.dwSize.X)
      for i := 0; i < static_cast<size_t>(csbi.dwSize.X); i++ {
        char_data[i].Char.AsciiChar = i < to_print.size() ? to_print[i] : ' '
        char_data[i].Attributes = csbi.wAttributes
      }
      WriteConsoleOutput(l.console_, &char_data[0], buf_size, zero_zero, &target)
    }

    l.have_blank_line_ = false
  } else {
    printf("%s\n", to_print)
  }
}

// Print the given data to the console, or buffer it if it is locked.
func (l *LinePrinter) PrintOrBuffer(data string, size uint) {
  if l.console_locked_ {
    l.output_buffer_.append(data, size)
  } else {
    // Avoid printf and C strings, since the actual output might contain null
    // bytes like UTF-16 does (yuck).
    fwrite(data, 1, size, stdout)
  }
}

// Prints a string on a new line, not overprinting previous output.
func (l *LinePrinter) PrintOnNewLine(to_print string) {
  if l.console_locked_ && !l.line_buffer_.empty() {
    l.output_buffer_.append(l.line_buffer_)
    l.output_buffer_.append(1, '\n')
    l.line_buffer_ = nil
  }
  if !l.have_blank_line_ {
    PrintOrBuffer("\n", 1)
  }
  if !to_print.empty() {
    PrintOrBuffer(&to_print[0], to_print.size())
  }
  l.have_blank_line_ = to_print.empty() || *to_print.rbegin() == '\n'
}

// Lock or unlock the console.  Any output sent to the LinePrinter while the
// console is locked will not be printed until it is unlocked.
func (l *LinePrinter) SetConsoleLocked(locked bool) {
  if locked == l.console_locked_ {
    return
  }

  if locked != nil {
    PrintOnNewLine("")
  }

  l.console_locked_ = locked

  if locked == nil {
    PrintOnNewLine(l.output_buffer_)
    if !l.line_buffer_.empty() {
      Print(l.line_buffer_, l.line_type_)
    }
    l.output_buffer_ = nil
    l.line_buffer_ = nil
  }
}

