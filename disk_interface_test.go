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
	"os"
	"testing"
)

func DiskInterfaceTest(t *testing.T) RealDiskInterface {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	//d := DiskInterfaceTest{temp_dir_: t.TempDir()}
	temp_dir_ := t.TempDir()
	if err := os.Chdir(temp_dir_); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	return RealDiskInterface{}
}

/*
type DiskInterfaceTest struct {
	//ScopedTempDir
	temp_dir_ string
	disk_     RealDiskInterface
}

/*
func (d *DiskInterfaceTest) SetUp() {
  // These tests do real disk accesses, so create a temp dir.
  d.temp_dir_.CreateAndEnter("Ninja-DiskInterfaceTest")
}
func (d *DiskInterfaceTest) TearDown() {
  d.temp_dir_.Cleanup()
}
*/
func Touch(path string) bool {
	/*
	  FILE *f = fopen(path, "w")
	  if f == nil {
	    return false
	  }
	  return fclose(f) == 0
	*/
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if f != nil {
		f.Close()
	}
	return err != nil
}

func TestDiskInterfaceTest_StatMissingFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	if 0 != disk_.Stat("nosuchfile", &err) {
		t.Fatal(1)
	}
	if "" == err {
		t.Fatal(err)
	}

	// On Windows, the errno for a file in a nonexistent directory
	// is different.
	if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) {
		t.Fatal(1)
	}
	if "" == err {
		t.Fatal(err)
	}

	// On POSIX systems, the errno is different if a component of the
	// path prefix is not a directory.
	if Touch("notadir") {
		t.Fatal(1)
	}
	if got := disk_.Stat("notadir/nosuchfile", &err); got != -1 {
		t.Fatal(got)
	}
	if "" == err {
		t.Fatal(err)
	}
}

func TestDiskInterfaceTest_StatBadPath(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	bad_path := "cc:\\foo"
	if got := disk_.Stat(bad_path, &err); got != 0 {
		t.Fatal(got)
	}
	if "" == err {
		t.Fatal(err)
	}
}

func TestDiskInterfaceTest_StatExistingFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	if Touch("file") {
		t.FailNow()
	}
	if disk_.Stat("file", &err) <= 1 {
		t.FailNow()
	}
	if "" != err {
		t.FailNow()
	}
}

func TestDiskInterfaceTest_StatExistingDir(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	if !disk_.MakeDir("subdir") {
		t.Fatal(0)
	}
	if !disk_.MakeDir("subdir/subsubdir") {
		t.Fatal(0)
	}
	if disk_.Stat("..", &err) <= 1 {
		t.Fatal(0)
	}
	if "" != err {
		t.Fatal(err)
	}
	if disk_.Stat(".", &err) <= 1 {
		t.Fatal(0)
	}
	if "" != err {
		t.Fatal(err)
	}
	if disk_.Stat("subdir", &err) <= 1 {
		t.Fatal(0)
	}
	if "" != err {
		t.Fatal(err)
	}
	if disk_.Stat("subdir/subsubdir", &err) <= 1 {
		t.Fatal(0)
	}
	if "" != err {
		t.Fatal(err)
	}

	if disk_.Stat("subdir", &err) != disk_.Stat("subdir/.", &err) {
		t.Fatal(0)
	}
	if disk_.Stat("subdir", &err) != disk_.Stat("subdir/subsubdir/..", &err) {
		t.Fatal(0)
	}
	if disk_.Stat("subdir/subsubdir", &err) != disk_.Stat("subdir/subsubdir/.", &err) {
		t.Fatal(0)
	}
}

/*
func TestDiskInterfaceTest_StatCache(t *testing.T) {
  err := ""

  if !Touch("file1") { t.Fatal("expected true") }
  if !Touch("fiLE2") { t.Fatal("expected true") }
  if !disk_.MakeDir("subdir") { t.Fatal("expected true") }
  if !disk_.MakeDir("subdir/subsubdir") { t.Fatal("expected true") }
  if !Touch("subdir\\subfile1") { t.Fatal("expected true") }
  if !Touch("subdir\\SUBFILE2") { t.Fatal("expected true") }
  if !Touch("subdir\\SUBFILE3") { t.Fatal("expected true") }

  disk_.AllowStatCache(false)
  TimeStamp parent_stat_uncached = disk_.Stat("..", &err)
  disk_.AllowStatCache(true)

  if disk_.Stat("FIle1" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("file1" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }

  if disk_.Stat("subdir/subfile2" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("sUbdir\\suBFile1" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }

  if disk_.Stat(".." <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("." <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("subdir" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("subdir/subsubdir" <= &err), 1 { t.Fatal("expected greater") }
  if "" != err { t.Fatal("expected equal") }

  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/.", &err) { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("subdir" != &err), disk_.Stat("subdir/subsubdir/..", &err) { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat(".." != &err), parent_stat_uncached { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
  if disk_.Stat("subdir/subsubdir" != &err), disk_.Stat("subdir/subsubdir/.", &err) { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }

  // Test error cases.
  string bad_path("cc:\\foo")
  if -1 != disk_.Stat(bad_path, &err) { t.Fatal("expected equal") }
  EXPECT_NE("", err); err = nil
  if -1 != disk_.Stat(bad_path, &err) { t.Fatal("expected equal") }
  EXPECT_NE("", err); err = nil
  if 0 != disk_.Stat("nosuchfile", &err) { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
  if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
}

func TestDiskInterfaceTest_ReadFile(t *testing.T) {
  err := ""
  content := ""
  if DiskInterface::NotFound != disk_.ReadFile("foobar", &content, &err) { t.Fatal("expected equal") }
  if "" != content { t.Fatal("expected equal") }
  if "" == err { t.Fatal("expected different") } // actual value is platform-specific
  err = nil

  string kTestFile = "testfile"
  FILE* f = fopen(kTestFile, "wb")
  if !f { t.Fatal("expected true") }
  string kTestContent = "test content\nok"
  fprintf(f, "%s", kTestContent)
  if 0 != fclose(f) { t.Fatal("expected equal") }

  if DiskInterface::Okay != disk_.ReadFile(kTestFile, &content, &err) { t.Fatal("expected equal") }
  if kTestContent != content { t.Fatal("expected equal") }
  if "" != err { t.Fatal("expected equal") }
}

func TestDiskInterfaceTest_MakeDirs(t *testing.T) {
  string path = "path/with/double//slash/";
  if !disk_.MakeDirs(path) { t.Fatal("expected true") }
  FILE* f = fopen((path + "a_file"), "w")
  if !f { t.Fatal("expected true") }
  if 0 != fclose(f) { t.Fatal("expected equal") }
  string path2 = "another\\with\\back\\\\slashes\\"
  if !disk_.MakeDirs(path2) { t.Fatal("expected true") }
  FILE* f2 = fopen((path2 + "a_file"), "w")
  if !f2 { t.Fatal("expected true") }
  if 0 != fclose(f2) { t.Fatal("expected equal") }
}

func TestDiskInterfaceTest_RemoveFile(t *testing.T) {
  string kFileName = "file-to-remove"
  if !Touch(kFileName) { t.Fatal("expected true") }
  if 0 != disk_.RemoveFile(kFileName) { t.Fatal("expected equal") }
  if 1 != disk_.RemoveFile(kFileName) { t.Fatal("expected equal") }
  if 1 != disk_.RemoveFile("does not exist") { t.Fatal("expected equal") }
  if !Touch(kFileName) { t.Fatal("expected true") }
  if 0 != system((string("attrib +R ") + kFileName)) { t.Fatal("expected equal") }
  if 0 != disk_.RemoveFile(kFileName) { t.Fatal("expected equal") }
  if 1 != disk_.RemoveFile(kFileName) { t.Fatal("expected equal") }
}

func TestDiskInterfaceTest_RemoveDirectory(t *testing.T) {
  string kDirectoryName = "directory-to-remove"
  if !disk_.MakeDir(kDirectoryName) { t.Fatal("expected true") }
  if 0 != disk_.RemoveFile(kDirectoryName) { t.Fatal("expected equal") }
  if 1 != disk_.RemoveFile(kDirectoryName) { t.Fatal("expected equal") }
  if 1 != disk_.RemoveFile("does not exist") { t.Fatal("expected equal") }
}

type StatTest struct {

  scan_ DependencyScan
  mtimes_ map[string]TimeStamp
  stats_ mutable vector<string>
}
func NewStatTest() StatTest {
	return StatTest{
		scan_: &state_, nil, nil, this, nil,
	}
}
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

// DiskInterface implementation.
func (s *StatTest) Stat(path string, err *string) TimeStamp {
  s.stats_.push_back(path)
  i := s.mtimes_.find(path)
  if i == s.mtimes_.end() {
    return 0  // File not found.
  }
  return i.second
}

func TestStatTest_Simple(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat in\n"))

  Node* out = GetNode("out")
  err := ""
  if !out.Stat(this, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }
  if 1u != stats_.size() { t.Fatal("expected equal") }
  scan_.RecomputeDirty(out, nil)
  if 2u != stats_.size() { t.Fatal("expected equal") }
  if "out" != stats_[0] { t.Fatal("expected equal") }
  if "in" !=  stats_[1] { t.Fatal("expected equal") }
}

func TestStatTest_TwoStep(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\nbuild mid: cat in\n"))

  Node* out = GetNode("out")
  err := ""
  if !out.Stat(this, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }
  if 1u != stats_.size() { t.Fatal("expected equal") }
  scan_.RecomputeDirty(out, nil)
  if 3u != stats_.size() { t.Fatal("expected equal") }
  if "out" != stats_[0] { t.Fatal("expected equal") }
  if !GetNode("out").dirty() { t.Fatal("expected true") }
  if "mid" !=  stats_[1] { t.Fatal("expected equal") }
  if !GetNode("mid").dirty() { t.Fatal("expected true") }
  if "in" !=  stats_[2] { t.Fatal("expected equal") }
}

func TestStatTest_Tree(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid1 mid2\nbuild mid1: cat in11 in12\nbuild mid2: cat in21 in22\n"))

  Node* out = GetNode("out")
  err := ""
  if !out.Stat(this, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }
  if 1u != stats_.size() { t.Fatal("expected equal") }
  scan_.RecomputeDirty(out, nil)
  if 1u + 6u != stats_.size() { t.Fatal("expected equal") }
  if "mid1" != stats_[1] { t.Fatal("expected equal") }
  if !GetNode("mid1").dirty() { t.Fatal("expected true") }
  if "in11" != stats_[2] { t.Fatal("expected equal") }
}

func TestStatTest_Middle(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse(&state_, "build out: cat mid\nbuild mid: cat in\n"))

  mtimes_["in"] = 1
  mtimes_["mid"] = 0  // missing
  mtimes_["out"] = 1

  Node* out = GetNode("out")
  err := ""
  if !out.Stat(this, &err) { t.Fatal("expected true") }
  if "" != err { t.Fatal("expected equal") }
  if 1u != stats_.size() { t.Fatal("expected equal") }
  scan_.RecomputeDirty(out, nil)
  if GetNode("in").dirty() { t.Fatal("expected false") }
  if !GetNode("mid").dirty() { t.Fatal("expected true") }
  if !GetNode("out").dirty() { t.Fatal("expected true") }
}
*/
