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

package ginja

import (
	"path/filepath"
	"testing"
)

type MissingDependencyTestDelegate struct {
}

func (m *MissingDependencyTestDelegate) OnMissingDep(node *Node, path string, generator *Rule) {}

type MissingDependencyScannerTest struct {
	t               *testing.T
	delegate_       MissingDependencyTestDelegate
	generator_rule_ *Rule
	compile_rule_   *Rule
	deps_log_       DepsLog
	state_          State
	filesystem_     VirtualFileSystem
	scanner_        MissingDependencyScanner
}

func NewMissingDependencyScannerTest(t *testing.T) *MissingDependencyScannerTest {
	m := &MissingDependencyScannerTest{
		t:               t,
		generator_rule_: NewRule("generator_rule"),
		compile_rule_:   NewRule("compile_rule"),
		deps_log_:       NewDepsLog(),
		state_:          NewState(),
		filesystem_:     NewVirtualFileSystem(),
	}
	m.scanner_ = NewMissingDependencyScanner(&m.delegate_, &m.deps_log_, &m.state_, &m.filesystem_)
	err := ""
	kTestDepsLogFilename := filepath.Join(t.TempDir(), "MissingDepTest-tempdepslog")
	m.deps_log_.OpenForWrite(kTestDepsLogFilename, &err)
	if err != "" {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = m.deps_log_.Close()
	})
	return m
}

func (m *MissingDependencyScannerTest) scanner() *MissingDependencyScanner {
	return &m.scanner_
}

func (m *MissingDependencyScannerTest) RecordDepsLogDep(from string, to string) {
	node_deps := []*Node{m.state_.LookupNode(to)}
	m.deps_log_.RecordDeps(m.state_.LookupNode(from), 0, node_deps)
}

func (m *MissingDependencyScannerTest) ProcessAllNodes() {
	err := ""
	nodes := m.state_.RootNodes(&err)
	if "" != err {
		m.t.Fatal("expected equal")
	}
	for _, it := range nodes {
		m.scanner().ProcessNode(it)
	}
}

func (m *MissingDependencyScannerTest) CreateInitialState() {
	deps_type := &EvalString{}
	deps_type.AddText("gcc")
	m.compile_rule_.AddBinding("deps", deps_type)
	m.generator_rule_.AddBinding("deps", deps_type)
	header_edge := m.state_.AddEdge(m.generator_rule_)
	m.state_.AddOut(header_edge, "generated_header", 0)
	compile_edge := m.state_.AddEdge(m.compile_rule_)
	m.state_.AddOut(compile_edge, "compiled_object", 0)
}

func (m *MissingDependencyScannerTest) CreateGraphDependencyBetween(from string, to string) {
	from_node := m.state_.LookupNode(from)
	from_edge := from_node.in_edge()
	m.state_.AddIn(from_edge, to, 0)
}

func (m *MissingDependencyScannerTest) AssertMissingDependencyBetween(flaky string, generated string, rule *Rule) {
	flaky_node := m.state_.LookupNode(flaky)
	if 1 != countNodes(m.scanner().nodes_missing_deps_, flaky_node) {
		m.t.Fatal("expected equal")
	}
	generated_node := m.state_.LookupNode(generated)
	if 1 != countNodes(m.scanner().generated_nodes_, generated_node) {
		m.t.Fatal("expected equal")
	}
	if 1 != countRules(m.scanner().generator_rules_, rule) {
		m.t.Fatal("expected equal")
	}
}

func countNodes(items map[*Node]struct{}, item *Node) int {
	c := 0
	for i := range items {
		if i == item {
			c++
		}
	}
	return c
}

func countRules(items map[*Rule]struct{}, item *Rule) int {
	c := 0
	for i := range items {
		if i == item {
			c++
		}
	}
	return c
}

func TestMissingDependencyScannerTest_EmptyGraph(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.ProcessAllNodes()
	if m.scanner().HadMissingDeps() {
		t.Fatal("expected false")
	}
}

func TestMissingDependencyScannerTest_NoMissingDep(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	m.ProcessAllNodes()
	if m.scanner().HadMissingDeps() {
		t.Fatal("expected false")
	}
}

func TestMissingDependencyScannerTest_MissingDepPresent(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	// compiled_object uses generated_header, without a proper dependency
	m.RecordDepsLogDep("compiled_object", "generated_header")
	m.ProcessAllNodes()
	if !m.scanner().HadMissingDeps() {
		t.Fatal("expected true")
	}
	if 1 != len(m.scanner().nodes_missing_deps_) {
		t.Fatal("expected equal")
	}
	if 1 != m.scanner().missing_dep_path_count_ {
		t.Fatal("expected equal")
	}
	m.AssertMissingDependencyBetween("compiled_object", "generated_header", m.generator_rule_)
}

func TestMissingDependencyScannerTest_MissingDepFixedDirect(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	// Adding the direct dependency fixes the missing dep
	m.CreateGraphDependencyBetween("compiled_object", "generated_header")
	m.RecordDepsLogDep("compiled_object", "generated_header")
	m.ProcessAllNodes()
	if m.scanner().HadMissingDeps() {
		t.Fatal("expected false")
	}
}

func TestMissingDependencyScannerTest_MissingDepFixedIndirect(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	// Adding an indirect dependency also fixes the issue
	intermediate_edge := m.state_.AddEdge(m.generator_rule_)
	m.state_.AddOut(intermediate_edge, "intermediate", 0)
	m.CreateGraphDependencyBetween("compiled_object", "intermediate")
	m.CreateGraphDependencyBetween("intermediate", "generated_header")
	m.RecordDepsLogDep("compiled_object", "generated_header")
	m.ProcessAllNodes()
	if m.scanner().HadMissingDeps() {
		t.Fatal("expected false")
	}
}

func TestMissingDependencyScannerTest_CyclicMissingDep(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	m.RecordDepsLogDep("generated_header", "compiled_object")
	m.RecordDepsLogDep("compiled_object", "generated_header")
	// In case of a cycle, both paths are reported (and there is
	// no way to fix the issue by adding deps).
	m.ProcessAllNodes()
	if !m.scanner().HadMissingDeps() {
		t.Fatal("expected true")
	}
	if 2 != len(m.scanner().nodes_missing_deps_) {
		t.Fatal("expected equal")
	}
	if 2 != m.scanner().missing_dep_path_count_ {
		t.Fatal("expected equal")
	}
	m.AssertMissingDependencyBetween("compiled_object", "generated_header", m.generator_rule_)
	m.AssertMissingDependencyBetween("generated_header", "compiled_object", m.compile_rule_)
}

func TestMissingDependencyScannerTest_CycleInGraph(t *testing.T) {
	m := NewMissingDependencyScannerTest(t)
	m.CreateInitialState()
	m.CreateGraphDependencyBetween("compiled_object", "generated_header")
	m.CreateGraphDependencyBetween("generated_header", "compiled_object")
	// The missing-deps tool doesn't deal with cycles in the graph, because
	// there will be an error loading the graph before we get to the tool.
	// This test is to illustrate that.
	err := ""
	m.state_.RootNodes(&err)
	if "" == err {
		t.Fatal("expected error")
	}
}
