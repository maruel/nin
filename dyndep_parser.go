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

package ginja

// Parses dyndep files.
type DyndepParser struct {
	Parser
	dyndep_file_ DyndepFile
	env_         *BindingEnv
}

// Parse a text string of input.  Used by tests.
func (d *DyndepParser) ParseTest(input string, err *string) bool {
	return d.Parse("input", input, err)
}

func NewDyndepParser(state *State, file_reader FileReader, dyndep_file DyndepFile) *DyndepParser {
	d := &DyndepParser{
		dyndep_file_: dyndep_file,
	}
	d.Parser = NewParser(state, file_reader, d)
	return d
}

// Parse a file, given its contents as a string.
func (d *DyndepParser) Parse(filename string, input string, err *string) bool {
	d.lexer_.Start(filename, input)

	// Require a supported ninja_dyndep_version value immediately so
	// we can exit before encountering any syntactic surprises.
	haveDyndepVersion := false

	for {
		token := d.lexer_.ReadToken()
		switch token {
		case BUILD:
			{
				if !haveDyndepVersion {
					return d.lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
				}
				if !d.ParseEdge(err) {
					return false
				}
				break
			}
		case IDENT:
			{
				d.lexer_.UnreadToken()
				if haveDyndepVersion {
					return d.lexer_.Error(string("unexpected ")+TokenName(token), err)
				}
				if !d.ParseDyndepVersion(err) {
					return false
				}
				haveDyndepVersion = true
				break
			}
		case ERROR:
			return d.lexer_.Error(d.lexer_.DescribeLastError(), err)
		case TEOF:
			if !haveDyndepVersion {
				return d.lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
			}
			return true
		case NEWLINE:
			break
		default:
			return d.lexer_.Error(string("unexpected ")+TokenName(token), err)
		}
	}
	return false // not reached
}

func (d *DyndepParser) ParseDyndepVersion(err *string) bool {
	name := ""
	let_value := EvalString{}
	if !d.ParseLet(&name, &let_value, err) {
		return false
	}
	if name != "ninja_dyndep_version" {
		return d.lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
	}
	version := let_value.Evaluate(d.env_)
	major, minor := ParseVersion(version)
	if major != 1 || minor != 0 {
		return d.lexer_.Error(string("unsupported 'ninja_dyndep_version = ")+version+"'", err)
		return false
	}
	return true
}

func (d *DyndepParser) ParseLet(key *string, value *EvalString, err *string) bool {
	if !d.lexer_.ReadIdent(key) {
		return d.lexer_.Error("expected variable name", err)
	}
	if !d.ExpectToken(EQUALS, err) {
		return false
	}
	if !d.lexer_.ReadVarValue(value, err) {
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
		if !d.lexer_.ReadPath(&out0, err) {
			return false
		}
		if out0.empty() {
			return d.lexer_.Error("expected path", err)
		}

		path := out0.Evaluate(d.env_)
		if len(path) == 0 {
			return d.lexer_.Error("empty path", err)
		}
		var slash_bits uint64
		path = CanonicalizePath(path, &slash_bits)
		node := d.state_.LookupNode(path)
		if node == nil || node.in_edge() == nil {
			return d.lexer_.Error("no build statement exists for '"+path+"'", err)
		}
		edge := node.in_edge()
		_, ok := d.dyndep_file_[edge]
		dyndeps = NewDyndeps()
		d.dyndep_file_[edge] = dyndeps
		if ok {
			return d.lexer_.Error("multiple statements for '"+path+"'", err)
		}
	}

	// Disallow explicit outputs.
	{
		var out EvalString
		if !d.lexer_.ReadPath(&out, err) {
			return false
		}
		if !out.empty() {
			return d.lexer_.Error("explicit outputs not supported", err)
		}
	}

	// Parse implicit outputs, if any.
	var outs []EvalString
	if d.lexer_.PeekToken(PIPE) {
		for {
			var out EvalString
			if !d.lexer_.ReadPath(&out, err) {
				return false // TODO(maruel): Bug upstream.
			}
			if out.empty() {
				break
			}
			outs = append(outs, out)
		}
	}

	if !d.ExpectToken(COLON, err) {
		return false
	}

	rule_name := ""
	if !d.lexer_.ReadIdent(&rule_name) || rule_name != "dyndep" {
		return d.lexer_.Error("expected build command name 'dyndep'", err)
	}

	// Disallow explicit inputs.
	{
		var in EvalString
		if !d.lexer_.ReadPath(&in, err) {
			return false
		}
		if !in.empty() {
			return d.lexer_.Error("explicit inputs not supported", err)
		}
	}

	// Parse implicit inputs, if any.
	var ins []EvalString
	if d.lexer_.PeekToken(PIPE) {
		for {
			var in EvalString
			if !d.lexer_.ReadPath(&in, err) {
				return false // TODO(maruel): Bug upstream.
			}
			if in.empty() {
				break
			}
			ins = append(ins, in)
		}
	}

	// Disallow order-only inputs.
	if d.lexer_.PeekToken(PIPE2) {
		return d.lexer_.Error("order-only inputs not supported", err)
	}

	if !d.ExpectToken(NEWLINE, err) {
		return false
	}

	if d.lexer_.PeekToken(INDENT) {
		key := ""
		var val EvalString
		if !d.ParseLet(&key, &val, err) {
			return false
		}
		if key != "restat" {
			return d.lexer_.Error("binding is not 'restat'", err)
		}
		value := val.Evaluate(d.env_)
		dyndeps.restat_ = value != ""
	}

	dyndeps.implicit_inputs_ = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(d.env_)
		if len(path) == 0 {
			return d.lexer_.Error("empty path", err)
		}
		var slash_bits uint64
		path = CanonicalizePath(path, &slash_bits)
		n := d.state_.GetNode(path, slash_bits)
		dyndeps.implicit_inputs_ = append(dyndeps.implicit_inputs_, n)
	}

	dyndeps.implicit_outputs_ = make([]*Node, 0, len(outs))
	for _, i := range outs {
		path := i.Evaluate(d.env_)
		if len(path) == 0 {
			return d.lexer_.Error("empty path", err)
		}
		var slash_bits uint64
		path = CanonicalizePath(path, &slash_bits)
		n := d.state_.GetNode(path, slash_bits)
		dyndeps.implicit_outputs_ = append(dyndeps.implicit_outputs_, n)
	}
	return true
}
