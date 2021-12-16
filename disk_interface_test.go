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

//go:build nobuild

package ginja


type DiskInterfaceTest struct {
  func (d *DiskInterfaceTest) SetUp() {
    // These tests do real disk accesses, so create a temp dir.
    temp_dir_.CreateAndEnter("Ninja-DiskInterfaceTest")
  }

  func (d *DiskInterfaceTest) TearDown() {
    temp_dir_.Cleanup()
  }

  func (d *DiskInterfaceTest) Touch(path string) bool {
    FILE *f = fopen(path, "w")
    if f == nil {
      return false
    }
    return fclose(f) == 0
  }

  ScopedTempDir temp_dir_
  RealDiskInterface disk_
}

func TestDiskInterfaceTest_StatMissingFile(t *testing.T) {
  string err
  EXPECT_EQ(0, disk_.Stat("nosuchfile", &err))
  EXPECT_EQ("", err)

  // On Windows, the errno for a file in a nonexistent directory
  // is different.
  EXPECT_EQ(0, disk_.Stat("nosuchdir/nosuchfile", &err))
  EXPECT_EQ("", err)

  // On POSIX systems, the errno is different if a component of the
  // path prefix is not a directory.
  ASSERT_TRUE(Touch("notadir"))
  EXPECT_EQ(0, disk_.Stat("notadir/nosuchfile", &err))
  EXPECT_EQ("", err)
}

func TestDiskInterfaceTest_StatBadPath(t *testing.T) {
  string err
  string bad_path("cc:\\foo")
  EXPECT_EQ(-1, disk_.Stat(bad_path, &err))
  EXPECT_NE("", err)
  string too_long_name(512, 'x')
  EXPECT_EQ(-1, disk_.Stat(too_long_name, &err))
  EXPECT_NE("", err)
}

func TestDiskInterfaceTest_StatExistingFile(t *testing.T) {
  string err
  ASSERT_TRUE(Touch("file"))
  EXPECT_GT(disk_.Stat("file", &err), 1)
  EXPECT_EQ("", err)
}

func TestDiskInterfaceTest_StatExistingDir(t *testing.T) {
  string err
  ASSERT_TRUE(disk_.MakeDir("subdir"))
  ASSERT_TRUE(disk_.MakeDir("subdir/subsubdir"))
  EXPECT_GT(disk_.Stat("..", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat(".", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("subdir", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("subdir/subsubdir", &err), 1)
  EXPECT_EQ("", err)

  EXPECT_EQ(disk_.Stat("subdir", &err), disk_.Stat("subdir/.", &err))
  EXPECT_EQ(disk_.Stat("subdir", &err), disk_.Stat("subdir/subsubdir/..", &err))
  EXPECT_EQ(disk_.Stat("subdir/subsubdir", &err), disk_.Stat("subdir/subsubdir/.", &err))
}

func TestDiskInterfaceTest_StatCache(t *testing.T) {
  string err

  ASSERT_TRUE(Touch("file1"))
  ASSERT_TRUE(Touch("fiLE2"))
  ASSERT_TRUE(disk_.MakeDir("subdir"))
  ASSERT_TRUE(disk_.MakeDir("subdir/subsubdir"))
  ASSERT_TRUE(Touch("subdir\\subfile1"))
  ASSERT_TRUE(Touch("subdir\\SUBFILE2"))
  ASSERT_TRUE(Touch("subdir\\SUBFILE3"))

  disk_.AllowStatCache(false)
  parent_stat_uncached := disk_.Stat("..", &err)
  disk_.AllowStatCache(true)

  EXPECT_GT(disk_.Stat("FIle1", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("file1", &err), 1)
  EXPECT_EQ("", err)

  EXPECT_GT(disk_.Stat("subdir/subfile2", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("sUbdir\\suBFile1", &err), 1)
  EXPECT_EQ("", err)

  EXPECT_GT(disk_.Stat("..", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat(".", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("subdir", &err), 1)
  EXPECT_EQ("", err)
  EXPECT_GT(disk_.Stat("subdir/subsubdir", &err), 1)
  EXPECT_EQ("", err)

  EXPECT_EQ(disk_.Stat("subdir", &err), disk_.Stat("subdir/.", &err))
  EXPECT_EQ("", err)
  EXPECT_EQ(disk_.Stat("subdir", &err), disk_.Stat("subdir/subsubdir/..", &err))
  EXPECT_EQ("", err)
  EXPECT_EQ(disk_.Stat("..", &err), parent_stat_uncached)
  EXPECT_EQ("", err)
  EXPECT_EQ(disk_.Stat("subdir/subsubdir", &err), disk_.Stat("subdir/subsubdir/.", &err))
  EXPECT_EQ("", err)

  // Test error cases.
  string bad_path("cc:\\foo")
  EXPECT_EQ(-1, disk_.Stat(bad_path, &err))
  EXPECT_NE("", err); err = nil
  EXPECT_EQ(-1, disk_.Stat(bad_path, &err))
  EXPECT_NE("", err); err = nil
  EXPECT_EQ(0, disk_.Stat("nosuchfile", &err))
  EXPECT_EQ("", err)
  EXPECT_EQ(0, disk_.Stat("nosuchdir/nosuchfile", &err))
  EXPECT_EQ("", err)
}

func TestDiskInterfaceTest_ReadFile(t *testing.T) {
  string err
  string content
  ASSERT_EQ(DiskInterface::NotFound, disk_.ReadFile("foobar", &content, &err))
  EXPECT_EQ("", content)
  EXPECT_NE("", err) // actual value is platform-specific
  err = nil

  kTestFile := "testfile"
  f := fopen(kTestFile, "wb")
  ASSERT_TRUE(f)
  kTestContent := "test content\nok"
  fprintf(f, "%s", kTestContent)
  ASSERT_EQ(0, fclose(f))

  ASSERT_EQ(DiskInterface::Okay, disk_.ReadFile(kTestFile, &content, &err))
  EXPECT_EQ(kTestContent, content)
  EXPECT_EQ("", err)
}

func TestDiskInterfaceTest_MakeDirs(t *testing.T) {
  string path = "path/with/double//slash/";
  EXPECT_TRUE(disk_.MakeDirs(path))
  f := fopen((path + "a_file"), "w")
  EXPECT_TRUE(f)
  EXPECT_EQ(0, fclose(f))
  string path2 = "another\\with\\back\\\\slashes\\"
  EXPECT_TRUE(disk_.MakeDirs(path2))
  FILE* f2 = fopen((path2 + "a_file"), "w")
  EXPECT_TRUE(f2)
  EXPECT_EQ(0, fclose(f2))
}

func TestDiskInterfaceTest_RemoveFile(t *testing.T) {
  kFileName := "file-to-remove"
  ASSERT_TRUE(Touch(kFileName))
  EXPECT_EQ(0, disk_.RemoveFile(kFileName))
  EXPECT_EQ(1, disk_.RemoveFile(kFileName))
  EXPECT_EQ(1, disk_.RemoveFile("does not exist"))
  ASSERT_TRUE(Touch(kFileName))
  EXPECT_EQ(0, system((string("attrib +R ") + kFileName)))
  EXPECT_EQ(0, disk_.RemoveFile(kFileName))
  EXPECT_EQ(1, disk_.RemoveFile(kFileName))
}

func TestDiskInterfaceTest_RemoveDirectory(t *testing.T) {
  kDirectoryName := "directory-to-remove"
  EXPECT_TRUE(disk_.MakeDir(kDirectoryName))
  EXPECT_EQ(0, disk_.RemoveFile(kDirectoryName))
  EXPECT_EQ(1, disk_.RemoveFile(kDirectoryName))
  EXPECT_EQ(1, disk_.RemoveFile("does not exist"))
}

type StatTest struct {
  StatTest() : scan_(&state_, nil, nil, this, nil) {}

  func (s *StatTest) WriteFile(path string, contents string) bool {
    assert(false)
    return true
  }
  func (s *StatTest) MakeDir(path string) bool {
    assert(false)
    return false
  }
  func (s *StatTest) ReadFile(path string, contents *string, err *string) Status {
    assert(false)
    return NotFound
  }
  func (s *StatTest) RemoveFile(path string) int {
    assert(false)
    return 0
  }

  DependencyScan scan_
  map<string, TimeStamp> mtimes_
  mutable vector<string> stats_
}

// DiskInterface implementation.
func (s *StatTest) Stat(path string, err *string) TimeStamp {
  stats_.push_back(path)
  map<string, TimeStamp>::const_iterator i = mtimes_.find(path)
  if i == mtimes_.end() {
    return 0  // File not found.
  }
  return i.second
}

func TestStatTest_Simple(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat in\n"))

  out := GetNode("out")
  string err
  EXPECT_TRUE(out.Stat(this, &err))
  EXPECT_EQ("", err)
  ASSERT_EQ(1u, stats_.size())
  scan_.RecomputeDirty(out, nil)
  ASSERT_EQ(2u, stats_.size())
  ASSERT_EQ("out", stats_[0])
  ASSERT_EQ("in",  stats_[1])
}

func TestStatTest_TwoStep(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n"))

  out := GetNode("out")
  string err
  EXPECT_TRUE(out.Stat(this, &err))
  EXPECT_EQ("", err)
  ASSERT_EQ(1u, stats_.size())
  scan_.RecomputeDirty(out, nil)
  ASSERT_EQ(3u, stats_.size())
  ASSERT_EQ("out", stats_[0])
  ASSERT_TRUE(GetNode("out").dirty())
  ASSERT_EQ("mid",  stats_[1])
  ASSERT_TRUE(GetNode("mid").dirty())
  ASSERT_EQ("in",  stats_[2])
}

func TestStatTest_Tree(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid1 mid2\n" "build mid1: cat in11 in12\n" "build mid2: cat in21 in22\n"))

  out := GetNode("out")
  string err
  EXPECT_TRUE(out.Stat(this, &err))
  EXPECT_EQ("", err)
  ASSERT_EQ(1u, stats_.size())
  scan_.RecomputeDirty(out, nil)
  ASSERT_EQ(1u + 6u, stats_.size())
  ASSERT_EQ("mid1", stats_[1])
  ASSERT_TRUE(GetNode("mid1").dirty())
  ASSERT_EQ("in11", stats_[2])
}

func TestStatTest_Middle(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n"))

  mtimes_["in"] = 1
  mtimes_["mid"] = 0  // missing
  mtimes_["out"] = 1

  out := GetNode("out")
  string err
  EXPECT_TRUE(out.Stat(this, &err))
  EXPECT_EQ("", err)
  ASSERT_EQ(1u, stats_.size())
  scan_.RecomputeDirty(out, nil)
  ASSERT_FALSE(GetNode("in").dirty())
  ASSERT_TRUE(GetNode("mid").dirty())
  ASSERT_TRUE(GetNode("out").dirty())
}

