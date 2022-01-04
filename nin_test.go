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
	t     *testing.T
	state State
}

func NewStateTestWithBuiltinRules(t *testing.T) StateTestWithBuiltinRules {
	s := StateTestWithBuiltinRules{
		t:     t,
		state: NewState(),
	}
	s.AddCatRule(&s.state)
	return s
}

// Add a "cat" rule to \a state.  Used by some tests; it's
// otherwise done by the ctor to state.
func (s *StateTestWithBuiltinRules) AddCatRule(state *State) {
	s.AssertParse(state, "rule cat\n  command = cat $in > $out\n", ManifestParserOptions{})
}

// Short way to get a Node by its path from state.
func (s *StateTestWithBuiltinRules) GetNode(path string) *Node {
	if strings.ContainsAny(path, "/\\") {
		s.t.Fatal(path)
	}
	return s.state.GetNode(path, 0)
}

func (s *StateTestWithBuiltinRules) AssertParse(state *State, input string, opts ManifestParserOptions) {
	parser := NewManifestParser(state, nil, opts)
	err := ""
	// In unit tests, inject the terminating 0 byte. In real code, it is injected
	// by RealDiskInterface.ReadFile.
	if !parser.parseTest(input, &err) {
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
	for _, e := range state.Edges {
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

	// The union of all in- and out-edges of each nodes should be exactly edges.
	nodeEdgeSet := map[*Edge]struct{}{}
	for _, n := range state.Paths {
		if n.InEdge != nil {
			nodeEdgeSet[n.InEdge] = struct{}{}
		}
		for _, oe := range n.OutEdges {
			nodeEdgeSet[oe] = struct{}{}
		}
	}
	if len(state.Edges) != len(nodeEdgeSet) {
		t.Fatal("the union of all in- and out-edges must match State.edges")
	}
}

// An implementation of DiskInterface that uses an in-memory representation
// of disk state.  It also logs file accesses and directory creations
// so it can be used by tests to verify disk access patterns.
type VirtualFileSystem struct {
	// In the C++ code, it's an ordered set. The only test cases that depends on
	// this is TestBuildTest_MakeDirs.
	directoriesMade map[string]struct{}
	filesRead       []string
	files           FileMap
	filesRemoved    map[string]struct{}
	filesCreated    map[string]struct{}

	// A simple fake timestamp for file operations.
	now TimeStamp
}

// An entry for a single in-memory file.
type Entry struct {
	mtime     TimeStamp
	statError error // If mtime is -1.
	contents  []byte
}
type FileMap map[string]Entry

func NewVirtualFileSystem() VirtualFileSystem {
	return VirtualFileSystem{
		directoriesMade: map[string]struct{}{},
		files:           FileMap{},
		filesRemoved:    map[string]struct{}{},
		filesCreated:    map[string]struct{}{},
		now:             1,
	}
}

// Tick "time" forwards; subsequent file operations will be newer than
// previous ones.
func (v *VirtualFileSystem) Tick() TimeStamp {
	v.now++
	return v.now
}

// "Create" a file with contents.
func (v *VirtualFileSystem) Create(path string, contents string) {
	f := v.files[path]
	f.mtime = v.now
	// Make a copy in case it's a unsafeString() to a buffer that could be
	// mutated later.
	f.contents = []byte(contents)
	v.files[path] = f
	v.filesCreated[path] = struct{}{}
}

// DiskInterface
func (v *VirtualFileSystem) Stat(path string) (TimeStamp, error) {
	i, ok := v.files[path]
	if ok {
		return i.mtime, i.statError
	}
	return 0, nil
}

func (v *VirtualFileSystem) WriteFile(path string, contents string) bool {
	v.Create(path, contents)
	return true
}

func (v *VirtualFileSystem) MakeDir(path string) bool {
	v.directoriesMade[path] = struct{}{}
	return true // success
}

func (v *VirtualFileSystem) ReadFile(path string) ([]byte, error) {
	v.filesRead = append(v.filesRead, path)
	i, ok := v.files[path]
	if ok {
		if len(i.contents) == 0 {
			return nil, nil
		}
		// Return a copy since a lot of the code modify the buffer in-place.
		n := make([]byte, len(i.contents)+1)
		copy(n, i.contents)
		return n, nil
	}
	return nil, os.ErrNotExist
}

func (v *VirtualFileSystem) RemoveFile(path string) int {
	if _, ok := v.directoriesMade[path]; ok {
		return -1
	}
	if _, ok := v.files[path]; ok {
		delete(v.files, path)
		v.filesRemoved[path] = struct{}{}
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
