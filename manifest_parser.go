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

//go:build nobuild

package ginja


enum DupeEdgeAction {
  kDupeEdgeActionWarn,
  kDupeEdgeActionError,
}

enum PhonyCycleAction {
  kPhonyCycleActionWarn,
  kPhonyCycleActionError,
}

type ManifestParserOptions struct {
  ManifestParserOptions()
      : dupe_edge_action_(kDupeEdgeActionWarn),
        phony_cycle_action_(kPhonyCycleActionWarn) {}
  dupe_edge_action_ DupeEdgeAction
  phony_cycle_action_ PhonyCycleAction
}

// Parses .ninja files.
type ManifestParser struct {
  ManifestParser(State* state, FileReader* file_reader, ManifestParserOptions options = ManifestParserOptions())

  // Parse a text string of input.  Used by tests.

  env_ *BindingEnv
  options_ ManifestParserOptions
  quiet_ bool
}
// Parse a text string of input.  Used by tests.
func (m *ManifestParser) ParseTest(input string, err *string) bool {
  quiet_ = true
  return Parse("input", input, err)
}


ManifestParser::ManifestParser(State* state, FileReader* file_reader, ManifestParserOptions options)
    : Parser(state, file_reader),
      options_(options), quiet_(false) {
  env_ = &state.bindings_
}

// Parse a file, given its contents as a string.
func (m *ManifestParser) Parse(filename string, input string, err *string) bool {
  lexer_.Start(filename, input)

  for ; ;  {
    token := lexer_.ReadToken()
    switch (token) {
    case Lexer::POOL:
      if !ParsePool(err) {
        return false
      }
      break
    case Lexer::BUILD:
      if !ParseEdge(err) {
        return false
      }
      break
    case Lexer::RULE:
      if !ParseRule(err) {
        return false
      }
      break
    case Lexer::DEFAULT:
      if !ParseDefault(err) {
        return false
      }
      break
    case Lexer::IDENT: {
      lexer_.UnreadToken()
      name := ""
      var let_value EvalString
      if !ParseLet(&name, &let_value, err) {
        return false
      }
      value := let_value.Evaluate(env_)
      // Check ninja_required_version immediately so we can exit
      // before encountering any syntactic surprises.
      if name == "ninja_required_version" {
        CheckNinjaVersion(value)
      }
      env_.AddBinding(name, value)
      break
    }
    case Lexer::INCLUDE:
      if !ParseFileInclude(false, err) {
        return false
      }
      break
    case Lexer::SUBNINJA:
      if !ParseFileInclude(true, err) {
        return false
      }
      break
    case Lexer::ERROR: {
      return lexer_.Error(lexer_.DescribeLastError(), err)
    }
    case Lexer::TEOF:
      return true
    case Lexer::NEWLINE:
      break
    default:
      return lexer_.Error(string("unexpected ") + Lexer::TokenName(token), err)
    }
  }
  return false  // not reached
}

// Parse various statement types.
func (m *ManifestParser) ParsePool(err *string) bool {
  name := ""
  if !lexer_.ReadIdent(&name) {
    return lexer_.Error("expected pool name", err)
  }

  if !ExpectToken(Lexer::NEWLINE, err) {
    return false
  }

  if state_.LookupPool(name) != nil {
    return lexer_.Error("duplicate pool '" + name + "'", err)
  }

  int depth = -1

  while lexer_.PeekToken(Lexer::INDENT) {
    key := ""
    var value EvalString
    if !ParseLet(&key, &value, err) {
      return false
    }

    if key == "depth" {
      depth_string := value.Evaluate(env_)
      depth = atol(depth_string)
      if depth < 0 {
        return lexer_.Error("invalid pool depth", err)
      }
    } else {
      return lexer_.Error("unexpected variable '" + key + "'", err)
    }
  }

  if depth < 0 {
    return lexer_.Error("expected 'depth =' line", err)
  }

  state_.AddPool(new Pool(name, depth))
  return true
}

func (m *ManifestParser) ParseRule(err *string) bool {
  name := ""
  if !lexer_.ReadIdent(&name) {
    return lexer_.Error("expected rule name", err)
  }

  if !ExpectToken(Lexer::NEWLINE, err) {
    return false
  }

  if env_.LookupRuleCurrentScope(name) != nil {
    return lexer_.Error("duplicate rule '" + name + "'", err)
  }

  rule := new Rule(name)  // XXX scoped_ptr

  while lexer_.PeekToken(Lexer::INDENT) {
    key := ""
    var value EvalString
    if !ParseLet(&key, &value, err) {
      return false
    }

    if Rule::IsReservedBinding(key) {
      rule.AddBinding(key, value)
    } else {
      // Die on other keyvals for now; revisit if we want to add a
      // scope here.
      return lexer_.Error("unexpected variable '" + key + "'", err)
    }
  }

  if rule.bindings_["rspfile"].empty() != rule.bindings_["rspfile_content"].empty() {
    return lexer_.Error("rspfile and rspfile_content need to be " "both specified", err)
  }

  if rule.bindings_["command"].empty() {
    return lexer_.Error("expected 'command =' line", err)
  }

  env_.AddRule(rule)
  return true
}

func (m *ManifestParser) ParseLet(key *string, value *EvalString, err *string) bool {
  if !lexer_.ReadIdent(key) {
    return lexer_.Error("expected variable name", err)
  }
  if !ExpectToken(Lexer::EQUALS, err) {
    return false
  }
  if !lexer_.ReadVarValue(value, err) {
    return false
  }
  return true
}

func (m *ManifestParser) ParseDefault(err *string) bool {
  var eval EvalString
  if !lexer_.ReadPath(&eval, err) {
    return false
  }
  if len(eval) == 0 {
    return lexer_.Error("expected target name", err)
  }

  do {
    path := eval.Evaluate(env_)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    var slash_bits uint64  // Unused because this only does lookup.
    CanonicalizePath(&path, &slash_bits)
    default_err := ""
    if !state_.AddDefault(path, &default_err) {
      return lexer_.Error(default_err, err)
    }

    eval.Clear()
    if !lexer_.ReadPath(&eval, err) {
      return false
    }
  } while (!eval.empty())

  return ExpectToken(Lexer::NEWLINE, err)
}

func (m *ManifestParser) ParseEdge(err *string) bool {
  vector<EvalString> ins, outs

  {
    var out EvalString
    if !lexer_.ReadPath(&out, err) {
      return false
    }
    while len(out) != 0 {
      outs.push_back(out)

      out.Clear()
      if !lexer_.ReadPath(&out, err) {
        return false
      }
    }
  }

  // Add all implicit outs, counting how many as we go.
  implicit_outs := 0
  if lexer_.PeekToken(Lexer::PIPE) {
    for ; ;  {
      var out EvalString
      if !lexer_.ReadPath(&out, err) {
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
    return lexer_.Error("expected path", err)
  }

  if !ExpectToken(Lexer::COLON, err) {
    return false
  }

  rule_name := ""
  if !lexer_.ReadIdent(&rule_name) {
    return lexer_.Error("expected build command name", err)
  }

  rule := env_.LookupRule(rule_name)
  if rule == nil {
    return lexer_.Error("unknown build rule '" + rule_name + "'", err)
  }

  for ; ;  {
    // XXX should we require one path here?
    var in EvalString
    if !lexer_.ReadPath(&in, err) {
      return false
    }
    if len(in) == 0 {
      break
    }
    ins.push_back(in)
  }

  // Add all implicit deps, counting how many as we go.
  implicit := 0
  if lexer_.PeekToken(Lexer::PIPE) {
    for ; ;  {
      var in EvalString
      if !lexer_.ReadPath(&in, err) {
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
  if lexer_.PeekToken(Lexer::PIPE2) {
    for ; ;  {
      var in EvalString
      if !lexer_.ReadPath(&in, err) {
        return false
      }
      if len(in) == 0 {
        break
      }
      ins.push_back(in)
      order_only++
    }
  }

  if !ExpectToken(Lexer::NEWLINE, err) {
    return false
  }

  // Bindings on edges are rare, so allocate per-edge envs only when needed.
  has_indent_token := lexer_.PeekToken(Lexer::INDENT)
  BindingEnv* env = has_indent_token ? new BindingEnv(env_) : env_
  while has_indent_token {
    key := ""
    var val EvalString
    if !ParseLet(&key, &val, err) {
      return false
    }

    env.AddBinding(key, val.Evaluate(env_))
    has_indent_token = lexer_.PeekToken(Lexer::INDENT)
  }

  edge := state_.AddEdge(rule)
  edge.env_ = env

  string pool_name = edge.GetBinding("pool")
  if !pool_name.empty() {
    pool := state_.LookupPool(pool_name)
    if pool == nil {
      return lexer_.Error("unknown pool name '" + pool_name + "'", err)
    }
    edge.pool_ = pool
  }

  edge.outputs_.reserve(outs.size())
  for size_t i = 0, e = outs.size(); i != e; i++ {
    path := outs[i].Evaluate(env)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    var slash_bits uint64
    CanonicalizePath(&path, &slash_bits)
    if !state_.AddOut(edge, path, slash_bits) {
      if options_.dupe_edge_action_ == kDupeEdgeActionError {
        lexer_.Error("multiple rules generate " + path, err)
        return false
      } else {
        if !quiet_ {
          Warning( "multiple rules generate %s. builds involving this target will " "not be correct; continuing anyway", path)
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
    state_.edges_.pop_back()
    var edge delete
    return true
  }
  edge.implicit_outs_ = implicit_outs

  edge.inputs_.reserve(ins.size())
  for i := ins.begin(); i != ins.end(); i++ {
    path := i.Evaluate(env)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    var slash_bits uint64
    CanonicalizePath(&path, &slash_bits)
    state_.AddIn(edge, path, slash_bits)
  }
  edge.implicit_deps_ = implicit
  edge.order_only_deps_ = order_only

  if options_.phony_cycle_action_ == kPhonyCycleActionWarn && edge.maybe_phonycycle_diagnostic() {
    // CMake 2.8.12.x and 3.0.x incorrectly write phony build statements
    // that reference themselves.  Ninja used to tolerate these in the
    // build graph but that has since been fixed.  Filter them out to
    // support users of those old CMake versions.
    out := edge.outputs_[0]
    vector<Node*>::iterator new_end =
        remove(edge.inputs_.begin(), edge.inputs_.end(), out)
    if new_end != edge.inputs_.end() {
      edge.inputs_.erase(new_end, edge.inputs_.end())
      if !quiet_ {
        Warning("phony target '%s' names itself as an input; " "ignoring [-w phonycycle=warn]", out.path())
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
    edge.dyndep_ = state_.GetNode(dyndep, slash_bits)
    edge.dyndep_.set_dyndep_pending(true)
    vector<Node*>::iterator dgi =
      find(edge.inputs_.begin(), edge.inputs_.end(), edge.dyndep_)
    if dgi == edge.inputs_.end() {
      return lexer_.Error("dyndep '" + dyndep + "' is not an input", err)
    }
  }

  return true
}

// Parse either a 'subninja' or 'include' line.
func (m *ManifestParser) ParseFileInclude(new_scope bool, err *string) bool {
  var eval EvalString
  if !lexer_.ReadPath(&eval, err) {
    return false
  }
  path := eval.Evaluate(env_)

  ManifestParser subparser(state_, file_reader_, options_)
  if new_scope {
    subparser.env_ = new BindingEnv(env_)
  } else {
    subparser.env_ = env_
  }

  if !subparser.Load(path, err, &lexer_) {
    return false
  }

  if !ExpectToken(Lexer::NEWLINE, err) {
    return false
  }

  return true
}

