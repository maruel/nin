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
	"sort"
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

// ManifestParser parses .ninja files.
type ManifestParser struct {
	// Immutable
	fileReader FileReader
	options    ManifestParserOptions

	// Mutable.
	lexer             lexer
	state             *State
	env               *BindingEnv
	subninjas         chan subninja
	subninjasEnqueued int32
}

// NewManifestParser returns an initialized ManifestParser.
func NewManifestParser(state *State, fileReader FileReader, options ManifestParserOptions) *ManifestParser {
	return &ManifestParser{
		fileReader: fileReader,
		options:    options,
		state:      state,
		env:        state.Bindings,
		subninjas:  make(chan subninja),
	}
}

// Parse a file, given its contents as a string.
func (m *ManifestParser) Parse(filename string, input []byte) error {
	defer metricRecord(".ninja parse")()

	// The object is reused when parsing subninjas.
	m.subninjasEnqueued = 0

	m.lexer.Start(filename, input)

	// subninja files are read as soon as the statement is parsed but they are
	// only processed once the current file is done. This enables lower latency
	// overall.
	var err error
loop:
	for err == nil {
		token := m.lexer.ReadToken()
		switch token {
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
func (m *ManifestParser) parsePool() error {
	name := m.lexer.readIdent()
	if name == "" {
		return m.lexer.Error("expected pool name")
	}

	if err := m.expectToken(NEWLINE); err != nil {
		return err
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

	if err := m.expectToken(NEWLINE); err != nil {
		return err
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

	return m.expectToken(NEWLINE)
}

func (m *ManifestParser) parseIdent() error {
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

	if err := m.expectToken(COLON); err != nil {
		return err
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

// parseInclude parses a 'include' line.
func (m *ManifestParser) parseInclude() error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	ls := m.lexer.lexerState
	if err := m.expectToken(NEWLINE); err != nil {
		return err
	}

	// Process state.
	path := eval.Evaluate(m.env)
	input, err := m.fileReader.ReadFile(path)
	if err != nil {
		// Wrap it.
		return m.error(fmt.Sprintf("loading '%s': %s", path, err), ls)
	}

	subparser := NewManifestParser(m.state, m.fileReader, m.options)
	subparser.env = m.env
	if err = subparser.Parse(path, input); err != nil {
		// Do not wrap error inside the included ninja.
		return err
	}
	return nil
}

// subninja is a struct used to manage parallel reading of subninja files.
type subninja struct {
	path     string
	contents []byte
	err      error
	ls       lexerState // lexer state when the subninja statement was parsed.
	index    int32
}

// parseSubninja parses a 'subninja' line and start a goroutine that will read
// the file and send the content to the channel, but not process it.
func (m *ManifestParser) parseSubninja() error {
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return err
	}
	path := eval.Evaluate(m.env)
	if err := m.expectToken(NEWLINE); err != nil {
		return err
	}

	// Success, start the goroutine to read it asynchronously.
	go readSubninjaAsync(m.fileReader, m.subninjasEnqueued, path, m.subninjas, m.lexer.lexerState)
	m.subninjasEnqueued++
	return nil
}

// readSubninjaAsync is the goroutine that reads the subninja file in parallel
// to the main build.ninja to reduce overall latency.
func readSubninjaAsync(fileReader FileReader, id int32, n string, ch chan<- subninja, ls lexerState) {
	c, err := fileReader.ReadFile(n)
	if err != nil {
		ch <- subninja{
			index: id,
			path:  n,
			err:   err,
			ls:    ls,
		}
	}
	ch <- subninja{
		index:    id,
		path:     n,
		contents: c,
		ls:       ls,
	}
}

// processSubninjaQueue empties the queue of subninja files to process.
func (m *ManifestParser) processSubninjaQueue() error {
	if false {
		// Ordered flow.

		// This is limited by the slowest read but has the advantage of
		// guaranteeing order. If we find out we need ordered processing, we can
		// execute them within the loop as we find the right ID.
		results := make([]subninja, m.subninjasEnqueued)
		for i := int32(0); i < m.subninjasEnqueued; i++ {
			results[i] = <-m.subninjas
		}
		// At this point, all the goroutines are released.
		sort.Slice(results, func(i, j int) bool {
			return results[i].index < results[j].index
		})
		for _, s := range results {
			if s.err != nil {
				// Wrap it.
				return m.error("loading '"+s.path+"': "+s.err.Error(), s.ls)
			}
		}
		subparser := NewManifestParser(m.state, m.fileReader, m.options)
		for _, s := range results {
			subparser.env = NewBindingEnv(m.env)
			if err := subparser.Parse(s.path, s.contents); err != nil {
				// Do not wrap error inside the subninja.
				return err
			}
		}
		return nil
	}

	// Out of order flow. This is the faster but may be incompatible?
	var err error
	subparser := NewManifestParser(m.state, m.fileReader, m.options)
	for i := int32(0); i < m.subninjasEnqueued; i++ {
		s := <-m.subninjas
		if err != nil {
			continue
		}
		if s.err != nil {
			// Wrap it.
			err = m.error(fmt.Sprintf("loading '%s': %s", s.path, s.err.Error()), s.ls)
			continue
		}
		// Reset the binding fresh.
		subparser.env = NewBindingEnv(m.env)
		// Do not wrap error inside the subninja.
		err = subparser.Parse(s.path, s.contents)
	}
	return err
}

func (m *ManifestParser) parseLet() (string, EvalString, error) {
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
func (m *ManifestParser) expectToken(expected Token) error {
	if token := m.lexer.ReadToken(); token != expected {
		msg := "expected " + expected.String() + ", got " + token.String() + expected.errorHint()
		return m.lexer.Error(msg)
	}
	return nil
}

func (m *ManifestParser) error(msg string, ls lexerState) error {
	return ls.error(msg, m.lexer.filename, m.lexer.input)
}
