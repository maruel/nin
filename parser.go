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

package nin

type Parse interface {
	Parse(filename string, input []byte, err *string) bool
}

// Base class for parsers.
type parser struct {
	Parse
	state      *State
	fileReader FileReader
	lexer      Lexer
}

func newParser(state *State, fileReader FileReader, p Parse) parser {
	return parser{
		Parse:      p,
		state:      state,
		fileReader: fileReader,
	}
}

// Load and parse a file.
func (p *parser) Load(filename string, err *string, parent *Lexer) bool {
	defer metricRecord(".ninja parse")()
	contents, err2 := p.fileReader.ReadFile(filename)
	if err2 != nil {
		*err = "loading '" + filename + "': " + err2.Error()
		if parent != nil {
			parent.Error(string(*err), err)
		}
		return false
	}
	return p.Parse.Parse(filename, contents, err)
}

// If the next token is not \a expected, produce an error string
// saying "expected foo, got bar".
func (p *parser) ExpectToken(expected Token, err *string) bool {
	if token := p.lexer.ReadToken(); token != expected {
		msg := "expected " + TokenName(expected) + ", got " + TokenName(token) + tokenErrorHint(expected)
		return p.lexer.Error(msg, err)
	}
	return true
}

// tokenErrorHint returns a human-readable token hint, used in error messages.
func tokenErrorHint(expected Token) string {
	if expected == COLON {
		return " ($ also escapes ':')"
	}
	return ""
}
