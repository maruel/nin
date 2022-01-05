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

import "strconv"

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
			if !m.parsePool(err) {
				return false
			}
		case BUILD:
			if !m.parseEdge(err) {
				return false
			}
		case RULE:
			if !m.parseRule(err) {
				return false
			}
		case DEFAULT:
			if !m.parseDefault(err) {
				return false
			}
		case IDENT:
			{
				m.lexer.UnreadToken()
				name := ""
				var letValue EvalString
				if !m.parseLet(&name, &letValue, err) {
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
			if !m.parseFileInclude(false, err) {
				return false
			}
		case SUBNINJA:
			if !m.parseFileInclude(true, err) {
				return false
			}
		case ERROR:
			return m.lexer.Error(m.lexer.DescribeLastError(), err)
		case TEOF:
			return true
		case NEWLINE:
		default:
			return m.lexer.Error("unexpected "+token.String(), err)
		}
	}
}

// Parse various statement types.
func (m *ManifestParser) parsePool(err *string) bool {
	name := ""
	if !m.lexer.ReadIdent(&name) {
		return m.lexer.Error("expected pool name", err)
	}

	if !m.expectToken(NEWLINE, err) {
		return false
	}

	if m.state.Pools[name] != nil {
		return m.lexer.Error("duplicate pool '"+name+"'", err)
	}

	depth := -1

	for m.lexer.PeekToken(INDENT) {
		key := ""
		var value EvalString
		if !m.parseLet(&key, &value, err) {
			return false
		}

		if key == "depth" {
			depthString := value.Evaluate(m.env)
			var err2 error
			depth, err2 = strconv.Atoi(depthString)
			if depth < 0 || err2 != nil {
				return m.lexer.Error("invalid pool depth", err)
			}
		} else {
			return m.lexer.Error("unexpected variable '"+key+"'", err)
		}
	}

	if depth < 0 {
		return m.lexer.Error("expected 'depth =' line", err)
	}

	m.state.Pools[name] = NewPool(name, depth)
	return true
}

func (m *ManifestParser) parseRule(err *string) bool {
	name := ""
	if !m.lexer.ReadIdent(&name) {
		return m.lexer.Error("expected rule name", err)
	}

	if !m.expectToken(NEWLINE, err) {
		return false
	}

	if m.env.Rules[name] != nil {
		return m.lexer.Error("duplicate rule '"+name+"'", err)
	}

	rule := NewRule(name)

	for m.lexer.PeekToken(INDENT) {
		key := ""
		var value EvalString
		if !m.parseLet(&key, &value, err) {
			return false
		}

		if IsReservedBinding(key) {
			rule.Bindings[key] = &value
		} else {
			// Die on other keyvals for now; revisit if we want to add a
			// scope here.
			return m.lexer.Error("unexpected variable '"+key+"'", err)
		}
	}

	b1, ok1 := rule.Bindings["rspfile"]
	b2, ok2 := rule.Bindings["rspfile_content"]
	if ok1 != ok2 || (ok1 && (len(b1.Parsed) == 0) != (len(b2.Parsed) == 0)) {
		return m.lexer.Error("rspfile and rspfile_content need to be both specified", err)
	}

	b, ok := rule.Bindings["command"]
	if !ok || len(b.Parsed) == 0 {
		return m.lexer.Error("expected 'command =' line", err)
	}
	m.env.Rules[rule.Name] = rule
	return true
}

func (m *ManifestParser) parseLet(key *string, value *EvalString, err *string) bool {
	if !m.lexer.ReadIdent(key) {
		return m.lexer.Error("expected variable name", err)
	}
	if !m.expectToken(EQUALS, err) {
		return false
	}
	if !m.lexer.ReadVarValue(value, err) {
		return false
	}
	return true
}

func (m *ManifestParser) parseDefault(err *string) bool {
	var eval EvalString
	if !m.lexer.ReadPath(&eval, err) {
		return false
	}
	if len(eval.Parsed) == 0 {
		return m.lexer.Error("expected target name", err)
	}

	for {
		path := eval.Evaluate(m.env)
		if len(path) == 0 {
			return m.lexer.Error("empty path", err)
		}
		defaultErr := ""
		if !m.state.addDefault(CanonicalizePath(path), &defaultErr) {
			return m.lexer.Error(defaultErr, err)
		}

		eval.Parsed = nil
		if !m.lexer.ReadPath(&eval, err) {
			return false
		}
		if len(eval.Parsed) == 0 {
			break
		}
	}

	return m.expectToken(NEWLINE, err)
}

func (m *ManifestParser) parseEdge(err *string) bool {
	var ins, outs, validations []EvalString

	{
		var out EvalString
		if !m.lexer.ReadPath(&out, err) {
			return false
		}
		for len(out.Parsed) != 0 {
			outs = append(outs, out)

			out.Parsed = nil
			if !m.lexer.ReadPath(&out, err) {
				return false
			}
		}
	}

	// Add all implicit outs, counting how many as we go.
	implicitOuts := 0
	if m.lexer.PeekToken(PIPE) {
		for {
			var out EvalString
			if !m.lexer.ReadPath(&out, err) {
				return false
			}
			if len(out.Parsed) == 0 {
				break
			}
			outs = append(outs, out)
			implicitOuts++
		}
	}

	if len(outs) == 0 {
		return m.lexer.Error("expected path", err)
	}

	if !m.expectToken(COLON, err) {
		return false
	}

	ruleName := ""
	if !m.lexer.ReadIdent(&ruleName) {
		return m.lexer.Error("expected build command name", err)
	}

	rule := m.env.LookupRule(ruleName)
	if rule == nil {
		return m.lexer.Error("unknown build rule '"+ruleName+"'", err)
	}

	for {
		// XXX should we require one path here?
		var in EvalString
		if !m.lexer.ReadPath(&in, err) {
			return false
		}
		if len(in.Parsed) == 0 {
			break
		}
		ins = append(ins, in)
	}

	// Add all implicit deps, counting how many as we go.
	implicit := 0
	if m.lexer.PeekToken(PIPE) {
		for {
			var in EvalString
			if !m.lexer.ReadPath(&in, err) {
				return false
			}
			if len(in.Parsed) == 0 {
				break
			}
			ins = append(ins, in)
			implicit++
		}
	}

	// Add all order-only deps, counting how many as we go.
	orderOnly := 0
	if m.lexer.PeekToken(PIPE2) {
		for {
			var in EvalString
			if !m.lexer.ReadPath(&in, err) {
				return false
			}
			if len(in.Parsed) == 0 {
				break
			}
			ins = append(ins, in)
			orderOnly++
		}
	}

	// Add all validations, counting how many as we go.
	if m.lexer.PeekToken(PIPEAT) {
		for {
			var validation EvalString
			if !m.lexer.ReadPath(&validation, err) {
				return false
			}
			if len(validation.Parsed) == 0 {
				break
			}
			validations = append(validations, validation)
		}
	}

	if !m.expectToken(NEWLINE, err) {
		return false
	}

	// Bindings on edges are rare, so allocate per-edge envs only when needed.
	hasIndentToken := m.lexer.PeekToken(INDENT)
	env := m.env
	if hasIndentToken {
		env = NewBindingEnv(m.env)
	}
	for hasIndentToken {
		key := ""
		var val EvalString
		if !m.parseLet(&key, &val, err) {
			return false
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
			return m.lexer.Error("unknown pool name '"+poolName+"'", err)
		}
		edge.Pool = pool
	}

	edge.Outputs = make([]*Node, 0, len(outs))
	for i := range outs {
		path := outs[i].Evaluate(env)
		if len(path) == 0 {
			return m.lexer.Error("empty path", err)
		}
		path, slashBits := CanonicalizePathBits(path)
		if !m.state.addOut(edge, path, slashBits) {
			if m.options.ErrOnDupeEdge {
				m.lexer.Error("multiple rules generate "+path, err)
				return false
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
		return true
	}
	edge.ImplicitOuts = int32(implicitOuts)

	edge.Inputs = make([]*Node, 0, len(ins))
	for _, i := range ins {
		path := i.Evaluate(env)
		if len(path) == 0 {
			return m.lexer.Error("empty path", err)
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
			return m.lexer.Error("empty path", err)
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
			return m.lexer.Error("dyndep '"+dyndep+"' is not an input", err)
		}
	}
	return true
}

// Parse either a 'subninja' or 'include' line.
func (m *ManifestParser) parseFileInclude(newScope bool, err *string) bool {
	var eval EvalString
	if !m.lexer.ReadPath(&eval, err) {
		return false
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

	if !subparser.Load(path, err, &m.lexer) {
		return false
	}

	if !m.expectToken(NEWLINE, err) {
		return false
	}
	return true
}
