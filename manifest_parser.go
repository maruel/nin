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
	"fmt"
	"strconv"
)

// ParseManifestOpts are the options when parsing a build.ninja file.
type ParseManifestOpts struct {
	// ErrOnDupeEdge causes duplicate rules for one target to print an error,
	// otherwise warns.
	ErrOnDupeEdge bool
	// ErrOnPhonyCycle causes phony cycles to print an error, otherwise warns.
	ErrOnPhonyCycle bool
	// Quiet silences warnings.
	Quiet bool
	// SerialSubninja causes subninjas to be loaded in the most compatible way,
	// as they are found, similar to the C++ implementation of ninja.
	//
	// When false, subninjas are processed at the very at the end of the main
	// manifest file instead of directly at the statement. This reduces overall
	// latency because the file can be read concurrently. This causes subninja
	// files to be processed out of order.
	SerialSubninja bool
}

// manifestParser parses .ninja files.
type manifestParser struct {
	// Immutable
	fr      FileReader
	options ParseManifestOpts

	// Mutable.
	lexer             lexer
	state             *State
	env               *BindingEnv
	subninjas         chan subninja
	subninjasEnqueued int32
}

// ParseManifest parses a manifest file (i.e. build.ninja).
//
// The input must contain a trailing terminating zero byte.
func ParseManifest(state *State, fr FileReader, options ParseManifestOpts, filename string, input []byte) error {
	m := manifestParser{
		fr:      fr,
		options: options,
		state:   state,
		env:     state.Bindings,
	}
	return m.parse(filename, input)
}

// parse parses a file, given its contents as a string.
func (m *manifestParser) parse(filename string, input []byte) error {
	defer metricRecord(".ninja parse")()

	m.subninjas = make(chan subninja)

	m.lexer.Start(filename, input)

	// subninja files are read as soon as the statement is parsed but they are
	// only processed once the current file is done. This enables lower latency
	// overall.
	var err error
loop:
	for err == nil {
		switch token := m.lexer.ReadToken(); token {
		case POOL:
			err = m.parsePool()
		case BUILD:
			err = m.parseEdge()
		case RULE:
			err = m.parseRule()
		case DEFAULT:
			err = m.parseDefault()
		case IDENT:
			err = m.parseIdent()
		case INCLUDE:
			err = m.parseInclude()
		case SUBNINJA:
			err = m.parseSubninja()
		case ERROR:
			err = m.lexer.Error(m.lexer.DescribeLastError())
		case TEOF:
			break loop
		case NEWLINE:
		default:
			err = m.lexer.Error("unexpected " + token.String())
		}
	}

	// At this point, m.env is completely immutable and can be accessed
	// concurrently.

	// Did the loop complete because of an error?
	if err != nil {
		// Do not forget to unblock the goroutines.
		for i := int32(0); i < m.subninjasEnqueued; i++ {
			<-m.subninjas
		}
		return err
	}

	// Finish the processing by parsing the subninja files.
	return m.processSubninjaQueue()
}

// Parse various statement types.
func (m *manifestParser) parsePool() error {
	name := m.lexer.readIdent()
	if name == "" {
		return m.lexer.Error("expected pool name")
	}

	if err := m.expectToken(NEWLINE); err != nil {
		return err
	}

	if m.state.Pools[name] != nil {
		// TODO(maruel): Use %q for real quoting.
		return m.lexer.Error(fmt.Sprintf("duplicate pool '%s'", name))
	}

	depth := -1

	for m.lexer.PeekToken(INDENT) {
		key, value, err := m.parseLet()
		if err != nil {
			return err
		}
		if key != "depth" {
			// TODO(maruel): Use %q for real quoting.
			return m.lexer.Error(fmt.Sprintf("unexpected variable '%s'", key))
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

func (m *manifestParser) parseRule() error {
	name := m.lexer.readIdent()
	if name == "" {
		return m.lexer.Error("expected rule name")
	}

	if err := m.expectToken(NEWLINE); err != nil {
		return err
	}

	if m.env.Rules[name] != nil {
		// TODO(maruel): Use %q for real quoting.
		return m.lexer.Error(fmt.Sprintf("duplicate rule '%s'", name))
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
			// TODO(maruel): Use %q for real quoting.
			return m.lexer.Error(fmt.Sprintf("unexpected variable '%s'", key))
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

func (m *manifestParser) parseDefault() error {
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
		if err = m.state.addDefault(CanonicalizePath(path)); err != nil {
			return m.lexer.Error(err.Error())
		}

		eval, err = m.lexer.readEvalString(true)
		if err != nil {
			return err
		}
		if len(eval.Parsed) == 0 {
			break
		}
	}

	return m.expectToken(NEWLINE)
}

func (m *manifestParser) parseIdent() error {
	m.lexer.UnreadToken()
	name, letValue, err := m.parseLet()
	if err != nil {
		return err
	}
	value := letValue.Evaluate(m.env)
	// Check ninjaRequiredVersion immediately so we can exit
	// before encountering any syntactic surprises.
	if name == "ninja_required_version" {
		if err := checkNinjaVersion(value); err != nil {
			return err
		}
	}
	m.env.Bindings[name] = value
	return nil
}

func (m *manifestParser) parseEdge() error {
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

	if err := m.expectToken(COLON); err != nil {
		return err
	}

	ruleName := m.lexer.readIdent()
	if ruleName == "" {
		return m.lexer.Error("expected build command name")
	}

	rule := m.env.LookupRule(ruleName)
	if rule == nil {
		// TODO(maruel): Use %q for real quoting.
		return m.lexer.Error(fmt.Sprintf("unknown build rule '%s'", ruleName))
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

	if err := m.expectToken(NEWLINE); err != nil {
		return err
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
			// TODO(maruel): Use %q for real quoting.
			return m.lexer.Error(fmt.Sprintf("unknown pool name '%s'", poolName))
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
			// TODO(maruel): Use %q for real quoting.
			return m.lexer.Error(fmt.Sprintf("dyndep '%s' is not an input", dyndep))
		}
	}
	return nil
}

// parseInclude parses a 'include' line.
func (m *manifestParser) parseInclude() error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	ls := m.lexer.lexerState
	if err = m.expectToken(NEWLINE); err != nil {
		return err
	}

	// Process state.
	path := eval.Evaluate(m.env)
	input, err := m.fr.ReadFile(path)
	if err != nil {
		// Wrap it.
		return m.error(fmt.Sprintf("loading '%s': %s", path, err), ls)
	}

	// Manually construct the object instead of using ParseManifest(), because
	// m.env may not equal to m.state.Bindings. This happens when the include
	// statement is inside a subninja.
	subparser := manifestParser{
		fr:      m.fr,
		options: m.options,
		state:   m.state,
		env:     m.env,
	}
	// Recursively parse the input into the current state.
	if err = subparser.parse(path, input); err != nil {
		// Do not wrap error inside the included ninja.
		return err
	}
	return nil
}

// subninja is a struct used to manage parallel reading of subninja files.
type subninja struct {
	filename string
	input    []byte
	err      error
	ls       lexerState // lexer state when the subninja statement was parsed.
}

// parseSubninja parses a 'subninja' line and start a goroutine that will read
// the file and send the content to the channel, but not process it.
func (m *manifestParser) parseSubninja() error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	filename := eval.Evaluate(m.env)
	ls := m.lexer.lexerState
	if err = m.expectToken(NEWLINE); err != nil {
		return err
	}

	if !m.options.SerialSubninja {
		// Start the goroutine to read it asynchronously. It will be processed
		// after the main manifest.
		go readSubninjaAsync(m.fr, filename, m.subninjas, ls)
		m.subninjasEnqueued++
		return nil
	}

	// Process the subninja right away. This is the most compatible way.
	input, err := m.fr.ReadFile(filename)
	if err != nil {
		// Wrap it.
		return m.error(fmt.Sprintf("loading '%s': %s", filename, err.Error()), ls)
	}
	return m.processOneSubninja(filename, input)
}

// readSubninjaAsync is the goroutine that reads the subninja file in parallel
// to the main build.ninja to reduce overall latency.
func readSubninjaAsync(fr FileReader, filename string, ch chan<- subninja, ls lexerState) {
	input, err := fr.ReadFile(filename)
	ch <- subninja{
		filename: filename,
		input:    input,
		err:      err,
		ls:       ls,
	}
}

// processSubninjaQueue empties the queue of subninja files to process.
func (m *manifestParser) processSubninjaQueue() error {
	// Out of order flow. This is the faster but may be incompatible?
	var err error
	for i := int32(0); i < m.subninjasEnqueued; i++ {
		s := <-m.subninjas
		if err != nil {
			continue
		}
		if s.err != nil {
			// Wrap it.
			err = m.error(fmt.Sprintf("loading '%s': %s", s.filename, s.err.Error()), s.ls)
			continue
		}
		err = m.processOneSubninja(s.filename, s.input)
	}
	return err
}

func (m *manifestParser) processOneSubninja(filename string, input []byte) error {
	subparser := manifestParser{
		fr:      m.fr,
		options: m.options,
		state:   m.state,
		// Reset the binding fresh with a temporary one that will not affect the
		// root one.
		env: NewBindingEnv(m.env),
	}
	// Do not wrap error inside the subninja.
	return subparser.parse(filename, input)
}

func (m *manifestParser) parseLet() (string, EvalString, error) {
	eval := EvalString{}
	key := m.lexer.readIdent()
	if key == "" {
		return key, eval, m.lexer.Error("expected variable name")
	}
	var err error
	if err = m.expectToken(EQUALS); err == nil {
		eval, err = m.lexer.readEvalString(false)
	}
	return key, eval, err
}

// expectToken produces an error string if the next token is not expected.
//
// The error says "expected foo, got bar".
func (m *manifestParser) expectToken(expected Token) error {
	if token := m.lexer.ReadToken(); token != expected {
		msg := "expected " + expected.String() + ", got " + token.String() + expected.errorHint()
		return m.lexer.Error(msg)
	}
	return nil
}

func (m *manifestParser) error(msg string, ls lexerState) error {
	return ls.error(msg, m.lexer.filename, m.lexer.input)
}
