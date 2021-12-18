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


type GraphTest struct {

  fs_ VirtualFileSystem
  scan_ DependencyScan
}
GraphTest() : scan_(&state_, nil, nil, &fs_, nil) {}

func TestGraphTest_MissingImplicit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat in | implicit\n"))
  fs_.Create("in", "")
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // A missing implicit dep *should* make the output dirty.
  // (In fact, a build will fail.)
  // This is a change from prior semantics of ninja.
  if GetNode("out").dirty() { t.FailNow() }
}

func TestGraphTest_ModifiedImplicit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat in | implicit\n"))
  fs_.Create("in", "")
  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("implicit", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // A modified implicit dep should make the output dirty.
  if GetNode("out").dirty() { t.FailNow() }
}

func TestGraphTest_FunkyMakefilePath(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild out.o: catdep foo.cc\n"))
  fs_.Create("foo.cc",  "")
  fs_.Create("out.o.d", "out.o: ./foo/../implicit.h\n")
  fs_.Create("out.o", "")
  fs_.Tick()
  fs_.Create("implicit.h", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // implicit.h has changed, though our depfile refers to it with a
  // non-canonical path; we should still find it.
  if GetNode("out.o").dirty() { t.FailNow() }
}

func TestGraphTest_ExplicitImplicit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild implicit.h: cat data\nbuild out.o: catdep foo.cc || implicit.h\n"))
  fs_.Create("implicit.h", "")
  fs_.Create("foo.cc", "")
  fs_.Create("out.o.d", "out.o: implicit.h\n")
  fs_.Create("out.o", "")
  fs_.Tick()
  fs_.Create("data", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // We have both an implicit and an explicit dep on implicit.h.
  // The implicit dep should "win" (in the sense that it should cause
  // the output to be dirty).
  if GetNode("out.o").dirty() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputParse(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out | out.imp: cat in\n"))

  Edge* edge = GetNode("out").in_edge()
  if 2 != edge.outputs_.size() { t.FailNow() }
  if "out" != edge.outputs_[0].path() { t.FailNow() }
  if "out.imp" != edge.outputs_[1].path() { t.FailNow() }
  if 1 != edge.implicit_outs_ { t.FailNow() }
  if edge != GetNode("out.imp").in_edge() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputMissing(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out | out.imp: cat in\n"))
  fs_.Create("in", "")
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("out").dirty() { t.FailNow() }
  if GetNode("out.imp").dirty() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputOutOfDate(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out | out.imp: cat in\n"))
  fs_.Create("out.imp", "")
  fs_.Tick()
  fs_.Create("in", "")
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("out").dirty() { t.FailNow() }
  if GetNode("out.imp").dirty() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputOnlyParse(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build | out.imp: cat in\n"))

  Edge* edge = GetNode("out.imp").in_edge()
  if 1 != edge.outputs_.size() { t.FailNow() }
  if "out.imp" != edge.outputs_[0].path() { t.FailNow() }
  if 1 != edge.implicit_outs_ { t.FailNow() }
  if edge != GetNode("out.imp").in_edge() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputOnlyMissing(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build | out.imp: cat in\n"))
  fs_.Create("in", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.imp"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("out.imp").dirty() { t.FailNow() }
}

func TestGraphTest_ImplicitOutputOnlyOutOfDate(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build | out.imp: cat in\n"))
  fs_.Create("out.imp", "")
  fs_.Tick()
  fs_.Create("in", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.imp"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("out.imp").dirty() { t.FailNow() }
}

func TestGraphTest_PathWithCurrentDirectory(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n"))
  fs_.Create("foo.cc", "")
  fs_.Create("out.o.d", "out.o: foo.cc\n")
  fs_.Create("out.o", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !GetNode("out.o").dirty() { t.FailNow() }
}

func TestGraphTest_RootNodes(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out1: cat in1\nbuild mid1: cat in1\nbuild out2: cat mid1\nbuild out3 out4: cat mid1\n"))

  err := ""
  root_nodes := state_.RootNodes(&err)
  if 4u != root_nodes.size() { t.FailNow() }
  for i := 0; i < root_nodes.size(); i++ {
    name := root_nodes[i].path()
    if "out" != name.substr(0, 3) { t.FailNow() }
  }
}

func TestGraphTest_VarInOutPathEscaping(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build a$ b: cat no'space with$ space$$ no\"space2\n"))

  Edge* edge = GetNode("a b").in_edge()
  if "cat no'space \"with space$\" \"no\\\"space2\" > \"a b\"" != edge.EvaluateCommand() { t.FailNow() }
  if "cat 'no'\\''space' 'with space$' 'no\"space2' > 'a b'" != edge.EvaluateCommand() { t.FailNow() }
}

// Regression test for https://github.com/ninja-build/ninja/issues/380
func TestGraphTest_DepfileWithCanonicalizablePath(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n"))
  fs_.Create("foo.cc", "")
  fs_.Create("out.o.d", "out.o: bar/../foo.cc\n")
  fs_.Create("out.o", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !GetNode("out.o").dirty() { t.FailNow() }
}

// Regression test for https://github.com/ninja-build/ninja/issues/404
func TestGraphTest_DepfileRemoved(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n"))
  fs_.Create("foo.h", "")
  fs_.Create("foo.cc", "")
  fs_.Tick()
  fs_.Create("out.o.d", "out.o: foo.h\n")
  fs_.Create("out.o", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !GetNode("out.o").dirty() { t.FailNow() }

  state_.Reset()
  fs_.RemoveFile("out.o.d")
  if scan_.RecomputeDirty(GetNode("out.o"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if GetNode("out.o").dirty() { t.FailNow() }
}

// Check that rule-level variables are in scope for eval.
func TestGraphTest_RuleVariablesInScope(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n"))
  Edge* edge = GetNode("out").in_edge()
  if "depfile is x" != edge.EvaluateCommand() { t.FailNow() }
}

// Check that build statements can override rule builtins like depfile.
func TestGraphTest_DepfileOverride(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule r\n  depfile = x\n  command = unused\nbuild out: r in\n  depfile = y\n"))
  Edge* edge = GetNode("out").in_edge()
  if "y" != edge.GetBinding("depfile") { t.FailNow() }
}

// Check that overridden values show up in expansion of rule-level bindings.
func TestGraphTest_DepfileOverrideParent(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n  depfile = y\n"))
  Edge* edge = GetNode("out").in_edge()
  if "depfile is y" != edge.GetBinding("command") { t.FailNow() }
}

// Verify that building a nested phony rule prints "no work to do"
func TestGraphTest_NestedPhonyPrintsDone(t *testing.T) {
  AssertParse(&state_, "build n1: phony \nbuild n2: phony n1\n" )
  err := ""
  if scan_.RecomputeDirty(GetNode("n2"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  var plan_ Plan
  if plan_.AddTarget(GetNode("n2"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if 0 != plan_.command_edge_count() { t.FailNow() }
  if !plan_.more_to_do() { t.FailNow() }
}

func TestGraphTest_PhonySelfReferenceError(t *testing.T) {
  var parser_opts ManifestParserOptions
  parser_opts.phony_cycle_action_ = kPhonyCycleActionError
  AssertParse(&state_, "build a: phony a\n", parser_opts)

  err := ""
  if !scan_.RecomputeDirty(GetNode("a"), &err) { t.FailNow() }
  if "dependency cycle: a . a [-w phonycycle=err]" != err { t.FailNow() }
}

func TestGraphTest_DependencyCycle(t *testing.T) {
  AssertParse(&state_, "build out: cat mid\nbuild mid: cat in\nbuild in: cat pre\nbuild pre: cat out\n")

  err := ""
  if !scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "dependency cycle: out . mid . in . pre . out" != err { t.FailNow() }
}

func TestGraphTest_CycleInEdgesButNotInNodes1(t *testing.T) {
  err := ""
  AssertParse(&state_, "build a b: cat a\n")
  if !scan_.RecomputeDirty(GetNode("b"), &err) { t.FailNow() }
  if "dependency cycle: a . a" != err { t.FailNow() }
}

func TestGraphTest_CycleInEdgesButNotInNodes2(t *testing.T) {
  err := ""
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build b a: cat a\n"))
  if !scan_.RecomputeDirty(GetNode("b"), &err) { t.FailNow() }
  if "dependency cycle: a . a" != err { t.FailNow() }
}

func TestGraphTest_CycleInEdgesButNotInNodes3(t *testing.T) {
  err := ""
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build a b: cat c\nbuild c: cat a\n"))
  if !scan_.RecomputeDirty(GetNode("b"), &err) { t.FailNow() }
  if "dependency cycle: a . c . a" != err { t.FailNow() }
}

func TestGraphTest_CycleInEdgesButNotInNodes4(t *testing.T) {
  err := ""
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build d: cat c\nbuild c: cat b\nbuild b: cat a\nbuild a e: cat d\nbuild f: cat e\n"))
  if !scan_.RecomputeDirty(GetNode("f"), &err) { t.FailNow() }
  if "dependency cycle: a . d . c . b . a" != err { t.FailNow() }
}

// Verify that cycles in graphs with multiple outputs are handled correctly
// in RecomputeDirty() and don't cause deps to be loaded multiple times.
func TestGraphTest_CycleWithLengthZeroFromDepfile(t *testing.T) {
  AssertParse(&state_, "rule deprule\n   depfile = dep.d\n   command = unused\nbuild a b: deprule\n" )
  fs_.Create("dep.d", "a: b\n")

  err := ""
  if !scan_.RecomputeDirty(GetNode("a"), &err) { t.FailNow() }
  if "dependency cycle: b . b" != err { t.FailNow() }

  // Despite the depfile causing edge to be a cycle (it has outputs a and b,
  // but the depfile also adds b as an input), the deps should have been loaded
  // only once:
  Edge* edge = GetNode("a").in_edge()
  if 1 != edge.inputs_.size() { t.FailNow() }
  if "b" != edge.inputs_[0].path() { t.FailNow() }
}

// Like CycleWithLengthZeroFromDepfile but with a higher cycle length.
func TestGraphTest_CycleWithLengthOneFromDepfile(t *testing.T) {
  AssertParse(&state_, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\n" )
  fs_.Create("dep.d", "a: c\n")

  err := ""
  if !scan_.RecomputeDirty(GetNode("a"), &err) { t.FailNow() }
  if "dependency cycle: b . c . b" != err { t.FailNow() }

  // Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
  // but c's in_edge has b as input but the depfile also adds |edge| as
  // output)), the deps should have been loaded only once:
  Edge* edge = GetNode("a").in_edge()
  if 1 != edge.inputs_.size() { t.FailNow() }
  if "c" != edge.inputs_[0].path() { t.FailNow() }
}

// Like CycleWithLengthOneFromDepfile but building a node one hop away from
// the cycle.
func TestGraphTest_CycleWithLengthOneFromDepfileOneHopAway(t *testing.T) {
  AssertParse(&state_, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\nbuild d: r a\n" )
  fs_.Create("dep.d", "a: c\n")

  err := ""
  if !scan_.RecomputeDirty(GetNode("d"), &err) { t.FailNow() }
  if "dependency cycle: b . c . b" != err { t.FailNow() }

  // Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
  // but c's in_edge has b as input but the depfile also adds |edge| as
  // output)), the deps should have been loaded only once:
  Edge* edge = GetNode("a").in_edge()
  if 1 != edge.inputs_.size() { t.FailNow() }
  if "c" != edge.inputs_[0].path() { t.FailNow() }
}

func TestGraphTest_Decanonicalize(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out\\out1: cat src\\in1\nbuild out\\out2/out3\\out4: cat mid1\nbuild out3 out4\\foo: cat mid1\n"))

  err := ""
  root_nodes := state_.RootNodes(&err)
  if 4u != root_nodes.size() { t.FailNow() }
  if root_nodes[0].path() != "out/out1" { t.FailNow() }
  if root_nodes[1].path() != "out/out2/out3/out4" { t.FailNow() }
  if root_nodes[2].path() != "out3" { t.FailNow() }
  if root_nodes[3].path() != "out4/foo" { t.FailNow() }
  if root_nodes[0].PathDecanonicalized() != "out\\out1" { t.FailNow() }
  if root_nodes[1].PathDecanonicalized() != "out\\out2/out3\\out4" { t.FailNow() }
  if root_nodes[2].PathDecanonicalized() != "out3" { t.FailNow() }
  if root_nodes[3].PathDecanonicalized() != "out4\\foo" { t.FailNow() }
}

func TestGraphTest_DyndepLoadTrivial(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !GetNode("dd").dyndep_pending() { t.FailNow() }

  Edge* edge = GetNode("out").in_edge()
  if 1u != edge.outputs_.size() { t.FailNow() }
  if "out" != edge.outputs_[0].path() { t.FailNow() }
  if 2u != edge.inputs_.size() { t.FailNow() }
  if "in" != edge.inputs_[0].path() { t.FailNow() }
  if "dd" != edge.inputs_[1].path() { t.FailNow() }
  if 0u != edge.implicit_deps_ { t.FailNow() }
  if 1u != edge.order_only_deps_ { t.FailNow() }
  if !edge.GetBindingBool("restat") { t.FailNow() }
}

func TestGraphTest_DyndepLoadImplicit(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out1: r in || dd\n  dyndep = dd\nbuild out2: r in\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out1: dyndep | out2\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !GetNode("dd").dyndep_pending() { t.FailNow() }

  Edge* edge = GetNode("out1").in_edge()
  if 1u != edge.outputs_.size() { t.FailNow() }
  if "out1" != edge.outputs_[0].path() { t.FailNow() }
  if 3u != edge.inputs_.size() { t.FailNow() }
  if "in" != edge.inputs_[0].path() { t.FailNow() }
  if "out2" != edge.inputs_[1].path() { t.FailNow() }
  if "dd" != edge.inputs_[2].path() { t.FailNow() }
  if 1u != edge.implicit_deps_ { t.FailNow() }
  if 1u != edge.order_only_deps_ { t.FailNow() }
  if !edge.GetBindingBool("restat") { t.FailNow() }
}

func TestGraphTest_DyndepLoadMissingFile(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if !scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "loading 'dd': No such file or directory" != err { t.FailNow() }
}

func TestGraphTest_DyndepLoadMissingEntry(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if !scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "'out' not mentioned in its dyndep file 'dd'" != err { t.FailNow() }
}

func TestGraphTest_DyndepLoadExtraEntry(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\nbuild out2: r in || dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild out2: dyndep\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if !scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "dyndep file 'dd' mentions output 'out2' whose build statement does not have a dyndep binding for the file" != err { t.FailNow() }
}

func TestGraphTest_DyndepLoadOutputWithMultipleRules1(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out1 | out-twice.imp: r in1\nbuild out2: r in2 || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if !scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "multiple rules generate out-twice.imp" != err { t.FailNow() }
}

func TestGraphTest_DyndepLoadOutputWithMultipleRules2(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out1: r in1 || dd1\n  dyndep = dd1\nbuild out2: r in2 || dd2\n  dyndep = dd2\n" )
  fs_.Create("dd1", "ninja_dyndep_version = 1\nbuild out1 | out-twice.imp: dyndep\n" )
  fs_.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n" )

  err := ""
  if GetNode("dd1").dyndep_pending() { t.FailNow() }
  if scan_.LoadDyndeps(GetNode("dd1"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if GetNode("dd2").dyndep_pending() { t.FailNow() }
  if !scan_.LoadDyndeps(GetNode("dd2"), &err) { t.FailNow() }
  if "multiple rules generate out-twice.imp" != err { t.FailNow() }
}

func TestGraphTest_DyndepLoadMultiple(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out1: r in1 || dd\n  dyndep = dd\nbuild out2: r in2 || dd\n  dyndep = dd\nbuild outNot: r in3 || dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out1 | out1imp: dyndep | in1imp\nbuild out2: dyndep | in2imp\n  restat = 1\n" )

  err := ""
  if GetNode("dd").dyndep_pending() { t.FailNow() }
  if scan_.LoadDyndeps(GetNode("dd"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !GetNode("dd").dyndep_pending() { t.FailNow() }

  Edge* edge1 = GetNode("out1").in_edge()
  if 2u != edge1.outputs_.size() { t.FailNow() }
  if "out1" != edge1.outputs_[0].path() { t.FailNow() }
  if "out1imp" != edge1.outputs_[1].path() { t.FailNow() }
  if 1u != edge1.implicit_outs_ { t.FailNow() }
  if 3u != edge1.inputs_.size() { t.FailNow() }
  if "in1" != edge1.inputs_[0].path() { t.FailNow() }
  if "in1imp" != edge1.inputs_[1].path() { t.FailNow() }
  if "dd" != edge1.inputs_[2].path() { t.FailNow() }
  if 1u != edge1.implicit_deps_ { t.FailNow() }
  if 1u != edge1.order_only_deps_ { t.FailNow() }
  if !edge1.GetBindingBool("restat") { t.FailNow() }
  if edge1 != GetNode("out1imp").in_edge() { t.FailNow() }
  Node* in1imp = GetNode("in1imp")
  if 1u != in1imp.out_edges().size() { t.FailNow() }
  if edge1 != in1imp.out_edges()[0] { t.FailNow() }

  Edge* edge2 = GetNode("out2").in_edge()
  if 1u != edge2.outputs_.size() { t.FailNow() }
  if "out2" != edge2.outputs_[0].path() { t.FailNow() }
  if 0u != edge2.implicit_outs_ { t.FailNow() }
  if 3u != edge2.inputs_.size() { t.FailNow() }
  if "in2" != edge2.inputs_[0].path() { t.FailNow() }
  if "in2imp" != edge2.inputs_[1].path() { t.FailNow() }
  if "dd" != edge2.inputs_[2].path() { t.FailNow() }
  if 1u != edge2.implicit_deps_ { t.FailNow() }
  if 1u != edge2.order_only_deps_ { t.FailNow() }
  if edge2.GetBindingBool("restat") { t.FailNow() }
  Node* in2imp = GetNode("in2imp")
  if 1u != in2imp.out_edges().size() { t.FailNow() }
  if edge2 != in2imp.out_edges()[0] { t.FailNow() }
}

func TestGraphTest_DyndepFileMissing(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n" )

  err := ""
  if !scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "loading 'dd': No such file or directory" != err { t.FailNow() }
}

func TestGraphTest_DyndepFileError(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\n" )

  err := ""
  if !scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "'out' not mentioned in its dyndep file 'dd'" != err { t.FailNow() }
}

func TestGraphTest_DyndepImplicitInputNewer(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n" )
  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("in", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !GetNode("in").dirty() { t.FailNow() }
  if !GetNode("dd").dirty() { t.FailNow() }

  // "out" is dirty due to dyndep-specified implicit input
  if GetNode("out").dirty() { t.FailNow() }
}

func TestGraphTest_DyndepFileReady(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n" )
  fs_.Create("dd-in", "")
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n" )
  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("in", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !GetNode("in").dirty() { t.FailNow() }
  if !GetNode("dd").dirty() { t.FailNow() }
  if GetNode("dd").in_edge().outputs_ready() { t.FailNow() }

  // "out" is dirty due to dyndep-specified implicit input
  if GetNode("out").dirty() { t.FailNow() }
}

func TestGraphTest_DyndepFileNotClean(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "this-should-not-be-loaded")
  fs_.Tick()
  fs_.Create("dd-in", "")
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("dd").dirty() { t.FailNow() }
  if !GetNode("dd").in_edge().outputs_ready() { t.FailNow() }

  // "out" is clean but not ready since "dd" is not ready
  if !GetNode("out").dirty() { t.FailNow() }
  if !GetNode("out").in_edge().outputs_ready() { t.FailNow() }
}

func TestGraphTest_DyndepFileNotReady(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild tmp: r\nbuild dd: r dd-in || tmp\nbuild out: r || dd\n  dyndep = dd\n" )
  fs_.Create("dd", "this-should-not-be-loaded")
  fs_.Create("dd-in", "")
  fs_.Tick()
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !GetNode("dd").dirty() { t.FailNow() }
  if !GetNode("dd").in_edge().outputs_ready() { t.FailNow() }
  if !GetNode("out").dirty() { t.FailNow() }
  if !GetNode("out").in_edge().outputs_ready() { t.FailNow() }
}

func TestGraphTest_DyndepFileSecondNotReady(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild dd1: r dd1-in\nbuild dd2-in: r || dd1\n  dyndep = dd1\nbuild dd2: r dd2-in\nbuild out: r || dd2\n  dyndep = dd2\n" )
  fs_.Create("dd1", "")
  fs_.Create("dd2", "")
  fs_.Create("dd2-in", "")
  fs_.Tick()
  fs_.Create("dd1-in", "")
  fs_.Create("out", "")

  err := ""
  if scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if GetNode("dd1").dirty() { t.FailNow() }
  if !GetNode("dd1").in_edge().outputs_ready() { t.FailNow() }
  if !GetNode("dd2").dirty() { t.FailNow() }
  if !GetNode("dd2").in_edge().outputs_ready() { t.FailNow() }
  if !GetNode("out").dirty() { t.FailNow() }
  if !GetNode("out").in_edge().outputs_ready() { t.FailNow() }
}

func TestGraphTest_DyndepFileCircular(t *testing.T) {
  AssertParse(&state_, "rule r\n  command = unused\nbuild out: r in || dd\n  depfile = out.d\n  dyndep = dd\nbuild in: r circ\n" )
  fs_.Create("out.d", "out: inimp\n")
  fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\n" )
  fs_.Create("out", "")

  Edge* edge = GetNode("out").in_edge()
  err := ""
  if !scan_.RecomputeDirty(GetNode("out"), &err) { t.FailNow() }
  if "dependency cycle: circ . in . circ" != err { t.FailNow() }

  // Verify that "out.d" was loaded exactly once despite
  // circular reference discovered from dyndep file.
  if 3u != edge.inputs_.size() { t.FailNow() }
  if "in" != edge.inputs_[0].path() { t.FailNow() }
  if "inimp" != edge.inputs_[1].path() { t.FailNow() }
  if "dd" != edge.inputs_[2].path() { t.FailNow() }
  if 1u != edge.implicit_deps_ { t.FailNow() }
  if 1u != edge.order_only_deps_ { t.FailNow() }
}

// Check that phony's dependencies' mtimes are propagated.
func TestGraphTest_PhonyDepsMtimes(t *testing.T) {
  err := ""
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n command = touch $out\nbuild in_ph: phony in1\nbuild out1: touch in_ph\n" ))
  fs_.Create("in1", "")
  fs_.Create("out1", "")
  Node* out1 = GetNode("out1")
  Node* in1  = GetNode("in1")

  if scan_.RecomputeDirty(out1, &err) { t.FailNow() }
  if !out1.dirty() { t.FailNow() }

  // Get the mtime of out1
  if in1.Stat(&fs_, &err) { t.FailNow() }
  if out1.Stat(&fs_, &err) { t.FailNow() }
  out1Mtime1 := out1.mtime()
  in1Mtime1 := in1.mtime()

  // Touch in1. This should cause out1 to be dirty
  state_.Reset()
  fs_.Tick()
  fs_.Create("in1", "")

  if in1.Stat(&fs_, &err) { t.FailNow() }
  if in1.mtime() <= in1Mtime1 { t.FailNow() }

  if scan_.RecomputeDirty(out1, &err) { t.FailNow() }
  if in1.mtime() <= in1Mtime1 { t.FailNow() }
  if out1.mtime() != out1Mtime1 { t.FailNow() }
  if out1.dirty() { t.FailNow() }
}

