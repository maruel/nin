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

package ginja


// SetWithLots need setrlimit.

string kSimpleCommand = "cmd /c dir \\"
string kSimpleCommand = "ls /"

type SubprocessTest struct {
  subprocs_ SubprocessSet
}

// Run a command that fails and emits to stderr.
func TestSubprocessTest_BadCommandStderr(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("cmd /c ninja_no_such_command")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    // Pretend we discovered that stderr was ready for writing.
    subprocs_.DoWork()
  }

  if ExitFailure != subproc.Finish() { t.FailNow() }
  if "" == subproc.GetOutput() { t.FailNow() }
}

// Run a command that does not exist
func TestSubprocessTest_NoSuchCommand(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("ninja_no_such_command")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    // Pretend we discovered that stderr was ready for writing.
    subprocs_.DoWork()
  }

  if ExitFailure != subproc.Finish() { t.FailNow() }
  if "" == subproc.GetOutput() { t.FailNow() }
  if "CreateProcess failed: The system cannot find the file " "specified.\n" != subproc.GetOutput() { t.FailNow() }
}

func TestSubprocessTest_InterruptChild(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -INT $$")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    subprocs_.DoWork()
  }

  if ExitInterrupted != subproc.Finish() { t.FailNow() }
}

func TestSubprocessTest_InterruptParent(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -INT $PPID ; sleep 1")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  if !"We should have been interrupted" { t.FailNow() }
}

func TestSubprocessTest_InterruptChildWithSigTerm(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -TERM $$")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    subprocs_.DoWork()
  }

  if ExitInterrupted != subproc.Finish() { t.FailNow() }
}

func TestSubprocessTest_InterruptParentWithSigTerm(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -TERM $PPID ; sleep 1")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  if !"We should have been interrupted" { t.FailNow() }
}

func TestSubprocessTest_InterruptChildWithSigHup(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -HUP $$")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    subprocs_.DoWork()
  }

  if ExitInterrupted != subproc.Finish() { t.FailNow() }
}

func TestSubprocessTest_InterruptParentWithSigHup(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("kill -HUP $PPID ; sleep 1")
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  if !"We should have been interrupted" { t.FailNow() }
}

func TestSubprocessTest_Console(t *testing.T) {
  // Skip test if we don't have the console ourselves.
  if isatty(0) && isatty(1) && isatty(2) {
    Subprocess* subproc =
        subprocs_.Add("test -t 0 -a -t 1 -a -t 2", /*use_console=*/true)
    if (Subprocess*)0 == subproc { t.FailNow() }

    while !subproc.Done() {
      subprocs_.DoWork()
    }

    if ExitSuccess != subproc.Finish() { t.FailNow() }
  }
}

func TestSubprocessTest_SetWithSingle(t *testing.T) {
  subproc := subprocs_.Add(kSimpleCommand)
  if (Subprocess *) 0 == subproc { t.FailNow() }

  while !subproc.Done() {
    subprocs_.DoWork()
  }
  if ExitSuccess != subproc.Finish() { t.FailNow() }
  if "" == subproc.GetOutput() { t.FailNow() }

  if 1u != subprocs_.finished_.size() { t.FailNow() }
}

func TestSubprocessTest_SetWithMulti(t *testing.T) {
  Subprocess* processes[3]
  string kCommands[3] = {
    kSimpleCommand,
    "cmd /c echo hi",
    "cmd /c time /t",
    "id -u",
    "pwd",
  }

  for i := 0; i < 3; i++ {
    processes[i] = subprocs_.Add(kCommands[i])
    if (Subprocess *) 0 == processes[i] { t.FailNow() }
  }

  if 3u != subprocs_.running_.size() { t.FailNow() }
  for i := 0; i < 3; i++ {
    if !processes[i].Done() { t.FailNow() }
    if "" != processes[i].GetOutput() { t.FailNow() }
  }

  while !processes[0].Done() || !processes[1].Done() || !processes[2].Done() {
    if subprocs_.running_.size() <= 0u { t.FailNow() }
    subprocs_.DoWork()
  }

  if 0u != subprocs_.running_.size() { t.FailNow() }
  if 3u != subprocs_.finished_.size() { t.FailNow() }

  for i := 0; i < 3; i++ {
    if ExitSuccess != processes[i].Finish() { t.FailNow() }
    if "" == processes[i].GetOutput() { t.FailNow() }
    delete processes[i]
  }
}

func TestSubprocessTest_SetWithLots(t *testing.T) {
  // Arbitrary big number; needs to be over 1024 to confirm we're no longer
  // hostage to pselect.
  kNumProcs := 1025

  // Make sure [ulimit -n] isn't going to stop us from working.
  var rlim rlimit
  if 0 != getrlimit(RLIMIT_NOFILE, &rlim) { t.FailNow() }
  if rlim.rlim_cur < kNumProcs {
    printf("Raise [ulimit -n] above %u (currently %lu) to make this test go\n", kNumProcs, rlim.rlim_cur)
    return
  }

  vector<Subprocess*> procs
  for i := 0; i < kNumProcs; i++ {
    Subprocess* subproc = subprocs_.Add("/bin/echo")
    if (Subprocess *) 0 == subproc { t.FailNow() }
    procs.push_back(subproc)
  }
  while !subprocs_.running_.empty() {
    subprocs_.DoWork()
  }
  for i := 0; i < procs.size(); i++ {
    if ExitSuccess != procs[i].Finish() { t.FailNow() }
    if "" == procs[i].GetOutput() { t.FailNow() }
  }
  if kNumProcs != subprocs_.finished_.size() { t.FailNow() }
}

// TODO: this test could work on Windows, just not sure how to simply
// read stdin.
// Verify that a command that attempts to read stdin correctly thinks
// that stdin is closed.
func TestSubprocessTest_ReadStdin(t *testing.T) {
  Subprocess* subproc = subprocs_.Add("cat -")
  while !subproc.Done() {
    subprocs_.DoWork()
  }
  if ExitSuccess != subproc.Finish() { t.FailNow() }
  if 1u != subprocs_.finished_.size() { t.FailNow() }
}

