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
	Parse(filename, input string, err *string) bool
}

// Base class for parsers.
type Parser struct {
	Parse
	state_       *State
	file_reader_ FileReader
	lexer_       Lexer
}

func NewParser(state *State, file_reader FileReader, p Parse) Parser {
	return Parser{
		Parse:        p,
		state_:       state,
		file_reader_: file_reader,
	}
}

// Load and parse a file.
func (p *Parser) Load(filename string, err *string, parent *Lexer) bool {
	defer METRIC_RECORD(".ninja parse")()
	contents := ""
	read_err := ""
	if p.file_reader_.ReadFile(filename, &contents, &read_err) != Okay {
		*err = "loading '" + filename + "': " + read_err
		if parent != nil {
			parent.Error(string(*err), err)
		}
		return false
	}
	return p.Parse.Parse(filename, contents, err)
}

// If the next token is not \a expected, produce an error string
// saying "expected foo, got bar".
func (p *Parser) ExpectToken(expected Token, err *string) bool {
	token := p.lexer_.ReadToken()
	if token != expected {
		message := "expected " + TokenName(expected)
		message += string(", got ") + TokenName(token)
		message += TokenErrorHint(expected)
		return p.lexer_.Error(message, err)
	}
	return true
}
