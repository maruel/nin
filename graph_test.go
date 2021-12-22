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

import "testing"

type GraphTest struct {
	StateTestWithBuiltinRules
	fs_   VirtualFileSystem
	scan_ DependencyScan
}

func NewGraphTest(t *testing.T) GraphTest {
	g := GraphTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		fs_:                       NewVirtualFileSystem(),
	}
	g.scan_ = NewDependencyScan(&g.state_, nil, nil, &g.fs_, nil)
	return g
}

func TestGraphTest_MissingImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out: cat in | implicit\n", ManifestParserOptions{})
	g.fs_.Create("in", "")
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// A missing implicit dep *should* make the output dirty.
	// (In fact, a build will fail.)
	// This is a change from prior semantics of ninja.
	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ModifiedImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out: cat in | implicit\n", ManifestParserOptions{})
	g.fs_.Create("in", "")
	g.fs_.Create("out", "")
	g.fs_.Tick()
	g.fs_.Create("implicit", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// A modified implicit dep should make the output dirty.
	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_FunkyMakefilePath(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild out.o: catdep foo.cc\n", ManifestParserOptions{})
	g.fs_.Create("foo.cc", "")
	g.fs_.Create("out.o.d", "out.o: ./foo/../implicit.h\n")
	g.fs_.Create("out.o", "")
	g.fs_.Tick()
	g.fs_.Create("implicit.h", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}

	// implicit.h has changed, though our depfile refers to it with a
	// non-canonical path; we should still find it.
	if !g.GetNode("out.o").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ExplicitImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild implicit.h: cat data\nbuild out.o: catdep foo.cc || implicit.h\n", ManifestParserOptions{})
	g.fs_.Create("implicit.h", "")
	g.fs_.Create("foo.cc", "")
	g.fs_.Create("out.o.d", "out.o: implicit.h\n")
	g.fs_.Create("out.o", "")
	g.fs_.Tick()
	g.fs_.Create("data", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// We have both an implicit and an explicit dep on implicit.h.
	// The implicit dep should "win" (in the sense that it should cause
	// the output to be dirty).
	if !g.GetNode("out.o").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputParse(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out | out.imp: cat in\n", ManifestParserOptions{})

	edge := g.GetNode("out").in_edge()
	if 2 != len(edge.outputs_) {
		t.Fatal("expected equal")
	}
	if "out" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out.imp" != edge.outputs_[1].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge.implicit_outs_ {
		t.Fatal("expected equal")
	}
	if edge != g.GetNode("out.imp").in_edge() {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_ImplicitOutputMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out | out.imp: cat in\n", ManifestParserOptions{})
	g.fs_.Create("in", "")
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
	if !g.GetNode("out.imp").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOutOfDate(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out | out.imp: cat in\n", ManifestParserOptions{})
	g.fs_.Create("out.imp", "")
	g.fs_.Tick()
	g.fs_.Create("in", "")
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
	if !g.GetNode("out.imp").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOnlyParse(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build | out.imp: cat in\n", ManifestParserOptions{})

	edge := g.GetNode("out.imp").in_edge()
	if 1 != len(edge.outputs_) {
		t.Fatal("expected equal")
	}
	if "out.imp" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge.implicit_outs_ {
		t.Fatal("expected equal")
	}
	if edge != g.GetNode("out.imp").in_edge() {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_ImplicitOutputOnlyMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build | out.imp: cat in\n", ManifestParserOptions{})
	g.fs_.Create("in", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.imp"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("out.imp").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOnlyOutOfDate(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build | out.imp: cat in\n", ManifestParserOptions{})
	g.fs_.Create("out.imp", "")
	g.fs_.Tick()
	g.fs_.Create("in", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.imp"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("out.imp").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_PathWithCurrentDirectory(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ManifestParserOptions{})
	g.fs_.Create("foo.cc", "")
	g.fs_.Create("out.o.d", "out.o: foo.cc\n")
	g.fs_.Create("out.o", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if g.GetNode("out.o").dirty() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_RootNodes(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out1: cat in1\nbuild mid1: cat in1\nbuild out2: cat mid1\nbuild out3 out4: cat mid1\n", ManifestParserOptions{})

	err := ""
	root_nodes := g.state_.RootNodes(&err)
	if 4 != len(root_nodes) {
		t.Fatal("expected equal")
	}
	for i := 0; i < len(root_nodes); i++ {
		name := root_nodes[i].path()
		if "out" != name[0:3] {
			t.Fatal("expected equal")
		}
	}
}

func TestGraphTest_VarInOutPathEscaping(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build a$ b: cat no'space with$ space$$ no\"space2\n", ManifestParserOptions{})

	edge := g.GetNode("a b").in_edge()
	want := "cat no'space \"with space$\" \"no\\\"space2\" > \"a b\""
	if got := edge.EvaluateCommand(false); want != got {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Regression test for https://github.com/ninja-build/ninja/issues/380
func TestGraphTest_DepfileWithCanonicalizablePath(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ManifestParserOptions{})
	g.fs_.Create("foo.cc", "")
	g.fs_.Create("out.o.d", "out.o: bar/../foo.cc\n")
	g.fs_.Create("out.o", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if g.GetNode("out.o").dirty() {
		t.Fatal("expected false")
	}
}

// Regression test for https://github.com/ninja-build/ninja/issues/404
func TestGraphTest_DepfileRemoved(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ManifestParserOptions{})
	g.fs_.Create("foo.h", "")
	g.fs_.Create("foo.cc", "")
	g.fs_.Tick()
	g.fs_.Create("out.o.d", "out.o: foo.h\n")
	g.fs_.Create("out.o", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	if g.GetNode("out.o").dirty() {
		t.Fatal("expected false")
	}

	g.state_.Reset()
	g.fs_.RemoveFile("out.o.d")
	if !g.scan_.RecomputeDirty(g.GetNode("out.o"), nil, &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	if !g.GetNode("out.o").dirty() {
		t.Fatal("expected true")
	}
}

// Check that rule-level variables are in scope for eval.
func TestGraphTest_RuleVariablesInScope(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n", ManifestParserOptions{})
	edge := g.GetNode("out").in_edge()
	if "depfile is x" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

// Check that build statements can override rule builtins like depfile.
func TestGraphTest_DepfileOverride(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  depfile = x\n  command = unused\nbuild out: r in\n  depfile = y\n", ManifestParserOptions{})
	edge := g.GetNode("out").in_edge()
	if "y" != edge.GetBinding("depfile") {
		t.Fatal("expected equal")
	}
}

// Check that overridden values show up in expansion of rule-level bindings.
func TestGraphTest_DepfileOverrideParent(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n  depfile = y\n", ManifestParserOptions{})
	edge := g.GetNode("out").in_edge()
	if "depfile is y" != edge.GetBinding("command") {
		t.Fatal("expected equal")
	}
}

// Verify that building a nested phony rule prints "no work to do"
func TestGraphTest_NestedPhonyPrintsDone(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build n1: phony \nbuild n2: phony n1\n", ManifestParserOptions{})
	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("n2"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	var plan_ Plan
	if !plan_.AddTarget(g.GetNode("n2"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if 0 != plan_.command_edge_count() {
		t.Fatal("expected equal")
	}
	if plan_.more_to_do() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_PhonySelfReferenceError(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	var parser_opts ManifestParserOptions
	parser_opts.phony_cycle_action_ = kPhonyCycleActionError
	g.AssertParse(&g.state_, "build a: phony a\n", parser_opts)

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("a"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: a -> a [-w phonycycle=err]" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DependencyCycle(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out: cat mid\nbuild mid: cat in\nbuild in: cat pre\nbuild pre: cat out\n", ManifestParserOptions{})

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: out -> mid -> in -> pre -> out" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes1(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	err := ""
	g.AssertParse(&g.state_, "build a b: cat a\n", ManifestParserOptions{})
	if g.scan_.RecomputeDirty(g.GetNode("b"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: a -> a" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes2(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	err := ""
	g.AssertParse(&g.state_, "build b a: cat a\n", ManifestParserOptions{})
	if g.scan_.RecomputeDirty(g.GetNode("b"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: a -> a" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes3(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	err := ""
	g.AssertParse(&g.state_, "build a b: cat c\nbuild c: cat a\n", ManifestParserOptions{})
	if g.scan_.RecomputeDirty(g.GetNode("b"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: a -> c -> a" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes4(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	err := ""
	g.AssertParse(&g.state_, "build d: cat c\nbuild c: cat b\nbuild b: cat a\nbuild a e: cat d\nbuild f: cat e\n", ManifestParserOptions{})
	if g.scan_.RecomputeDirty(g.GetNode("f"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: a -> d -> c -> b -> a" != err {
		t.Fatal("expected equal")
	}
}

// Verify that cycles in graphs with multiple outputs are handled correctly
// in RecomputeDirty() and don't cause deps to be loaded multiple times.
func TestGraphTest_CycleWithLengthZeroFromDepfile(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule deprule\n   depfile = dep.d\n   command = unused\nbuild a b: deprule\n", ManifestParserOptions{})
	g.fs_.Create("dep.d", "a: b\n")

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("a"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: b -> b" != err {
		t.Fatal("expected equal")
	}

	// Despite the depfile causing edge to be a cycle (it has outputs a and b,
	// but the depfile also adds b as an input), the deps should have been loaded
	// only once:
	edge := g.GetNode("a").in_edge()
	if 1 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "b" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
}

// Like CycleWithLengthZeroFromDepfile but with a higher cycle length.
func TestGraphTest_CycleWithLengthOneFromDepfile(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\n", ManifestParserOptions{})
	g.fs_.Create("dep.d", "a: c\n")

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("a"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: b -> c -> b" != err {
		t.Fatal("expected equal")
	}

	// Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
	// but c's in_edge has b as input but the depfile also adds |edge| as
	// output)), the deps should have been loaded only once:
	edge := g.GetNode("a").in_edge()
	if 1 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "c" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
}

// Like CycleWithLengthOneFromDepfile but building a node one hop away from
// the cycle.
func TestGraphTest_CycleWithLengthOneFromDepfileOneHopAway(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\nbuild d: r a\n", ManifestParserOptions{})
	g.fs_.Create("dep.d", "a: c\n")

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("d"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: b -> c -> b" != err {
		t.Fatal("expected equal")
	}

	// Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
	// but c's in_edge has b as input but the depfile also adds |edge| as
	// output)), the deps should have been loaded only once:
	edge := g.GetNode("a").in_edge()
	if 1 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "c" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_Decanonicalize(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "build out\\out1: cat src\\in1\nbuild out\\out2/out3\\out4: cat mid1\nbuild out3 out4\\foo: cat mid1\n", ManifestParserOptions{})

	err := ""
	root_nodes := g.state_.RootNodes(&err)
	if 4 != len(root_nodes) {
		t.Fatal("expected equal")
	}
	if root_nodes[0].path() != "out/out1" {
		t.Fatal("expected equal")
	}
	if root_nodes[1].path() != "out/out2/out3/out4" {
		t.Fatal("expected equal")
	}
	if root_nodes[2].path() != "out3" {
		t.Fatal("expected equal")
	}
	if root_nodes[3].path() != "out4/foo" {
		t.Fatal("expected equal")
	}
	if root_nodes[0].PathDecanonicalized() != "out\\out1" {
		t.Fatal("expected equal")
	}
	if root_nodes[1].PathDecanonicalized() != "out\\out2/out3\\out4" {
		t.Fatal("expected equal")
	}
	if root_nodes[2].PathDecanonicalized() != "out3" {
		t.Fatal("expected equal")
	}
	if root_nodes[3].PathDecanonicalized() != "out4\\foo" {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadTrivial(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if !g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected false")
	}

	edge := g.GetNode("out").in_edge()
	if 1 != len(edge.outputs_) {
		t.Fatal("expected equal")
	}
	if "out" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if 2 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "dd" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if 0 != edge.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge.order_only_deps_ {
		t.Fatal("expected equal")
	}
	if edge.GetBindingBool("restat") {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepLoadImplicit(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out1: r in || dd\n  dyndep = dd\nbuild out2: r in\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out1: dyndep | out2\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if !g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected false")
	}

	edge := g.GetNode("out1").in_edge()
	if 1 != len(edge.outputs_) {
		t.Fatal("expected equal")
	}
	if "out1" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if 3 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out2" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "dd" != edge.inputs_[2].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge.order_only_deps_ {
		t.Fatal("expected equal")
	}
	if edge.GetBindingBool("restat") {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepLoadMissingFile(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ManifestParserOptions{})

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected false")
	}
	if "loading 'dd': No such file or directory" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadMissingEntry(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected false")
	}
	if "'out' not mentioned in its dyndep file 'dd'" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadExtraEntry(t *testing.T) {
	t.Skip("TODO")
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\nbuild out2: r in || dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild out2: dyndep\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected false")
	}
	if "dyndep file 'dd' mentions output 'out2' whose build statement does not have a dyndep binding for the file" != err {
		t.Fatal("expected equal")
	}
}

/*
func TestGraphTest_DyndepLoadOutputWithMultipleRules1(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out1 | out-twice.imp: r in1\nbuild out2: r in2 || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected false")
	}
	if "multiple rules generate out-twice.imp" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadOutputWithMultipleRules2(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out1: r in1 || dd1\n  dyndep = dd1\nbuild out2: r in2 || dd2\n  dyndep = dd2\n", ManifestParserOptions{})
	g.fs_.Create("dd1", "ninja_dyndep_version = 1\nbuild out1 | out-twice.imp: dyndep\n")
	g.fs_.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")

	err := ""
	if !g.GetNode("dd1").dyndep_pending() {
		t.Fatal("expected true")
	}
	if !g.scan_.LoadDyndeps(g.GetNode("dd1"), DyndepFile{}, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !g.GetNode("dd2").dyndep_pending() {
		t.Fatal("expected true")
	}
	if g.scan_.LoadDyndeps(g.GetNode("dd2"), DyndepFile{}, &err) {
		t.Fatal("expected false")
	}
	if "multiple rules generate out-twice.imp" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadMultiple(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out1: r in1 || dd\n  dyndep = dd\nbuild out2: r in2 || dd\n  dyndep = dd\nbuild outNot: r in3 || dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out1 | out1imp: dyndep | in1imp\nbuild out2: dyndep | in2imp\n  restat = 1\n")

	err := ""
	if !g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected true")
	}
	if !g.scan_.LoadDyndeps(g.GetNode("dd"), DyndepFile{}, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if g.GetNode("dd").dyndep_pending() {
		t.Fatal("expected false")
	}

	edge1 := g.GetNode("out1").in_edge()
	if 2 != len(edge1.outputs_) {
		t.Fatal("expected equal")
	}
	if "out1" != edge1.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out1imp" != edge1.outputs_[1].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge1.implicit_outs_ {
		t.Fatal("expected equal")
	}
	if 3 != len(edge1.inputs_) {
		t.Fatal("expected equal")
	}
	if "in1" != edge1.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "in1imp" != edge1.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "dd" != edge1.inputs_[2].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge1.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge1.order_only_deps_ {
		t.Fatal("expected equal")
	}
	if edge1.GetBindingBool("restat") {
		t.Fatal("expected false")
	}
	if edge1 != g.GetNode("out1imp").in_edge() {
		t.Fatal("expected equal")
	}
	in1imp := g.GetNode("in1imp")
	if 1 != len(in1imp.out_edges()) {
		t.Fatal("expected equal")
	}
	if edge1 != in1imp.out_edges()[0] {
		t.Fatal("expected equal")
	}

	edge2 := g.GetNode("out2").in_edge()
	if 1 != len(edge2.outputs_) {
		t.Fatal("expected equal")
	}
	if "out2" != edge2.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if 0 != edge2.implicit_outs_ {
		t.Fatal("expected equal")
	}
	if 3 != len(edge2.inputs_) {
		t.Fatal("expected equal")
	}
	if "in2" != edge2.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "in2imp" != edge2.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "dd" != edge2.inputs_[2].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge2.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge2.order_only_deps_ {
		t.Fatal("expected equal")
	}
	if !edge2.GetBindingBool("restat") {
		t.Fatal("expected true")
	}
	in2imp := g.GetNode("in2imp")
	if 1 != len(in2imp.out_edges()) {
		t.Fatal("expected equal")
	}
	if edge2 != in2imp.out_edges()[0] {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepFileMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected false")
	}
	if "loading 'dd': No such file or directory" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepFileError(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\n")

	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected false")
	}
	if "'out' not mentioned in its dyndep file 'dd'" != err {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepImplicitInputNewer(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	g.fs_.Create("out", "")
	g.fs_.Tick()
	g.fs_.Create("in", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if g.GetNode("in").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").dirty() {
		t.Fatal("expected false")
	}

	// "out" is dirty due to dyndep-specified implicit input
	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_DyndepFileReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd-in", "")
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	g.fs_.Create("out", "")
	g.fs_.Tick()
	g.fs_.Create("in", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if g.GetNode("in").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").dirty() {
		t.Fatal("expected false")
	}
	if !g.GetNode("dd").in_edge().outputs_ready() {
		t.Fatal("expected true")
	}

	// "out" is dirty due to dyndep-specified implicit input
	if !g.GetNode("out").dirty() {
		t.Fatal("expected true")
	}
}

func TestGraphTest_DyndepFileNotClean(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "this-should-not-be-loaded")
	g.fs_.Tick()
	g.fs_.Create("dd-in", "")
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("dd").dirty() {
		t.Fatal("expected true")
	}
	if g.GetNode("dd").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}

	// "out" is clean but not ready since "dd" is not ready
	if g.GetNode("out").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("out").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileNotReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild tmp: r\nbuild dd: r dd-in || tmp\nbuild out: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	g.fs_.Create("dd", "this-should-not-be-loaded")
	g.fs_.Create("dd-in", "")
	g.fs_.Tick()
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if g.GetNode("dd").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
	if g.GetNode("out").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("out").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileSecondNotReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild dd1: r dd1-in\nbuild dd2-in: r || dd1\n  dyndep = dd1\nbuild dd2: r dd2-in\nbuild out: r || dd2\n  dyndep = dd2\n", ManifestParserOptions{})
	g.fs_.Create("dd1", "")
	g.fs_.Create("dd2", "")
	g.fs_.Create("dd2-in", "")
	g.fs_.Tick()
	g.fs_.Create("dd1-in", "")
	g.fs_.Create("out", "")

	err := ""
	if !g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !g.GetNode("dd1").dirty() {
		t.Fatal("expected true")
	}
	if g.GetNode("dd1").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
	if g.GetNode("dd2").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("dd2").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
	if g.GetNode("out").dirty() {
		t.Fatal("expected false")
	}
	if g.GetNode("out").in_edge().outputs_ready() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileCircular(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  depfile = out.d\n  dyndep = dd\nbuild in: r circ\n", ManifestParserOptions{})
	g.fs_.Create("out.d", "out: inimp\n")
	g.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\n")
	g.fs_.Create("out", "")

	edge := g.GetNode("out").in_edge()
	err := ""
	if g.scan_.RecomputeDirty(g.GetNode("out"), nil, &err) {
		t.Fatal("expected false")
	}
	if "dependency cycle: circ -> in -> circ" != err {
		t.Fatal("expected equal")
	}

	// Verify that "out.d" was loaded exactly once despite
	// circular reference discovered from dyndep file.
	if 3 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "inimp" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "dd" != edge.inputs_[2].path() {
		t.Fatal("expected equal")
	}
	if 1 != edge.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge.order_only_deps_ {
		t.Fatal("expected equal")
	}
}

// Check that phony's dependencies' mtimes are propagated.
func TestGraphTest_PhonyDepsMtimes(t *testing.T) {
	g := NewGraphTest(t)
	err := ""
	g.AssertParse(&g.state_, "rule touch\n command = touch $out\nbuild in_ph: phony in1\nbuild out1: touch in_ph\n", ManifestParserOptions{})
	g.fs_.Create("in1", "")
	g.fs_.Create("out1", "")
	out1 := g.GetNode("out1")
	in1 := g.GetNode("in1")

	if !g.scan_.RecomputeDirty(out1, nil, &err) {
		t.Fatal("expected true")
	}
	if !!out1.dirty() {
		t.Fatal("expected true")
	}

	// Get the mtime of out1
	if !in1.Stat(&g.fs_, &err) {
		t.Fatal("expected true")
	}
	if !out1.Stat(&g.fs_, &err) {
		t.Fatal("expected true")
	}
	out1Mtime1 := out1.mtime()
	in1Mtime1 := in1.mtime()

	// Touch in1. This should cause out1 to be dirty
	g.state_.Reset()
	g.fs_.Tick()
	g.fs_.Create("in1", "")

	if !in1.Stat(&g.fs_, &err) {
		t.Fatal("expected true")
	}
	if in1.mtime() <= in1Mtime1 {
		t.Fatal("expected greater")
	}

	if !g.scan_.RecomputeDirty(out1, nil, &err) {
		t.Fatal("expected true")
	}
	if in1.mtime() <= in1Mtime1 {
		t.Fatal("expected greater")
	}
	if out1.mtime() != out1Mtime1 {
		t.Fatal("expected equal")
	}
	if !out1.dirty() {
		t.Fatal("expected true")
	}
}
*/
