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

import "testing"

func TestLexer_ReadVarValue(t *testing.T) {
	lexer := NewLexer("plain text $var $VaR ${x}\n")
	eval := EvalString{}
	err := ""
	if !lexer.ReadVarValue(&eval, &err) {
		t.Fatalf("ReadVarValue = %v; %s", eval, err)
	}
	if err != "" {
		t.Fatal(err)
	}
	if got := eval.Serialize(); got != "[plain text ][$var][ ][$VaR][ ][$x]" {
		t.Fatal(got)
	}
}

func TestLexer_ReadEvalStringEscapes(t *testing.T) {
	lexer := NewLexer("$ $$ab c$: $\ncde\n")
	var eval EvalString
	err := ""
	if !lexer.ReadVarValue(&eval, &err) {
		t.Fatalf("ReadVarValue = %v; %s", eval, err)
	}
	if err != "" {
		t.Fatal(err)
	}
	if got := eval.Serialize(); got != "[ $ab c: cde]" {
		t.Fatal(got)
	}
}

func TestLexer_ReadIdent(t *testing.T) {
	lexer := NewLexer("foo baR baz_123 foo-bar")
	ident := ""
	if !lexer.ReadIdent(&ident) {
		t.Fatal()
	}
	if ident != "foo" {
		t.Fatal()
	}
	if !lexer.ReadIdent(&ident) {
		t.Fatal()
	}
	if ident != "baR" {
		t.Fatal()
	}
	if !lexer.ReadIdent(&ident) {
		t.Fatal()
	}
	if ident != "baz_123" {
		t.Fatal()
	}
	if !lexer.ReadIdent(&ident) {
		t.Fatal()
	}
	if ident != "foo-bar" {
		t.Fatal()
	}
}

func TestLexer_ReadIdentCurlies(t *testing.T) {
	// Verify that ReadIdent includes dots in the name,
	// but in an expansion $bar.dots stops at the dot.
	lexer := NewLexer("foo.dots $bar.dots ${bar.dots}\n")
	ident := ""
	if !lexer.ReadIdent(&ident) {
		t.Fatal()
	}
	if ident != "foo.dots" {
		t.Fatal(ident)
	}
	eval := EvalString{}
	err := ""
	if !lexer.ReadVarValue(&eval, &err) {
		t.Fatal()
	}
	if err != "" {
		t.Fatal(err)
	}
	if got := eval.Serialize(); got != "[$bar][.dots ][$bar.dots]" {
		t.Fatal(got)
	}
}

func TestLexer_Error(t *testing.T) {
	lexer := NewLexer("foo$\nbad $")
	eval := EvalString{}
	err := ""
	if lexer.ReadVarValue(&eval, &err) {
		t.Fatal()
	}
	if "input:2: bad $-escape (literal $ must be written as $$)\nbad $\n    ^ near here" != err {
		t.Fatal(err)
	}
}

func TestLexer_CommentEOF(t *testing.T) {
	// Verify we don't run off the end of the string when the EOF is
	// mid-comment.
	lexer := NewLexer("# foo")
	token := lexer.ReadToken()
	if ERROR != token {
		t.Fatal(token)
	}
}

func TestLexer_Tabs(t *testing.T) {
	// Verify we print a useful error on a disallowed character.
	lexer := NewLexer("   \tfoobar")
	token := lexer.ReadToken()
	if INDENT != token {
		t.Fatal()
	}
	token = lexer.ReadToken()
	if ERROR != token {
		t.Fatal()
	}
	if got := lexer.DescribeLastError(); got != "tabs are not allowed, use spaces" {
		t.Fatal()
	}
}
