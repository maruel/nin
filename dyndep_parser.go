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

// Parses dyndep files.
type DyndepParser struct {
	Parser
	dyndepFile DyndepFile
	env        *BindingEnv
}

// Parse a text string of input.  Used by tests.
func (d *DyndepParser) ParseTest(input string, err *string) bool {
	return d.Parse("input", input+"\x00", err)
}

func NewDyndepParser(state *State, fileReader FileReader, dyndepFile DyndepFile) *DyndepParser {
	d := &DyndepParser{
		dyndepFile: dyndepFile,
	}
	d.Parser = NewParser(state, fileReader, d)
	return d
}

// Parse a file, given its contents as a string.
func (d *DyndepParser) Parse(filename string, input string, err *string) bool {
	d.lexer.Start(filename, input)

	// Require a supported ninjaDyndepVersion value immediately so
	// we can exit before encountering any syntactic surprises.
	haveDyndepVersion := false

	for {
		token := d.lexer.ReadToken()
		switch token {
		case BUILD:
			if !haveDyndepVersion {
				return d.lexer.Error("expected 'ninja_dyndep_version = ...'", err)
			}
			if !d.ParseEdge(err) {
				return false
			}
		case IDENT:
			d.lexer.UnreadToken()
			if haveDyndepVersion {
				return d.lexer.Error(string("unexpected ")+TokenName(token), err)
			}
			if !d.ParseDyndepVersion(err) {
				return false
			}
			haveDyndepVersion = true
		case ERROR:
			return d.lexer.Error(d.lexer.DescribeLastError(), err)
		case TEOF:
			if !haveDyndepVersion {
				return d.lexer.Error("expected 'ninja_dyndep_version = ...'", err)
			}
			return true
		case NEWLINE:
		default:
			return d.lexer.Error(string("unexpected ")+TokenName(token), err)
		}
	}
}

func (d *DyndepParser) ParseDyndepVersion(err *string) bool {
	name := ""
	letValue := EvalString{}
	if !d.ParseLet(&name, &letValue, err) {
		return false
	}
	if name != "ninja_dyndep_version" {
		return d.lexer.Error("expected 'ninja_dyndep_version = ...'", err)
	}
	version := letValue.Evaluate(d.env)
	major, minor := ParseVersion(version)
	if major != 1 || minor != 0 {
		return d.lexer.Error("unsupported 'ninja_dyndep_version = "+version+"'", err)
	}
	return true
}

func (d *DyndepParser) ParseLet(key *string, value *EvalString, err *string) bool {
	if !d.lexer.ReadIdent(key) {
		return d.lexer.Error("expected variable name", err)
	}
	if !d.ExpectToken(EQUALS, err) {
		return false
	}
	if !d.lexer.ReadVarValue(value, err) {
		return false
	}
	return true
}

func (d *DyndepParser) ParseEdge(err *string) bool {
	// Parse one explicit output.  We expect it to already have an edge.
	// We will record its dynamically-discovered dependency information.
	var dyndeps *Dyndeps
	{
		out0 := EvalString{}
		if !d.lexer.ReadPath(&out0, err) {
			return false
		}
		if len(out0.Parsed) == 0 {
			return d.lexer.Error("expected path", err)
		}

		path := out0.Evaluate(d.env)
		if len(path) == 0 {
			return d.lexer.Error("empty path", err)
		}
		path = CanonicalizePath(path)
		node := d.state.Paths[path]
		if node == nil || node.InEdge == nil {
			return d.lexer.Error("no build statement exists for '"+path+"'", err)
		}
		edge := node.InEdge
		_, ok := d.dyndepFile[edge]
		dyndeps = NewDyndeps()
		d.dyndepFile[edge] = dyndeps
		if ok {
			return d.lexer.Error("multiple statements for '"+path+"'", err)
		}
	}

	// Disallow explicit outputs.
	{
		var out EvalString
		if !d.lexer.ReadPath(&out, err) {
			return false
		}
		if len(out.Parsed) != 0 {
			return d.lexer.Error("explicit outputs not supported", err)
		}
	}

	// Parse implicit outputs, if any.
	var outs []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			var out EvalString
			if !d.lexer.ReadPath(&out, err) {
				return false // TODO(maruel): Bug upstream.
			}
			if len(out.Parsed) == 0 {
				break
			}
			outs = append(outs, out)
		}
	}

	if !d.ExpectToken(COLON, err) {
		return false
	}

	ruleName := ""
	if !d.lexer.ReadIdent(&ruleName) || ruleName != "dyndep" {
		return d.lexer.Error("expected build command name 'dyndep'", err)
	}

	// Disallow explicit inputs.
	{
		var in EvalString
		if !d.lexer.ReadPath(&in, err) {
			return false
		}
		if len(in.Parsed) != 0 {
			return d.lexer.Error("explicit inputs not supported", err)
		}
	}

	// Parse implicit inputs, if any.
	var ins []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			var in EvalString
			if !d.lexer.ReadPath(&in, err) {
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
		return d.lexer.Error("order-only inputs not supported", err)
	}

	if !d.ExpectToken(NEWLINE, err) {
		return false
	}

	if d.lexer.PeekToken(INDENT) {
		key := ""
		var val EvalString
		if !d.ParseLet(&key, &val, err) {
			return false
		}
		if key != "restat" {
			return d.lexer.Error("binding is not 'restat'", err)
		}
		value := val.Evaluate(d.env)
		dyndeps.restat = value != ""
	}

	dyndeps.implicitInputs = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			return d.lexer.Error("empty path", err)
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitInputs = append(dyndeps.implicitInputs, n)
	}

	dyndeps.implicitOutputs = make([]*Node, 0, len(outs))
	for _, i := range outs {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			return d.lexer.Error("empty path", err)
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitOutputs = append(dyndeps.implicitOutputs, n)
	}
	return true
}
