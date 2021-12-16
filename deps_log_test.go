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
  func (d *DepsLogTest) SetUp() {
    // In case a crashing test left a stale file behind.
    unlink(kTestFilename)
  }
  func (d *DepsLogTest) TearDown() {
    unlink(kTestFilename)
  }
}

func TestDepsLogTest_WriteRead(t *testing.T) {
  State state1
  DepsLog log1
  string err
  if log1.OpenForWrite(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  {
    vector<Node*> deps
    deps.push_back(state1.GetNode("foo.h", 0))
    deps.push_back(state1.GetNode("bar.h", 0))
    log1.RecordDeps(state1.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state1.GetNode("foo.h", 0))
    deps.push_back(state1.GetNode("bar2.h", 0))
    log1.RecordDeps(state1.GetNode("out2.o", 0), 2, deps)

    log_deps := log1.GetDeps(state1.GetNode("out.o", 0))
    if log_deps { t.FailNow() }
    if 1 != log_deps.mtime { t.FailNow() }
    if 2 != log_deps.node_count { t.FailNow() }
    if "foo.h" != log_deps.nodes[0].path() { t.FailNow() }
    if "bar.h" != log_deps.nodes[1].path() { t.FailNow() }
  }

  log1.Close()

  State state2
  DepsLog log2
  if log2.Load(kTestFilename, &state2, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if log1.nodes().size() != log2.nodes().size() { t.FailNow() }
  for (int i = 0; i < (int)log1.nodes().size(); ++i) {
    Node* node1 = log1.nodes()[i]
    Node* node2 = log2.nodes()[i]
    if i != node1.id() { t.FailNow() }
    if node1.id() != node2.id() { t.FailNow() }
  }

  // Spot-check the entries in log2.
  log_deps := log2.GetDeps(state2.GetNode("out2.o", 0))
  if log_deps { t.FailNow() }
  if 2 != log_deps.mtime { t.FailNow() }
  if 2 != log_deps.node_count { t.FailNow() }
  if "foo.h" != log_deps.nodes[0].path() { t.FailNow() }
  if "bar2.h" != log_deps.nodes[1].path() { t.FailNow() }
}

func TestDepsLogTest_LotsOfDeps(t *testing.T) {
  const int kNumDeps = 100000  // More than 64k.

  State state1
  DepsLog log1
  string err
  if log1.OpenForWrite(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  {
    vector<Node*> deps
    for (int i = 0; i < kNumDeps; ++i) {
      char buf[32]
      sprintf(buf, "file%d.h", i)
      deps.push_back(state1.GetNode(buf, 0))
    }
    log1.RecordDeps(state1.GetNode("out.o", 0), 1, deps)

    log_deps := log1.GetDeps(state1.GetNode("out.o", 0))
    if kNumDeps != log_deps.node_count { t.FailNow() }
  }

  log1.Close()

  State state2
  DepsLog log2
  if log2.Load(kTestFilename, &state2, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  log_deps := log2.GetDeps(state2.GetNode("out.o", 0))
  if kNumDeps != log_deps.node_count { t.FailNow() }
}

// Verify that adding the same deps twice doesn't grow the file.
func TestDepsLogTest_DoubleEntry(t *testing.T) {
  // Write some deps to the file and grab its size.
  int file_size
  {
    State state
    DepsLog log
    string err
    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    file_size = (int)st.st_size
    if file_size <= 0 { t.FailNow() }
  }

  // Now reload the file, and read the same deps.
  {
    State state
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }

    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    int file_size_2 = (int)st.st_size
    if file_size != file_size_2 { t.FailNow() }
  }
}

// Verify that adding the new deps works and can be compacted away.
func TestDepsLogTest_Recompact(t *testing.T) {
  const char kManifest[] =
"rule cc\n"
"  command = cc\n"
"  deps = gcc\n"
"build out.o: cc\n"
"build other_out.o: cc\n"

  // Write some deps to the file and grab its size.
  int file_size
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    DepsLog log
    string err
    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

    deps = nil
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("baz.h", 0))
    log.RecordDeps(state.GetNode("other_out.o", 0), 1, deps)

    log.Close()

    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    file_size = (int)st.st_size
    if file_size <= 0 { t.FailNow() }
  }

  // Now reload the file, and add slightly different deps.
  int file_size_2
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }

    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    file_size_2 = (int)st.st_size
    // The file should grow to record the new deps.
    if file_size_2 <= file_size { t.FailNow() }
  }

  // Now reload the file, verify the new deps have replaced the old, then
  // recompact.
  int file_size_3
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }

    out := state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 1 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }

    other_out := state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 2 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }
    if "baz.h" != deps.nodes[1].path() { t.FailNow() }

    if log.Recompact(kTestFilename, &err) { t.FailNow() }

    // The in-memory deps graph should still be valid after recompaction.
    deps = log.GetDeps(out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 1 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }
    if out != log.nodes()[out.id()] { t.FailNow() }

    deps = log.GetDeps(other_out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 2 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }
    if "baz.h" != deps.nodes[1].path() { t.FailNow() }
    if other_out != log.nodes()[other_out.id()] { t.FailNow() }

    // The file should have shrunk a bit for the smaller deps.
    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    file_size_3 = (int)st.st_size
    if file_size_3 >= file_size_2 { t.FailNow() }
  }

  // Now reload the file and recompact with an empty manifest. The previous
  // entries should be removed.
  {
    State state
    // Intentionally not parsing kManifest here.
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }

    out := state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 1 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }

    other_out := state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    if deps { t.FailNow() }
    if 1 != deps.mtime { t.FailNow() }
    if 2 != deps.node_count { t.FailNow() }
    if "foo.h" != deps.nodes[0].path() { t.FailNow() }
    if "baz.h" != deps.nodes[1].path() { t.FailNow() }

    if log.Recompact(kTestFilename, &err) { t.FailNow() }

    // The previous entries should have been removed.
    deps = log.GetDeps(out)
    if !deps { t.FailNow() }

    deps = log.GetDeps(other_out)
    if !deps { t.FailNow() }

    // The .h files pulled in via deps should no longer have ids either.
    if -1 != state.LookupNode("foo.h").id() { t.FailNow() }
    if -1 != state.LookupNode("baz.h").id() { t.FailNow() }

    // The file should have shrunk more.
    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    int file_size_4 = (int)st.st_size
    if file_size_4 >= file_size_3 { t.FailNow() }
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
  for (size_t i = 0; i < sizeof(kInvalidHeaders) / sizeof(kInvalidHeaders[0]); ++i) {
    deps_log := fopen(kTestFilename, "wb")
    if deps_log != nil { t.FailNow() }
    ASSERT_EQ( strlen(kInvalidHeaders[i]), fwrite(kInvalidHeaders[i], 1, strlen(kInvalidHeaders[i]), deps_log))
    ASSERT_EQ(0 ,fclose(deps_log))

    string err
    DepsLog log
    State state
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }
    if "bad deps log signature or version; starting over" != err { t.FailNow() }
  }
}

// Simulate what happens when loading a truncated log file.
func TestDepsLogTest_Truncated(t *testing.T) {
  // Create a file with some entries.
  {
    State state
    DepsLog log
    string err
    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
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
  struct stat st
  if 0 != stat(kTestFilename, &st) { t.FailNow() }

  // Try reloading at truncated sizes.
  // Track how many nodes/deps were found; they should decrease with
  // smaller sizes.
  node_count := 5
  deps_count := 2
  for (int size = (int)st.st_size; size > 0; --size) {
    string err
    if Truncate(kTestFilename, size, &err) { t.FailNow() }

    State state
    DepsLog log
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }
    if len(err) != 0 {
      // At some point the log will be so short as to be unparsable.
      break
    }

    if node_count < (int)log.nodes().size() { t.FailNow() }
    node_count = log.nodes().size()

    // Count how many non-NULL deps entries there are.
    new_deps_count := 0
    for (vector<DepsLog::Deps*>::const_iterator i = log.deps().begin(); i != log.deps().end(); ++i) {
      if *i {
        ++new_deps_count
      }
    }
    if deps_count < new_deps_count { t.FailNow() }
    deps_count = new_deps_count
  }
}

// Run the truncation-recovery logic.
func TestDepsLogTest_TruncatedRecovery(t *testing.T) {
  // Create a file with some entries.
  {
    State state
    DepsLog log
    string err
    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    vector<Node*> deps
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
    struct stat st
    if 0 != stat(kTestFilename, &st) { t.FailNow() }
    string err
    if Truncate(kTestFilename, st.st_size - 2, &err) { t.FailNow() }
  }

  // Load the file again, add an entry.
  {
    State state
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }
    if "premature end of file; recovering" != err { t.FailNow() }
    err = nil

    // The truncated entry should've been discarded.
    if nil != log.GetDeps(state.GetNode("out2.o", 0)) { t.FailNow() }

    if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
    if "" != err { t.FailNow() }

    // Add a new entry.
    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar2.h", 0))
    log.RecordDeps(state.GetNode("out2.o", 0), 3, deps)

    log.Close()
  }

  // Load the file a third time to verify appending after a mangled
  // entry doesn't break things.
  {
    State state
    DepsLog log
    string err
    if log.Load(kTestFilename, &state, &err) { t.FailNow() }

    // The truncated entry should exist.
    deps := log.GetDeps(state.GetNode("out2.o", 0))
    if deps { t.FailNow() }
  }
}

func TestDepsLogTest_ReverseDepsNodes(t *testing.T) {
  State state
  DepsLog log
  string err
  if log.OpenForWrite(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  vector<Node*> deps
  deps.push_back(state.GetNode("foo.h", 0))
  deps.push_back(state.GetNode("bar.h", 0))
  log.RecordDeps(state.GetNode("out.o", 0), 1, deps)

  deps = nil
  deps.push_back(state.GetNode("foo.h", 0))
  deps.push_back(state.GetNode("bar2.h", 0))
  log.RecordDeps(state.GetNode("out2.o", 0), 2, deps)

  log.Close()

  rev_deps := log.GetFirstReverseDepsNode(state.GetNode("foo.h", 0))
  EXPECT_TRUE(rev_deps == state.GetNode("out.o", 0) || rev_deps == state.GetNode("out2.o", 0))

  rev_deps = log.GetFirstReverseDepsNode(state.GetNode("bar.h", 0))
  if rev_deps == state.GetNode("out.o", 0) { t.FailNow() }
}

