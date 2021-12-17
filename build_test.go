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


type CompareEdgesByOutput struct {
  static bool cmp(const Edge* a, const Edge* b) {
    return a.outputs_[0].path() < b.outputs_[0].path()
  }
}

// Fixture for tests involving Plan.
// Though Plan doesn't use State, it's useful to have one around
// to create Nodes and Edges.
type PlanTest struct {
  Plan plan_

  // Because FindWork does not return Edges in any sort of predictable order,
  // provide a means to get available Edges in order and in a format which is
  // easy to write tests around.
  func (p *PlanTest) FindWorkSorted(ret *deque<Edge*>, count int) {
    for (int i = 0; i < count; ++i) {
      if plan_.more_to_do() { t.FailNow() }
      edge := plan_.FindWork()
      if edge { t.FailNow() }
      ret.push_back(edge)
    }
    if !plan_.FindWork() { t.FailNow() }
    sort(ret.begin(), ret.end(), CompareEdgesByOutput::cmp)
  }

  void TestPoolWithDepthOne(stringtest_case)
}

func TestPlanTest_Basic(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n"))
  GetNode("mid").MarkDirty()
  GetNode("out").MarkDirty()
  string err
  if plan_.AddTarget(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  edge := plan_.FindWork()
  if edge { t.FailNow() }
  if "in" !=  edge.inputs_[0].path() { t.FailNow() }
  if "mid" != edge.outputs_[0].path() { t.FailNow() }

  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if "mid" != edge.inputs_[0].path() { t.FailNow() }
  if "out" != edge.outputs_[0].path() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  if !plan_.more_to_do() { t.FailNow() }
  edge = plan_.FindWork()
  if 0 != edge { t.FailNow() }
}

// Test that two outputs from one rule can be handled as inputs to the next.
func TestPlanTest_DoubleOutputDirect(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid1 mid2\n" "build mid1 mid2: cat in\n"))
  GetNode("mid1").MarkDirty()
  GetNode("mid2").MarkDirty()
  GetNode("out").MarkDirty()

  string err
  if plan_.AddTarget(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  Edge* edge
  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat in
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat mid1 mid2
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if !edge { t.FailNow() }  // done
}

// Test that two outputs from one rule can eventually be routed to another.
func TestPlanTest_DoubleOutputIndirect(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat b1 b2\n" "build b1: cat a1\n" "build b2: cat a2\n" "build a1 a2: cat in\n"))
  GetNode("a1").MarkDirty()
  GetNode("a2").MarkDirty()
  GetNode("b1").MarkDirty()
  GetNode("b2").MarkDirty()
  GetNode("out").MarkDirty()
  string err
  if plan_.AddTarget(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  Edge* edge
  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat in
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat a1
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat a2
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat b1 b2
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if !edge { t.FailNow() }  // done
}

// Test that two edges from one output can both execute.
func TestPlanTest_DoubleDependent(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat a1 a2\n" "build a1: cat mid\n" "build a2: cat mid\n" "build mid: cat in\n"))
  GetNode("mid").MarkDirty()
  GetNode("a1").MarkDirty()
  GetNode("a2").MarkDirty()
  GetNode("out").MarkDirty()

  string err
  if plan_.AddTarget(GetNode("out"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  Edge* edge
  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat in
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat mid
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat mid
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }  // cat a1 a2
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if !edge { t.FailNow() }  // done
}

func (p *PlanTest) TestPoolWithDepthOne(test_case string) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, test_case))
  GetNode("out1").MarkDirty()
  GetNode("out2").MarkDirty()
  string err
  if plan_.AddTarget(GetNode("out1"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.AddTarget(GetNode("out2"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  edge := plan_.FindWork()
  if edge { t.FailNow() }
  if "in" !=  edge.inputs_[0].path() { t.FailNow() }
  if "out1" != edge.outputs_[0].path() { t.FailNow() }

  // This will be false since poolcat is serialized
  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if "in" != edge.inputs_[0].path() { t.FailNow() }
  if "out2" != edge.outputs_[0].path() { t.FailNow() }

  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  if !plan_.more_to_do() { t.FailNow() }
  edge = plan_.FindWork()
  if 0 != edge { t.FailNow() }
}

func TestPlanTest_PoolWithDepthOne(t *testing.T) {
  TestPoolWithDepthOne( "pool foobar\n" "  depth = 1\n" "rule poolcat\n" "  command = cat $in > $out\n" "  pool = foobar\n" "build out1: poolcat in\n" "build out2: poolcat in\n")
}

func TestPlanTest_ConsolePool(t *testing.T) {
  TestPoolWithDepthOne( "rule poolcat\n" "  command = cat $in > $out\n" "  pool = console\n" "build out1: poolcat in\n" "build out2: poolcat in\n")
}

func TestPlanTest_PoolsWithDepthTwo(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "pool foobar\n" "  depth = 2\n" "pool bazbin\n" "  depth = 2\n" "rule foocat\n" "  command = cat $in > $out\n" "  pool = foobar\n" "rule bazcat\n" "  command = cat $in > $out\n" "  pool = bazbin\n" "build out1: foocat in\n" "build out2: foocat in\n" "build out3: foocat in\n" "build outb1: bazcat in\n" "build outb2: bazcat in\n" "build outb3: bazcat in\n" "  pool =\n" "build allTheThings: cat out1 out2 out3 outb1 outb2 outb3\n" ))
  // Mark all the out* nodes dirty
  for (int i = 0; i < 3; ++i) {
    GetNode("out" + string(1, '1' + static_cast<char>(i))).MarkDirty()
    GetNode("outb" + string(1, '1' + static_cast<char>(i))).MarkDirty()
  }
  GetNode("allTheThings").MarkDirty()

  string err
  if plan_.AddTarget(GetNode("allTheThings"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  deque<Edge*> edges
  FindWorkSorted(&edges, 5)

  for (int i = 0; i < 4; ++i) {
    Edge *edge = edges[i]
    if "in" !=  edge.inputs_[0].path() { t.FailNow() }
    string base_name(i < 2 ? "out" : "outb")
    if base_name + string(1 != '1' + (i % 2)), edge.outputs_[0].path() { t.FailNow() }
  }

  // outb3 is exempt because it has an empty pool
  edge := edges[4]
  if edge { t.FailNow() }
  if "in" !=  edge.inputs_[0].path() { t.FailNow() }
  if "outb3" != edge.outputs_[0].path() { t.FailNow() }

  // finish out1
  plan_.EdgeFinished(edges.front(), Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }
  edges.pop_front()

  // out3 should be available
  Edge* out3 = plan_.FindWork()
  if out3 { t.FailNow() }
  if "in" !=  out3.inputs_[0].path() { t.FailNow() }
  if "out3" != out3.outputs_[0].path() { t.FailNow() }

  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(out3, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  if !plan_.FindWork() { t.FailNow() }

  for (deque<Edge*>::iterator it = edges.begin(); it != edges.end(); ++it) {
    plan_.EdgeFinished(*it, Plan::kEdgeSucceeded, &err)
    if "" != err { t.FailNow() }
  }

  last := plan_.FindWork()
  if last { t.FailNow() }
  if "allTheThings" != last.outputs_[0].path() { t.FailNow() }

  plan_.EdgeFinished(last, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  if !plan_.more_to_do() { t.FailNow() }
  if !plan_.FindWork() { t.FailNow() }
}

func TestPlanTest_PoolWithRedundantEdges(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "pool compile\n" "  depth = 1\n" "rule gen_foo\n" "  command = touch foo.cpp\n" "rule gen_bar\n" "  command = touch bar.cpp\n" "rule echo\n" "  command = echo $out > $out\n" "build foo.cpp.obj: echo foo.cpp || foo.cpp\n" "  pool = compile\n" "build bar.cpp.obj: echo bar.cpp || bar.cpp\n" "  pool = compile\n" "build libfoo.a: echo foo.cpp.obj bar.cpp.obj\n" "build foo.cpp: gen_foo\n" "build bar.cpp: gen_bar\n" "build all: phony libfoo.a\n"))
  GetNode("foo.cpp").MarkDirty()
  GetNode("foo.cpp.obj").MarkDirty()
  GetNode("bar.cpp").MarkDirty()
  GetNode("bar.cpp.obj").MarkDirty()
  GetNode("libfoo.a").MarkDirty()
  GetNode("all").MarkDirty()
  string err
  if plan_.AddTarget(GetNode("all"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  edge := nil

  deque<Edge*> initial_edges
  FindWorkSorted(&initial_edges, 2)

  edge = initial_edges[1]  // Foo first
  if "foo.cpp" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if !plan_.FindWork() { t.FailNow() }
  if "foo.cpp" != edge.inputs_[0].path() { t.FailNow() }
  if "foo.cpp" != edge.inputs_[1].path() { t.FailNow() }
  if "foo.cpp.obj" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = initial_edges[0]  // Now for bar
  if "bar.cpp" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if !plan_.FindWork() { t.FailNow() }
  if "bar.cpp" != edge.inputs_[0].path() { t.FailNow() }
  if "bar.cpp" != edge.inputs_[1].path() { t.FailNow() }
  if "bar.cpp.obj" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if !plan_.FindWork() { t.FailNow() }
  if "foo.cpp.obj" != edge.inputs_[0].path() { t.FailNow() }
  if "bar.cpp.obj" != edge.inputs_[1].path() { t.FailNow() }
  if "libfoo.a" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if !plan_.FindWork() { t.FailNow() }
  if "libfoo.a" != edge.inputs_[0].path() { t.FailNow() }
  if "all" != edge.outputs_[0].path() { t.FailNow() }
  plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if !edge { t.FailNow() }
  if !plan_.more_to_do() { t.FailNow() }
}

func TestPlanTest_PoolWithFailingEdge(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "pool foobar\n" "  depth = 1\n" "rule poolcat\n" "  command = cat $in > $out\n" "  pool = foobar\n" "build out1: poolcat in\n" "build out2: poolcat in\n"))
  GetNode("out1").MarkDirty()
  GetNode("out2").MarkDirty()
  string err
  if plan_.AddTarget(GetNode("out1"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.AddTarget(GetNode("out2"), &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if plan_.more_to_do() { t.FailNow() }

  edge := plan_.FindWork()
  if edge { t.FailNow() }
  if "in" !=  edge.inputs_[0].path() { t.FailNow() }
  if "out1" != edge.outputs_[0].path() { t.FailNow() }

  // This will be false since poolcat is serialized
  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeFailed, &err)
  if "" != err { t.FailNow() }

  edge = plan_.FindWork()
  if edge { t.FailNow() }
  if "in" != edge.inputs_[0].path() { t.FailNow() }
  if "out2" != edge.outputs_[0].path() { t.FailNow() }

  if !plan_.FindWork() { t.FailNow() }

  plan_.EdgeFinished(edge, Plan::kEdgeFailed, &err)
  if "" != err { t.FailNow() }

  if plan_.more_to_do() { t.FailNow() } // Jobs have failed
  edge = plan_.FindWork()
  if 0 != edge { t.FailNow() }
}

// Fake implementation of CommandRunner, useful for tests.
type FakeCommandRunner struct {
  explicit FakeCommandRunner(VirtualFileSystem* fs) :
      max_active_edges_(1), fs_(fs) {}

  vector<string> commands_ran_
  vector<Edge*> active_edges_
  size_t max_active_edges_
  VirtualFileSystem* fs_
}

type BuildTest struct {
  BuildTest() : config_(MakeConfig()), command_runner_(&fs_), status_(config_),
                builder_(&state_, config_, nil, nil, &fs_, &status_, 0) {
  }

  explicit BuildTest(DepsLog* log)
      : config_(MakeConfig()), command_runner_(&fs_), status_(config_),
        builder_(&state_, config_, nil, log, &fs_, &status_, 0) {}

  func (b *BuildTest) SetUp() {
    StateTestWithBuiltinRules::SetUp()

    builder_.command_runner_.reset(&command_runner_)
    AssertParse(&state_, "build cat1: cat in1\n" "build cat2: cat in1 in2\n" "build cat12: cat cat1 cat2\n")

    fs_.Create("in1", "")
    fs_.Create("in2", "")
  }

  ~BuildTest() {
    builder_.command_runner_.release()
  }

  virtual bool IsPathDead(StringPiece s) const { return false; }

  func (b *BuildTest) MakeConfig() BuildConfig {
    BuildConfig config
    config.verbosity = BuildConfig::QUIET
    return config
  }

  BuildConfig config_
  FakeCommandRunner command_runner_
  VirtualFileSystem fs_
  StatusPrinter status_
  Builder builder_
}

// Rebuild target in the 'working tree' (fs_).
// State of command_runner_ and logs contents (if specified) ARE MODIFIED.
// Handy to check for NOOP builds, and higher-level rebuild tests.
func (b *BuildTest) RebuildTarget(target string, manifest string, log_path string, deps_path string, state *State) {
  State local_state, *pstate = &local_state
  if state != nil {
    pstate = state
  }
  ASSERT_NO_FATAL_FAILURE(AddCatRule(pstate))
  AssertParse(pstate, manifest)

  string err
  BuildLog build_log, *pbuild_log = nil
  if log_path {
    if build_log.Load(log_path, &err) { t.FailNow() }
    if build_log.OpenForWrite(log_path, *this, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    pbuild_log = &build_log
  }

  DepsLog deps_log, *pdeps_log = nil
  if deps_path {
    if deps_log.Load(deps_path, pstate, &err) { t.FailNow() }
    if deps_log.OpenForWrite(deps_path, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    pdeps_log = &deps_log
  }

  Builder builder(pstate, config_, pbuild_log, pdeps_log, &fs_, &status_, 0)
  if builder.AddTarget(target, &err) { t.FailNow() }

  command_runner_.commands_ran_ = nil
  builder.command_runner_.reset(&command_runner_)
  if !builder.AlreadyUpToDate() {
    build_res := builder.Build(&err)
    if build_res { t.FailNow() }
  }
  builder.command_runner_.release()
}

// CommandRunner impl
func (f *FakeCommandRunner) CanRunMore() bool {
  return active_edges_.size() < max_active_edges_
}

func (f *FakeCommandRunner) StartCommand(edge *Edge) bool {
  if !active_edges_.size() < max_active_edges_ { panic("oops") }
  if !find(active_edges_.begin(), active_edges_.end(), edge) == active_edges_.end() { panic("oops") }
  commands_ran_.push_back(edge.EvaluateCommand())
  if edge.rule().name() == "cat"  || edge.rule().name() == "cat_rsp" || edge.rule().name() == "cat_rsp_out" || edge.rule().name() == "cc" || edge.rule().name() == "cp_multi_msvc" || edge.rule().name() == "cp_multi_gcc" || edge.rule().name() == "touch" || edge.rule().name() == "touch-interrupt" || edge.rule().name() == "touch-fail-tick2" {
    for (vector<Node*>::iterator out = edge.outputs_.begin(); out != edge.outputs_.end(); ++out) {
      fs_.Create((*out).path(), "")
    }
  } else if edge.rule().name() == "true" || edge.rule().name() == "fail" || edge.rule().name() == "interrupt" || edge.rule().name() == "console" {
    // Don't do anything.
  } else if edge.rule().name() == "cp" {
    if !!edge.inputs_.empty() { panic("oops") }
    if !edge.outputs_.size() == 1 { panic("oops") }
    string content
    string err
    if fs_.ReadFile(edge.inputs_[0].path(), &content, &err) == DiskInterface::Okay {
      fs_.WriteFile(edge.outputs_[0].path(), content)
    }
  } else if edge.rule().name() == "touch-implicit-dep-out" {
    dep := edge.GetBinding("test_dependency")
    fs_.Create(dep, "")
    fs_.Tick()
    for (vector<Node*>::iterator out = edge.outputs_.begin(); out != edge.outputs_.end(); ++out) {
      fs_.Create((*out).path(), "")
    }
  } else if edge.rule().name() == "touch-out-implicit-dep" {
    dep := edge.GetBinding("test_dependency")
    for (vector<Node*>::iterator out = edge.outputs_.begin(); out != edge.outputs_.end(); ++out) {
      fs_.Create((*out).path(), "")
    }
    fs_.Tick()
    fs_.Create(dep, "")
  } else if edge.rule().name() == "generate-depfile" {
    dep := edge.GetBinding("test_dependency")
    depfile := edge.GetUnescapedDepfile()
    string contents
    for (vector<Node*>::iterator out = edge.outputs_.begin(); out != edge.outputs_.end(); ++out) {
      contents += (*out).path() + ": " + dep + "\n"
      fs_.Create((*out).path(), "")
    }
    fs_.Create(depfile, contents)
  } else {
    printf("unknown command\n")
    return false
  }

  active_edges_.push_back(edge)

  // Allow tests to control the order by the name of the first output.
  sort(active_edges_.begin(), active_edges_.end(), CompareEdgesByOutput::cmp)

  return true
}

func (f *FakeCommandRunner) WaitForCommand(result *Result) bool {
  if active_edges_.empty() {
    return false
  }

  // All active edges were already completed immediately when started,
  // so we can pick any edge here.  Pick the last edge.  Tests can
  // control the order of edges by the name of the first output.
  edge_iter := active_edges_.end() - 1

  edge := *edge_iter
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
    active_edges_.erase(edge_iter)
    return true
  }

  if edge.rule().name() == "cp_multi_msvc" {
    const string prefix = edge.GetBinding("msvc_deps_prefix")
    for (vector<Node*>::iterator in = edge.inputs_.begin(); in != edge.inputs_.end(); ++in) {
      result.output += prefix + (*in).path() + '\n'
    }
  }

  if edge.rule().name() == "fail" || (edge.rule().name() == "touch-fail-tick2" && fs_.now_ == 2) {
    result.status = ExitFailure
  } else {
    result.status = ExitSuccess
  }

  // Provide a way for test cases to verify when an edge finishes that
  // some other edge is still active.  This is useful for test cases
  // covering behavior involving multiple active edges.
  verify_active_edge := edge.GetBinding("verify_active_edge")
  if !verify_active_edge.empty() {
    verify_active_edge_found := false
    for (vector<Edge*>::iterator i = active_edges_.begin(); i != active_edges_.end(); ++i) {
      if !(*i).outputs_.empty() && (*i).outputs_[0].path() == verify_active_edge {
        verify_active_edge_found = true
      }
    }
    if verify_active_edge_found { t.FailNow() }
  }

  active_edges_.erase(edge_iter)
  return true
}

func (f *FakeCommandRunner) GetActiveEdges() vector<Edge*> {
  return active_edges_
}

func (f *FakeCommandRunner) Abort() {
  active_edges_ = nil
}

// Mark a path dirty.
func (b *BuildTest) Dirty(path string) {
  node := GetNode(path)
  node.MarkDirty()

  // If it's an input file, mark that we've already stat()ed it and
  // it's missing.
  if !node.in_edge() {
    node.MarkMissing()
  }
}

func TestBuildTest_NoWork(t *testing.T) {
  string err
  if builder_.AlreadyUpToDate() { t.FailNow() }
}

func TestBuildTest_OneStep(t *testing.T) {
  // Given a dirty target with one ready input,
  // we should rebuild the target.
  Dirty("cat1")
  string err
  if builder_.AddTarget("cat1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cat in1 > cat1" != command_runner_.commands_ran_[0] { t.FailNow() }
}

TEST_F(BuildTest, OneStep2) {
  // Given a target with one dirty input,
  // we should rebuild the target.
  Dirty("cat1")
  string err
  if builder_.AddTarget("cat1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cat in1 > cat1" != command_runner_.commands_ran_[0] { t.FailNow() }
}

func TestBuildTest_TwoStep(t *testing.T) {
  string err
  if builder_.AddTarget("cat12", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  // Depending on how the pointers work out, we could've ran
  // the first two commands in either order.
  if (command_runner_.commands_ran_[0] == "cat in1 > cat1" && command_runner_.commands_ran_[1] == "cat in1 in2 > cat2") || (command_runner_.commands_ran_[1] == "cat in1 > cat1" && command_runner_.commands_ran_[0] == "cat in1 in2 > cat2") { t.FailNow() }

  if "cat cat1 cat2 > cat12" != command_runner_.commands_ran_[2] { t.FailNow() }

  fs_.Tick()

  // Modifying in2 requires rebuilding one intermediate file
  // and the final file.
  fs_.Create("in2", "")
  state_.Reset()
  if builder_.AddTarget("cat12", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 5u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cat in1 in2 > cat2" != command_runner_.commands_ran_[3] { t.FailNow() }
  if "cat cat1 cat2 > cat12" != command_runner_.commands_ran_[4] { t.FailNow() }
}

func TestBuildTest_TwoOutputs(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build out1 out2: touch in.txt\n"))

  fs_.Create("in.txt", "")

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "touch out1 out2" != command_runner_.commands_ran_[0] { t.FailNow() }
}

func TestBuildTest_ImplicitOutput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "build out | out.imp: touch in.txt\n"))
  fs_.Create("in.txt", "")

  string err
  if builder_.AddTarget("out.imp", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[0] { t.FailNow() }
}

// Test case from
//   https://github.com/ninja-build/ninja/issues/148
func TestBuildTest_MultiOutIn(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build in1 otherfile: touch in\n" "build out: touch in | in1\n"))

  fs_.Create("in", "")
  fs_.Tick()
  fs_.Create("in1", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestBuildTest_Chain(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build c2: cat c1\n" "build c3: cat c2\n" "build c4: cat c3\n" "build c5: cat c4\n"))

  fs_.Create("c1", "")

  string err
  if builder_.AddTarget("c5", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 4u != command_runner_.commands_ran_.size() { t.FailNow() }

  err = nil
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("c5", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  fs_.Tick()

  fs_.Create("c3", "")
  err = nil
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("c5", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }  // 3->4, 4->5
}

func TestBuildTest_MissingInput(t *testing.T) {
  // Input is referenced by build file, but no rule for it.
  string err
  Dirty("in1")
  if !builder_.AddTarget("cat1", &err) { t.FailNow() }
  if "'in1' != needed by 'cat1', missing and no known rule to make it", err { t.FailNow() }
}

func TestBuildTest_MissingTarget(t *testing.T) {
  // Target is not referenced by build file.
  string err
  if !builder_.AddTarget("meow", &err) { t.FailNow() }
  if "unknown target: 'meow'" != err { t.FailNow() }
}

func TestBuildTest_MakeDirs(t *testing.T) {
  string err

  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build subdir\\dir2\\file: cat in1\n"))
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build subdir/dir2/file: cat in1\n"))
  if builder_.AddTarget("subdir/dir2/file", &err) { t.FailNow() }

  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != fs_.directories_made_.size() { t.FailNow() }
  if "subdir" != fs_.directories_made_[0] { t.FailNow() }
  if "subdir/dir2" != fs_.directories_made_[1] { t.FailNow() }
}

func TestBuildTest_DepFileMissing(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n  depfile = $out.d\n" "build fo$ o.o: cc foo.c\n"))
  fs_.Create("foo.c", "")

  if builder_.AddTarget("fo o.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != fs_.files_read_.size() { t.FailNow() }
  if "fo o.o.d" != fs_.files_read_[0] { t.FailNow() }
}

func TestBuildTest_DepFileOK(t *testing.T) {
  string err
  orig_edges := state_.edges_.size()
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n  depfile = $out.d\n" "build foo.o: cc foo.c\n"))
  edge := state_.edges_.back()

  fs_.Create("foo.c", "")
  GetNode("bar.h").MarkDirty()  // Mark bar.h as missing.
  fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != fs_.files_read_.size() { t.FailNow() }
  if "foo.o.d" != fs_.files_read_[0] { t.FailNow() }

  // Expect three new edges: one generating foo.o, and two more from
  // loading the depfile.
  if orig_edges + 3 != (int)state_.edges_.size() { t.FailNow() }
  // Expect our edge to now have three inputs: foo.c and two headers.
  if 3u != edge.inputs_.size() { t.FailNow() }

  // Expect the command line we generate to only use the original input.
  if "cc foo.c" != edge.EvaluateCommand() { t.FailNow() }
}

func TestBuildTest_DepFileParseError(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n  depfile = $out.d\n" "build foo.o: cc foo.c\n"))
  fs_.Create("foo.c", "")
  fs_.Create("foo.o.d", "randomtext\n")
  if !builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if "foo.o.d: expected ':' in depfile" != err { t.FailNow() }
}

func TestBuildTest_EncounterReadyTwice(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build c: touch\n" "build b: touch || c\n" "build a: touch | b || c\n"))

  c_out := GetNode("c").out_edges()
  if 2u != c_out.size() { t.FailNow() }
  if "b" != c_out[0].outputs_[0].path() { t.FailNow() }
  if "a" != c_out[1].outputs_[0].path() { t.FailNow() }

  fs_.Create("b", "")
  if builder_.AddTarget("a", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_OrderOnlyDeps(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n  depfile = $out.d\n" "build foo.o: cc foo.c || otherfile\n"))
  edge := state_.edges_.back()

  fs_.Create("foo.c", "")
  fs_.Create("otherfile", "")
  fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // One explicit, two implicit, one order only.
  if 4u != edge.inputs_.size() { t.FailNow() }
  if 2 != edge.implicit_deps_ { t.FailNow() }
  if 1 != edge.order_only_deps_ { t.FailNow() }
  // Verify the inputs are in the order we expect
  // (explicit then implicit then orderonly).
  if "foo.c" != edge.inputs_[0].path() { t.FailNow() }
  if "blah.h" != edge.inputs_[1].path() { t.FailNow() }
  if "bar.h" != edge.inputs_[2].path() { t.FailNow() }
  if "otherfile" != edge.inputs_[3].path() { t.FailNow() }

  // Expect the command line we generate to only use the original input.
  if "cc foo.c" != edge.EvaluateCommand() { t.FailNow() }

  // explicit dep dirty, expect a rebuild.
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  fs_.Tick()

  // Recreate the depfile, as it should have been deleted by the build.
  fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")

  // implicit dep dirty, expect a rebuild.
  fs_.Create("blah.h", "")
  fs_.Create("bar.h", "")
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  fs_.Tick()

  // Recreate the depfile, as it should have been deleted by the build.
  fs_.Create("foo.o.d", "foo.o: blah.h bar.h\n")

  // order only dep dirty, no rebuild.
  fs_.Create("otherfile", "")
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  // implicit dep missing, expect rebuild.
  fs_.RemoveFile("bar.h")
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_RebuildOrderOnlyDeps(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n" "rule true\n  command = true\n" "build oo.h: cc oo.h.in\n" "build foo.o: cc foo.c || oo.h\n"))

  fs_.Create("foo.c", "")
  fs_.Create("oo.h.in", "")

  // foo.o and order-only dep dirty, build both.
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // all clean, no rebuild.
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  // order-only dep missing, build it only.
  fs_.RemoveFile("oo.h")
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cc oo.h.in" != command_runner_.commands_ran_[0] { t.FailNow() }

  fs_.Tick()

  // order-only dep dirty, build it only.
  fs_.Create("oo.h.in", "")
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("foo.o", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cc oo.h.in" != command_runner_.commands_ran_[0] { t.FailNow() }
}

func TestBuildTest_DepFileCanonicalize(t *testing.T) {
  string err
  orig_edges := state_.edges_.size()
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n  command = cc $in\n  depfile = $out.d\n" "build gen/stuff\\things/foo.o: cc x\\y/z\\foo.c\n"))
  edge := state_.edges_.back()

  fs_.Create("x/y/z/foo.c", "")
  GetNode("bar.h").MarkDirty()  // Mark bar.h as missing.
  // Note, different slashes from manifest.
  fs_.Create("gen/stuff\\things/foo.o.d", "gen\\stuff\\things\\foo.o: blah.h bar.h\n")
  if builder_.AddTarget("gen/stuff/things/foo.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != fs_.files_read_.size() { t.FailNow() }
  // The depfile path does not get Canonicalize as it seems unnecessary.
  if "gen/stuff\\things/foo.o.d" != fs_.files_read_[0] { t.FailNow() }

  // Expect three new edges: one generating foo.o, and two more from
  // loading the depfile.
  if orig_edges + 3 != (int)state_.edges_.size() { t.FailNow() }
  // Expect our edge to now have three inputs: foo.c and two headers.
  if 3u != edge.inputs_.size() { t.FailNow() }

  // Expect the command line we generate to only use the original input, and
  // using the slashes from the manifest.
  if "cc x\\y/z\\foo.c" != edge.EvaluateCommand() { t.FailNow() }
}

func TestBuildTest_Phony(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat bar.cc\n" "build all: phony out\n"))
  fs_.Create("bar.cc", "")

  if builder_.AddTarget("all", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // Only one command to run, because phony runs no command.
  if !builder_.AlreadyUpToDate() { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_PhonyNoWork(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat bar.cc\n" "build all: phony out\n"))
  fs_.Create("bar.cc", "")
  fs_.Create("out", "")

  if builder_.AddTarget("all", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
}

// Test a self-referencing phony.  Ideally this should not work, but
// ninja 1.7 and below tolerated and CMake 2.8.12.x and 3.0.x both
// incorrectly produce it.  We tolerate it for compatibility.
func TestBuildTest_PhonySelfReference(t *testing.T) {
  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build a: phony a\n"))

  if builder_.AddTarget("a", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
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
func TestPhonyUseCase(t *BuildTest, i int) {
  state_ := t.state_
  builder_ := t.builder_
  command_runner_ := t.command_runner_
  fs_ := t.fs_

  string err
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" " command = touch $out\n" "build notreal: phony blank\n" "build phony1: phony notreal\n" "build phony2: phony\n" "build phony3: phony blank\n" "build phony4: phony notreal\n" "build phony5: phony\n" "build phony6: phony blank\n" "\n" "build test1: touch phony1\n" "build test2: touch phony2\n" "build test3: touch phony3\n" "build test4: touch phony4\n" "build test5: touch phony5\n" "build test6: touch phony6\n" ))

  // Set up test.
  builder_.command_runner_.release() // BuildTest owns the CommandRunner
  builder_.command_runner_.reset(&command_runner_)

  fs_.Create("blank", "")  // a "real" file
  if builder_.AddTarget("test1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("test2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("test3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("test4", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("test5", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("test6", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }

  string ci
  ci += static_cast<char>('0' + i)

  // Tests 1, 3, 4, and 6 should rebuild when the input is updated.
  if i != 2 && i != 5 {
    Node* testNode  = t.GetNode("test" + ci)
    phonyNode := t.GetNode("phony" + ci)
    inputNode := t.GetNode("blank")

    state_.Reset()
    startTime := fs_.now_

    // Build number 1
    if builder_.AddTarget("test" + ci, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if !builder_.AlreadyUpToDate() {
      if builder_.Build(&err) { t.FailNow() }
    }
    if "" != err { t.FailNow() }

    // Touch the input file
    state_.Reset()
    command_runner_.commands_ran_ = nil
    fs_.Tick()
    fs_.Create("blank", "")  // a "real" file
    if builder_.AddTarget("test" + ci, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Second build, expect testN edge to be rebuilt
    // and phonyN node's mtime to be updated.
    if !builder_.AlreadyUpToDate() { t.FailNow() }
    if builder_.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
    if string("touch test") + ci != command_runner_.commands_ran_[0] { t.FailNow() }
    if builder_.AlreadyUpToDate() { t.FailNow() }

    inputTime := inputNode.mtime()

    if !phonyNode.exists() { t.FailNow() }
    if !phonyNode.dirty() { t.FailNow() }

    if phonyNode.mtime() <= startTime { t.FailNow() }
    if phonyNode.mtime() != inputTime { t.FailNow() }
    if testNode.Stat(&fs_, &err) { t.FailNow() }
    if testNode.exists() { t.FailNow() }
    if testNode.mtime() <= startTime { t.FailNow() }
  } else {
    // Tests 2 and 5: Expect dependents to always rebuild.

    state_.Reset()
    command_runner_.commands_ran_ = nil
    fs_.Tick()
    command_runner_.commands_ran_ = nil
    if builder_.AddTarget("test" + ci, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if !builder_.AlreadyUpToDate() { t.FailNow() }
    if builder_.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
    if "touch test" + ci != command_runner_.commands_ran_[0] { t.FailNow() }

    state_.Reset()
    command_runner_.commands_ran_ = nil
    if builder_.AddTarget("test" + ci, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if !builder_.AlreadyUpToDate() { t.FailNow() }
    if builder_.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
    if "touch test" + ci != command_runner_.commands_ran_[0] { t.FailNow() }
  }
}

TEST_F(BuildTest, PhonyUseCase1) { TestPhonyUseCase(this, 1); }
TEST_F(BuildTest, PhonyUseCase2) { TestPhonyUseCase(this, 2); }
TEST_F(BuildTest, PhonyUseCase3) { TestPhonyUseCase(this, 3); }
TEST_F(BuildTest, PhonyUseCase4) { TestPhonyUseCase(this, 4); }
TEST_F(BuildTest, PhonyUseCase5) { TestPhonyUseCase(this, 5); }
TEST_F(BuildTest, PhonyUseCase6) { TestPhonyUseCase(this, 6); }

func TestBuildTest_Fail(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule fail\n" "  command = fail\n" "build out1: fail\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "subcommand failed" != err { t.FailNow() }
}

func TestBuildTest_SwallowFailures(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule fail\n" "  command = fail\n" "build out1: fail\n" "build out2: fail\n" "build out3: fail\n" "build all: phony out1 out2 out3\n"))

  // Swallow two failures, die on the third.
  config_.failures_allowed = 3

  string err
  if builder_.AddTarget("all", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "subcommands failed" != err { t.FailNow() }
}

func TestBuildTest_SwallowFailuresLimit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule fail\n" "  command = fail\n" "build out1: fail\n" "build out2: fail\n" "build out3: fail\n" "build final: cat out1 out2 out3\n"))

  // Swallow ten failures; we should stop before building final.
  config_.failures_allowed = 11

  string err
  if builder_.AddTarget("final", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cannot make progress due to previous errors" != err { t.FailNow() }
}

func TestBuildTest_SwallowFailuresPool(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "pool failpool\n" "  depth = 1\n" "rule fail\n" "  command = fail\n" "  pool = failpool\n" "build out1: fail\n" "build out2: fail\n" "build out3: fail\n" "build final: cat out1 out2 out3\n"))

  // Swallow ten failures; we should stop before building final.
  config_.failures_allowed = 11

  string err
  if builder_.AddTarget("final", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cannot make progress due to previous errors" != err { t.FailNow() }
}

func TestBuildTest_PoolEdgesReadyButNotWanted(t *testing.T) {
  fs_.Create("x", "")

  string manifest =
    "pool some_pool\n"
    "  depth = 4\n"
    "rule touch\n"
    "  command = touch $out\n"
    "  pool = some_pool\n"
    "rule cc\n"
    "  command = touch grit\n"
    "\n"
    "build B.d.stamp: cc | x\n"
    "build C.stamp: touch B.d.stamp\n"
    "build final.stamp: touch || C.stamp\n"

  RebuildTarget("final.stamp", manifest)

  fs_.RemoveFile("B.d.stamp")

  State save_state
  RebuildTarget("final.stamp", manifest, nil, nil, &save_state)
  if save_state.LookupPool("some_pool").current_use() < 0 { t.FailNow() }
}

type BuildWithLogTest struct {
  BuildWithLogTest() {
    builder_.SetBuildLog(&build_log_)
  }

  BuildLog build_log_
}

func TestBuildWithLogTest_ImplicitGeneratedOutOfDate(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "  generator = 1\n" "build out.imp: touch | in\n"))
  fs_.Create("out.imp", "")
  fs_.Tick()
  fs_.Create("in", "")

  string err

  if builder_.AddTarget("out.imp", &err) { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }

  if GetNode("out.imp").dirty() { t.FailNow() }
}

TEST_F(BuildWithLogTest, ImplicitGeneratedOutOfDate2) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch-implicit-dep-out\n" "  command = touch $test_dependency ; sleep 1 ; touch $out\n" "  generator = 1\n" "build out.imp: touch-implicit-dep-out | inimp inimp2\n" "  test_dependency = inimp\n"))
  fs_.Create("inimp", "")
  fs_.Create("out.imp", "")
  fs_.Tick()
  fs_.Create("inimp2", "")
  fs_.Tick()

  string err

  if builder_.AddTarget("out.imp", &err) { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()
  builder_.Cleanup()
  builder_.plan_.Reset()

  if builder_.AddTarget("out.imp", &err) { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
  if !GetNode("out.imp").dirty() { t.FailNow() }
}

func TestBuildWithLogTest_NotInLogButOnDisk(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n" "  command = cc\n" "build out1: cc in\n"))

  // Create input/output that would be considered up to date when
  // not considering the command line hash.
  fs_.Create("in", "")
  fs_.Create("out1", "")
  string err

  // Because it's not in the log, it should not be up-to-date until
  // we build again.
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()

  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
}

func TestBuildWithLogTest_RebuildAfterFailure(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch-fail-tick2\n" "  command = touch-fail-tick2\n" "build out1: touch-fail-tick2 in\n"))

  string err

  fs_.Create("in", "")

  // Run once successfully to get out1 in the log
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()
  builder_.Cleanup()
  builder_.plan_.Reset()

  fs_.Tick()
  fs_.Create("in", "")

  // Run again with a failure that updates the output file timestamp
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if !builder_.Build(&err) { t.FailNow() }
  if "subcommand failed" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()
  builder_.Cleanup()
  builder_.plan_.Reset()

  fs_.Tick()

  // Run again, should rerun even though the output file is up to date on disk
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestBuildWithLogTest_RebuildWithNoInputs(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch\n" "build out1: touch\n" "build out2: touch in\n"))

  string err

  fs_.Create("in", "")

  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()

  fs_.Tick()

  fs_.Create("in", "")

  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildWithLogTest_RestatTest(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  restat = 1\n" "rule cc\n" "  command = cc\n" "  restat = 1\n" "build out1: cc in\n" "build out2: true out1\n" "build out3: cat out2\n"))

  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Create("out3", "")

  fs_.Tick()

  fs_.Create("in", "")

  // Do a pre-build so that there's commands in the log for the outputs,
  // otherwise, the lack of an entry in the build log will cause out3 to rebuild
  // regardless of restat.
  string err
  if builder_.AddTarget("out3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if 3u != builder_.plan_.command_edge_count() { t.FailNow() }
  command_runner_.commands_ran_ = nil
  state_.Reset()

  fs_.Tick()

  fs_.Create("in", "")
  // "cc" touches out1, so we should build out2.  But because "true" does not
  // touch out2, we should cancel the build of out3.
  if builder_.AddTarget("out3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // If we run again, it should be a no-op, because the build log has recorded
  // that we've already built out2 with an input timestamp of 2 (from out1).
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  fs_.Tick()

  fs_.Create("in", "")

  // The build log entry should not, however, prevent us from rebuilding out2
  // if out1 changes.
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildWithLogTest_RestatMissingFile(t *testing.T) {
  // If a restat rule doesn't create its output, and the output didn't
  // exist before the rule was run, consider that behavior equivalent
  // to a rule that doesn't modify its existent output file.

  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  restat = 1\n" "rule cc\n" "  command = cc\n" "build out1: true in\n" "build out2: cc out1\n"))

  fs_.Create("in", "")
  fs_.Create("out2", "")

  // Do a pre-build so that there's commands in the log for the outputs,
  // otherwise, the lack of an entry in the build log will cause out2 to rebuild
  // regardless of restat.
  string err
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  command_runner_.commands_ran_ = nil
  state_.Reset()

  fs_.Tick()
  fs_.Create("in", "")
  fs_.Create("out2", "")

  // Run a build, expect only the first command to run.
  // It doesn't touch its output (due to being the "true" command), so
  // we shouldn't run the dependent build.
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildWithLogTest_RestatSingleDependentOutputDirty(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  restat = 1\n" "rule touch\n" "  command = touch\n" "build out1: true in\n" "build out2 out3: touch out1\n" "build out4: touch out2\n" ))

  // Create the necessary files
  fs_.Create("in", "")

  string err
  if builder_.AddTarget("out4", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }

  fs_.Tick()
  fs_.Create("in", "")
  fs_.RemoveFile("out3")

  // Since "in" is missing, out1 will be built. Since "out3" is missing,
  // out2 and out3 will be built even though "in" is not touched when built.
  // Then, since out2 is rebuilt, out4 should be rebuilt -- the restat on the
  // "true" rule should not lead to the "touch" edge writing out2 and out3 being
  // cleard.
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out4", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
}

// Test scenario, in which an input file is removed, but output isn't changed
// https://github.com/ninja-build/ninja/issues/295
func TestBuildWithLogTest_RestatMissingInput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  depfile = $out.d\n" "  restat = 1\n" "rule cc\n" "  command = cc\n" "build out1: true in\n" "build out2: cc out1\n"))

  // Create all necessary files
  fs_.Create("in", "")

  // The implicit dependencies and the depfile itself
  // are newer than the output
  restat_mtime := fs_.Tick()
  fs_.Create("out1.d", "out1: will.be.deleted restat.file\n")
  fs_.Create("will.be.deleted", "")
  fs_.Create("restat.file", "")

  // Run the build, out1 and out2 get built
  string err
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // See that an entry in the logfile is created, capturing
  // the right mtime
  log_entry := build_log_.LookupByOutput("out1")
  if nil != log_entry { t.FailNow() }
  if restat_mtime != log_entry.mtime { t.FailNow() }

  // Now remove a file, referenced from depfile, so that target becomes
  // dirty, but the output does not change
  fs_.RemoveFile("will.be.deleted")

  // Trigger the build again - only out1 gets built
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  // Check that the logfile entry remains correctly set
  log_entry = build_log_.LookupByOutput("out1")
  if nil != log_entry { t.FailNow() }
  if restat_mtime != log_entry.mtime { t.FailNow() }
}

func TestBuildWithLogTest_GeneratedPlainDepfileMtime(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule generate-depfile\n" "  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\n" "build out: generate-depfile\n" "  test_dependency = inimp\n" "  depfile = out.d\n"))
  fs_.Create("inimp", "")
  fs_.Tick()

  string err

  if builder_.AddTarget("out", &err) { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()
  builder_.Cleanup()
  builder_.plan_.Reset()

  if builder_.AddTarget("out", &err) { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
}

type BuildDryRun struct {
  BuildDryRun() {
    config_.dry_run = true
  }
}

func TestBuildDryRun_AllCommandsShown(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  restat = 1\n" "rule cc\n" "  command = cc\n" "  restat = 1\n" "build out1: cc in\n" "build out2: true out1\n" "build out3: cat out2\n"))

  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Create("out3", "")

  fs_.Tick()

  fs_.Create("in", "")

  // "cc" touches out1, so we should build out2.  But because "true" does not
  // touch out2, we should cancel the build of out3.
  string err
  if builder_.AddTarget("out3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
}

// Test that RSP files are created when & where appropriate and deleted after
// successful execution.
TEST_F(BuildTest, RspFileSuccess)
{
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cat_rsp\n" "  command = cat $rspfile > $out\n" "  rspfile = $rspfile\n" "  rspfile_content = $long_command\n" "rule cat_rsp_out\n" "  command = cat $rspfile > $out\n" "  rspfile = $out.rsp\n" "  rspfile_content = $long_command\n" "build out1: cat in\n" "build out2: cat_rsp in\n" "  rspfile = out 2.rsp\n" "  long_command = Some very long command\n" "build out$ 3: cat_rsp_out in\n" "  long_command = Some very long command\n"))

  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Create("out 3", "")

  fs_.Tick()

  fs_.Create("in", "")

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AddTarget("out 3", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  size_t files_created = fs_.files_created_.size()
  size_t files_removed = fs_.files_removed_.size()

  if builder_.Build(&err) { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }

  // The RSP files were created
  if files_created + 2 != fs_.files_created_.size() { t.FailNow() }
  if 1u != fs_.files_created_.count("out 2.rsp") { t.FailNow() }
  if 1u != fs_.files_created_.count("out 3.rsp") { t.FailNow() }

  // The RSP files were removed
  if files_removed + 2 != fs_.files_removed_.size() { t.FailNow() }
  if 1u != fs_.files_removed_.count("out 2.rsp") { t.FailNow() }
  if 1u != fs_.files_removed_.count("out 3.rsp") { t.FailNow() }
}

// Test that RSP file is created but not removed for commands, which fail
func TestBuildTest_RspFileFailure(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule fail\n" "  command = fail\n" "  rspfile = $rspfile\n" "  rspfile_content = $long_command\n" "build out: fail in\n" "  rspfile = out.rsp\n" "  long_command = Another very long command\n"))

  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("in", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  size_t files_created = fs_.files_created_.size()
  size_t files_removed = fs_.files_removed_.size()

  if !builder_.Build(&err) { t.FailNow() }
  if "subcommand failed" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  // The RSP file was created
  if files_created + 1 != fs_.files_created_.size() { t.FailNow() }
  if 1u != fs_.files_created_.count("out.rsp") { t.FailNow() }

  // The RSP file was NOT removed
  if files_removed != fs_.files_removed_.size() { t.FailNow() }
  if 0u != fs_.files_removed_.count("out.rsp") { t.FailNow() }

  // The RSP file contains what it should
  if "Another very long command" != fs_.files_["out.rsp"].contents { t.FailNow() }
}

// Test that contents of the RSP file behaves like a regular part of
// command line, i.e. triggers a rebuild if changed
func TestBuildWithLogTest_RspFileCmdLineChange(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cat_rsp\n" "  command = cat $rspfile > $out\n" "  rspfile = $rspfile\n" "  rspfile_content = $long_command\n" "build out: cat_rsp in\n" "  rspfile = out.rsp\n" "  long_command = Original very long command\n"))

  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("in", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // 1. Build for the 1st time (-> populate log)
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  // 2. Build again (no change)
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  // 3. Alter the entry in the logfile
  // (to simulate a change in the command line between 2 builds)
  log_entry := build_log_.LookupByOutput("out")
  if nil != log_entry { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash( "cat out.rsp > out;rspfile=Original very long command", log_entry.command_hash))
  log_entry.command_hash++  // Change the command hash to something else.
  // Now expect the target to be rebuilt
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_InterruptCleanup(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule interrupt\n" "  command = interrupt\n" "rule touch-interrupt\n" "  command = touch-interrupt\n" "build out1: interrupt in1\n" "build out2: touch-interrupt in2\n"))

  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Tick()
  fs_.Create("in1", "")
  fs_.Create("in2", "")

  // An untouched output of an interrupted command should be retained.
  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !builder_.Build(&err) { t.FailNow() }
  if "interrupted by user" != err { t.FailNow() }
  builder_.Cleanup()
  if fs_.Stat("out1" <= &err), 0 { t.FailNow() }
  err = ""

  // A touched output of an interrupted command should be deleted.
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !builder_.Build(&err) { t.FailNow() }
  if "interrupted by user" != err { t.FailNow() }
  builder_.Cleanup()
  if 0 != fs_.Stat("out2", &err) { t.FailNow() }
}

func TestBuildTest_StatFailureAbortsBuild(t *testing.T) {
  const string kTooLongToStat(400, 'i')
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, ("build " + kTooLongToStat + ": cat in\n")))
  fs_.Create("in", "")

  // This simulates a stat failure:
  fs_.files_[kTooLongToStat].mtime = -1
  fs_.files_[kTooLongToStat].stat_error = "stat failed"

  string err
  if !builder_.AddTarget(kTooLongToStat, &err) { t.FailNow() }
  if "stat failed" != err { t.FailNow() }
}

func TestBuildTest_PhonyWithNoInputs(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build nonexistent: phony\n" "build out1: cat || nonexistent\n" "build out2: cat nonexistent\n"))
  fs_.Create("out1", "")
  fs_.Create("out2", "")

  // out1 should be up to date even though its input is dirty, because its
  // order-only dependency has nothing to do.
  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }

  // out2 should still be out of date though, because its input is dirty.
  err = nil
  command_runner_.commands_ran_ = nil
  state_.Reset()
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_DepsGccWithEmptyDepfileErrorsOut(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cc\n" "  command = cc\n" "  deps = gcc\n" "build out: cc\n"))
  Dirty("out")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if !builder_.AlreadyUpToDate() { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if "subcommand failed" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_StatusFormatElapsed(t *testing.T) {
  status_.BuildStarted()
  // Before any task is done, the elapsed time must be zero.
  if "[%/e0.000]" != status_.FormatProgressStatus("[%%/e%e]", 0) { t.FailNow() }
}

func TestBuildTest_StatusFormatReplacePlaceholder(t *testing.T) {
  if "[%/s0/t0/r0/u0/f0]" != status_.FormatProgressStatus("[%%/s%s/t%t/r%r/u%u/f%f]", 0) { t.FailNow() }
}

func TestBuildTest_FailedDepsParse(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build bad_deps.o: cat in1\n" "  deps = gcc\n" "  depfile = in1.d\n"))

  string err
  if builder_.AddTarget("bad_deps.o", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // These deps will fail to parse, as they should only have one
  // path to the left of the colon.
  fs_.Create("in1.d", "AAA BBB")

  if !builder_.Build(&err) { t.FailNow() }
  if "subcommand failed" != err { t.FailNow() }
}

type BuildWithQueryDepsLogTest struct {
  BuildWithQueryDepsLogTest() : BuildTest(&log_) {
  }

  ~BuildWithQueryDepsLogTest() {
    log_.Close()
  }

  func (b *BuildWithQueryDepsLogTest) SetUp() {
    BuildTest::SetUp()

    temp_dir_.CreateAndEnter("BuildWithQueryDepsLogTest")

    string err
    if log_.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
  }

  ScopedTempDir temp_dir_

  DepsLog log_
}

// Test a MSVC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileMSVC(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_msvc\n" "    command = echo 'using $in' && for file in $out; do cp $in $$file; done\n" "    deps = msvc\n" "    msvc_deps_prefix = using \n" "build out1 out2: cp_multi_msvc in1\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'using in1' && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 1 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 1 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
}

// Test a GCC-style deps log with multiple outputs.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOneLine(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_gcc\n" "    command = echo '$out: $in' > in.d && for file in $out; do cp in1 $$file; done\n" "    deps = gcc\n" "    depfile = in.d\n" "build out1 out2: cp_multi_gcc in1 in2\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  fs_.Create("in.d", "out1 out2: in1 in2")
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'out1 out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 2 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out1_deps.nodes[1].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 2 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out2_deps.nodes[1].path() { t.FailNow() }
}

// Test a GCC-style deps log with multiple outputs using a line per input.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineInput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_gcc\n" "    command = echo '$out: in1\\n$out: in2' > in.d && for file in $out; do cp in1 $$file; done\n" "    deps = gcc\n" "    depfile = in.d\n" "build out1 out2: cp_multi_gcc in1 in2\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  fs_.Create("in.d", "out1 out2: in1\nout1 out2: in2")
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'out1 out2: in1\\nout1 out2: in2' > in.d && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 2 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out1_deps.nodes[1].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 2 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out2_deps.nodes[1].path() { t.FailNow() }
}

// Test a GCC-style deps log with multiple outputs using a line per output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCMultiLineOutput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_gcc\n" "    command = echo 'out1: $in\\nout2: $in' > in.d && for file in $out; do cp in1 $$file; done\n" "    deps = gcc\n" "    depfile = in.d\n" "build out1 out2: cp_multi_gcc in1 in2\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  fs_.Create("in.d", "out1: in1 in2\nout2: in1 in2")
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'out1: in1 in2\\nout2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 2 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out1_deps.nodes[1].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 2 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out2_deps.nodes[1].path() { t.FailNow() }
}

// Test a GCC-style deps log with multiple outputs mentioning only the main output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlyMainOutput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_gcc\n" "    command = echo 'out1: $in' > in.d && for file in $out; do cp in1 $$file; done\n" "    deps = gcc\n" "    depfile = in.d\n" "build out1 out2: cp_multi_gcc in1 in2\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  fs_.Create("in.d", "out1: in1 in2")
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'out1: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 2 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out1_deps.nodes[1].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 2 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out2_deps.nodes[1].path() { t.FailNow() }
}

// Test a GCC-style deps log with multiple outputs mentioning only the secondary output.
func TestBuildWithQueryDepsLogTest_TwoOutputsDepFileGCCOnlySecondaryOutput(t *testing.T) {
  // Note: This ends up short-circuiting the node creation due to the primary
  // output not being present, but it should still work.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule cp_multi_gcc\n" "    command = echo 'out2: $in' > in.d && for file in $out; do cp in1 $$file; done\n" "    deps = gcc\n" "    depfile = in.d\n" "build out1 out2: cp_multi_gcc in1 in2\n"))

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  fs_.Create("in.d", "out2: in1 in2")
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "echo 'out2: in1 in2' > in.d && for file in out1 out2; do cp in1 $file; done" != command_runner_.commands_ran_[0] { t.FailNow() }

  Node* out1_node = state_.LookupNode("out1")
  DepsLog::Deps* out1_deps = log_.GetDeps(out1_node)
  if 2 != out1_deps.node_count { t.FailNow() }
  if "in1" != out1_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out1_deps.nodes[1].path() { t.FailNow() }

  Node* out2_node = state_.LookupNode("out2")
  DepsLog::Deps* out2_deps = log_.GetDeps(out2_node)
  if 2 != out2_deps.node_count { t.FailNow() }
  if "in1" != out2_deps.nodes[0].path() { t.FailNow() }
  if "in2" != out2_deps.nodes[1].path() { t.FailNow() }
}

// Tests of builds involving deps logs necessarily must span
// multiple builds.  We reuse methods on BuildTest but not the
// builder_ it sets up, because we want pristine objects for
// each build.
type BuildWithDepsLogTest struct {
  BuildWithDepsLogTest() {}

  func (b *BuildWithDepsLogTest) SetUp() {
    BuildTest::SetUp()

    temp_dir_.CreateAndEnter("BuildWithDepsLogTest")
  }

  func (b *BuildWithDepsLogTest) TearDown() {
    temp_dir_.Cleanup()
  }

  ScopedTempDir temp_dir_

  // Shadow parent class builder_ so we don't accidentally use it.
  void* builder_
}

// Run a straightforwad build where the deps log is used.
func TestBuildWithDepsLogTest_Straightforward(t *testing.T) {
  string err
  // Note: in1 was created by the superclass SetUp().
  string manifest =
      "build out: cat in1\n"
      "  deps = gcc\n"
      "  depfile = in1.d\n"
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Run the build once, everything should be ok.
    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    fs_.Create("in1.d", "out: in2")
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // The deps file should have been removed.
    if 0 != fs_.Stat("in1.d", &err) { t.FailNow() }
    // Recreate it for the next step.
    fs_.Create("in1.d", "out: in2")
    deps_log.Close()
    builder.command_runner_.release()
  }

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Touch the file only mentioned in the deps.
    fs_.Tick()
    fs_.Create("in2", "")

    // Run the build again.
    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    command_runner_.commands_ran_ = nil
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // We should have rebuilt the output due to in2 being
    // out of date.
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

    builder.command_runner_.release()
  }
}

// Verify that obsolete dependency info causes a rebuild.
// 1) Run a successful build where everything has time t, record deps.
// 2) Move input/output to time t+1 -- despite files in alignment,
//    should still need to rebuild due to deps at older time.
func TestBuildWithDepsLogTest_ObsoleteDeps(t *testing.T) {
  string err
  // Note: in1 was created by the superclass SetUp().
  string manifest =
      "build out: cat in1\n"
      "  deps = gcc\n"
      "  depfile = in1.d\n"
  {
    // Run an ordinary build that gathers dependencies.
    fs_.Create("in1", "")
    fs_.Create("in1.d", "out: ")

    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Run the build once, everything should be ok.
    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  // Push all files one tick forward so that only the deps are out
  // of date.
  fs_.Tick()
  fs_.Create("in1", "")
  fs_.Create("out", "")

  // The deps file should have been removed, so no need to timestamp it.
  if 0 != fs_.Stat("in1.d", &err) { t.FailNow() }

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    command_runner_.commands_ran_ = nil
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Recreate the deps file here because the build expects them to exist.
    fs_.Create("in1.d", "out: ")

    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // We should have rebuilt the output due to the deps being
    // out of date.
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

    builder.command_runner_.release()
  }
}

func TestBuildWithDepsLogTest_DepsIgnoredInDryRun(t *testing.T) {
  string manifest =
      "build out: cat in1\n"
      "  deps = gcc\n"
      "  depfile = in1.d\n"

  fs_.Create("out", "")
  fs_.Tick()
  fs_.Create("in1", "")

  State state
  ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

  // The deps log is NULL in dry runs.
  config_.dry_run = true
  Builder builder(&state, config_, nil, nil, &fs_, &status_, 0)
  builder.command_runner_.reset(&command_runner_)
  command_runner_.commands_ran_ = nil

  string err
  if builder.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

  builder.command_runner_.release()
}

// Check that a restat rule generating a header cancels compilations correctly.
func TestBuildTest_RestatDepfileDependency(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "  restat = 1\n" "build header.h: true header.in\n" "build out: cat in1\n" "  depfile = in1.d\n"))  // Would be "write if out-of-date" in reality.

  fs_.Create("header.h", "")
  fs_.Create("in1.d", "out: header.h")
  fs_.Tick()
  fs_.Create("header.in", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
}

// Check that a restat rule generating a header cancels compilations correctly,
// depslog case.
func TestBuildWithDepsLogTest_RestatDepfileDependencyDepsLog(t *testing.T) {
  string err
  // Note: in1 was created by the superclass SetUp().
  string manifest =
      "rule true\n"
      "  command = true\n"  // Would be "write if out-of-date" in reality.
      "  restat = 1\n"
      "build header.h: true header.in\n"
      "build out: cat in1\n"
      "  deps = gcc\n"
      "  depfile = in1.d\n"
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Run the build once, everything should be ok.
    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    fs_.Create("in1.d", "out: header.h")
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AddCatRule(&state))
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Touch the input of the restat rule.
    fs_.Tick()
    fs_.Create("header.in", "")

    // Run the build again.
    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    command_runner_.commands_ran_ = nil
    if builder.AddTarget("out", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Rule "true" should have run again, but the build of "out" should have
    // been cancelled due to restat propagating through the depfile header.
    if 1u != command_runner_.commands_ran_.size() { t.FailNow() }

    builder.command_runner_.release()
  }
}

func TestBuildWithDepsLogTest_DepFileOKDepsLog(t *testing.T) {
  string err
  string manifest =
      "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\n"
      "build fo$ o.o: cc foo.c\n"

  fs_.Create("foo.c", "")

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Run the build once, everything should be ok.
    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("fo o.o", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    fs_.Create("fo o.o.d", "fo\\ o.o: blah.h bar.h\n")
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)

    edge := state.edges_.back()

    state.GetNode("bar.h", 0).MarkDirty()  // Mark bar.h as missing.
    if builder.AddTarget("fo o.o", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Expect three new edges: one generating fo o.o, and two more from
    // loading the depfile.
    if 3u != state.edges_.size() { t.FailNow() }
    // Expect our edge to now have three inputs: foo.c and two headers.
    if 3u != edge.inputs_.size() { t.FailNow() }

    // Expect the command line we generate to only use the original input.
    if "cc foo.c" != edge.EvaluateCommand() { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }
}

func TestBuildWithDepsLogTest_DiscoveredDepDuringBuildChanged(t *testing.T) {
  string err
  string manifest =
    "rule touch-out-implicit-dep\n"
    "  command = touch $out ; sleep 1 ; touch $test_dependency\n"
    "rule generate-depfile\n"
    "  command = touch $out ; echo \"$out: $test_dependency\" > $depfile\n"
    "build out1: touch-out-implicit-dep in1\n"
    "  test_dependency = inimp\n"
    "build out2: generate-depfile in1 || out1\n"
    "  test_dependency = inimp\n"
    "  depfile = out2.d\n"
    "  deps = gcc\n"

  fs_.Create("in1", "")
  fs_.Tick()

  BuildLog build_log

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, &build_log, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out2", &err) { t.FailNow() }
    if !builder.AlreadyUpToDate() { t.FailNow() }

    if builder.Build(&err) { t.FailNow() }
    if builder.AlreadyUpToDate() { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  fs_.Tick()
  fs_.Create("in1", "")

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, &build_log, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out2", &err) { t.FailNow() }
    if !builder.AlreadyUpToDate() { t.FailNow() }

    if builder.Build(&err) { t.FailNow() }
    if builder.AlreadyUpToDate() { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  fs_.Tick()

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, &build_log, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("out2", &err) { t.FailNow() }
    if builder.AlreadyUpToDate() { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }
}

func TestBuildWithDepsLogTest_DepFileDepsLogCanonicalize(t *testing.T) {
  string err
  string manifest =
      "rule cc\n  command = cc $in\n  depfile = $out.d\n  deps = gcc\n"
      "build a/b\\c\\d/e/fo$ o.o: cc x\\y/z\\foo.c\n"

  fs_.Create("x/y/z/foo.c", "")

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    // Run the build once, everything should be ok.
    DepsLog deps_log
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)
    if builder.AddTarget("a/b/c/d/e/fo o.o", &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    // Note, different slashes from manifest.
    fs_.Create("a/b\\c\\d/e/fo o.o.d", "a\\b\\c\\d\\e\\fo\\ o.o: blah.h bar.h\n")
    if builder.Build(&err) { t.FailNow() }
    if "" != err { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }

  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, manifest))

    DepsLog deps_log
    if deps_log.Load("ninja_deps", &state, &err) { t.FailNow() }
    if deps_log.OpenForWrite("ninja_deps", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    Builder builder(&state, config_, nil, &deps_log, &fs_, &status_, 0)
    builder.command_runner_.reset(&command_runner_)

    edge := state.edges_.back()

    state.GetNode("bar.h", 0).MarkDirty()  // Mark bar.h as missing.
    if builder.AddTarget("a/b/c/d/e/fo o.o", &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Expect three new edges: one generating fo o.o, and two more from
    // loading the depfile.
    if 3u != state.edges_.size() { t.FailNow() }
    // Expect our edge to now have three inputs: foo.c and two headers.
    if 3u != edge.inputs_.size() { t.FailNow() }

    // Expect the command line we generate to only use the original input.
    // Note, slashes from manifest, not .d.
    if "cc x\\y/z\\foo.c" != edge.EvaluateCommand() { t.FailNow() }

    deps_log.Close()
    builder.command_runner_.release()
  }
}

// Check that a restat rule doesn't clear an edge if the depfile is missing.
// Follows from: https://github.com/ninja-build/ninja/issues/603
func TestBuildTest_RestatMissingDepfile(t *testing.T) {
string manifest =
"rule true\n"
"  command = true\n"  // Would be "write if out-of-date" in reality.
"  restat = 1\n"
"build header.h: true header.in\n"
"build out: cat header.h\n"
"  depfile = out.d\n"

  fs_.Create("header.h", "")
  fs_.Tick()
  fs_.Create("out", "")
  fs_.Create("header.in", "")

  // Normally, only 'header.h' would be rebuilt, as
  // its rule doesn't touch the output and has 'restat=1' set.
  // But we are also missing the depfile for 'out',
  // which should force its command to run anyway!
  RebuildTarget("out", manifest)
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
}

// Check that a restat rule doesn't clear an edge if the deps are missing.
// https://github.com/ninja-build/ninja/issues/603
func TestBuildWithDepsLogTest_RestatMissingDepfileDepslog(t *testing.T) {
  string err
  string manifest =
"rule true\n"
"  command = true\n"  // Would be "write if out-of-date" in reality.
"  restat = 1\n"
"build header.h: true header.in\n"
"build out: cat header.h\n"
"  deps = gcc\n"
"  depfile = out.d\n"

  // Build once to populate ninja deps logs from out.d
  fs_.Create("header.in", "")
  fs_.Create("out.d", "out: header.h")
  fs_.Create("header.h", "")

  RebuildTarget("out", manifest, "build_log", "ninja_deps")
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // Sanity: this rebuild should be NOOP
  RebuildTarget("out", manifest, "build_log", "ninja_deps")
  if 0u != command_runner_.commands_ran_.size() { t.FailNow() }

  // Touch 'header.in', blank dependencies log (create a different one).
  // Building header.h triggers 'restat' outputs cleanup.
  // Validate that out is rebuilt netherless, as deps are missing.
  fs_.Tick()
  fs_.Create("header.in", "")

  // (switch to a new blank deps_log "ninja_deps2")
  RebuildTarget("out", manifest, "build_log", "ninja_deps2")
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // Sanity: this build should be NOOP
  RebuildTarget("out", manifest, "build_log", "ninja_deps2")
  if 0u != command_runner_.commands_ran_.size() { t.FailNow() }

  // Check that invalidating deps by target timestamp also works here
  // Repeat the test but touch target instead of blanking the log.
  fs_.Tick()
  fs_.Create("header.in", "")
  fs_.Create("out", "")
  RebuildTarget("out", manifest, "build_log", "ninja_deps2")
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }

  // And this build should be NOOP again
  RebuildTarget("out", manifest, "build_log", "ninja_deps2")
  if 0u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_WrongOutputInDepfileCausesRebuild(t *testing.T) {
  string err
  string manifest =
"rule cc\n"
"  command = cc $in\n"
"  depfile = $out.d\n"
"build foo.o: cc foo.c\n"

  fs_.Create("foo.c", "")
  fs_.Create("foo.o", "")
  fs_.Create("header.h", "")
  fs_.Create("foo.o.d", "bar.o.d: header.h\n")

  RebuildTarget("foo.o", manifest, "build_log", "ninja_deps")
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_Console(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule console\n" "  command = console\n" "  pool = console\n" "build cons: console in.txt\n"))

  fs_.Create("in.txt", "")

  string err
  if builder_.AddTarget("cons", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
}

func TestBuildTest_DyndepMissingAndNoRule(t *testing.T) {
  // Verify that we can diagnose when a dyndep file is missing and
  // has no rule to build it.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build out: touch || dd\n" "  dyndep = dd\n" ))

  string err
  if !builder_.AddTarget("out", &err) { t.FailNow() }
  if "loading 'dd': No such file or directory" != err { t.FailNow() }
}

func TestBuildTest_DyndepReadyImplicitConnection(t *testing.T) {
  // Verify that a dyndep file can be loaded immediately to discover
  // that one edge has an implicit output that is also an implicit
  // input of another edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "build tmp: touch || dd\n" "  dyndep = dd\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd", "ninja_dyndep_version = 1\n" "build out | out.imp: dyndep | tmp.imp\n" "build tmp | tmp.imp: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "touch tmp tmp.imp" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
}

func TestBuildTest_DyndepReadySyntaxError(t *testing.T) {
  // Verify that a dyndep file can be loaded immediately to discover
  // and reject a syntax error in it.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd", "build out: dyndep\n" )

  string err
  if !builder_.AddTarget("out", &err) { t.FailNow() }
  if "dd:1: expected 'ninja_dyndep_version = ...'\n" != err { t.FailNow() }
}

func TestBuildTest_DyndepReadyCircular(t *testing.T) {
  // Verify that a dyndep file can be loaded immediately to discover
  // and reject a circular dependency.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule r\n" "  command = unused\n" "build out: r in || dd\n" "  dyndep = dd\n" "build in: r circ\n" ))
  fs_.Create("dd", "ninja_dyndep_version = 1\n" "build out | circ: dyndep\n" )
  fs_.Create("out", "")

  string err
  if !builder_.AddTarget("out", &err) { t.FailNow() }
  if "dependency cycle: circ . in . circ" != err { t.FailNow() }
}

func TestBuildTest_DyndepBuild(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover nothing.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  size_t files_created = fs_.files_created_.size()
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch out" != command_runner_.commands_ran_[1] { t.FailNow() }
  if 2u != fs_.files_read_.size() { t.FailNow() }
  if "dd-in" != fs_.files_read_[0] { t.FailNow() }
  if "dd" != fs_.files_read_[1] { t.FailNow() }
  if 2u + files_created != fs_.files_created_.size() { t.FailNow() }
  if 1u != fs_.files_created_.count("dd") { t.FailNow() }
  if 1u != fs_.files_created_.count("out") { t.FailNow() }
}

func TestBuildTest_DyndepBuildSyntaxError(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // and reject a syntax error in it.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd-in", "build out: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if "dd:1: expected 'ninja_dyndep_version = ...'\n" != err { t.FailNow() }
}

func TestBuildTest_DyndepBuildUnrelatedOutput(t *testing.T) {
  // Verify that a dyndep file can have dependents that do not specify
  // it as their dyndep binding.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build unrelated: touch || dd\n" "build out: touch unrelated || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out: dyndep\n" )
  fs_.Tick()
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch unrelated" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverNewOutput(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // a new output of an edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out: touch in || dd\n" "  dyndep = dd\n" ))
  fs_.Create("in", "")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out | out.imp: dyndep\n" )
  fs_.Tick()
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
}

TEST_F(BuildTest, DyndepBuildDiscoverNewOutputWithMultipleRules1) {
  // Verify that a dyndep file can be built and loaded to discover
  // a new output of an edge that is already the output of another edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out1 | out-twice.imp: touch in\n" "build out2: touch in || dd\n" "  dyndep = dd\n" ))
  fs_.Create("in", "")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out2 | out-twice.imp: dyndep\n" )
  fs_.Tick()
  fs_.Create("out1", "")
  fs_.Create("out2", "")

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if "multiple rules generate out-twice.imp" != err { t.FailNow() }
}

TEST_F(BuildTest, DyndepBuildDiscoverNewOutputWithMultipleRules2) {
  // Verify that a dyndep file can be built and loaded to discover
  // a new output of an edge that is already the output of another
  // edge also discovered by dyndep.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd1: cp dd1-in\n" "build out1: touch || dd1\n" "  dyndep = dd1\n" "build dd2: cp dd2-in || dd1\n" "build out2: touch || dd2\n" "  dyndep = dd2\n" )) // make order predictable for test
  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Create("dd1-in", "ninja_dyndep_version = 1\n" "build out1 | out-twice.imp: dyndep\n" )
  fs_.Create("dd2-in", "")
  fs_.Create("dd2", "ninja_dyndep_version = 1\n" "build out2 | out-twice.imp: dyndep\n" )
  fs_.Tick()
  fs_.Create("out1", "")
  fs_.Create("out2", "")

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  if "multiple rules generate out-twice.imp" != err { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverNewInput(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // a new input to an edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build in: touch\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out: dyndep | in\n" )
  fs_.Tick()
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch in" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverImplicitConnection(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // that one edge has an implicit output that is also an implicit
  // input of another edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build tmp: touch || dd\n" "  dyndep = dd\n" "build out: touch || dd\n" "  dyndep = dd\n" ))
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out | out.imp: dyndep | tmp.imp\n" "build tmp | tmp.imp: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch tmp tmp.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverOutputAndDepfileInput(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // that one edge has an implicit output that is also reported by
  // a depfile as an input of another edge.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build tmp: touch || dd\n" "  dyndep = dd\n" "build out: cp tmp\n" "  depfile = out.d\n" ))
  fs_.Create("out.d", "out: tmp.imp\n")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build tmp | tmp.imp: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // Loading the depfile gave tmp.imp a phony input edge.
  if GetNode("tmp.imp").in_edge().is_phony() { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // Loading the dyndep file gave tmp.imp a real input edge.
  if !GetNode("tmp.imp").in_edge().is_phony() { t.FailNow() }

  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch tmp tmp.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "cp tmp out" != command_runner_.commands_ran_[2] { t.FailNow() }
  if 1u != fs_.files_created_.count("tmp.imp") { t.FailNow() }
  if builder_.AlreadyUpToDate() { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdge(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // that an edge is actually wanted due to a missing implicit output.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build tmp: touch || dd\n" "  dyndep = dd\n" "build out: touch tmp || dd\n" "  dyndep = dd\n" ))
  fs_.Create("tmp", "")
  fs_.Create("out", "")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out: dyndep\n" "build tmp | tmp.imp: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch tmp tmp.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverNowWantEdgeAndDependent(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // that an edge and a dependent are actually wanted.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build tmp: touch || dd\n" "  dyndep = dd\n" "build out: touch tmp\n" ))
  fs_.Create("tmp", "")
  fs_.Create("out", "")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build tmp | tmp.imp: dyndep\n" )

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch tmp tmp.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out out.imp" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverCircular(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // and reject a circular dependency.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule r\n" "  command = unused\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out: r in || dd\n" "  depfile = out.d\n" "  dyndep = dd\n" "build in: r || dd\n" "  dyndep = dd\n" ))
  fs_.Create("out.d", "out: inimp\n")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out | circ: dyndep\n" "build in: dyndep | circ\n" )
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if !builder_.Build(&err) { t.FailNow() }
  // Depending on how the pointers in Plan::ready_ work out, we could have
  // discovered the cycle from either starting point.
  if err == "dependency cycle: circ . in . circ" || err == "dependency cycle: in . circ . in" { t.FailNow() }
}

func TestBuildWithLogTest_DyndepBuildDiscoverRestat(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover
  // that an edge has a restat binding.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule true\n" "  command = true\n" "rule cp\n" "  command = cp $in $out\n" "build dd: cp dd-in\n" "build out1: true in || dd\n" "  dyndep = dd\n" "build out2: cat out1\n"))

  fs_.Create("out1", "")
  fs_.Create("out2", "")
  fs_.Create("dd-in", "ninja_dyndep_version = 1\n" "build out1: dyndep\n" "  restat = 1\n" )
  fs_.Tick()
  fs_.Create("in", "")

  // Do a pre-build so that there's commands in the log for the outputs,
  // otherwise, the lack of an entry in the build log will cause "out2" to
  // rebuild regardless of restat.
  string err
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd-in dd" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "true" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "cat out1 > out2" != command_runner_.commands_ran_[2] { t.FailNow() }

  command_runner_.commands_ran_ = nil
  state_.Reset()
  fs_.Tick()
  fs_.Create("in", "")

  // We touched "in", so we should build "out1".  But because "true" does not
  // touch "out1", we should cancel the build of "out2".
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if 1u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "true" != command_runner_.commands_ran_[0] { t.FailNow() }
}

func TestBuildTest_DyndepBuildDiscoverScheduledEdge(t *testing.T) {
  // Verify that a dyndep file can be built and loaded to discover a
  // new input that itself is an output from an edge that has already
  // been scheduled but not finished.  We should not re-schedule it.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build out1 | out1.imp: touch\n" "build zdd: cp zdd-in\n" "  verify_active_edge = out1\n" "build out2: cp out1 || zdd\n" "  dyndep = zdd\n" )) // verify out1 is active when zdd is finished
  fs_.Create("zdd-in", "ninja_dyndep_version = 1\n" "build out2: dyndep | out1.imp\n" )

  // Enable concurrent builds so that we can load the dyndep file
  // while another edge is still active.
  command_runner_.max_active_edges_ = 2

  // During the build "out1" and "zdd" should be built concurrently.
  // The fake command runner will finish these in reverse order
  // of the names of the first outputs, so "zdd" will finish first
  // and we will load the dyndep file while the edge for "out1" is
  // still active.  This will add a new dependency on "out1.imp",
  // also produced by the active edge.  The builder should not
  // re-schedule the already-active edge.

  string err
  if builder_.AddTarget("out1", &err) { t.FailNow() }
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  // Depending on how the pointers in Plan::ready_ work out, the first
  // two commands may have run in either order.
  if (command_runner_.commands_ran_[0] == "touch out1 out1.imp" && command_runner_.commands_ran_[1] == "cp zdd-in zdd") || (command_runner_.commands_ran_[1] == "touch out1 out1.imp" && command_runner_.commands_ran_[0] == "cp zdd-in zdd") { t.FailNow() }
  if "cp out1 out2" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepTwoLevelDirect(t *testing.T) {
  // Verify that a clean dyndep file can depend on a dirty dyndep file
  // and be loaded properly after the dirty one is built and loaded.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd1: cp dd1-in\n" "build out1 | out1.imp: touch || dd1\n" "  dyndep = dd1\n" "build dd2: cp dd2-in || dd1\n" "build out2: touch || dd2\n" "  dyndep = dd2\n" )) // direct order-only dep on dd1
  fs_.Create("out1.imp", "")
  fs_.Create("out2", "")
  fs_.Create("out2.imp", "")
  fs_.Create("dd1-in", "ninja_dyndep_version = 1\n" "build out1: dyndep\n" )
  fs_.Create("dd2-in", "")
  fs_.Create("dd2", "ninja_dyndep_version = 1\n" "build out2 | out2.imp: dyndep | out1.imp\n" )

  // During the build dd1 should be built and loaded.  The RecomputeDirty
  // called as a result of loading dd1 should not cause dd2 to be loaded
  // because the builder will never get a chance to update the build plan
  // to account for dd2.  Instead dd2 should only be later loaded once the
  // builder recognizes that it is now ready (as its order-only dependency
  // on dd1 has been satisfied).  This test case verifies that each dyndep
  // file is loaded to update the build graph independently.

  string err
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd1-in dd1" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch out1 out1.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out2 out2.imp" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepTwoLevelIndirect(t *testing.T) {
  // Verify that dyndep files can add to an edge new implicit inputs that
  // correspond to implicit outputs added to other edges by other dyndep
  // files on which they (order-only) depend.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out $out.imp\n" "rule cp\n" "  command = cp $in $out\n" "build dd1: cp dd1-in\n" "build out1: touch || dd1\n" "  dyndep = dd1\n" "build dd2: cp dd2-in || out1\n" "build out2: touch || dd2\n" "  dyndep = dd2\n" )) // indirect order-only dep on dd1
  fs_.Create("out1.imp", "")
  fs_.Create("out2", "")
  fs_.Create("out2.imp", "")
  fs_.Create("dd1-in", "ninja_dyndep_version = 1\n" "build out1 | out1.imp: dyndep\n" )
  fs_.Create("dd2-in", "")
  fs_.Create("dd2", "ninja_dyndep_version = 1\n" "build out2 | out2.imp: dyndep | out1.imp\n" )

  // During the build dd1 should be built and loaded.  Then dd2 should
  // be built and loaded.  Loading dd2 should cause the builder to
  // recognize that out2 needs to be built even though it was originally
  // clean without dyndep info.

  string err
  if builder_.AddTarget("out2", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 3u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd1-in dd1" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch out1 out1.imp" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch out2 out2.imp" != command_runner_.commands_ran_[2] { t.FailNow() }
}

func TestBuildTest_DyndepTwoLevelDiscoveredReady(t *testing.T) {
  // Verify that a dyndep file can discover a new input whose
  // edge also has a dyndep file that is ready to load immediately.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd0: cp dd0-in\n" "build dd1: cp dd1-in\n" "build in: touch\n" "build tmp: touch || dd0\n" "  dyndep = dd0\n" "build out: touch || dd1\n" "  dyndep = dd1\n" ))
  fs_.Create("dd1-in", "ninja_dyndep_version = 1\n" "build out: dyndep | tmp\n" )
  fs_.Create("dd0-in", "")
  fs_.Create("dd0", "ninja_dyndep_version = 1\n" "build tmp: dyndep | in\n" )
  fs_.Tick()
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 4u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd1-in dd1" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "touch in" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch tmp" != command_runner_.commands_ran_[2] { t.FailNow() }
  if "touch out" != command_runner_.commands_ran_[3] { t.FailNow() }
}

func TestBuildTest_DyndepTwoLevelDiscoveredDirty(t *testing.T) {
  // Verify that a dyndep file can discover a new input whose
  // edge also has a dyndep file that needs to be built.
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "rule cp\n" "  command = cp $in $out\n" "build dd0: cp dd0-in\n" "build dd1: cp dd1-in\n" "build in: touch\n" "build tmp: touch || dd0\n" "  dyndep = dd0\n" "build out: touch || dd1\n" "  dyndep = dd1\n" ))
  fs_.Create("dd1-in", "ninja_dyndep_version = 1\n" "build out: dyndep | tmp\n" )
  fs_.Create("dd0-in", "ninja_dyndep_version = 1\n" "build tmp: dyndep | in\n" )
  fs_.Tick()
  fs_.Create("out", "")

  string err
  if builder_.AddTarget("out", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if builder_.Build(&err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 5u != command_runner_.commands_ran_.size() { t.FailNow() }
  if "cp dd1-in dd1" != command_runner_.commands_ran_[0] { t.FailNow() }
  if "cp dd0-in dd0" != command_runner_.commands_ran_[1] { t.FailNow() }
  if "touch in" != command_runner_.commands_ran_[2] { t.FailNow() }
  if "touch tmp" != command_runner_.commands_ran_[3] { t.FailNow() }
  if "touch out" != command_runner_.commands_ran_[4] { t.FailNow() }
}

