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

//go:build nobuild

package ginja


const char kTestFilename[] = "DepsLogTest-tempfile"

type DepsLogTest struct {
}
func (d *DepsLogTest) SetUp() {
  // In case a crashing test left a stale file behind.
  unlink(kTestFilename)
}
func (d *DepsLogTest) TearDown() {
  unlink(kTestFilename)
}

func TestDepsLogTest_WriteRead(t *testing.T) {
  var state1 State
  var log1 DepsLog
  err := ""
  if !log1.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }

  {
    var deps []*Node
    deps.push_back(state1.GetNode("foo.h", 0))
    deps.push_back(state1.GetNode("bar.h", 0))
    log1.RecordDeps(state1.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state1.GetNode("foo.h", 0))
    deps.push_back(state1.GetNode("bar2.h", 0))
    log1.RecordDeps(state1.GetNode("out2.o", 0), 2, deps)

    DepsLog::Deps* log_deps = log1.GetDeps(state1.GetNode("out.o", 0))
    if !log_deps { t.Fatal("expected true") }
    if 1 != log_deps.mtime { t.Fatal("expected equal") }
    if 2 != log_deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != log_deps.nodes[0].path() { t.Fatal("expected equal") }
    if "bar.h" != log_deps.nodes[1].path() { t.Fatal("expected equal") }
  }

  log1.Close()

  var state2 State
  var log2 DepsLog
  if !log2.Load(kTestFilename, &state2, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }

  if log1.nodes().size() != log2.nodes().size() { t.Fatal("expected equal") }
  for i := 0; i < (int)log1.nodes().size(); i++ {
    node1 := log1.nodes()[i]
    node2 := log2.nodes()[i]
    if i != node1.id() { t.Fatal("expected equal") }
    if node1.id() != node2.id() { t.Fatal("expected equal") }
  }

  // Spot-check the entries in log2.
  DepsLog::Deps* log_deps = log2.GetDeps(state2.GetNode("out2.o", 0))
  if !log_deps { t.Fatal("expected true") }
  if 2 != log_deps.mtime { t.Fatal("expected equal") }
  if 2 != log_deps.node_count { t.Fatal("expected equal") }
  if "foo.h" != log_deps.nodes[0].path() { t.Fatal("expected equal") }
  if "bar2.h" != log_deps.nodes[1].path() { t.Fatal("expected equal") }
}

func TestDepsLogTest_LotsOfDeps(t *testing.T) {
  kNumDeps := 100000  // More than 64k.

  var state1 State
  var log1 DepsLog
  err := ""
  if !log1.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }

  {
    var deps []*Node
    for i := 0; i < kNumDeps; i++ {
      char buf[32]
      sprintf(buf, "file%d.h", i)
      deps.push_back(state1.GetNode(buf, 0))
    }
    log1.RecordDeps(state1.GetNode("out.o", 0), 1, deps)

    DepsLog::Deps* log_deps = log1.GetDeps(state1.GetNode("out.o", 0))
    if kNumDeps != log_deps.node_count { t.Fatal("expected equal") }
  }

  log1.Close()

  var state2 State
  var log2 DepsLog
  if !log2.Load(kTestFilename, &state2, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }

  DepsLog::Deps* log_deps = log2.GetDeps(state2.GetNode("out.o", 0))
  if kNumDeps != log_deps.node_count { t.Fatal("expected equal") }
}

// Verify that adding the same deps twice doesn't grow the file.
func TestDepsLogTest_DoubleEntry(t *testing.T) {
  // Write some deps to the file and grab its size.
  file_size := 0
  {
    var state State
    var log DepsLog
    err := ""
    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size = (int)st.st_size
    if file_size <= 0 { t.Fatal("expected greater") }
  }

  // Now reload the file, and read the same deps.
  {
    var state State
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }

    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size_2 := (int)st.st_size
    if file_size != file_size_2 { t.Fatal("expected equal") }
  }
}

// Verify that adding the new deps works and can be compacted away.
func TestDepsLogTest_Recompact(t *testing.T) {
  const char kManifest[] =
"rule cc\n  command = cc\n  deps = gcc\nbuild out.o: cc\nbuild other_out.o: cc\n"

  // Write some deps to the file and grab its size.
  file_size := 0
  {
    var state State
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    var log DepsLog
    err := ""
    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("baz.h", 0))
    log.RecordDeps(state.GetNode("other_out.o", 0), 1, deps)

    log.Close()

    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size = (int)st.st_size
    if file_size <= 0 { t.Fatal("expected greater") }
  }

  // Now reload the file, and add slightly different deps.
  file_size_2 := 0
  {
    var state State
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }

    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size_2 = (int)st.st_size
    // The file should grow to record the new deps.
    if file_size_2 <= file_size { t.Fatal("expected greater") }
  }

  // Now reload the file, verify the new deps have replaced the old, then
  // recompact.
  file_size_3 := 0
  {
    var state State
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }

    Node* out = state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 1 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }

    Node* other_out = state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 2 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }
    if "baz.h" != deps.nodes[1].path() { t.Fatal("expected equal") }

    if !log.Recompact(kTestFilename, &err) { t.Fatal("expected true") }

    // The in-memory deps graph should still be valid after recompaction.
    deps = log.GetDeps(out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 1 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }
    if out != log.nodes()[out.id()] { t.Fatal("expected equal") }

    deps = log.GetDeps(other_out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 2 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }
    if "baz.h" != deps.nodes[1].path() { t.Fatal("expected equal") }
    if other_out != log.nodes()[other_out.id()] { t.Fatal("expected equal") }

    // The file should have shrunk a bit for the smaller deps.
    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size_3 = (int)st.st_size
    if file_size_3 >= file_size_2 { t.Fatal("expected less or equal") }
  }

  // Now reload the file and recompact with an empty manifest. The previous
  // entries should be removed.
  {
    var state State
    // Intentionally not parsing kManifest here.
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }

    Node* out = state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 1 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }

    Node* other_out = state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    if !deps { t.Fatal("expected true") }
    if 1 != deps.mtime { t.Fatal("expected equal") }
    if 2 != deps.node_count { t.Fatal("expected equal") }
    if "foo.h" != deps.nodes[0].path() { t.Fatal("expected equal") }
    if "baz.h" != deps.nodes[1].path() { t.Fatal("expected equal") }

    if !log.Recompact(kTestFilename, &err) { t.Fatal("expected true") }

    // The previous entries should have been removed.
    deps = log.GetDeps(out)
    if deps { t.Fatal("expected false") }

    deps = log.GetDeps(other_out)
    if deps { t.Fatal("expected false") }

    // The .h files pulled in via deps should no longer have ids either.
    if -1 != state.LookupNode("foo.h").id() { t.Fatal("expected equal") }
    if -1 != state.LookupNode("baz.h").id() { t.Fatal("expected equal") }

    // The file should have shrunk more.
    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    file_size_4 := (int)st.st_size
    if file_size_4 >= file_size_3 { t.Fatal("expected less or equal") }
  }
}

// Verify that invalid file headers cause a new build.
func TestDepsLogTest_InvalidHeader(t *testing.T) {
  stringkInvalidHeaders[] = {
    "",                              // Empty file.
    "# ninjad",                      // Truncated first line.
    "# ninjadeps\n",                 // No version int.
    "# ninjadeps\n\001\002",         // Truncated version int.
    "# ninjadeps\n\001\002\003\004"  // Invalid version int.
  }
  for i := 0; i < sizeof(kInvalidHeaders) / sizeof(kInvalidHeaders[0]); i++ {
    FILE* deps_log = fopen(kTestFilename, "wb")
    if !deps_log != nil { t.Fatal("expected true") }
    if  strlen(kInvalidHeaders[i]) != fwrite(kInvalidHeaders[i], 1, strlen(kInvalidHeaders[i]), deps_log) { t.Fatal("expected equal") }
    ASSERT_EQ(0 ,fclose(deps_log))

    err := ""
    var log DepsLog
    var state State
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }
    if "bad deps log signature or version; starting over" != err { t.Fatal("expected equal") }
  }
}

// Simulate what happens when loading a truncated log file.
func TestDepsLogTest_Truncated(t *testing.T) {
  // Create a file with some entries.
  {
    var state State
    var log DepsLog
    err := ""
    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar2.h", 0))
    log.RecordDeps(state.GetNode("out2.o", 0), 2, deps)

    log.Close()
  }

  // Get the file size.
  var st stat
  if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }

  // Try reloading at truncated sizes.
  // Track how many nodes/deps were found; they should decrease with
  // smaller sizes.
  node_count := 5
  deps_count := 2
  for size := (int)st.st_size; size > 0; size-- {
    err := ""
    if !Truncate(kTestFilename, size, &err) { t.Fatal("expected true") }

    var state State
    var log DepsLog
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }
    if len(err) != 0 {
      // At some point the log will be so short as to be unparsable.
      break
    }

    if node_count < (int)log.nodes().size() { t.Fatal("expected greater or equal") }
    node_count = log.nodes().size()

    // Count how many non-NULL deps entries there are.
    new_deps_count := 0
    for i := log.deps().begin(); i != log.deps().end(); i++ {
      if *i {
        new_deps_count++
      }
    }
    if deps_count < new_deps_count { t.Fatal("expected greater or equal") }
    deps_count = new_deps_count
  }
}

// Run the truncation-recovery logic.
func TestDepsLogTest_TruncatedRecovery(t *testing.T) {
  // Create a file with some entries.
  {
    var state State
    var log DepsLog
    err := ""
    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar2.h", 0))
    log.RecordDeps(state.GetNode("out2.o", 0), 2, deps)

    log.Close()
  }

  // Shorten the file, corrupting the last record.
  {
    var st stat
    if 0 != stat(kTestFilename, &st) { t.Fatal("expected equal") }
    err := ""
    if !Truncate(kTestFilename, st.st_size - 2, &err) { t.Fatal("expected true") }
  }

  // Load the file again, add an entry.
  {
    var state State
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }
    if "premature end of file; recovering" != err { t.Fatal("expected equal") }
    err = nil

    // The truncated entry should've been discarded.
    if nil != log.GetDeps(state.GetNode("out2.o", 0)) { t.Fatal("expected equal") }

    if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
    if "" != err { t.Fatal("expected equal") }

    // Add a new entry.
    var deps []*Node
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar2.h", 0))
    log.RecordDeps(state.GetNode("out2.o", 0), 3, deps)

    log.Close()
  }

  // Load the file a third time to verify appending after a mangled
  // entry doesn't break things.
  {
    var state State
    var log DepsLog
    err := ""
    if !log.Load(kTestFilename, &state, &err) { t.Fatal("expected true") }

    // The truncated entry should exist.
    DepsLog::Deps* deps = log.GetDeps(state.GetNode("out2.o", 0))
    if !deps { t.Fatal("expected true") }
  }
}

func TestDepsLogTest_ReverseDepsNodes(t *testing.T) {
  var state State
  var log DepsLog
  err := ""
  if !log.OpenForWrite(kTestFilename, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }

  var deps []*Node
  deps.push_back(state.GetNode("foo.h", 0))
  deps.push_back(state.GetNode("bar.h", 0))
  log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

  deps = nil
  deps.push_back(state.GetNode("foo.h", 0))
  deps.push_back(state.GetNode("bar2.h", 0))
  log.RecordDeps(state.GetNode("out2.o", 0), 2, deps)

  log.Close()

  Node* rev_deps = log.GetFirstReverseDepsNode(state.GetNode("foo.h", 0))
  if !rev_deps == state.GetNode("out.o", 0) || rev_deps == state.GetNode("out2.o", 0) { t.Fatal("expected true") }

  rev_deps = log.GetFirstReverseDepsNode(state.GetNode("bar.h", 0))
  if !rev_deps == state.GetNode("out.o", 0) { t.Fatal("expected true") }
}

