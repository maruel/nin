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

// DyndepParser parses dyndep files.
type DyndepParser struct {
	// Immutable.
	fileReader FileReader

	// Mutable.
	lexer      lexer
	state      *State
	dyndepFile DyndepFile
	env        *BindingEnv
}

// NewDyndepParser returns an initialized DyndepParser.
func NewDyndepParser(state *State, fileReader FileReader, dyndepFile DyndepFile) *DyndepParser {
	return &DyndepParser{
		fileReader: fileReader,
		state:      state,
		dyndepFile: dyndepFile,
	}
}

// If the next token is not \a expected, produce an error string
// saying "expected foo, got bar".
func (d *DyndepParser) expectToken(expected Token) error {
	if token := d.lexer.ReadToken(); token != expected {
		return d.lexer.Error("expected " + expected.String() + ", got " + token.String() + expected.errorHint())
	}
	return nil
}

// Parse a file, given its contents as a string.
func (d *DyndepParser) Parse(filename string, input []byte) error {
	defer metricRecord(".ninja parse")()
	d.lexer.Start(filename, input)

	// Require a supported ninjaDyndepVersion value immediately so
	// we can exit before encountering any syntactic surprises.
	haveDyndepVersion := false

	for {
		token := d.lexer.ReadToken()
		switch token {
		case BUILD:
			if !haveDyndepVersion {
				return d.lexer.Error("expected 'ninja_dyndep_version = ...'")
			}
			if err := d.parseEdge(); err != nil {
				return err
			}
		case IDENT:
			d.lexer.UnreadToken()
			if haveDyndepVersion {
				return d.lexer.Error("unexpected " + token.String())
			}
			if err := d.parseDyndepVersion(); err != nil {
				return err
			}
			haveDyndepVersion = true
		case ERROR:
			return d.lexer.Error(d.lexer.DescribeLastError())
		case TEOF:
			if !haveDyndepVersion {
				return d.lexer.Error("expected 'ninja_dyndep_version = ...'")
			}
			return nil
		case NEWLINE:
		default:
			return d.lexer.Error("unexpected " + token.String())
		}
	}
}

func (d *DyndepParser) parseDyndepVersion() error {
	name, letValue, err := d.parseLet()
	if err != nil {
		return err
	}
	if name != "ninja_dyndep_version" {
		return d.lexer.Error("expected 'ninja_dyndep_version = ...'")
	}
	version := letValue.Evaluate(d.env)
	major, minor := parseVersion(version)
	if major != 1 || minor != 0 {
		return d.lexer.Error("unsupported 'ninja_dyndep_version = " + version + "'")
	}
	return nil
}

func (d *DyndepParser) parseLet() (string, EvalString, error) {
	key := d.lexer.readIdent()
	eval := EvalString{}
	var err error
	if key == "" {
		err = d.lexer.Error("expected variable name")
	} else if err = d.expectToken(EQUALS); err == nil {
		eval, err = d.lexer.readEvalString(false)
	}
	return key, eval, err
}

func (d *DyndepParser) parseEdge() error {
	// Parse one explicit output.  We expect it to already have an edge.
	// We will record its dynamically-discovered dependency information.
	var dyndeps *Dyndeps
	eval, err := d.lexer.readEvalString(true)
	if err != nil {
		return err
	} else if len(eval.Parsed) == 0 {
		return d.lexer.Error("expected path")
	}

	path := eval.Evaluate(d.env)
	if len(path) == 0 {
		return d.lexer.Error("empty path")
	}
	path = CanonicalizePath(path)
	node := d.state.Paths[path]
	if node == nil || node.InEdge == nil {
		return d.lexer.Error("no build statement exists for '" + path + "'")
	}
	edge := node.InEdge
	if _, ok := d.dyndepFile[edge]; ok {
		return d.lexer.Error("multiple statements for '" + path + "'")
	}
	dyndeps = &Dyndeps{}
	d.dyndepFile[edge] = dyndeps

	// Disallow explicit outputs.
	eval, err = d.lexer.readEvalString(true)
	if err != nil {
		return err
	} else if len(eval.Parsed) != 0 {
		return d.lexer.Error("explicit outputs not supported")
	}

	// Parse implicit outputs, if any.
	var outs []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			eval, err = d.lexer.readEvalString(true)
			if err != nil {
				// TODO(maruel): Bug upstream.
				return err
			}
			if len(eval.Parsed) == 0 {
				break
			}
			outs = append(outs, eval)
		}
	}

	if err = d.expectToken(COLON); err != nil {
		return err
	}

	if ruleName := d.lexer.readIdent(); ruleName == "" || ruleName != "dyndep" {
		return d.lexer.Error("expected build command name 'dyndep'")
	}

	// Disallow explicit inputs.
	eval, err = d.lexer.readEvalString(true)
	if err != nil {
		return err
	} else if len(eval.Parsed) != 0 {
		return d.lexer.Error("explicit inputs not supported")
	}

	// Parse implicit inputs, if any.
	var ins []EvalString
	if d.lexer.PeekToken(PIPE) {
		for {
			eval, err = d.lexer.readEvalString(true)
			if err != nil {
				// TODO(maruel): Bug upstream.
				return err
			}
			if len(eval.Parsed) == 0 {
				break
			}
			ins = append(ins, eval)
		}
	}

	// Disallow order-only inputs.
	if d.lexer.PeekToken(PIPE2) {
		return d.lexer.Error("order-only inputs not supported")
	}

	if err = d.expectToken(NEWLINE); err != nil {
		return err
	}

	if d.lexer.PeekToken(INDENT) {
		key, val, err := d.parseLet()
		if err != nil {
			return err
		}
		if key != "restat" {
			return d.lexer.Error("binding is not 'restat'")
		}
		value := val.Evaluate(d.env)
		dyndeps.restat = value != ""
	}

	dyndeps.implicitInputs = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			return d.lexer.Error("empty path")
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitInputs = append(dyndeps.implicitInputs, n)
	}

	dyndeps.implicitOutputs = make([]*Node, 0, len(outs))
	for _, i := range outs {
		path := i.Evaluate(d.env)
		if len(path) == 0 {
			return d.lexer.Error("empty path")
		}
		n := d.state.GetNode(CanonicalizePathBits(path))
		dyndeps.implicitOutputs = append(dyndeps.implicitOutputs, n)
	}
	return nil
}
