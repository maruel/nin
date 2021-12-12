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

package ginga


// An interface for a scope for variable (e.g. "$foo") lookups.
struct Env {
  virtual ~Env() {}
}

// A tokenized string that contains variable references.
// Can be evaluated relative to an Env.
struct EvalString {

  void Clear() { parsed_.clear(); }
  bool empty() const { return parsed_.empty(); }

  enum TokenType { RAW, SPECIAL }
  typedef vector<pair<string, TokenType> > TokenList
  TokenList parsed_
}

// An invocable build command and associated metadata (description, etc.).
struct Rule {
  explicit Rule(string name) : name_(name) {}

  string name() const { return name_; }

  static bool IsReservedBinding(string var)

  const EvalString* GetBinding(string key) const

  // Allow the parsers to reach into this object and fill out its fields.
  friend struct ManifestParser

  string name_
  typedef map<string, EvalString> Bindings
  Bindings bindings_
}

// An Env which contains a mapping of variables to values
// as well as a pointer to a parent scope.
struct BindingEnv {
  BindingEnv() : parent_(nil) {}
  explicit BindingEnv(BindingEnv* parent) : parent_(parent) {}

  virtual ~BindingEnv() {}

  const Rule* LookupRule(string rule_name)
  const Rule* LookupRuleCurrentScope(string rule_name)
  const map<string, const Rule*>& GetRules() const

  map<string, string> bindings_
  map<string, const Rule*> rules_
  BindingEnv* parent_
}


func (b *BindingEnv) LookupVariable(var string) string {
  map<string, string>::iterator i = bindings_.find(var)
  if i != bindings_.end() {
    return i.second
  }
  if parent_ {
    return parent_.LookupVariable(var)
  }
  return ""
}

func (b *BindingEnv) AddBinding(key string, val string) {
  bindings_[key] = val
}

func (b *BindingEnv) AddRule(rule *const Rule) {
  assert(LookupRuleCurrentScope(rule.name()) == nil)
  rules_[rule.name()] = rule
}

const Rule* BindingEnv::LookupRuleCurrentScope(string rule_name) {
  map<string, const Rule*>::iterator i = rules_.find(rule_name)
  if i == rules_.end() {
    return nil
  }
  return i.second
}

const Rule* BindingEnv::LookupRule(string rule_name) {
  map<string, const Rule*>::iterator i = rules_.find(rule_name)
  if i != rules_.end() {
    return i.second
  }
  if parent_ {
    return parent_.LookupRule(rule_name)
  }
  return nil
}

func (r *Rule) AddBinding(key string, val *EvalString) {
  bindings_[key] = val
}

const EvalString* Rule::GetBinding(string key) {
  Bindings::const_iterator i = bindings_.find(key)
  if i == bindings_.end() {
    return nil
  }
  return &i.second
}

// static
func (r *Rule) IsReservedBinding(var string) bool {
  return var == "command" ||
      var == "depfile" ||
      var == "dyndep" ||
      var == "description" ||
      var == "deps" ||
      var == "generator" ||
      var == "pool" ||
      var == "restat" ||
      var == "rspfile" ||
      var == "rspfile_content" ||
      var == "msvc_deps_prefix"
}

const map<string, const Rule*>& BindingEnv::GetRules() {
  return rules_
}

func (b *BindingEnv) LookupWithFallback(var string, eval *const EvalString, env *Env) string {
  map<string, string>::iterator i = bindings_.find(var)
  if i != bindings_.end() {
    return i.second
  }

  if eval != nil {
    return eval.Evaluate(env)
  }

  if parent_ {
    return parent_.LookupVariable(var)
  }

  return ""
}

func (e *EvalString) Evaluate(env *Env) string {
  string result
  for (TokenList::const_iterator i = parsed_.begin(); i != parsed_.end(); ++i) {
    if i.second == RAW {
      result.append(i.first)
    } else {
      result.append(env.LookupVariable(i.first))
    }
  }
  return result
}

func (e *EvalString) AddText(text StringPiece) {
  // Add it to the end of an existing RAW token if possible.
  if !parsed_.empty() && parsed_.back().second == RAW {
    parsed_.back().first.append(text.str_, text.len_)
  } else {
    parsed_.push_back(make_pair(text.AsString(), RAW))
  }
}
func (e *EvalString) AddSpecial(text StringPiece) {
  parsed_.push_back(make_pair(text.AsString(), SPECIAL))
}

func (e *EvalString) Serialize() string {
  string result
  for (TokenList::const_iterator i = parsed_.begin(); i != parsed_.end(); ++i) {
    result.append("[")
    if i.second == SPECIAL {
      result.append("$")
    }
    result.append(i.first)
    result.append("]")
  }
  return result
}

func (e *EvalString) Unparse() string {
  string result
  for (TokenList::const_iterator i = parsed_.begin(); i != parsed_.end(); ++i) {
    special := (i.second == SPECIAL)
    if special != nil {
      result.append("${")
    }
    result.append(i.first)
    if special != nil {
      result.append("}")
    }
  }
  return result
}

