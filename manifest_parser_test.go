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

import (
	"runtime"
	"testing"
)

type ParserTest struct {
	t     *testing.T
	state State
	fs_   VirtualFileSystem
}

func NewParserTest(t *testing.T) ParserTest {
	return ParserTest{
		t:     t,
		state: NewState(),
		fs_:   NewVirtualFileSystem(),
	}
}

func (p *ParserTest) AssertParse(input string) {
	parser := NewManifestParser(&p.state, &p.fs_, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest(input, &err) {
		p.t.Fatal(err)
	}
	if "" != err {
		p.t.Fatal(err)
	}
	VerifyGraph(p.t, &p.state)
}

func TestParserTest_Empty(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("")
}

func TestParserTest_Rules(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\n\nrule date\n  command = date > $out\n\nbuild result: cat in_1.cc in-2.O\n")

	if 3 != len(p.state.bindings_.GetRules()) {
		t.Fatal("expected equal")
	}
	rule := p.state.bindings_.GetRules()["cat"]
	if got := rule.name(); got != "cat" {
		t.Fatal(got)
	}
	if "[cat ][$in][ > ][$out]" != rule.GetBinding("command").Serialize() {
		t.Fatal("expected equal")
	}
}

func TestParserTest_RuleAttributes(t *testing.T) {
	p := NewParserTest(t)
	// Check that all of the allowed rule attributes are parsed ok.
	p.AssertParse("rule cat\n  command = a\n  depfile = a\n  deps = a\n  description = a\n  generator = a\n  restat = a\n  rspfile = a\n  rspfile_content = a\n")
}

func TestParserTest_IgnoreIndentedComments(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("  #indented comment\nrule cat\n  command = cat $in > $out\n  #generator = 1\n  restat = 1 # comment\n  #comment\nbuild result: cat in_1.cc in-2.O\n  #comment\n")

	if 2 != len(p.state.bindings_.GetRules()) {
		t.Fatal("expected equal")
	}
	rule := p.state.bindings_.GetRules()["cat"]
	if "cat" != rule.name() {
		t.Fatal("expected equal")
	}
	edge := p.state.GetNode("result", 0).in_edge()
	if !edge.GetBindingBool("restat") {
		t.Fatal("expected true")
	}
	if edge.GetBindingBool("generator") {
		t.Fatal("expected false")
	}
}

func TestParserTest_IgnoreIndentedBlankLines(t *testing.T) {
	p := NewParserTest(t)
	// the indented blanks used to cause parse errors
	p.AssertParse("  \nrule cat\n  command = cat $in > $out\n  \nbuild result: cat in_1.cc in-2.O\n  \nvariable=1\n")

	// the variable must be in the top level environment
	if "1" != p.state.bindings_.LookupVariable("variable") {
		t.Fatal("expected equal")
	}
}

func TestParserTest_ResponseFiles(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $in\n\nbuild out: cat_rsp in\n  rspfile=out.rsp\n")

	if 2 != len(p.state.bindings_.GetRules()) {
		t.Fatal("expected equal")
	}
	rule := p.state.bindings_.GetRules()["cat_rsp"]
	if "cat_rsp" != rule.name() {
		t.Fatal("expected equal")
	}
	if "[cat ][$rspfile][ > ][$out]" != rule.GetBinding("command").Serialize() {
		t.Fatal("expected equal")
	}
	if "[$rspfile]" != rule.GetBinding("rspfile").Serialize() {
		t.Fatal("expected equal")
	}
	if "[$in]" != rule.GetBinding("rspfile_content").Serialize() {
		t.Fatal("expected equal")
	}
}

func TestParserTest_InNewline(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat_rsp\n  command = cat $in_newline > $out\n\nbuild out: cat_rsp in in2\n  rspfile=out.rsp\n")

	if 2 != len(p.state.bindings_.GetRules()) {
		t.Fatal("expected equal")
	}
	rule := p.state.bindings_.GetRules()["cat_rsp"]
	if "cat_rsp" != rule.name() {
		t.Fatal("expected equal")
	}
	if "[cat ][$in_newline][ > ][$out]" != rule.GetBinding("command").Serialize() {
		t.Fatal("expected equal")
	}

	edge := p.state.edges_[0]
	if "cat in\nin2 > out" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Variables(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("l = one-letter-test\nrule link\n  command = ld $l $extra $with_under -o $out $in\n\nextra = -pthread\nwith_under = -under\nbuild a: link b c\nnested1 = 1\nnested2 = $nested1/2\nbuild supernested: link x\n  extra = $nested2/3\n")

	if 2 != len(p.state.edges_) {
		t.Fatalf("%v", p.state.edges_)
	}
	edge := p.state.edges_[0]
	if got := edge.EvaluateCommand(false); "ld one-letter-test -pthread -under -o a b c" != got {
		t.Fatal(got)
	}
	if "1/2" != p.state.bindings_.LookupVariable("nested2") {
		t.Fatal("expected equal")
	}

	edge = p.state.edges_[1]
	if "ld one-letter-test 1/2/3 -under -o supernested x" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestParserTest_VariableScope(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("foo = bar\nrule cmd\n  command = cmd $foo $in $out\n\nbuild inner: cmd a\n  foo = baz\nbuild outer: cmd b\n\n") // Extra newline after build line tickles a regression.

	if 2 != len(p.state.edges_) {
		t.Fatal("expected equal")
	}
	if "cmd baz a inner" != p.state.edges_[0].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
	if "cmd bar b outer" != p.state.edges_[1].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Continuation(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule link\n  command = foo bar $\n    baz\n\nbuild a: link c $\n d e f\n")

	if 2 != len(p.state.bindings_.GetRules()) {
		t.Fatal("expected equal")
	}
	rule := p.state.bindings_.GetRules()["link"]
	if "link" != rule.name() {
		t.Fatal("expected equal")
	}
	if "[foo bar baz]" != rule.GetBinding("command").Serialize() {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Backslash(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("foo = bar\\baz\nfoo2 = bar\\ baz\n")
	if "bar\\baz" != p.state.bindings_.LookupVariable("foo") {
		t.Fatal("expected equal")
	}
	if "bar\\ baz" != p.state.bindings_.LookupVariable("foo2") {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Comment(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("# this is a comment\nfoo = not # a comment\n")
	if "not # a comment" != p.state.bindings_.LookupVariable("foo") {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Dollars(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule foo\n  command = ${out}bar$$baz$$$\nblah\nx = $$dollar\nbuild $x: foo y\n")
	if "$dollar" != p.state.bindings_.LookupVariable("x") {
		t.Fatal("expected equal")
	}
	want := "'$dollar'bar$baz$blah"
	if runtime.GOOS == "windows" {
		want = "$dollarbar$baz$blah"
	}
	if want != p.state.edges_[0].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestParserTest_EscapeSpaces(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule spaces\n  command = something\nbuild foo$ bar: spaces $$one two$$$ three\n")
	if p.state.LookupNode("foo bar") == nil {
		t.Fatal("expected true")
	}
	if p.state.edges_[0].outputs_[0].path() != "foo bar" {
		t.Fatal("expected equal")
	}
	if p.state.edges_[0].inputs_[0].path() != "$one" {
		t.Fatal("expected equal")
	}
	if p.state.edges_[0].inputs_[1].path() != "two$ three" {
		t.Fatal("expected equal")
	}
	if p.state.edges_[0].EvaluateCommand(false) != "something" {
		t.Fatal("expected equal")
	}
}

func TestParserTest_CanonicalizeFile(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in/1 in//2\nbuild in/1: cat\nbuild in/2: cat\n")

	if p.state.LookupNode("in/1") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("in/2") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("in//1") != nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("in//2") != nil {
		t.Fatal("expected false")
	}
}

func TestParserTest_CanonicalizeFileBackslashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in\\1 in\\\\2\nbuild in\\1: cat\nbuild in\\2: cat\n")

	node := p.state.LookupNode("in/1")
	if node == nil {
		t.Fatal("expected true")
	}
	if 1 != node.slash_bits() {
		t.Fatal("expected equal")
	}
	node = p.state.LookupNode("in/2")
	if node == nil {
		t.Fatal("expected true")
	}
	if 1 != node.slash_bits() {
		t.Fatal("expected equal")
	}
	if p.state.LookupNode("in//1") != nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("in//2") != nil {
		t.Fatal("expected false")
	}
}

func TestParserTest_PathVariables(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\ndir = out\nbuild $dir/exe: cat src\n")

	if p.state.LookupNode("$dir/exe") != nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("out/exe") == nil {
		t.Fatal("expected true")
	}
}

func TestParserTest_CanonicalizePaths(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild ./out.o: cat ./bar/baz/../foo.cc\n")

	if p.state.LookupNode("./out.o") != nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("out.o") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("./bar/baz/../foo.cc") != nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("bar/foo.cc") == nil {
		t.Fatal("expected true")
	}
}

func TestParserTest_CanonicalizePathsBackslashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild ./out.o: cat ./bar/baz/../foo.cc\nbuild .\\out2.o: cat .\\bar/baz\\..\\foo.cc\nbuild .\\out3.o: cat .\\bar\\baz\\..\\foo3.cc\n")

	if p.state.LookupNode("./out.o") == nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode(".\\out2.o") == nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode(".\\out3.o") == nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode("out.o") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("out2.o") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("out3.o") == nil {
		t.Fatal("expected true")
	}
	if p.state.LookupNode("./bar/baz/../foo.cc") == nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode(".\\bar/baz\\..\\foo.cc") == nil {
		t.Fatal("expected false")
	}
	if p.state.LookupNode(".\\bar/baz\\..\\foo3.cc") == nil {
		t.Fatal("expected false")
	}
	node := p.state.LookupNode("bar/foo.cc")
	if node == nil {
		t.Fatal("expected true")
	}
	if 0 != node.slash_bits() {
		t.Fatal("expected equal")
	}
	node = p.state.LookupNode("bar/foo3.cc")
	if node == nil {
		t.Fatal("expected true")
	}
	if 1 != node.slash_bits() {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DuplicateEdgeWithMultipleOutputs(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n")
	// AssertParse() checks that the generated build graph is self-consistent.
	// That's all the checking that this test needs.
}

func TestParserTest_NoDeadPointerFromDuplicateEdge(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in\nbuild out: cat in\n")
	// AssertParse() checks that the generated build graph is self-consistent.
	// That's all the checking that this test needs.
}

func TestParserTest_DuplicateEdgeWithMultipleOutputsError(t *testing.T) {
	p := NewParserTest(t)
	kInput := "rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n"
	var parser_opts ManifestParserOptions
	parser_opts.dupe_edge_action_ = kDupeEdgeActionError
	parser := NewManifestParser(&p.state, &p.fs_, parser_opts)
	err := ""
	if parser.ParseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:5: multiple rules generate out1\n" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DuplicateEdgeInIncludedFile(t *testing.T) {
	p := NewParserTest(t)
	p.fs_.Create("sub.ninja", "rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n")
	kInput := "subninja sub.ninja\n"
	var parser_opts ManifestParserOptions
	parser_opts.dupe_edge_action_ = kDupeEdgeActionError
	parser := NewManifestParser(&p.state, &p.fs_, parser_opts)
	err := ""
	if parser.ParseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "sub.ninja:5: multiple rules generate out1\n" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_PhonySelfReferenceIgnored(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("build a: phony a\n")

	node := p.state.LookupNode("a")
	edge := node.in_edge()
	if len(edge.inputs_) != 0 {
		t.Fatal("expected true")
	}
}

func TestParserTest_PhonySelfReferenceKept(t *testing.T) {
	p := NewParserTest(t)
	kInput := "build a: phony a\n"
	var parser_opts ManifestParserOptions
	parser_opts.phony_cycle_action_ = kPhonyCycleActionError
	parser := NewManifestParser(&p.state, &p.fs_, parser_opts)
	err := ""
	if !parser.ParseTest(kInput, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	node := p.state.LookupNode("a")
	edge := node.in_edge()
	if len(edge.inputs_) != 1 {
		t.Fatal("expected equal")
	}
	if edge.inputs_[0] != node {
		t.Fatal("expected equal")
	}
}

func TestParserTest_ReservedWords(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule build\n  command = rule run $out\nbuild subninja: build include default foo.cc\ndefault subninja\n")
}

func TestParserTest_Errors(t *testing.T) {
	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("subn", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected '=', got eof\nsubn\n    ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("foobar", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected '=', got eof\nfoobar\n      ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x 3", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected '=', got identifier\nx 3\n  ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x = 3", &err) {
			t.Fatal("expected false")
		}
		if "input:1: unexpected EOF\nx = 3\n     ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x = 3\ny 2", &err) {
			t.Fatal("expected false")
		}
		if "input:2: expected '=', got identifier\ny 2\n  ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x = $", &err) {
			t.Fatal("expected false")
		}
		if "input:1: bad $-escape (literal $ must be written as $$)\nx = $\n    ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x = $\n $[\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: bad $-escape (literal $ must be written as $$)\n $[\n ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("x = a$\n b$\n $\n", &err) {
			t.Fatal("expected false")
		}
		if "input:4: unexpected EOF\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("build\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected path\nbuild\n     ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("build x: y z\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: unknown build rule 'y'\nbuild x: y z\n         ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("build x:: y z\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected build command name\nbuild x:: y z\n        ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = cat ok\nbuild x: cat $\n :\n", &err) {
			t.Fatal("expected false")
		}
		if "input:4: expected newline, got ':'\n :\n ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: expected 'command =' line\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = echo\nrule cat\n  command = echo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: duplicate rule 'cat'\nrule cat\n        ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = echo\n  rspfile = cat.rsp\n", &err) {
			t.Fatal("expected false")
		}
		if "input:4: rspfile and rspfile_content need to be both specified\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = ${fafsd\nfoo = bar\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: bad $-escape (literal $ must be written as $$)\n  command = ${fafsd\n            ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = cat\nbuild $.: cat foo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: bad $-escape (literal $ must be written as $$)\nbuild $.: cat foo\n      ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cat\n  command = cat\nbuild $: cat foo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: expected ':', got newline ($ also escapes ':')\nbuild $: cat foo\n                ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule %foo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected rule name\nrule %foo\n     ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cc\n  command = foo\n  othervar = bar\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: unexpected variable 'othervar'\n  othervar = bar\n                ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cc\n  command = foo\nbuild $.: cc bar.cc\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: bad $-escape (literal $ must be written as $$)\nbuild $.: cc bar.cc\n      ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cc\n  command = foo\n  && bar", &err) {
			t.Fatal("expected false")
		}
		if "input:3: expected variable name\n  && bar\n  ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule cc\n  command = foo\nbuild $: cc bar.cc\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: expected ':', got newline ($ also escapes ':')\nbuild $: cc bar.cc\n                  ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("default\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected target name\ndefault\n       ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("default nonexistent\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: unknown target 'nonexistent'\ndefault nonexistent\n                   ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule r\n  command = r\nbuild b: r\ndefault b:\n", &err) {
			t.Fatal("expected false")
		}
		if "input:4: expected newline, got ':'\ndefault b:\n         ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("default $a\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: empty path\ndefault $a\n          ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("rule r\n  command = r\nbuild $a: r $c\n", &err) {
			t.Fatal("expected false")
		}
		// XXX the line number is wrong; we should evaluate paths in ParseEdge
		// as we see them, not after we've read them all!
		if "input:4: empty path\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		// the indented blank line must terminate the rule
		// this also verifies that "unexpected (token)" errors are correct
		if parser.ParseTest("rule r\n  command = r\n  \n  generator = 1\n", &err) {
			t.Fatal("expected false")
		}
		if "input:4: unexpected indent\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("pool\n", &err) {
			t.Fatal("expected false")
		}
		if "input:1: expected pool name\npool\n    ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("pool foo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: expected 'depth =' line\n" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("pool foo\n  depth = 4\npool foo\n", &err) {
			t.Fatal("expected false")
		}
		if "input:3: duplicate pool 'foo'\npool foo\n        ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("pool foo\n  depth = -1\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: invalid pool depth\n  depth = -1\n            ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		if parser.ParseTest("pool foo\n  bar = 1\n", &err) {
			t.Fatal("expected false")
		}
		if "input:2: unexpected variable 'bar'\n  bar = 1\n         ^ near here" != err {
			t.Fatal("expected equal")
		}
	}

	{
		local_state := NewState()
		parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
		err := ""
		// Pool names are dereferenced at edge parsing time.
		if parser.ParseTest("rule run\n  command = echo\n  pool = unnamed_pool\nbuild out: run in\n", &err) {
			t.Fatal("expected false")
		}
		if "input:5: unknown pool name 'unnamed_pool'\n" != err {
			t.Fatal("expected equal")
		}
	}
}

func TestParserTest_MissingInput(t *testing.T) {
	p := NewParserTest(t)
	local_state := NewState()
	parser := NewManifestParser(&local_state, &p.fs_, ManifestParserOptions{})
	err := ""
	if parser.Load("build.ninja", &err, nil) {
		t.Fatal("expected false")
	}
	if "loading 'build.ninja': No such file or directory" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_MultipleOutputs(t *testing.T) {
	local_state := NewState()
	parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest("rule cc\n  command = foo\n  depfile = bar\nbuild a.o b.o: cc c.cc\n", &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_MultipleOutputsWithDeps(t *testing.T) {
	local_state := NewState()
	parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest("rule cc\n  command = foo\n  deps = gcc\nbuild a.o b.o: cc c.cc\n", &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_SubNinja(t *testing.T) {
	p := NewParserTest(t)
	p.fs_.Create("test.ninja", "var2 = inner\nbuild $builddir/inner: varref\n")
	p.AssertParse("builddir = some_dir/\nrule varref\n  command = varref $var2\nvar2 = outer\nbuild $builddir/outer: varref\nsubninja test.ninja\nbuild $builddir/outer2: varref\n")
	if 1 != len(p.fs_.files_read_) {
		t.Fatal("expected equal")
	}

	if "test.ninja" != p.fs_.files_read_[0] {
		t.Fatal("expected equal")
	}
	if p.state.LookupNode("some_dir/outer") == nil {
		t.Fatal("expected true")
	}
	// Verify our builddir setting is inherited.
	if p.state.LookupNode("some_dir/inner") == nil {
		t.Fatal("expected true")
	}

	if 3 != len(p.state.edges_) {
		t.Fatal("expected equal")
	}
	if "varref outer" != p.state.edges_[0].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
	if "varref inner" != p.state.edges_[1].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
	if "varref outer" != p.state.edges_[2].EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestParserTest_MissingSubNinja(t *testing.T) {
	p := NewParserTest(t)
	parser := NewManifestParser(&p.state, &p.fs_, ManifestParserOptions{})
	err := ""
	if parser.ParseTest("subninja foo.ninja\n", &err) {
		t.Fatal("expected false")
	}
	if "input:1: loading 'foo.ninja': No such file or directory\nsubninja foo.ninja\n                  ^ near here" != err {
		t.Fatal(err)
	}
}

func TestParserTest_DuplicateRuleInDifferentSubninjas(t *testing.T) {
	p := NewParserTest(t)
	// Test that rules are scoped to subninjas.
	p.fs_.Create("test.ninja", "rule cat\n  command = cat\n")
	parser := NewManifestParser(&p.state, &p.fs_, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest("rule cat\n  command = cat\nsubninja test.ninja\n", &err) {
		t.Fatal("expected true")
	}
}

func TestParserTest_DuplicateRuleInDifferentSubninjasWithInclude(t *testing.T) {
	p := NewParserTest(t)
	// Test that rules are scoped to subninjas even with includes.
	p.fs_.Create("rules.ninja", "rule cat\n  command = cat\n")
	p.fs_.Create("test.ninja", "include rules.ninja\nbuild x : cat\n")
	parser := NewManifestParser(&p.state, &p.fs_, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest("include rules.ninja\nsubninja test.ninja\nbuild y : cat\n", &err) {
		t.Fatal("expected true")
	}
}

func TestParserTest_Include(t *testing.T) {
	p := NewParserTest(t)
	p.fs_.Create("include.ninja", "var2 = inner\n")
	p.AssertParse("var2 = outer\ninclude include.ninja\n")

	if 1 != len(p.fs_.files_read_) {
		t.Fatal("expected equal")
	}
	if "include.ninja" != p.fs_.files_read_[0] {
		t.Fatal("expected equal")
	}
	if "inner" != p.state.bindings_.LookupVariable("var2") {
		t.Fatal("expected equal")
	}
}

func TestParserTest_BrokenInclude(t *testing.T) {
	p := NewParserTest(t)
	p.fs_.Create("include.ninja", "build\n")
	parser := NewManifestParser(&p.state, &p.fs_, ManifestParserOptions{})
	err := ""
	if parser.ParseTest("include include.ninja\n", &err) {
		t.Fatal("expected false")
	}
	if "include.ninja:1: expected path\nbuild\n     ^ near here" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_Implicit(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo: cat bar | baz\n")

	edge := p.state.LookupNode("foo").in_edge()
	if !edge.is_implicit(1) {
		t.Fatal("expected true")
	}
}

func TestParserTest_OrderOnly(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo: cat bar || baz\n")

	edge := p.state.LookupNode("foo").in_edge()
	if !edge.is_order_only(1) {
		t.Fatal("expected true")
	}
}

func TestParserTest_ImplicitOutput(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo | imp: cat bar\n")

	edge := p.state.LookupNode("imp").in_edge()
	if len(edge.outputs_) != 2 {
		t.Fatal("expected equal")
	}
	if !edge.is_implicit_out(1) {
		t.Fatal("expected true")
	}
}

func TestParserTest_ImplicitOutputEmpty(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo | : cat bar\n")

	edge := p.state.LookupNode("foo").in_edge()
	if len(edge.outputs_) != 1 {
		t.Fatal("expected equal")
	}
	if edge.is_implicit_out(0) {
		t.Fatal("expected false")
	}
}

func TestParserTest_ImplicitOutputDupe(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo baz | foo baq foo: cat bar\n")

	edge := p.state.LookupNode("foo").in_edge()
	if len(edge.outputs_) != 3 {
		t.Fatal("expected equal")
	}
	if edge.is_implicit_out(0) {
		t.Fatal("expected false")
	}
	if edge.is_implicit_out(1) {
		t.Fatal("expected false")
	}
	if !edge.is_implicit_out(2) {
		t.Fatal("expected true")
	}
}

func TestParserTest_ImplicitOutputDupes(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild foo foo foo | foo foo foo foo: cat bar\n")

	edge := p.state.LookupNode("foo").in_edge()
	if len(edge.outputs_) != 1 {
		t.Fatal("expected equal")
	}
	if edge.is_implicit_out(0) {
		t.Fatal("expected false")
	}
}

func TestParserTest_NoExplicitOutput(t *testing.T) {
	p := NewParserTest(t)
	parser := NewManifestParser(&p.state, nil, ManifestParserOptions{})
	err := ""
	if !parser.ParseTest("rule cat\n  command = cat $in > $out\nbuild | imp : cat bar\n", &err) {
		t.Fatal("expected true")
	}
}

func TestParserTest_DefaultDefault(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild a: cat foo\nbuild b: cat foo\nbuild c: cat foo\nbuild d: cat foo\n")

	err := ""
	if 4 != len(p.state.DefaultNodes(&err)) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DefaultDefaultCycle(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild a: cat a\n")

	err := ""
	if 0 != len(p.state.DefaultNodes(&err)) {
		t.Fatal("expected equal")
	}
	if "could not determine root nodes of build graph" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DefaultStatements(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild a: cat foo\nbuild b: cat foo\nbuild c: cat foo\nbuild d: cat foo\nthird = c\ndefault a b\ndefault $third\n")

	err := ""
	nodes := p.state.DefaultNodes(&err)
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(nodes) {
		t.Fatal("expected equal")
	}
	if "a" != nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "b" != nodes[1].path() {
		t.Fatal("expected equal")
	}
	if "c" != nodes[2].path() {
		t.Fatal("expected equal")
	}
}

func TestParserTest_UTF8(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule utf8\n  command = true\n  description = compilaci\xC3\xB3\n")
}

func TestParserTest_CRLF(t *testing.T) {
	local_state := NewState()
	parser := NewManifestParser(&local_state, nil, ManifestParserOptions{})
	err := ""

	if !parser.ParseTest("# comment with crlf\r\n", &err) {
		t.Fatal("expected true")
	}
	if !parser.ParseTest("foo = foo\nbar = bar\r\n", &err) {
		t.Fatal("expected true")
	}
	if !parser.ParseTest("pool link_pool\r\n  depth = 15\r\n\r\nrule xyz\r\n  command = something$expand \r\n  description = YAY!\r\n", &err) {
		t.Fatal("expected true")
	}
}

func TestParserTest_DyndepNotSpecified(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in\n")
	edge := p.state.GetNode("result", 0).in_edge()
	if edge.dyndep_ != nil {
		t.Fatal("expected false")
	}
}

func TestParserTest_DyndepNotInput(t *testing.T) {
	lstate := NewState()
	parser := NewManifestParser(&lstate, nil, ManifestParserOptions{})
	err := ""
	if parser.ParseTest("rule touch\n  command = touch $out\nbuild result: touch\n  dyndep = notin\n", &err) {
		t.Fatal("expected false")
	}
	if "input:5: dyndep 'notin' is not an input\n" != err {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DyndepExplicitInput(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in\n  dyndep = in\n")
	edge := p.state.GetNode("result", 0).in_edge()
	if edge.dyndep_ == nil {
		t.Fatal("expected true")
	}
	if !edge.dyndep_.dyndep_pending() {
		t.Fatal("expected true")
	}
	if edge.dyndep_.path() != "in" {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DyndepImplicitInput(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in | dd\n  dyndep = dd\n")
	edge := p.state.GetNode("result", 0).in_edge()
	if edge.dyndep_ == nil {
		t.Fatal("expected true")
	}
	if !edge.dyndep_.dyndep_pending() {
		t.Fatal("expected true")
	}
	if edge.dyndep_.path() != "dd" {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DyndepOrderOnlyInput(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in || dd\n  dyndep = dd\n")
	edge := p.state.GetNode("result", 0).in_edge()
	if edge.dyndep_ == nil {
		t.Fatal("expected true")
	}
	if !edge.dyndep_.dyndep_pending() {
		t.Fatal("expected true")
	}
	if edge.dyndep_.path() != "dd" {
		t.Fatal("expected equal")
	}
}

func TestParserTest_DyndepRuleInput(t *testing.T) {
	p := NewParserTest(t)
	p.AssertParse("rule cat\n  command = cat $in > $out\n  dyndep = $in\nbuild result: cat in\n")
	edge := p.state.GetNode("result", 0).in_edge()
	if edge.dyndep_ == nil {
		t.Fatal("expected true")
	}
	if !edge.dyndep_.dyndep_pending() {
		t.Fatal("expected true")
	}
	if edge.dyndep_.path() != "in" {
		t.Fatal("expected equal")
	}
}
