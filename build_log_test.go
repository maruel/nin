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
	"bytes"
	"errors"
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
	b.AssertParse(&b.state, "build out: cat mid\nbuild mid: cat in\n", ParseManifestOpts{})

	log1 := NewBuildLog()
	defer log1.Close()
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	if err := log1.OpenForWrite(testFilename, b); err != nil {
		t.Fatal(err)
	}
	log1.RecordCommand(b.state.Edges[0], 15, 18, 0)
	log1.RecordCommand(b.state.Edges[1], 20, 25, 0)
	log1.Close()

	log2 := NewBuildLog()
	defer log2.Close()
	if s, err := log2.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}

	if 2 != len(log1.Entries) {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.Entries) {
		t.Fatal("expected equal")
	}
	e1 := log1.Entries["out"]
	if e1 == nil {
		t.Fatal("expected true")
	}
	e2 := log2.Entries["out"]
	if e2 == nil {
		t.Fatal("expected true")
	}
	if *e1 != *e2 {
		t.Fatal("expected true")
	}
	if 15 != e1.startTime {
		t.Fatal("expected equal")
	}
	if "out" != e1.output {
		t.Fatal("expected equal")
	}
}

func TestBuildLogTest_FirstWriteAddsSignature(t *testing.T) {
	b := NewBuildLogTest(t)
	// Bump when the version is changed.
	expectedVersion := []byte("# ninja log v5\n")

	log := NewBuildLog()
	defer log.Close()
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	if err := log.OpenForWrite(testFilename, b); err != nil {
		t.Fatal(err)
	}
	log.Close()

	contents, err2 := ioutil.ReadFile(testFilename)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(expectedVersion, contents) {
		t.Fatal(string(contents))
	}

	// Opening the file anew shouldn't add a second version string.
	if err := log.OpenForWrite(testFilename, b); err != nil {
		t.Fatal(err)
	}
	log.Close()

	contents, err2 = ioutil.ReadFile(testFilename)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(expectedVersion, contents) {
		t.Fatal(string(contents))
	}
}

func TestBuildLogTest_DoubleEntry(t *testing.T) {
	b := NewBuildLogTest(t)
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := "# ninja log v4\n0\t1\t2\tout\tcommand abc\n3\t4\t5\tout\tcommand def\n"
	if err := ioutil.WriteFile(testFilename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}

	e := log.Entries["out"]
	if e == nil {
		t.Fatal("expected true")
	}
	b.AssertHash("command def", e.commandHash)
}

func TestBuildLogTest_Truncate(t *testing.T) {
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state, "build out: cat mid\nbuild mid: cat in\n", ParseManifestOpts{})
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")

	{
		log1 := NewBuildLog()
		defer log1.Close()
		if err := log1.OpenForWrite(testFilename, b); err != nil {
			t.Fatal(err)
		}
		log1.RecordCommand(b.state.Edges[0], 15, 18, 0)
		log1.RecordCommand(b.state.Edges[1], 20, 25, 0)
		log1.Close()
	}

	// For all possible truncations of the input file, assert that we don't
	// crash when parsing.
	for size := getFileSize(t, testFilename); size > 0; size-- {
		log2 := NewBuildLog()
		defer log2.Close()
		if err := log2.OpenForWrite(testFilename, b); err != nil {
			t.Fatal(err)
		}
		log2.RecordCommand(b.state.Edges[0], 15, 18, 0)
		log2.RecordCommand(b.state.Edges[1], 20, 25, 0)
		log2.Close()

		if err := os.Truncate(testFilename, int64(size)); err != nil {
			t.Fatal(err)
		}

		log3 := NewBuildLog()
		defer log3.Close()
		if s, err := log3.Load(testFilename); s != LoadSuccess && err != nil {
			t.Fatal(s, err)
		}
		log3.Close()
	}
}

func TestBuildLogTest_ObsoleteOldVersion(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v3\n123 456 0 out command\n")
	if err := ioutil.WriteFile(testFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err == nil {
		t.Fatal(s, err)
	} else if !strings.Contains(err.Error(), "version") {
		t.Fatal(s, err)
	}
}

func TestBuildLogTest_SpacesInOutputV4(t *testing.T) {
	b := NewBuildLogTest(t)
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n123\t456\t456\tout with space\tcommand\n")
	if err := ioutil.WriteFile(testFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}

	e := log.Entries["out with space"]
	if e == nil {
		t.Fatal("expected true")
	}
	if 123 != e.startTime {
		t.Fatal("expected equal")
	}
	if 456 != e.endTime {
		t.Fatal("expected equal")
	}
	if 456 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command", e.commandHash)
}

func TestBuildLogTest_DuplicateVersionHeader(t *testing.T) {
	b := NewBuildLogTest(t)
	// Old versions of ninja accidentally wrote multiple version headers to the
	// build log on Windows. This shouldn't crash, and the second version header
	// should be ignored.
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n123\t456\t456\tout\tcommand\n# ninja log v4\n456\t789\t789\tout2\tcommand2\n")
	if err := ioutil.WriteFile(testFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}

	e := log.Entries["out"]
	if e == nil {
		t.Fatal("expected true")
	}
	if 123 != e.startTime {
		t.Fatal("expected equal")
	}
	if 456 != e.endTime {
		t.Fatal("expected equal")
	}
	if 456 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command", e.commandHash)

	e = log.Entries["out2"]
	if e == nil {
		t.Fatal("expected true")
	}
	if 456 != e.startTime {
		t.Fatal("expected equal")
	}
	if 789 != e.endTime {
		t.Fatal("expected equal")
	}
	if 789 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command2", e.commandHash)
}

type TestDiskInterface struct {
	t *testing.T
}

func (t *TestDiskInterface) Stat(path string) (TimeStamp, error) {
	return 4, nil
}
func (t *TestDiskInterface) WriteFile(path string, contents string) error {
	t.t.Fatal("Should not be reached")
	return errors.New("not implemented")
}
func (t *TestDiskInterface) MakeDir(path string) error {
	t.t.Fatal("Should not be reached")
	return errors.New("not implemented")
}
func (t *TestDiskInterface) ReadFile(path string) ([]byte, error) {
	t.t.Fatal("Should not be reached")
	return nil, errors.New("not implemented")
}
func (t *TestDiskInterface) RemoveFile(path string) error {
	t.t.Fatal("Should not be reached")
	return errors.New("not implemented")
}

func TestBuildLogTest_Restat(t *testing.T) {
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	content := []byte("# ninja log v4\n1\t2\t3\tout\tcommand\n")
	if err := ioutil.WriteFile(testFilename, content, 0o600); err != nil {
		t.Fatal(err)
	}
	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}
	e := log.Entries["out"]
	if 3 != e.mtime {
		t.Fatal("expected equal")
	}

	// TODO(maruel): The original test case is broken.
	testDiskInterface := TestDiskInterface{t}
	if err := log.Restat(testFilename, &testDiskInterface, []string{"out2"}); err != nil {
		t.Fatal(err)
	}
	e = log.Entries["out"]
	if 3 != e.mtime {
		t.Fatal(e.mtime)
	} // unchanged, since the filter doesn't match

	if err := log.Restat(testFilename, &testDiskInterface, nil); err != nil {
		t.Fatal(err)
	}
	e = log.Entries["out"]
	if 4 != e.mtime {
		t.Fatal("expected equal")
	}
}

func TestBuildLogTest_VeryLongInputLine(t *testing.T) {
	b := NewBuildLogTest(t)
	// Ninja's build log buffer in C++ is currently 256kB. Lines longer than that
	// are silently ignored, but don't affect parsing of other lines.
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")
	f, err2 := os.OpenFile(testFilename, os.O_CREATE|os.O_WRONLY, 0o600)
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

	log := NewBuildLog()
	defer log.Close()
	if s, err := log.Load(testFilename); s != LoadSuccess && err != nil {
		t.Fatal(s, err)
	}

	// Difference from C++ version!
	// In the Go version, lines are not ignored.
	e := log.Entries["out"]
	if nil == e {
		t.Fatal("expected equal")
	}

	e = log.Entries["out2"]
	if e == nil {
		t.Fatal("expected true")
	}
	if 456 != e.startTime {
		t.Fatal("expected equal")
	}
	if 789 != e.endTime {
		t.Fatal("expected equal")
	}
	if 789 != e.mtime {
		t.Fatal("expected equal")
	}
	b.AssertHash("command2", e.commandHash)
}

func TestBuildLogTest_MultiTargetEdge(t *testing.T) {
	b := NewBuildLogTest(t)
	b.AssertParse(&b.state, "build out out.d: cat\n", ParseManifestOpts{})

	log := NewBuildLog()
	defer log.Close()
	log.RecordCommand(b.state.Edges[0], 21, 22, 0)

	if 2 != len(log.Entries) {
		t.Fatal("expected equal")
	}
	e1 := log.Entries["out"]
	if e1 == nil {
		t.Fatal("expected true")
	}
	e2 := log.Entries["out.d"]
	if e2 == nil {
		t.Fatal("expected true")
	}
	if "out" != e1.output {
		t.Fatal("expected equal")
	}
	if "out.d" != e2.output {
		t.Fatal("expected equal")
	}
	if 21 != e1.startTime {
		t.Fatal("expected equal")
	}
	if 21 != e2.startTime {
		t.Fatal("expected equal")
	}
	if 22 != e2.endTime {
		t.Fatal("expected equal")
	}
	if 22 != e2.endTime {
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
	b.AssertParse(&b.state, "build out: cat in\nbuild out2: cat in\n", ParseManifestOpts{})
	testFilename := filepath.Join(t.TempDir(), "BuildLogTest-tempfile")

	{
		log1 := NewBuildLog()
		defer log1.Close()
		if err := log1.OpenForWrite(testFilename, b); err != nil {
			t.Fatal(err)
		}
		// Record the same edge several times, to trigger recompaction
		// the next time the log is opened.
		for i := 0; i < 200; i++ {
			log1.RecordCommand(b.state.Edges[0], 15, int32(18+i), 0)
		}
		log1.RecordCommand(b.state.Edges[1], 21, 22, 0)
		log1.Close()
	}

	// Load...
	{
		log2 := NewBuildLog()
		defer log2.Close()
		if s, err := log2.Load(testFilename); s != LoadSuccess && err != nil {
			t.Fatal(s, err)
		}
		if 2 != len(log2.Entries) {
			t.Fatal("expected equal")
		}
		if log2.Entries["out"] == nil {
			t.Fatal("expected true")
		}
		if log2.Entries["out2"] == nil {
			t.Fatal("expected true")
		}
		// ...and force a recompaction.
		if err := log2.OpenForWrite(testFilename, b); err != nil {
			t.Fatal(err)
		}
		log2.Close()
	}

	// "out2" is dead, it should've been removed.
	{
		log3 := NewBuildLog()
		defer log3.Close()
		if s, err := log3.Load(testFilename); s != LoadSuccess && err != nil {
			t.Fatal(s, err)
		}
		if 1 != len(log3.Entries) {
			t.Fatalf("%#v", log3.Entries)
		}
		if log3.Entries["out"] == nil {
			t.Fatal("expected true")
		}
		if log3.Entries["out2"] != nil {
			t.Fatal("expected false")
		}
		log3.Close()
	}
}

func TestHashCommand(t *testing.T) {
	if got := HashCommand(cmdHashCommand); got != 0x7c3f62c6da547bcb {
		t.Fatal(got)
	}
	if got := HashCommand("short"); got != 0x6de374e6eaf07e1a {
		t.Fatal(got)
	}
}

var optGuardBenchmarkHashCommand uint64

// Found the command by printing the longest command ran when building
// ninja_test.
const cmdHashCommand = "rm -f build/libninja.a && ar crs build/libninja.a build/browse.o build/build.o build/build_log.o build/clean.o build/clparser.o build/debug_flags.o build/depfile_parser.o build/deps_log.o build/disk_interface.o build/dyndep.o build/dyndep_parser.o build/edit_distance.o build/eval_env.o build/graph.o build/graphviz.o build/json.o build/lexer.o build/line_printer.o build/manifest_parser.o build/metrics.o build/missing_deps.o build/parser.o build/state.o build/status.o build/string_piece_util.o build/util.o build/version.o build/subprocess-posix.o"

// BenchmarkHashCommand runs a benchmark against HashCommand() with both a
// large and a short string.
func BenchmarkHashCommand(b *testing.B) {
	b.ReportAllocs()
	v := optGuardBenchmarkHashCommand
	for i := 0; i < b.N; i++ {
		v += HashCommand(cmdHashCommand)
		v += HashCommand("short")
	}
	optGuardBenchmarkHashCommand = v
}
