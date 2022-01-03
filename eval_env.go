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

type TokenListItem struct {
	Value     string
	IsSpecial bool
}

func (t *TokenListItem) String() string {
	out := fmt.Sprintf("%q:", t.Value)
	if t.IsSpecial {
		out += "raw"
	} else {
		out += "special"
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
		if !p.IsSpecial {
			x := p.Value
			s[i] = x
			total += len(x)
		} else {
			x := env.LookupVariable(p.Value)
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
	e.Parsed = append(e.Parsed, TokenListItem{text, false})
}

func (e *EvalString) AddSpecial(text string) {
	e.Parsed = append(e.Parsed, TokenListItem{text, true})
}

// Construct a human-readable representation of the parsed state,
// for use in tests.
func (e *EvalString) Serialize() string {
	result := ""
	for _, i := range e.Parsed {
		result += "["
		if i.IsSpecial {
			result += "$"
		}
		result += i.Value
		result += "]"
	}
	return result
}

// @return The string with variables not expanded.
func (e *EvalString) Unparse() string {
	result := ""
	for _, i := range e.Parsed {
		special := i.IsSpecial
		if special {
			result += "${"
		}
		result += i.Value
		if special {
			result += "}"
		}
	}
	return result
}

//

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
	Name     string
	Bindings map[string]*EvalString
}

func NewRule(name string) *Rule {
	return &Rule{
		Name:     name,
		Bindings: map[string]*EvalString{},
	}
}

func (r *Rule) String() string {
	out := "Rule:" + r.Name + "{"
	names := make([]string, 0, len(r.Bindings))
	for n := range r.Bindings {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, n := range names {
		if i != 0 {
			out += ","
		}
		out += n + ":" + r.Bindings[n].String()
	}
	out += "}"
	return out
}

//

// An Env which contains a mapping of variables to values
// as well as a pointer to a parent scope.
type BindingEnv struct {
	Bindings map[string]string
	Rules    map[string]*Rule
	Parent   *BindingEnv
}

func NewBindingEnv(parent *BindingEnv) *BindingEnv {
	return &BindingEnv{
		Bindings: map[string]string{},
		Rules:    map[string]*Rule{},
		Parent:   parent,
	}
}

func (b *BindingEnv) String() string {
	out := "BindingEnv{"
	if b.Parent != nil {
		out += "(has parent)"
	}
	out += "\n  Bindings:"
	names := make([]string, 0, len(b.Bindings))
	for n := range b.Bindings {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out += "\n    " + n + ":" + b.Bindings[n]
	}
	out += "\n  Rules:"
	names = make([]string, 0, len(b.Rules))
	for n := range b.Rules {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out += "\n    " + n + ":" + b.Rules[n].String()
	}
	out += "\n}"
	return out
}

func (b *BindingEnv) LookupVariable(v string) string {
	if i, ok := b.Bindings[v]; ok {
		return i
	}
	if b.Parent != nil {
		return b.Parent.LookupVariable(v)
	}
	return ""
}

func (b *BindingEnv) LookupRule(ruleName string) *Rule {
	if i := b.Rules[ruleName]; i != nil {
		return i
	}
	if b.Parent != nil {
		return b.Parent.LookupRule(ruleName)
	}
	return nil
}

// This is tricky.  Edges want lookup scope to go in this order:
// 1) value set on edge itself (edge_->env_)
// 2) value set on rule, with expansion in the edge's scope
// 3) value set on enclosing scope of edge (edge_->env_->parent_)
// This function takes as parameters the necessary info to do (2).
func (b *BindingEnv) LookupWithFallback(v string, eval *EvalString, env Env) string {
	if i, ok := b.Bindings[v]; ok {
		return i
	}
	if eval != nil {
		return eval.Evaluate(env)
	}
	if b.Parent != nil {
		return b.Parent.LookupVariable(v)
	}
	return ""
}
