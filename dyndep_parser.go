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

//go:build nobuild

package ginja


// Parses dyndep files.
type DyndepParser struct {

  // Parse a text string of input.  Used by tests.

  dyndep_file_ *DyndepFile
  env_ BindingEnv
}
  // Parse a text string of input.  Used by tests.
  func (d *DyndepParser) ParseTest(input string, err *string) bool {
    return Parse("input", input, err)
  }


DyndepParser::DyndepParser(State* state, FileReader* file_reader, DyndepFile* dyndep_file)
    : Parser(state, file_reader)
    , dyndep_file_(dyndep_file) {
}

// Parse a file, given its contents as a string.
func (d *DyndepParser) Parse(filename string, input string, err *string) bool {
  lexer_.Start(filename, input)

  // Require a supported ninja_dyndep_version value immediately so
  // we can exit before encountering any syntactic surprises.
  haveDyndepVersion := false

  for ; ;  {
    token := lexer_.ReadToken()
    switch (token) {
    case Lexer::BUILD: {
      if !haveDyndepVersion {
        return lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
      }
      if !ParseEdge(err) {
        return false
      }
      break
    }
    case Lexer::IDENT: {
      lexer_.UnreadToken()
      if haveDyndepVersion {
        return lexer_.Error(string("unexpected ") + Lexer::TokenName(token), err)
      }
      if !ParseDyndepVersion(err) {
        return false
      }
      haveDyndepVersion = true
      break
    }
    case Lexer::ERROR:
      return lexer_.Error(lexer_.DescribeLastError(), err)
    case Lexer::TEOF:
      if !haveDyndepVersion {
        return lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
      }
      return true
    case Lexer::NEWLINE:
      break
    default:
      return lexer_.Error(string("unexpected ") + Lexer::TokenName(token), err)
    }
  }
  return false  // not reached
}

func (d *DyndepParser) ParseDyndepVersion(err *string) bool {
  name := ""
  var let_value EvalString
  if !ParseLet(&name, &let_value, err) {
    return false
  }
  if name != "ninja_dyndep_version" {
    return lexer_.Error("expected 'ninja_dyndep_version = ...'", err)
  }
  version := let_value.Evaluate(&env_)
  int major, minor
  ParseVersion(version, &major, &minor)
  if major != 1 || minor != 0 {
    return lexer_.Error( string("unsupported 'ninja_dyndep_version = ") + version + "'", err)
    return false
  }
  return true
}

func (d *DyndepParser) ParseLet(key *string, value *EvalString, err *string) bool {
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

func (d *DyndepParser) ParseEdge(err *string) bool {
  // Parse one explicit output.  We expect it to already have an edge.
  // We will record its dynamically-discovered dependency information.
  dyndeps := nil
  {
    var out0 EvalString
    if !lexer_.ReadPath(&out0, err) {
      return false
    }
    if out0.empty() {
      return lexer_.Error("expected path", err)
    }

    path := out0.Evaluate(&env_)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    var slash_bits uint64
    CanonicalizePath(&path, &slash_bits)
    node := state_.LookupNode(path)
    if !node || !node.in_edge() {
      return lexer_.Error("no build statement exists for '" + path + "'", err)
    }
    edge := node.in_edge()
    pair<DyndepFile::iterator, bool> res =
      dyndep_file_.insert(DyndepFile::value_type(edge, Dyndeps()))
    if !res.second {
      return lexer_.Error("multiple statements for '" + path + "'", err)
    }
    dyndeps = &res.first.second
  }

  // Disallow explicit outputs.
  {
    var out EvalString
    if !lexer_.ReadPath(&out, err) {
      return false
    }
    if len(out) != 0 {
      return lexer_.Error("explicit outputs not supported", err)
    }
  }

  // Parse implicit outputs, if any.
  var outs []EvalString
  if lexer_.PeekToken(Lexer::PIPE) {
    for ; ;  {
      var out EvalString
      if !lexer_.ReadPath(&out, err) {
        return err
      }
      if len(out) == 0 {
        break
      }
      outs.push_back(out)
    }
  }

  if !ExpectToken(Lexer::COLON, err) {
    return false
  }

  rule_name := ""
  if !lexer_.ReadIdent(&rule_name) || rule_name != "dyndep" {
    return lexer_.Error("expected build command name 'dyndep'", err)
  }

  // Disallow explicit inputs.
  {
    var in EvalString
    if !lexer_.ReadPath(&in, err) {
      return false
    }
    if len(in) != 0 {
      return lexer_.Error("explicit inputs not supported", err)
    }
  }

  // Parse implicit inputs, if any.
  var ins []EvalString
  if lexer_.PeekToken(Lexer::PIPE) {
    for ; ;  {
      var in EvalString
      if !lexer_.ReadPath(&in, err) {
        return err
      }
      if len(in) == 0 {
        break
      }
      ins.push_back(in)
    }
  }

  // Disallow order-only inputs.
  if lexer_.PeekToken(Lexer::PIPE2) {
    return lexer_.Error("order-only inputs not supported", err)
  }

  if !ExpectToken(Lexer::NEWLINE, err) {
    return false
  }

  if lexer_.PeekToken(Lexer::INDENT) {
    key := ""
    var val EvalString
    if !ParseLet(&key, &val, err) {
      return false
    }
    if key != "restat" {
      return lexer_.Error("binding is not 'restat'", err)
    }
    value := val.Evaluate(&env_)
    dyndeps.restat_ = !value.empty()
  }

  dyndeps.implicit_inputs_.reserve(ins.size())
  for i := ins.begin(); i != ins.end(); i++ {
    path := i.Evaluate(&env_)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    var slash_bits uint64
    CanonicalizePath(&path, &slash_bits)
    n := state_.GetNode(path, slash_bits)
    dyndeps.implicit_inputs_.push_back(n)
  }

  dyndeps.implicit_outputs_.reserve(outs.size())
  for i := outs.begin(); i != outs.end(); i++ {
    path := i.Evaluate(&env_)
    if len(path) == 0 {
      return lexer_.Error("empty path", err)
    }
    path_err := ""
    var slash_bits uint64
    CanonicalizePath(&path, &slash_bits)
    n := state_.GetNode(path, slash_bits)
    dyndeps.implicit_outputs_.push_back(n)
  }

  return true
}

