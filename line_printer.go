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

package ginga


// Prints lines of text, possibly overprinting previously printed lines
// if the terminal supports it.
struct LinePrinter {
  LinePrinter()

  bool is_smart_terminal() const { return smart_terminal_; }
  void set_smart_terminal(bool smart) { smart_terminal_ = smart; }

  bool supports_color() const { return supports_color_; }

  enum LineType {
    FULL,
    ELIDE
  }

  // Whether we can do fancy terminal control codes.
  bool smart_terminal_

  // Whether we can use ISO 6429 (ANSI) color sequences.
  bool supports_color_

  // Whether the caret is at the beginning of a blank line.
  bool have_blank_line_

  // Whether console is locked.
  bool console_locked_

  // Buffered current line while console is locked.
  string line_buffer_

  // Buffered line type while console is locked.
  LineType line_type_

  // Buffered console output while console is locked.
  string output_buffer_

  void* console_

}


LinePrinter::LinePrinter() : have_blank_line_(true), console_locked_(false) {
  term := getenv("TERM")
  smart_terminal_ = isatty(1) && term && string(term) != "dumb"
  if term && string(term) == "dumb" {
    smart_terminal_ = false
  } else {
    console_ = GetStdHandle(STD_OUTPUT_HANDLE)
    CONSOLE_SCREEN_BUFFER_INFO csbi
    smart_terminal_ = GetConsoleScreenBufferInfo(console_, &csbi)
  }
  supports_color_ = smart_terminal_
  if !supports_color_ {
    clicolor_force := getenv("CLICOLOR_FORCE")
    supports_color_ = clicolor_force && string(clicolor_force) != "0"
  }
  // Try enabling ANSI escape sequence support on Windows 10 terminals.
  if supports_color_ {
    DWORD mode
    if GetConsoleMode(console_, &mode) {
      if !SetConsoleMode(console_, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING) {
        supports_color_ = false
      }
    }
  }
}

func (l *LinePrinter) Print(to_print string, type LineType) {
  if console_locked_ {
    line_buffer_ = to_print
    line_type_ = type
    return
  }

  if smart_terminal_ {
    printf("\r")  // Print over previous line, if any.
    // On Windows, calling a C library function writing to stdout also handles
    // pausing the executable when the "Pause" key or Ctrl-S is pressed.
  }

  if smart_terminal_ && type == ELIDE {
    CONSOLE_SCREEN_BUFFER_INFO csbi
    GetConsoleScreenBufferInfo(console_, &csbi)

    to_print = ElideMiddle(to_print, static_cast<size_t>(csbi.dwSize.X))
    if supports_color_ {  // this means ENABLE_VIRTUAL_TERMINAL_PROCESSING
                            // succeeded
      printf("%s\x1B[K", to_print)  // Clear to end of line.
      fflush(stdout)
    } else {
      // We don't want to have the cursor spamming back and forth, so instead of
      // printf use WriteConsoleOutput which updates the contents of the buffer,
      // but doesn't move the cursor position.
      buf_size := { csbi.dwSize.X, 1 }
      zero_zero := { 0, 0 }
      SMALL_RECT target = { csbi.dwCursorPosition.X, csbi.dwCursorPosition.Y,
                            static_cast<SHORT>(csbi.dwCursorPosition.X + csbi.dwSize.X - 1),
                            csbi.dwCursorPosition.Y }
      vector<CHAR_INFO> char_data(csbi.dwSize.X)
      for (size_t i = 0; i < static_cast<size_t>(csbi.dwSize.X); ++i) {
        char_data[i].Char.AsciiChar = i < to_print.size() ? to_print[i] : ' '
        char_data[i].Attributes = csbi.wAttributes
      }
      WriteConsoleOutput(console_, &char_data[0], buf_size, zero_zero, &target)
    }
    // Limit output to width of the terminal if provided so we don't cause
    // line-wrapping.
    winsize size
    if (ioctl(STDOUT_FILENO, TIOCGWINSZ, &size) == 0) && size.ws_col {
      to_print = ElideMiddle(to_print, size.ws_col)
    }
    printf("%s", to_print)
    printf("\x1B[K")  // Clear to end of line.
    fflush(stdout)

    have_blank_line_ = false
  } else {
    printf("%s\n", to_print)
  }
}

func (l *LinePrinter) PrintOrBuffer(data string, size size_t) {
  if console_locked_ {
    output_buffer_.append(data, size)
  } else {
    // Avoid printf and C strings, since the actual output might contain null
    // bytes like UTF-16 does (yuck).
    fwrite(data, 1, size, stdout)
  }
}

func (l *LinePrinter) PrintOnNewLine(to_print string) {
  if console_locked_ && !line_buffer_.empty() {
    output_buffer_.append(line_buffer_)
    output_buffer_.append(1, '\n')
    line_buffer_ = nil
  }
  if !have_blank_line_ {
    PrintOrBuffer("\n", 1)
  }
  if !to_print.empty() {
    PrintOrBuffer(&to_print[0], to_print.size())
  }
  have_blank_line_ = to_print.empty() || *to_print.rbegin() == '\n'
}

func (l *LinePrinter) SetConsoleLocked(locked bool) {
  if locked == console_locked_ {
    return
  }

  if locked != nil {
    PrintOnNewLine("")
  }

  console_locked_ = locked

  if locked == nil {
    PrintOnNewLine(output_buffer_)
    if !line_buffer_.empty() {
      Print(line_buffer_, line_type_)
    }
    output_buffer_ = nil
    line_buffer_ = nil
  }
}

