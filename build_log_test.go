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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type BuildLogTest struct {
	StateTestWithBuiltinRules
}

func NewBuildLogTest(t *testing.T) *BuildLogTest {
	return &BuildLogTest{NewStateTestWithBuiltinRules(t)}
}

func (b *BuildLogTest) IsPathDead(s string) bool {
	return false
}

func TestBuildLogTest_WriteRead(t *testing.T) {
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	log1 := NewBuildLog()
	defer log1.Close()
	err := ""
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	if !log1.OpenForWrite(kTestFilename, b, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log1.RecordCommand(b.state_.edges_[0], 15, 18, 0)
	log1.RecordCommand(b.state_.edges_[1], 20, 25, 0)
	log1.Close()

	log2 := NewBuildLog()
	defer log2.Close()
	if log2.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	if 2 != len(log1.entries()) {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.entries()) {
		t.Fatal("expected equal")
	}
	e1 := log1.LookupByOutput("out")
	if e1 == nil {
		t.Fatal("expected true")
	}
	e2 := log2.LookupByOutput("out")
	if e2 == nil {
		t.Fatal("expected true")
	}
	if *e1 != *e2 {
		t.Fatal("expected true")
	}
	if 15 != e1.start_time {
		t.Fatal("expected equal")
	}
	if "out" != e1.output {
		t.Fatal("expected equal")
	}
}

func TestBuildLogTest_FirstWriteAddsSignature(t *testing.T) {
	b := NewBuildLogTest(t)
	// Bump when the version is changed.
	kExpectedVersion := []byte("# ninja log v5\n")

	log := NewBuildLog()
	defer log.Close()
	err := ""
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	if !log.OpenForWrite(kTestFilename, b, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log.Close()

	contents, err2 := ioutil.ReadFile(kTestFilename)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(kExpectedVersion, contents) {
		t.Fatal(string(contents))
	}

	// Opening the file anew shouldn't add a second version string.
	if !log.OpenForWrite(kTestFilename, b, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log.Close()

	contents, err2 = ioutil.ReadFile(kTestFilename)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(kExpectedVersion, contents) {
		t.Fatal(string(contents))
	}
}

func TestBuildLogTest_DoubleEntry(t *testing.T) {
	b := NewBuildLogTest(t)
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := "# ninja log v4\n0\t1\t2\tout\tcommand abc\n3\t4\t5\tout\tcommand def\n"
	if err := ioutil.WriteFile(kTestFilename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	e := log.LookupByOutput("out")
	if e == nil {
		t.Fatal("expected true")
	}
	b.AssertHash("command def", e.command_hash)
}

func TestBuildLogTest_Truncate(t *testing.T) {
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")

	{
		log1 := NewBuildLog()
		defer log1.Close()
		err := ""
		if !log1.OpenForWrite(kTestFilename, b, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		log1.RecordCommand(b.state_.edges_[0], 15, 18, 0)
		log1.RecordCommand(b.state_.edges_[1], 20, 25, 0)
		log1.Close()
	}

	// For all possible truncations of the input file, assert that we don't
	// crash when parsing.
	for size := getFileSize(t, kTestFilename); size > 0; size-- {
		log2 := NewBuildLog()
		defer log2.Close()
		err := ""
		if !log2.OpenForWrite(kTestFilename, b, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		log2.RecordCommand(b.state_.edges_[0], 15, 18, 0)
		log2.RecordCommand(b.state_.edges_[1], 20, 25, 0)
		log2.Close()

		if err := os.Truncate(kTestFilename, int64(size)); err != nil {
			t.Fatal(err)
		}

		log3 := NewBuildLog()
		defer log3.Close()
		err = ""
		if log3.Load(kTestFilename, &err) != LOAD_SUCCESS || err != "" {
			t.Fatal(err)
		}
		log3.Close()
	}
}

func TestBuildLogTest_ObsoleteOldVersion(t *testing.T) {
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v3\n123 456 0 out command\n")
	if err := ioutil.WriteFile(kTestFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal(err)
	}
	if !strings.Contains(err, "version") {
		t.Fatal("expected different")
	}
}

func TestBuildLogTest_SpacesInOutputV4(t *testing.T) {
	b := NewBuildLogTest(t)
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n123\t456\t456\tout with space\tcommand\n")
	if err := ioutil.WriteFile(kTestFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal(err)
	}

	e := log.LookupByOutput("out with space")
	if e == nil {
		t.Fatal("expected true")
	}
	if 123 != e.start_time {
		t.Fatal("expected equal")
	}
	if 456 != e.end_time {
		t.Fatal("expected equal")
	}
	if 456 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command", e.command_hash)
}

func TestBuildLogTest_DuplicateVersionHeader(t *testing.T) {
	b := NewBuildLogTest(t)
	// Old versions of ninja accidentally wrote multiple version headers to the
	// build log on Windows. This shouldn't crash, and the second version header
	// should be ignored.
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n123\t456\t456\tout\tcommand\n# ninja log v4\n456\t789\t789\tout2\tcommand2\n")
	if err := ioutil.WriteFile(kTestFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	e := log.LookupByOutput("out")
	if e == nil {
		t.Fatal("expected true")
	}
	if 123 != e.start_time {
		t.Fatal("expected equal")
	}
	if 456 != e.end_time {
		t.Fatal("expected equal")
	}
	if 456 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command", e.command_hash)

	e = log.LookupByOutput("out2")
	if e == nil {
		t.Fatal("expected true")
	}
	if 456 != e.start_time {
		t.Fatal("expected equal")
	}
	if 789 != e.end_time {
		t.Fatal("expected equal")
	}
	if 789 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command2", e.command_hash)
}

type TestDiskInterface struct {
	t *testing.T
}

func (t *TestDiskInterface) Stat(path string, err *string) TimeStamp {
	return 4
}
func (t *TestDiskInterface) WriteFile(path string, contents string) bool {
	t.t.Fatal("Should not be reached")
	return true
}
func (t *TestDiskInterface) MakeDir(path string) bool {
	t.t.Fatal("Should not be reached")
	return false
}
func (t *TestDiskInterface) ReadFile(path string, contents *string, err *string) DiskStatus {
	t.t.Fatal("Should not be reached")
	return NotFound
}
func (t *TestDiskInterface) RemoveFile(path string) int {
	t.t.Fatal("Should not be reached")
	return 0
}

func TestBuildLogTest_Restat(t *testing.T) {
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n1\t2\t3\tout\tcommand\n")
	if err := ioutil.WriteFile(kTestFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}
	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	e := log.LookupByOutput("out")
	if 3 != e.mtime {
		t.Fatal("expected equal")
	}

	// TODO(maruel): The original test case is broken.
	testDiskInterface := TestDiskInterface{t}
	if !log.Restat(kTestFilename, &testDiskInterface, []string{"out2"}, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	e = log.LookupByOutput("out")
	if 3 != e.mtime {
		t.Fatal(e.mtime)
	} // unchanged, since the filter doesn't match

	if !log.Restat(kTestFilename, &testDiskInterface, nil, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	e = log.LookupByOutput("out")
	if 4 != e.mtime {
		t.Fatal("expected equal")
	}
}

func TestBuildLogTest_VeryLongInputLine(t *testing.T) {
	b := NewBuildLogTest(t)
	// Ninja's build log buffer in C++ is currently 256kB. Lines longer than that
	// are silently ignored, but don't affect parsing of other lines.
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	f, err2 := os.OpenFile(kTestFilename, os.O_CREATE|os.O_WRONLY, 0o600)
	if err2 != nil {
		t.Fatal(err2)
	}
	fmt.Fprintf(f, "# ninja log v4\n")
	fmt.Fprintf(f, "123\t456\t456\tout\tcommand start")
	for i := 0; i < (512<<10)/len(" more_command"); i++ {
		f.WriteString(" more_command")
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "456\t789\t789\tout2\tcommand2\n")
	f.Close()

	err := ""
	log := NewBuildLog()
	defer log.Close()
	if log.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	// Difference from C++ version!
	// In the Go version, lines are not ignored.
	e := log.LookupByOutput("out")
	if nil == e {
		t.Fatal("expected equal")
	}

	e = log.LookupByOutput("out2")
	if e == nil {
		t.Fatal("expected true")
	}
	if 456 != e.start_time {
		t.Fatal("expected equal")
	}
	if 789 != e.end_time {
		t.Fatal("expected equal")
	}
	if 789 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command2", e.command_hash)
}

func TestBuildLogTest_MultiTargetEdge(t *testing.T) {
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out out.d: cat\n", ManifestParserOptions{})

	log := NewBuildLog()
	defer log.Close()
	log.RecordCommand(b.state_.edges_[0], 21, 22, 0)

	if 2 != len(log.entries()) {
		t.Fatal("expected equal")
	}
	e1 := log.LookupByOutput("out")
	if e1 == nil {
		t.Fatal("expected true")
	}
	e2 := log.LookupByOutput("out.d")
	if e2 == nil {
		t.Fatal("expected true")
	}
	if "out" != e1.output {
		t.Fatal("expected equal")
	}
	if "out.d" != e2.output {
		t.Fatal("expected equal")
	}
	if 21 != e1.start_time {
		t.Fatal("expected equal")
	}
	if 21 != e2.start_time {
		t.Fatal("expected equal")
	}
	if 22 != e2.end_time {
		t.Fatal("expected equal")
	}
	if 22 != e2.end_time {
		t.Fatal("expected equal")
	}
}

type BuildLogRecompactTest struct {
	*BuildLogTest
}

func (b *BuildLogRecompactTest) IsPathDead(s string) bool {
	return s == "out2"
}

func NewBuildLogRecompactTest(t *testing.T) *BuildLogRecompactTest {
	return &BuildLogRecompactTest{NewBuildLogTest(t)}
}

func TestBuildLogRecompactTest_Recompact(t *testing.T) {
	b := NewBuildLogRecompactTest(t)
	b.AssertParse(&b.state_, "build out: cat in\nbuild out2: cat in\n", ManifestParserOptions{})
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	err := ""

	{
		log1 := NewBuildLog()
		defer log1.Close()
		if !log1.OpenForWrite(kTestFilename, b, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		// Record the same edge several times, to trigger recompaction
		// the next time the log is opened.
		for i := 0; i < 200; i++ {
			log1.RecordCommand(b.state_.edges_[0], 15, int32(18+i), 0)
		}
		log1.RecordCommand(b.state_.edges_[1], 21, 22, 0)
		log1.Close()
	}

	// Load...
	{
		log2 := NewBuildLog()
		defer log2.Close()
		if log2.Load(kTestFilename, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if 2 != len(log2.entries()) {
			t.Fatal("expected equal")
		}
		if log2.LookupByOutput("out") == nil {
			t.Fatal("expected true")
		}
		if log2.LookupByOutput("out2") == nil {
			t.Fatal("expected true")
		}
		// ...and force a recompaction.
		if !log2.OpenForWrite(kTestFilename, b, &err) {
			t.Fatal("expected true")
		}
		log2.Close()
	}

	// "out2" is dead, it should've been removed.
	{
		log3 := NewBuildLog()
		defer log3.Close()
		if log3.Load(kTestFilename, &err) != LOAD_SUCCESS {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if 1 != len(log3.entries()) {
			t.Fatalf("%#v", log3.entries())
		}
		if log3.LookupByOutput("out") == nil {
			t.Fatal("expected true")
		}
		if log3.LookupByOutput("out2") != nil {
			t.Fatal("expected false")
		}
		log3.Close()
	}
}
