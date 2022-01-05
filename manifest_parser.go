// Copyright 2011 Google Inc. All Rights Reserved.
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

import (
	"errors"
	"strconv"
)

// ManifestParserOptions are the options when parsing a build.ninja file.
type ManifestParserOptions struct {
	// ErrOnDupeEdge causes duplicate rules for one target to print an error,
	// otherwise warns.
	ErrOnDupeEdge bool
	// ErrOnPhonyCycle causes phony cycles to print an error, otherwise warns.
	ErrOnPhonyCycle bool
	// Silence warnings.
	Quiet bool
}

// Parses .ninja files.
type ManifestParser struct {
	parser
	env     *BindingEnv
	options ManifestParserOptions
}

func NewManifestParser(state *State, fileReader FileReader, options ManifestParserOptions) *ManifestParser {
	m := &ManifestParser{
		options: options,
		env:     state.Bindings,
	}
	m.parser = newParser(state, fileReader, m)
	return m
}

// Parse a file, given its contents as a string.
func (m *ManifestParser) Parse(filename string, input []byte, err *string) bool {
	m.lexer.Start(filename, input)

	for {
		token := m.lexer.ReadToken()
		switch token {
		case POOL:
			if err2 := m.parsePool(); err2 != nil {
				*err = err2.Error()
				return false
			}
		case BUILD:
			if err2 := m.parseEdge(); err2 != nil {
				*err = err2.Error()
				return false
			}
		case RULE:
			if err2 := m.parseRule(); err2 != nil {
				*err = err2.Error()
				return false
			}
		case DEFAULT:
			if err2 := m.parseDefault(); err2 != nil {
				*err = err2.Error()
				return false
			}
		case IDENT:
			{
				m.lexer.UnreadToken()
				name, letValue, err2 := m.parseLet()
				if err2 != nil {
					*err = err2.Error()
					return false
				}
				value := letValue.Evaluate(m.env)
				// Check ninjaRequiredVersion immediately so we can exit
				// before encountering any syntactic surprises.
				if name == "ninja_required_version" {
					if err2 := checkNinjaVersion(value); err2 != nil {
						*err = err2.Error()
						return false
					}
				}
				m.env.Bindings[name] = value
			}
		case INCLUDE:
			if err2 := m.parseFileInclude(false); err2 != nil {
				*err = err2.Error()
				return false
			}
		case SUBNINJA:
			if err2 := m.parseFileInclude(true); err2 != nil {
				*err = err2.Error()
				return false
			}
		case ERROR:
			*err = m.lexer.Error(m.lexer.DescribeLastError()).Error()
			return false
		case TEOF:
			return true
		case NEWLINE:
		default:
			*err = m.lexer.Error("unexpected " + token.String()).Error()
			return false
		}
	}
}

// Parse various statement types.
func (m *ManifestParser) parsePool() error {
	name := m.lexer.readIdent()
	if name == "" {
		return m.lexer.Error("expected pool name")
	}

	err2 := ""
	if !m.expectToken(NEWLINE, &err2) {
		return errors.New(err2)
	}

	if m.state.Pools[name] != nil {
		return m.lexer.Error("duplicate pool '" + name + "'")
	}

	depth := -1

	for m.lexer.PeekToken(INDENT) {
		key, value, err := m.parseLet()
		if err != nil {
			return err
		}
		if key != "depth" {
			return m.lexer.Error("unexpected variable '" + key + "'")
		}
		// TODO(maruel): Do we want to use ParseInt() here? Aka support hex.
		if depth, err = strconv.Atoi(value.Evaluate(m.env)); depth < 0 || err != nil {
			return m.lexer.Error("invalid pool depth")
		}
	}

	if depth < 0 {
		return m.lexer.Error("expected 'depth =' line")
	}

	m.state.Pools[name] = NewPool(name, depth)
	return nil
}

func (m *ManifestParser) parseRule() error {
	name := m.lexer.readIdent()
	if name == "" {
		return m.lexer.Error("expected rule name")
	}

	err2 := ""
	if !m.expectToken(NEWLINE, &err2) {
		return errors.New(err2)
	}

	if m.env.Rules[name] != nil {
		return m.lexer.Error("duplicate rule '" + name + "'")
	}

	rule := NewRule(name)
	for m.lexer.PeekToken(INDENT) {
		key, value, err := m.parseLet()
		if err != nil {
			return err
		}

		if !IsReservedBinding(key) {
			// Die on other keyvals for now; revisit if we want to add a
			// scope here.
			return m.lexer.Error("unexpected variable '" + key + "'")
		}
		rule.Bindings[key] = &value
	}

	b1, ok1 := rule.Bindings["rspfile"]
	b2, ok2 := rule.Bindings["rspfile_content"]
	if ok1 != ok2 || (ok1 && (len(b1.Parsed) == 0) != (len(b2.Parsed) == 0)) {
		return m.lexer.Error("rspfile and rspfile_content need to be both specified")
	}

	b, ok := rule.Bindings["command"]
	if !ok || len(b.Parsed) == 0 {
		return m.lexer.Error("expected 'command =' line")
	}
	m.env.Rules[rule.Name] = rule
	return nil
}

func (m *ManifestParser) parseLet() (string, EvalString, error) {
	key := m.lexer.readIdent()
	if key == "" {
		return "", EvalString{}, m.lexer.Error("expected variable name")
	}
	err2 := ""
	if !m.expectToken(EQUALS, &err2) {
		return "", EvalString{}, errors.New(err2)
	}
	ev, err := m.lexer.readEvalString(false)
	return key, ev, err
}

func (m *ManifestParser) parseDefault() error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	if len(eval.Parsed) == 0 {
		return m.lexer.Error("expected target name")
	}

	for {
		path := eval.Evaluate(m.env)
		if len(path) == 0 {
			return m.lexer.Error("empty path")

		}
		defaultErr := ""
		if !m.state.addDefault(CanonicalizePath(path), &defaultErr) {
			return m.lexer.Error(defaultErr)
		}

		eval, err = m.lexer.readEvalString(true)
		if err != nil {
			return err
		}
		if len(eval.Parsed) == 0 {
			break
		}
	}

	err2 := ""
	if !m.expectToken(NEWLINE, &err2) {
		return errors.New(err2)
	}
	return nil
}

func (m *ManifestParser) parseEdge() error {
	var outs []EvalString
	for {
		ev, err := m.lexer.readEvalString(true)
		if err != nil {
			return err
		}
		if len(ev.Parsed) == 0 {
			break
		}
		outs = append(outs, ev)
	}

	// Add all implicit outs, counting how many as we go.
	implicitOuts := 0
	if m.lexer.PeekToken(PIPE) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			outs = append(outs, ev)
			implicitOuts++
		}
	}

	if len(outs) == 0 {
		return m.lexer.Error("expected path")
	}

	err2 := ""
	if !m.expectToken(COLON, &err2) {
		return errors.New(err2)
	}

	ruleName := m.lexer.readIdent()
	if ruleName == "" {
		return m.lexer.Error("expected build command name")
	}

	rule := m.env.LookupRule(ruleName)
	if rule == nil {
		return m.lexer.Error("unknown build rule '" + ruleName + "'")
	}

	var ins []EvalString
	for {
		// XXX should we require one path here?
		ev, err := m.lexer.readEvalString(true)
		if err != nil {
			return err
		}
		if len(ev.Parsed) == 0 {
			break
		}
		ins = append(ins, ev)
	}

	// Add all implicit deps, counting how many as we go.
	implicit := 0
	if m.lexer.PeekToken(PIPE) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			ins = append(ins, ev)
			implicit++
		}
	}

	// Add all order-only deps, counting how many as we go.
	orderOnly := 0
	if m.lexer.PeekToken(PIPE2) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			ins = append(ins, ev)
			orderOnly++
		}
	}

	// Add all validations, counting how many as we go.
	var validations []EvalString
	if m.lexer.PeekToken(PIPEAT) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			validations = append(validations, ev)
		}
	}

	if !m.expectToken(NEWLINE, &err2) {
		return errors.New(err2)
	}

	// Bindings on edges are rare, so allocate per-edge envs only when needed.
	hasIndentToken := m.lexer.PeekToken(INDENT)
	env := m.env
	if hasIndentToken {
		env = NewBindingEnv(m.env)
	}
	for hasIndentToken {
		key, val, err := m.parseLet()
		if err != nil {
			return err
		}

		env.Bindings[key] = val.Evaluate(m.env)
		hasIndentToken = m.lexer.PeekToken(INDENT)
	}

	edge := m.state.addEdge(rule)
	edge.Env = env

	poolName := edge.GetBinding("pool")
	if poolName != "" {
		pool := m.state.Pools[poolName]
		if pool == nil {
			return m.lexer.Error("unknown pool name '" + poolName + "'")
		}
		edge.Pool = pool
	}

	edge.Outputs = make([]*Node, 0, len(outs))
	for i := range outs {
		path := outs[i].Evaluate(env)
		if len(path) == 0 {
			return m.lexer.Error("empty path")
		}
		path, slashBits := CanonicalizePathBits(path)
		if !m.state.addOut(edge, path, slashBits) {
			if m.options.ErrOnDupeEdge {
				return m.lexer.Error("multiple rules generate " + path)
			}
			if !m.options.Quiet {
				warningf("multiple rules generate %s. builds involving this target will not be correct; continuing anyway", path)
			}
			if len(outs)-i <= implicitOuts {
				implicitOuts--
			}
		}
	}
	if len(edge.Outputs) == 0 {
		// All outputs of the edge are already created by other edges. Don't add
		// this edge.  Do this check before input nodes are connected to the edge.
		m.state.Edges = m.state.Edges[:len(m.state.Edges)-1]
		return nil
	}
	edge.ImplicitOuts = int32(implicitOuts)

	edge.Inputs = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(env)
		if len(path) == 0 {
			return m.lexer.Error("empty path")
		}
		path, slashBits := CanonicalizePathBits(path)
		m.state.addIn(edge, path, slashBits)
	}
	edge.ImplicitDeps = int32(implicit)
	edge.OrderOnlyDeps = int32(orderOnly)

	edge.Validations = make([]*Node, 0, len(validations))
	for _, v := range validations {
		path := v.Evaluate(env)
		if path == "" {
			return m.lexer.Error("empty path")
		}
		path, slashBits := CanonicalizePathBits(path)
		m.state.addValidation(edge, path, slashBits)
	}

	if !m.options.ErrOnPhonyCycle && edge.maybePhonycycleDiagnostic() {
		// CMake 2.8.12.x and 3.0.x incorrectly write phony build statements
		// that reference themselves.  Ninja used to tolerate these in the
		// build graph but that has since been fixed.  Filter them out to
		// support users of those old CMake versions.
		out := edge.Outputs[0]
		for i, n := range edge.Inputs {
			if n == out {
				copy(edge.Inputs[i:], edge.Inputs[i+1:])
				edge.Inputs = edge.Inputs[:len(edge.Inputs)-1]
				if !m.options.Quiet {
					warningf("phony target '%s' names itself as an input; ignoring [-w phonycycle=warn]", out.Path)
				}
				break
			}
		}
	}

	// Lookup, validate, and save any dyndep binding.  It will be used later
	// to load generated dependency information dynamically, but it must
	// be one of our manifest-specified inputs.
	dyndep := edge.GetUnescapedDyndep()
	if len(dyndep) != 0 {
		n := m.state.GetNode(CanonicalizePathBits(dyndep))
		n.DyndepPending = true
		edge.Dyndep = n
		found := false
		for _, x := range edge.Inputs {
			if x == n {
				found = true
				break
			}
		}
		if !found {
			return m.lexer.Error("dyndep '" + dyndep + "' is not an input")
		}
	}
	return nil
}

// Parse either a 'subninja' or 'include' line.
func (m *ManifestParser) parseFileInclude(newScope bool) error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	path := eval.Evaluate(m.env)

	// TODO(maruel): Parse the file in a separate goroutine. The challenge is to
	// not create lock contention.
	subparser := NewManifestParser(m.state, m.fileReader, m.options)
	if newScope {
		subparser.env = NewBindingEnv(m.env)
	} else {
		subparser.env = m.env
	}

	err2 := ""
	if !subparser.Load(path, &err2, &m.lexer) {
		return errors.New(err2)
	}

	if !m.expectToken(NEWLINE, &err2) {
		return errors.New(err2)
	}
	return nil
}
