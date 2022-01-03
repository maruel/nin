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
	state_      *State
	fileReader_ FileReader
	lexer_      Lexer
}

func NewParser(state *State, fileReader FileReader, p Parse) Parser {
	return Parser{
		Parse:       p,
		state_:      state,
		fileReader_: fileReader,
	}
}

// Load and parse a file.
func (p *Parser) Load(filename string, err *string, parent *Lexer) bool {
	defer METRIC_RECORD(".ninja parse")()
	contents := ""
	readErr := ""
	if p.fileReader_.ReadFile(filename, &contents, &readErr) != Okay {
		*err = "loading '" + filename + "': " + readErr
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
