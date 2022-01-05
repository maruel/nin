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
	"errors"
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
	plan plan
}

func NewPlanTest(t *testing.T) *PlanTest {
	return &PlanTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		plan:                      newPlan(nil),
	}
}

// Because FindWork does not return Edges in any sort of predictable order,
// provide a means to get available Edges in order and in a format which is
// easy to write tests around.
func (p *PlanTest) FindWorkSorted(count int) []*Edge {
	var out []*Edge
	for i := 0; i < count; i++ {
		if !p.plan.moreToDo() {
			p.t.Fatal("expected true")
		}
		edge := p.plan.findWork()
		if edge == nil {
			p.t.Fatal("expected true")
		}
		out = append(out, edge)
	}
	if p.plan.findWork() != nil {
		p.t.Fatal("expected false")
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Outputs[0].Path < out[j].Outputs[0].Path
	})
	return out
}

func TestPlanTest_Basic(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	p.GetNode("mid").Dirty = true
	p.GetNode("out").Dirty = true
	err := ""
	if !p.plan.addTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		t.Fatalf("plan is inconsistent: %#v", p.plan)
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "mid" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	if e := p.plan.findWork(); e != nil {
		t.Fatalf("%#v", e)
	}

	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "mid" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan.moreToDo() {
		t.Fatal("expected false")
	}
	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected equal")
	}
}

// Test that two outputs from one rule can be handled as inputs to the next.
func TestPlanTest_DoubleOutputDirect(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "build out: cat mid1 mid2\nbuild mid1 mid2: cat in\n", ManifestParserOptions{})
	p.GetNode("mid1").Dirty = true
	p.GetNode("mid2").Dirty = true
	p.GetNode("out").Dirty = true

	err := ""
	if !p.plan.addTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid1 mid2
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

// Test that two outputs from one rule can eventually be routed to another.
func TestPlanTest_DoubleOutputIndirect(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "build out: cat b1 b2\nbuild b1: cat a1\nbuild b2: cat a2\nbuild a1 a2: cat in\n", ManifestParserOptions{})
	p.GetNode("a1").Dirty = true
	p.GetNode("a2").Dirty = true
	p.GetNode("b1").Dirty = true
	p.GetNode("b2").Dirty = true
	p.GetNode("out").Dirty = true
	err := ""
	if !p.plan.addTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a1
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a2
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat b1 b2
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

// Test that two edges from one output can both execute.
func TestPlanTest_DoubleDependent(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "build out: cat a1 a2\nbuild a1: cat mid\nbuild a2: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	p.GetNode("mid").Dirty = true
	p.GetNode("a1").Dirty = true
	p.GetNode("a2").Dirty = true
	p.GetNode("out").Dirty = true

	err := ""
	if !p.plan.addTarget(p.GetNode("out"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat in
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat mid
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	} // cat a1 a2
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected false")
	} // done
}

func (p *PlanTest) TestPoolWithDepthOne(testCase string) {
	p.AssertParse(&p.state, testCase, ManifestParserOptions{})
	p.GetNode("out1").Dirty = true
	p.GetNode("out2").Dirty = true
	err := ""
	if !p.plan.addTarget(p.GetNode("out1"), &err) {
		p.t.Fatal("expected true")
	}
	if "" != err {
		p.t.Fatal("expected equal")
	}
	if !p.plan.addTarget(p.GetNode("out2"), &err) {
		p.t.Fatal("expected true")
	}
	if "" != err {
		p.t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		p.t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		p.t.Fatal("expected true")
	}
	if "in" != edge.Inputs[0].Path {
		p.t.Fatal("expected equal")
	}
	if "out1" != edge.Outputs[0].Path {
		p.t.Fatal("expected equal")
	}

	// This will be false since poolcat is serialized
	if p.plan.findWork() != nil {
		p.t.Fatal("expected false")
	}

	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		p.t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		p.t.Fatal("expected true")
	}
	if "in" != edge.Inputs[0].Path {
		p.t.Fatal("expected equal")
	}
	if "out2" != edge.Outputs[0].Path {
		p.t.Fatal("expected equal")
	}

	if p.plan.findWork() != nil {
		p.t.Fatal("expected false")
	}

	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		p.t.Fatal("expected equal")
	}

	if p.plan.moreToDo() {
		p.t.Fatal("expected false")
	}
	edge = p.plan.findWork()
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
	p.AssertParse(&p.state, "pool foobar\n  depth = 2\npool bazbin\n  depth = 2\nrule foocat\n  command = cat $in > $out\n  pool = foobar\nrule bazcat\n  command = cat $in > $out\n  pool = bazbin\nbuild out1: foocat in\nbuild out2: foocat in\nbuild out3: foocat in\nbuild outb1: bazcat in\nbuild outb2: bazcat in\nbuild outb3: bazcat in\n  pool =\nbuild allTheThings: cat out1 out2 out3 outb1 outb2 outb3\n", ManifestParserOptions{})
	// Mark all the out* nodes dirty
	for i := 0; i < 3; i++ {
		p.GetNode(fmt.Sprintf("out%d", i+1)).Dirty = true
		p.GetNode(fmt.Sprintf("outb%d", i+1)).Dirty = true
	}
	p.GetNode("allTheThings").Dirty = true

	err := ""
	if !p.plan.addTarget(p.GetNode("allTheThings"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	edges := p.FindWorkSorted(5)

	for i := 0; i < 4; i++ {
		edge := edges[i]
		if "in" != edge.Inputs[0].Path {
			t.Fatal("expected equal")
		}
		baseName := "outb"
		if i < 2 {
			baseName = "out"
		}
		if want := fmt.Sprintf("%s%d", baseName, 1+(i%2)); want != edge.Outputs[0].Path {
			t.Fatal(want)
		}
	}

	// outb3 is exempt because it has an empty pool
	edge := edges[4]
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "outb3" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	// finish out1
	p.plan.edgeFinished(edges[0], EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}
	edges = edges[1:]

	// out3 should be available
	out3 := p.plan.findWork()
	if out3 == nil {
		t.Fatal("expected true")
	}
	if "in" != out3.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out3" != out3.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}

	p.plan.edgeFinished(out3, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}

	for _, it := range edges {
		p.plan.edgeFinished(it, EdgeSucceeded, &err)
		if "" != err {
			t.Fatal("expected equal")
		}
	}

	last := p.plan.findWork()
	if last == nil {
		t.Fatal("expected true")
	}
	if "allTheThings" != last.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	p.plan.edgeFinished(last, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if p.plan.moreToDo() {
		t.Fatal("expected false")
	}
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}
}

func TestPlanTest_PoolWithRedundantEdges(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "pool compile\n  depth = 1\nrule gen_foo\n  command = touch foo.cpp\nrule gen_bar\n  command = touch bar.cpp\nrule echo\n  command = echo $out > $out\nbuild foo.cpp.obj: echo foo.cpp || foo.cpp\n  pool = compile\nbuild bar.cpp.obj: echo bar.cpp || bar.cpp\n  pool = compile\nbuild libfoo.a: echo foo.cpp.obj bar.cpp.obj\nbuild foo.cpp: gen_foo\nbuild bar.cpp: gen_bar\nbuild all: phony libfoo.a\n", ManifestParserOptions{})
	p.GetNode("foo.cpp").Dirty = true
	p.GetNode("foo.cpp.obj").Dirty = true
	p.GetNode("bar.cpp").Dirty = true
	p.GetNode("bar.cpp.obj").Dirty = true
	p.GetNode("libfoo.a").Dirty = true
	p.GetNode("all").Dirty = true
	err := ""
	if !p.plan.addTarget(p.GetNode("all"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	initialEdges := p.FindWorkSorted(2)

	edge := initialEdges[1] // Foo first
	if "foo.cpp" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}
	if "foo.cpp" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "foo.cpp" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "foo.cpp.obj" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = initialEdges[0] // Now for bar
	if "bar.cpp" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}
	if "bar.cpp" != edge.Inputs[0].Path {
		t.Fatal(edge.Inputs[0].Path)
	}
	if "bar.cpp" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "bar.cpp.obj" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}
	if "foo.cpp.obj" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "bar.cpp.obj" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "libfoo.a" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}
	if "libfoo.a" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "all" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}
	p.plan.edgeFinished(edge, EdgeSucceeded, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected false")
	}
	if p.plan.moreToDo() {
		t.Fatal("expected false")
	}
}

func TestPlanTest_PoolWithFailingEdge(t *testing.T) {
	p := NewPlanTest(t)
	p.AssertParse(&p.state, "pool foobar\n  depth = 1\nrule poolcat\n  command = cat $in > $out\n  pool = foobar\nbuild out1: poolcat in\nbuild out2: poolcat in\n", ManifestParserOptions{})
	p.GetNode("out1").Dirty = true
	p.GetNode("out2").Dirty = true
	err := ""
	if !p.plan.addTarget(p.GetNode("out1"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.addTarget(p.GetNode("out2"), &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	}

	edge := p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out1" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	// This will be false since poolcat is serialized
	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}

	p.plan.edgeFinished(edge, EdgeFailed, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	edge = p.plan.findWork()
	if edge == nil {
		t.Fatal("expected true")
	}
	if "in" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "out2" != edge.Outputs[0].Path {
		t.Fatal("expected equal")
	}

	if p.plan.findWork() != nil {
		t.Fatal("expected false")
	}

	p.plan.edgeFinished(edge, EdgeFailed, &err)
	if "" != err {
		t.Fatal("expected equal")
	}

	if !p.plan.moreToDo() {
		t.Fatal("expected true")
	} // Jobs have failed
	edge = p.plan.findWork()
	if edge != nil {
		t.Fatal("expected equal")
	}
}

type BuildTestBase struct {
	StateTestWithBuiltinRules
	config        BuildConfig
	commandRunner FakeCommandRunner
	fs            VirtualFileSystem
	status        *StatusPrinter
}

func NewBuildTestBase(t *testing.T) *BuildTestBase {
	b := &BuildTestBase{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		config:                    NewBuildConfig(),
		fs:                        NewVirtualFileSystem(),
	}
	b.config.Verbosity = Quiet
	b.commandRunner = NewFakeCommandRunner(t, &b.fs)
	//b.builder = NewBuilder(&b.state, &b.config, nil, nil, &b.fs, b.status, 0)
	b.status = NewStatusPrinter(&b.config)
	//b.builder.commandRunner = &b.commandRunner
	b.AssertParse(&b.state, "build cat1: cat in1\nbuild cat2: cat in1 in2\nbuild cat12: cat cat1 cat2\n", ManifestParserOptions{})
	b.fs.Create("in1", "")
	b.fs.Create("in2", "")
	return b
}

func (b *BuildTestBase) IsPathDead(s string) bool {
	return false
}

// Rebuild target in the 'working tree' (fs).
// State of commandRunner and logs contents (if specified) ARE MODIFIED.
// Handy to check for NOOP builds, and higher-level rebuild tests.
func (b *BuildTestBase) RebuildTarget(target, manifest, logPath, depsPath string, state *State) {
	pstate := state
	if pstate == nil {
		localState := NewState()
		pstate = &localState
	}
	b.AddCatRule(pstate)
	b.AssertParse(pstate, manifest, ManifestParserOptions{})

	err := ""
	var pbuildLog *BuildLog
	if logPath != "" {
		buildLog := NewBuildLog()
		defer buildLog.Close()
		if s := buildLog.Load(logPath, &err); s != LoadSuccess && s != LoadNotFound {
			b.t.Fatalf("%s = %d: %s", logPath, s, err)
		}
		if !buildLog.OpenForWrite(logPath, b, &err) {
			b.t.Fatal(err)
		}
		if "" != err {
			b.t.Fatal(err)
		}
		pbuildLog = &buildLog
	}

	var pdepsLog *DepsLog
	if depsPath != "" {
		pdepsLog = &DepsLog{}
		defer pdepsLog.Close()
		if s := pdepsLog.Load(depsPath, pstate, &err); s != LoadSuccess && s != LoadNotFound {
			b.t.Fatalf("%s = %d: %s", depsPath, s, err)
		}
		if !pdepsLog.OpenForWrite(depsPath, &err) {
			b.t.Fatal("expected true")
		}
		if "" != err {
			b.t.Fatal("expected equal")
		}
	}

	builder := NewBuilder(pstate, &b.config, pbuildLog, pdepsLog, &b.fs, b.status, 0)
	if builder.addTargetName(target, &err) == nil {
		b.t.Fatal(err)
	}

	b.commandRunner.commandsRan = nil
	builder.commandRunner = &b.commandRunner
	if !builder.AlreadyUpToDate() {
		if !builder.Build(&err) {
			b.t.Fatal(err)
		}
	}
}

type BuildTest struct {
	*BuildTestBase
	builder *Builder
}

func NewBuildTest(t *testing.T) *BuildTest {
	b := &BuildTest{
		BuildTestBase: NewBuildTestBase(t),
	}
	b.builder = NewBuilder(&b.state, &b.config, nil, nil, &b.fs, b.status, 0)
	b.builder.commandRunner = &b.commandRunner
	// TODO(maruel): Only do it for tests that write to disk.
	CreateTempDirAndEnter(t)
	return b
}

// Fake implementation of CommandRunner, useful for tests.
type FakeCommandRunner struct {
	t              *testing.T
	commandsRan    []string
	activeEdges    []*Edge
	maxActiveEdges uint
	fs             *VirtualFileSystem
}

func NewFakeCommandRunner(t *testing.T, fs *VirtualFileSystem) FakeCommandRunner {
	return FakeCommandRunner{
		t:              t,
		maxActiveEdges: 1,
		fs:             fs,
	}
}

// CommandRunner impl
func (f *FakeCommandRunner) CanRunMore() bool {
	return len(f.activeEdges) < int(f.maxActiveEdges)
}

func (f *FakeCommandRunner) StartCommand(edge *Edge) bool {
	cmd := edge.EvaluateCommand(false)
	//f.t.Logf("StartCommand(%s)", cmd)
	if len(f.activeEdges) > int(f.maxActiveEdges) {
		f.t.Fatal("oops")
	}
	found := false
	for _, a := range f.activeEdges {
		if a == edge {
			found = true
			break
		}
	}
	if found {
		f.t.Fatalf("running same edge twice")
	}
	f.commandsRan = append(f.commandsRan, cmd)
	if edge.Rule.Name == "cat" || edge.Rule.Name == "cat_rsp" || edge.Rule.Name == "cat_rsp_out" || edge.Rule.Name == "cc" || edge.Rule.Name == "cp_multi_msvc" || edge.Rule.Name == "cp_multi_gcc" || edge.Rule.Name == "touch" || edge.Rule.Name == "touch-interrupt" || edge.Rule.Name == "touch-fail-tick2" {
		for _, out := range edge.Outputs {
			f.fs.Create(out.Path, "")
		}
	} else if edge.Rule.Name == "true" || edge.Rule.Name == "fail" || edge.Rule.Name == "interrupt" || edge.Rule.Name == "console" {
		// Don't do anything.
	} else if edge.Rule.Name == "cp" {
		if len(edge.Inputs) == 0 {
			f.t.Fatal("oops")
		}
		if len(edge.Outputs) != 1 {
			f.t.Fatalf("%#v", edge.Outputs)
		}
		content, err := f.fs.ReadFile(edge.Inputs[0].Path)
		if err == nil {
			// ReadFile append a zero byte, strip it when writing back.
			c := content
			if len(c) != 0 {
				c = c[:len(c)-1]
			}
			f.fs.WriteFile(edge.Outputs[0].Path, string(c))
		}
	} else if edge.Rule.Name == "touch-implicit-dep-out" {
		dep := edge.GetBinding("test_dependency")
		f.fs.Create(dep, "")
		f.fs.Tick()
		for _, out := range edge.Outputs {
			f.fs.Create(out.Path, "")
		}
	} else if edge.Rule.Name == "touch-out-implicit-dep" {
		dep := edge.GetBinding("test_dependency")
		for _, out := range edge.Outputs {
			f.fs.Create(out.Path, "")
		}
		f.fs.Tick()
		f.fs.Create(dep, "")
	} else if edge.Rule.Name == "generate-depfile" {
		dep := edge.GetBinding("test_dependency")
		depfile := edge.GetUnescapedDepfile()
		contents := ""
		for _, out := range edge.Outputs {
			contents += out.Path + ": " + dep + "\n"
			f.fs.Create(out.Path, "")
		}
		f.fs.Create(depfile, contents)
	} else {
		fmt.Printf("unknown command\n")
		return false
	}

	f.activeEdges = append(f.activeEdges, edge)

	// Allow tests to control the order by the name of the first output.
	sort.Slice(f.activeEdges, func(i, j int) bool {
		return f.activeEdges[i].Outputs[0].Path < f.activeEdges[j].Outputs[0].Path
	})
	return true
}

func (f *FakeCommandRunner) WaitForCommand(result *Result) bool {
	if len(f.activeEdges) == 0 {
		return false
	}

	// All active edges were already completed immediately when started,
	// so we can pick any edge here.  Pick the last edge.  Tests can
	// control the order of edges by the name of the first output.
	edgeIter := len(f.activeEdges) - 1

	edge := f.activeEdges[edgeIter]
	result.Edge = edge

	if edge.Rule.Name == "interrupt" || edge.Rule.Name == "touch-interrupt" {
		result.ExitCode = ExitInterrupted
		return true
	}

	if edge.Rule.Name == "console" {
		if edge.Pool == ConsolePool {
			result.ExitCode = ExitSuccess
		} else {
			result.ExitCode = ExitFailure
		}
		copy(f.activeEdges[edgeIter:], f.activeEdges[edgeIter+1:])
		f.activeEdges = f.activeEdges[:len(f.activeEdges)-1]
		return true
	}

	if edge.Rule.Name == "cp_multi_msvc" {
		prefix := edge.GetBinding("msvc_deps_prefix")
		for _, in := range edge.Inputs {
			result.Output += prefix + in.Path + "\n"
		}
	}

	if edge.Rule.Name == "fail" || (edge.Rule.Name == "touch-fail-tick2" && f.fs.now == 2) {
		result.ExitCode = ExitFailure
	} else {
		result.ExitCode = ExitSuccess
	}

	// Provide a way for test cases to verify when an edge finishes that
	// some other edge is still active.  This is useful for test cases
	// covering behavior involving multiple active edges.
	verifyActiveEdge := edge.GetBinding("verify_active_edge")
	if verifyActiveEdge != "" {
		verifyActiveEdgeFound := false
		for _, i := range f.activeEdges {
			if len(i.Outputs) != 0 && i.Outputs[0].Path == verifyActiveEdge {
				verifyActiveEdgeFound = true
			}
		}
		if !verifyActiveEdgeFound {
			f.t.Fatal("expected true")
		}
	}

	copy(f.activeEdges[edgeIter:], f.activeEdges[edgeIter+1:])
	f.activeEdges = f.activeEdges[:len(f.activeEdges)-1]
	return true
}

func (f *FakeCommandRunner) GetActiveEdges() []*Edge {
	return f.activeEdges
}

func (f *FakeCommandRunner) Abort() {
	f.activeEdges = nil
}

// Mark a path dirty.
func (b *BuildTest) Dirty(path string) {
	node := b.GetNode(path)
	node.Dirty = true

	// If it's an input file, mark that we've already stat()ed it and
	// it's missing.
	if node.InEdge == nil {
		if node.MTime == -1 {
			node.MTime = 0
		}
		node.Exists = ExistenceStatusMissing
	}
}

func TestBuildTest_NoWork(t *testing.T) {
	b := NewBuildTest(t)
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildTest_OneStep(t *testing.T) {
	b := NewBuildTest(t)
	// Given a dirty target with one ready input,
	// we should rebuild the target.
	b.Dirty("cat1")
	err := ""
	if b.builder.addTargetName("cat1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	wantCommands := []string{"cat in1 > cat1"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_OneStep2(t *testing.T) {
	b := NewBuildTest(t)
	// Given a target with one dirty input,
	// we should rebuild the target.
	b.Dirty("cat1")
	err := ""
	if b.builder.addTargetName("cat1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	wantCommands := []string{"cat in1 > cat1"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_TwoStep(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	if b.builder.addTargetName("cat12", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	// Depending on how the pointers work out, we could've ran
	// the first two commands in either order.
	if !(b.commandRunner.commandsRan[0] == "cat in1 > cat1" && b.commandRunner.commandsRan[1] == "cat in1 in2 > cat2") || (b.commandRunner.commandsRan[1] == "cat in1 > cat1" && b.commandRunner.commandsRan[0] == "cat in1 in2 > cat2") {
		t.Fatal("expected true")
	}

	if "cat cat1 cat2 > cat12" != b.commandRunner.commandsRan[2] {
		t.Fatal("expected equal")
	}

	b.fs.Tick()

	// Modifying in2 requires rebuilding one intermediate file
	// and the final file.
	b.fs.Create("in2", "")
	b.state.Reset()
	if b.builder.addTargetName("cat12", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 5 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if "cat in1 in2 > cat2" != b.commandRunner.commandsRan[3] {
		t.Fatal("expected equal")
	}
	if "cat cat1 cat2 > cat12" != b.commandRunner.commandsRan[4] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_TwoOutputs(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nbuild out1 out2: touch in.txt\n", ManifestParserOptions{})

	b.fs.Create("in.txt", "")

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"touch out1 out2"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_ImplicitOutput(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nbuild out | out.imp: touch in.txt\n", ManifestParserOptions{})
	b.fs.Create("in.txt", "")

	err := ""
	if b.builder.addTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

// Test case from
//   https://github.com/ninja-build/ninja/issues/148
func TestBuildTest_MultiOutIn(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nbuild in1 otherfile: touch in\nbuild out: touch in | in1\n", ManifestParserOptions{})

	b.fs.Create("in", "")
	b.fs.Tick()
	b.fs.Create("in1", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_Chain(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build c2: cat c1\nbuild c3: cat c2\nbuild c4: cat c3\nbuild c5: cat c4\n", ManifestParserOptions{})

	b.fs.Create("c1", "")

	err := ""
	if b.builder.addTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 4 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	err = ""
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.fs.Tick()

	b.fs.Create("c3", "")
	err = ""
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("c5", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	} // 3->4, 4->5
}

func TestBuildTest_MissingInput(t *testing.T) {
	b := NewBuildTest(t)
	// Input is referenced by build file, but no rule for it.
	err := ""
	b.Dirty("in1")
	if b.builder.addTargetName("cat1", &err) != nil {
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
	if b.builder.addTargetName("meow", &err) != nil {
		t.Fatal("expected false")
	}
	if "unknown target: 'meow'" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_MissingInputTarget(t *testing.T) {
	b := NewBuildTest(t)
	// Target is a missing input file
	err := ""
	b.Dirty("in1")
	if b.builder.addTargetName("in1", &err) != nil {
		t.Fatal("unexpected success")
	}
	if err != "'in1' missing and no known rule to make it" {
		t.Fatal(err)
	}
}

func TestBuildTest_MakeDirs(t *testing.T) {
	b := NewBuildTest(t)
	err := ""

	p := filepath.Join("subdir", "dir2", "file")
	b.AssertParse(&b.state, "build "+p+": cat in1\n", ManifestParserOptions{})
	if b.builder.addTargetName("subdir/dir2/file", &err) == nil {
		t.Fatal(err)
	}

	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantMade := map[string]struct{}{
		"subdir":                        {},
		filepath.Join("subdir", "dir2"): {},
	}
	if diff := cmp.Diff(wantMade, b.fs.directoriesMade); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DepFileMissing(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild fo$ o.o: cc foo.c\n", ManifestParserOptions{})
	b.fs.Create("foo.c", "")

	if b.builder.addTargetName("fo o.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.fs.filesRead) {
		t.Fatal("expected equal")
	}
	if "fo o.o.d" != b.fs.filesRead[0] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepFileOK(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	origEdges := len(b.state.Edges)
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n", ManifestParserOptions{})
	edge := b.state.Edges[len(b.state.Edges)-1]

	b.fs.Create("foo.c", "")
	b.GetNode("bar.h").Dirty = true // Mark bar.h as missing.
	b.fs.Create("foo.o.d", "foo.o: blah.h bar.h\n")
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.fs.filesRead) {
		t.Fatal("expected equal")
	}
	if "foo.o.d" != b.fs.filesRead[0] {
		t.Fatal("expected equal")
	}

	// Expect three new edges: one generating foo.o, and two more from
	// loading the depfile.
	if origEdges+3 != len(b.state.Edges) {
		t.Fatal("expected equal")
	}
	// Expect our edge to now have three inputs: foo.c and two headers.
	if 3 != len(edge.Inputs) {
		t.Fatalf("%#v", edge.Inputs)
	}

	// Expect the command line we generate to only use the original input.
	if "cc foo.c" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepFileParseError(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n", ManifestParserOptions{})
	b.fs.Create("foo.c", "")
	b.fs.Create("foo.o.d", "randomtext\n")
	if b.builder.addTargetName("foo.o", &err) != nil {
		t.Fatal("expected false")
	}
	if "foo.o.d: expected ':' in depfile" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_EncounterReadyTwice(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nbuild c: touch\nbuild b: touch || c\nbuild a: touch | b || c\n", ManifestParserOptions{})

	cOut := b.GetNode("c").OutEdges
	if 2 != len(cOut) {
		t.Fatal("expected equal")
	}
	if "b" != cOut[0].Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if "a" != cOut[1].Outputs[0].Path {
		t.Fatal("expected equal")
	}

	b.fs.Create("b", "")
	if b.builder.addTargetName("a", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_OrderOnlyDeps(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c || otherfile\n", ManifestParserOptions{})
	edge := b.state.Edges[len(b.state.Edges)-1]

	b.fs.Create("foo.c", "")
	b.fs.Create("otherfile", "")
	b.fs.Create("foo.o.d", "foo.o: blah.h bar.h\n")
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// One explicit, two implicit, one order only.
	if 4 != len(edge.Inputs) {
		t.Fatal("expected equal")
	}
	if 2 != edge.ImplicitDeps {
		t.Fatal("expected equal")
	}
	if 1 != edge.OrderOnlyDeps {
		t.Fatal("expected equal")
	}
	// Verify the inputs are in the order we expect
	// (explicit then implicit then orderonly).
	if "foo.c" != edge.Inputs[0].Path {
		t.Fatal("expected equal")
	}
	if "blah.h" != edge.Inputs[1].Path {
		t.Fatal("expected equal")
	}
	if "bar.h" != edge.Inputs[2].Path {
		t.Fatal("expected equal")
	}
	if "otherfile" != edge.Inputs[3].Path {
		t.Fatal("expected equal")
	}

	// Expect the command line we generate to only use the original input.
	if "cc foo.c" != edge.EvaluateCommand(false) {
		t.Fatal("expected equal")
	}

	// explicit dep dirty, expect a rebuild.
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.fs.Tick()

	// Recreate the depfile, as it should have been deleted by the build.
	b.fs.Create("foo.o.d", "foo.o: blah.h bar.h\n")

	// implicit dep dirty, expect a rebuild.
	b.fs.Create("blah.h", "")
	b.fs.Create("bar.h", "")
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.fs.Tick()

	// Recreate the depfile, as it should have been deleted by the build.
	b.fs.Create("foo.o.d", "foo.o: blah.h bar.h\n")

	// order only dep dirty, no rebuild.
	b.fs.Create("otherfile", "")
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// implicit dep missing, expect rebuild.
	b.fs.RemoveFile("bar.h")
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_RebuildOrderOnlyDeps(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\nrule true\n  command = true\nbuild oo.h: cc oo.h.in\nbuild foo.o: cc foo.c || oo.h\n", ManifestParserOptions{})

	b.fs.Create("foo.c", "")
	b.fs.Create("oo.h.in", "")

	// foo.o and order-only dep dirty, build both.
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// all clean, no rebuild.
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// order-only dep missing, build it only.
	b.fs.RemoveFile("oo.h")
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cc oo.h.in"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	b.fs.Tick()

	// order-only dep dirty, build it only.
	b.fs.Create("oo.h.in", "")
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("foo.o", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands = []string{"cc oo.h.in"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DepFileCanonicalize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	t.Skip("TODO")
	b := NewBuildTest(t)
	err := ""
	origEdges := len(b.state.Edges)
	if origEdges != 3 {
		t.Fatal(origEdges)
	}
	b.AssertParse(&b.state, "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild gen/stuff\\things/foo.o: cc x\\y/z\\foo.c\n", ManifestParserOptions{})
	edge := b.state.Edges[len(b.state.Edges)-1]

	b.fs.Create("x/y/z/foo.c", "")
	b.GetNode("bar.h").Dirty = true // Mark bar.h as missing.
	// Note, different slashes from manifest.
	b.fs.Create("gen/stuff\\things/foo.o.d", "gen\\stuff\\things\\foo.o: blah.h bar.h\n")
	if b.builder.addTargetName("gen/stuff/things/foo.o", &err) == nil {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	// The depfile path does not get Canonicalize as it seems unnecessary.
	wantReads := []string{"gen/stuff\\things/foo.o.d"}
	if diff := cmp.Diff(wantReads, b.fs.filesRead); diff != "" {
		t.Fatal(diff)
	}

	// Expect three new edges: one generating foo.o, and two more from
	// loading the depfile.
	if origEdges+3 != len(b.state.Edges) {
		t.Fatal(len(b.state.Edges))
	}
	// Expect our edge to now have three inputs: foo.c and two headers.
	if 3 != len(edge.Inputs) {
		t.Fatal(len(edge.Inputs))
	}

	// Expect the command line we generate to only use the original input, and
	// using the slashes from the manifest.
	if got := edge.EvaluateCommand(false); got != "cc x\\y/z\\foo.c" {
		t.Fatalf("%q", got)
	}
}

func TestBuildTest_Phony(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "build out: cat bar.cc\nbuild all: phony out\n", ManifestParserOptions{})
	b.fs.Create("bar.cc", "")

	if b.builder.addTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Only one command to run, because phony runs no command.
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PhonyNoWork(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "build out: cat bar.cc\nbuild all: phony out\n", ManifestParserOptions{})
	b.fs.Create("bar.cc", "")
	b.fs.Create("out", "")

	if b.builder.addTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

// Test a self-referencing phony.  Ideally this should not work, but
// ninja 1.7 and below tolerated and CMake 2.8.12.x and 3.0.x both
// incorrectly produce it.  We tolerate it for compatibility.
func TestBuildTest_PhonySelfReference(t *testing.T) {
	b := NewBuildTest(t)
	err := ""
	b.AssertParse(&b.state, "build a: phony a\n", ManifestParserOptions{Quiet: true})

	if b.builder.addTargetName("a", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
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
	b.AssertParse(&b.state, "rule touch\n command = touch $out\nbuild notreal: phony blank\nbuild phony1: phony notreal\nbuild phony2: phony\nbuild phony3: phony blank\nbuild phony4: phony notreal\nbuild phony5: phony\nbuild phony6: phony blank\n\nbuild test1: touch phony1\nbuild test2: touch phony2\nbuild test3: touch phony3\nbuild test4: touch phony4\nbuild test5: touch phony5\nbuild test6: touch phony6\n", ManifestParserOptions{})

	// Set up test.
	b.builder.commandRunner = nil // BuildTest owns the CommandRunner
	b.builder.commandRunner = &b.commandRunner

	b.fs.Create("blank", "") // a "real" file
	if b.builder.addTargetName("test1", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.addTargetName("test2", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.addTargetName("test3", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.addTargetName("test4", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.addTargetName("test5", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.addTargetName("test6", &err) == nil {
		b.t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
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

		b.state.Reset()
		startTime := b.fs.now

		// Build number 1
		if b.builder.addTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if !b.builder.AlreadyUpToDate() {
			if !b.builder.Build(&err) {
				t.Fatal("expected true")
			}
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Touch the input file
		b.state.Reset()
		b.commandRunner.commandsRan = nil
		b.fs.Tick()
		b.fs.Create("blank", "") // a "real" file
		if b.builder.addTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Second build, expect testN edge to be rebuilt
		// and phonyN node's mtime to be updated.
		if b.builder.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		wantCommands := []string{"touch test" + ci}
		if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
			t.Fatal(diff)
		}
		if !b.builder.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		inputTime := inputNode.MTime

		if phonyNode.Exists == ExistenceStatusExists {
			t.Fatal("expected false")
		}
		if phonyNode.Dirty {
			t.Fatal("expected false")
		}

		if phonyNode.MTime <= startTime {
			t.Fatal("expected greater")
		}
		if phonyNode.MTime != inputTime {
			t.Fatal("expected equal")
		}
		if err := testNode.Stat(&b.fs); err != nil {
			t.Fatal(err)
		}
		if testNode.Exists != ExistenceStatusExists {
			t.Fatal("expected true")
		}
		if testNode.MTime <= startTime {
			t.Fatal("expected greater")
		}
	} else {
		// Tests 2 and 5: Expect dependents to always rebuild.

		b.state.Reset()
		b.commandRunner.commandsRan = nil
		b.fs.Tick()
		b.commandRunner.commandsRan = nil
		if b.builder.addTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if b.builder.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		wantCommands := []string{"touch test" + ci}
		if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
			t.Fatal(diff)
		}

		b.state.Reset()
		b.commandRunner.commandsRan = nil
		if b.builder.addTargetName("test"+ci, &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if b.builder.AlreadyUpToDate() {
			t.Fatal("expected false")
		}
		if !b.builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		wantCommands = []string{"touch test" + ci}
		if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
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
	b.AssertParse(&b.state, "rule fail\n  command = fail\nbuild out1: fail\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailures(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule fail\n  command = fail\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild all: phony out1 out2 out3\n", ManifestParserOptions{})

	// Swallow two failures, die on the third.
	b.config.FailuresAllowed = 3

	err := ""
	if b.builder.addTargetName("all", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if "subcommands failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailuresLimit(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule fail\n  command = fail\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild final: cat out1 out2 out3\n", ManifestParserOptions{})

	// Swallow ten failures; we should stop before building final.
	b.config.FailuresAllowed = 11

	err := ""
	if b.builder.addTargetName("final", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if "cannot make progress due to previous errors" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_SwallowFailuresPool(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "pool failpool\n  depth = 1\nrule fail\n  command = fail\n  pool = failpool\nbuild out1: fail\nbuild out2: fail\nbuild out3: fail\nbuild final: cat out1 out2 out3\n", ManifestParserOptions{})

	// Swallow ten failures; we should stop before building final.
	b.config.FailuresAllowed = 11

	err := ""
	if b.builder.addTargetName("final", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if "cannot make progress due to previous errors" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PoolEdgesReadyButNotWanted(t *testing.T) {
	b := NewBuildTest(t)
	b.fs.Create("x", "")

	manifest := "pool some_pool\n  depth = 4\nrule touch\n  command = touch $out\n  pool = some_pool\nrule cc\n  command = touch grit\n\nbuild B.d.stamp: cc | x\nbuild C.stamp: touch B.d.stamp\nbuild final.stamp: touch || C.stamp\n"

	b.RebuildTarget("final.stamp", manifest, "", "", nil)

	b.fs.RemoveFile("B.d.stamp")

	saveState := NewState()
	b.RebuildTarget("final.stamp", manifest, "", "", &saveState)
	if saveState.Pools["some_pool"].currentUse < 0 {
		t.Fatal("expected greater or equal")
	}
}

type BuildWithLogTest struct {
	*BuildTest
	buildLog BuildLog
}

func NewBuildWithLogTest(t *testing.T) *BuildWithLogTest {
	b := &BuildWithLogTest{
		BuildTest: NewBuildTest(t),
		buildLog:  NewBuildLog(),
	}
	t.Cleanup(func() {
		b.buildLog.Close()
	})
	b.builder.scan.buildLog = &b.buildLog
	return b
}

func TestBuildWithLogTest_ImplicitGeneratedOutOfDate(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\n  generator = 1\nbuild out.imp: touch | in\n", ManifestParserOptions{})
	b.fs.Create("out.imp", "")
	b.fs.Tick()
	b.fs.Create("in", "")

	err := ""

	if b.builder.addTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.GetNode("out.imp").Dirty {
		t.Fatal("expected true")
	}
}

func TestBuildWithLogTest_ImplicitGeneratedOutOfDate2(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule touch-implicit-dep-out\n  command = touch $test_dependency ; sleep 1 ; touch $out\n  generator = 1\nbuild out.imp: touch-implicit-dep-out | inimp inimp2\n  test_dependency = inimp\n", ManifestParserOptions{})
	b.fs.Create("inimp", "")
	b.fs.Create("out.imp", "")
	b.fs.Tick()
	b.fs.Create("inimp2", "")
	b.fs.Tick()

	err := ""

	if b.builder.addTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	b.builder.cleanup()
	b.builder.plan.Reset()

	if b.builder.addTargetName("out.imp", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
	if b.GetNode("out.imp").Dirty {
		t.Fatal("expected false")
	}
}

func TestBuildWithLogTest_NotInLogButOnDisk(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule cc\n  command = cc\nbuild out1: cc in\n", ManifestParserOptions{})

	// Create input/output that would be considered up to date when
	// not considering the command line hash.
	b.fs.Create("in", "")
	b.fs.Create("out1", "")
	err := ""

	// Because it's not in the log, it should not be up-to-date until
	// we build again.
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()

	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildWithLogTest_RebuildAfterFailure(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule touch-fail-tick2\n  command = touch-fail-tick2\nbuild out1: touch-fail-tick2 in\n", ManifestParserOptions{})

	err := ""

	b.fs.Create("in", "")

	// Run once successfully to get out1 in the log
	if b.builder.addTargetName("out1", &err) == nil || err != "" {
		t.Fatal(err)
	}
	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	b.builder.cleanup()
	b.builder.plan.Reset()

	b.fs.Tick()
	b.fs.Create("in", "")

	// Run again with a failure that updates the output file timestamp
	if b.builder.addTargetName("out1", &err) == nil || err != "" {
		t.Fatal(err)
	}
	if b.builder.Build(&err) || err != "subcommand failed" {
		t.Fatal(err)
	}
	err = ""
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	b.builder.cleanup()
	b.builder.plan.Reset()

	b.fs.Tick()

	// Run again, should rerun even though the output file is up to date on disk
	if b.builder.addTargetName("out1", &err) == nil || err != "" {
		t.Fatal(err)
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}
	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RebuildWithNoInputs(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule touch\n  command = touch\nbuild out1: touch\nbuild out2: touch in\n", ManifestParserOptions{})

	err := ""

	b.fs.Create("in", "")

	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()

	b.fs.Tick()

	b.fs.Create("in", "")

	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatTest(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\n  restat = 1\nbuild out1: cc in\nbuild out2: true out1\nbuild out3: cat out2\n", ManifestParserOptions{})

	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Create("out3", "")

	b.fs.Tick()

	b.fs.Create("in", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause out3 to rebuild
	// regardless of restat.
	err := ""
	if b.builder.addTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	if 3 != b.builder.plan.commandEdges {
		t.Fatal("expected equal")
	}
	b.commandRunner.commandsRan = nil
	b.state.Reset()

	b.fs.Tick()

	b.fs.Create("in", "")
	// "cc" touches out1, so we should build out2.  But because "true" does not
	// touch out2, we should cancel the build of out3.
	if b.builder.addTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// If we run again, it should be a no-op, because the build log has recorded
	// that we've already built out2 with an input timestamp of 2 (from out1).
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.fs.Tick()

	b.fs.Create("in", "")

	// The build log entry should not, however, prevent us from rebuilding out2
	// if out1 changes.
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatMissingFile(t *testing.T) {
	b := NewBuildWithLogTest(t)
	// If a restat rule doesn't create its output, and the output didn't
	// exist before the rule was run, consider that behavior equivalent
	// to a rule that doesn't modify its existent output file.

	b.AssertParse(&b.state, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\nbuild out1: true in\nbuild out2: cc out1\n", ManifestParserOptions{})

	b.fs.Create("in", "")
	b.fs.Create("out2", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause out2 to rebuild
	// regardless of restat.
	err := ""
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.commandRunner.commandsRan = nil
	b.state.Reset()

	b.fs.Tick()
	b.fs.Create("in", "")
	b.fs.Create("out2", "")

	// Run a build, expect only the first command to run.
	// It doesn't touch its output (due to being the "true" command), so
	// we shouldn't run the dependent build.
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_RestatSingleDependentOutputDirty(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule true\n  command = true\n  restat = 1\nrule touch\n  command = touch\nbuild out1: true in\nbuild out2 out3: touch out1\nbuild out4: touch out2\n", ManifestParserOptions{})

	// Create the necessary files
	b.fs.Create("in", "")

	err := ""
	if b.builder.addTargetName("out4", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	b.fs.Tick()
	b.fs.Create("in", "")
	b.fs.RemoveFile("out3")

	// Since "in" is missing, out1 will be built. Since "out3" is missing,
	// out2 and out3 will be built even though "in" is not touched when built.
	// Then, since out2 is rebuilt, out4 should be rebuilt -- the restat on the
	// "true" rule should not lead to the "touch" edge writing out2 and out3 being
	// cleard.
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out4", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

// Test scenario, in which an input file is removed, but output isn't changed
// https://github.com/ninja-build/ninja/issues/295
func TestBuildWithLogTest_RestatMissingInput(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule true\n  command = true\n  depfile = $out.d\n  restat = 1\nrule cc\n  command = cc\nbuild out1: true in\nbuild out2: cc out1\n", ManifestParserOptions{})

	// Create all necessary files
	b.fs.Create("in", "")

	// The implicit dependencies and the depfile itself
	// are newer than the output
	restatMtime := b.fs.Tick()
	b.fs.Create("out1.d", "out1: will.be.deleted restat.file\n")
	b.fs.Create("will.be.deleted", "")
	b.fs.Create("restat.file", "")

	// Run the build, out1 and out2 get built
	err := ""
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// See that an entry in the logfile is created, capturing
	// the right mtime
	logEntry := b.buildLog.Entries["out1"]
	if nil == logEntry {
		t.Fatal("expected true")
	}
	if restatMtime != logEntry.mtime {
		t.Fatal("expected equal")
	}

	// Now remove a file, referenced from depfile, so that target becomes
	// dirty, but the output does not change
	b.fs.RemoveFile("will.be.deleted")

	// Trigger the build again - only out1 gets built
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// Check that the logfile entry remains correctly set
	logEntry = b.buildLog.Entries["out1"]
	if nil == logEntry {
		t.Fatal("expected true")
	}
	if restatMtime != logEntry.mtime {
		t.Fatal("expected equal")
	}
}

func TestBuildWithLogTest_GeneratedPlainDepfileMtime(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule generate-depfile\n  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\nbuild out: generate-depfile\n  test_dependency = inimp\n  depfile = out.d\n", ManifestParserOptions{})
	b.fs.Create("inimp", "")
	b.fs.Tick()

	err := ""

	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	b.builder.cleanup()
	b.builder.plan.Reset()

	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func NewBuildDryRunTest(t *testing.T) *BuildWithLogTest {
	b := NewBuildWithLogTest(t)
	b.config.DryRun = true
	return b
}

func TestBuildDryRun_AllCommandsShown(t *testing.T) {
	b := NewBuildDryRunTest(t)
	b.AssertParse(&b.state, "rule true\n  command = true\n  restat = 1\nrule cc\n  command = cc\n  restat = 1\nbuild out1: cc in\nbuild out2: true out1\nbuild out3: cat out2\n", ManifestParserOptions{})

	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Create("out3", "")

	b.fs.Tick()

	b.fs.Create("in", "")

	// "cc" touches out1, so we should build out2.  But because "true" does not
	// touch out2, we should cancel the build of out3.
	err := ""
	if b.builder.addTargetName("out3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

// Test that RSP files are created when & where appropriate and deleted after
// successful execution.
func TestBuildTest_RspFileSuccess(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $long_command\nrule cat_rsp_out\n  command = cat $rspfile > $out\n  rspfile = $out.rsp\n  rspfile_content = $long_command\nbuild out1: cat in\nbuild out2: cat_rsp in\n  rspfile = out 2.rsp\n  long_command = Some very long command\nbuild out$ 3: cat_rsp_out in\n  long_command = Some very long command\n", ManifestParserOptions{})

	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Create("out 3", "")

	b.fs.Tick()

	b.fs.Create("in", "")

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}
	if b.builder.addTargetName("out 3", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}

	wantCreated := map[string]struct{}{
		"in":    {},
		"in1":   {},
		"in2":   {},
		"out 3": {},
		"out1":  {},
		"out2":  {},
	}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}
	wantRemoved := map[string]struct{}{}
	if diff := cmp.Diff(wantRemoved, b.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal(b.commandRunner.commandsRan)
	}

	// The RSP files were created
	wantCreated["out 2.rsp"] = struct{}{}
	wantCreated["out 3.rsp"] = struct{}{}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}

	// The RSP files were removed
	wantRemoved["out 2.rsp"] = struct{}{}
	wantRemoved["out 3.rsp"] = struct{}{}
	if diff := cmp.Diff(wantRemoved, b.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}
}

// Test that RSP file is created but not removed for commands, which fail
func TestBuildTest_RspFileFailure(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule fail\n  command = fail\n  rspfile = $rspfile\n  rspfile_content = $long_command\nbuild out: fail in\n  rspfile = out.rsp\n  long_command = Another very long command\n", ManifestParserOptions{})

	b.fs.Create("out", "")
	b.fs.Tick()
	b.fs.Create("in", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	wantCreated := map[string]struct{}{
		"in":  {},
		"in1": {},
		"in2": {},
		"out": {},
	}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}
	wantRemoved := map[string]struct{}{}
	if diff := cmp.Diff(wantRemoved, b.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
	wantCommand := []string{"fail"}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file was created
	wantCreated["out.rsp"] = struct{}{}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file was NOT removed
	if diff := cmp.Diff(wantRemoved, b.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}

	// The RSP file contains what it should
	if c, err := b.fs.ReadFile("out.rsp"); err != nil || string(c) != "Another very long command\x00" {
		t.Fatal(c, err)
	}
}

// Test that contents of the RSP file behaves like a regular part of
// command line, i.e. triggers a rebuild if changed
func TestBuildWithLogTest_RspFileCmdLineChange(t *testing.T) {
	b := NewBuildWithLogTest(t)
	b.AssertParse(&b.state, "rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $long_command\nbuild out: cat_rsp in\n  rspfile = out.rsp\n  long_command = Original very long command\n", ManifestParserOptions{})

	b.fs.Create("out", "")
	b.fs.Tick()
	b.fs.Create("in", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// 1. Build for the 1st time (-> populate log)
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	wantCommand := []string{"cat out.rsp > out"}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	// 2. Build again (no change)
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// 3. Alter the entry in the logfile
	// (to simulate a change in the command line between 2 builds)
	logEntry := b.buildLog.Entries["out"]
	if nil == logEntry {
		t.Fatal("expected true")
	}
	b.AssertHash("cat out.rsp > out;rspfile=Original very long command", logEntry.commandHash)
	logEntry.commandHash++ // Change the command hash to something else.
	// Now expect the target to be rebuilt
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_InterruptCleanup(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule interrupt\n  command = interrupt\nrule touch-interrupt\n  command = touch-interrupt\nbuild out1: interrupt in1\nbuild out2: touch-interrupt in2\n", ManifestParserOptions{})

	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Tick()
	b.fs.Create("in1", "")
	b.fs.Create("in2", "")

	// An untouched output of an interrupted command should be retained.
	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if "interrupted by user" != err {
		t.Fatal("expected equal")
	}
	b.builder.cleanup()
	if mtime, err := b.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	err = ""

	// A touched output of an interrupted command should be deleted.
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if "interrupted by user" != err {
		t.Fatal("expected equal")
	}
	b.builder.cleanup()
	if mtime, err := b.fs.Stat("out2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestBuildTest_StatFailureAbortsBuild(t *testing.T) {
	b := NewBuildTest(t)
	tooLongToStat := strings.Repeat("i", 400)
	b.AssertParse(&b.state, ("build " + tooLongToStat + ": cat in\n"), ManifestParserOptions{})
	b.fs.Create("in", "")

	// This simulates a stat failure:
	b.fs.files[tooLongToStat] = Entry{
		mtime:     -1,
		statError: errors.New("stat failed"),
	}

	err := ""
	if b.builder.addTargetName(tooLongToStat, &err) != nil {
		t.Fatal("expected false")
	}
	if "stat failed" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_PhonyWithNoInputs(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build nonexistent: phony\nbuild out1: cat || nonexistent\nbuild out2: cat nonexistent\n", ManifestParserOptions{})
	b.fs.Create("out1", "")
	b.fs.Create("out2", "")

	// out1 should be up to date even though its input is dirty, because its
	// order-only dependency has nothing to do.
	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}

	// out2 should still be out of date though, because its input is dirty.
	err = ""
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DepsGccWithEmptyDepfileErrorsOut(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule cc\n  command = cc\n  deps = gcc\nbuild out: cc\n", ManifestParserOptions{})
	b.Dirty("out")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if b.builder.AlreadyUpToDate() {
		t.Fatal("expected false")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_StatusFormatElapsed(t *testing.T) {
	b := NewBuildTest(t)
	b.status.BuildStarted()
	// Before any task is done, the elapsed time must be zero.
	if "[%/e0.000]" != b.status.formatProgressStatus("[%%/e%e]", 0) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_StatusFormatReplacePlaceholder(t *testing.T) {
	b := NewBuildTest(t)
	if "[%/s0/t0/r0/u0/f0]" != b.status.formatProgressStatus("[%%/s%s/t%t/r%r/u%u/f%f]", 0) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_FailedDepsParse(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build bad_deps.o: cat in1\n  deps = gcc\n  depfile = in1.d\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("bad_deps.o", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// These deps will fail to parse, as they should only have one
	// path to the left of the colon.
	b.fs.Create("in1.d", "AAA BBB")

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	if "subcommand failed" != err {
		t.Fatal("expected equal")
	}
}

type BuildWithQueryDepsLogTest struct {
	*BuildTestBase
	log     DepsLog
	builder *Builder
}

func NewBuildWithQueryDepsLogTest(t *testing.T) *BuildWithQueryDepsLogTest {
	b := &BuildWithQueryDepsLogTest{
		BuildTestBase: NewBuildTestBase(t),
	}
	CreateTempDirAndEnter(t)
	err := ""
	if !b.log.OpenForWrite("ninja_deps", &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	t.Cleanup(func() {
		if err2 := b.log.Close(); err2 != nil {
			t.Error(err2)
		}
	})
	b.builder = NewBuilder(&b.state, &b.config, nil, &b.log, &b.fs, b.status, 0)
	b.builder.commandRunner = &b.commandRunner
	return b
}

// Test a MSVC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileMSVC(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state, "rule cp_multi_msvc\n    command = echo 'using $in' && for file in $out; do cp $in $$file; done\n    deps = msvc\n    msvc_deps_prefix = using \nbuild out1 out2: cp_multi_msvc in1\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"echo 'using in1' && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 1 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 1 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOneLine(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state, "rule cp_multi_gcc\n    command = echo '$out: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs.Create("in.d", "out1 out2: in1 in2")
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"echo 'out1 out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 2 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out1Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 2 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out2Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs using a line per input.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineInput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state, "rule cp_multi_gcc\n    command = echo '$out: in1\\n$out: in2' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs.Create("in.d", "out1 out2: in1\nout1 out2: in2")
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"echo 'out1 out2: in1\\nout1 out2: in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 2 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out1Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 2 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out2Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs using a line per output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state, "rule cp_multi_gcc\n    command = echo 'out1: $in\\nout2: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs.Create("in.d", "out1: in1 in2\nout2: in1 in2")
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"echo 'out1: in1 in2\\nout2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 2 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out1Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 2 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out2Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs mentioning only the main output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlyMainOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	b.AssertParse(&b.state, "rule cp_multi_gcc\n    command = echo 'out1: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs.Create("in.d", "out1: in1 in2")
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommand := []string{"echo 'out1: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 2 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out1Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 2 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out2Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

// Test a GCC-style deps log with multiple outputs mentioning only the secondary output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlySecondaryOutput(t *testing.T) {
	b := NewBuildWithQueryDepsLogTest(t)
	// Note: This ends up short-circuiting the node creation due to the primary
	// output not being present, but it should still work.
	b.AssertParse(&b.state, "rule cp_multi_gcc\n    command = echo 'out2: $in' > in.d && for file in $out; do cp in1 $$file; done\n    deps = gcc\n    depfile = in.d\nbuild out1 out2: cp_multi_gcc in1 in2\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	b.fs.Create("in.d", "out2: in1 in2")
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommand := []string{"echo 'out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done"}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	out1Node := b.state.Paths["out1"]
	out1Deps := b.log.GetDeps(out1Node)
	if 2 != len(out1Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out1Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out1Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}

	out2Node := b.state.Paths["out2"]
	out2Deps := b.log.GetDeps(out2Node)
	if 2 != len(out2Deps.Nodes) {
		t.Fatal("expected equal")
	}
	if "in1" != out2Deps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "in2" != out2Deps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

// Tests of builds involving deps logs necessarily must span
// multiple builds.  We reuse methods on BuildTest but not the
// b.builder it sets up, because we want pristine objects for
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
		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out", &err) == nil {
			t.Fatal(err)
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs.Create("in1.d", "out: in2")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// The deps file should have been removed.
		if mtime, err := b.fs.Stat("in1.d"); mtime != 0 || err != nil {
			t.Fatal(mtime, err)
		}
		// Recreate it for the next step.
		b.fs.Create("in1.d", "out: in2")
		depsLog.Close()
		builder.commandRunner = nil
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Touch the file only mentioned in the deps.
		b.fs.Tick()
		b.fs.Create("in2", "")

		// Run the build again.
		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		b.commandRunner.commandsRan = nil
		if builder.addTargetName("out", &err) == nil {
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
		if 1 != len(b.commandRunner.commandsRan) {
			t.Fatal("expected equal")
		}

		builder.commandRunner = nil
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
		b.fs.Create("in1", "")
		b.fs.Create("in1.d", "out: ")

		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out", &err) == nil {
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

		builder.commandRunner = nil
	}

	// Push all files one tick forward so that only the deps are out
	// of date.
	b.fs.Tick()
	b.fs.Create("in1", "")
	b.fs.Create("out", "")

	// The deps file should have been removed, so no need to timestamp it.
	if mtime, err := b.fs.Stat("in1.d"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		b.commandRunner.commandsRan = nil
		if builder.addTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Recreate the deps file here because the build expects them to exist.
		b.fs.Create("in1.d", "out: ")

		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// We should have rebuilt the output due to the deps being
		// out of date.
		if 1 != len(b.commandRunner.commandsRan) {
			t.Fatal("expected equal")
		}

		builder.commandRunner = nil
	}
}

func TestBuildWithDepsLogTest_DepsIgnoredInDryRun(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	manifest := "build out: cat in1\n  deps = gcc\n  depfile = in1.d\n"

	b.fs.Create("out", "")
	b.fs.Tick()
	b.fs.Create("in1", "")

	state := NewState()
	b.AddCatRule(&state)
	b.AssertParse(&state, manifest, ManifestParserOptions{})

	// The deps log is NULL in dry runs.
	b.config.DryRun = true
	builder := NewBuilder(&state, &b.config, nil, nil, &b.fs, b.status, 0)
	builder.commandRunner = &b.commandRunner
	b.commandRunner.commandsRan = nil

	err := ""
	if builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !builder.Build(&err) {
		t.Fatal("expected true")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	builder.commandRunner = nil
}

// Check that a restat rule generating a header cancels compilations correctly.
func TestBuildTest_RestatDepfileDependency(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat in1\n  depfile = in1.d\n", ManifestParserOptions{}) // Would be "write if out-of-date" in reality

	b.fs.Create("header.h", "")
	b.fs.Create("in1.d", "out: header.h")
	b.fs.Tick()
	b.fs.Create("header.in", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
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
		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs.Create("in1.d", "out: header.h")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}

	{
		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Touch the input of the restat rule.
		b.fs.Tick()
		b.fs.Create("header.in", "")

		// Run the build again.
		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		b.commandRunner.commandsRan = nil
		if builder.addTargetName("out", &err) == nil {
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
		if 1 != len(b.commandRunner.commandsRan) {
			t.Fatal("expected equal")
		}

		builder.commandRunner = nil
	}
}

func TestBuildWithDepsLogTest_DepFileOKDepsLog(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\nbuild fo$ o.o: cc foo.c\n"

	b.fs.Create("foo.c", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		b.fs.Create("fo o.o.d", "fo\\ o.o: blah.h bar.h\n")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner

		edge := state.Edges[len(state.Edges)-1]

		state.GetNode("bar.h", 0).Dirty = true // Mark bar.h as missing.
		if builder.addTargetName("fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Expect three new edges: one generating fo o.o, and two more from
		// loading the depfile.
		if 3 != len(state.Edges) {
			t.Fatal("expected equal")
		}
		// Expect our edge to now have three inputs: foo.c and two headers.
		if 3 != len(edge.Inputs) {
			t.Fatal("expected equal")
		}

		// Expect the command line we generate to only use the original input.
		if "cc foo.c" != edge.EvaluateCommand(false) {
			t.Fatal("expected equal")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}
}

func TestBuildWithDepsLogTest_DiscoveredDepDuringBuildChanged(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule touch-out-implicit-dep\n  command = touch $out ; sleep 1 ; touch $test_dependency\nrule generate-depfile\n  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\nbuild out1: touch-out-implicit-dep in1\n  test_dependency = inimp\nbuild out2: generate-depfile in1 || out1\n  test_dependency = inimp\n  depfile = out2.d\n  deps = gcc\n"

	b.fs.Create("in1", "")
	b.fs.Tick()

	buildLog := NewBuildLog()
	defer buildLog.Close()

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, &buildLog, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out2", &err) == nil {
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

		depsLog.Close()
		builder.commandRunner = nil
	}

	b.fs.Tick()
	b.fs.Create("in1", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, &buildLog, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out2", &err) == nil {
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

		depsLog.Close()
		builder.commandRunner = nil
	}

	b.fs.Tick()

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, &buildLog, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("out2", &err) == nil {
			t.Fatal("expected true")
		}
		if !builder.AlreadyUpToDate() {
			t.Fatal("expected true")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}
}

func TestBuildWithDepsLogTest_DepFileDepsLogCanonicalize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	b := NewBuildWithDepsLogTest(t)
	err := ""
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\nbuild a/b\\c\\d/e/fo$ o.o: cc x\\y/z\\foo.c\n"

	b.fs.Create("x/y/z/foo.c", "")

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		// Run the build once, everything should be ok.
		depsLog := DepsLog{}
		defer depsLog.Close()
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner
		if builder.addTargetName("a/b/c/d/e/fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		// Note, different slashes from manifest.
		b.fs.Create("a/b\\c\\d/e/fo o.o.d", "a\\b\\c\\d\\e\\fo\\ o.o: blah.h bar.h\n")
		if !builder.Build(&err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}

	{
		state := NewState()
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		defer depsLog.Close()
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner

		edge := state.Edges[len(state.Edges)-1]

		state.GetNode("bar.h", 0).Dirty = true // Mark bar.h as missing.
		if builder.addTargetName("a/b/c/d/e/fo o.o", &err) == nil {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Expect three new edges: one generating fo o.o, and two more from
		// loading the depfile.
		if 3 != len(state.Edges) {
			t.Fatal("expected equal")
		}
		// Expect our edge to now have three inputs: foo.c and two headers.
		if 3 != len(edge.Inputs) {
			t.Fatal("expected equal")
		}

		// Expect the command line we generate to only use the original input.
		// Note, slashes from manifest, not .d.
		if "cc x\\y/z\\foo.c" != edge.EvaluateCommand(false) {
			t.Fatal("expected equal")
		}

		depsLog.Close()
		builder.commandRunner = nil
	}
}

// Check that a restat rule doesn't clear an edge if the depfile is missing.
// Follows from: https://github.com/ninja-build/ninja/issues/603
func TestBuildTest_RestatMissingDepfile(t *testing.T) {
	b := NewBuildTest(t)
	manifest := "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat header.h\n  depfile = out.d\n" // Would be "write if out-of-date" in reality.

	b.fs.Create("header.h", "")
	b.fs.Tick()
	b.fs.Create("out", "")
	b.fs.Create("header.in", "")

	// Normally, only 'header.h' would be rebuilt, as
	// its rule doesn't touch the output and has 'restat=1' set.
	// But we are also missing the depfile for 'out',
	// which should force its command to run anyway!
	b.RebuildTarget("out", manifest, "", "", nil)
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

// Check that a restat rule doesn't clear an edge if the deps are missing.
// https://github.com/ninja-build/ninja/issues/603
func TestBuildWithDepsLogTest_RestatMissingDepfileDepslog(t *testing.T) {
	b := NewBuildWithDepsLogTest(t)
	manifest := "rule true\n  command = true\n  restat = 1\nbuild header.h: true header.in\nbuild out: cat header.h\n  deps = gcc\n  depfile = out.d\n" // Would be "write if out-of-date" in reality.

	// Build once to populate ninja deps logs from out.d
	b.fs.Create("header.in", "")
	b.fs.Create("out.d", "out: header.h")
	b.fs.Create("header.h", "")

	b.RebuildTarget("out", manifest, "build_log", "ninja_deps", nil)
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// Sanity: this rebuild should be NOOP
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps", nil)
	if 0 != len(b.commandRunner.commandsRan) {
		t.Fatalf("Expected no command; %#v", b.commandRunner.commandsRan)
	}

	// Touch 'header.in', blank dependencies log (create a different one).
	// Building header.h triggers 'restat' outputs cleanup.
	// Validate that out is rebuilt netherless, as deps are missing.
	b.fs.Tick()
	b.fs.Create("header.in", "")

	// (switch to a new blank depsLog "ninja_deps2")
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// Sanity: this build should be NOOP
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 0 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// Check that invalidating deps by target timestamp also works here
	// Repeat the test but touch target instead of blanking the log.
	b.fs.Tick()
	b.fs.Create("header.in", "")
	b.fs.Create("out", "")
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 2 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}

	// And this build should be NOOP again
	b.RebuildTarget("out", manifest, "build_log", "ninja_deps2", nil)
	if 0 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_WrongOutputInDepfileCausesRebuild(t *testing.T) {
	b := NewBuildTest(t)
	manifest := "rule cc\n  command = cc $in\n  depfile = $out.d\nbuild foo.o: cc foo.c\n"

	b.fs.Create("foo.c", "")
	b.fs.Create("foo.o", "")
	b.fs.Create("header.h", "")
	b.fs.Create("foo.o.d", "bar.o.d: header.h\n")

	b.RebuildTarget("foo.o", manifest, "build_log", "ninja_deps", nil)
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_Console(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "rule console\n  command = console\n  pool = console\nbuild cons: console in.txt\n", ManifestParserOptions{})

	b.fs.Create("in.txt", "")

	err := ""
	if b.builder.addTargetName("cons", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepMissingAndNoRule(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that we can diagnose when a dyndep file is missing and
	// has no rule to build it.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})

	err := ""
	if b.builder.addTargetName("out", &err) != nil {
		t.Fatal("expected false")
	}
	if "loading 'dd': file does not exist" != err {
		t.Fatal(err)
	}
}

func TestBuildTest_DyndepReadyImplicitConnection(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be loaded immediately to discover
	// that one edge has an implicit output that is also an implicit
	// input of another edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep | tmp.imp\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepReadySyntaxError(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be loaded immediately to discover
	// and reject a syntax error in it.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd", "build out: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) != nil {
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
	b.AssertParse(&b.state, "rule r\n  command = unused\nbuild out: r in || dd\n  dyndep = dd\nbuild in: r circ\n", ManifestParserOptions{})
	b.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\n")
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) != nil {
		t.Fatal("expected false")
	}
	if "dependency cycle: circ -> in -> circ" != err {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepBuild(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover nothing.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	wantCreated := map[string]struct{}{"dd-in": {}, "in1": {}, "in2": {}}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommand := []string{"cp dd-in dd", "touch out"}
	if diff := cmp.Diff(wantCommand, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
	wantFilesRead := []string{"dd-in", "dd"}
	if diff := cmp.Diff(wantFilesRead, b.fs.filesRead); diff != "" {
		t.Fatal(diff)
	}
	wantCreated["dd"] = struct{}{}
	wantCreated["out"] = struct{}{}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildSyntaxError(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// and reject a syntax error in it.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "build out: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
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
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild unrelated: touch || dd\nbuild out: touch unrelated || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch unrelated", "touch out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewOutput(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new output of an edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("in", "")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewOutputWithMultipleRules1(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new output of an edge that is already the output of another edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out1 | out-twice.imp: touch in\nbuild out2: touch in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("in", "")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")
	b.fs.Tick()
	b.fs.Create("out1", "")
	b.fs.Create("out2", "")

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
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
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || dd1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // make order predictable for test
	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1 | out-twice.imp: dyndep\n")
	b.fs.Create("dd2-in", "")
	b.fs.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out-twice.imp: dyndep\n")
	b.fs.Tick()
	b.fs.Create("out1", "")
	b.fs.Create("out2", "")

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
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
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild in: touch\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch in", "touch out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewInputWithValidation(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file cannot contain the |@ validation
	// syntax.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep |@ validation\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected success")
	}
	if err != "" {
		t.Fatal(err)
	}

	if b.builder.Build(&err) {
		t.Fatal("expected failure")
	}

	errFirstLine := strings.SplitN(err, "\n", 2)[0]
	if "dd:2: expected newline, got '|@'" != errFirstLine {
		t.Fatal(errFirstLine)
	}
}

func TestBuildTest_DyndepBuildDiscoverNewInputWithTransitiveValidation(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// a new input to an edge that has a validation edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild in: touch |@ validation\nbuild validation: touch in out\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep | in\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal(err)
	}
	if err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) {
		t.Fatal(err)
	}
	if err != "" {
		t.Fatal(err)
	}
	wantCommands := []string{"cp dd-in dd", "touch in", "touch out", "touch validation"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverImplicitConnection(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that one edge has an implicit output that is also an implicit
	// input of another edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep | tmp.imp\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverOutputAndDepfileInput(t *testing.T) {
	// WARNING: I (maruel) am not 100% sure about this test case.
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that one edge has an implicit output that is also reported by
	// a depfile as an input of another edge.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: cp tmp\n  depfile = out.d\n", ManifestParserOptions{})
	b.fs.Create("out.d", "out: tmp.imp\n")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Loading the depfile gave tmp.imp a phony input edge.
	if b.GetNode("tmp.imp").InEdge.Rule != PhonyRule {
		t.Fatal("expected true")
	}

	wantCreated := map[string]struct{}{
		"dd-in": {},
		"in1":   {},
		"in2":   {},
		"out.d": {},
	}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Loading the dyndep file gave tmp.imp a real input edge.
	if b.GetNode("tmp.imp").InEdge.Rule == PhonyRule {
		t.Fatal("expected false")
	}

	wantCommands := []string{"cp dd-in dd", "touch tmp tmp.imp", "cp tmp out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
	wantCreated["dd"] = struct{}{}
	wantCreated["out"] = struct{}{}
	wantCreated["tmp"] = struct{}{}
	wantCreated["tmp.imp"] = struct{}{}
	if diff := cmp.Diff(wantCreated, b.fs.filesCreated); diff != "" {
		t.Fatal(diff)
	}
	if !b.builder.AlreadyUpToDate() {
		t.Fatal("expected true")
	}
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdge(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge is actually wanted due to a missing implicit output.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch tmp || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("tmp", "")
	b.fs.Create("out", "")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdgeAndDependent(t *testing.T) {
	t.Skip("TODO")
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge and a dependent are actually wanted.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild tmp: touch || dd\n  dyndep = dd\nbuild out: touch tmp\n", ManifestParserOptions{})
	b.fs.Create("tmp", "")
	b.fs.Create("out", "")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild tmp | tmp.imp: dyndep\n")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	/* Ugh
	fmt.Printf("State:\n")
	b.state.Dump()
	fmt.Printf("Plan:\n")
	b.builder.plan.Dump()
	*/

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}

	/* Ugh
	fmt.Printf("After:\n")
	fmt.Printf("State:\n")
	b.state.Dump()
	fmt.Printf("Plan:\n")
	b.builder.plan.Dump()
	*/
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "touch tmp tmp.imp", "touch out out.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverCircular(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// and reject a circular dependency.
	b.AssertParse(&b.state, "rule r\n  command = unused\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out: r in || dd\n  depfile = out.d\n  dyndep = dd\nbuild in: r || dd\n  dyndep = dd\n", ManifestParserOptions{})
	b.fs.Create("out.d", "out: inimp\n")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out | circ: dyndep\nbuild in: dyndep | circ\n")
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if b.builder.Build(&err) {
		t.Fatal("expected false")
	}
	// Depending on how the pointers in ready work out, we could have
	// discovered the cycle from either starting point.
	if err != "dependency cycle: circ -> in -> circ" && err != "dependency cycle: in -> circ -> in" {
		t.Fatal(err)
	}
}

func TestBuildWithLogTest_DyndepBuildDiscoverRestat(t *testing.T) {
	b := NewBuildWithLogTest(t)
	// Verify that a dyndep file can be built and loaded to discover
	// that an edge has a restat binding.
	b.AssertParse(&b.state, "rule true\n  command = true\nrule cp\n  command = cp $in $out\nbuild dd: cp dd-in\nbuild out1: true in || dd\n  dyndep = dd\nbuild out2: cat out1\n", ManifestParserOptions{})

	b.fs.Create("out1", "")
	b.fs.Create("out2", "")
	b.fs.Create("dd-in", "ninja_dyndep_version = 1\nbuild out1: dyndep\n  restat = 1\n")
	b.fs.Tick()
	b.fs.Create("in", "")

	// Do a pre-build so that there's commands in the log for the outputs,
	// otherwise, the lack of an entry in the build log will cause "out2" to
	// rebuild regardless of restat.
	err := ""
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd-in dd", "true", "cat out1 > out2"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	b.fs.Tick()
	b.fs.Create("in", "")

	// We touched "in", so we should build "out1".  But because "true" does not
	// touch "out1", we should cancel the build of "out2".
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	wantCommands = []string{"true"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepBuildDiscoverScheduledEdge(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can be built and loaded to discover a
	// new input that itself is an output from an edge that has already
	// been scheduled but not finished.  We should not re-schedule it.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild out1 | out1.imp: touch\nbuild zdd: cp zdd-in\n  verify_active_edge = out1\nbuild out2: cp out1 || zdd\n  dyndep = zdd\n", ManifestParserOptions{}) // verify out1 is active when zdd is finished
	b.fs.Create("zdd-in", "ninja_dyndep_version = 1\nbuild out2: dyndep | out1.imp\n")

	// Enable concurrent builds so that we can load the dyndep file
	// while another edge is still active.
	b.commandRunner.maxActiveEdges = 2

	// During the build "out1" and "zdd" should be built concurrently.
	// The fake command runner will finish these in reverse order
	// of the names of the first outputs, so "zdd" will finish first
	// and we will load the dyndep file while the edge for "out1" is
	// still active.  This will add a new dependency on "out1.imp",
	// also produced by the active edge.  The builder should not
	// re-schedule the already-active edge.

	err := ""
	if b.builder.addTargetName("out1", &err) == nil {
		t.Fatal("expected true")
	}
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 3 != len(b.commandRunner.commandsRan) {
		t.Fatal("expected equal")
	}
	// Depending on how the pointers in ready work out, the first
	// two commands may have run in either order.
	if !(b.commandRunner.commandsRan[0] == "touch out1 out1.imp" && b.commandRunner.commandsRan[1] == "cp zdd-in zdd") || (b.commandRunner.commandsRan[1] == "touch out1 out1.imp" && b.commandRunner.commandsRan[0] == "cp zdd-in zdd") {
		t.Fatal("expected true")
	}
	if "cp out1 out2" != b.commandRunner.commandsRan[2] {
		t.Fatal("expected equal")
	}
}

func TestBuildTest_DyndepTwoLevelDirect(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a clean dyndep file can depend on a dirty dyndep file
	// and be loaded properly after the dirty one is built and loaded.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1 | out1.imp: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || dd1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // direct order-only dep on dd1
	b.fs.Create("out1.imp", "")
	b.fs.Create("out2", "")
	b.fs.Create("out2.imp", "")
	b.fs.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1: dyndep\n")
	b.fs.Create("dd2-in", "")
	b.fs.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out2.imp: dyndep | out1.imp\n")

	// During the build dd1 should be built and loaded.  The RecomputeDirty
	// called as a result of loading dd1 should not cause dd2 to be loaded
	// because the builder will never get a chance to update the build plan
	// to account for dd2.  Instead dd2 should only be later loaded once the
	// builder recognizes that it is now ready (as its order-only dependency
	// on dd1 has been satisfied).  This test case verifies that each dyndep
	// file is loaded to update the build graph independently.

	err := ""
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd1-in dd1", "touch out1 out1.imp", "touch out2 out2.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelIndirect(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that dyndep files can add to an edge new implicit inputs that
	// correspond to implicit outputs added to other edges by other dyndep
	// files on which they (order-only) depend.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out $out.imp\nrule cp\n  command = cp $in $out\nbuild dd1: cp dd1-in\nbuild out1: touch || dd1\n  dyndep = dd1\nbuild dd2: cp dd2-in || out1\nbuild out2: touch || dd2\n  dyndep = dd2\n", ManifestParserOptions{}) // indirect order-only dep on dd1
	b.fs.Create("out1.imp", "")
	b.fs.Create("out2", "")
	b.fs.Create("out2.imp", "")
	b.fs.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out1 | out1.imp: dyndep\n")
	b.fs.Create("dd2-in", "")
	b.fs.Create("dd2", "ninja_dyndep_version = 1\nbuild out2 | out2.imp: dyndep | out1.imp\n")

	// During the build dd1 should be built and loaded.  Then dd2 should
	// be built and loaded.  Loading dd2 should cause the builder to
	// recognize that out2 needs to be built even though it was originally
	// clean without dyndep info.

	err := ""
	if b.builder.addTargetName("out2", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd1-in dd1", "touch out1 out1.imp", "touch out2 out2.imp"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelDiscoveredReady(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can discover a new input whose
	// edge also has a dyndep file that is ready to load immediately.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd0: cp dd0-in\nbuild dd1: cp dd1-in\nbuild in: touch\nbuild tmp: touch || dd0\n  dyndep = dd0\nbuild out: touch || dd1\n  dyndep = dd1\n", ManifestParserOptions{})
	b.fs.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out: dyndep | tmp\n")
	b.fs.Create("dd0-in", "")
	b.fs.Create("dd0", "ninja_dyndep_version = 1\nbuild tmp: dyndep | in\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	wantCommands := []string{"cp dd1-in dd1", "touch in", "touch tmp", "touch out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_DyndepTwoLevelDiscoveredDirty(t *testing.T) {
	b := NewBuildTest(t)
	// Verify that a dyndep file can discover a new input whose
	// edge also has a dyndep file that needs to be built.
	b.AssertParse(&b.state, "rule touch\n  command = touch $out\nrule cp\n  command = cp $in $out\nbuild dd0: cp dd0-in\nbuild dd1: cp dd1-in\nbuild in: touch\nbuild tmp: touch || dd0\n  dyndep = dd0\nbuild out: touch || dd1\n  dyndep = dd1\n", ManifestParserOptions{})
	b.fs.Create("dd1-in", "ninja_dyndep_version = 1\nbuild out: dyndep | tmp\n")
	b.fs.Create("dd0-in", "ninja_dyndep_version = 1\nbuild tmp: dyndep | in\n")
	b.fs.Tick()
	b.fs.Create("out", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}
	wantCommands := []string{"cp dd1-in dd1", "cp dd0-in dd0", "touch in", "touch tmp", "touch out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_Validation(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build out: cat in |@ validate\nbuild validate: cat in2\n", ManifestParserOptions{})

	b.fs.Create("in", "")
	b.fs.Create("in2", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	if len(b.commandRunner.commandsRan) != 2 {
		t.Fatal("size")
	}

	// Test touching "in" only rebuilds "out" ("validate" doesn't depend on
	// "out").
	b.fs.Tick()
	b.fs.Create("in", "")

	err = ""
	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommands := []string{"cat in > out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	// Test touching "in2" only rebuilds "validate" ("out" doesn't depend on
	// "validate").
	b.fs.Tick()
	b.fs.Create("in2", "")

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommands = []string{"cat in2 > validate"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_ValidationDependsOnOutput(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build out: cat in |@ validate\nbuild validate: cat in2 | out\n", ManifestParserOptions{})

	b.fs.Create("in", "")
	b.fs.Create("in2", "")
	err := ""
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	if len(b.commandRunner.commandsRan) != 2 {
		t.Fatal(b.commandRunner.commandsRan)
	}

	// Test touching "in" rebuilds "out" and "validate".
	b.fs.Tick()
	b.fs.Create("in", "")

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	if len(b.commandRunner.commandsRan) != 2 {
		t.Fatal(b.commandRunner.commandsRan)
	}

	// Test touching "in2" only rebuilds "validate" ("out" doesn't depend on
	// "validate").
	b.fs.Tick()
	b.fs.Create("in2", "")

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommands := []string{"cat in2 > validate"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildWithDepsLogTest_ValidationThroughDepfile(t *testing.T) {
	b := NewBuildTest(t)
	manifest := "build out: cat in |@ validate\nbuild validate: cat in2 | out\nbuild out2: cat in3\n  deps = gcc\n  depfile = out2.d\n"

	err := ""
	{
		b.fs.Create("in", "")
		b.fs.Create("in2", "")
		b.fs.Create("in3", "")
		b.fs.Create("out2.d", "out: out")

		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		if !depsLog.OpenForWrite("ninja_deps", &err) || err != "" {
			t.Fatal(err)
		}
		defer depsLog.Close()

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner

		if builder.addTargetName("out2", &err) == nil || err != "" {
			t.Fatal(err)
		}

		if !builder.Build(&err) || err != "" {
			t.Fatal(err)
		}

		// On the first build, only the out2 command is run.
		wantCommands := []string{"cat in3 > out2"}
		if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
			t.Fatal(diff)
		}

		// The deps file should have been removed.
		if mtime, err := b.fs.Stat("out2.d"); mtime != 0 || err != nil {
			t.Fatal(mtime, err)
		}

		depsLog.Close()
	}

	b.fs.Tick()
	b.commandRunner.commandsRan = nil

	{
		b.fs.Create("in2", "")
		b.fs.Create("in3", "")

		state := NewState()
		b.AddCatRule(&state)
		b.AssertParse(&state, manifest, ManifestParserOptions{})

		depsLog := DepsLog{}
		if depsLog.Load("ninja_deps", &state, &err) != LoadSuccess {
			t.Fatal(err)
		}
		if !depsLog.OpenForWrite("ninja_deps", &err) || err != "" {
			t.Fatal(err)
		}

		builder := NewBuilder(&state, &b.config, nil, &depsLog, &b.fs, b.status, 0)
		builder.commandRunner = &b.commandRunner

		if builder.addTargetName("out2", &err) == nil || err != "" {
			t.Fatal(err)
		}

		if !builder.Build(&err) || err != "" {
			t.Fatal(err)
		}

		// The out and validate actions should have been run as well as out2.
		if len(b.commandRunner.commandsRan) != 3 {
			t.Fatal(b.commandRunner.commandsRan)
		}
		// out has to run first, as both out2 and validate depend on it.
		if b.commandRunner.commandsRan[0] != "cat in > out" {
			t.Fatal(b.commandRunner.commandsRan)
		}

		depsLog.Close()
	}
}

func TestBuildTest_ValidationCircular(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build out: cat in |@ out2\nbuild out2: cat in2 |@ out\n", ManifestParserOptions{})
	b.fs.Create("in", "")
	b.fs.Create("in2", "")

	err := ""
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	if len(b.commandRunner.commandsRan) != 2 {
		t.Fatal(b.commandRunner.commandsRan)
	}

	// Test touching "in" rebuilds "out".
	b.fs.Tick()
	b.fs.Create("in", "")

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommands := []string{"cat in > out"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}

	// Test touching "in2" rebuilds "out2".
	b.fs.Tick()
	b.fs.Create("in2", "")

	b.commandRunner.commandsRan = nil
	b.state.Reset()
	if b.builder.addTargetName("out", &err) == nil || err != "" {
		t.Fatal(err)
	}

	if !b.builder.Build(&err) || err != "" {
		t.Fatal(err)
	}

	wantCommands = []string{"cat in2 > out2"}
	if diff := cmp.Diff(wantCommands, b.commandRunner.commandsRan); diff != "" {
		t.Fatal(diff)
	}
}

func TestBuildTest_ValidationWithCircularDependency(t *testing.T) {
	b := NewBuildTest(t)
	b.AssertParse(&b.state, "build out: cat in |@ validate\nbuild validate: cat validate_in | out\nbuild validate_in: cat validate\n", ManifestParserOptions{})

	b.fs.Create("in", "")

	err := ""
	if b.builder.addTargetName("out", &err) != nil {
		t.Fatal("expected failure")
	}
	if err != "dependency cycle: validate -> validate_in -> validate" {
		t.Fatal(err)
	}
}
