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
	"os"
	"strings"
	"testing"
)

// A base test fixture that includes a State object with a
// builtin "cat" rule.
type StateTestWithBuiltinRules struct {
	t      *testing.T
	state_ State
}

func NewStateTestWithBuiltinRules(t *testing.T) StateTestWithBuiltinRules {
	s := StateTestWithBuiltinRules{
		t:      t,
		state_: NewState(),
	}
	s.AddCatRule(&s.state_)
	return s
}

// Add a "cat" rule to \a state.  Used by some tests; it's
// otherwise done by the ctor to state_.
func (s *StateTestWithBuiltinRules) AddCatRule(state *State) {
	s.AssertParse(state, "rule cat\n  command = cat $in > $out\n", ManifestParserOptions{})
}

// Short way to get a Node by its path from state_.
func (s *StateTestWithBuiltinRules) GetNode(path string) *Node {
	if strings.ContainsAny(path, "/\\") {
		s.t.Fatal(path)
	}
	return s.state_.GetNode(path, 0)
}

func (s *StateTestWithBuiltinRules) AssertParse(state *State, input string, opts ManifestParserOptions) {
	parser := NewManifestParser(state, nil, opts)
	err := ""
	// In unit tests, inject the terminating 0 byte. In real code, it is injected
	// by RealDiskInterface.ReadFile.
	if !parser.ParseTest(input+"\x00", &err) {
		s.t.Fatal(err)
	}
	if err != "" {
		s.t.Fatal(err)
	}
	VerifyGraph(s.t, state)
}

func (s *StateTestWithBuiltinRules) AssertHash(expected string, actual uint64) {
	if HashCommand(expected) != actual {
		s.t.Fatalf("want %08x; got %08x", expected, actual)
	}
}

func VerifyGraph(t *testing.T, state *State) {
	for _, e := range state.edges_ {
		if len(e.Outputs) == 0 {
			t.Fatal("all edges need at least one output")
		}
		for _, inNode := range e.Inputs {
			found := false
			for _, oe := range inNode.OutEdges {
				if oe == e {
					found = true
				}
			}
			if !found {
				t.Fatal("each edge's inputs must have the edge as out-edge")
			}
		}
		for _, outNode := range e.Outputs {
			if outNode.InEdge != e {
				t.Fatal("each edge's output must have the edge as in-edge")
			}
		}
	}

	// The union of all in- and out-edges of each nodes should be exactly edges_.
	nodeEdgeSet := map[*Edge]struct{}{}
	for _, n := range state.paths_ {
		if n.InEdge != nil {
			nodeEdgeSet[n.InEdge] = struct{}{}
		}
		for _, oe := range n.OutEdges {
			nodeEdgeSet[oe] = struct{}{}
		}
	}
	if len(state.edges_) != len(nodeEdgeSet) {
		t.Fatal("the union of all in- and out-edges must match State.edges_")
	}
}

// An implementation of DiskInterface that uses an in-memory representation
// of disk state.  It also logs file accesses and directory creations
// so it can be used by tests to verify disk access patterns.
type VirtualFileSystem struct {
	// In the C++ code, it's an ordered set. The only test cases that depends on
	// this is TestBuildTest_MakeDirs.
	directoriesMade_ map[string]struct{}
	filesRead_       []string
	files_           FileMap
	filesRemoved_    map[string]struct{}
	filesCreated_    map[string]struct{}

	// A simple fake timestamp for file operations.
	now_ TimeStamp
}

// An entry for a single in-memory file.
type Entry struct {
	mtime     TimeStamp
	statError string // If mtime is -1.
	contents  string
}
type FileMap map[string]Entry

func NewVirtualFileSystem() VirtualFileSystem {
	return VirtualFileSystem{
		directoriesMade_: map[string]struct{}{},
		files_:           FileMap{},
		filesRemoved_:    map[string]struct{}{},
		filesCreated_:    map[string]struct{}{},
		now_:             1,
	}
}

// Tick "time" forwards; subsequent file operations will be newer than
// previous ones.
func (v *VirtualFileSystem) Tick() TimeStamp {
	v.now_++
	return v.now_
}

// "Create" a file with contents.
func (v *VirtualFileSystem) Create(path string, contents string) {
	f := v.files_[path]
	f.mtime = v.now_
	f.contents = contents
	v.files_[path] = f
	v.filesCreated_[path] = struct{}{}
}

// DiskInterface
func (v *VirtualFileSystem) Stat(path string, err *string) TimeStamp {
	i, ok := v.files_[path]
	if ok {
		*err = i.statError
		return i.mtime
	}
	return 0
}

func (v *VirtualFileSystem) WriteFile(path string, contents string) bool {
	v.Create(path, contents)
	return true
}

func (v *VirtualFileSystem) MakeDir(path string) bool {
	v.directoriesMade_[path] = struct{}{}
	return true // success
}

func (v *VirtualFileSystem) ReadFile(path string, contents *string, err *string) DiskStatus {
	v.filesRead_ = append(v.filesRead_, path)
	i, ok := v.files_[path]
	if ok {
		if len(i.contents) == 0 {
			*contents = ""
		} else {
			*contents = i.contents + "\x00"
		}
		return Okay
	}
	*err = "No such file or directory"
	return NotFound
}

func (v *VirtualFileSystem) RemoveFile(path string) int {
	if _, ok := v.directoriesMade_[path]; ok {
		return -1
	}
	if _, ok := v.files_[path]; ok {
		delete(v.files_, path)
		v.filesRemoved_[path] = struct{}{}
		return 0
	}
	return 1
}

// CreateTempDirAndEnter creates a temporary directory and "cd" into it.
func CreateTempDirAndEnter(t *testing.T) string {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	return tempDir
}
