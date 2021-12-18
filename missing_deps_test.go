// Copyright 2019 Google Inc. All Rights Reserved.
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


const char kTestDepsLogFilename[] = "MissingDepTest-tempdepslog"

class MissingDependencyTestDelegate : public MissingDependencyScannerDelegate {
  func OnMissingDep(node *Node, path string, generator *Rule) {}
}

type MissingDependencyScannerTest struct {
  MissingDependencyScannerTest()
      : generator_rule_("generator_rule"), compile_rule_("compile_rule"),
        scanner_(&delegate_, &deps_log_, &state_, &filesystem_) {
    err string
    deps_log_.OpenForWrite(kTestDepsLogFilename, &err)
    ASSERT_EQ("", err)
  }

  delegate_ MissingDependencyTestDelegate
  generator_rule_ Rule
  compile_rule_ Rule
  deps_log_ DepsLog
  state_ State
  filesystem_ VirtualFileSystem
  scanner_ MissingDependencyScanner
}
func (m *MissingDependencyScannerTest) scanner() *MissingDependencyScanner {
	return scanner_
}
func (m *MissingDependencyScannerTest) RecordDepsLogDep(from string, to string) {
  Node* node_deps[] = { state_.LookupNode(to) }
  deps_log_.RecordDeps(state_.LookupNode(from), 0, 1, node_deps)
}
func (m *MissingDependencyScannerTest) ProcessAllNodes() {
  err := ""
  nodes := state_.RootNodes(&err)
  if "" != err { t.FailNow() }
  for it := nodes.begin(); it != nodes.end(); it++ {
    scanner().ProcessNode(*it)
  }
}
func (m *MissingDependencyScannerTest) CreateInitialState() {
  var deps_type EvalString
  deps_type.AddText("gcc")
  compile_rule_.AddBinding("deps", deps_type)
  generator_rule_.AddBinding("deps", deps_type)
  header_edge := state_.AddEdge(&generator_rule_)
  state_.AddOut(header_edge, "generated_header", 0)
  compile_edge := state_.AddEdge(&compile_rule_)
  state_.AddOut(compile_edge, "compiled_object", 0)
}
func (m *MissingDependencyScannerTest) CreateGraphDependencyBetween(from string, to string) {
  from_node := state_.LookupNode(from)
  from_edge := from_node.in_edge()
  state_.AddIn(from_edge, to, 0)
}
func (m *MissingDependencyScannerTest) AssertMissingDependencyBetween(flaky string, generated string, rule *Rule) {
  flaky_node := state_.LookupNode(flaky)
  if 1u != scanner().nodes_missing_deps_.count(flaky_node) { t.FailNow() }
  generated_node := state_.LookupNode(generated)
  if 1u != scanner().generated_nodes_.count(generated_node) { t.FailNow() }
  if 1u != scanner().generator_rules_.count(rule) { t.FailNow() }
}

func TestMissingDependencyScannerTest_EmptyGraph(t *testing.T) {
  ProcessAllNodes()
  if !scanner().HadMissingDeps() { t.FailNow() }
}

func TestMissingDependencyScannerTest_NoMissingDep(t *testing.T) {
  CreateInitialState()
  ProcessAllNodes()
  if !scanner().HadMissingDeps() { t.FailNow() }
}

func TestMissingDependencyScannerTest_MissingDepPresent(t *testing.T) {
  CreateInitialState()
  // compiled_object uses generated_header, without a proper dependency
  RecordDepsLogDep("compiled_object", "generated_header")
  ProcessAllNodes()
  if scanner().HadMissingDeps() { t.FailNow() }
  if 1u != scanner().nodes_missing_deps_.size() { t.FailNow() }
  if 1u != scanner().missing_dep_path_count_ { t.FailNow() }
  AssertMissingDependencyBetween("compiled_object", "generated_header", &generator_rule_)
}

func TestMissingDependencyScannerTest_MissingDepFixedDirect(t *testing.T) {
  CreateInitialState()
  // Adding the direct dependency fixes the missing dep
  CreateGraphDependencyBetween("compiled_object", "generated_header")
  RecordDepsLogDep("compiled_object", "generated_header")
  ProcessAllNodes()
  if !scanner().HadMissingDeps() { t.FailNow() }
}

func TestMissingDependencyScannerTest_MissingDepFixedIndirect(t *testing.T) {
  CreateInitialState()
  // Adding an indirect dependency also fixes the issue
  intermediate_edge := state_.AddEdge(&generator_rule_)
  state_.AddOut(intermediate_edge, "intermediate", 0)
  CreateGraphDependencyBetween("compiled_object", "intermediate")
  CreateGraphDependencyBetween("intermediate", "generated_header")
  RecordDepsLogDep("compiled_object", "generated_header")
  ProcessAllNodes()
  if !scanner().HadMissingDeps() { t.FailNow() }
}

func TestMissingDependencyScannerTest_CyclicMissingDep(t *testing.T) {
  CreateInitialState()
  RecordDepsLogDep("generated_header", "compiled_object")
  RecordDepsLogDep("compiled_object", "generated_header")
  // In case of a cycle, both paths are reported (and there is
  // no way to fix the issue by adding deps).
  ProcessAllNodes()
  if scanner().HadMissingDeps() { t.FailNow() }
  if 2u != scanner().nodes_missing_deps_.size() { t.FailNow() }
  if 2u != scanner().missing_dep_path_count_ { t.FailNow() }
  AssertMissingDependencyBetween("compiled_object", "generated_header", &generator_rule_)
  AssertMissingDependencyBetween("generated_header", "compiled_object", &compile_rule_)
}

func TestMissingDependencyScannerTest_CycleInGraph(t *testing.T) {
  CreateInitialState()
  CreateGraphDependencyBetween("compiled_object", "generated_header")
  CreateGraphDependencyBetween("generated_header", "compiled_object")
  // The missing-deps tool doesn't deal with cycles in the graph, because
  // there will be an error loading the graph before we get to the tool.
  // This test is to illustrate that.
  err := ""
  nodes := state_.RootNodes(&err)
  if "" == err { t.FailNow() }
}

