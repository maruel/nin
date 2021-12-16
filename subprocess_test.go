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
  SubprocessSet subprocs_
}

// Run a command that fails and emits to stderr.
func TestSubprocessTest_BadCommandStderr(t *testing.T) {
  subproc := subprocs_.Add("cmd /c ninja_no_such_command")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    // Pretend we discovered that stderr was ready for writing.
    subprocs_.DoWork()
  }

  EXPECT_EQ(ExitFailure, subproc.Finish())
  EXPECT_NE("", subproc.GetOutput())
}

// Run a command that does not exist
func TestSubprocessTest_NoSuchCommand(t *testing.T) {
  subproc := subprocs_.Add("ninja_no_such_command")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    // Pretend we discovered that stderr was ready for writing.
    subprocs_.DoWork()
  }

  EXPECT_EQ(ExitFailure, subproc.Finish())
  EXPECT_NE("", subproc.GetOutput())
  ASSERT_EQ("CreateProcess failed: The system cannot find the file " "specified.\n", subproc.GetOutput())
}

func TestSubprocessTest_InterruptChild(t *testing.T) {
  subproc := subprocs_.Add("kill -INT $$")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    subprocs_.DoWork()
  }

  EXPECT_EQ(ExitInterrupted, subproc.Finish())
}

func TestSubprocessTest_InterruptParent(t *testing.T) {
  subproc := subprocs_.Add("kill -INT $PPID ; sleep 1")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  ASSERT_FALSE("We should have been interrupted")
}

func TestSubprocessTest_InterruptChildWithSigTerm(t *testing.T) {
  subproc := subprocs_.Add("kill -TERM $$")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    subprocs_.DoWork()
  }

  EXPECT_EQ(ExitInterrupted, subproc.Finish())
}

func TestSubprocessTest_InterruptParentWithSigTerm(t *testing.T) {
  subproc := subprocs_.Add("kill -TERM $PPID ; sleep 1")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  ASSERT_FALSE("We should have been interrupted")
}

func TestSubprocessTest_InterruptChildWithSigHup(t *testing.T) {
  subproc := subprocs_.Add("kill -HUP $$")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    subprocs_.DoWork()
  }

  EXPECT_EQ(ExitInterrupted, subproc.Finish())
}

func TestSubprocessTest_InterruptParentWithSigHup(t *testing.T) {
  subproc := subprocs_.Add("kill -HUP $PPID ; sleep 1")
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return
    }
  }

  ASSERT_FALSE("We should have been interrupted")
}

func TestSubprocessTest_Console(t *testing.T) {
  // Skip test if we don't have the console ourselves.
  if isatty(0) && isatty(1) && isatty(2) {
    Subprocess* subproc =
        subprocs_.Add("test -t 0 -a -t 1 -a -t 2", /*use_console=*/true)
    ASSERT_NE((Subprocess*)0, subproc)

    while (!subproc.Done()) {
      subprocs_.DoWork()
    }

    EXPECT_EQ(ExitSuccess, subproc.Finish())
  }
}

func TestSubprocessTest_SetWithSingle(t *testing.T) {
  subproc := subprocs_.Add(kSimpleCommand)
  ASSERT_NE((Subprocess *) 0, subproc)

  while (!subproc.Done()) {
    subprocs_.DoWork()
  }
  ASSERT_EQ(ExitSuccess, subproc.Finish())
  ASSERT_NE("", subproc.GetOutput())

  ASSERT_EQ(1u, subprocs_.finished_.size())
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

  for (int i = 0; i < 3; ++i) {
    processes[i] = subprocs_.Add(kCommands[i])
    ASSERT_NE((Subprocess *) 0, processes[i])
  }

  ASSERT_EQ(3u, subprocs_.running_.size())
  for (int i = 0; i < 3; ++i) {
    ASSERT_FALSE(processes[i].Done())
    ASSERT_EQ("", processes[i].GetOutput())
  }

  while (!processes[0].Done() || !processes[1].Done() || !processes[2].Done()) {
    ASSERT_GT(subprocs_.running_.size(), 0u)
    subprocs_.DoWork()
  }

  ASSERT_EQ(0u, subprocs_.running_.size())
  ASSERT_EQ(3u, subprocs_.finished_.size())

  for (int i = 0; i < 3; ++i) {
    ASSERT_EQ(ExitSuccess, processes[i].Finish())
    ASSERT_NE("", processes[i].GetOutput())
    delete processes[i]
  }
}

func TestSubprocessTest_SetWithLots(t *testing.T) {
  // Arbitrary big number; needs to be over 1024 to confirm we're no longer
  // hostage to pselect.
  const unsigned kNumProcs = 1025

  // Make sure [ulimit -n] isn't going to stop us from working.
  rlimit rlim
  ASSERT_EQ(0, getrlimit(RLIMIT_NOFILE, &rlim))
  if rlim.rlim_cur < kNumProcs {
    printf("Raise [ulimit -n] above %u (currently %lu) to make this test go\n", kNumProcs, rlim.rlim_cur)
    return
  }

  vector<Subprocess*> procs
  for (size_t i = 0; i < kNumProcs; ++i) {
    subproc := subprocs_.Add("/bin/echo")
    ASSERT_NE((Subprocess *) 0, subproc)
    procs.push_back(subproc)
  }
  while (!subprocs_.running_.empty())
    subprocs_.DoWork()
  for (size_t i = 0; i < procs.size(); ++i) {
    ASSERT_EQ(ExitSuccess, procs[i].Finish())
    ASSERT_NE("", procs[i].GetOutput())
  }
  ASSERT_EQ(kNumProcs, subprocs_.finished_.size())
}

// TODO: this test could work on Windows, just not sure how to simply
// read stdin.
// Verify that a command that attempts to read stdin correctly thinks
// that stdin is closed.
func TestSubprocessTest_ReadStdin(t *testing.T) {
  subproc := subprocs_.Add("cat -")
  while (!subproc.Done()) {
    subprocs_.DoWork()
  }
  ASSERT_EQ(ExitSuccess, subproc.Finish())
  ASSERT_EQ(1u, subprocs_.finished_.size())
}

