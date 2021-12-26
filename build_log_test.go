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
	"path/filepath"
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
	t.Skip("TODO")
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})

	var log1 BuildLog
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

	var log2 BuildLog
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
	t.Skip("TODO")
	b := NewBuildLogTest(t)
	//kExpectedVersion := "# ninja log vX\n"
	//kVersionPos := len(kExpectedVersion) - 2 // Points at 'X'.

	var log BuildLog
	//contents := ""
	err := ""
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	if !log.OpenForWrite(kTestFilename, b, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log.Close()

	t.Skip("TODO")
	/*
		if 0 != b.ReadFile(kTestFilename, &contents, &err) {
			t.Fatal("expected equal")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if len(contents) >= kVersionPos {
			contents[kVersionPos] = 'X'
		}
		if kExpectedVersion != contents {
			t.Fatal("expected equal")
		}

		// Opening the file anew shouldn't add a second version string.
		if !log.OpenForWrite(kTestFilename, b, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		log.Close()

		contents = ""
		if 0 != b.ReadFile(kTestFilename, &contents, &err) {
			t.Fatal("expected equal")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		if len(contents) >= kVersionPos {
			contents[kVersionPos] = 'X'
		}
		if kExpectedVersion != contents {
			t.Fatal("expected equal")
		}
	*/
}

func TestBuildLogTest_DoubleEntry(t *testing.T) {
	t.Skip("TODO")
	/*
			b := NewBuildLogTest(t)
			kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
		  FILE* f = fopen(kTestFilename, "wb")
		  fprintf(f, "# ninja log v4\n")
		  fprintf(f, "0\t1\t2\tout\tcommand abc\n")
		  fprintf(f, "3\t4\t5\tout\tcommand def\n")
		  fclose(f)

		  err := ""
		  var log BuildLog
		  if !log.Load(kTestFilename, &err) { t.Fatal("expected true") }
		  if "" != err { t.Fatal("expected equal") }

			e := log.LookupByOutput("out")
		  if !e { t.Fatal("expected true") }
		  AssertHash("command def", e.command_hash)
	*/
}

func TestBuildLogTest_Truncate(t *testing.T) {
	t.Skip("TODO")
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out: cat mid\nbuild mid: cat in\n", ManifestParserOptions{})
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")

	{
		var log1 BuildLog
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

	t.Skip("TODO")
	/*
		var statbuf stat
		if 0 != stat(kTestFilename, &statbuf) {
			t.Fatal("expected equal")
		}
		if statbuf.st_size <= 0 {
			t.Fatal("expected greater")
		}

		// For all possible truncations of the input file, assert that we don't
		// crash when parsing.
		for size := statbuf.st_size; size > 0; size-- {
			var log2 BuildLog
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

			if !Truncate(kTestFilename, size, &err) {
				t.Fatal("expected true")
			}

			var log3 BuildLog
			err = nil
			if !log3.Load(kTestFilename, &err) == LOAD_SUCCESS || !err.empty() {
				t.Fatal("expected true")
			}
		}
	*/
}

func TestBuildLogTest_ObsoleteOldVersion(t *testing.T) {
	t.Skip("TODO")
	/*
		b := NewBuildLogTest(t)
		kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
		FILE * f = fopen(kTestFilename, "wb")
		fprintf(f, "# ninja log v3\n")
		fprintf(f, "123 456 0 out command\n")
		fclose(f)

		err := ""
		var log BuildLog
		if !log.Load(kTestFilename, &err) {
			t.Fatal("expected true")
		}
		if err.find("version") == string::npos { t.Fatal("expected different") }
	*/
}

func TestBuildLogTest_SpacesInOutputV4(t *testing.T) {
	t.Skip("TODO")
	/*
		b := NewBuildLogTest(t)
		FILE * f = fopen(kTestFilename, "wb")
		fprintf(f, "# ninja log v4\n")
		fprintf(f, "123\t456\t456\tout with space\tcommand\n")
		fclose(f)

		err := ""
		var log BuildLog
		if !log.Load(kTestFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		e := log.LookupByOutput("out with space")
		if !e {
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
		AssertHash("command", e.command_hash)
	*/
}

func TestBuildLogTest_DuplicateVersionHeader(t *testing.T) {
	t.Skip("TODO")
	/*
		b := NewBuildLogTest(t)
		// Old versions of ninja accidentally wrote multiple version headers to the
		// build log on Windows. This shouldn't crash, and the second version header
		// should be ignored.
		FILE * f = fopen(kTestFilename, "wb")
		fprintf(f, "# ninja log v4\n")
		fprintf(f, "123\t456\t456\tout\tcommand\n")
		fprintf(f, "# ninja log v4\n")
		fprintf(f, "456\t789\t789\tout2\tcommand2\n")
		fclose(f)

		err := ""
		var log BuildLog
		if !log.Load(kTestFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		e := log.LookupByOutput("out")
		if !e {
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
		AssertHash("command", e.command_hash)

		e = log.LookupByOutput("out2")
		if !e {
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
		AssertHash("command2", e.command_hash)
	*/
}

type TestDiskInterface struct {
}

func (t *TestDiskInterface) Stat(path string, err *string) TimeStamp {
	return 4
}
func (t *TestDiskInterface) WriteFile(path string, contents string) bool {
	if !false {
		panic("oops")
	}
	return true
}
func (t *TestDiskInterface) MakeDir(path string) bool {
	if !false {
		panic("oops")
	}
	return false
}
func (t *TestDiskInterface) ReadFile(path string, contents *string, err *string) DiskStatus {
	if !false {
		panic("oops")
	}
	return NotFound
}
func (t *TestDiskInterface) RemoveFile(path string) int {
	if !false {
		panic("oops")
	}
	return 0
}

func TestBuildLogTest_Restat(t *testing.T) {
	t.Skip("TODO")
	/*
		b := NewBuildLogTest(t)
		FILE * f = fopen(kTestFilename, "wb")
		fprintf(f, "# ninja log v4\n1\t2\t3\tout\tcommand\n")
		fclose(f)
		err := ""
		var log BuildLog
		if !log.Load(kTestFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}
		e := log.LookupByOutput("out")
		if 3 != e.mtime {
			t.Fatal("expected equal")
		}

		var testDiskInterface TestDiskInterface
		out2 := []byte{'o', 'u', 't', '2', 0}
		filter2 := out2
		if !log.Restat(kTestFilename, testDiskInterface, 1, filter2, &err) { t.Fatal("expected true") }
		if "" != err { t.Fatal("expected equal") }
		e = log.LookupByOutput("out")
		if 3 != e.mtime { t.Fatal("expected equal") } // unchanged, since the filter doesn't match

		if !log.Restat(kTestFilename, testDiskInterface, 0, nil, &err) { t.Fatal("expected true") }
		if "" != err { t.Fatal("expected equal") }
		e = log.LookupByOutput("out")
		if 4 != e.mtime { t.Fatal("expected equal") }
	*/
}

func TestBuildLogTest_VeryLongInputLine(t *testing.T) {
	t.Skip("TODO")
	/*
		b := NewBuildLogTest(t)
		// Ninja's build log buffer is currently 256kB. Lines longer than that are
		// silently ignored, but don't affect parsing of other lines.
		FILE * f = fopen(kTestFilename, "wb")
		fprintf(f, "# ninja log v4\n")
		fprintf(f, "123\t456\t456\tout\tcommand start")
		for i := 0; i < (512<<10)/strlen(" more_command"); i++ {
			fputs(" more_command", f)
		}
		fprintf(f, "\n")
		fprintf(f, "456\t789\t789\tout2\tcommand2\n")
		fclose(f)

		err := ""
		var log BuildLog
		if !log.Load(kTestFilename, &err) {
			t.Fatal("expected true")
		}
		if "" != err {
			t.Fatal("expected equal")
		}

		e := log.LookupByOutput("out")
		if nil != e {
			t.Fatal("expected equal")
		}

		e = log.LookupByOutput("out2")
		if !e {
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
		AssertHash("command2", e.command_hash)
	*/
}

func TestBuildLogTest_MultiTargetEdge(t *testing.T) {
	t.Skip("TODO")
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out out.d: cat\n", ManifestParserOptions{})

	var log BuildLog
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
}

func (b *BuildLogRecompactTest) IsPathDead(s string) bool {
	return s == "out2"
}

func TestBuildLogRecompactTest_Recompact(t *testing.T) {
	t.Skip("TODO")
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state_, "build out: cat in\nbuild out2: cat in\n", ManifestParserOptions{})

	var log1 BuildLog
	err := ""
	kTestFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
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

	// Load...
	var log2 BuildLog
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

	// "out2" is dead, it should've been removed.
	var log3 BuildLog
	if log3.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 1 != len(log3.entries()) {
		t.Fatal("expected equal")
	}
	if log3.LookupByOutput("out") == nil {
		t.Fatal("expected true")
	}
	if log3.LookupByOutput("out2") != nil {
		t.Fatal("expected false")
	}
}
