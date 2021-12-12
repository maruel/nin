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

package ginga


namespace {

const char kTestFilename[] = "DepsLogTest-tempfile"

struct DepsLogTest : public testing::Test {
  func SetUp() {
    // In case a crashing test left a stale file behind.
    unlink(kTestFilename)
  }
  func TearDown() {
    unlink(kTestFilename)
  }
}

TEST_F(DepsLogTest, WriteRead) {
  State state1
  DepsLog log1
  string err
  EXPECT_TRUE(log1.OpenForWrite(kTestFilename, &err))
  ASSERT_EQ("", err)

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
    ASSERT_TRUE(log_deps)
    ASSERT_EQ(1, log_deps.mtime)
    ASSERT_EQ(2, log_deps.node_count)
    ASSERT_EQ("foo.h", log_deps.nodes[0].path())
    ASSERT_EQ("bar.h", log_deps.nodes[1].path())
  }

  log1.Close()

  State state2
  DepsLog log2
  EXPECT_TRUE(log2.Load(kTestFilename, &state2, &err))
  ASSERT_EQ("", err)

  ASSERT_EQ(log1.nodes().size(), log2.nodes().size())
  for (int i = 0; i < (int)log1.nodes().size(); ++i) {
    Node* node1 = log1.nodes()[i]
    Node* node2 = log2.nodes()[i]
    ASSERT_EQ(i, node1.id())
    ASSERT_EQ(node1.id(), node2.id())
  }

  // Spot-check the entries in log2.
  log_deps := log2.GetDeps(state2.GetNode("out2.o", 0))
  ASSERT_TRUE(log_deps)
  ASSERT_EQ(2, log_deps.mtime)
  ASSERT_EQ(2, log_deps.node_count)
  ASSERT_EQ("foo.h", log_deps.nodes[0].path())
  ASSERT_EQ("bar2.h", log_deps.nodes[1].path())
}

TEST_F(DepsLogTest, LotsOfDeps) {
  const int kNumDeps = 100000  // More than 64k.

  State state1
  DepsLog log1
  string err
  EXPECT_TRUE(log1.OpenForWrite(kTestFilename, &err))
  ASSERT_EQ("", err)

  {
    vector<Node*> deps
    for (int i = 0; i < kNumDeps; ++i) {
      char buf[32]
      sprintf(buf, "file%d.h", i)
      deps.push_back(state1.GetNode(buf, 0))
    }
    log1.RecordDeps(state1.GetNode("out.o", 0), 1, deps)

    log_deps := log1.GetDeps(state1.GetNode("out.o", 0))
    ASSERT_EQ(kNumDeps, log_deps.node_count)
  }

  log1.Close()

  State state2
  DepsLog log2
  EXPECT_TRUE(log2.Load(kTestFilename, &state2, &err))
  ASSERT_EQ("", err)

  log_deps := log2.GetDeps(state2.GetNode("out.o", 0))
  ASSERT_EQ(kNumDeps, log_deps.node_count)
}

// Verify that adding the same deps twice doesn't grow the file.
TEST_F(DepsLogTest, DoubleEntry) {
  // Write some deps to the file and grab its size.
  int file_size
  {
    State state
    DepsLog log
    string err
    EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    ASSERT_EQ(0, stat(kTestFilename, &st))
    file_size = (int)st.st_size
    ASSERT_GT(file_size, 0)
  }

  // Now reload the file, and read the same deps.
  {
    State state
    DepsLog log
    string err
    EXPECT_TRUE(log.Load(kTestFilename, &state, &err))

    EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    deps.push_back(state.GetNode("bar.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    ASSERT_EQ(0, stat(kTestFilename, &st))
    int file_size_2 = (int)st.st_size
    ASSERT_EQ(file_size, file_size_2)
  }
}

// Verify that adding the new deps works and can be compacted away.
TEST_F(DepsLogTest, Recompact) {
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
    ASSERT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

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
    ASSERT_EQ(0, stat(kTestFilename, &st))
    file_size = (int)st.st_size
    ASSERT_GT(file_size, 0)
  }

  // Now reload the file, and add slightly different deps.
  int file_size_2
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    DepsLog log
    string err
    ASSERT_TRUE(log.Load(kTestFilename, &state, &err))

    ASSERT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

    vector<Node*> deps
    deps.push_back(state.GetNode("foo.h", 0))
    log.RecordDeps(state.GetNode("out.o", 0), 1, deps)
    log.Close()

    struct stat st
    ASSERT_EQ(0, stat(kTestFilename, &st))
    file_size_2 = (int)st.st_size
    // The file should grow to record the new deps.
    ASSERT_GT(file_size_2, file_size)
  }

  // Now reload the file, verify the new deps have replaced the old, then
  // recompact.
  int file_size_3
  {
    State state
    ASSERT_NO_FATAL_FAILURE(AssertParse(&state, kManifest))
    DepsLog log
    string err
    ASSERT_TRUE(log.Load(kTestFilename, &state, &err))

    out := state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(1, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())

    other_out := state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(2, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())
    ASSERT_EQ("baz.h", deps.nodes[1].path())

    ASSERT_TRUE(log.Recompact(kTestFilename, &err))

    // The in-memory deps graph should still be valid after recompaction.
    deps = log.GetDeps(out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(1, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())
    ASSERT_EQ(out, log.nodes()[out.id()])

    deps = log.GetDeps(other_out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(2, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())
    ASSERT_EQ("baz.h", deps.nodes[1].path())
    ASSERT_EQ(other_out, log.nodes()[other_out.id()])

    // The file should have shrunk a bit for the smaller deps.
    struct stat st
    ASSERT_EQ(0, stat(kTestFilename, &st))
    file_size_3 = (int)st.st_size
    ASSERT_LT(file_size_3, file_size_2)
  }

  // Now reload the file and recompact with an empty manifest. The previous
  // entries should be removed.
  {
    State state
    // Intentionally not parsing kManifest here.
    DepsLog log
    string err
    ASSERT_TRUE(log.Load(kTestFilename, &state, &err))

    out := state.GetNode("out.o", 0)
    deps := log.GetDeps(out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(1, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())

    other_out := state.GetNode("other_out.o", 0)
    deps = log.GetDeps(other_out)
    ASSERT_TRUE(deps)
    ASSERT_EQ(1, deps.mtime)
    ASSERT_EQ(2, deps.node_count)
    ASSERT_EQ("foo.h", deps.nodes[0].path())
    ASSERT_EQ("baz.h", deps.nodes[1].path())

    ASSERT_TRUE(log.Recompact(kTestFilename, &err))

    // The previous entries should have been removed.
    deps = log.GetDeps(out)
    ASSERT_FALSE(deps)

    deps = log.GetDeps(other_out)
    ASSERT_FALSE(deps)

    // The .h files pulled in via deps should no longer have ids either.
    ASSERT_EQ(-1, state.LookupNode("foo.h").id())
    ASSERT_EQ(-1, state.LookupNode("baz.h").id())

    // The file should have shrunk more.
    struct stat st
    ASSERT_EQ(0, stat(kTestFilename, &st))
    int file_size_4 = (int)st.st_size
    ASSERT_LT(file_size_4, file_size_3)
  }
}

// Verify that invalid file headers cause a new build.
TEST_F(DepsLogTest, InvalidHeader) {
  stringkInvalidHeaders[] = {
    "",                              // Empty file.
    "# ninjad",                      // Truncated first line.
    "# ninjadeps\n",                 // No version int.
    "# ninjadeps\n\001\002",         // Truncated version int.
    "# ninjadeps\n\001\002\003\004"  // Invalid version int.
  }
  for (size_t i = 0; i < sizeof(kInvalidHeaders) / sizeof(kInvalidHeaders[0]); ++i) {
    deps_log := fopen(kTestFilename, "wb")
    ASSERT_TRUE(deps_log != nil)
    ASSERT_EQ( strlen(kInvalidHeaders[i]), fwrite(kInvalidHeaders[i], 1, strlen(kInvalidHeaders[i]), deps_log))
    ASSERT_EQ(0 ,fclose(deps_log))

    string err
    DepsLog log
    State state
    ASSERT_TRUE(log.Load(kTestFilename, &state, &err))
    EXPECT_EQ("bad deps log signature or version; starting over", err)
  }
}

// Simulate what happens when loading a truncated log file.
TEST_F(DepsLogTest, Truncated) {
  // Create a file with some entries.
  {
    State state
    DepsLog log
    string err
    EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

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
  ASSERT_EQ(0, stat(kTestFilename, &st))

  // Try reloading at truncated sizes.
  // Track how many nodes/deps were found; they should decrease with
  // smaller sizes.
  node_count := 5
  deps_count := 2
  for (int size = (int)st.st_size; size > 0; --size) {
    string err
    ASSERT_TRUE(Truncate(kTestFilename, size, &err))

    State state
    DepsLog log
    EXPECT_TRUE(log.Load(kTestFilename, &state, &err))
    if len(err) != 0 {
      // At some point the log will be so short as to be unparsable.
      break
    }

    ASSERT_GE(node_count, (int)log.nodes().size())
    node_count = log.nodes().size()

    // Count how many non-NULL deps entries there are.
    new_deps_count := 0
    for (vector<DepsLog::Deps*>::const_iterator i = log.deps().begin(); i != log.deps().end(); ++i) {
      if *i {
        ++new_deps_count
      }
    }
    ASSERT_GE(deps_count, new_deps_count)
    deps_count = new_deps_count
  }
}

// Run the truncation-recovery logic.
TEST_F(DepsLogTest, TruncatedRecovery) {
  // Create a file with some entries.
  {
    State state
    DepsLog log
    string err
    EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

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
    ASSERT_EQ(0, stat(kTestFilename, &st))
    string err
    ASSERT_TRUE(Truncate(kTestFilename, st.st_size - 2, &err))
  }

  // Load the file again, add an entry.
  {
    State state
    DepsLog log
    string err
    EXPECT_TRUE(log.Load(kTestFilename, &state, &err))
    ASSERT_EQ("premature end of file; recovering", err)
    err = nil

    // The truncated entry should've been discarded.
    EXPECT_EQ(nil, log.GetDeps(state.GetNode("out2.o", 0)))

    EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
    ASSERT_EQ("", err)

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
    EXPECT_TRUE(log.Load(kTestFilename, &state, &err))

    // The truncated entry should exist.
    deps := log.GetDeps(state.GetNode("out2.o", 0))
    ASSERT_TRUE(deps)
  }
}

TEST_F(DepsLogTest, ReverseDepsNodes) {
  State state
  DepsLog log
  string err
  EXPECT_TRUE(log.OpenForWrite(kTestFilename, &err))
  ASSERT_EQ("", err)

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
  EXPECT_TRUE(rev_deps == state.GetNode("out.o", 0))
}

}  // anonymous namespace

