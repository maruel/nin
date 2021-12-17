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

  var temp_dir_ ScopedTempDir
  disk_ RealDiskInterface
}

func TestDiskInterfaceTest_StatMissingFile(t *testing.T) {
  err := ""
  if 0 != disk_.Stat("nosuchfile", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // On Windows, the errno for a file in a nonexistent directory
  // is different.
  if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // On POSIX systems, the errno is different if a component of the
  // path prefix is not a directory.
  if Touch("notadir") { t.FailNow() }
  if 0 != disk_.Stat("notadir/nosuchfile", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestDiskInterfaceTest_StatBadPath(t *testing.T) {
  err := ""
  string bad_path("cc:\\foo")
  if -1 != disk_.Stat(bad_path, &err) { t.FailNow() }
  if "" == err { t.FailNow() }
  string too_long_name(512, 'x')
  if -1 != disk_.Stat(too_long_name, &err) { t.FailNow() }
  if "" == err { t.FailNow() }
}

func TestDiskInterfaceTest_StatExistingFile(t *testing.T) {
  err := ""
  if Touch("file") { t.FailNow() }
  if disk_.Stat("file" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestDiskInterfaceTest_StatExistingDir(t *testing.T) {
  err := ""
  if disk_.MakeDir("subdir") { t.FailNow() }
  if disk_.MakeDir("subdir/subsubdir") { t.FailNow() }
  if disk_.Stat(".." <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("." <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir/subsubdir" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }

  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/.", &err) { t.FailNow() }
  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/subsubdir/..", &err) { t.FailNow() }
  if disk_.Stat("subdir/subsubdir" != &err), disk_.Stat("subdir/subsubdir/.", &err) { t.FailNow() }
}

func TestDiskInterfaceTest_StatCache(t *testing.T) {
  err := ""

  if Touch("file1") { t.FailNow() }
  if Touch("fiLE2") { t.FailNow() }
  if disk_.MakeDir("subdir") { t.FailNow() }
  if disk_.MakeDir("subdir/subsubdir") { t.FailNow() }
  if Touch("subdir\\subfile1") { t.FailNow() }
  if Touch("subdir\\SUBFILE2") { t.FailNow() }
  if Touch("subdir\\SUBFILE3") { t.FailNow() }

  disk_.AllowStatCache(false)
  TimeStamp parent_stat_uncached = disk_.Stat("..", &err)
  disk_.AllowStatCache(true)

  if disk_.Stat("FIle1" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("file1" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }

  if disk_.Stat("subdir/subfile2" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("sUbdir\\suBFile1" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }

  if disk_.Stat(".." <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("." <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir/subsubdir" <= &err), 1 { t.FailNow() }
  if "" != err { t.FailNow() }

  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/.", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/subsubdir/..", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat(".." != &err), parent_stat_uncached { t.FailNow() }
  if "" != err { t.FailNow() }
  if disk_.Stat("subdir/subsubdir" != &err), disk_.Stat("subdir/subsubdir/.", &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // Test error cases.
  string bad_path("cc:\\foo")
  if -1 != disk_.Stat(bad_path, &err) { t.FailNow() }
  EXPECT_NE("", err); err = nil
  if -1 != disk_.Stat(bad_path, &err) { t.FailNow() }
  EXPECT_NE("", err); err = nil
  if 0 != disk_.Stat("nosuchfile", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestDiskInterfaceTest_ReadFile(t *testing.T) {
  err := ""
  content := ""
  if DiskInterface::NotFound != disk_.ReadFile("foobar", &content, &err) { t.FailNow() }
  if "" != content { t.FailNow() }
  if "" == err { t.FailNow() } // actual value is platform-specific
  err = nil

  string kTestFile = "testfile"
  FILE* f = fopen(kTestFile, "wb")
  if f { t.FailNow() }
  string kTestContent = "test content\nok"
  fprintf(f, "%s", kTestContent)
  if 0 != fclose(f) { t.FailNow() }

  if DiskInterface::Okay != disk_.ReadFile(kTestFile, &content, &err) { t.FailNow() }
  if kTestContent != content { t.FailNow() }
  if "" != err { t.FailNow() }
}

func TestDiskInterfaceTest_MakeDirs(t *testing.T) {
  string path = "path/with/double//slash/";
  if disk_.MakeDirs(path) { t.FailNow() }
  FILE* f = fopen((path + "a_file"), "w")
  if f { t.FailNow() }
  if 0 != fclose(f) { t.FailNow() }
  string path2 = "another\\with\\back\\\\slashes\\"
  if disk_.MakeDirs(path2) { t.FailNow() }
  FILE* f2 = fopen((path2 + "a_file"), "w")
  if f2 { t.FailNow() }
  if 0 != fclose(f2) { t.FailNow() }
}

func TestDiskInterfaceTest_RemoveFile(t *testing.T) {
  string kFileName = "file-to-remove"
  if Touch(kFileName) { t.FailNow() }
  if 0 != disk_.RemoveFile(kFileName) { t.FailNow() }
  if 1 != disk_.RemoveFile(kFileName) { t.FailNow() }
  if 1 != disk_.RemoveFile("does not exist") { t.FailNow() }
  if Touch(kFileName) { t.FailNow() }
  if 0 != system((string("attrib +R ") + kFileName)) { t.FailNow() }
  if 0 != disk_.RemoveFile(kFileName) { t.FailNow() }
  if 1 != disk_.RemoveFile(kFileName) { t.FailNow() }
}

func TestDiskInterfaceTest_RemoveDirectory(t *testing.T) {
  string kDirectoryName = "directory-to-remove"
  if disk_.MakeDir(kDirectoryName) { t.FailNow() }
  if 0 != disk_.RemoveFile(kDirectoryName) { t.FailNow() }
  if 1 != disk_.RemoveFile(kDirectoryName) { t.FailNow() }
  if 1 != disk_.RemoveFile("does not exist") { t.FailNow() }
}

type StatTest struct {
  StatTest() : scan_(&state_, nil, nil, this, nil) {}

  func (s *StatTest) WriteFile(path string, contents string) bool {
    if !false { panic("oops") }
    return true
  }
  func (s *StatTest) MakeDir(path string) bool {
    if !false { panic("oops") }
    return false
  }
  func (s *StatTest) ReadFile(path string, contents *string, err *string) Status {
    if !false { panic("oops") }
    return NotFound
  }
  func (s *StatTest) RemoveFile(path string) int {
    if !false { panic("oops") }
    return 0
  }

  var scan_ DependencyScan
  map<string, TimeStamp> mtimes_
  mutable vector<string> stats_
}

// DiskInterface implementation.
func (s *StatTest) Stat(path string, err *string) TimeStamp {
  stats_.push_back(path)
  i := mtimes_.find(path)
  if i == mtimes_.end() {
    return 0  // File not found.
  }
  return i.second
}

func TestStatTest_Simple(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat in\n"))

  Node* out = GetNode("out")
  err := ""
  if out.Stat(this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != stats_.size() { t.FailNow() }
  scan_.RecomputeDirty(out, nil)
  if 2u != stats_.size() { t.FailNow() }
  if "out" != stats_[0] { t.FailNow() }
  if "in" !=  stats_[1] { t.FailNow() }
}

func TestStatTest_TwoStep(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n"))

  Node* out = GetNode("out")
  err := ""
  if out.Stat(this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != stats_.size() { t.FailNow() }
  scan_.RecomputeDirty(out, nil)
  if 3u != stats_.size() { t.FailNow() }
  if "out" != stats_[0] { t.FailNow() }
  if GetNode("out").dirty() { t.FailNow() }
  if "mid" !=  stats_[1] { t.FailNow() }
  if GetNode("mid").dirty() { t.FailNow() }
  if "in" !=  stats_[2] { t.FailNow() }
}

func TestStatTest_Tree(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid1 mid2\n" "build mid1: cat in11 in12\n" "build mid2: cat in21 in22\n"))

  Node* out = GetNode("out")
  err := ""
  if out.Stat(this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != stats_.size() { t.FailNow() }
  scan_.RecomputeDirty(out, nil)
  if 1u + 6u != stats_.size() { t.FailNow() }
  if "mid1" != stats_[1] { t.FailNow() }
  if GetNode("mid1").dirty() { t.FailNow() }
  if "in11" != stats_[2] { t.FailNow() }
}

func TestStatTest_Middle(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n"))

  mtimes_["in"] = 1
  mtimes_["mid"] = 0  // missing
  mtimes_["out"] = 1

  Node* out = GetNode("out")
  err := ""
  if out.Stat(this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != stats_.size() { t.FailNow() }
  scan_.RecomputeDirty(out, nil)
  if !GetNode("in").dirty() { t.FailNow() }
  if GetNode("mid").dirty() { t.FailNow() }
  if GetNode("out").dirty() { t.FailNow() }
}

