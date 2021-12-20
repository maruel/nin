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

//void AssertParse(State* state, string input, ManifestParserOptions = ManifestParserOptions())

func (s *StateTestWithBuiltinRules) AssertParse(state *State, input string, opts ManifestParserOptions) {
	parser := NewManifestParser(state, nil, opts)
	err := ""
	if !parser.ParseTest(input, &err) {
		s.t.Fatal(err)
	}
	if "" != err {
		s.t.Fatal(err)
	}
	s.VerifyGraph(state)
}

func (s *StateTestWithBuiltinRules) AssertHash(expected string, actual uint64) {
	s.t.Fatal("TODO")
	/*
		if HashCommand(expected) != actual {
			panic(actual)
		}
	*/
}

func (s *StateTestWithBuiltinRules) VerifyGraph(state *State) {
	for _, e := range state.edges_ {
		if len(e.outputs_) == 0 {
			s.t.Fatal("all edges need at least one output")
		}
		for _, in_node := range e.inputs_ {
			found := false
			for _, oe := range in_node.out_edges() {
				if oe == e {
					found = true
				}
			}
			if !found {
				s.t.Fatal("each edge's inputs must have the edge as out-edge")
			}
		}
		for _, out_node := range e.outputs_ {
			if out_node.in_edge() != e {
				s.t.Fatal("each edge's output must have the edge as in-edge")
			}
		}
	}

	// The union of all in- and out-edges of each nodes should be exactly edges_.
	node_edge_set := map[*Edge]struct{}{}
	for _, n := range state.paths_ {
		if n.in_edge() != nil {
			node_edge_set[n.in_edge()] = struct{}{}
		}
		for _, oe := range n.out_edges() {
			node_edge_set[oe] = struct{}{}
		}
	}
	if len(state.edges_) != len(node_edge_set) {
		s.t.Fatal("the union of all in- and out-edges must match State.edges_")
	}
}

// An implementation of DiskInterface that uses an in-memory representation
// of disk state.  It also logs file accesses and directory creations
// so it can be used by tests to verify disk access patterns.
type VirtualFileSystem struct {
	directories_made_ []string
	files_read_       []string
	files_            FileMap
	files_removed_    map[string]struct{}
	files_created_    map[string]struct{}

	// A simple fake timestamp for file operations.
	now_ TimeStamp
}

// An entry for a single in-memory file.
type Entry struct {
	mtime      TimeStamp
	stat_error string // If mtime is -1.
	contents   string
}
type FileMap map[string]Entry

func NewVirtualFileSystem() VirtualFileSystem {
	return VirtualFileSystem{
		files_:         FileMap{},
		files_removed_: map[string]struct{}{},
		files_created_: map[string]struct{}{},
		now_:           1,
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
	v.files_created_[path] = struct{}{}
}

// DiskInterface
func (v *VirtualFileSystem) Stat(path string, err *string) TimeStamp {
	i, ok := v.files_[path]
	if ok {
		*err = i.stat_error
		return i.mtime
	}
	return 0
}

func (v *VirtualFileSystem) WriteFile(path string, contents string) bool {
	v.Create(path, contents)
	return true
}

func (v *VirtualFileSystem) MakeDir(path string) bool {
	v.directories_made_ = append(v.directories_made_, path)
	return true // success
}

func (v *VirtualFileSystem) ReadFile(path string, contents *string, err *string) DiskStatus {
	v.files_read_ = append(v.files_read_, path)
	i, ok := v.files_[path]
	if ok {
		*contents = i.contents
		return Okay
	}
	*err = "Not found"
	return NotFound
}

func (v *VirtualFileSystem) RemoveFile(path string) int {
	panic("TODO")
	/*
		if find(v.directories_made_.begin(), v.directories_made_.end(), path) != v.directories_made_.end() {
			return -1
		}
		i := v.files_.find(path)
		if i != v.files_.end() {
			v.files_.erase(i)
			v.files_removed_.insert(path)
			return 0
		} else {
			return 1
		}
	*/
	return 0
}

type ScopedTempDir struct {
	// The temp directory containing our dir.
	start_dir_ string
	// The subdirectory name for our dir, or empty if it hasn't been set up.
	temp_dir_name_ string
}

/*
// Create a temporary directory and chdir into it.
func (s *ScopedTempDir) CreateAndEnter(name string) {
  // First change into the system temp dir and save it for cleanup.
  s.start_dir_ = GetSystemTempDir()
  if s.start_dir_.empty() {
    Fatal("couldn't get system temp dir")
  }
  if chdir(s.start_dir_) < 0 {
    Fatal("chdir: %s", strerror(errno))
  }

  // Create a temporary subdirectory of that.
  char name_template[1024]
  strcpy(name_template, name)
  strcat(name_template, "-XXXXXX")
  tempname := mkdtemp(name_template)
  if tempname == nil {
    Fatal("mkdtemp: %s", strerror(errno))
  }
  s.temp_dir_name_ = tempname

  // chdir into the new temporary directory.
  if chdir(s.temp_dir_name_) < 0 {
    Fatal("chdir: %s", strerror(errno))
  }
}

// Clean up the temporary directory.
func (s *ScopedTempDir) Cleanup() {
  if s.temp_dir_name_.empty() {
    return  // Something went wrong earlier.
  }

  // Move out of the directory we're about to clobber.
  if chdir(s.start_dir_) < 0 {
    Fatal("chdir: %s", strerror(errno))
  }

  string command = "rmdir /s /q " + s.temp_dir_name_
  if system(command) < 0 {
    Fatal("system: %s", strerror(errno))
  }

  s.temp_dir_name_ = nil
}
*/
