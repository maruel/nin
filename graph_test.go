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
	"runtime"
	"testing"
)

type GraphTest struct {
	StateTestWithBuiltinRules
	fs   VirtualFileSystem
	scan DependencyScan
}

func NewGraphTest(t *testing.T) GraphTest {
	g := GraphTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		fs:                        NewVirtualFileSystem(),
	}
	g.scan = NewDependencyScan(&g.state, nil, nil, &g.fs)
	return g
}

func TestGraphTest_MissingImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out: cat in | implicit\n", ParseManifestOpts{})
	g.fs.Create("in", "")
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	// A missing implicit dep *should* make the output dirty.
	// (In fact, a build will fail.)
	// This is a change from prior semantics of ninja.
	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ModifiedImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out: cat in | implicit\n", ParseManifestOpts{})
	g.fs.Create("in", "")
	g.fs.Create("out", "")
	g.fs.Tick()
	g.fs.Create("implicit", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	// A modified implicit dep should make the output dirty.
	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_FunkyMakefilePath(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild out.o: catdep foo.cc\n", ParseManifestOpts{})
	g.fs.Create("foo.cc", "")
	g.fs.Create("out.o.d", "out.o: ./foo/../implicit.h\n")
	g.fs.Create("out.o", "")
	g.fs.Tick()
	g.fs.Create("implicit.h", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}

	// implicit.h has changed, though our depfile refers to it with a
	// non-canonical path; we should still find it.
	if !g.GetNode("out.o").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ExplicitImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild implicit.h: cat data\nbuild out.o: catdep foo.cc || implicit.h\n", ParseManifestOpts{})
	g.fs.Create("implicit.h", "")
	g.fs.Create("foo.cc", "")
	g.fs.Create("out.o.d", "out.o: implicit.h\n")
	g.fs.Create("out.o", "")
	g.fs.Tick()
	g.fs.Create("data", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}

	// We have both an implicit and an explicit dep on implicit.h.
	// The implicit dep should "win" (in the sense that it should cause
	// the output to be dirty).
	if !g.GetNode("out.o").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputParse(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out | out.imp: cat in\n", ParseManifestOpts{})

	edge := g.GetNode("out").InEdge
	if 2 != len(edge.Outputs) {
		t.Fatal("expected equal")
	}
	if "out" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out.imp" != edge.Outputs[1].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge.ImplicitOuts {
		t.Fatal("expected equal")
	}
	if edge != g.GetNode("out.imp").InEdge {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_ImplicitOutputMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out | out.imp: cat in\n", ParseManifestOpts{})
	g.fs.Create("in", "")
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
	if !g.GetNode("out.imp").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOutOfDate(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out | out.imp: cat in\n", ParseManifestOpts{})
	g.fs.Create("out.imp", "")
	g.fs.Tick()
	g.fs.Create("in", "")
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
	if !g.GetNode("out.imp").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOnlyParse(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build | out.imp: cat in\n", ParseManifestOpts{})

	edge := g.GetNode("out.imp").InEdge
	if 1 != len(edge.Outputs) {
		t.Fatal("expected equal")
	}
	if "out.imp" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge.ImplicitOuts {
		t.Fatal("expected equal")
	}
	if edge != g.GetNode("out.imp").InEdge {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_ImplicitOutputOnlyMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build | out.imp: cat in\n", ParseManifestOpts{})
	g.fs.Create("in", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.imp"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("out.imp").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_ImplicitOutputOnlyOutOfDate(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build | out.imp: cat in\n", ParseManifestOpts{})
	g.fs.Create("out.imp", "")
	g.fs.Tick()
	g.fs.Create("in", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.imp"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("out.imp").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_PathWithCurrentDirectory(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ParseManifestOpts{})
	g.fs.Create("foo.cc", "")
	g.fs.Create("out.o.d", "out.o: foo.cc\n")
	g.fs.Create("out.o", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}

	if g.GetNode("out.o").Dirty {
		t.Fatal("expected false")
	}
}

func TestGraphTest_RootNodes(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out1: cat in1\nbuild mid1: cat in1\nbuild out2: cat mid1\nbuild out3 out4: cat mid1\n", ParseManifestOpts{})

	rootNodes := g.state.RootNodes()
	if 4 != len(rootNodes) {
		t.Fatal("expected equal")
	}
	for i := 0; i < len(rootNodes); i++ {
		name := rootNodes[i].Path
		if "out" != name[0:3] {
			t.Fatal("expected equal")
		}
	}
}

func TestGraphTest_VarInOutPathEscaping(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build a$ b: cat no'space with$ space$$ no\"space2\n", ParseManifestOpts{})

	edge := g.GetNode("a b").InEdge
	want := "cat 'no'\\''space' 'with space$' 'no\"space2' > 'a b'"
	if runtime.GOOS == "windows" {
		want = "cat no'space \"with space$\" \"no\\\"space2\" > \"a b\""
	}
	if got := edge.EvaluateCommand(false); want != got {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Regression test for https://github.com/ninja-build/ninja/issues/380
func TestGraphTest_DepfileWithCanonicalizablePath(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ParseManifestOpts{})
	g.fs.Create("foo.cc", "")
	g.fs.Create("out.o.d", "out.o: bar/../foo.cc\n")
	g.fs.Create("out.o", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}

	if g.GetNode("out.o").Dirty {
		t.Fatal("expected false")
	}
}

// Regression test for https://github.com/ninja-build/ninja/issues/404
func TestGraphTest_DepfileRemoved(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule catdep\n  depfile = $out.d\n  command = cat $in > $out\nbuild ./out.o: catdep ./foo.cc\n", ParseManifestOpts{})
	g.fs.Create("foo.h", "")
	g.fs.Create("foo.cc", "")
	g.fs.Tick()
	g.fs.Create("out.o.d", "out.o: foo.h\n")
	g.fs.Create("out.o", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}
	if g.GetNode("out.o").Dirty {
		t.Fatal("expected false")
	}

	g.state.Reset()
	g.fs.RemoveFile("out.o.d")
	if err := g.scan.RecomputeDirty(g.GetNode("out.o"), nil); err != nil {
		t.Fatal(err)
	}
	if !g.GetNode("out.o").Dirty {
		t.Fatal("expected true")
	}
}

// Check that rule-level variables are in scope for eval.
func TestGraphTest_RuleVariablesInScope(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n", ParseManifestOpts{})
	edge := g.GetNode("out").InEdge
	if got := edge.EvaluateCommand(false); got != "depfile is x" {
		t.Fatal(got)
	}
}

// Check that build statements can override rule builtins like depfile.
func TestGraphTest_DepfileOverride(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  depfile = x\n  command = unused\nbuild out: r in\n  depfile = y\n", ParseManifestOpts{})
	edge := g.GetNode("out").InEdge
	if "y" != edge.GetBinding("depfile") {
		t.Fatal("expected equal")
	}
}

// Check that overridden values show up in expansion of rule-level bindings.
func TestGraphTest_DepfileOverrideParent(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  depfile = x\n  command = depfile is $depfile\nbuild out: r in\n  depfile = y\n", ParseManifestOpts{})
	edge := g.GetNode("out").InEdge
	if "depfile is y" != edge.GetBinding("command") {
		t.Fatal("expected equal")
	}
}

// Verify that building a nested phony rule prints "no work to do"
func TestGraphTest_NestedPhonyPrintsDone(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build n1: phony \nbuild n2: phony n1\n", ParseManifestOpts{})
	if err := g.scan.RecomputeDirty(g.GetNode("n2"), nil); err != nil {
		t.Fatal(err)
	}

	plan := newPlan(nil)
	err := ""
	if !plan.addTarget(g.GetNode("n2"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if 0 != plan.commandEdges {
		t.Fatal("expected equal")
	}
	if plan.moreToDo() {
		t.Fatal("expected false")
	}
}

func TestGraphTest_PhonySelfReferenceError(t *testing.T) {
	g := NewGraphTest(t)
	var parserOpts ParseManifestOpts
	parserOpts.ErrOnPhonyCycle = true
	g.AssertParse(&g.state, "build a: phony a\n", parserOpts)

	if err := g.scan.RecomputeDirty(g.GetNode("a"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: a -> a [-w phonycycle=err]" {
		t.Fatal(err)
	}
}

func TestGraphTest_DependencyCycle(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out: cat mid\nbuild mid: cat in\nbuild in: cat pre\nbuild pre: cat out\n", ParseManifestOpts{})

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: out -> mid -> in -> pre -> out" {
		t.Fatal(err)
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes1(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build a b: cat a\n", ParseManifestOpts{})
	if err := g.scan.RecomputeDirty(g.GetNode("b"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: a -> a" {
		t.Fatal(err)
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes2(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build b a: cat a\n", ParseManifestOpts{})
	if err := g.scan.RecomputeDirty(g.GetNode("b"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: a -> a" {
		t.Fatal(err)
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes3(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build a b: cat c\nbuild c: cat a\n", ParseManifestOpts{})
	if err := g.scan.RecomputeDirty(g.GetNode("b"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: a -> c -> a" {
		t.Fatal(err)
	}
}

func TestGraphTest_CycleInEdgesButNotInNodes4(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build d: cat c\nbuild c: cat b\nbuild b: cat a\nbuild a e: cat d\nbuild f: cat e\n", ParseManifestOpts{})
	if err := g.scan.RecomputeDirty(g.GetNode("f"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: a -> d -> c -> b -> a" {
		t.Fatal(err)
	}
}

// Verify that cycles in graphs with multiple outputs are handled correctly
// in RecomputeDirty() and don't cause deps to be loaded multiple times.
func TestGraphTest_CycleWithLengthZeroFromDepfile(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule deprule\n   depfile = dep.d\n   command = unused\nbuild a b: deprule\n", ParseManifestOpts{})
	g.fs.Create("dep.d", "a: b\n")

	if err := g.scan.RecomputeDirty(g.GetNode("a"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: b -> b" {
		t.Fatal(err)
	}

	// Despite the depfile causing edge to be a cycle (it has outputs a and b,
	// but the depfile also adds b as an input), the deps should have been loaded
	// only once:
	edge := g.GetNode("a").InEdge
	if 1 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if "b" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
}

// Like CycleWithLengthZeroFromDepfile but with a higher cycle length.
func TestGraphTest_CycleWithLengthOneFromDepfile(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\n", ParseManifestOpts{})
	g.fs.Create("dep.d", "a: c\n")

	if err := g.scan.RecomputeDirty(g.GetNode("a"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: b -> c -> b" {
		t.Fatal(err)
	}

	// Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
	// but c's in_edge has b as input but the depfile also adds |edge| as
	// output)), the deps should have been loaded only once:
	edge := g.GetNode("a").InEdge
	if 1 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if "c" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
}

// Like CycleWithLengthOneFromDepfile but building a node one hop away from
// the cycle.
func TestGraphTest_CycleWithLengthOneFromDepfileOneHopAway(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule deprule\n   depfile = dep.d\n   command = unused\nrule r\n   command = unused\nbuild a b: deprule\nbuild c: r b\nbuild d: r a\n", ParseManifestOpts{})
	g.fs.Create("dep.d", "a: c\n")

	if err := g.scan.RecomputeDirty(g.GetNode("d"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: b -> c -> b" {
		t.Fatal(err)
	}

	// Despite the depfile causing edge to be a cycle (|edge| has outputs a and b,
	// but c's in_edge has b as input but the depfile also adds |edge| as
	// output)), the deps should have been loaded only once:
	edge := g.GetNode("a").InEdge
	if 1 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if "c" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_Decanonicalize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out\\out1: cat src\\in1\nbuild out\\out2/out3\\out4: cat mid1\nbuild out3 out4\\foo: cat mid1\n", ParseManifestOpts{})

	rootNodes := g.state.RootNodes()
	if 4 != len(rootNodes) {
		t.Fatal("expected equal")
	}
	if rootNodes[0].Path != "out/out1" {
		t.Fatal("expected equal")
	}
	if rootNodes[1].Path != "out/out2/out3/out4" {
		t.Fatal("expected equal")
	}
	if rootNodes[2].Path != "out3" {
		t.Fatal("expected equal")
	}
	if rootNodes[3].Path != "out4/foo" {
		t.Fatal("expected equal")
	}
	if rootNodes[0].PathDecanonicalized() != "out\\out1" {
		t.Fatal("expected equal")
	}
	if rootNodes[1].PathDecanonicalized() != "out\\out2/out3\\out4" {
		t.Fatal("expected equal")
	}
	if rootNodes[2].PathDecanonicalized() != "out3" {
		t.Fatal("expected equal")
	}
	if rootNodes[3].PathDecanonicalized() != "out4\\foo" {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepLoadTrivial(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err != nil {
		t.Fatal(err)
	}
	if g.GetNode("dd").DyndepPending {
		t.Fatal("expected false")
	}

	edge := g.GetNode("out").InEdge
	if 1 != len(edge.Outputs) {
		t.Fatal("expected equal")
	}
	if "out" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if 2 != len(edge.Inputs) {
		t.Fatal(len(edge.Inputs))
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "dd" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if 0 != edge.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
	if edge.GetBinding("restat") != "" {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepLoadImplicit(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out1: r in || dd\n  dyndep = dd\nbuild out2: r in\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out1: dyndep | out2\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err != nil {
		t.Fatal(err)
	}
	if g.GetNode("dd").DyndepPending {
		t.Fatal("expected false")
	}

	edge := g.GetNode("out1").InEdge
	if 1 != len(edge.Outputs) {
		t.Fatal("expected equal")
	}
	if "out1" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if 3 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out2" != edge.Inputs[1].Path {
		t.Fatal(edge.Inputs[1].Path)
	}
	if "dd" != edge.Inputs[2].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
	if edge.GetBinding("restat") != "" {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepLoadMissingFile(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ParseManifestOpts{})

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "loading 'dd': file does not exist" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepLoadMissingEntry(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "'out' not mentioned in its dyndep file 'dd'" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepLoadExtraEntry(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\nbuild out2: r in || dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild out2: dyndep\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dyndep file 'dd' mentions output 'out2' whose build statement does not have a dyndep binding for the file" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepLoadOutputWithMultipleRules1(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out1 | out-twice.imp: r in1\nbuild out2: r in2 || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "multiple rules generate out-twice.imp" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepLoadOutputWithMultipleRules2(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out1: r in1 || dd1\n  dyndep = dd1\nbuild out2: r in2 || dd2\n  dyndep = dd2\n", ParseManifestOpts{})
	g.fs.Create("dd1", "ninja_dyndep_version = 1\nbuild out1 | out-twice.imp: dyndep\n")
	g.fs.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")

	if !g.GetNode("dd1").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd1"), DyndepFile{}); err != nil {
		t.Fatal(err)
	}
	if !g.GetNode("dd2").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd2"), DyndepFile{}); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "multiple rules generate out-twice.imp" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepLoadMultiple(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out1: r in1 || dd\n  dyndep = dd\nbuild out2: r in2 || dd\n  dyndep = dd\nbuild outNot: r in3 || dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out1 | out1imp: dyndep | in1imp\nbuild out2: dyndep | in2imp\n  restat = 1\n")

	if !g.GetNode("dd").DyndepPending {
		t.Fatal("expected true")
	}
	if err := g.scan.LoadDyndeps(g.GetNode("dd"), DyndepFile{}); err != nil {
		t.Fatal(err)
	}
	if g.GetNode("dd").DyndepPending {
		t.Fatal("expected false")
	}
	edge1 := g.GetNode("out1").InEdge
	if 2 != len(edge1.Outputs) {
		t.Fatal("expected equal")
	}
	if "out1" != edge1.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out1imp" != edge1.Outputs[1].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge1.ImplicitOuts {
		t.Fatal("expected equal")
	}
	if 3 != len(edge1.Inputs) {
		t.Fatal("expected equal")
	}
	if "in1" != edge1.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "in1imp" != edge1.Inputs[1].Path {
		t.Fatal(edge1.Inputs[1].Path)
	}
	if "dd" != edge1.Inputs[2].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge1.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge1.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
	if edge1.GetBinding("restat") != "" {
		t.Fatal("expected false")
	}
	if edge1 != g.GetNode("out1imp").InEdge {
		t.Fatal("expected equal")
	}
	in1imp := g.GetNode("in1imp")
	if 1 != len(in1imp.OutEdges) {
		t.Fatal("expected equal")
	}
	if edge1 != in1imp.OutEdges[0] {
		t.Fatal("expected equal")
	}

	edge2 := g.GetNode("out2").InEdge
	if 1 != len(edge2.Outputs) {
		t.Fatal("expected equal")
	}
	if "out2" != edge2.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if 0 != edge2.ImplicitOuts {
		t.Fatal("expected equal")
	}
	if 3 != len(edge2.Inputs) {
		t.Fatal("expected equal")
	}
	if "in2" != edge2.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "in2imp" != edge2.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "dd" != edge2.Inputs[2].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge2.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge2.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
	if edge2.GetBinding("restat") == "" {
		t.Fatal("expected true")
	}
	in2imp := g.GetNode("in2imp")
	if 1 != len(in2imp.OutEdges) {
		t.Fatal("expected equal")
	}
	if edge2 != in2imp.OutEdges[0] {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_DyndepFileMissing(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "loading 'dd': file does not exist" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepFileError(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\n")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "'out' not mentioned in its dyndep file 'dd'" {
		t.Fatal(err)
	}
}

func TestGraphTest_DyndepImplicitInputNewer(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	g.fs.Create("out", "")
	g.fs.Tick()
	g.fs.Create("in", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if g.GetNode("in").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").Dirty {
		t.Fatal("expected false")
	}

	// "out" is dirty due to dyndep-specified implicit input
	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_DyndepFileReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd-in", "")
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	g.fs.Create("out", "")
	g.fs.Tick()
	g.fs.Create("in", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if g.GetNode("in").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").Dirty {
		t.Fatal("expected false")
	}
	if !g.GetNode("dd").InEdge.OutputsReady {
		t.Fatal("expected true")
	}

	// "out" is dirty due to dyndep-specified implicit input
	if !g.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
}

func TestGraphTest_DyndepFileNotClean(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild dd: r dd-in\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "this-should-not-be-loaded")
	g.fs.Tick()
	g.fs.Create("dd-in", "")
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("dd").Dirty {
		t.Fatal("expected true")
	}
	if g.GetNode("dd").InEdge.OutputsReady {
		t.Fatal("expected false")
	}

	// "out" is clean but not ready since "dd" is not ready
	if g.GetNode("out").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("out").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileNotReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild tmp: r\nbuild dd: r dd-in || tmp\nbuild out: r || dd\n  dyndep = dd\n", ParseManifestOpts{})
	g.fs.Create("dd", "this-should-not-be-loaded")
	g.fs.Create("dd-in", "")
	g.fs.Tick()
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if g.GetNode("dd").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("dd").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
	if g.GetNode("out").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("out").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileSecondNotReady(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild dd1: r dd1-in\nbuild dd2-in: r || dd1\n  dyndep = dd1\nbuild dd2: r dd2-in\nbuild out: r || dd2\n  dyndep = dd2\n", ParseManifestOpts{})
	g.fs.Create("dd1", "")
	g.fs.Create("dd2", "")
	g.fs.Create("dd2-in", "")
	g.fs.Tick()
	g.fs.Create("dd1-in", "")
	g.fs.Create("out", "")

	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err != nil {
		t.Fatal(err)
	}

	if !g.GetNode("dd1").Dirty {
		t.Fatal("expected true")
	}
	if g.GetNode("dd1").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
	if g.GetNode("dd2").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("dd2").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
	if g.GetNode("out").Dirty {
		t.Fatal("expected false")
	}
	if g.GetNode("out").InEdge.OutputsReady {
		t.Fatal("expected false")
	}
}

func TestGraphTest_DyndepFileCircular(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule r\n  command = unused\nbuild out: r in || dd\n  depfile = out.d\n  dyndep = dd\nbuild in: r circ\n", ParseManifestOpts{})
	g.fs.Create("out.d", "out: inimp\n")
	g.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\n")
	g.fs.Create("out", "")

	edge := g.GetNode("out").InEdge
	if err := g.scan.RecomputeDirty(g.GetNode("out"), nil); err == nil {
		t.Fatal("expected false")
	} else if err.Error() != "dependency cycle: circ -> in -> circ" {
		t.Fatal("expected equal")
	}

	// Verify that "out.d" was loaded exactly once despite
	// circular reference discovered from dyndep file.
	if 3 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "inimp" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "dd" != edge.Inputs[2].Path {
		t.Fatal("expected equal")
	}
	if 1 != edge.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
}

func TestGraphTest_Validation(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "build out: cat in |@ validate\nbuild validate: cat in\n", ParseManifestOpts{})

	g.fs.Create("in", "")
	var validationNodes []*Node
	if err := g.scan.RecomputeDirty(g.GetNode("out"), &validationNodes); err != nil {
		t.Fatal(err)
	}

	if len(validationNodes) != 1 || validationNodes[0].Path != "validate" {
		t.Fatal(validationNodes)
	}

	if !g.GetNode("out").Dirty {
		t.Fatal("expected dirty")
	}
	if !g.GetNode("validate").Dirty {
		t.Fatal("expected dirty")
	}
}

// Check that phony's dependencies' mtimes are propagated.
func TestGraphTest_PhonyDepsMtimes(t *testing.T) {
	g := NewGraphTest(t)
	g.AssertParse(&g.state, "rule touch\n command = touch $out\nbuild in_ph: phony in1\nbuild out1: touch in_ph\n", ParseManifestOpts{})
	g.fs.Create("in1", "")
	g.fs.Create("out1", "")
	out1 := g.GetNode("out1")
	in1 := g.GetNode("in1")

	if err := g.scan.RecomputeDirty(out1, nil); err != nil {
		t.Fatal(err)
	}
	if out1.Dirty {
		t.Fatal("expected true")
	}

	// Get the mtime of out1
	if err := in1.Stat(&g.fs); err != nil {
		t.Fatal(err)
	}
	if err := out1.Stat(&g.fs); err != nil {
		t.Fatal(err)
	}
	out1Mtime1 := out1.MTime
	in1Mtime1 := in1.MTime

	// Touch in1. This should cause out1 to be dirty
	g.state.Reset()
	g.fs.Tick()
	g.fs.Create("in1", "")

	if err := in1.Stat(&g.fs); err != nil {
		t.Fatal(err)
	}
	if in1.MTime <= in1Mtime1 {
		t.Fatal("expected greater")
	}

	if err := g.scan.RecomputeDirty(out1, nil); err != nil {
		t.Fatal(err)
	}
	if in1.MTime <= in1Mtime1 {
		t.Fatal("expected greater")
	}
	if out1.MTime != out1Mtime1 {
		t.Fatal("expected equal")
	}
	if !out1.Dirty {
		t.Fatal("expected true")
	}
}
