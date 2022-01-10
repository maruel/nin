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

// Env is an interface for a scope for variable (e.g. "$foo") lookups.
type Env interface {
	LookupVariable(v string) string
}

// EvalStringToken is one token in the EvalString list of items.
type EvalStringToken struct {
	Value     string
	IsSpecial bool
}

func (t *EvalStringToken) String() string {
	out := fmt.Sprintf("%q:", t.Value)
	if t.IsSpecial {
		out += "raw"
	} else {
		out += "special"
	}
	return out
}

// EvalString is a tokenized string that contains variable references.
//
// Can be evaluated relative to an Env.
type EvalString struct {
	Parsed []EvalStringToken
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

// Evaluate returns the evaluated string with variable expanded using value
// found in environment env.
func (e *EvalString) Evaluate(env Env) string {
	// Warning: this function is recursive.
	var z [64]string
	var s []string
	if l := len(e.Parsed); l <= cap(z) {
		s = z[:l]
	} else {
		s = make([]string, l)
	}
	total := 0
	for i, p := range e.Parsed {
		if !p.IsSpecial {
			x := p.Value
			s[i] = x
			total += len(x)
		} else {
			// TODO(maruel): What about when the variable is undefined? It'd be good
			// to surface this to the user, at least optionally.
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

// Serialize constructs a human-readable representation of the parsed state.
//
// Used in tests.
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

// Unparse returns the string with variables not expanded.
//
// Used for diagnostics.
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

// IsReservedBinding returns true if the binding name is reserved by ninja.
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

// Rule is an invocable build command and associated metadata (description,
// etc.).
type Rule struct {
	Name     string
	Bindings map[string]*EvalString
}

// NewRule returns an initialized Rule.
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

// BindingEnv is an Env which contains a mapping of variables to values
// as well as a pointer to a parent scope.
type BindingEnv struct {
	Bindings map[string]string
	Rules    map[string]*Rule
	Parent   *BindingEnv
}

// NewBindingEnv returns an initialized BindingEnv.
func NewBindingEnv(parent *BindingEnv) *BindingEnv {
	return &BindingEnv{
		Bindings: map[string]string{},
		Rules:    map[string]*Rule{},
		Parent:   parent,
	}
}

// String serializes the bindings.
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

// LookupVariable returns a variable's value.
func (b *BindingEnv) LookupVariable(v string) string {
	if i, ok := b.Bindings[v]; ok {
		return i
	}
	if b.Parent != nil {
		return b.Parent.LookupVariable(v)
	}
	return ""
}

// LookupRule returns a rule by name.
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
// 1) value set on edge itself (edge->env)
// 2) value set on rule, with expansion in the edge's scope
// 3) value set on enclosing scope of edge (edge->env->parent)
// This function takes as parameters the necessary info to do (2).
func (b *BindingEnv) lookupWithFallback(v string, eval *EvalString, env Env) string {
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
