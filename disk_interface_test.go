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
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

func DiskInterfaceTest(t *testing.T) RealDiskInterface {
	CreateTempDirAndEnter(t)
	return NewRealDiskInterface()
}

func Touch(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if f != nil {
		f.Close()
	}
	return err == nil
}

func TestDiskInterfaceTest_StatMissingFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	if 0 != disk_.Stat("nosuchfile", &err) {
		t.Fatal(1)
	}
	if "" != err {
		t.Fatal(err)
	}

	// On Windows, the errno for a file in a nonexistent directory
	// is different.
	if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) {
		t.Fatal(1)
	}
	if "" != err {
		t.Fatal(err)
	}

	// On POSIX systems, the errno is different (ENOTDIR) if a component of the
	// path prefix is not a directory.
	if !Touch("notadir") {
		t.Fatal(1)
	}
	if got := disk_.Stat("notadir/nosuchfile", &err); got != 0 {
		t.Fatal(got)
	}
	if "" != err {
		t.Fatal(err)
	}
}

func TestDiskInterfaceTest_StatBadPath(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""

	bad_path := strings.Repeat("x", 512)
	if runtime.GOOS == "windows" {
		bad_path = "cc:\\foo"
	}
	if got := disk_.Stat(bad_path, &err); got != -1 {
		t.Fatal(got)
	}
	if "" == err {
		t.Fatal(err)
	}
}

func TestDiskInterfaceTest_StatExistingFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	if !Touch("file") {
		t.Fatal("failed")
	}
	if disk_.Stat("file", &err) <= 1 {
		t.Fatal("failed")
	}
	if "" != err {
		t.Fatal(err)
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

func TestDiskInterfaceTest_StatCache(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}
	t.Skip("TODO")
	disk_ := DiskInterfaceTest(t)
	err := ""

	if !Touch("file1") {
		t.Fatal("expected true")
	}
	if !Touch("fiLE2") {
		t.Fatal("expected true")
	}
	if !disk_.MakeDir("subdir") {
		t.Fatal("expected true")
	}
	if !disk_.MakeDir("subdir/subsubdir") {
		t.Fatal("expected true")
	}
	if !Touch("subdir\\subfile1") {
		t.Fatal("expected true")
	}
	if !Touch("subdir\\SUBFILE2") {
		t.Fatal("expected true")
	}
	if !Touch("subdir\\SUBFILE3") {
		t.Fatal("expected true")
	}

	disk_.AllowStatCache(false)
	parent_stat_uncached := disk_.Stat("..", &err)
	disk_.AllowStatCache(true)

	if got := disk_.Stat("FIle1", &err); got <= 1 {
		t.Fatal(got)
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("file1", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if disk_.Stat("subdir/subfile2", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("sUbdir\\suBFile1", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if disk_.Stat("..", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat(".", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("subdir", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("subdir/subsubdir", &err) <= 1 {
		t.Fatal("expected greater")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if disk_.Stat("subdir", &err) != disk_.Stat("subdir/.", &err) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("subdir", &err) != disk_.Stat("subdir/subsubdir/..", &err) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("..", &err) != parent_stat_uncached {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if disk_.Stat("subdir/subsubdir", &err) != disk_.Stat("subdir/subsubdir/.", &err) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Test error cases.
	bad_path := "cc:\\foo"
	if -1 != disk_.Stat(bad_path, &err) {
		t.Fatal("expected equal")
	}
	if "" == err {
		t.Fatal("expected error")
	}
	err = ""
	if -1 != disk_.Stat(bad_path, &err) {
		t.Fatal("expected equal")
	}
	if "" == err {
		t.Fatal("expected error")
	}
	err = ""
	if 0 != disk_.Stat("nosuchfile", &err) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 0 != disk_.Stat("nosuchdir/nosuchfile", &err) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_ReadFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	err := ""
	content := ""
	if NotFound != disk_.ReadFile("foobar", &content, &err) {
		t.Fatal("expected equal")
	}
	if "" != content {
		t.Fatal("expected equal")
	}
	if "" == err {
		t.Fatal("expected different")
	} // actual value is platform-specific
	err = ""

	kTestFile := "testfile"
	f, _ := os.OpenFile(kTestFile, os.O_CREATE|os.O_RDWR, 0o600)
	if f == nil {
		t.Fatal("expected true")
	}
	kTestContent := "test content\nok"
	fmt.Fprintf(f, "%s", kTestContent)
	if nil != f.Close() {
		t.Fatal("expected equal")
	}

	if Okay != disk_.ReadFile(kTestFile, &content, &err) {
		t.Fatal("expected equal")
	}
	if content != kTestContent+"\x00" {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_MakeDirs(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	path := "path/with/double//slash/"
	if !MakeDirs(&disk_, path) {
		t.Fatal("expected true")
	}
	f, _ := os.OpenFile(path+"a_file", os.O_CREATE|os.O_RDWR, 0o600)
	if f == nil {
		t.Fatal("expected true")
	}
	if nil != f.Close() {
		t.Fatal("expected equal")
	}
	path2 := "another\\with\\back\\\\slashes\\"
	if !MakeDirs(&disk_, path2) {
		t.Fatal("expected true")
	}
	f2, _ := os.OpenFile(path2+"a_file", os.O_CREATE|os.O_RDWR, 0o600)
	if f2 == nil {
		t.Fatal("expected true")
	}
	if err := f2.Close(); err != nil {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_RemoveFile(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	kFileName := "file-to-remove"
	if !Touch(kFileName) {
		t.Fatal("expected true")
	}
	if 0 != disk_.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk_.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk_.RemoveFile("does not exist") {
		t.Fatal("expected equal")
	}
	if !Touch(kFileName) {
		t.Fatal("expected true")
	}
	// Make it read-only.
	if err := os.Chmod(kFileName, 0o400); err != nil {
		t.Fatal(err)
	}
	if 0 != disk_.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk_.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_RemoveDirectory(t *testing.T) {
	disk_ := DiskInterfaceTest(t)
	kDirectoryName := "directory-to-remove"
	if !disk_.MakeDir(kDirectoryName) {
		t.Fatal("expected true")
	}
	if 0 != disk_.RemoveFile(kDirectoryName) {
		t.Fatal("expected equal")
	}
	if 1 != disk_.RemoveFile(kDirectoryName) {
		t.Fatal("expected equal")
	}
	if 1 != disk_.RemoveFile("does not exist") {
		t.Fatal("expected equal")
	}
}

type StatTest struct {
	StateTestWithBuiltinRules
	scan_   DependencyScan
	mtimes_ map[string]TimeStamp
	stats_  []string
}

func (s *StatTest) WriteFile(path string, contents string) bool {
	s.t.Fatal("Unexpected function call")
	return false
}
func (s *StatTest) MakeDir(path string) bool {
	s.t.Fatal("Unexpected function call")
	return false
}
func (s *StatTest) ReadFile(path string, contents *string, err *string) DiskStatus {
	s.t.Fatal("Unexpected function call")
	return NotFound
}
func (s *StatTest) RemoveFile(path string) int {
	s.t.Fatal("Unexpected function call")
	return 0
}

// DiskInterface implementation.
func (s *StatTest) Stat(path string, err *string) TimeStamp {
	s.stats_ = append(s.stats_, path)
	return s.mtimes_[path]
}

func NewStatTest(t *testing.T) *StatTest {
	s := &StatTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		mtimes_:                   map[string]TimeStamp{},
	}
	s.scan_ = NewDependencyScan(&s.state_, nil, nil, s, nil)
	return s
}

func TestStatTest_Simple(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state_, "build out: cat in\n", ManifestParserOptions{})

	out := s.GetNode("out")
	err := ""
	if !out.Stat(s, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	s.scan_.RecomputeDirty(out, nil, nil)
	if 2 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	if "out" != s.stats_[0] {
		t.Fatal("expected equal")
	}
	if "in" != s.stats_[1] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_TwoStep(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	out := s.GetNode("out")
	err := ""
	if !out.Stat(s, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	s.scan_.RecomputeDirty(out, nil, nil)
	if 3 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	if "out" != s.stats_[0] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
	if "mid" != s.stats_[1] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("mid").Dirty {
		t.Fatal("expected true")
	}
	if "in" != s.stats_[2] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_Tree(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state_, "build out: cat mid1 mid2\nbuild mid1: cat in11 in12\nbuild mid2: cat in21 in22\n", ManifestParserOptions{})

	out := s.GetNode("out")
	err := ""
	if !out.Stat(s, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	s.scan_.RecomputeDirty(out, nil, nil)
	if 1+6 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	if "mid1" != s.stats_[1] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("mid1").Dirty {
		t.Fatal("expected true")
	}
	if "in11" != s.stats_[2] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_Middle(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	s.mtimes_["in"] = 1
	s.mtimes_["mid"] = 0 // missing
	s.mtimes_["out"] = 1

	out := s.GetNode("out")
	err := ""
	if !out.Stat(s, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(s.stats_) {
		t.Fatal("expected equal")
	}
	s.scan_.RecomputeDirty(out, nil, nil)
	if s.GetNode("in").Dirty {
		t.Fatal("expected false")
	}
	if !s.GetNode("mid").Dirty {
		t.Fatal("expected true")
	}
	if !s.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
}
