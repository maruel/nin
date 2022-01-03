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
)

// An interface for a scope for variable (e.g. "$foo") lookups.
type Env interface {
	LookupVariable(v string) string
}

type TokenType bool

const (
	RAW     TokenType = false
	SPECIAL TokenType = true
)

type TokenListItem struct {
	first  string
	second TokenType
}

func (t *TokenListItem) String() string {
	out := fmt.Sprintf("%q:", t.first)
	if t.second == RAW {
		out += "RAW"
	} else {
		out += "SPECIAL"
	}
	return out
}

type TokenList []TokenListItem

// A tokenized string that contains variable references.
// Can be evaluated relative to an Env.
type EvalString struct {
	Parsed TokenList
}

func (e *EvalString) String() string {
	out := ""
	for i, t := range e.Parsed {
		if i != 0 {
			out += ","
		}
		out += t.String()
	}
	return out
}

// @return The evaluated string with variable expanded using value found in
//         environment @a env.
func (e *EvalString) Evaluate(env Env) string {
	// Warning: this function is recursive.
	var z [64]string
	var s []string
	if l := len(e.Parsed); l <= cap(z) {
		s = z[:l]
	} else {
		s = make([]string, len(e.Parsed))
	}
	total := 0
	for i, p := range e.Parsed {
		if p.second == RAW {
			x := p.first
			s[i] = x
			total += len(x)
		} else {
			x := env.LookupVariable(p.first)
			s[i] = x
			total += len(x)
		}
	}
	out := make([]byte, total)
	offset := 0
	for _, x := range s {
		l := len(x)
		copy(out[offset:], x)
		offset += l
	}
	return unsafeString(out)
}

func (e *EvalString) AddText(text string) {
	e.Parsed = append(e.Parsed, TokenListItem{text, RAW})
}

func (e *EvalString) AddSpecial(text string) {
	e.Parsed = append(e.Parsed, TokenListItem{text, SPECIAL})
}

// Construct a human-readable representation of the parsed state,
// for use in tests.
func (e *EvalString) Serialize() string {
	result := ""
	for _, i := range e.Parsed {
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
	for _, i := range e.Parsed {
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

//

type Bindings map[string]*EvalString

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

// An invocable build command and associated metadata (description, etc.).
type Rule struct {
	name_     string
	bindings_ Bindings
}

func NewRule(name string) *Rule {
	return &Rule{
		name_:     name,
		bindings_: make(Bindings),
	}
}

func (r *Rule) name() string {
	return r.name_
}

func (r *Rule) String() string {
	out := "Rule:" + r.name_ + "{"
	names := make([]string, 0, len(r.bindings_))
	for n := range r.bindings_ {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, n := range names {
		if i != 0 {
			out += ","
		}
		out += n + ":" + r.bindings_[n].String()
	}
	out += "}"
	return out
}

func (r *Rule) AddBinding(key string, val *EvalString) {
	r.bindings_[key] = val
}

func (r *Rule) GetBinding(key string) *EvalString {
	//log.Printf("Rule.GetBinding(%s): %#v", key, r.bindings_[key])
	return r.bindings_[key]
}

//

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

func (b *BindingEnv) String() string {
	out := "BindingEnv{"
	if b.parent_ != nil {
		out += "(has parent)"
	}
	out += "\n  Bindings:"
	names := make([]string, 0, len(b.bindings_))
	for n := range b.bindings_ {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out += "\n    " + n + ":" + b.bindings_[n]
	}
	out += "\n  Rules:"
	names = make([]string, 0, len(b.rules_))
	for n := range b.rules_ {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out += "\n    " + n + ":" + b.rules_[n].String()
	}
	out += "\n}"
	return out
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
	if b.LookupRuleCurrentScope(rule.name_) != nil {
		panic("oops")
	}
	b.rules_[rule.name_] = rule
}

func (b *BindingEnv) LookupRuleCurrentScope(ruleName string) *Rule {
	return b.rules_[ruleName]
}

func (b *BindingEnv) LookupRule(ruleName string) *Rule {
	i := b.rules_[ruleName]
	if i != nil {
		return i
	}
	if b.parent_ != nil {
		return b.parent_.LookupRule(ruleName)
	}
	return nil
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
