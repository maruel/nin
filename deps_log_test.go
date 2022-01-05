// Copyright 2012 Google Inc. All Rights Reserved.
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepsLogTest_WriteRead(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	state1 := NewState()
	log1 := DepsLog{}
	err := ""
	if !log1.OpenForWrite(testFilename, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	{
		var deps []*Node
		deps = append(deps, state1.GetNode("foo.h", 0))
		deps = append(deps, state1.GetNode("bar.h", 0))
		if !log1.recordDeps(state1.GetNode("out.o", 0), 1, deps) {
			t.Fatal("oops")
		}

		deps = nil
		deps = append(deps, state1.GetNode("foo.h", 0))
		deps = append(deps, state1.GetNode("bar2.h", 0))
		if !log1.recordDeps(state1.GetNode("out2.o", 0), 2, deps) {
			t.Fatal("oops")
		}

		logDeps := log1.GetDeps(state1.GetNode("out.o", 0))
		if logDeps == nil {
			t.Fatal("expected true")
		}
		if 1 != logDeps.MTime {
			t.Fatal("expected equal")
		}
		if 2 != len(logDeps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != logDeps.Nodes[0].Path {
			t.Fatal("expected equal")
		}
		if "bar.h" != logDeps.Nodes[1].Path {
			t.Fatal("expected equal")
		}
	}

	log1.Close()

	state2 := NewState()
	log2 := DepsLog{}
	if log2.Load(testFilename, &state2, &err) != LoadSuccess {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}

	if len(log1.Nodes) != len(log2.Nodes) {
		t.Fatal("expected equal")
	}
	for i := 0; i < len(log1.Nodes); i++ {
		node1 := log1.Nodes[i]
		node2 := log2.Nodes[i]
		if int32(i) != node1.ID {
			t.Fatal("expected equal")
		}
		if node1.ID != node2.ID {
			t.Fatal("expected equal")
		}
	}

	// Spot-check the entries in log2.
	logDeps := log2.GetDeps(state2.GetNode("out2.o", 0))
	if logDeps == nil {
		t.Fatal("expected true")
	}
	if 2 != logDeps.MTime {
		t.Fatal("expected equal")
	}
	if 2 != len(logDeps.Nodes) {
		t.Fatal("expected equal")
	}
	if "foo.h" != logDeps.Nodes[0].Path {
		t.Fatal("expected equal")
	}
	if "bar2.h" != logDeps.Nodes[1].Path {
		t.Fatal("expected equal")
	}
}

func TestDepsLogTest_LotsOfDeps(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	const numDeps = 100000 // More than 64k.

	state1 := NewState()
	log1 := DepsLog{}
	err := ""
	if !log1.OpenForWrite(testFilename, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	{
		var deps []*Node
		for i := 0; i < numDeps; i++ {
			buf := fmt.Sprintf("file%d.h", i)
			deps = append(deps, state1.GetNode(buf, 0))
		}
		log1.recordDeps(state1.GetNode("out.o", 0), 1, deps)

		logDeps := log1.GetDeps(state1.GetNode("out.o", 0))
		if numDeps != len(logDeps.Nodes) {
			t.Fatal("expected equal")
		}
	}

	log1.Close()

	state2 := NewState()
	log2 := DepsLog{}
	if log2.Load(testFilename, &state2, &err) != LoadSuccess {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	logDeps := log2.GetDeps(state2.GetNode("out.o", 0))
	if numDeps != len(logDeps.Nodes) {
		t.Fatal("expected equal")
	}
}

func getFileSize(t *testing.T, p string) int {
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return int(fi.Size())
}

// Verify that adding the same deps twice doesn't grow the file.
func TestDepsLogTest_DoubleEntry(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	// Write some deps to the file and grab its size.
	fileSize := 0
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)
		log.Close()

		fileSize = getFileSize(t, testFilename)
		if fileSize <= 0 {
			t.Fatal("expected greater")
		}
	}

	// Now reload the file, and read the same deps.
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)
		log.Close()

		if fileSize2 := getFileSize(t, testFilename); fileSize != fileSize2 {
			t.Fatal(fileSize2)
		}
	}
}

// Verify that adding the new deps works and can be compacted away.
func TestDepsLogTest_Recompact(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	manifest := "rule cc\n  command = cc\n  deps = gcc\nbuild out.o: cc\nbuild other_out.o: cc\n"

	// Write some deps to the file and grab its size.
	fileSize := 0
	{
		state := NewState()
		assertParse(t, manifest, &state)
		log := DepsLog{}
		err := ""
		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)

		deps = nil
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("baz.h", 0))
		log.recordDeps(state.GetNode("other_out.o", 0), 1, deps)

		log.Close()

		fileSize = getFileSize(t, testFilename)
		if fileSize <= 0 {
			t.Fatal("expected greater")
		}
	}

	// Now reload the file, and add slightly different deps.
	fileSize2 := 0
	{
		state := NewState()
		assertParse(t, manifest, &state)
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)
		log.Close()

		fileSize2 = getFileSize(t, testFilename)
		// The file should grow to record the new deps.
		if fileSize2 <= fileSize {
			t.Fatal("expected greater")
		}
	}

	// Now reload the file, verify the new deps have replaced the old, then
	// recompact.
	fileSize3 := 0
	{
		state := NewState()
		assertParse(t, manifest, &state)
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		out := state.GetNode("out.o", 0)
		deps := log.GetDeps(out)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 1 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}

		otherOut := state.GetNode("other_out.o", 0)
		deps = log.GetDeps(otherOut)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 2 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}
		if "baz.h" != deps.Nodes[1].Path {
			t.Fatal("expected equal")
		}

		if !log.Recompact(testFilename, &err) {
			t.Fatal("expected true")
		}

		// The in-memory deps graph should still be valid after recompaction.
		deps = log.GetDeps(out)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 1 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}
		if out != log.Nodes[out.ID] {
			t.Fatal("expected equal")
		}

		deps = log.GetDeps(otherOut)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 2 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}
		if "baz.h" != deps.Nodes[1].Path {
			t.Fatal("expected equal")
		}
		if otherOut != log.Nodes[otherOut.ID] {
			t.Fatal("expected equal")
		}

		// The file should have shrunk a bit for the smaller deps.
		fileSize3 = getFileSize(t, testFilename)
		if fileSize3 >= fileSize2 {
			t.Fatal("expected less or equal")
		}
	}

	// Now reload the file and recompact with an empty manifest. The previous
	// entries should be removed.
	{
		state := NewState()
		// Intentionally not parsing manifest here.
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		out := state.GetNode("out.o", 0)
		deps := log.GetDeps(out)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 1 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}

		otherOut := state.GetNode("other_out.o", 0)
		deps = log.GetDeps(otherOut)
		if deps == nil {
			t.Fatal("expected true")
		}
		if 1 != deps.MTime {
			t.Fatal("expected equal")
		}
		if 2 != len(deps.Nodes) {
			t.Fatal("expected equal")
		}
		if "foo.h" != deps.Nodes[0].Path {
			t.Fatal("expected equal")
		}
		if "baz.h" != deps.Nodes[1].Path {
			t.Fatal("expected equal")
		}

		if !log.Recompact(testFilename, &err) {
			t.Fatal("expected true")
		}

		// The previous entries should have been removed.
		deps = log.GetDeps(out)
		if deps != nil {
			t.Fatal("expected false")
		}

		deps = log.GetDeps(otherOut)
		if deps != nil {
			t.Fatal("expected false")
		}

		// The .h files pulled in via deps should no longer have ids either.
		if -1 != state.Paths["foo.h"].ID {
			t.Fatal("expected equal")
		}
		if -1 != state.Paths["baz.h"].ID {
			t.Fatal("expected equal")
		}

		// The file should have shrunk more.
		if fileSize4 := getFileSize(t, testFilename); fileSize4 >= fileSize3 {
			t.Fatal(fileSize4)
		}
	}
}

// Verify that invalid file headers cause a new build.
func TestDepsLogTest_InvalidHeader(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	kInvalidHeaders := []string{
		"",                              // Empty file.
		"# ninjad",                      // Truncated first line.
		"# ninjadeps\n",                 // No version int.
		"# ninjadeps\n\001\002",         // Truncated version int.
		"# ninjadeps\n\001\002\003\004", // Invalid version int.
	}
	for i := 0; i < len(kInvalidHeaders); i++ {
		depsLog, err2 := os.OpenFile(testFilename, os.O_CREATE|os.O_WRONLY, 0o600)
		if depsLog == nil {
			t.Fatal(err2)
		}
		if _, err := depsLog.Write([]byte(kInvalidHeaders[i])); err != nil {
			t.Fatal(err)
		}
		if err := depsLog.Close(); err != nil {
			t.Fatal(err)
		}

		err := ""
		log := DepsLog{}
		state := NewState()
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		if !strings.HasPrefix(err, "bad deps log signature ") {
			t.Fatalf("%q", err)
		}
	}
}

// Simulate what happens when loading a truncated log file.
func TestDepsLogTest_Truncated(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	// Create a file with some entries.
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)

		deps = nil
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar2.h", 0))
		log.recordDeps(state.GetNode("out2.o", 0), 2, deps)

		log.Close()
	}

	// Get the file size.
	fileSize := getFileSize(t, testFilename)

	// Try reloading at truncated sizes.
	// Track how many nodes/deps were found; they should decrease with
	// smaller sizes.
	nodeCount := 5
	depsCount := 2
	for size := fileSize; size > 0; size-- {
		if err := os.Truncate(testFilename, int64(size)); err != nil {
			t.Fatal(err)
		}

		state := NewState()
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) == LoadNotFound {
			t.Fatal(err)
		}
		if len(err) != 0 {
			// At some point the log will be so short as to be unparsable.
			break
		}

		if nodeCount < len(log.Nodes) {
			t.Fatal("expected greater or equal")
		}
		nodeCount = len(log.Nodes)

		// Count how many non-NULL deps entries there are.
		newDepsCount := 0
		for _, i := range log.Deps {
			if i != nil {
				newDepsCount++
			}
		}
		if depsCount < newDepsCount {
			t.Fatal("expected greater or equal")
		}
		depsCount = newDepsCount
	}
}

// Run the truncation-recovery logic.
func TestDepsLogTest_TruncatedRecovery(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	// Create a file with some entries.
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar.h", 0))
		log.recordDeps(state.GetNode("out.o", 0), 1, deps)

		deps = nil
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar2.h", 0))
		log.recordDeps(state.GetNode("out2.o", 0), 2, deps)

		log.Close()
	}

	// Shorten the file, corrupting the last record.
	{
		fileSize := getFileSize(t, testFilename)
		const cut = 2
		if err := os.Truncate(testFilename, int64(fileSize-cut)); err != nil {
			t.Fatal(err)
		}
		if f2 := getFileSize(t, testFilename); f2 != fileSize-cut {
			t.Fatal(f2)
		}
	}

	// Load the file again, add an entry.
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}
		if !strings.HasPrefix(err, "premature end of file after") {
			t.Fatal(err)
		}
		err = ""

		// The truncated entry should've been discarded.
		if nil != log.GetDeps(state.GetNode("out2.o", 0)) {
			t.Fatal("expected out2.o to be stripped")
		}

		if !log.OpenForWrite(testFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		// Add a new entry.
		var deps []*Node
		deps = append(deps, state.GetNode("foo.h", 0))
		deps = append(deps, state.GetNode("bar2.h", 0))
		log.recordDeps(state.GetNode("out2.o", 0), 3, deps)

		log.Close()
	}

	// Load the file a third time to verify appending after a mangled
	// entry doesn't break things.
	{
		state := NewState()
		log := DepsLog{}
		err := ""
		if log.Load(testFilename, &state, &err) != LoadSuccess {
			t.Fatal("expected true")
		}

		// The truncated entry should exist.
		deps := log.GetDeps(state.GetNode("out2.o", 0))
		if deps == nil {
			t.Fatal("expected true")
		}
	}
}

func TestDepsLogTest_ReverseDepsNodes(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "DepsLogTest-tempfile")
	state := NewState()
	log := DepsLog{}
	err := ""
	if !log.OpenForWrite(testFilename, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	var deps []*Node
	deps = append(deps, state.GetNode("foo.h", 0))
	deps = append(deps, state.GetNode("bar.h", 0))
	log.recordDeps(state.GetNode("out.o", 0), 1, deps)

	deps = nil
	deps = append(deps, state.GetNode("foo.h", 0))
	deps = append(deps, state.GetNode("bar2.h", 0))
	log.recordDeps(state.GetNode("out2.o", 0), 2, deps)

	log.Close()

	revDeps := log.GetFirstReverseDepsNode(state.GetNode("foo.h", 0))
	if revDeps != state.GetNode("out.o", 0) || revDeps == state.GetNode("out2.o", 0) {
		t.Fatal("expected true")
	}

	revDeps = log.GetFirstReverseDepsNode(state.GetNode("bar.h", 0))
	if revDeps != state.GetNode("out.o", 0) {
		t.Fatal("expected true")
	}
}
