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
	name, letValue, err2 := d.parseLet()
	if err2 != nil {
		*err = err2.Error()
		return false
	}
	if name != "ninja_dyndep_version" {
		*err = d.lexer.Error("expected 'ninja_dyndep_version = ...'").Error()
		return false
	}
	version := letValue.Evaluate(d.env)
	major, minor := parseVersion(version)
	if major != 1 || minor != 0 {
		*err = d.lexer.Error("unsupported 'ninja_dyndep_version = " + version + "'").Error()
		return false
	}
	return true
}

func (d *DyndepParser) parseLet() (string, EvalString, error) {
	key := d.lexer.readIdent()
	if key == "" {
		return "", EvalString{}, d.lexer.Error("expected variable name")
	}
	err2 := ""
	if !d.expectToken(EQUALS, &err2) {
		return "", EvalString{}, errors.New(err2)
	}
	eval, err := d.lexer.readEvalString(false)
	return key, eval, err
}

func (d *DyndepParser) parseEdge(err *string) bool {
	// Parse one explicit output.  We expect it to already have an edge.
	// We will record its dynamically-discovered dependency information.
	var dyndeps *Dyndeps
	{
		out0, err2 := d.lexer.readEvalString(true)
		if err2 != nil {
			*err = err2.Error()
			return false
		}
		if len(out0.Parsed) == 0 {
			*err = d.lexer.Error("expected path").Error()
			return false
		}

		path := out0.Evaluate(d.env)
		if len(path) == 0 {
			*err = d.lexer.Error("empty path").Error()
			return false
		}
		path = CanonicalizePath(path)
		node := d.state.Paths[path]
		if node == nil || node.InEdge == nil {
			*err = d.lexer.Error("no build statement exists for '" + path + "'").Error()
			return false
		}
		edge := node.InEdge
		_, ok := d.dyndepFile[edge]
		dyndeps = NewDyndeps()
		d.dyndepFile[edge] = dyndeps
		if ok {
			*err = d.lexer.Error("multiple statements for '" + path + "'").Error()
			return false
		}
	}

	// Disallow explicit outputs.
	{
		out, err2 := d.lexer.readEvalString(true)
		if err2 != nil {
			*err = err2.Error()
			return false
		}
		if len(out.Parsed) != 0 {
			*err = d.lexer.Error("explicit outputs not supported").Error()
			return false
		}
	}

	// Parse implicit outputs, if any.
	var outs []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			out, err2 := d.lexer.readEvalString(true)
			if err2 != nil {
				*err = err2.Error()
				return false // TODO(maruel): Bug upstream.
			}
			if len(out.Parsed) == 0 {
				break
			}
			outs = append(outs, out)
		}
	}

	if !d.expectToken(COLON, err) {
		return false
	}

	if ruleName := d.lexer.readIdent(); ruleName == "" || ruleName != "dyndep" {
		*err = d.lexer.Error("expected build command name 'dyndep'").Error()
		return false
	}

	// Disallow explicit inputs.
	{
		in, err2 := d.lexer.readEvalString(true)
		if err2 != nil {
			*err = err2.Error()
			return false
		}
		if len(in.Parsed) != 0 {
			*err = d.lexer.Error("explicit inputs not supported").Error()
			return false
		}
	}

	// Parse implicit inputs, if any.
	var ins []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			in, err2 := d.lexer.readEvalString(true)
			if err2 != nil {
				*err = err2.Error()
				return false // TODO(maruel): Bug upstream.
			}
			if len(in.Parsed) == 0 {
				break
			}
			ins = append(ins, in)
		}
	}

	// Disallow order-only inputs.
	if d.lexer.PeekToken(PIPE2) {
		*err = d.lexer.Error("order-only inputs not supported").Error()
		return false
	}

	if !d.expectToken(NEWLINE, err) {
		return false
	}

	if d.lexer.PeekToken(INDENT) {
		key, val, err2 := d.parseLet()
		if err2 != nil {
			*err = err2.Error()
			return false
		}
		if key != "restat" {
			*err = d.lexer.Error("binding is not 'restat'").Error()
			return false
		}
		value := val.Evaluate(d.env)
		dyndeps.restat = value != ""
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
