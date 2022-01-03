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

package nin

import (
	"path/filepath"
	"testing"
)

type MissingDependencyTestDelegate struct {
}

func (m *MissingDependencyTestDelegate) OnMissingDep(node *Node, path string, generator *Rule) {}

type MissingDependencyScannerTest struct {
	t              *testing.T
	delegate_      MissingDependencyTestDelegate
	generatorRule_ *Rule
	compileRule_   *Rule
	depsLog_       DepsLog
	state_         State
	filesystem_    VirtualFileSystem
	scanner_       MissingDependencyScanner
}

func NewMissingDependencyScannerTest(t *testing.T) *MissingDependencyScannerTest {
	m := &MissingDependencyScannerTest{
		t:              t,
		generatorRule_: NewRule("generator_rule"),
		compileRule_:   NewRule("compile_rule"),
		depsLog_:       NewDepsLog(),
		state_:         NewState(),
		filesystem_:    NewVirtualFileSystem(),
	}
	m.scanner_ = NewMissingDependencyScanner(&m.delegate_, &m.depsLog_, &m.state_, &m.filesystem_)
	err := ""
	kTestDepsLogFilename := filepath.Join(t.TempDir(), "MissingDepTest-tempdepslog")
	m.depsLog_.OpenForWrite(kTestDepsLogFilename, &err)
	if err != "" {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = m.depsLog_.Close()
	})
	return m
}

func (m *MissingDependencyScannerTest) scanner() *MissingDependencyScanner {
	return &m.scanner_
}

func (m *MissingDependencyScannerTest) RecordDepsLogDep(from string, to string) {
	nodeDeps := []*Node{m.state_.LookupNode(to)}
	m.depsLog_.RecordDeps(m.state_.LookupNode(from), 0, nodeDeps)
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
	depsType := &EvalString{}
	depsType.AddText("gcc")
	m.compileRule_.Bindings["deps"] = depsType
	m.generatorRule_.Bindings["deps"] = depsType
	headerEdge := m.state_.AddEdge(m.generatorRule_)
	m.state_.AddOut(headerEdge, "generated_header", 0)
	compileEdge := m.state_.AddEdge(m.compileRule_)
	m.state_.AddOut(compileEdge, "compiled_object", 0)
}

func (m *MissingDependencyScannerTest) CreateGraphDependencyBetween(from string, to string) {
	fromNode := m.state_.LookupNode(from)
	fromEdge := fromNode.InEdge
	m.state_.AddIn(fromEdge, to, 0)
}

func (m *MissingDependencyScannerTest) AssertMissingDependencyBetween(flaky string, generated string, rule *Rule) {
	flakyNode := m.state_.LookupNode(flaky)
	if 1 != countNodes(m.scanner().nodesMissingDeps_, flakyNode) {
		m.t.Fatal("expected equal")
	}
	generatedNode := m.state_.LookupNode(generated)
	if 1 != countNodes(m.scanner().generatedNodes_, generatedNode) {
		m.t.Fatal("expected equal")
	}
	if 1 != countRules(m.scanner().generatorRules_, rule) {
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
	// compiledObject uses generatedHeader, without a proper dependency
	m.RecordDepsLogDep("compiled_object", "generated_header")
	m.ProcessAllNodes()
	if !m.scanner().HadMissingDeps() {
		t.Fatal("expected true")
	}
	if 1 != len(m.scanner().nodesMissingDeps_) {
		t.Fatal("expected equal")
	}
	if 1 != m.scanner().missingDepPathCount_ {
		t.Fatal("expected equal")
	}
	m.AssertMissingDependencyBetween("compiled_object", "generated_header", m.generatorRule_)
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
	intermediateEdge := m.state_.AddEdge(m.generatorRule_)
	m.state_.AddOut(intermediateEdge, "intermediate", 0)
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
	if 2 != len(m.scanner().nodesMissingDeps_) {
		t.Fatal("expected equal")
	}
	if 2 != m.scanner().missingDepPathCount_ {
		t.Fatal("expected equal")
	}
	m.AssertMissingDependencyBetween("compiled_object", "generated_header", m.generatorRule_)
	m.AssertMissingDependencyBetween("generated_header", "compiled_object", m.compileRule_)
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
