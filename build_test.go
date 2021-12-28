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
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Fixture for tests involving Plan.
// Though Plan doesn't use State, it's useful to have one around
// to create Nodes and Edges.
type PlanTest struct {
	StateTestWithBuiltinRules
	plan_ Plan
}

func NewPlanTest(t *testing.T) *PlanTest {
	return &PlanTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		plan_:                     NewPlan(nil),
	}
}

// Because FindWork does not return Edges in any sort of predictable order,
// provide a means to get available Edges in order and in a format which is
// easy to write tests around.
func (p *PlanTest) FindWorkSorted(count int) []*Edge {
	var out []*Edge
	for i := 0; i < count; i++ {
		if !p.plan_.more_to_do() {
			p.t.Fatal("expected true")
		}
		edge := p.plan_.FindWork()
		if edge == nil {
			p.t.Fatal("expected true")
		}
		out = append(out, edge)
	}
	if p.plan_.FindWork() != nil {
		p.t.Fatal("expected false")
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].outputs_[0].path() < out[j].outputs_[0].path()
	})
	return out
}

func TestPlanTest_Basic(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	p.GetNode("mid").MarkDirty()
	p.GetNode("out").MarkDirty()
	err := ""
	if !p.plan_.AddTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		t.Fatalf("plan is inconsistent: %#v", p.plan_)
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "mid" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	if e := p.plan_.FindWork(); e != nil {
		t.Fatalf("%#v", e)
	}

	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "mid" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan_.more_to_do() {
		t.Fatal("expected false")
	}
	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected equal")
	}
}

// Test that two outputs from one rule can be handled as inputs to the next.
func TestPlanTest_DoubleOutputDirect(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "build out: cat mid1 mid2\nbuild mid1 mid2: cat in\n", ManifestParserOptions{})
	p.GetNode("mid1").MarkDirty()
	p.GetNode("mid2").MarkDirty()
	p.GetNode("out").MarkDirty()

	err := ""
	if !p.plan_.AddTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid1 mid2
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

// Test that two outputs from one rule can eventually be routed to another.
func TestPlanTest_DoubleOutputIndirect(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "build out: cat b1 b2\nbuild b1: cat a1\nbuild b2: cat a2\nbuild a1 a2: cat in\n", ManifestParserOptions{})
	p.GetNode("a1").MarkDirty()
	p.GetNode("a2").MarkDirty()
	p.GetNode("b1").MarkDirty()
	p.GetNode("b2").MarkDirty()
	p.GetNode("out").MarkDirty()
	err := ""
	if !p.plan_.AddTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a1
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a2
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat b1 b2
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

// Test that two edges from one output can both execute.
func TestPlanTest_DoubleDependent(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "build out: cat a1 a2\nbuild a1: cat mid\nbuild a2: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	p.GetNode("mid").MarkDirty()
	p.GetNode("a1").MarkDirty()
	p.GetNode("a2").MarkDirty()
	p.GetNode("out").MarkDirty()

	err := ""
	if !p.plan_.AddTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a1 a2
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

func (p *PlanTest) TestPoolWithDepthOne(test_case string) {
	p.AssertParse(&p.state_, test_case, ManifestParserOptions{})
	p.GetNode("out1").MarkDirty()
	p.GetNode("out2").MarkDirty()
	err := ""
	if !p.plan_.AddTarget(p.GetNode("out1"), &err) {
		p.t.Fatal("expected true")
	}
	if "" != err {
		p.t.Fatal("expected equal")
	}
	if !p.plan_.AddTarget(p.GetNode("out2"), &err) {
		p.t.Fatal("expected true")
	}
	if "" != err {
		p.t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		p.t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		p.t.Fatal("expected true")
	}
	if "in" != edge.inputs_[0].path() {
		p.t.Fatal("expected equal")
	}
	if "out1" != edge.outputs_[0].path() {
		p.t.Fatal("expected equal")
	}

	// This will be false since poolcat is serialized
	if p.plan_.FindWork() != nil {
		p.t.Fatal("expected false")
	}

	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		p.t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		p.t.Fatal("expected true")
	}
	if "in" != edge.inputs_[0].path() {
		p.t.Fatal("expected equal")
	}
	if "out2" != edge.outputs_[0].path() {
		p.t.Fatal("expected equal")
	}

	if p.plan_.FindWork() != nil {
		p.t.Fatal("expected false")
	}

	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		p.t.Fatal("expected equal")
	}

	if p.plan_.more_to_do() {
		p.t.Fatal("expected false")
	}
	edge = p.plan_.FindWork()
	if edge != nil {
		p.t.Fatal("expected equal")
	}
}

func TestPlanTest_PoolWithDepthOne(t *testing.T) {
	p := NewPlanTest(t)
	p.TestPoolWithDepthOne("pool foobar\n  depth = 1\nrule poolcat\n  command = cat $in > $out\n  pool = foobar\nbuild out1: poolcat in\nbuild out2: poolcat in\n")
}

func TestPlanTest_ConsolePool(t *testing.T) {
	p := NewPlanTest(t)
	p.TestPoolWithDepthOne("rule poolcat\n  command = cat $in > $out\n  pool = console\nbuild out1: poolcat in\nbuild out2: poolcat in\n")
}

func TestPlanTest_PoolsWithDepthTwo(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "pool foobar\n  depth = 2\npool bazbin\n  depth = 2\nrule foocat\n  command = cat $in > $out\n  pool = foobar\nrule bazcat\n  command = cat $in > $out\n  pool = bazbin\nbuild out1: foocat in\nbuild out2: foocat in\nbuild out3: foocat in\nbuild outb1: bazcat in\nbuild outb2: bazcat in\nbuild outb3: bazcat in\n  pool =\nbuild allTheThings: cat out1 out2 out3 outb1 outb2 outb3\n", ManifestParserOptions{})
	// Mark all the out* nodes dirty
	for i := 0; i < 3; i++ {
		p.GetNode(fmt.Sprintf("out%d", i+1)).MarkDirty()
		p.GetNode(fmt.Sprintf("outb%d", i+1)).MarkDirty()
	}
	p.GetNode("allTheThings").MarkDirty()

	err := ""
	if !p.plan_.AddTarget(p.GetNode("allTheThings"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	edges := p.FindWorkSorted(5)

	for i := 0; i < 4; i++ {
		edge := edges[i]
		if "in" != edge.inputs_[0].path() {
			t.Fatal("expected equal")
		}
		base_name := "outb"
		if i < 2 {
			base_name = "out"
		}
		if want := fmt.Sprintf("%s%d", base_name, 1+(i%2)); want != edge.outputs_[0].path() {
			t.Fatal(want)
		}
	}

	// outb3 is exempt because it has an empty pool
	edge := edges[4]
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "outb3" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	// finish out1
	p.plan_.EdgeFinished(edges[0], kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}
	edges = edges[1:]

	// out3 should be available
	out3 := p.plan_.FindWork()
	if out3 == nil {
		t.Fatal("expected true")
	}
	if "in" != out3.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out3" != out3.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}

	p.plan_.EdgeFinished(out3, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}

	for _, it := range edges {
		p.plan_.EdgeFinished(it, kEdgeSucceeded, &err)
		if "" != err {
			t.Fatal("expected equal")
		}
	}

	last := p.plan_.FindWork()
	if last == nil {
		t.Fatal("expected true")
	}
	if "allTheThings" != last.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	p.plan_.EdgeFinished(last, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan_.more_to_do() {
		t.Fatal("expected false")
	}
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}
}

func TestPlanTest_PoolWithRedundantEdges(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "pool compile\n  depth = 1\nrule gen_foo\n  command = touch foo.cpp\nrule gen_bar\n  command = touch bar.cpp\nrule echo\n  command = echo $out > $out\nbuild foo.cpp.obj: echo foo.cpp || foo.cpp\n  pool = compile\nbuild bar.cpp.obj: echo bar.cpp || bar.cpp\n  pool = compile\nbuild libfoo.a: echo foo.cpp.obj bar.cpp.obj\nbuild foo.cpp: gen_foo\nbuild bar.cpp: gen_bar\nbuild all: phony libfoo.a\n", ManifestParserOptions{})
	p.GetNode("foo.cpp").MarkDirty()
	p.GetNode("foo.cpp.obj").MarkDirty()
	p.GetNode("bar.cpp").MarkDirty()
	p.GetNode("bar.cpp.obj").MarkDirty()
	p.GetNode("libfoo.a").MarkDirty()
	p.GetNode("all").MarkDirty()
	err := ""
	if !p.plan_.AddTarget(p.GetNode("all"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	initial_edges := p.FindWorkSorted(2)

	edge := initial_edges[1] // Foo first
	if "foo.cpp" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}
	if "foo.cpp" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "foo.cpp" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "foo.cpp.obj" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = initial_edges[0] // Now for bar
	if "bar.cpp" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}
	if "bar.cpp" != edge.inputs_[0].path() {
		t.Fatal(edge.inputs_[0].path())
	}
	if "bar.cpp" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "bar.cpp.obj" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}
	if "foo.cpp.obj" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "bar.cpp.obj" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "libfoo.a" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}
	if "libfoo.a" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "all" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}
	p.plan_.EdgeFinished(edge, kEdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected false")
	}
	if p.plan_.more_to_do() {
		t.Fatal("expected false")
	}
}

func TestPlanTest_PoolWithFailingEdge(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state_, "pool foobar\n  depth = 1\nrule poolcat\n  command = cat $in > $out\n  pool = foobar\nbuild out1: poolcat in\nbuild out2: poolcat in\n", ManifestParserOptions{})
	p.GetNode("out1").MarkDirty()
	p.GetNode("out2").MarkDirty()
	err := ""
	if !p.plan_.AddTarget(p.GetNode("out1"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.AddTarget(p.GetNode("out2"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	}

	edge := p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out1" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	// This will be false since poolcat is serialized
	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}

	p.plan_.EdgeFinished(edge, kEdgeFailed, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan_.FindWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "out2" != edge.outputs_[0].path() {
		t.Fatal("expected equal")
	}

	if p.plan_.FindWork() != nil {
		t.Fatal("expected false")
	}

	p.plan_.EdgeFinished(edge, kEdgeFailed, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if !p.plan_.more_to_do() {
		t.Fatal("expected true")
	} // Jobs have failed
	edge = p.plan_.FindWork()
	if edge != nil {
		t.Fatal("expected equal")
	}
}

type BuildTestBase struct {
	StateTestWithBuiltinRules
	config_         BuildConfig
	command_runner_ FakeCommandRunner
	fs_             VirtualFileSystem
	status_         StatusPrinter
}

func NewBuildTestBase(t *testing.T) *BuildTestBase {
	b := &BuildTestBase{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		config_:                   NewBuildConfig(),
		fs_:                       NewVirtualFileSystem(),
	}
	b.config_.verbosity = QUIET
	b.command_runner_ = NewFakeCommandRunner(t, &b.fs_)
	//b.builder_ = NewBuilder(&b.state_, &b.config_, nil, nil, &b.fs_, &b.status_, 0)
	b.status_ = NewStatusPrinter(&b.config_)
	//b.builder_.command_runner_ = &b.command_runner_
	b.AssertParse(&b.state_, "build cat1: cat in1\nbuild cat2: cat in1 in2\nbuild cat12: cat cat1 cat2\n", ManifestParserOptions{})
	b.fs_.Create("in1", "")
	b.fs_.Create("in2", "")
	return b
}

func (b *BuildTestBase) IsPathDead(s string) bool {
	return false
}

// Rebuild target in the 'working tree' (fs_).
// State of command_runner_ and logs contents (if specified) ARE MODIFIED.
// Handy to check for NOOP builds, and higher-level rebuild tests.
func (b *BuildTestBase) RebuildTarget(target, manifest, log_path, deps_path string, state *State) {
	pstate := state
	if pstate == nil {
		local_state := NewState()
		pstate = &local_state
	}
	b.AddCatRule(pstate)
	b.AssertParse(pstate, manifest, ManifestParserOptions{})

	err := ""
	var pbuild_log *BuildLog
	if log_path != "" {
		build_log := NewBuildLog()
		if s := build_log.Load(log_path, &err); s != LOAD_SUCCESS && s != LOAD_NOT_FOUND {
			b.t.Fatalf("%s = %d: %s", log_path, s, err)
		}
		if !build_log.OpenForWrite(log_path, b, &err) {
			b.t.Fatal(err)
		}
		if "" != err {
			b.t.Fatal(err)
		}
		pbuild_log = &build_log
	}

	var pdeps_log *DepsLog
	if deps_path != "" {
		deps_log := NewDepsLog()
		if s := deps_log.Load(deps_path, pstate, &err); s != LOAD_SUCCESS && s != LOAD_NOT_FOUND {
			b.t.Fatalf("%s = %d: %s", deps_path, s, err)
		}
		if !deps_log.OpenForWrite(deps_path, &err) {
			b.t.Fatal("expected true")
		}
		if "" != err {
			b.t.Fatal("expected equal")
		}
		pdeps_log = &deps_log
	}

	builder := NewBuilder(pstate, &b.config_, pbuild_log, pdeps_log, &b.fs_, &b.status_, 0)
	if builder.AddTargetName(target, &err) == nil {
		b.t.Fatal(err)
	}

	b.command_runner_.commands_ran_ = nil
	builder.command_runner_ = &b.command_runner_
	if !builder.AlreadyUpToDate() {
		if !builder.Build(&err) {
			b.t.Fatal(err)
		}
	}
}

type BuildTest struct {
	*BuildTestBase
	builder_ *Builder
}

func NewBuildTest(t *testing.T) *BuildTest {
	b := &BuildTest{
		BuildTestBase: NewBuildTestBase(t),
	}
	b.builder_ = NewBuilder(&b.state_, &b.config_, nil, nil, &b.fs_, &b.status_, 0)
	b.builder_.command_runner_ = &b.command_runner_
	// TODO(maruel): Only do it for tests that write to disk.
	CreateTempDirAndEnter(t)
	return b
}

// Fake implementation of CommandRunner, useful for tests.
type FakeCommandRunner struct {
	t                 *testing.T
	commands_ran_     []string
	active_edges_     []*Edge
	max_active_edges_ uint
	fs_               *VirtualFileSystem
}

func NewFakeCommandRunner(t *testing.T, fs *VirtualFileSystem) FakeCommandRunner {
	return FakeCommandRunner{
		t:                 t,
		max_active_edges_: 1,
		fs_:               fs,
	}
}

// CommandRunner impl
func (f *FakeCommandRunner) CanRunMore() bool {
	return len(f.active_edges_) < int(f.max_active_edges_)
}

func (f *FakeCommandRunner) StartCommand(edge *Edge) bool {
	cmd := edge.EvaluateCommand(false)
	//f.t.Logf("StartCommand(%s)", cmd)
	if len(f.active_edges_) > int(f.max_active_edges_) {
		f.t.Fatal("oops")
	}
	found := false
	for _, a := range f.active_edges_ {
		if a == edge {
			found = true
			break
		}
	}
	if found {
		f.t.Fatalf("running same edge twice")
	}
	f.commands_ran_ = append(f.commands_ran_, cmd)
	if edge.rule().name() == "cat" || edge.rule().name() == "cat_rsp" || edge.rule().name() == "cat_rsp_out" || edge.rule().name() == "cc" || edge.rule().name() == "cp_multi_msvc" || edge.rule().name() == "cp_multi_gcc" || edge.rule().name() == "touch" || edge.rule().name() == "touch-interrupt" || edge.rule().name() == "touch-fail-tick2" {
		for _, out := range edge.outputs_ {
			f.fs_.Create(out.path(), "")
		}
	} else if edge.rule().name() == "true" || edge.rule().name() == "fail" || edge.rule().name() == "interrupt" || edge.rule().name() == "console" {
		// Don't do anything.
	} else if edge.rule().name() == "cp" {
		if len(edge.inputs_) == 0 {
			f.t.Fatal("oops")
		}
		if len(edge.outputs_) != 1 {
			f.t.Fatalf("%#v", edge.outputs_)
		}
		content := ""
		err := ""
		if f.fs_.ReadFile(edge.inputs_[0].path(), &content, &err) == Okay {
			f.fs_.WriteFile(edge.outputs_[0].path(), content)
		}
	} else if edge.rule().name() == "touch-implicit-dep-out" {
		dep := edge.GetBinding("test_dependency")
		f.fs_.Create(dep, "")
		f.fs_.Tick()
		for _, out := range edge.outputs_ {
			f.fs_.Create(out.path(), "")
		}
	} else if edge.rule().name() == "touch-out-implicit-dep" {
		dep := edge.GetBinding("test_dependency")
		for _, out := range edge.outputs_ {
			f.fs_.Create(out.path(), "")
		}
		f.fs_.Tick()
		f.fs_.Create(dep, "")
	} else if edge.rule().name() == "generate-depfile" {
		dep := edge.GetBinding("test_dependency")
		depfile := edge.GetUnescapedDepfile()
		contents := ""
		for _, out := range edge.outputs_ {
			contents += out.path() + ": " + dep + "\n"
			f.fs_.Create(out.path(), "")
		}
		f.fs_.Create(depfile, contents)
	} else {
		fmt.Printf("unknown command\n")
		return false
	}

	f.active_edges_ = append(f.active_edges_, edge)

	// Allow tests to control the order by the name of the first output.
	sort.Slice(f.active_edges_, func(i, j int) bool {
		return f.active_edges_[i].outputs_[0].path() < f.active_edges_[j].outputs_[0].path()
	})
	return true
}

func (f *FakeCommandRunner) WaitForCommand(result *Result) bool {
	if len(f.active_edges_) == 0 {
		return false
	}

	// All active edges were already completed immediately when started,
	// so we can pick any edge here.  Pick the last edge.  Tests can
	// control the order of edges by the name of the first output.
	edge_iter := len(f.active_edges_) - 1

	edge := f.active_edges_[edge_iter]
	result.edge = edge

	if edge.rule().name() == "interrupt" || edge.rule().name() == "touch-interrupt" {
		result.status = ExitInterrupted
		return true
	}

	if edge.rule().name() == "console" {
		if edge.use_console() {
			result.status = ExitSuccess
		} else {
			result.status = ExitFailure
		}
		copy(f.active_edges_[edge_iter:], f.active_edges_[edge_iter+1:])
		f.active_edges_ = f.active_edges_[:len(f.active_edges_)-1]
		return true
	}

	if edge.rule().name() == "cp_multi_msvc" {
		prefix := edge.GetBinding("msvc_deps_prefix")
		for _, in := range edge.inputs_ {
			result.output += prefix + in.path() + "\n"
		}
	}

	if edge.rule().name() == "fail" || (edge.rule().name() == "touch-fail-tick2" && f.fs_.now_ == 2) {
		result.status = ExitFailure
	} else {
		result.status = ExitSuccess
	}

	// Provide a way for test cases to verify when an edge finishes that
	// some other edge is still active.  This is useful for test cases
	// covering behavior involving multiple active edges.
	verify_active_edge := edge.GetBinding("verify_active_edge")
	if verify_active_edge != "" {
		verify_active_edge_found := false
		for _, i := range f.active_edges_ {
			if len(i.outputs_) != 0 && i.outputs_[0].path() == verify_active_edge {
				verify_active_edge_found = true
			}
		}
		if !verify_active_edge_found {
			f.t.Fatal("expected true")
		}
	}

	copy(f.active_edges_[edge_iter:], f.active_edges_[edge_iter+1:])
	f.active_edges_ = f.active_edges_[:len(f.active_edges_)-1]
	return true
}

func (f *FakeCommandRunner) GetActiveEdges() []*Edge {
	return f.active_edges_
}

func (f *FakeCommandRunner) Abort() {
	f.active_edges_ = nil
}

// Mark a path dirty.
func (b *BuildTest) Dirty(path string) {
	node := b.GetNode(path)
	node.MarkDirty()

	// If it's an input file, mark that we've already stat()ed it and
	// it's missing.
	if node.in_edge() == nil {
		node.MarkMissing()
	}
}

func TestBuildTest_NoWork(t *testing.T) {
	b := NewBuildTest(t)
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildTest_OneStep(t *testing.T) {
	b := NewBuildTest(t)
	// Given a dirty target with one ready input,
	// we should rebuild the target.
	b.Dirty("cat1")
	err := ""
	if b.builder_.AddTargetName("cat1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	want_commands := []string{"cat in1 > cat1"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_OneStep2(t *testing.T) {
	b := NewBuildTest(t)
	// Given a target with one dirty input,
	// we should rebuild the target.
	b.Dirty("cat1")
	err := ""
	if b.builder_.AddTargetName("cat1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	want_commands := []string{"cat in1 > cat1"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_TwoStep(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	if b.builder_.AddTargetName("cat12", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	// Depending on how the pointers work out, we could've ran
	// the first two commands in either order.
	if !(b.command_runner_.commands_ran_[0] == "cat in1 > cat1" && b.command_runner_.commands_ran_[1] == "cat in1 in2 > cat2") || (b.command_runner_.commands_ran_[1] == "cat in1 > cat1" && b.command_runner_.commands_ran_[0] == "cat in1 in2 > cat2") {
		t.Fatal("expected true")
	}

	if "cat cat1 cat2 > cat12" != b.command_runner_.commands_ran_[2] {
		t.Fatal("expected equal")
	}

	b.fs_.Tick()

	// Modifying in2 requires rebuilding one intermediate file
	// and the final file.
	b.fs_.Create("in2", "")
	b.state_.Reset()
	if b.builder_.AddTargetName("cat12", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 5 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "cat in1 in2 > cat2" != b.command_runner_.commands_ran_[3] {
		t.Fatal("expected equal")
	}
	if "cat cat1 cat2 > cat12" != b.command_runner_.commands_ran_[4] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_TwoOutputs(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nbuild out1 out2: touch in.txt\n", ManifestParserOptions{})

	b.fs_.Create("in.txt", "")

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"touch out1 out2"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_ImplicitOutput(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nbuild out | out.imp: touch in.txt\n", ManifestParserOptions{})
	b.fs_.Create("in.txt", "")

	err := ""
	if b.builder_.AddTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

// Test case from
//   https://github.com/ninja-build/ninja/issues/148
func TestBuildTest_MultiOutIn(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nbuild in1 otherfile: touch in\nbuild out: touch in | in1\n", ManifestParserOptions{})

	b.fs_.Create("in", "")
	b.fs_.Tick()
	b.fs_.Create("in1", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_Chain(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "build c2: cat c1\nbuild c3: cat c2\nbuild c4: cat c3\nbuild c5: cat c4\n", ManifestParserOptions{})

	b.fs_.Create("c1", "")

	err := ""
	if b.builder_.AddTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 4 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	err = ""
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.fs_.Tick()

	b.fs_.Create("c3", "")
	err = ""
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	} // 3->4, 4->5
}

func TestBuildTest_MissingInput(t *testing.T) {
	b := NewBuildTest(t)
	// Input is referenced by build file, but no rule for it.
	err := ""
	b.Dirty("in1")
	if b.builder_.AddTargetName("cat1", &err) != nil {
		t.Fatal("expected false")
	}
	if "'in1', needed by 'cat1', missing and no known rule to make it" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_MissingTarget(t *testing.T) {
	b := NewBuildTest(t)
	// Target is not referenced by build file.
	err := ""
	if b.builder_.AddTargetName("meow", &err) != nil {
		t.Fatal("expected false")
	}
	if "unknown target: 'meow'" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_MakeDirs(t *testing.T) {
	b := NewBuildTest(t)
	err := ""

	p := filepath.Join("subdir", "dir2", "file")
	b.AssertParse(&b.state_, "build "+p+": cat in1\n", ManifestParserOptions{})
	if b.builder_.AddTargetName("subdir/dir2/file", &err) == nil {
		t.Fatal(err)
	}

	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_made := map[string]struct{}{
		"subdir":                        {},
		filepath.Join("subdir", "dir2"): {},
	}
	if diff := cmp.Diff(want_made, b.fs_.directories_made_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DepFileMissing(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild fo$ o.o: cc foo.c\n", ManifestParserOptions{})
	b.fs_.Create("foo.c", "")

	if b.builder_.AddTargetName("fo o.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.fs_.files_read_) {
		t.Fatal("expected equal")
	}
	if "fo o.o.d" != b.fs_.files_read_[0] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepFileOK(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	orig_edges := len(b.state_.edges_)
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n", ManifestParserOptions{})
	edge := b.state_.edges_[len(b.state_.edges_)-1]

	b.fs_.Create("foo.c", "")
	b.GetNode("bar.h").MarkDirty() // Mark bar.h as missing.
	b.fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.fs_.files_read_) {
		t.Fatal("expected equal")
	}
	if "foo.o.d" != b.fs_.files_read_[0] {
		t.Fatal("expected equal")
	}

	// Expect three new edges: one generating foo.o, and two more from
	// loading the depfile.
	if orig_edges+3 != len(b.state_.edges_) {
		t.Fatal("expected equal")
	}
	// Expect our edge to now have three inputs: foo.c and two headers.
	if 3 != len(edge.inputs_) {
		t.Fatalf("%#v", edge.inputs_)
	}

	// Expect the command line we generate to only use the original input.
	if "cc foo.c" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepFileParseError(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n", ManifestParserOptions{})
	b.fs_.Create("foo.c", "")
	b.fs_.Create("foo.o.d", "randomtext\n")
	if b.builder_.AddTargetName("foo.o", &err) != nil {
		t.Fatal("expected false")
	}
	if "foo.o.d: expected ':' in depfile" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_EncounterReadyTwice(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nbuild c: touch\nbuild b: touch || c\nbuild a: touch | b || c\n", ManifestParserOptions{})

	c_out := b.GetNode("c").out_edges()
	if 2 != len(c_out) {
		t.Fatal("expected equal")
	}
	if "b" != c_out[0].outputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "a" != c_out[1].outputs_[0].path() {
		t.Fatal("expected equal")
	}

	b.fs_.Create("b", "")
	if b.builder_.AddTargetName("a", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_OrderOnlyDeps(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c || otherfile\n", ManifestParserOptions{})
	edge := b.state_.edges_[len(b.state_.edges_)-1]

	b.fs_.Create("foo.c", "")
	b.fs_.Create("otherfile", "")
	b.fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// One explicit, two implicit, one order only.
	if 4 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}
	if 2 != edge.implicit_deps_ {
		t.Fatal("expected equal")
	}
	if 1 != edge.order_only_deps_ {
		t.Fatal("expected equal")
	}
	// Verify the inputs are in the order we expect
	// (explicit then implicit then orderonly).
	if "foo.c" != edge.inputs_[0].path() {
		t.Fatal("expected equal")
	}
	if "blah.h" != edge.inputs_[1].path() {
		t.Fatal("expected equal")
	}
	if "bar.h" != edge.inputs_[2].path() {
		t.Fatal("expected equal")
	}
	if "otherfile" != edge.inputs_[3].path() {
		t.Fatal("expected equal")
	}

	// Expect the command line we generate to only use the original input.
	if "cc foo.c" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}

	// explicit dep dirty, expect a rebuild.
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.fs_.Tick()

	// Recreate the depfile, as it should have been deleted by the build.
	b.fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")

	// implicit dep dirty, expect a rebuild.
	b.fs_.Create("blah.h", "")
	b.fs_.Create("bar.h", "")
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.fs_.Tick()

	// Recreate the depfile, as it should have been deleted by the build.
	b.fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")

	// order only dep dirty, no rebuild.
	b.fs_.Create("otherfile", "")
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// implicit dep missing, expect rebuild.
	b.fs_.RemoveFile("bar.h")
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_RebuildOrderOnlyDeps(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\nrule true\n  command = true\nbuild oo.h: cc oo.h.in\nbuild foo.o: cc foo.c || oo.h\n", ManifestParserOptions{})

	b.fs_.Create("foo.c", "")
	b.fs_.Create("oo.h.in", "")

	// foo.o and order-only dep dirty, build both.
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// all clean, no rebuild.
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// order-only dep missing, build it only.
	b.fs_.RemoveFile("oo.h")
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cc oo.h.in"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	b.fs_.Tick()

	// order-only dep dirty, build it only.
	b.fs_.Create("oo.h.in", "")
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands = []string{"cc oo.h.in"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DepFileCanonicalize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	b := NewBuildTest(t)
	err := ""
	orig_edges := len(b.state_.edges_)
	b.AssertParse(&b.state_, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild gen/stuff\\things/foo.o: cc x\\y/z\\foo.c\n", ManifestParserOptions{})
	edge := b.state_.edges_[len(b.state_.edges_)-1]

	b.fs_.Create("x/y/z/foo.c", "")
	b.GetNode("bar.h").MarkDirty() // Mark bar.h as missing.
	// Note, different slashes from manifest.
	b.fs_.Create("gen/stuff\\things/foo.o.d", "gen\\stuff\\things\\foo.o: blah.h bar.h\n")
	if b.builder_.AddTargetName("gen/stuff/things/foo.o", &err) == nil {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	// The depfile path does not get Canonicalize as it seems unnecessary.
	want_reads := []string{"gen/stuff\\things/foo.o.d"}
	if diff := cmp.Diff(want_reads, b.fs_.files_read_); diff != "" {
		t.Fatal(diff)
	}

	// Expect three new edges: one generating foo.o, and two more from
	// loading the depfile.
	if orig_edges+3 != len(b.state_.edges_) {
		t.Fatal("expected equal")
	}
	// Expect our edge to now have three inputs: foo.c and two headers.
	if 3 != len(edge.inputs_) {
		t.Fatal("expected equal")
	}

	// Expect the command line we generate to only use the original input, and
	// using the slashes from the manifest.
	if "cc x\\y/z\\foo.c" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_Phony(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "build out: cat bar.cc\nbuild all: phony out\n", ManifestParserOptions{})
	b.fs_.Create("bar.cc", "")

	if b.builder_.AddTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Only one command to run, because phony runs no command.
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PhonyNoWork(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "build out: cat bar.cc\nbuild all: phony out\n", ManifestParserOptions{})
	b.fs_.Create("bar.cc", "")
	b.fs_.Create("out", "")

	if b.builder_.AddTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

// Test a self-referencing phony.  Ideally this should not work, but
// ninja 1.7 and below tolerated and CMake 2.8.12.x and 3.0.x both
// incorrectly produce it.  We tolerate it for compatibility.
func TestBuildTest_PhonySelfReference(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "build a: phony a\n", ManifestParserOptions{})

	if b.builder_.AddTargetName("a", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

// There are 6 different cases for phony rules:
//
// 1. output edge does not exist, inputs are not real
// 2. output edge does not exist, no inputs
// 3. output edge does not exist, inputs are real, newest mtime is M
// 4. output edge is real, inputs are not real
// 5. output edge is real, no inputs
// 6. output edge is real, inputs are real, newest mtime is M
//
// Expected results :
// 1. Edge is marked as clean, mtime is newest mtime of dependents.
//     Touching inputs will cause dependents to rebuild.
// 2. Edge is marked as dirty, causing dependent edges to always rebuild
// 3. Edge is marked as clean, mtime is newest mtime of dependents.
//     Touching inputs will cause dependents to rebuild.
// 4. Edge is marked as clean, mtime is newest mtime of dependents.
//     Touching inputs will cause dependents to rebuild.
// 5. Edge is marked as dirty, causing dependent edges to always rebuild
// 6. Edge is marked as clean, mtime is newest mtime of dependents.
//     Touching inputs will cause dependents to rebuild.
func PhonyUseCase(t *testing.T, i int) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state_, "rule touch\n command = touch $out\nbuild notreal: phony blank\nbuild phony1: phony notreal\nbuild phony2: phony\nbuild phony3: phony blank\nbuild phony4: phony notreal\nbuild phony5: phony\nbuild phony6: phony blank\n\nbuild test1: touch phony1\nbuild test2: touch phony2\nbuild test3: touch phony3\nbuild test4: touch phony4\nbuild test5: touch phony5\nbuild test6: touch phony6\n", ManifestParserOptions{})

	// Set up test.
	b.builder_.command_runner_ = nil // BuildTest owns the CommandRunner
	b.builder_.command_runner_ = &b.command_runner_

	b.fs_.Create("blank", "") // a "real" file
	if b.builder_.AddTargetName("test1", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AddTargetName("test2", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AddTargetName("test3", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AddTargetName("test4", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AddTargetName("test5", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AddTargetName("test6", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		b.t.Fatal("expected true")
	}
	if "" != err {
		b.t.Fatal("expected equal")
	}

	ci := strconv.Itoa(i)

	// Tests 1, 3, 4, and 6 should rebuild when the input is updated.
	if i != 2 && i != 5 {
		testNode := b.GetNode("test" + ci)
		phonyNode := b.GetNode("phony" + ci)
		inputNode := b.GetNode("blank")

		b.state_.Reset()
		startTime := b.fs_.now_

		// Build number 1
		if b.builder_.AddTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if !b.builder_.AlreadyUpToDate() {
			if !b.builder_.Build(&err) {
				t.Fatal("expected true")
			}
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Touch the input file
		b.state_.Reset()
		b.command_runner_.commands_ran_ = nil
		b.fs_.Tick()
		b.fs_.Create("blank", "") // a "real" file
		if b.builder_.AddTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Second build, expect testN edge to be rebuilt
		// and phonyN node's mtime to be updated.
		if b.builder_.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder_.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		want_commands := []string{"touch test" + ci}
		if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
			t.Fatal(diff)
		}
		if !b.builder_.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		inputTime := inputNode.mtime()

		if phonyNode.exists() {
			t.Fatal("expected false")
		}
		if phonyNode.dirty() {
			t.Fatal("expected false")
		}

		if phonyNode.mtime() <= startTime {
			t.Fatal("expected greater")
		}
		if phonyNode.mtime() != inputTime {
			t.Fatal("expected equal")
		}
		if !testNode.Stat(&b.fs_, &err) {
			t.Fatal("expected true")
		}
		if !testNode.exists() {
			t.Fatal("expected true")
		}
		if testNode.mtime() <= startTime {
			t.Fatal("expected greater")
		}
	} else {
		// Tests 2 and 5: Expect dependents to always rebuild.

		b.state_.Reset()
		b.command_runner_.commands_ran_ = nil
		b.fs_.Tick()
		b.command_runner_.commands_ran_ = nil
		if b.builder_.AddTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if b.builder_.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder_.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		want_commands := []string{"touch test" + ci}
		if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
			t.Fatal(diff)
		}

		b.state_.Reset()
		b.command_runner_.commands_ran_ = nil
		if b.builder_.AddTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if b.builder_.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder_.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		want_commands = []string{"touch test" + ci}
		if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestBuildTest_PhonyUseCase(t *testing.T) {
	for i := 1; i < 7; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) { PhonyUseCase(t, i) })
	}
}

func TestBuildTest_Fail(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule fail\n  command = fail\nbuild out1: fail\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailures(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule fail\n  command = fail\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild all: phony out1 out2 out3\n", ManifestParserOptions{})

	// Swallow two failures, die on the third.
	b.config_.failures_allowed = 3

	err := ""
	if b.builder_.AddTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "subcommands failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailuresLimit(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule fail\n  command = fail\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild final: cat out1 out2 out3\n", ManifestParserOptions{})

	// Swallow ten failures; we should stop before building final.
	b.config_.failures_allowed = 11

	err := ""
	if b.builder_.AddTargetName("final", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "cannot make progress due to previous errors" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailuresPool(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "pool failpool\n  depth = 1\nrule fail\n  command = fail\n  pool = failpool\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild final: cat out1 out2 out3\n", ManifestParserOptions{})

	// Swallow ten failures; we should stop before building final.
	b.config_.failures_allowed = 11

	err := ""
	if b.builder_.AddTargetName("final", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "cannot make progress due to previous errors" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PoolEdgesReadyButNotWanted(t *testing.T) {
	b := NewBuildTest(t)
	b.fs_.Create("x", "")

	manifest := "pool some_pool\n  depth = 4\nrule touch\n  command = touch $out\n  pool = some_pool\nrule cc\n  command = touch grit\n\nbuild B.d.stamp: cc | x\nbuild C.stamp: touch B.d.stamp\nbuild final.stamp: touch || C.stamp\n"

	b.RebuildTarget("final.stamp", manifest, "", "", nil)

	b.fs_.RemoveFile("B.d.stamp")

	save_state := NewState()
	b.RebuildTarget("final.stamp", manifest, "", "", &save_state)
	if save_state.LookupPool("some_pool").current_use() < 0 {
		t.Fatal("expected greater or equal")
	}
}

type BuildWithLogTest struct {
	*BuildTest
	build_log_ BuildLog
}

func NewBuildWithLogTest(t *testing.T) *BuildWithLogTest {
	b := &BuildWithLogTest{
		BuildTest:  NewBuildTest(t),
		build_log_: NewBuildLog(),
	}
	b.builder_.SetBuildLog(&b.build_log_)
	return b
}

func TestBuildWithLogTest_ImplicitGeneratedOutOfDate(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\n  generator = 1\nbuild out.imp: touch | in\n", ManifestParserOptions{})
	b.fs_.Create("out.imp", "")
	b.fs_.Tick()
	b.fs_.Create("in", "")

	err := ""

	if b.builder_.AddTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.GetNode("out.imp").dirty() {
		t.Fatal("expected true")
	}
}

func TestBuildWithLogTest_ImplicitGeneratedOutOfDate2(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule touch-implicit-dep-out\n  command = touch $test_dependency ; sleep 1 ; touch $out\n  generator = 1\nbuild out.imp: touch-implicit-dep-out | inimp inimp2\n  test_dependency = inimp\n", ManifestParserOptions{})
	b.fs_.Create("inimp", "")
	b.fs_.Create("out.imp", "")
	b.fs_.Tick()
	b.fs_.Create("inimp2", "")
	b.fs_.Tick()

	err := ""

	if b.builder_.AddTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	b.builder_.Cleanup()
	b.builder_.plan_.Reset()

	if b.builder_.AddTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
	if b.GetNode("out.imp").dirty() {
		t.Fatal("expected false")
	}
}

func TestBuildWithLogTest_NotInLogButOnDisk(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule cc\n  command = cc\nbuild out1: cc in\n", ManifestParserOptions{})

	// Create input/output that would be considered up to date when
	// not considering the command line hash.
	b.fs_.Create("in", "")
	b.fs_.Create("out1", "")
	err := ""

	// Because it's not in the log, it should not be up-to-date until
	// we build again.
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()

	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildWithLogTest_RebuildAfterFailure(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule touch-fail-tick2\n  command = touch-fail-tick2\nbuild out1: touch-fail-tick2 in\n", ManifestParserOptions{})

	err := ""

	b.fs_.Create("in", "")

	// Run once successfully to get out1 in the log
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	b.builder_.Cleanup()
	b.builder_.plan_.Reset()

	b.fs_.Tick()
	b.fs_.Create("in", "")

	// Run again with a failure that updates the output file timestamp
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	b.builder_.Cleanup()
	b.builder_.plan_.Reset()

	b.fs_.Tick()

	// Run again, should rerun even though the output file is up to date on disk
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RebuildWithNoInputs(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule touch\n  command = touch\nbuild out1: touch\nbuild out2: touch in\n", ManifestParserOptions{})

	err := ""

	b.fs_.Create("in", "")

	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()

	b.fs_.Tick()

	b.fs_.Create("in", "")

	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatTest(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\n  restat = 1\nbuild out1: cc in\nbuild out2: true out1\nbuild out3: cat out2\n", ManifestParserOptions{})

	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("out3", "")

	b.fs_.Tick()

	b.fs_.Create("in", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause out3 to rebuild
	// regardless of restat.
	err := ""
	if b.builder_.AddTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	if 3 != b.builder_.plan_.command_edge_count() {
		t.Fatal("expected equal")
	}
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()

	b.fs_.Tick()

	b.fs_.Create("in", "")
	// "cc" touches out1, so we should build out2.  But because "true" does not
	// touch out2, we should cancel the build of out3.
	if b.builder_.AddTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// If we run again, it should be a no-op, because the build log has recorded
	// that we've already built out2 with an input timestamp of 2 (from out1).
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.fs_.Tick()

	b.fs_.Create("in", "")

	// The build log entry should not, however, prevent us from rebuilding out2
	// if out1 changes.
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatMissingFile(t *testing.T) {
	b := NewBuildWithLogTest(t)
	// If a restat rule doesn't create its output, and the output didn't
	// exist before the rule was run, consider that behavior equivalent
	// to a rule that doesn't modify its existent output file.

	b.AssertParse(&b.state_, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\nbuild out1: true in\nbuild out2: cc out1\n", ManifestParserOptions{})

	b.fs_.Create("in", "")
	b.fs_.Create("out2", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause out2 to rebuild
	// regardless of restat.
	err := ""
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()

	b.fs_.Tick()
	b.fs_.Create("in", "")
	b.fs_.Create("out2", "")

	// Run a build, expect only the first command to run.
	// It doesn't touch its output (due to being the "true" command), so
	// we shouldn't run the dependent build.
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatSingleDependentOutputDirty(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule true\n  command = true\n  restat = 1\nrule touch\n  command = touch\nbuild out1: true in\nbuild out2 out3: touch out1\nbuild out4: touch out2\n", ManifestParserOptions{})

	// Create the necessary files
	b.fs_.Create("in", "")

	err := ""
	if b.builder_.AddTargetName("out4", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	b.fs_.Tick()
	b.fs_.Create("in", "")
	b.fs_.RemoveFile("out3")

	// Since "in" is missing, out1 will be built. Since "out3" is missing,
	// out2 and out3 will be built even though "in" is not touched when built.
	// Then, since out2 is rebuilt, out4 should be rebuilt -- the restat on the
	// "true" rule should not lead to the "touch" edge writing out2 and out3 being
	// cleard.
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out4", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

// Test scenario, in which an input file is removed, but output isn't changed
// https://github.com/ninja-build/ninja/issues/295
func TestBuildWithLogTest_RestatMissingInput(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule true\n  command = true\n  depfile = $out.d\n  restat = 1\nrule cc\n  command = cc\nbuild out1: true in\nbuild out2: cc out1\n", ManifestParserOptions{})

	// Create all necessary files
	b.fs_.Create("in", "")

	// The implicit dependencies and the depfile itself
	// are newer than the output
	restat_mtime := b.fs_.Tick()
	b.fs_.Create("out1.d", "out1: will.be.deleted restat.file\n")
	b.fs_.Create("will.be.deleted", "")
	b.fs_.Create("restat.file", "")

	// Run the build, out1 and out2 get built
	err := ""
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// See that an entry in the logfile is created, capturing
	// the right mtime
	log_entry := b.build_log_.LookupByOutput("out1")
	if nil == log_entry {
		t.Fatal("expected true")
	}
	if restat_mtime != log_entry.mtime {
		t.Fatal("expected equal")
	}

	// Now remove a file, referenced from depfile, so that target becomes
	// dirty, but the output does not change
	b.fs_.RemoveFile("will.be.deleted")

	// Trigger the build again - only out1 gets built
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// Check that the logfile entry remains correctly set
	log_entry = b.build_log_.LookupByOutput("out1")
	if nil == log_entry {
		t.Fatal("expected true")
	}
	if restat_mtime != log_entry.mtime {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_GeneratedPlainDepfileMtime(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule generate-depfile\n  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\nbuild out: generate-depfile\n  test_dependency = inimp\n  depfile = out.d\n", ManifestParserOptions{})
	b.fs_.Create("inimp", "")
	b.fs_.Tick()

	err := ""

	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	b.builder_.Cleanup()
	b.builder_.plan_.Reset()

	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func NewBuildDryRunTest(t *testing.T) *BuildWithLogTest {
	b := NewBuildWithLogTest(t)
	b.config_.dry_run = true
	return b
}

func TestBuildDryRun_AllCommandsShown(t *testing.T) {
	b := NewBuildDryRunTest(t)
	b.AssertParse(&b.state_, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\n  restat = 1\nbuild out1: cc in\nbuild out2: true out1\nbuild out3: cat out2\n", ManifestParserOptions{})

	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("out3", "")

	b.fs_.Tick()

	b.fs_.Create("in", "")

	// "cc" touches out1, so we should build out2.  But because "true" does not
	// touch out2, we should cancel the build of out3.
	err := ""
	if b.builder_.AddTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

// Test that RSP files are created when & where appropriate and deleted after
// successful execution.
func TestBuildTest_RspFileSuccess(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $long_command\nrule cat_rsp_out\n  command = cat $rspfile > $out\n  rspfile = $out.rsp\n  rspfile_content = $long_command\nbuild out1: cat in\nbuild out2: cat_rsp in\n  rspfile = out 2.rsp\n  long_command = Some very long command\nbuild out$ 3: cat_rsp_out in\n  long_command = Some very long command\n", ManifestParserOptions{})

	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("out 3", "")

	b.fs_.Tick()

	b.fs_.Create("in", "")

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}
	if b.builder_.AddTargetName("out 3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}

	want_created := map[string]struct{}{
		"in":    {},
		"in1":   {},
		"in2":   {},
		"out 3": {},
		"out1":  {},
		"out2":  {},
	}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}
	want_removed := map[string]struct{}{}
	if diff := cmp.Diff(want_removed, b.fs_.files_removed_); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal(b.command_runner_.commands_ran_)
	}

	// The RSP files were created
	want_created["out 2.rsp"] = struct{}{}
	want_created["out 3.rsp"] = struct{}{}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}

	// The RSP files were removed
	want_removed["out 2.rsp"] = struct{}{}
	want_removed["out 3.rsp"] = struct{}{}
	if diff := cmp.Diff(want_removed, b.fs_.files_removed_); diff != "" {
		t.Fatal(diff)
	}
}

// Test that RSP file is created but not removed for commands, which fail
func TestBuildTest_RspFileFailure(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule fail\n  command = fail\n  rspfile = $rspfile\n  rspfile_content = $long_command\nbuild out: fail in\n  rspfile = out.rsp\n  long_command = Another very long command\n", ManifestParserOptions{})

	b.fs_.Create("out", "")
	b.fs_.Tick()
	b.fs_.Create("in", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	want_created := map[string]struct{}{
		"in":  {},
		"in1": {},
		"in2": {},
		"out": {},
	}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}
	want_removed := map[string]struct{}{}
	if diff := cmp.Diff(want_removed, b.fs_.files_removed_); diff != "" {
		t.Fatal(diff)
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
	want_command := []string{"fail"}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file was created
	want_created["out.rsp"] = struct{}{}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file was NOT removed
	if diff := cmp.Diff(want_removed, b.fs_.files_removed_); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file contains what it should
	if "Another very long command" != b.fs_.files_["out.rsp"].contents {
		t.Fatal("expected equal")
	}
}

// Test that contents of the RSP file behaves like a regular part of
// command line, i.e. triggers a rebuild if changed
func TestBuildWithLogTest_RspFileCmdLineChange(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state_, "rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $long_command\nbuild out: cat_rsp in\n  rspfile = out.rsp\n  long_command = Original very long command\n", ManifestParserOptions{})

	b.fs_.Create("out", "")
	b.fs_.Tick()
	b.fs_.Create("in", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// 1. Build for the 1st time (-> populate log)
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	want_command := []string{"cat out.rsp > out"}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	// 2. Build again (no change)
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// 3. Alter the entry in the logfile
	// (to simulate a change in the command line between 2 builds)
	log_entry := b.build_log_.LookupByOutput("out")
	if nil == log_entry {
		t.Fatal("expected true")
	}
	b.AssertHash("cat out.rsp > out;rspfile=Original very long command", log_entry.command_hash)
	log_entry.command_hash++ // Change the command hash to something else.
	// Now expect the target to be rebuilt
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_InterruptCleanup(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule interrupt\n  command = interrupt\nrule touch-interrupt\n  command = touch-interrupt\nbuild out1: interrupt in1\nbuild out2: touch-interrupt in2\n", ManifestParserOptions{})

	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Tick()
	b.fs_.Create("in1", "")
	b.fs_.Create("in2", "")

	// An untouched output of an interrupted command should be retained.
	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "interrupted by user" != err {
		t.Fatal("expected equal")
	}
	b.builder_.Cleanup()
	if b.fs_.Stat("out1", &err) <= 0 {
		t.Fatal("expected greater")
	}
	err = ""

	// A touched output of an interrupted command should be deleted.
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "interrupted by user" != err {
		t.Fatal("expected equal")
	}
	b.builder_.Cleanup()
	if 0 != b.fs_.Stat("out2", &err) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_StatFailureAbortsBuild(t *testing.T) {
	b := NewBuildTest(t)
	kTooLongToStat := strings.Repeat("i", 400)
	b.AssertParse(&b.state_, ("build " + kTooLongToStat + ": cat in\n"), ManifestParserOptions{})
	b.fs_.Create("in", "")

	// This simulates a stat failure:
	b.fs_.files_[kTooLongToStat] = Entry{
		mtime:      -1,
		stat_error: "stat failed",
	}

	err := ""
	if b.builder_.AddTargetName(kTooLongToStat, &err) != nil {
		t.Fatal("expected false")
	}
	if "stat failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PhonyWithNoInputs(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "build nonexistent: phony\nbuild out1: cat || nonexistent\nbuild out2: cat nonexistent\n", ManifestParserOptions{})
	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")

	// out1 should be up to date even though its input is dirty, because its
	// order-only dependency has nothing to do.
	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// out2 should still be out of date though, because its input is dirty.
	err = ""
	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepsGccWithEmptyDepfileErrorsOut(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule cc\n  command = cc\n  deps = gcc\nbuild out: cc\n", ManifestParserOptions{})
	b.Dirty("out")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder_.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_StatusFormatElapsed(t *testing.T) {
	b := NewBuildTest(t)
	b.status_.BuildStarted()
	// Before any task is done, the elapsed time must be zero.
	if "[%/e0.000]" != b.status_.FormatProgressStatus("[%%/e%e]", 0) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_StatusFormatReplacePlaceholder(t *testing.T) {
	b := NewBuildTest(t)
	if "[%/s0/t0/r0/u0/f0]" != b.status_.FormatProgressStatus("[%%/s%s/t%t/r%r/u%u/f%f]", 0) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_FailedDepsParse(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "build bad_deps.o: cat in1\n  deps = gcc\n  depfile = in1.d\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("bad_deps.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// These deps will fail to parse, as they should only have one
	// path to the left of the colon.
	b.fs_.Create("in1.d", "AAA BBB")

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
}

type BuildWithQueryDepsLogTest struct {
	*BuildTestBase
	log_     DepsLog
	builder_ *Builder
}

func NewBuildWithQueryDepsLogTest(t *testing.T) *BuildWithQueryDepsLogTest {
	b := &BuildWithQueryDepsLogTest{
		BuildTestBase: NewBuildTestBase(t),
		log_:          NewDepsLog(),
	}
	t.Cleanup(func() {
		b.log_.Close()
	})
	CreateTempDirAndEnter(t)
	err := ""
	if !b.log_.OpenForWrite("ninja_deps", &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.builder_ = NewBuilder(&b.state_, &b.config_, nil, &b.log_, &b.fs_, &b.status_, 0)
	b.builder_.command_runner_ = &b.command_runner_
	return b
}

// Test a MSVC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileMSVC(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state_, "rule cp_multi_msvc\n    command = echo 'using $in' && for file in $out; do cp $in $$file; done\n    deps = msvc\n    msvc_deps_prefix = using \nbuild out1 out2: cp_multi_msvc in1\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"echo 'using in1' && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 1 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 1 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOneLine(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state_, "rule cp_multi_gcc\n    command = echo '$out: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs_.Create("in.d", "out1 out2: in1 in2")
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"echo 'out1 out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 2 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out1_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 2 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out2_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs using a line per input.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineInput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state_, "rule cp_multi_gcc\n    command = echo '$out: in1\\n$out: in2' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs_.Create("in.d", "out1 out2: in1\nout1 out2: in2")
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"echo 'out1 out2: in1\\nout1 out2: in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 2 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out1_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 2 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out2_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs using a line per output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state_, "rule cp_multi_gcc\n    command = echo 'out1: $in\\nout2: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs_.Create("in.d", "out1: in1 in2\nout2: in1 in2")
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"echo 'out1: in1 in2\\nout2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 2 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out1_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 2 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out2_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs mentioning only the main output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlyMainOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state_, "rule cp_multi_gcc\n    command = echo 'out1: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs_.Create("in.d", "out1: in1 in2")
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_command := []string{"echo 'out1: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 2 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out1_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 2 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out2_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs mentioning only the secondary output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlySecondaryOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	// Note: This ends up short-circuiting the node creation due to the primary
	// output not being present, but it should still work.
	b.AssertParse(&b.state_, "rule cp_multi_gcc\n    command = echo 'out2: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs_.Create("in.d", "out2: in1 in2")
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_command := []string{"echo 'out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	out1_node := b.state_.LookupNode("out1")
	out1_deps := b.log_.GetDeps(out1_node)
	if 2 != out1_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out1_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out1_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}

	out2_node := b.state_.LookupNode("out2")
	out2_deps := b.log_.GetDeps(out2_node)
	if 2 != out2_deps.node_count {
		t.Fatal("expected equal")
	}
	if "in1" != out2_deps.nodes[0].path() {
		t.Fatal("expected equal")
	}
	if "in2" != out2_deps.nodes[1].path() {
		t.Fatal("expected equal")
	}
}

// Tests of builds involving deps logs necessarily must span
// multiple builds.  We reuse methods on BuildTest but not the
// b.builder_ it sets up, because we want pristine objects for
// each build.
func NewBuildWithDepsLogTest(t *testing.T) *BuildTest {
	b := NewBuildTest(t)
	CreateTempDirAndEnter(t)
	return b
}

// Run a straightforward build where the deps log is used.
func TestBuildWithDepsLogTest_Straightforward(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	// Note: in1 was created by the superclass SetUp().
	manifest := "build out: cat in1\n  deps = gcc\n  depfile = in1.d\n"
	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal(err)
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs_.Create("in1.d", "out: in2")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// The deps file should have been removed.
		if 0 != b.fs_.Stat("in1.d", &err) {
			t.Fatal("expected equal")
		}
		// Recreate it for the next step.
		b.fs_.Create("in1.d", "out: in2")
		deps_log.Close()
		builder.command_runner_ = nil
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Touch the file only mentioned in the deps.
		b.fs_.Tick()
		b.fs_.Create("in2", "")

		// Run the build again.
		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		b.command_runner_.commands_ran_ = nil
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// We should have rebuilt the output due to in2 being
		// out of date.
		if 1 != len(b.command_runner_.commands_ran_) {
			t.Fatal("expected equal")
		}

		builder.command_runner_ = nil
	}
}

// Verify that obsolete dependency info causes a rebuild.
// 1) Run a successful build where everything has time t, record deps.
// 2) Move input/output to time t+1 -- despite files in alignment,
//    should still need to rebuild due to deps at older time.
func TestBuildWithDepsLogTest_ObsoleteDeps(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	// Note: in1 was created by the superclass SetUp().
	manifest := "build out: cat in1\n  deps = gcc\n  depfile = in1.d\n"
	{
		// Run an ordinary build that gathers dependencies.
		b.fs_.Create("in1", "")
		b.fs_.Create("in1.d", "out: ")

		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	// Push all files one tick forward so that only the deps are out
	// of date.
	b.fs_.Tick()
	b.fs_.Create("in1", "")
	b.fs_.Create("out", "")

	// The deps file should have been removed, so no need to timestamp it.
	if 0 != b.fs_.Stat("in1.d", &err) {
		t.Fatal("expected equal")
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		b.command_runner_.commands_ran_ = nil
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Recreate the deps file here because the build expects them to exist.
		b.fs_.Create("in1.d", "out: ")

		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// We should have rebuilt the output due to the deps being
		// out of date.
		if 1 != len(b.command_runner_.commands_ran_) {
			t.Fatal("expected equal")
		}

		builder.command_runner_ = nil
	}
}

func TestBuildWithDepsLogTest_DepsIgnoredInDryRun(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	manifest := "build out: cat in1\n  deps = gcc\n  depfile = in1.d\n"

	b.fs_.Create("out", "")
	b.fs_.Tick()
	b.fs_.Create("in1", "")

	state := NewState()
	b.AddCatRule(&state)
	b.AssertParse(&state, manifest, ManifestParserOptions{})

	// The deps log is NULL in dry runs.
	b.config_.dry_run = true
	builder := NewBuilder(&state, &b.config_, nil, nil, &b.fs_, &b.status_, 0)
	builder.command_runner_ = &b.command_runner_
	b.command_runner_.commands_ran_ = nil

	err := ""
	if builder.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	builder.command_runner_ = nil
}

// Check that a restat rule generating a header cancels compilations correctly.
func TestBuildTest_RestatDepfileDependency(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat in1\n  depfile = in1.d\n", ManifestParserOptions{}) // Would be "write if out-of-date" in reality

	b.fs_.Create("header.h", "")
	b.fs_.Create("in1.d", "out: header.h")
	b.fs_.Tick()
	b.fs_.Create("header.in", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

// Check that a restat rule generating a header cancels compilations correctly,
// depslog case.
func TestBuildWithDepsLogTest_RestatDepfileDependencyDepsLog(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	// Note: in1 was created by the superclass SetUp().
	manifest := "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat in1\n  deps = gcc\n  depfile = in1.d\n" // Would be "write if out-of-date" in reality.
	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs_.Create("in1.d", "out: header.h")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Touch the input of the restat rule.
		b.fs_.Tick()
		b.fs_.Create("header.in", "")

		// Run the build again.
		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		b.command_runner_.commands_ran_ = nil
		if builder.AddTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Rule "true" should have run again, but the build of "out" should have
		// been cancelled due to restat propagating through the depfile header.
		if 1 != len(b.command_runner_.commands_ran_) {
			t.Fatal("expected equal")
		}

		builder.command_runner_ = nil
	}
}

func TestBuildWithDepsLogTest_DepFileOKDepsLog(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\nbuild fo$ o.o: cc foo.c\n"

	b.fs_.Create("foo.c", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs_.Create("fo o.o.d", "fo\\ o.o: blah.h bar.h\n")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_

		edge := state.edges_[len(state.edges_)-1]

		state.GetNode("bar.h", 0).MarkDirty() // Mark bar.h as missing.
		if builder.AddTargetName("fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Expect three new edges: one generating fo o.o, and two more from
		// loading the depfile.
		if 3 != len(state.edges_) {
			t.Fatal("expected equal")
		}
		// Expect our edge to now have three inputs: foo.c and two headers.
		if 3 != len(edge.inputs_) {
			t.Fatal("expected equal")
		}

		// Expect the command line we generate to only use the original input.
		if "cc foo.c" != edge.EvaluateCommand(false) {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}
}

func TestBuildWithDepsLogTest_DiscoveredDepDuringBuildChanged(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule touch-out-implicit-dep\n  command = touch $out ; sleep 1 ; touch $test_dependency\nrule generate-depfile\n  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\nbuild out1: touch-out-implicit-dep in1\n  test_dependency = inimp\nbuild out2: generate-depfile in1 || out1\n  test_dependency = inimp\n  depfile = out2.d\n  deps = gcc\n"

	b.fs_.Create("in1", "")
	b.fs_.Tick()

	build_log := NewBuildLog()

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, &build_log, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out2", &err) == nil {
			t.Fatal("expected true")
		}
		if builder.AlreadyUpToDate() {
			t.Fatal("expected false")
		}

		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if !builder.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	b.fs_.Tick()
	b.fs_.Create("in1", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, &build_log, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out2", &err) == nil {
			t.Fatal("expected true")
		}
		if builder.AlreadyUpToDate() {
			t.Fatal("expected false")
		}

		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if !builder.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	b.fs_.Tick()

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, &build_log, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("out2", &err) == nil {
			t.Fatal("expected true")
		}
		if !builder.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}
}

func TestBuildWithDepsLogTest_DepFileDepsLogCanonicalize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\nbuild a/b\\c\\d/e/fo$ o.o: cc x\\y/z\\foo.c\n"

	b.fs_.Create("x/y/z/foo.c", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		var deps_log DepsLog
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_
		if builder.AddTargetName("a/b/c/d/e/fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		// Note, different slashes from manifest.
		b.fs_.Create("a/b\\c\\d/e/fo o.o.d", "a\\b\\c\\d\\e\\fo\\ o.o: blah.h bar.h\n")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		var deps_log DepsLog
		if deps_log.Load("ninja_deps", &state, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if !deps_log.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config_, nil, &deps_log, &b.fs_, &b.status_, 0)
		builder.command_runner_ = &b.command_runner_

		edge := state.edges_[len(state.edges_)-1]

		state.GetNode("bar.h", 0).MarkDirty() // Mark bar.h as missing.
		if builder.AddTargetName("a/b/c/d/e/fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Expect three new edges: one generating fo o.o, and two more from
		// loading the depfile.
		if 3 != len(state.edges_) {
			t.Fatal("expected equal")
		}
		// Expect our edge to now have three inputs: foo.c and two headers.
		if 3 != len(edge.inputs_) {
			t.Fatal("expected equal")
		}

		// Expect the command line we generate to only use the original input.
		// Note, slashes from manifest, not .d.
		if "cc x\\y/z\\foo.c" != edge.EvaluateCommand(false) {
			t.Fatal("expected equal")
		}

		deps_log.Close()
		builder.command_runner_ = nil
	}
}

// Check that a restat rule doesn't clear an edge if the depfile is missing.
// Follows from: https://github.com/ninja-build/ninja/issues/603
func TestBuildTest_RestatMissingDepfile(t *testing.T) {
	b := NewBuildTest(t)
	manifest := "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat header.h\n  depfile = out.d\n" // Would be "write if out-of-date" in reality.

	b.fs_.Create("header.h", "")
	b.fs_.Tick()
	b.fs_.Create("out", "")
	b.fs_.Create("header.in", "")

	// Normally, only 'header.h' would be rebuilt, as
	// its rule doesn't touch the output and has 'restat=1' set.
	// But we are also missing the depfile for 'out',
	// which should force its command to run anyway!
	b.RebuildTarget("out", manifest, "", "", nil)
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

// Check that a restat rule doesn't clear an edge if the deps are missing.
// https://github.com/ninja-build/ninja/issues/603
func TestBuildWithDepsLogTest_RestatMissingDepfileDepslog(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	manifest := "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat header.h\n  deps = gcc\n  depfile = out.d\n" // Would be "write if out-of-date" in reality.

	// Build once to populate ninja deps logs from out.d
	b.fs_.Create("header.in", "")
	b.fs_.Create("out.d", "out: header.h")
	b.fs_.Create("header.h", "")

	b.RebuildTarget("out", manifest, "build_log", "ninja_deps", nil)
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// Sanity: this rebuild should be NOOP
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps", nil)
	if 0 != len(b.command_runner_.commands_ran_) {
		t.Fatalf("Expected no command; %#v", b.command_runner_.commands_ran_)
	}

	// Touch 'header.in', blank dependencies log (create a different one).
	// Building header.h triggers 'restat' outputs cleanup.
	// Validate that out is rebuilt netherless, as deps are missing.
	b.fs_.Tick()
	b.fs_.Create("header.in", "")

	// (switch to a new blank deps_log "ninja_deps2")
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// Sanity: this build should be NOOP
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 0 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// Check that invalidating deps by target timestamp also works here
	// Repeat the test but touch target instead of blanking the log.
	b.fs_.Tick()
	b.fs_.Create("header.in", "")
	b.fs_.Create("out", "")
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 2 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}

	// And this build should be NOOP again
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 0 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_WrongOutputInDepfileCausesRebuild(t *testing.T) {
	b := NewBuildTest(t)
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n"

	b.fs_.Create("foo.c", "")
	b.fs_.Create("foo.o", "")
	b.fs_.Create("header.h", "")
	b.fs_.Create("foo.o.d", "bar.o.d: header.h\n")

	b.RebuildTarget("foo.o", manifest, "build_log", "ninja_deps", nil)
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_Console(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state_, "rule console\n  command = console\n  pool = console\nbuild cons: console in.txt\n", ManifestParserOptions{})

	b.fs_.Create("in.txt", "")

	err := ""
	if b.builder_.AddTargetName("cons", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepMissingAndNoRule(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that we can diagnose when a dyndep file is missing and
	// has no rule to build it.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})

	err := ""
	if b.builder_.AddTargetName("out", &err) != nil {
		t.Fatal("expected false")
	}
	if "loading 'dd': No such file or directory" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepReadyImplicitConnection(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be loaded immediately to discover
	// that one edge has an implicit output that is also an implicit
	// input of another edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep | tmp.imp\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepReadySyntaxError(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be loaded immediately to discover
	// and reject a syntax error in it.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd", "build out: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) != nil {
		t.Fatal("expected false")
	}
	if "dd:1: expected 'ninja_dyndep_version = ...'\n" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepReadyCircular(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be loaded immediately to discover
	// and reject a circular dependency.
	b.AssertParse(&b.state_, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\nbuild in: r circ\n", ManifestParserOptions{})
	b.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\n")
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) != nil {
		t.Fatal("expected false")
	}
	if "dependency cycle: circ -> in -> circ" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepBuild(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover nothing.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	want_created := map[string]struct{}{"dd-in": {}, "in1": {}, "in2": {}}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	want_command := []string{"cp dd-in dd", "touch out"}
	if diff := cmp.Diff(want_command, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
	want_files_read := []string{"dd-in", "dd"}
	if diff := cmp.Diff(want_files_read, b.fs_.files_read_); diff != "" {
		t.Fatal(diff)
	}
	want_created["dd"] = struct{}{}
	want_created["out"] = struct{}{}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildSyntaxError(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// and reject a syntax error in it.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd-in", "build out: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "dd:1: expected 'ninja_dyndep_version = ...'\n" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepBuildUnrelatedOutput(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can have dependents that do not specify
	// it as their dyndep binding.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild unrelated: touch || dd\nbuild out: touch unrelated || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\n")
	b.fs_.Tick()
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch unrelated", "touch out"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewOutput(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new output of an edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("in", "")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep\n")
	b.fs_.Tick()
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewOutputWithMultipleRules1(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new output of an edge that is already the output of another edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out1 | out-twice.imp: touch in\nbuild out2: touch in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("in", "")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")
	b.fs_.Tick()
	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "multiple rules generate out-twice.imp" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepBuildDiscoverNewOutputWithMultipleRules2(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new output of an edge that is already the output of another
	// edge also discovered by dyndep.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || dd1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // make order predictable for test
	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1 | out-twice.imp: dyndep\n")
	b.fs_.Create("dd2-in", "")
	b.fs_.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")
	b.fs_.Tick()
	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	if "multiple rules generate out-twice.imp" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepBuildDiscoverNewInput(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new input to an edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild in: touch\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	b.fs_.Tick()
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch in", "touch out"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverImplicitConnection(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that one edge has an implicit output that is also an implicit
	// input of another edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep | tmp.imp\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverOutputAndDepfileInput(t *testing.T) {
	// WARNING: I (maruel) am not 100% sure about this test case.
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that one edge has an implicit output that is also reported by
	// a depfile as an input of another edge.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: cp tmp\n  depfile = out.d\n", ManifestParserOptions{})
	b.fs_.Create("out.d", "out: tmp.imp\n")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Loading the depfile gave tmp.imp a phony input edge.
	if !b.GetNode("tmp.imp").in_edge().is_phony() {
		t.Fatal("expected true")
	}

	want_created := map[string]struct{}{
		"dd-in": {},
		"in1":   {},
		"in2":   {},
		"out.d": {},
	}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Loading the dyndep file gave tmp.imp a real input edge.
	if b.GetNode("tmp.imp").in_edge().is_phony() {
		t.Fatal("expected false")
	}

	want_commands := []string{"cp dd-in dd", "touch tmp tmp.imp", "cp tmp out"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
	want_created["dd"] = struct{}{}
	want_created["out"] = struct{}{}
	want_created["tmp"] = struct{}{}
	want_created["tmp.imp"] = struct{}{}
	if diff := cmp.Diff(want_created, b.fs_.files_created_); diff != "" {
		t.Fatal(diff)
	}
	if !b.builder_.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdge(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge is actually wanted due to a missing implicit output.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch tmp || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("tmp", "")
	b.fs_.Create("out", "")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdgeAndDependent(t *testing.T) {
	t.Skip("TODO")
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge and a dependent are actually wanted.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch tmp\n", ManifestParserOptions{})
	b.fs_.Create("tmp", "")
	b.fs_.Create("out", "")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	/* Ugh
	fmt.Printf("State:\n")
	b.state_.Dump()
	fmt.Printf("Plan:\n")
	b.builder_.plan_.Dump()
	*/

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}

	/* Ugh
	fmt.Printf("After:\n")
	fmt.Printf("State:\n")
	b.state_.Dump()
	fmt.Printf("Plan:\n")
	b.builder_.plan_.Dump()
	*/
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverCircular(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// and reject a circular dependency.
	b.AssertParse(&b.state_, "rule r\n  command = unused\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: r in || dd\n  depfile = out.d\n  dyndep = dd\nbuild in: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs_.Create("out.d", "out: inimp\n")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\nbuild in: dyndep | circ\n")
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder_.Build(&err) {
		t.Fatal("expected false")
	}
	// Depending on how the pointers in ready_ work out, we could have
	// discovered the cycle from either starting point.
	if err != "dependency cycle: circ -> in -> circ" && err != "dependency cycle: in -> circ -> in" {
		t.Fatal(err)
	}
}

func TestBuildWithLogTest_DyndepBuildDiscoverRestat(t *testing.T) {
	b := NewBuildWithLogTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge has a restat binding.
	b.AssertParse(&b.state_, "rule true\n  command = true\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out1: true in || dd\n  dyndep = dd\nbuild out2: cat out1\n", ManifestParserOptions{})

	b.fs_.Create("out1", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("dd-in", "ninja_dyndep_version = 1\nbuild out1: dyndep\n  restat = 1\n")
	b.fs_.Tick()
	b.fs_.Create("in", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause "out2" to
	// rebuild regardless of restat.
	err := ""
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd-in dd", "true", "cat out1 > out2"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}

	b.command_runner_.commands_ran_ = nil
	b.state_.Reset()
	b.fs_.Tick()
	b.fs_.Create("in", "")

	// We touched "in", so we should build "out1".  But because "true" does not
	// touch "out1", we should cancel the build of "out2".
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	want_commands = []string{"true"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverScheduledEdge(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover a
	// new input that itself is an output from an edge that has already
	// been scheduled but not finished.  We should not re-schedule it.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild out1 | out1.imp: touch\nbuild zdd: cp zdd-in\n  verify_active_edge = out1\nbuild out2: cp out1 || zdd\n  dyndep = zdd\n", ManifestParserOptions{}) // verify out1 is active when zdd is finished
	b.fs_.Create("zdd-in", "ninja_dyndep_version = 1\nbuild out2: dyndep | out1.imp\n")

	// Enable concurrent builds so that we can load the dyndep file
	// while another edge is still active.
	b.command_runner_.max_active_edges_ = 2

	// During the build "out1" and "zdd" should be built concurrently.
	// The fake command runner will finish these in reverse order
	// of the names of the first outputs, so "zdd" will finish first
	// and we will load the dyndep file while the edge for "out1" is
	// still active.  This will add a new dependency on "out1.imp",
	// also produced by the active edge.  The builder should not
	// re-schedule the already-active edge.

	err := ""
	if b.builder_.AddTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.command_runner_.commands_ran_) {
		t.Fatal("expected equal")
	}
	// Depending on how the pointers in ready_ work out, the first
	// two commands may have run in either order.
	if !(b.command_runner_.commands_ran_[0] == "touch out1 out1.imp" && b.command_runner_.commands_ran_[1] == "cp zdd-in zdd") || (b.command_runner_.commands_ran_[1] == "touch out1 out1.imp" && b.command_runner_.commands_ran_[0] == "cp zdd-in zdd") {
		t.Fatal("expected true")
	}
	if "cp out1 out2" != b.command_runner_.commands_ran_[2] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepTwoLevelDirect(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a clean dyndep file can depend on a dirty dyndep file
	// and be loaded properly after the dirty one is built and loaded.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1 | out1.imp: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || dd1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // direct order-only dep on dd1
	b.fs_.Create("out1.imp", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("out2.imp", "")
	b.fs_.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1: dyndep\n")
	b.fs_.Create("dd2-in", "")
	b.fs_.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out2.imp: dyndep | out1.imp\n")

	// During the build dd1 should be built and loaded.  The RecomputeDirty
	// called as a result of loading dd1 should not cause dd2 to be loaded
	// because the builder will never get a chance to update the build plan
	// to account for dd2.  Instead dd2 should only be later loaded once the
	// builder recognizes that it is now ready (as its order-only dependency
	// on dd1 has been satisfied).  This test case verifies that each dyndep
	// file is loaded to update the build graph independently.

	err := ""
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd1-in dd1", "touch out1 out1.imp", "touch out2 out2.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelIndirect(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that dyndep files can add to an edge new implicit inputs that
	// correspond to implicit outputs added to other edges by other dyndep
	// files on which they (order-only) depend.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || out1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // indirect order-only dep on dd1
	b.fs_.Create("out1.imp", "")
	b.fs_.Create("out2", "")
	b.fs_.Create("out2.imp", "")
	b.fs_.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1 | out1.imp: dyndep\n")
	b.fs_.Create("dd2-in", "")
	b.fs_.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out2.imp: dyndep | out1.imp\n")

	// During the build dd1 should be built and loaded.  Then dd2 should
	// be built and loaded.  Loading dd2 should cause the builder to
	// recognize that out2 needs to be built even though it was originally
	// clean without dyndep info.

	err := ""
	if b.builder_.AddTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd1-in dd1", "touch out1 out1.imp", "touch out2 out2.imp"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelDiscoveredReady(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can discover a new input whose
	// edge also has a dyndep file that is ready to load immediately.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd0: cp dd0-in\nbuild dd1: cp dd1-in\nbuild in: touch\nbuild tmp: touch || dd0\n  dyndep = dd0\nbuild out: touch || dd1\n  dyndep = dd1\n", ManifestParserOptions{})
	b.fs_.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out: dyndep | tmp\n")
	b.fs_.Create("dd0-in", "")
	b.fs_.Create("dd0", "ninja_dyndep_version = 1\nbuild tmp: dyndep | in\n")
	b.fs_.Tick()
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd1-in dd1", "touch in", "touch tmp", "touch out"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelDiscoveredDirty(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can discover a new input whose
	// edge also has a dyndep file that needs to be built.
	b.AssertParse(&b.state_, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd0: cp dd0-in\nbuild dd1: cp dd1-in\nbuild in: touch\nbuild tmp: touch || dd0\n  dyndep = dd0\nbuild out: touch || dd1\n  dyndep = dd1\n", ManifestParserOptions{})
	b.fs_.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out: dyndep | tmp\n")
	b.fs_.Create("dd0-in", "ninja_dyndep_version = 1\nbuild tmp: dyndep | in\n")
	b.fs_.Tick()
	b.fs_.Create("out", "")

	err := ""
	if b.builder_.AddTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder_.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	want_commands := []string{"cp dd1-in dd1", "cp dd0-in dd0", "touch in", "touch tmp", "touch out"}
	if diff := cmp.Diff(want_commands, b.command_runner_.commands_ran_); diff != "" {
		t.Fatal(diff)
	}
}
