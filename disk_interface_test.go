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
	disk := DiskInterfaceTest(t)
	if mtime, err := disk.Stat("nosuchfile"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// On Windows, the errno for a file in a nonexistent directory
	// is different.
	if mtime, err := disk.Stat("nosuchdir/nosuchfile"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// On POSIX systems, the errno is different (ENOTDIR) if a component of the
	// path prefix is not a directory.
	if !Touch("notadir") {
		t.Fatal(1)
	}
	if mtime, err := disk.Stat("notadir/nosuchfile"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestDiskInterfaceTest_StatBadPath(t *testing.T) {
	disk := DiskInterfaceTest(t)
	badPath := strings.Repeat("x", 512)
	if runtime.GOOS == "windows" {
		badPath = "cc:\\foo"
	}
	if mtime, err := disk.Stat(badPath); mtime != -1 || err == nil {
		t.Fatal(mtime, err)
	}
}

func TestDiskInterfaceTest_StatExistingFile(t *testing.T) {
	disk := DiskInterfaceTest(t)
	if !Touch("file") {
		t.Fatal("failed")
	}
	if mtime, err := disk.Stat("file"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestDiskInterfaceTest_StatExistingDir(t *testing.T) {
	disk := DiskInterfaceTest(t)
	if !disk.MakeDir("subdir") {
		t.Fatal(0)
	}
	if !disk.MakeDir("subdir/subsubdir") {
		t.Fatal(0)
	}
	if mtime, err := disk.Stat(".."); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("."); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("subdir"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("subdir/subsubdir"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	mtime1, err1 := disk.Stat("subdir")
	mtime2, err2 := disk.Stat("subdir/.")
	if mtime1 != mtime2 || err1 != err2 {
		t.Fatal(mtime1, err1, mtime2, err2)
	}
	mtime3, err3 := disk.Stat("subdir/subsubdir/..")
	if mtime1 != mtime3 || err1 != err3 {
		t.Fatal(mtime1, err1, mtime3, err3)
	}
	mtime4, err4 := disk.Stat("subdir/subsubdir")
	mtime5, err5 := disk.Stat("subdir/subsubdir/.")
	if mtime4 != mtime5 || err4 != err5 {
		t.Fatal(mtime4, err4, mtime5, err5)
	}
}

func TestDiskInterfaceTest_StatCache(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}
	t.Skip("TODO")
	disk := DiskInterfaceTest(t)
	if !Touch("file1") {
		t.Fatal("expected true")
	}
	if !Touch("fiLE2") {
		t.Fatal("expected true")
	}
	if !disk.MakeDir("subdir") {
		t.Fatal("expected true")
	}
	if !disk.MakeDir("subdir/subsubdir") {
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

	disk.AllowStatCache(false)
	parentStatUncached, erru := disk.Stat("..")
	if erru != nil {
		t.Fatal(erru)
	}
	disk.AllowStatCache(true)

	if mtime, err := disk.Stat("FIle1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("file1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("subdir/subfile2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("sUbdir\\suBFile1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat(".."); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("."); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("subdir"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("subdir/subsubdir"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	mtime1, err1 := disk.Stat("subdir")
	mtime2, err2 := disk.Stat("subdir/.")
	if mtime1 != mtime2 || err1 != err2 {
		t.Fatal(mtime1, err1, mtime2, err2)
	}
	mtime3, err3 := disk.Stat("subdir/subsubdir/..")
	if mtime1 != mtime3 || err1 != err3 {
		t.Fatal(mtime1, err1, mtime3, err3)
	}
	if mtime, err := disk.Stat(".."); mtime != parentStatUncached || err != nil {
		t.Fatal(mtime, err)
	}
	mtime4, err4 := disk.Stat("subdir/subsubdir")
	mtime5, err5 := disk.Stat("subdir/subsubdir/.")
	if mtime4 != mtime5 || err4 != err5 {
		t.Fatal(mtime4, err4, mtime5, err5)
	}

	// Test error cases.
	badPath := "cc:\\foo"
	if mtime, err := disk.Stat(badPath); mtime != -1 || err == nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat(badPath); mtime != -1 || err == nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("nosuchfile"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := disk.Stat("nosuchdir/nosuchfile"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestDiskInterfaceTest_ReadFile(t *testing.T) {
	disk := DiskInterfaceTest(t)
	err := ""
	content := ""
	if NotFound != disk.ReadFile("foobar", &content, &err) {
		t.Fatal("expected equal")
	}
	if "" != content {
		t.Fatal("expected equal")
	}
	if "" == err {
		t.Fatal("expected different")
	} // actual value is platform-specific
	err = ""

	testFile := "testfile"
	f, _ := os.OpenFile(testFile, os.O_CREATE|os.O_RDWR, 0o600)
	if f == nil {
		t.Fatal("expected true")
	}
	testContent := "test content\nok"
	fmt.Fprintf(f, "%s", testContent)
	if nil != f.Close() {
		t.Fatal("expected equal")
	}

	if Okay != disk.ReadFile(testFile, &content, &err) {
		t.Fatal("expected equal")
	}
	if content != testContent+"\x00" {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_MakeDirs(t *testing.T) {
	disk := DiskInterfaceTest(t)
	path := "path/with/double//slash/"
	if !MakeDirs(&disk, path) {
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
	if !MakeDirs(&disk, path2) {
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
	disk := DiskInterfaceTest(t)
	kFileName := "file-to-remove"
	if !Touch(kFileName) {
		t.Fatal("expected true")
	}
	if 0 != disk.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk.RemoveFile("does not exist") {
		t.Fatal("expected equal")
	}
	if !Touch(kFileName) {
		t.Fatal("expected true")
	}
	// Make it read-only.
	if err := os.Chmod(kFileName, 0o400); err != nil {
		t.Fatal(err)
	}
	if 0 != disk.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
	if 1 != disk.RemoveFile(kFileName) {
		t.Fatal("expected equal")
	}
}

func TestDiskInterfaceTest_RemoveDirectory(t *testing.T) {
	disk := DiskInterfaceTest(t)
	kDirectoryName := "directory-to-remove"
	if !disk.MakeDir(kDirectoryName) {
		t.Fatal("expected true")
	}
	if 0 != disk.RemoveFile(kDirectoryName) {
		t.Fatal("expected equal")
	}
	if 1 != disk.RemoveFile(kDirectoryName) {
		t.Fatal("expected equal")
	}
	if 1 != disk.RemoveFile("does not exist") {
		t.Fatal("expected equal")
	}
}

type StatTest struct {
	StateTestWithBuiltinRules
	scan   DependencyScan
	mtimes map[string]TimeStamp
	stats  []string
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
func (s *StatTest) Stat(path string) (TimeStamp, error) {
	s.stats = append(s.stats, path)
	return s.mtimes[path], nil
}

func NewStatTest(t *testing.T) *StatTest {
	s := &StatTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		mtimes:                    map[string]TimeStamp{},
	}
	s.scan = NewDependencyScan(&s.state, nil, nil, s)
	return s
}

func TestStatTest_Simple(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state, "build out: cat in\n", ManifestParserOptions{})

	out := s.GetNode("out")
	if err := out.Stat(s); err != nil {
		t.Fatal(err)
	}
	if 1 != len(s.stats) {
		t.Fatal("expected equal")
	}
	s.scan.RecomputeDirty(out, nil, nil)
	if 2 != len(s.stats) {
		t.Fatal("expected equal")
	}
	if "out" != s.stats[0] {
		t.Fatal("expected equal")
	}
	if "in" != s.stats[1] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_TwoStep(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	out := s.GetNode("out")
	if err := out.Stat(s); err != nil {
		t.Fatal(err)
	}
	if 1 != len(s.stats) {
		t.Fatal("expected equal")
	}
	s.scan.RecomputeDirty(out, nil, nil)
	if 3 != len(s.stats) {
		t.Fatal("expected equal")
	}
	if "out" != s.stats[0] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("out").Dirty {
		t.Fatal("expected true")
	}
	if "mid" != s.stats[1] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("mid").Dirty {
		t.Fatal("expected true")
	}
	if "in" != s.stats[2] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_Tree(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state, "build out: cat mid1 mid2\nbuild mid1: cat in11 in12\nbuild mid2: cat in21 in22\n", ManifestParserOptions{})

	out := s.GetNode("out")
	if err := out.Stat(s); err != nil {
		t.Fatal(err)
	}
	if 1 != len(s.stats) {
		t.Fatal("expected equal")
	}
	s.scan.RecomputeDirty(out, nil, nil)
	if 1+6 != len(s.stats) {
		t.Fatal("expected equal")
	}
	if "mid1" != s.stats[1] {
		t.Fatal("expected equal")
	}
	if !s.GetNode("mid1").Dirty {
		t.Fatal("expected true")
	}
	if "in11" != s.stats[2] {
		t.Fatal("expected equal")
	}
}

func TestStatTest_Middle(t *testing.T) {
	s := NewStatTest(t)
	s.AssertParse(&s.state, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	s.mtimes["in"] = 1
	s.mtimes["mid"] = 0 // missing
	s.mtimes["out"] = 1

	out := s.GetNode("out")
	if err := out.Stat(s); err != nil {
		t.Fatal(err)
	}
	if 1 != len(s.stats) {
		t.Fatal("expected equal")
	}
	s.scan.RecomputeDirty(out, nil, nil)
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
