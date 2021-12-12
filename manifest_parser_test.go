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
  func AssertParse(input string) {
    string err
    EXPECT_TRUE(parser.ParseTest(input, &err))
    ASSERT_EQ("", err)
    VerifyGraph(state)
  }

  State state
  VirtualFileSystem fs_
}

TEST_F(ParserTest, Empty) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(""))
}

TEST_F(ParserTest, Rules) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n" "  command = cat $in > $out\n" "\n" "rule date\n" "  command = date > $out\n" "\n" "build result: cat in_1.cc in-2.O\n"))

  ASSERT_EQ(3u, state.bindings_.GetRules().size())
  const Rule* rule = state.bindings_.GetRules().begin().second
  EXPECT_EQ("cat", rule.name())
  EXPECT_EQ("[cat ][$in][ > ][$out]", rule.GetBinding("command").Serialize())
}

TEST_F(ParserTest, RuleAttributes) {
  // Check that all of the allowed rule attributes are parsed ok.
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n" "  command = a\n" "  depfile = a\n" "  deps = a\n" "  description = a\n" "  generator = a\n" "  restat = a\n" "  rspfile = a\n" "  rspfile_content = a\n" ))
}

TEST_F(ParserTest, IgnoreIndentedComments) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "  #indented comment\n" "rule cat\n" "  command = cat $in > $out\n" "  #generator = 1\n" "  restat = 1 # comment\n" "  #comment\n" "build result: cat in_1.cc in-2.O\n" "  #comment\n"))

  ASSERT_EQ(2u, state.bindings_.GetRules().size())
  const Rule* rule = state.bindings_.GetRules().begin().second
  EXPECT_EQ("cat", rule.name())
  edge := state.GetNode("result", 0).in_edge()
  EXPECT_TRUE(edge.GetBindingBool("restat"))
  EXPECT_FALSE(edge.GetBindingBool("generator"))
}

TEST_F(ParserTest, IgnoreIndentedBlankLines) {
  // the indented blanks used to cause parse errors
  ASSERT_NO_FATAL_FAILURE(AssertParse( "  \n" "rule cat\n" "  command = cat $in > $out\n" "  \n" "build result: cat in_1.cc in-2.O\n" "  \n" "variable=1\n"))

  // the variable must be in the top level environment
  EXPECT_EQ("1", state.bindings_.LookupVariable("variable"))
}

TEST_F(ParserTest, ResponseFiles) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat_rsp\n" "  command = cat $rspfile > $out\n" "  rspfile = $rspfile\n" "  rspfile_content = $in\n" "\n" "build out: cat_rsp in\n" "  rspfile=out.rsp\n"))

  ASSERT_EQ(2u, state.bindings_.GetRules().size())
  const Rule* rule = state.bindings_.GetRules().begin().second
  EXPECT_EQ("cat_rsp", rule.name())
  EXPECT_EQ("[cat ][$rspfile][ > ][$out]", rule.GetBinding("command").Serialize())
  EXPECT_EQ("[$rspfile]", rule.GetBinding("rspfile").Serialize())
  EXPECT_EQ("[$in]", rule.GetBinding("rspfile_content").Serialize())
}

TEST_F(ParserTest, InNewline) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat_rsp\n" "  command = cat $in_newline > $out\n" "\n" "build out: cat_rsp in in2\n" "  rspfile=out.rsp\n"))

  ASSERT_EQ(2u, state.bindings_.GetRules().size())
  const Rule* rule = state.bindings_.GetRules().begin().second
  EXPECT_EQ("cat_rsp", rule.name())
  EXPECT_EQ("[cat ][$in_newline][ > ][$out]", rule.GetBinding("command").Serialize())

  edge := state.edges_[0]
  EXPECT_EQ("cat in\nin2 > out", edge.EvaluateCommand())
}

TEST_F(ParserTest, Variables) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "l = one-letter-test\n" "rule link\n" "  command = ld $l $extra $with_under -o $out $in\n" "\n" "extra = -pthread\n" "with_under = -under\n" "build a: link b c\n" "nested1 = 1\n" "nested2 = $nested1/2\n" "build supernested: link x\n" "  extra = $nested2/3\n"))

  ASSERT_EQ(2u, state.edges_.size())
  edge := state.edges_[0]
  EXPECT_EQ("ld one-letter-test -pthread -under -o a b c", edge.EvaluateCommand())
  EXPECT_EQ("1/2", state.bindings_.LookupVariable("nested2"))

  edge = state.edges_[1]
  EXPECT_EQ("ld one-letter-test 1/2/3 -under -o supernested x", edge.EvaluateCommand())
}

TEST_F(ParserTest, VariableScope) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "foo = bar\n" "rule cmd\n" "  command = cmd $foo $in $out\n" "\n" "build inner: cmd a\n" "  foo = baz\n" "build outer: cmd b\n" "\n" ))  // Extra newline after build line tickles a regression.

  ASSERT_EQ(2u, state.edges_.size())
  EXPECT_EQ("cmd baz a inner", state.edges_[0].EvaluateCommand())
  EXPECT_EQ("cmd bar b outer", state.edges_[1].EvaluateCommand())
}

TEST_F(ParserTest, Continuation) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule link\n" "  command = foo bar $\n" "    baz\n" "\n" "build a: link c $\n" " d e f\n"))

  ASSERT_EQ(2u, state.bindings_.GetRules().size())
  const Rule* rule = state.bindings_.GetRules().begin().second
  EXPECT_EQ("link", rule.name())
  EXPECT_EQ("[foo bar baz]", rule.GetBinding("command").Serialize())
}

TEST_F(ParserTest, Backslash) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "foo = bar\\baz\n" "foo2 = bar\\ baz\n" ))
  EXPECT_EQ("bar\\baz", state.bindings_.LookupVariable("foo"))
  EXPECT_EQ("bar\\ baz", state.bindings_.LookupVariable("foo2"))
}

TEST_F(ParserTest, Comment) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "# this is a comment\n" "foo = not # a comment\n"))
  EXPECT_EQ("not # a comment", state.bindings_.LookupVariable("foo"))
}

TEST_F(ParserTest, Dollars) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule foo\n" "  command = ${out}bar$$baz$$$\n" "blah\n" "x = $$dollar\n" "build $x: foo y\n" ))
  EXPECT_EQ("$dollar", state.bindings_.LookupVariable("x"))
  EXPECT_EQ("$dollarbar$baz$blah", state.edges_[0].EvaluateCommand())
  EXPECT_EQ("'$dollar'bar$baz$blah", state.edges_[0].EvaluateCommand())
}

TEST_F(ParserTest, EscapeSpaces) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule spaces\n" "  command = something\n" "build foo$ bar: spaces $$one two$$$ three\n" ))
  EXPECT_TRUE(state.LookupNode("foo bar"))
  EXPECT_EQ(state.edges_[0].outputs_[0].path(), "foo bar")
  EXPECT_EQ(state.edges_[0].inputs_[0].path(), "$one")
  EXPECT_EQ(state.edges_[0].inputs_[1].path(), "two$ three")
  EXPECT_EQ(state.edges_[0].EvaluateCommand(), "something")
}

TEST_F(ParserTest, CanonicalizeFile) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "rule cat\n" "  command = cat $in > $out\n" "build out: cat in/1 in "build in/1: cat\n" "build in/2: cat\n"))//2\n"

  EXPECT_TRUE(state.LookupNode("in/1"))
  EXPECT_TRUE(state.LookupNode("in/2"))
