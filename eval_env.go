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

// An interface for a scope for variable (e.g. "$foo") lookups.
type Env interface {
	LookupVariable(v string) string
}

type TokenType int

const (
	RAW TokenType = iota
	SPECIAL
)

type TokenListItem struct {
	first  string
	second TokenType
}

type TokenList []TokenListItem

// A tokenized string that contains variable references.
// Can be evaluated relative to an Env.
type EvalString struct {
	parsed_ TokenList
}

func (e *EvalString) Clear()      { e.parsed_ = nil }
func (e *EvalString) empty() bool { return len(e.parsed_) == 0 }

type Bindings map[string]*EvalString

// An invocable build command and associated metadata (description, etc.).
type Rule struct {
	name_     string
	bindings_ Bindings
}

func NewRule(name string) *Rule {
	return &Rule{name_: name}
}

func (r *Rule) name() string {
	return r.name_
}

// An Env which contains a mapping of variables to values
// as well as a pointer to a parent scope.
type BindingEnv struct {
	bindings_ map[string]string
	rules_    map[string]*Rule
	parent_   *BindingEnv
}

func NewBindingEnv(parent *BindingEnv) *BindingEnv {
	return &BindingEnv{
		bindings_: map[string]string{},
		rules_:    map[string]*Rule{},
		parent_:   parent,
	}
}

func (b *BindingEnv) LookupVariable(v string) string {
	if i, ok := b.bindings_[v]; ok {
		return i
	}
	if b.parent_ != nil {
		return b.parent_.LookupVariable(v)
	}
	return ""
}

func (b *BindingEnv) AddBinding(key string, val string) {
	b.bindings_[key] = val
}

func (b *BindingEnv) AddRule(rule *Rule) {
	assert(b.LookupRuleCurrentScope(rule.name_) == nil)
	b.rules_[rule.name_] = rule
}

func (b *BindingEnv) LookupRuleCurrentScope(rule_name string) *Rule {
	return b.rules_[rule_name]
}

func (b *BindingEnv) LookupRule(rule_name string) *Rule {
	i := b.rules_[rule_name]
	if i != nil {
		return i
	}
	if b.parent_ != nil {
		return b.parent_.LookupRule(rule_name)
	}
	return nil
}

func (r *Rule) AddBinding(key string, val *EvalString) {
	r.bindings_[key] = val
}

func (r *Rule) GetBinding(key string) *EvalString {
	return r.bindings_[key]
}

func IsReservedBinding(v string) bool {
	return v == "command" ||
		v == "depfile" ||
		v == "dyndep" ||
		v == "description" ||
		v == "deps" ||
		v == "generator" ||
		v == "pool" ||
		v == "restat" ||
		v == "rspfile" ||
		v == "rspfile_content" ||
		v == "msvc_deps_prefix"
}

func (b *BindingEnv) GetRules() map[string]*Rule {
	return b.rules_
}

// This is tricky.  Edges want lookup scope to go in this order:
// 1) value set on edge itself (edge_->env_)
// 2) value set on rule, with expansion in the edge's scope
// 3) value set on enclosing scope of edge (edge_->env_->parent_)
// This function takes as parameters the necessary info to do (2).
func (b *BindingEnv) LookupWithFallback(v string, eval *EvalString, env Env) string {
	if i, ok := b.bindings_[v]; ok {
		return i
	}

	if eval != nil {
		return eval.Evaluate(env)
	}

	if b.parent_ != nil {
		return b.parent_.LookupVariable(v)
	}

	return ""
}

// @return The evaluated string with variable expanded using value found in
//         environment @a env.
func (e *EvalString) Evaluate(env Env) string {
	result := ""
	for _, i := range e.parsed_ {
		if i.second == RAW {
			result += i.first
		} else {
			result += env.LookupVariable(i.first)
		}
	}
	return result
}

func (e *EvalString) AddText(text string) {
	// Add it to the end of an existing RAW token if possible.
	if len(e.parsed_) != 0 && e.parsed_[len(e.parsed_)-1].second == RAW {
		e.parsed_[len(e.parsed_)-1].first = e.parsed_[len(e.parsed_)-1].first + text
	} else {
		e.parsed_ = append(e.parsed_, TokenListItem{text, RAW})
	}
}

func (e *EvalString) AddSpecial(text string) {
	e.parsed_ = append(e.parsed_, TokenListItem{text, SPECIAL})
}

// Construct a human-readable representation of the parsed state,
// for use in tests.
func (e *EvalString) Serialize() string {
	result := ""
	for _, i := range e.parsed_ {
		result += "["
		if i.second == SPECIAL {
			result += "$"
		}
		result += i.first
		result += "]"
	}
	return result
}

// @return The string with variables not expanded.
func (e *EvalString) Unparse() string {
	result := ""
	for _, i := range e.parsed_ {
		special := (i.second == SPECIAL)
		if special {
			result += "${"
		}
		result += i.first
		if special {
			result += "}"
		}
	}
	return result
}
