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

package ginja

import (
	"runtime"
	"testing"
)

func testCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd /c dir \\"
	}
	return "ls /"
}

func NewSubprocessSetTest(t *testing.T) SubprocessSet {
	s := NewSubprocessSet()
	t.Cleanup(s.Clear)
	return s
}

// Run a command that fails and emits to stderr.
func TestSubprocessTest_BadCommandStderr(t *testing.T) {
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("cmd /c ninja_no_such_command", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		// Pretend we discovered that stderr was ready for writing.
		subprocs_.DoWork()
	}

	// ExitFailure
	// Returns 127 on posix and 1 on Windows.
	if got := subproc.Finish(); got != 127 && got != 1 {
		t.Fatal(got)
	}
	if "" == subproc.GetOutput() {
		t.Fatal("expected different")
	}
}

// Run a command that does not exist
func TestSubprocessTest_NoSuchCommand(t *testing.T) {
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("ninja_no_such_command", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		// Pretend we discovered that stderr was ready for writing.
		subprocs_.DoWork()
	}

	// ExitFailure
	// 127 on posix, -1 on Windows.
	if got := subproc.Finish(); got != 127 && got != -1 {
		t.Fatal(got)
	}
	if got := subproc.GetOutput(); got != "" {
		t.Fatalf("%q", got)
	}
	/*
		if runtime.GOOS == "windows" {
			if "CreateProcess failed: The system cannot find the file specified.\n" != subproc.GetOutput() {
				t.Fatal()
			}
		}
	*/
}

func TestSubprocessTest_InterruptChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	t.Skip("TODO")
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -INT $$", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		subprocs_.DoWork()
	}

	// ExitInterrupted
	if got := subproc.Finish(); got != -1 {
		t.Fatal(got)
	}
}

func TestSubprocessTest_InterruptParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	// I'm not sure how to handle this test case, it's kind of flaky. I may use
	// go run with two specialized processes instead.
	t.Skip("TODO")
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -INT $PPID ; sleep 1", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		if subprocs_.DoWork() {
			return
		}
	}

	t.Fatal("We should have been interrupted")
}

func TestSubprocessTest_InterruptChildWithSigTerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	t.Skip("TODO")
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -TERM $$", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		subprocs_.DoWork()
	}

	if ExitInterrupted != subproc.Finish() {
		t.Fatal("expected equal")
	}
}

func TestSubprocessTest_InterruptParentWithSigTerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	t.Skip("TODO")
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -TERM $PPID ; sleep 1", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		if subprocs_.DoWork() {
			return
		}
	}

	t.Fatal("We should have been interrupted")
}

func TestSubprocessTest_InterruptChildWithSigHup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	t.Skip("TODO")
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -HUP $$", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		subprocs_.DoWork()
	}

	if ExitInterrupted != subproc.Finish() {
		t.Fatal("expected equal")
	}
}

func TestSubprocessTest_InterruptParentWithSigHup(t *testing.T) {
	t.Skip("TODO")
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("kill -HUP $PPID ; sleep 1", false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		if subprocs_.DoWork() {
			return
		}
	}

	t.Fatal("We should have been interrupted")
}

func TestSubprocessTest_Console(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("can't run on Windows")
	}
	t.Skip("TODO")
	/*
		// Skip test if we don't have the console ourselves.
		// TODO(maruel): Sub-run with a fake pty?
		if !isatty(0) || !isatty(1) || !isatty(2) {
			t.Skip("need a real console to run this test")
		}
	*/
	subprocs_ := NewSubprocessSetTest(t)
	// use_console = true
	subproc := subprocs_.Add("test -t 0 -a -t 1 -a -t 2", true)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		subprocs_.DoWork()
	}

	if got := subproc.Finish(); got != ExitSuccess {
		t.Fatal(got)
	}
}

func TestSubprocessTest_SetWithSingle(t *testing.T) {
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add(testCommand(), false)
	if nil == subproc {
		t.Fatal("expected different")
	}

	for !subproc.Done() {
		subprocs_.DoWork()
	}
	if ExitSuccess != subproc.Finish() {
		t.Fatal("expected equal")
	}
	if "" == subproc.GetOutput() {
		t.Fatal("expected different")
	}

	if got := subprocs_.Finished(); got != 1 {
		t.Fatal(got)
	}
}

func TestSubprocessTest_SetWithMulti(t *testing.T) {
	processes := [3]Subprocess{}
	commands := []string{testCommand()}
	if runtime.GOOS == "windows" {
		commands = append(commands, "cmd /c echo hi", "cmd /c time /t")
	} else {
		commands = append(commands, "id -u", "pwd")
	}

	subprocs_ := NewSubprocessSetTest(t)
	for i := 0; i < 3; i++ {
		processes[i] = subprocs_.Add(commands[i], false)
		if nil == processes[i] {
			t.Fatal("expected different")
		}
	}

	if 3 != subprocs_.Running() {
		t.Fatal("expected equal")
	}
	/* The expectations with the C++ code is different.
	for i := 0; i < 3; i++ {
		if processes[i].Done() {
			t.Fatal("expected false")
		}
		if got := processes[i].GetOutput(); got != "" {
			t.Fatalf("%q", got)
		}
	}
	*/

	for !processes[0].Done() || !processes[1].Done() || !processes[2].Done() {
		if subprocs_.Running() <= 0 {
			t.Fatal("expected greater")
		}
		subprocs_.DoWork()
	}

	if 0 != subprocs_.Running() {
		t.Fatal("expected equal")
	}
	if 3 != subprocs_.Finished() {
		t.Fatal("expected equal")
	}

	for i := 0; i < 3; i++ {
		if ExitSuccess != processes[i].Finish() {
			t.Fatal("expected equal")
		}
		if "" == processes[i].GetOutput() {
			t.Fatal("expected different")
		}
		processes[i].Close()
	}
}

func TestSubprocessTest_SetWithLots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipped on windows")
	}
	// TODO(maruel): This test takes 780ms on my workstation, which is way too
	// high. The C++ version takes ~90ms. This means that the process creation
	// logic has to be dramatically optimized.

	// Arbitrary big number; needs to be over 1024 to confirm we're no longer
	// hostage to pselect.
	kNumProcs := 1025

	/*
	  // Make sure [ulimit -n] isn't going to stop us from working.
	  var rlim rlimit
	  if 0 != getrlimit(RLIMIT_NOFILE, &rlim) { t.Fatal("expected equal") }
	  if rlim.rlim_cur < kNumProcs {
	    fmt.Printf("Raise [ulimit -n] above %u (currently %lu) to make this test go\n", kNumProcs, rlim.rlim_cur)
	    return
	  }
	*/

	subprocs_ := NewSubprocessSetTest(t)
	var procs []Subprocess
	for i := 0; i < kNumProcs; i++ {
		subproc := subprocs_.Add("/bin/echo", false)
		if nil == subproc {
			t.Fatal("expected different")
		}
		procs = append(procs, subproc)
	}
	for subprocs_.Running() != 0 {
		subprocs_.DoWork()
	}
	for i := 0; i < len(procs); i++ {
		if got := procs[i].Finish(); got != ExitSuccess {
			t.Fatal(got)
		}
		if "" == procs[i].GetOutput() {
			t.Fatal("expected different")
		}
	}
	if kNumProcs != subprocs_.Finished() {
		t.Fatal("expected equal")
	}
}

// TODO: this test could work on Windows, just not sure how to simply
// read stdin.
// Verify that a command that attempts to read stdin correctly thinks
// that stdin is closed.
func TestSubprocessTest_ReadStdin(t *testing.T) {
	t.Skip("TODO")
	if runtime.GOOS == "windows" {
		t.Skip("Has to be ported")
	}
	subprocs_ := NewSubprocessSetTest(t)
	subproc := subprocs_.Add("cat -", false)
	for !subproc.Done() {
		subprocs_.DoWork()
	}
	if ExitSuccess != subproc.Finish() {
		t.Fatal("expected equal")
	}
	if 1 != subprocs_.Finished() {
		t.Fatal("expected equal")
	}
}
