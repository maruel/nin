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

package ginja

type DupeEdgeAction int

const (
	kDupeEdgeActionWarn DupeEdgeAction = iota
	kDupeEdgeActionError
)

type PhonyCycleAction int

const (
	kPhonyCycleActionWarn PhonyCycleAction = iota
	kPhonyCycleActionError
)

type ManifestParserOptions struct {
	dupe_edge_action_   DupeEdgeAction
	phony_cycle_action_ PhonyCycleAction
}

func NewManifestParserOptions() ManifestParserOptions {
	return ManifestParserOptions{
		dupe_edge_action_:   kDupeEdgeActionWarn,
		phony_cycle_action_: kPhonyCycleActionWarn,
	}
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
	return m.Parse("input", input, err)
}

func NewManifestParser(state *State, file_reader FileReader, options ManifestParserOptions) ManifestParser {
	return ManifestParser{
		/* TODO
		Parser:   NewParser(state, file_reader),
		options_: options,
		env_:     &state.bindings_,
		*/
	}
}

// Parse a file, given its contents as a string.
func (m *ManifestParser) Parse(filename string, input string, err *string) bool {
	/*
	   m.lexer_.Start(filename, input)

	   for ; ;  {
	     token := m.lexer_.ReadToken()
	     switch (token) {
	     case POOL:
	       if !ParsePool(err) {
	         return false
	       }
	       break
	     case BUILD:
	       if !ParseEdge(err) {
	         return false
	       }
	       break
	     case RULE:
	       if !ParseRule(err) {
	         return false
	       }
	       break
	     case DEFAULT:
	       if !ParseDefault(err) {
	         return false
	       }
	       break
	     case IDENT: {
	       m.lexer_.UnreadToken()
	       name := ""
	       var let_value EvalString
	       if !ParseLet(&name, &let_value, err) {
	         return false
	       }
	       value := let_value.Evaluate(m.env_)
	       // Check ninja_required_version immediately so we can exit
	       // before encountering any syntactic surprises.
	       if name == "ninja_required_version" {
	         CheckNinjaVersion(value)
	       }
	       m.env_.AddBinding(name, value)
	       break
	     }
	     case INCLUDE:
	       if !ParseFileInclude(false, err) {
	         return false
	       }
	       break
	     case SUBNINJA:
	       if !ParseFileInclude(true, err) {
	         return false
	       }
	       break
	     case ERROR: {
	       return m.lexer_.Error(m.lexer_.DescribeLastError(), err)
	     }
	     case TEOF:
	       return true
	     case NEWLINE:
	       break
	     default:
	       return m.lexer_.Error(string("unexpected ") + TokenName(token), err)
	     }
	   }
	*/
	return false // not reached
}

// Parse various statement types.
func (m *ManifestParser) ParsePool(err *string) bool {
	/*
	     name := ""
	     if !m.lexer_.ReadIdent(&name) {
	       return m.lexer_.Error("expected pool name", err)
	     }

	     if !m.ExpectToken(NEWLINE, err) {
	       return false
	     }

	     if m.state_.LookupPool(name) != nil {
	       return m.lexer_.Error("duplicate pool '" + name + "'", err)
	     }

	     int depth = -1

	     for m.lexer_.PeekToken(INDENT) {
	       key := ""
	       var value EvalString
	       if !ParseLet(&key, &value, err) {
	         return false
	       }

	       if key == "depth" {
	         depth_string := value.Evaluate(m.env_)
	         depth = atol(depth_string)
	         if depth < 0 {
	           return m.lexer_.Error("invalid pool depth", err)
	         }
	       } else {
	         return m.lexer_.Error("unexpected variable '" + key + "'", err)
	       }
	     }

	     if depth < 0 {
	       return m.lexer_.Error("expected 'depth =' line", err)
	     }

	     m.state_.AddPool(new Pool(name, depth))
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
	       return m.lexer_.Error("duplicate rule '" + name + "'", err)
	     }

	     rule := new Rule(name)  // XXX scoped_ptr

	     for m.lexer_.PeekToken(INDENT) {
	       key := ""
	       var value EvalString
	       if !ParseLet(&key, &value, err) {
	         return false
	       }

	       if IsReservedBinding(key) {
	         rule.AddBinding(key, value)
	       } else {
	         // Die on other keyvals for now; revisit if we want to add a
	         // scope here.
	         return m.lexer_.Error("unexpected variable '" + key + "'", err)
	       }
	     }

	     if rule.bindings_["rspfile"].empty() != rule.bindings_["rspfile_content"].empty() {
	       return m.lexer_.Error("rspfile and rspfile_content need to be both specified", err)
	     }

	     if rule.bindings_["command"].empty() {
	       return m.lexer_.Error("expected 'command =' line", err)
	     }

	     m.env_.AddRule(rule)
	*/
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
	if eval.empty() {
		return m.lexer_.Error("expected target name", err)
	}

	for {
		path := eval.Evaluate(m.env_)
		if len(path) == 0 {
			return m.lexer_.Error("empty path", err)
		}
		var slash_bits uint64 // Unused because this only does lookup.
		path = CanonicalizePath(path, &slash_bits)
		default_err := ""
		if !m.state_.AddDefault(path, &default_err) {
			return m.lexer_.Error(default_err, err)
		}

		eval.Clear()
		if !m.lexer_.ReadPath(&eval, err) {
			return false
		}
		if eval.empty() {
			break
		}
	}

	return m.ExpectToken(NEWLINE, err)
}

func (m *ManifestParser) ParseEdge(err *string) bool {
	/*
	  vector<EvalString> ins, outs

	  {
	    var out EvalString
	    if !m.lexer_.ReadPath(&out, err) {
	      return false
	    }
	    for len(out) != 0 {
	      outs.push_back(out)

	      out.Clear()
	      if !m.lexer_.ReadPath(&out, err) {
	        return false
	      }
	    }
	  }

	  // Add all implicit outs, counting how many as we go.
	  implicit_outs := 0
	  if m.lexer_.PeekToken(PIPE) {
	    for ; ;  {
	      var out EvalString
	      if !m.lexer_.ReadPath(&out, err) {
	        return false
	      }
	      if len(out) == 0 {
	        break
	      }
	      outs.push_back(out)
	      implicit_outs++
	    }
	  }

	  if len(outs) == 0 {
	    return m.lexer_.Error("expected path", err)
	  }

	  if !m.ExpectToken(COLON, err) {
	    return false
	  }

	  rule_name := ""
	  if !m.lexer_.ReadIdent(&rule_name) {
	    return m.lexer_.Error("expected build command name", err)
	  }

	  rule := m.env_.LookupRule(rule_name)
	  if rule == nil {
	    return m.lexer_.Error("unknown build rule '" + rule_name + "'", err)
	  }

	  for ; ;  {
	    // XXX should we require one path here?
	    var in EvalString
	    if !m.lexer_.ReadPath(&in, err) {
	      return false
	    }
	    if len(in) == 0 {
	      break
	    }
	    ins.push_back(in)
	  }

	  // Add all implicit deps, counting how many as we go.
	  implicit := 0
	  if m.lexer_.PeekToken(PIPE) {
	    for ; ;  {
	      var in EvalString
	      if !m.lexer_.ReadPath(&in, err) {
	        return false
	      }
	      if len(in) == 0 {
	        break
	      }
	      ins.push_back(in)
	      implicit++
	    }
	  }

	  // Add all order-only deps, counting how many as we go.
	  order_only := 0
	  if m.lexer_.PeekToken(PIPE2) {
	    for ; ;  {
	      var in EvalString
	      if !m.lexer_.ReadPath(&in, err) {
	        return false
	      }
	      if len(in) == 0 {
	        break
	      }
	      ins.push_back(in)
	      order_only++
	    }
	  }

	  if !m.ExpectToken(NEWLINE, err) {
	    return false
	  }

	  // Bindings on edges are rare, so allocate per-edge envs only when needed.
	  has_indent_token := m.lexer_.PeekToken(INDENT)
	  BindingEnv* env = has_indent_token ? new BindingEnv(m.env_) : m.env_
	  for has_indent_token {
	    key := ""
	    var val EvalString
	    if !ParseLet(&key, &val, err) {
	      return false
	    }

	    env.AddBinding(key, val.Evaluate(m.env_))
	    has_indent_token = m.lexer_.PeekToken(INDENT)
	  }

	  edge := m.state_.AddEdge(rule)
	  edge.env_ = env

	  string pool_name = edge.GetBinding("pool")
	  if !pool_name.empty() {
	    pool := m.state_.LookupPool(pool_name)
	    if pool == nil {
	      return m.lexer_.Error("unknown pool name '" + pool_name + "'", err)
	    }
	    edge.pool_ = pool
	  }

	  edge.outputs_.reserve(outs.size())
	  for size_t i = 0, e = outs.size(); i != e; i++ {
	    path := outs[i].Evaluate(env)
	    if len(path) == 0 {
	      return m.lexer_.Error("empty path", err)
	    }
	    var slash_bits uint64
	    CanonicalizePath(&path, &slash_bits)
	    if !m.state_.AddOut(edge, path, slash_bits) {
	      if m.options_.dupe_edge_action_ == kDupeEdgeActionError {
	        m.lexer_.Error("multiple rules generate " + path, err)
	        return false
	      } else {
	        if !m.quiet_ {
	          Warning( "multiple rules generate %s. builds involving this target will not be correct; continuing anyway", path)
	        }
	        if e - i <= static_cast<size_t>(implicit_outs) {
	          implicit_outs--
	        }
	      }
	    }
	  }
	  if edge.outputs_.empty() {
	    // All outputs of the edge are already created by other edges. Don't add
	    // this edge.  Do this check before input nodes are connected to the edge.
	    m.state_.edges_.pop_back()
	    var edge delete
	    return true
	  }
	  edge.implicit_outs_ = implicit_outs

	  edge.inputs_.reserve(ins.size())
	  for i := ins.begin(); i != ins.end(); i++ {
	    path := i.Evaluate(env)
	    if len(path) == 0 {
	      return m.lexer_.Error("empty path", err)
	    }
	    var slash_bits uint64
	    CanonicalizePath(&path, &slash_bits)
	    m.state_.AddIn(edge, path, slash_bits)
	  }
	  edge.implicit_deps_ = implicit
	  edge.order_only_deps_ = order_only

	  if m.options_.phony_cycle_action_ == kPhonyCycleActionWarn && edge.maybe_phonycycle_diagnostic() {
	    // CMake 2.8.12.x and 3.0.x incorrectly write phony build statements
	    // that reference themselves.  Ninja used to tolerate these in the
	    // build graph but that has since been fixed.  Filter them out to
	    // support users of those old CMake versions.
	    out := edge.outputs_[0]
	    vector<Node*>::iterator new_end = remove(edge.inputs_.begin(), edge.inputs_.end(), out)
	    if new_end != edge.inputs_.end() {
	      edge.inputs_.erase(new_end, edge.inputs_.end())
	      if !m.quiet_ {
	        Warning("phony target '%s' names itself as an input; ignoring [-w phonycycle=warn]", out.path())
	      }
	    }
	  }

	  // Lookup, validate, and save any dyndep binding.  It will be used later
	  // to load generated dependency information dynamically, but it must
	  // be one of our manifest-specified inputs.
	  dyndep := edge.GetUnescapedDyndep()
	  if len(dyndep) != 0 {
	    var slash_bits uint64
	    CanonicalizePath(&dyndep, &slash_bits)
	    edge.dyndep_ = m.state_.GetNode(dyndep, slash_bits)
	    edge.dyndep_.set_dyndep_pending(true)
	    vector<Node*>::iterator dgi =find(edge.inputs_.begin(), edge.inputs_.end(), edge.dyndep_)
	    if dgi == edge.inputs_.end() {
	      return m.lexer_.Error("dyndep '" + dyndep + "' is not an input", err)
	    }
	  }
	*/
	return true
}

// Parse either a 'subninja' or 'include' line.
func (m *ManifestParser) ParseFileInclude(new_scope bool, err *string) bool {
	/*
	  var eval EvalString
	  if !m.lexer_.ReadPath(&eval, err) {
	    return false
	  }
	  path := eval.Evaluate(m.env_)

	  ManifestParser subparser(m.state_, m.file_reader_, m.options_)
	  if new_scope {
	    subparser.env_ = new BindingEnv(m.env_)
	  } else {
	    subparser.env_ = m.env_
	  }

	  if !subparser.Load(path, err, &m.lexer_) {
	    return false
	  }

	  if !m.ExpectToken(NEWLINE, err) {
	    return false
	  }
	*/
	return true
}
