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


type ParserTest struct {

  state State
  fs_ VirtualFileSystem
}
func (p *ParserTest) AssertParse(input string) {
  ManifestParser parser(&state, &p.fs_)
  err := ""
  if parser.ParseTest(input, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  VerifyGraph(state)
}

func TestParserTest_Empty(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(""))
}

func TestParserTest_Rules(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n  command = cat $in > $out\n\nrule date\n  command = date > $out\n\nbuild result: cat in_1.cc in-2.O\n"))

  if 3u != state.bindings_.GetRules().size() { t.FailNow() }
  rule := state.bindings_.GetRules().begin().second
  if "cat" != rule.name() { t.FailNow() }
  if "[cat ][$in][ > ][$out]" != rule.GetBinding("command").Serialize() { t.FailNow() }
}

func TestParserTest_RuleAttributes(t *testing.T) {
  // Check that all of the allowed rule attributes are parsed ok.
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n  command = a\n  depfile = a\n  deps = a\n  description = a\n  generator = a\n  restat = a\n  rspfile = a\n  rspfile_content = a\n" ))
}

func TestParserTest_IgnoreIndentedComments(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "  #indented comment\nrule cat\n  command = cat $in > $out\n  #generator = 1\n  restat = 1 # comment\n  #comment\nbuild result: cat in_1.cc in-2.O\n  #comment\n"))

  if 2u != state.bindings_.GetRules().size() { t.FailNow() }
  rule := state.bindings_.GetRules().begin().second
  if "cat" != rule.name() { t.FailNow() }
  Edge* edge = state.GetNode("result", 0).in_edge()
  if edge.GetBindingBool("restat") { t.FailNow() }
  if !edge.GetBindingBool("generator") { t.FailNow() }
}

func TestParserTest_IgnoreIndentedBlankLines(t *testing.T) {
  // the indented blanks used to cause parse errors
  ASSERT_NO_FATAL_FAILURE(AssertParse( "  \nrule cat\n  command = cat $in > $out\n  \nbuild result: cat in_1.cc in-2.O\n  \nvariable=1\n"))

  // the variable must be in the top level environment
  if "1" != state.bindings_.LookupVariable("variable") { t.FailNow() }
}

func TestParserTest_ResponseFiles(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $in\n\nbuild out: cat_rsp in\n  rspfile=out.rsp\n"))

  if 2u != state.bindings_.GetRules().size() { t.FailNow() }
  rule := state.bindings_.GetRules().begin().second
  if "cat_rsp" != rule.name() { t.FailNow() }
  if "[cat ][$rspfile][ > ][$out]" != rule.GetBinding("command").Serialize() { t.FailNow() }
  if "[$rspfile]" != rule.GetBinding("rspfile").Serialize() { t.FailNow() }
  if "[$in]" != rule.GetBinding("rspfile_content").Serialize() { t.FailNow() }
}

func TestParserTest_InNewline(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat_rsp\n  command = cat $in_newline > $out\n\nbuild out: cat_rsp in in2\n  rspfile=out.rsp\n"))

  if 2u != state.bindings_.GetRules().size() { t.FailNow() }
  rule := state.bindings_.GetRules().begin().second
  if "cat_rsp" != rule.name() { t.FailNow() }
  if "[cat ][$in_newline][ > ][$out]" != rule.GetBinding("command").Serialize() { t.FailNow() }

  edge := state.edges_[0]
  if "cat in\nin2 > out" != edge.EvaluateCommand() { t.FailNow() }
}

func TestParserTest_Variables(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "l = one-letter-test\nrule link\n  command = ld $l $extra $with_under -o $out $in\n\nextra = -pthread\nwith_under = -under\nbuild a: link b c\nnested1 = 1\nnested2 = $nested1/2\nbuild supernested: link x\n  extra = $nested2/3\n"))

  if 2u != state.edges_.size() { t.FailNow() }
  edge := state.edges_[0]
  if "ld one-letter-test -pthread -under -o a b c" != edge.EvaluateCommand() { t.FailNow() }
  if "1/2" != state.bindings_.LookupVariable("nested2") { t.FailNow() }

  edge = state.edges_[1]
  if "ld one-letter-test 1/2/3 -under -o supernested x" != edge.EvaluateCommand() { t.FailNow() }
}

func TestParserTest_VariableScope(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "foo = bar\nrule cmd\n  command = cmd $foo $in $out\n\nbuild inner: cmd a\n  foo = baz\nbuild outer: cmd b\n\n" ))  // Extra newline after build line tickles a regression.

  if 2u != state.edges_.size() { t.FailNow() }
  if "cmd baz a inner" != state.edges_[0].EvaluateCommand() { t.FailNow() }
  if "cmd bar b outer" != state.edges_[1].EvaluateCommand() { t.FailNow() }
}

func TestParserTest_Continuation(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule link\n  command = foo bar $\n    baz\n\nbuild a: link c $\n d e f\n"))

  if 2u != state.bindings_.GetRules().size() { t.FailNow() }
  rule := state.bindings_.GetRules().begin().second
  if "link" != rule.name() { t.FailNow() }
  if "[foo bar baz]" != rule.GetBinding("command").Serialize() { t.FailNow() }
}

func TestParserTest_Backslash(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "foo = bar\\baz\nfoo2 = bar\\ baz\n" ))
  if "bar\\baz" != state.bindings_.LookupVariable("foo") { t.FailNow() }
  if "bar\\ baz" != state.bindings_.LookupVariable("foo2") { t.FailNow() }
}

func TestParserTest_Comment(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "# this is a comment\nfoo = not # a comment\n"))
  if "not # a comment" != state.bindings_.LookupVariable("foo") { t.FailNow() }
}

func TestParserTest_Dollars(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule foo\n  command = ${out}bar$$baz$$$\nblah\nx = $$dollar\nbuild $x: foo y\n" ))
  if "$dollar" != state.bindings_.LookupVariable("x") { t.FailNow() }
  if "$dollarbar$baz$blah" != state.edges_[0].EvaluateCommand() { t.FailNow() }
}

func TestParserTest_EscapeSpaces(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule spaces\n  command = something\nbuild foo$ bar: spaces $$one two$$$ three\n" ))
  if state.LookupNode("foo bar") { t.FailNow() }
  if state.edges_[0].outputs_[0].path() != "foo bar" { t.FailNow() }
  if state.edges_[0].inputs_[0].path() != "$one" { t.FailNow() }
  if state.edges_[0].inputs_[1].path() != "two$ three" { t.FailNow() }
  if state.edges_[0].EvaluateCommand() != "something" { t.FailNow() }
}

func TestParserTest_CanonicalizeFile(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n  command = cat $in > $out\nbuild out: cat in/1 in "build in/1: cat\nbuild in/2: cat\n"))//2\n"

  EXPECT_TRUE(state.LookupNode("in/1"))
  EXPECT_TRUE(state.LookupNode("in/2"))
