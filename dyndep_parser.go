// Copyright 2015 Google Inc. All Rights Reserved.
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

import "errors"

// Parses dyndep files.
type DyndepParser struct {
	parser
	dyndepFile DyndepFile
	env        *BindingEnv
}

func NewDyndepParser(state *State, fileReader FileReader, dyndepFile DyndepFile) *DyndepParser {
	d := &DyndepParser{
		dyndepFile: dyndepFile,
	}
	d.parser = newParser(state, fileReader, d)
	return d
}

// Parse a file, given its contents as a string.
func (d *DyndepParser) Parse(filename string, input []byte, err *string) bool {
	d.lexer.Start(filename, input)

	// Require a supported ninjaDyndepVersion value immediately so
	// we can exit before encountering any syntactic surprises.
	haveDyndepVersion := false

	for {
		token := d.lexer.ReadToken()
		switch token {
		case BUILD:
			if !haveDyndepVersion {
				*err = d.lexer.Error("expected 'ninja_dyndep_version = ...'").Error()
				return false
			}
			if !d.parseEdge(err) {
				return false
			}
		case IDENT:
			d.lexer.UnreadToken()
			if haveDyndepVersion {
				*err = d.lexer.Error("unexpected " + token.String()).Error()
				return false
			}
			if !d.parseDyndepVersion(err) {
				return false
			}
			haveDyndepVersion = true
		case ERROR:
			*err = d.lexer.Error(d.lexer.DescribeLastError()).Error()
		case TEOF:
			if !haveDyndepVersion {
				*err = d.lexer.Error("expected 'ninja_dyndep_version = ...'").Error()
				return false
			}
			return true
		case NEWLINE:
		default:
			*err = d.lexer.Error("unexpected " + token.String()).Error()
			return false
		}
	}
}

func (d *DyndepParser) parseDyndepVersion(err *string) bool {
	eval := EvalString{}
	if name, err2 := d.parseLet(&eval); err2 != nil {
		*err = err2.Error()
		return false
	} else if name != "ninja_dyndep_version" {
		*err = d.lexer.Error("expected 'ninja_dyndep_version = ...'").Error()
		return false
	}
	version := eval.Evaluate(d.env)
	major, minor := parseVersion(version)
	if major != 1 || minor != 0 {
		*err = d.lexer.Error("unsupported 'ninja_dyndep_version = " + version + "'").Error()
		return false
	}
	return true
}

func (d *DyndepParser) parseLet(eval *EvalString) (string, error) {
	key := d.lexer.readIdent()
	if key == "" {
		return key, d.lexer.Error("expected variable name")
	}
	err2 := ""
	if !d.expectToken(EQUALS, &err2) {
		return key, errors.New(err2)
	}
	return key, d.lexer.readEvalString(eval, false)
		eval.Parsed = eval.Parsed[:0]
		if key, err2 := d.parseLet(&eval); err2 != nil {
			*err = err2.Error()
			return false
		} else if key != "restat" {
			*err = d.lexer.Error("binding is not 'restat'").Error()
			return false
		}
		dyndeps.restat = eval.Evaluate(d.env) != ""
	}

	dyndeps.implicitInputs = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			*err = d.lexer.Error("empty path").Error()
			return false
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitInputs = append(dyndeps.implicitInputs, n)
	}

	dyndeps.implicitOutputs = make([]*Node, 0, len(outs))
	for _, i := range outs {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			*err = d.lexer.Error("empty path").Error()
			return false
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitOutputs = append(dyndeps.implicitOutputs, n)
	}
	return true
}
