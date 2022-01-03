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

type DupeEdgeAction bool

const (
	DupeEdgeActionWarn  DupeEdgeAction = false
	DupeEdgeActionError DupeEdgeAction = true
)

type PhonyCycleAction bool

const (
	PhonyCycleActionWarn  PhonyCycleAction = false
	PhonyCycleActionError PhonyCycleAction = true
)

type ManifestParserOptions struct {
	dupeEdgeAction_   DupeEdgeAction
	phonyCycleAction_ PhonyCycleAction
}

// Parses .ninja files.
type ManifestParser struct {
	Parser
	env_     *BindingEnv
	options_ ManifestParserOptions
	quiet_   bool
}

// Parse a text string of input.  Used by tests.
func (m *ManifestParser) ParseTest(input string, err *string) bool {
	m.quiet_ = true
	return m.Parse("input", input+"\x00", err)
}

func NewManifestParser(state *State, fileReader FileReader, options ManifestParserOptions) *ManifestParser {
	m := &ManifestParser{
		options_: options,
		env_:     state.bindings_,
	}
	m.Parser = NewParser(state, fileReader, m)
	return m
}

// Parse a file, given its contents as a string.
func (m *ManifestParser) Parse(filename string, input string, err *string) bool {
	m.lexer_.Start(filename, input)

	for {
		token := m.lexer_.ReadToken()
		switch token {
		case POOL:
			if !m.ParsePool(err) {
				return false
			}
		case BUILD:
			if !m.ParseEdge(err) {
				return false
			}
		case RULE:
			if !m.ParseRule(err) {
				return false
			}
		case DEFAULT:
			if !m.ParseDefault(err) {
				return false
			}
		case IDENT:
			{
				m.lexer_.UnreadToken()
				name := ""
				var letValue EvalString
				if !m.ParseLet(&name, &letValue, err) {
					return false
				}
				value := letValue.Evaluate(m.env_)
				// Check ninjaRequiredVersion immediately so we can exit
				// before encountering any syntactic surprises.
				if name == "ninja_required_version" {
					CheckNinjaVersion(value)
				}
				m.env_.AddBinding(name, value)
			}
		case INCLUDE:
			if !m.ParseFileInclude(false, err) {
				return false
			}
		case SUBNINJA:
			if !m.ParseFileInclude(true, err) {
				return false
			}
		case ERROR:
			return m.lexer_.Error(m.lexer_.DescribeLastError(), err)
		case TEOF:
			return true
		case NEWLINE:
		default:
			return m.lexer_.Error(string("unexpected ")+TokenName(token), err)
		}
	}
}

// Parse various statement types.
func (m *ManifestParser) ParsePool(err *string) bool {
	name := ""
	if !m.lexer_.ReadIdent(&name) {
		return m.lexer_.Error("expected pool name", err)
	}

	if !m.ExpectToken(NEWLINE, err) {
		return false
	}

	if m.state_.LookupPool(name) != nil {
		return m.lexer_.Error("duplicate pool '"+name+"'", err)
	}

	depth := -1

	for m.lexer_.PeekToken(INDENT) {
		key := ""
		var value EvalString
		if !m.ParseLet(&key, &value, err) {
			return false
		}

		if key == "depth" {
			depthString := value.Evaluate(m.env_)
			var err2 error
			depth, err2 = strconv.Atoi(depthString)
			if depth < 0 || err2 != nil {
				return m.lexer_.Error("invalid pool depth", err)
			}
		} else {
			return m.lexer_.Error("unexpected variable '"+key+"'", err)
		}
	}

	if depth < 0 {
		return m.lexer_.Error("expected 'depth =' line", err)
	}

	m.state_.AddPool(NewPool(name, depth))
	return true
}

func (m *ManifestParser) ParseRule(err *string) bool {
	name := ""
	if !m.lexer_.ReadIdent(&name) {
		return m.lexer_.Error("expected rule name", err)
	}

	if !m.ExpectToken(NEWLINE, err) {
		return false
	}

	if m.env_.LookupRuleCurrentScope(name) != nil {
		return m.lexer_.Error("duplicate rule '"+name+"'", err)
	}

	rule := NewRule(name)

	for m.lexer_.PeekToken(INDENT) {
		key := ""
		var value EvalString
		if !m.ParseLet(&key, &value, err) {
			return false
		}

		if IsReservedBinding(key) {
			rule.AddBinding(key, &value)
		} else {
			// Die on other keyvals for now; revisit if we want to add a
			// scope here.
			return m.lexer_.Error("unexpected variable '"+key+"'", err)
		}
	}

	b1, ok1 := rule.bindings_["rspfile"]
	b2, ok2 := rule.bindings_["rspfile_content"]
	if ok1 != ok2 || (ok1 && (len(b1.Parsed) == 0) != (len(b2.Parsed) == 0)) {
		return m.lexer_.Error("rspfile and rspfile_content need to be both specified", err)
	}

	b, ok := rule.bindings_["command"]
	if !ok || len(b.Parsed) == 0 {
		return m.lexer_.Error("expected 'command =' line", err)
	}
	m.env_.AddRule(rule)
	return true
}

func (m *ManifestParser) ParseLet(key *string, value *EvalString, err *string) bool {
	if !m.lexer_.ReadIdent(key) {
		return m.lexer_.Error("expected variable name", err)
	}
	if !m.ExpectToken(EQUALS, err) {
		return false
	}
	if !m.lexer_.ReadVarValue(value, err) {
		return false
	}
	return true
}

func (m *ManifestParser) ParseDefault(err *string) bool {
	var eval EvalString
	if !m.lexer_.ReadPath(&eval, err) {
		return false
	}
	if len(eval.Parsed) == 0 {
		return m.lexer_.Error("expected target name", err)
	}

	for {
		path := eval.Evaluate(m.env_)
		if len(path) == 0 {
			return m.lexer_.Error("empty path", err)
		}
		defaultErr := ""
		if !m.state_.AddDefault(CanonicalizePath(path), &defaultErr) {
			return m.lexer_.Error(defaultErr, err)
		}

		eval.Parsed = nil
		if !m.lexer_.ReadPath(&eval, err) {
			return false
		}
		if len(eval.Parsed) == 0 {
			break
		}
	}

	return m.ExpectToken(NEWLINE, err)
}

func (m *ManifestParser) ParseEdge(err *string) bool {
	var ins, outs, validations []EvalString

	{
		var out EvalString
		if !m.lexer_.ReadPath(&out, err) {
			return false
		}
		for len(out.Parsed) != 0 {
			outs = append(outs, out)

			out.Parsed = nil
			if !m.lexer_.ReadPath(&out, err) {
				return false
			}
		}
	}

	// Add all implicit outs, counting how many as we go.
	implicitOuts := 0
	if m.lexer_.PeekToken(PIPE) {
		for {
			var out EvalString
			if !m.lexer_.ReadPath(&out, err) {
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
		return m.lexer_.Error("expected path", err)
	}

	if !m.ExpectToken(COLON, err) {
		return false
	}

	ruleName := ""
	if !m.lexer_.ReadIdent(&ruleName) {
		return m.lexer_.Error("expected build command name", err)
	}

	rule := m.env_.LookupRule(ruleName)
	if rule == nil {
		return m.lexer_.Error("unknown build rule '"+ruleName+"'", err)
	}

	for {
		// XXX should we require one path here?
		var in EvalString
		if !m.lexer_.ReadPath(&in, err) {
			return false
		}
		if len(in.Parsed) == 0 {
			break
		}
		ins = append(ins, in)
	}

	// Add all implicit deps, counting how many as we go.
	implicit := 0
	if m.lexer_.PeekToken(PIPE) {
		for {
			var in EvalString
			if !m.lexer_.ReadPath(&in, err) {
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
	if m.lexer_.PeekToken(PIPE2) {
		for {
			var in EvalString
			if !m.lexer_.ReadPath(&in, err) {
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
	if m.lexer_.PeekToken(PIPEAT) {
		for {
			var validation EvalString
			if !m.lexer_.ReadPath(&validation, err) {
				return false
			}
			if len(validation.Parsed) == 0 {
				break
			}
			validations = append(validations, validation)
		}
	}

	if !m.ExpectToken(NEWLINE, err) {
		return false
	}

	// Bindings on edges are rare, so allocate per-edge envs only when needed.
	hasIndentToken := m.lexer_.PeekToken(INDENT)
	env := m.env_
	if hasIndentToken {
		env = NewBindingEnv(m.env_)
	}
	for hasIndentToken {
		key := ""
		var val EvalString
		if !m.ParseLet(&key, &val, err) {
			return false
		}

		env.AddBinding(key, val.Evaluate(m.env_))
		hasIndentToken = m.lexer_.PeekToken(INDENT)
	}

	edge := m.state_.AddEdge(rule)
	edge.Env = env

	poolName := edge.GetBinding("pool")
	if poolName != "" {
		pool := m.state_.LookupPool(poolName)
		if pool == nil {
			return m.lexer_.Error("unknown pool name '"+poolName+"'", err)
		}
		edge.Pool = pool
	}

	// TODO: edge.outputs_.reserve(outs.size())
	for i := range outs {
		path := outs[i].Evaluate(env)
		if len(path) == 0 {
			return m.lexer_.Error("empty path", err)
		}
		path, slashBits := CanonicalizePathBits(path)
		if !m.state_.AddOut(edge, path, slashBits) {
			if m.options_.dupeEdgeAction_ == DupeEdgeActionError {
				m.lexer_.Error("multiple rules generate "+path, err)
				return false
			}
			if !m.quiet_ {
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
		m.state_.edges_ = m.state_.edges_[:len(m.state_.edges_)-1]
		return true
	}
	edge.ImplicitOuts = int32(implicitOuts)

	// TODO: edge.inputs_.reserve(ins.size())
	for _, i := range ins {
		path := i.Evaluate(env)
		if len(path) == 0 {
			return m.lexer_.Error("empty path", err)
		}
		path, slashBits := CanonicalizePathBits(path)
		m.state_.AddIn(edge, path, slashBits)
	}
	edge.ImplicitDeps = int32(implicit)
	edge.OrderOnlyDeps = int32(orderOnly)

	//edge.validations_.reserve(validations.size());
	for _, v := range validations {
		path := v.Evaluate(env)
		if path == "" {
			return m.lexer_.Error("empty path", err)
		}
		path, slashBits := CanonicalizePathBits(path)
		m.state_.AddValidation(edge, path, slashBits)
	}

	if m.options_.phonyCycleAction_ == PhonyCycleActionWarn && edge.maybePhonycycleDiagnostic() {
		// CMake 2.8.12.x and 3.0.x incorrectly write phony build statements
		// that reference themselves.  Ninja used to tolerate these in the
		// build graph but that has since been fixed.  Filter them out to
		// support users of those old CMake versions.
		out := edge.Outputs[0]
		for i, n := range edge.Inputs {
			if n == out {
				copy(edge.Inputs[i:], edge.Inputs[i+1:])
				edge.Inputs = edge.Inputs[:len(edge.Inputs)-1]
				if !m.quiet_ {
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
		n := m.state_.GetNode(CanonicalizePathBits(dyndep))
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
			return m.lexer_.Error("dyndep '"+dyndep+"' is not an input", err)
		}
	}
	return true
}

// Parse either a 'subninja' or 'include' line.
func (m *ManifestParser) ParseFileInclude(newScope bool, err *string) bool {
	var eval EvalString
	if !m.lexer_.ReadPath(&eval, err) {
		return false
	}
	path := eval.Evaluate(m.env_)

	// TODO(maruel): Parse the file in a separate goroutine. The challenge is to
	// not create lock contention.
	subparser := NewManifestParser(m.state_, m.fileReader_, m.options_)
	if newScope {
		subparser.env_ = NewBindingEnv(m.env_)
	} else {
		subparser.env_ = m.env_
	}

	if !subparser.Load(path, err, &m.lexer_) {
		return false
	}

	if !m.ExpectToken(NEWLINE, err) {
		return false
	}
	return true
}
