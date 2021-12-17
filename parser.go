// Copyright 2018 Google Inc. All Rights Reserved.
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


// Base class for parsers.
type Parser struct {
  Parser(State* state, FileReader* file_reader)
      : state_(state), file_reader_(file_reader) {}

  State* state_
  FileReader* file_reader_
  Lexer lexer_

}


// Load and parse a file.
func (p *Parser) Load(filename string, err *string, parent *Lexer) bool {
  METRIC_RECORD(".ninja parse")
  string contents
  string read_err
  if file_reader_.ReadFile(filename, &contents, &read_err) != FileReader::Okay {
    *err = "loading '" + filename + "': " + read_err
    if parent != nil {
      parent.Error(string(*err), err)
    }
    return false
  }

  // The lexer needs a nul byte at the end of its input, to know when it's done.
  // It takes a StringPiece, and StringPiece's string constructor uses
  // string::data().  data()'s return value isn't guaranteed to be
  // null-terminated (although in practice - libc++, libstdc++, msvc's stl --
  // it is, and C++11 demands that too), so add an explicit nul byte.
  contents.resize(contents.size() + 1)

  return Parse(filename, contents, err)
}

// If the next token is not \a expected, produce an error string
// saying "expected foo, got bar".
func (p *Parser) ExpectToken(expected Lexer::Token, err *string) bool {
  token := lexer_.ReadToken()
  if token != expected {
    string message = string("expected ") + Lexer::TokenName(expected)
    message += string(", got ") + Lexer::TokenName(token)
    message += Lexer::TokenErrorHint(expected)
    return lexer_.Error(message, err)
  }
  return true
}

