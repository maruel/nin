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

// manifestParserConcurrent parses .ninja files.
type manifestParserConcurrent struct {
	manifestParserRoutine
	manifestParserState
}

// manifestParserRoutine is the state of the parsing goroutine.
type manifestParserRoutine struct {
	// Mutable.
	lexer lexer
	manifestParserContext
}

type manifestParserContext struct {
	env               *BindingEnv
	doneParsing       barrier
	subninjas         chan error
	subninjasEnqueued int32
}

type barrier struct {
	want chan struct{}
}

func (b *barrier) unblock() {
	// TODO(maruel): Memory leak?
	go func() {
		for range b.want {
		}
	}()
}

func (b *barrier) wait() {
	b.want <- struct{}{}
}

// manifestParserState is the state of the processing goroutine.
type manifestParserState struct {
	// Mutable.
	state *State

	// Immutable.
	options ParseManifestOpts
	fr      FileReader
	// These need to be saved since this goroutine doesn't have access to lexer
	// to reconstruct errors.
	filename string
	input    []byte
}

// parse parses a file, given its contents as a string.
//
// Primary                 Processing             Subninja Nth
//  Where                  goroutine              goroutine
//  ParseManifest()         process()              processSubninjaReal()
//  was called                │                         │
//    │                       │                         │
//    ▼                       ▼                         ▼
// ───────────────────────────────────────────────────────────────
//    │
// Initiate
//    │
//    ───────────────────────►│
//    │                       │
//    ┌────────────┐          │
//    │parses and  │          │
//    │send actions│   actions│
//    └──────────────────────►│
//                            ├─────────────────────┐
//                            │EvalString.Evaluate()│
//                            │update m.state       │
//                            └─────────────────────┘
//    │                       │
//    ├────────────┐          │
//    │parses and  │          │
//    │send actions│   actions│
//    └──────────────────────►│
//                            ├─────────────────────┐
//                            │EvalString.Evaluate()│
//                            │update m.state       │
//                            └─────────────────────┘
//    │                       │
//    ├──────────────┐        │
//    │parseInclude()│ actions│
//    └──────────────────────►│
//                            ├─────────────────────┐
//                            │processInclude()     │
//                            │EvalString.Evaluate()│
//                            │run new parser here  │
//                            └─────────────────────┘
//    │                       │
//    ├───────────────┐       │
//    │parseSubninja()│actions│
//    └──────────────────────►│
//                            ├─────────────────────┐
//                            │processSubninja()    │
//                            │EvalString.Evaluate()│
//                            │Start goroutine to   │
//                            │read file            │
//                            └────────────────────────►│
//                                                      │
//    │                                           ┌─────┴───────┐
//    ├───────┐                                   │read subninja│
//    │Done   │                                   └─────────────┘
//    │parsing│     doneParsing                         │
//    └────────────────────────────────────────────────►│
//                                                 ┌────┴───────┐
//                            │                    │parses and  │
//                            │actions             │send actions│
//                            │◄────────────────────────────────┘
//                            │
//                            ├─────────────────────┐
//                            │EvalString.Evaluate()│
//                            │update m.state       │
//                            └─────────────────────┘
//    │                       │
//    │                       │
//    │processResult          │
//    │◄───────────────────────
//    │
//    ▼
//   Done
//
// "parses and send actions" is one of: parsePool(), parseEdge(), parseRule(),
// parseDefault() or parseIdent().
//
// Warning: a subninja can have includes and subninjas itself. They need to
// block the subninja parsing goroutine and until main action routine unblocks
// the barrier.
func (m *manifestParserConcurrent) parseMain(filename string, input []byte) error {
	defer metricRecord(".ninja parse")()

	// We want some amount of buffering to help with the parsing getting ahead of
	// the processing.
	actions := make(chan interface{}, 1024)
	processResult := make(chan error)

	// For error().
	m.manifestParserState.filename = filename
	m.manifestParserState.input = input

	// Processing goroutine
	go func() {
		// This goroutine doesn't have access to m.lexer. It will enqueue
		// goroutines to read subninjas in parallel, hence the wg to ensure we wait
		// for them when we terminate early.
		processResult <- m.manifestParserState.process(actions)
	}()

	// First, block on the parser to be done.
	err := m.parse(filename, input, actions)
	close(actions)

	// Between actions results and parsing results, prioritize parsing results.
	if err2 := <-processResult; err == nil {
		err = err2
	}
	return err
}

// parse runs the lexer parsing loop.
//
// It parses but does not process. It enqueues actions into actions that the
// main goroutine shall execute.
func (m *manifestParserRoutine) parse(filename string, input []byte, actions chan<- interface{}) error {
	// The parsing done by the lexer can be done in a separate thread. What is
	// important is that m.state is never touched concurrently.
	m.subninjas = make(chan error)

	// Parse the main manifest (build.ninja).
	if err := m.lexer.Start(filename, input); err != nil {
		return err
	}
	// subninja files are read as soon as the statement is parsed but they are
	// only processed once the current file is done. This enables lower latency
	// overall.
	var err error
	var d interface{}
loop:
	for err == nil {
		switch token := m.lexer.ReadToken(); token {
		case POOL:
			if d, err = m.parsePool(); err != nil {
				break loop
			}
			actions <- d
		case BUILD:
			if d, err = m.parseEdge(); err != nil {
				break loop
			}
			actions <- d
		case RULE:
			if d, err = m.parseRule(); err != nil {
				break loop
			}
			actions <- d
		case DEFAULT:
			if d, err = m.parseDefault(); err != nil {
				break loop
			}
			actions <- d
		case IDENT:
			if d, err = m.parseIdent(); err != nil {
				break loop
			}
			actions <- d
		case INCLUDE:
			if d, err = m.parseInclude(); err != nil {
				break loop
			}
			actions <- d
		case SUBNINJA:
			if d, err = m.parseSubninja(); err != nil {
				break loop
			}
			actions <- d
		case ERROR:
			err = m.lexer.Error(m.lexer.DescribeLastError())
		case TEOF:
			break loop
		case NEWLINE:
		default:
			err = m.lexer.Error("unexpected " + token.String())
		}
	}

	// Send an event of ourselves to notify actions that it is ready to process
	// subninjas.
	actions <- m

	m.doneParsing.wait()

	// Once all the actions created by m.parse() are processed, m.env is
	// completely immutable and can be accessed concurrently.
	//
	// We can safely process the subninja files since m.env becomes static.
	//
	// This is done here because parse() doesn't have access to m.subninjas.
	for i := int32(0); i < m.subninjasEnqueued; i++ {
		if err2 := <-m.subninjas; err == nil {
			err = err2
		}
	}
	return err
}

func (m *manifestParserState) process(actions chan interface{}) error {
	var err error
	for a := range actions {
		if err != nil {
			// Ignore following actions if we got an error but we still need to
			// continue emptying the channel.
			switch d := a.(type) {
			case *manifestParserRoutine:
				// Unblock all the subninjas for this specific routine. It is important
				// because recursive subninjas have to wait for their parent subninja to
				// be processed.
				d.doneParsing.unblock()
			default:
			}
			continue
		}
		switch d := a.(type) {
		case dataPool:
			err = m.processPool(d)
		case dataEdge:
			err = m.processEdge(d)
		case dataRule:
			err = m.processRule(d)
		case dataDefault:
			err = m.processDefault(d)
		case dataIdent:
			err = m.processIdent(d)
		case dataInclude:
			// Loads the included file immediately.
			// An include can be in a subninja, so the right BindingsEnv must be
			// loaded.
			err = m.processInclude(d)
		case dataSubninja:
			// Enqueues the file to read into m.subninjas.
			// A subninja can be from another subninja, so the right BindingsEnv must
			// be loaded.
			err = m.processSubninja(d, actions)
		case *manifestParserRoutine:
			// Unblock all the subninjas for this specific routine. It is important
			// because recursive subninjas have to wait for their parent subninja to
			// be processed.
			d.doneParsing.unblock()
		}
	}
	return err
}

// parsePool parses a "pool" statement.
func (m *manifestParserRoutine) parsePool() (dataPool, error) {
	d := dataPool{env: m.env}
	d.name = m.lexer.readIdent()
	if d.name == "" {
		return d, m.lexer.Error("expected pool name")
	}
	if err := m.expectToken(NEWLINE); err != nil {
		return d, err
	}
	d.ls = m.lexer.lexerState
	for m.lexer.PeekToken(INDENT) {
		key := ""
		var err error
		key, d.eval, err = m.parseLet()
		if err != nil {
			return d, err
		}
		if key != "depth" {
			// TODO(maruel): Use %q for real quoting.
			return d, m.lexer.Error(fmt.Sprintf("unexpected variable '%s'", key))
		}
		d.dls = m.lexer.lexerState
	}
	if len(d.eval.Parsed) == 0 {
		return d, m.lexer.Error("expected 'depth =' line")
	}
	return d, nil
}

// processPool updates m.state with a parsed pool statement.
func (m *manifestParserState) processPool(d dataPool) error {
	if m.state.Pools[d.name] != nil {
		// TODO(maruel): Use %q for real quoting.
		return m.error(fmt.Sprintf("duplicate pool '%s'", d.name), d.ls)
	}
	// TODO(maruel): Do we want to use ParseInt() here? Aka support hex.
	depth, err := strconv.Atoi(d.eval.Evaluate(d.env))
	if depth < 0 || err != nil {
		return m.error("invalid pool depth", d.dls)
	}
	m.state.Pools[d.name] = NewPool(d.name, depth)
	return nil
}

// parseRule parses a "rule" statement.
func (m *manifestParserRoutine) parseRule() (dataRule, error) {
	d := dataRule{env: m.env}
	name := m.lexer.readIdent()
	if name == "" {
		return d, m.lexer.Error("expected rule name")
	}
	if err := m.expectToken(NEWLINE); err != nil {
		return d, err
	}
	d.ls = m.lexer
	d.rule = NewRule(name)
	for m.lexer.PeekToken(INDENT) {
		key, value, err := m.parseLet()
		if err != nil {
			return d, err
		}

		if !IsReservedBinding(key) {
			// Die on other keyvals for now; revisit if we want to add a
			// scope here.
			// TODO(maruel): Use %q for real quoting.
			return d, m.lexer.Error(fmt.Sprintf("unexpected variable '%s'", key))
		}
		d.rule.Bindings[key] = &value
	}

	b1, ok1 := d.rule.Bindings["rspfile"]
	b2, ok2 := d.rule.Bindings["rspfile_content"]
	if ok1 != ok2 || (ok1 && (len(b1.Parsed) == 0) != (len(b2.Parsed) == 0)) {
		return d, m.lexer.Error("rspfile and rspfile_content need to be both specified")
	}

	b, ok := d.rule.Bindings["command"]
	if !ok || len(b.Parsed) == 0 {
		return d, m.lexer.Error("expected 'command =' line")
	}
	return d, nil
}

// processRule updates m.state with a parsed rule statement.
func (m *manifestParserState) processRule(d dataRule) error {
	if d.env.Rules[d.rule.Name] != nil {
		// TODO(maruel): Use %q for real quoting.
		return d.ls.Error(fmt.Sprintf("duplicate rule '%s'", d.rule.Name))
	}
	d.env.Rules[d.rule.Name] = d.rule
	return nil
}

// parseDefault parses a "default" statement.
func (m *manifestParserRoutine) parseDefault() (dataDefault, error) {
	d := dataDefault{env: m.env}
	eval, err := m.lexer.readEvalString(true)
	if err != nil {
		return d, err
	}
	if len(eval.Parsed) == 0 {
		return d, m.lexer.Error("expected target name")
	}

	d.evals = []*parsedEval{{eval, m.lexer}}
	for {
		if eval, err = m.lexer.readEvalString(true); err != nil {
			return d, err
		}
		if len(eval.Parsed) == 0 {
			break
		}
		d.evals = append(d.evals, &parsedEval{eval, m.lexer})
	}
	return d, m.expectToken(NEWLINE)
}

// processDefault updates m.state with a parsed default statement.
func (m *manifestParserState) processDefault(d dataDefault) error {
	for i := 0; i < len(d.evals); i++ {
		path := d.evals[i].eval.Evaluate(d.env)
		if len(path) == 0 {
			return d.evals[i].ls.Error("empty path")
		}
		if err := m.state.addDefault(CanonicalizePath(path)); err != nil {
			return d.evals[i].ls.Error(err.Error())
		}
	}
	return nil
}

// parseIdent parses a generic statement as a fallback.
func (m *manifestParserRoutine) parseIdent() (dataIdent, error) {
	d := dataIdent{env: m.env}
	m.lexer.UnreadToken()
	var err error
	d.name, d.eval, err = m.parseLet()
	return d, err
}

// processIdent updates m.state with a parsed ident statement.
func (m *manifestParserState) processIdent(d dataIdent) error {
	value := d.eval.Evaluate(d.env)
	// Check ninjaRequiredVersion immediately so we can exit
	// before encountering any syntactic surprises.
	if d.name == "ninja_required_version" {
		if err := checkNinjaVersion(value); err != nil {
			return err
		}
	}
	d.env.Bindings[d.name] = value
	return nil
}

// parseEdge parses a "build" statement that results into an edge, which
// defines inputs and outputs.
func (m *manifestParserRoutine) parseEdge() (dataEdge, error) {
	d := dataEdge{env: m.env}
	for {
		ev, err := m.lexer.readEvalString(true)
		if err != nil {
			return d, err
		}
		if len(ev.Parsed) == 0 {
			break
		}
		d.outs = append(d.outs, ev)
	}

	// Add all implicit outs, counting how many as we go.
	if m.lexer.PeekToken(PIPE) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return d, err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			d.outs = append(d.outs, ev)
			d.implicitOuts++
		}
	}

	if len(d.outs) == 0 {
		return d, m.lexer.Error("expected path")
	}

	if err := m.expectToken(COLON); err != nil {
		return d, err
	}

	d.ruleName = m.lexer.readIdent()
	if d.ruleName == "" {
		return d, m.lexer.Error("expected build command name")
	}

	// Save the lexer for unknown rule check later.
	d.lsRule = m.lexer

	for {
		// XXX should we require one path here?
		ev, err := m.lexer.readEvalString(true)
		if err != nil {
			return d, err
		}
		if len(ev.Parsed) == 0 {
			break
		}
		d.ins = append(d.ins, ev)
	}

	// Add all implicit deps, counting how many as we go.
	if m.lexer.PeekToken(PIPE) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return d, err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			d.ins = append(d.ins, ev)
			d.implicit++
		}
	}

	// Add all order-only deps, counting how many as we go.
	if m.lexer.PeekToken(PIPE2) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return d, err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			d.ins = append(d.ins, ev)
			d.orderOnly++
		}
	}

	// Add all validations, counting how many as we go.
	if m.lexer.PeekToken(PIPEAT) {
		for {
			ev, err := m.lexer.readEvalString(true)
			if err != nil {
				return d, err
			}
			if len(ev.Parsed) == 0 {
				break
			}
			d.validations = append(d.validations, ev)
		}
	}

	if err := m.expectToken(NEWLINE); err != nil {
		return d, err
	}

	// Bindings on edges are rare, so allocate per-edge envs only when needed.
	d.hadIndentToken = m.lexer.PeekToken(INDENT)
	// Accumulate the bindings for now, will process them later.
	for h := d.hadIndentToken; h; h = m.lexer.PeekToken(INDENT) {
		key, val, err := m.parseLet()
		if err != nil {
			return d, err
		}
		d.bindings = append(d.bindings, &keyEval{key, val})
	}
	d.lsEnd = m.lexer.lexerState
	return d, nil
}

// processEdge updates m.state with a parsed edge statement.
func (m *manifestParserState) processEdge(d dataEdge) error {
	rule := d.env.LookupRule(d.ruleName)
	if rule == nil {
		// TODO(maruel): Use %q for real quoting.
		return d.lsRule.Error(fmt.Sprintf("unknown build rule '%s'", d.ruleName))
	}
	env := d.env
	if d.hadIndentToken {
		env = NewBindingEnv(d.env)
	}
	for _, i := range d.bindings {
		env.Bindings[i.key] = i.eval.Evaluate(d.env)
	}

	edge := m.state.addEdge(rule)
	edge.Env = env

	if poolName := edge.GetBinding("pool"); poolName != "" {
		pool := m.state.Pools[poolName]
		if pool == nil {
			// TODO(maruel): Use %q for real quoting.
			return d.lsEnd.error(fmt.Sprintf("unknown pool name '%s'", poolName), d.lsRule.filename, d.lsRule.input)
		}
		edge.Pool = pool
	}

	edge.Outputs = make([]*Node, 0, len(d.outs))
	for i, o := range d.outs {
		path := o.Evaluate(env)
		if len(path) == 0 {
			return d.lsEnd.error("empty path", d.lsRule.filename, d.lsRule.input)
		}
		path, slashBits := CanonicalizePathBits(path)
		if !m.state.addOut(edge, path, slashBits) {
			if m.options.ErrOnDupeEdge {
				return d.lsEnd.error("multiple rules generate "+path, d.lsRule.filename, d.lsRule.input)
			}
			if !m.options.Quiet {
				warningf("multiple rules generate %s. builds involving this target will not be correct; continuing anyway", path)
			}
			if len(d.outs)-i <= d.implicitOuts {
				d.implicitOuts--
			}
		}
	}
	if len(edge.Outputs) == 0 {
		// All outputs of the edge are already created by other edges. Don't add
		// this edge.  Do this check before input nodes are connected to the edge.
		m.state.Edges = m.state.Edges[:len(m.state.Edges)-1]
		return nil
	}
	edge.ImplicitOuts = int32(d.implicitOuts)

	edge.Inputs = make([]*Node, 0, len(d.ins))
	for _, i := range d.ins {
		path := i.Evaluate(env)
		if len(path) == 0 {
			return d.lsEnd.error("empty path", d.lsRule.filename, d.lsRule.input)
		}
		path, slashBits := CanonicalizePathBits(path)
		m.state.addIn(edge, path, slashBits)
	}
	edge.ImplicitDeps = int32(d.implicit)
	edge.OrderOnlyDeps = int32(d.orderOnly)

	edge.Validations = make([]*Node, 0, len(d.validations))
	for _, v := range d.validations {
		path := v.Evaluate(env)
		if path == "" {
			return d.lsEnd.error("empty path", d.lsRule.filename, d.lsRule.input)
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
			return d.lsEnd.error(fmt.Sprintf("dyndep '%s' is not an input", dyndep), d.lsRule.filename, d.lsRule.input)
		}
	}
	return nil
}

// parseInclude parses a "include" line.
func (m *manifestParserRoutine) parseInclude() (dataInclude, error) {
	d := dataInclude{env: m.env}
	var err error
	if d.eval, err = m.lexer.readEvalString(true); err != nil {
		return d, err
	}
	d.ls = m.lexer
	return d, m.expectToken(NEWLINE)
}

// processInclude updates m.state by parsing an included ninja file.
//
// This is a stop-the-world event.
func (m *manifestParserState) processInclude(d dataInclude) error {
	path := d.eval.Evaluate(d.env)
	input, err := m.fr.ReadFile(path)
	if err != nil {
		// Wrap it.
		// TODO(maruel): Use %q for real quoting.
		return d.ls.Error(fmt.Sprintf("loading '%s': %s", path, err))
	}

	// Synchronously parse the inner file. This is because the following lines
	// may require declarations from this file.
	//
	// Manually construct the object instead of using ParseManifest(), because
	// d.env may not equal to m.state.Bindings. This happens when the include
	// statement is inside a subninja.
	subparser := manifestParserConcurrent{
		manifestParserRoutine: manifestParserRoutine{
			manifestParserContext: manifestParserContext{
				env: d.env,
				doneParsing: barrier{
					want: make(chan struct{}),
				},
			},
		},
		manifestParserState: manifestParserState{
			fr:      m.fr,
			options: m.options,
			state:   m.state,
		},
	}
	// Recursively parse the input into the current state. This works because we
	// completely hang the primary process() goroutine.
	// Do not wrap error inside the included ninja.
	return subparser.parseMain(path, input)
}

// parseSubninja parses a "subninja" statement.
func (m *manifestParserRoutine) parseSubninja() (dataSubninja, error) {
	d := dataSubninja{context: &m.manifestParserContext}
	var err error
	if d.eval, err = m.lexer.readEvalString(true); err != nil {
		return d, err
	}
	d.ls = m.lexer
	return d, m.expectToken(NEWLINE)
}

// processSubninja is an action in the main thread to evaluate the subninja to
// parse and start up a separate goroutine to read it.
func (m *manifestParserState) processSubninja(d dataSubninja, actions chan<- interface{}) error {
	// We can finally resolve what file path it is. Start the read to process
	// later.
	filename := d.eval.Evaluate(d.context.env)
	// Start the goroutine to read it asynchronously. It will send an action back.
	// TODO(maruel): Use a workerpool, something around runtime.NumCPU() ?
	d.context.subninjasEnqueued++
	go m.processSubninjaReal(filename, d, actions)
	return nil
}

// processSubninjaReal is the goroutine that reads the subninja file in parallel
// to the main build.ninja to reduce overall latency, then parse the subninja
// once the parent's parser is done processing its actions.
//
// Contrary to the include, here we run a separate concurrent parsing loop. The
// state modification is still in the main loop.
func (m *manifestParserState) processSubninjaReal(filename string, d dataSubninja, actions chan<- interface{}) {
	input, err := m.fr.ReadFile(filename)
	if err != nil {
		// Wrap it.
		// TODO(maruel): Use %q for real quoting.
		err = d.ls.Error(fmt.Sprintf("loading '%s': %s", filename, err.Error()))
	}

	// We are NOT allowed to write to actions, because we are in a completely new
	// and separate goroutine.
	d.context.doneParsing.wait()

	// At this point, the parent's m.env is completely immutable and can be
	// accessed concurrently. We can now safely process the subninja
	// concurrently!

	if err == nil {
		subparser := manifestParserConcurrent{
			manifestParserRoutine: manifestParserRoutine{
				manifestParserContext: manifestParserContext{
					// Reset the binding fresh with a temporary one that will not affect the
					// root one.
					env: NewBindingEnv(d.context.env),
					doneParsing: barrier{
						want: make(chan struct{}),
					},
				},
			},
			manifestParserState: manifestParserState{
				fr:      m.fr,
				options: m.options,
				state:   m.state,
			},
		}
		// We must not use subparser.parseMain() here, since we want it to send the
		// actions to the main thread. So use parse() directly. This is fine
		// because we do not want to handle grand-children subninjas here.
		err = subparser.parse(filename, input, actions)
	}
	d.context.subninjas <- err
}

func (m *manifestParserRoutine) parseLet() (string, EvalString, error) {
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
func (m *manifestParserRoutine) expectToken(expected Token) error {
	if token := m.lexer.ReadToken(); token != expected {
		msg := "expected " + expected.String() + ", got " + token.String() + expected.errorHint()
		return m.lexer.Error(msg)
	}
	return nil
}

func (m *manifestParserState) error(msg string, ls lexerState) error {
	return ls.error(msg, m.filename, m.input)
}

type dataPool struct {
	env     *BindingEnv
	name    string
	eval    EvalString
	ls, dls lexerState
}

type dataEdge struct {
	env                    *BindingEnv
	ruleName               string
	bindings               []*keyEval
	lsRule                 lexer
	lsEnd                  lexerState // Kind of a hack.
	ins, outs, validations []EvalString
	implicit, orderOnly    int
	implicitOuts           int
	hadIndentToken         bool
}

type dataRule struct {
	env  *BindingEnv
	rule *Rule
	ls   lexer
}

type dataDefault struct {
	env   *BindingEnv
	evals []*parsedEval
}

type dataIdent struct {
	env  *BindingEnv
	name string
	eval EvalString
}

type dataInclude struct {
	env  *BindingEnv
	eval EvalString
	ls   lexer
}

type dataSubninja struct {
	eval    EvalString
	ls      lexer
	context *manifestParserContext
}

type parsedEval struct {
	eval EvalString
	ls   lexer
}

type keyEval struct {
	key  string
	eval EvalString
}
