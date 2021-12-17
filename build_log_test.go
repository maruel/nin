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


const char kTestFilename[] = "BuildLogTest-tempfile"

type BuildLogTest struct {
  virtual bool IsPathDead(string s) const { return false; }
}
  func (b *BuildLogTest) SetUp() {
    // In case a crashing test left a stale file behind.
    unlink(kTestFilename)
  }
  func (b *BuildLogTest) TearDown() {
    unlink(kTestFilename)
  }

func TestBuildLogTest_WriteRead(t *testing.T) {
  AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n")

  var log1 BuildLog
  err := ""
  if log1.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  log1.RecordCommand(state_.edges_[0], 15, 18)
  log1.RecordCommand(state_.edges_[1], 20, 25)
  log1.Close()

  var log2 BuildLog
  if log2.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  if 2u != log1.entries().size() { t.FailNow() }
  if 2u != log2.entries().size() { t.FailNow() }
  BuildLog::LogEntry* e1 = log1.LookupByOutput("out")
  if e1 { t.FailNow() }
  BuildLog::LogEntry* e2 = log2.LookupByOutput("out")
  if e2 { t.FailNow() }
  if *e1 == *e2 { t.FailNow() }
  if 15 != e1.start_time { t.FailNow() }
  if "out" != e1.output { t.FailNow() }
}

func TestBuildLogTest_FirstWriteAddsSignature(t *testing.T) {
  const char kExpectedVersion[] = "# ninja log vX\n"
  const size_t kVersionPos = strlen(kExpectedVersion) - 2  // Points at 'X'.

  var log BuildLog
  string contents, err

  if log.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  log.Close()

  if 0 != ReadFile(kTestFilename, &contents, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if contents.size() >= kVersionPos {
    contents[kVersionPos] = 'X'
  }
  if kExpectedVersion != contents { t.FailNow() }

  // Opening the file anew shouldn't add a second version string.
  if log.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  log.Close()

  contents = nil
  if 0 != ReadFile(kTestFilename, &contents, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if contents.size() >= kVersionPos {
    contents[kVersionPos] = 'X'
  }
  if kExpectedVersion != contents { t.FailNow() }
}

func TestBuildLogTest_DoubleEntry(t *testing.T) {
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v4\n")
  fprintf(f, "0\t1\t2\tout\tcommand abc\n")
  fprintf(f, "3\t4\t5\tout\tcommand def\n")
  fclose(f)

  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  BuildLog::LogEntry* e = log.LookupByOutput("out")
  if e { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash("command def", e.command_hash))
}

func TestBuildLogTest_Truncate(t *testing.T) {
  AssertParse(&state_, "build out: cat mid\n" "build mid: cat in\n")

  {
    var log1 BuildLog
    err := ""
    if log1.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    log1.RecordCommand(state_.edges_[0], 15, 18)
    log1.RecordCommand(state_.edges_[1], 20, 25)
    log1.Close()
  }

  var statbuf stat
  if 0 != stat(kTestFilename, &statbuf) { t.FailNow() }
  if statbuf.st_size <= 0 { t.FailNow() }

  // For all possible truncations of the input file, assert that we don't
  // crash when parsing.
  for size := statbuf.st_size; size > 0; size-- {
    var log2 BuildLog
    err := ""
    if log2.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
    if "" != err { t.FailNow() }
    log2.RecordCommand(state_.edges_[0], 15, 18)
    log2.RecordCommand(state_.edges_[1], 20, 25)
    log2.Close()

    if Truncate(kTestFilename, size, &err) { t.FailNow() }

    var log3 BuildLog
    err = nil
    if log3.Load(kTestFilename, &err) == LOAD_SUCCESS || !err.empty() { t.FailNow() }
  }
}

func TestBuildLogTest_ObsoleteOldVersion(t *testing.T) {
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v3\n")
  fprintf(f, "123 456 0 out command\n")
  fclose(f)

  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if err.find("version") == string::npos { t.FailNow() }
}

func TestBuildLogTest_SpacesInOutputV4(t *testing.T) {
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v4\n")
  fprintf(f, "123\t456\t456\tout with space\tcommand\n")
  fclose(f)

  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  BuildLog::LogEntry* e = log.LookupByOutput("out with space")
  if e { t.FailNow() }
  if 123 != e.start_time { t.FailNow() }
  if 456 != e.end_time { t.FailNow() }
  if 456 != e.mtime { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash("command", e.command_hash))
}

func TestBuildLogTest_DuplicateVersionHeader(t *testing.T) {
  // Old versions of ninja accidentally wrote multiple version headers to the
  // build log on Windows. This shouldn't crash, and the second version header
  // should be ignored.
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v4\n")
  fprintf(f, "123\t456\t456\tout\tcommand\n")
  fprintf(f, "# ninja log v4\n")
  fprintf(f, "456\t789\t789\tout2\tcommand2\n")
  fclose(f)

  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  BuildLog::LogEntry* e = log.LookupByOutput("out")
  if e { t.FailNow() }
  if 123 != e.start_time { t.FailNow() }
  if 456 != e.end_time { t.FailNow() }
  if 456 != e.mtime { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash("command", e.command_hash))

  e = log.LookupByOutput("out2")
  if e { t.FailNow() }
  if 456 != e.start_time { t.FailNow() }
  if 789 != e.end_time { t.FailNow() }
  if 789 != e.mtime { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash("command2", e.command_hash))
}

type TestDiskInterface struct {
}
  func (t *TestDiskInterface) Stat(path string, err *string) TimeStamp {
    return 4
  }
  func (t *TestDiskInterface) WriteFile(path string, contents string) bool {
    if !false { panic("oops") }
    return true
  }
  func (t *TestDiskInterface) MakeDir(path string) bool {
    if !false { panic("oops") }
    return false
  }
  func (t *TestDiskInterface) ReadFile(path string, contents *string, err *string) Status {
    if !false { panic("oops") }
    return NotFound
  }
  func (t *TestDiskInterface) RemoveFile(path string) int {
    if !false { panic("oops") }
    return 0
  }

func TestBuildLogTest_Restat(t *testing.T) {
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v4\n" "1\t2\t3\tout\tcommand\n")
  fclose(f)
  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  BuildLog::LogEntry* e = log.LookupByOutput("out")
  if 3 != e.mtime { t.FailNow() }

  var testDiskInterface TestDiskInterface
  char out2[] = { 'o', 'u', 't', '2', 0 }
  char* filter2[] = { out2 }
  if log.Restat(kTestFilename, testDiskInterface, 1, filter2, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  e = log.LookupByOutput("out")
  if 3 != e.mtime { t.FailNow() } // unchanged, since the filter doesn't match

  if log.Restat(kTestFilename, testDiskInterface, 0, nil, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  e = log.LookupByOutput("out")
  if 4 != e.mtime { t.FailNow() }
}

func TestBuildLogTest_VeryLongInputLine(t *testing.T) {
  // Ninja's build log buffer is currently 256kB. Lines longer than that are
  // silently ignored, but don't affect parsing of other lines.
  FILE* f = fopen(kTestFilename, "wb")
  fprintf(f, "# ninja log v4\n")
  fprintf(f, "123\t456\t456\tout\tcommand start")
  for i := 0; i < (512 << 10) / strlen(" more_command"); i++ {
    fputs(" more_command", f)
  }
  fprintf(f, "\n")
  fprintf(f, "456\t789\t789\tout2\tcommand2\n")
  fclose(f)

  err := ""
  var log BuildLog
  if log.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  BuildLog::LogEntry* e = log.LookupByOutput("out")
  if nil != e { t.FailNow() }

  e = log.LookupByOutput("out2")
  if e { t.FailNow() }
  if 456 != e.start_time { t.FailNow() }
  if 789 != e.end_time { t.FailNow() }
  if 789 != e.mtime { t.FailNow() }
  ASSERT_NO_FATAL_FAILURE(AssertHash("command2", e.command_hash))
}

func TestBuildLogTest_MultiTargetEdge(t *testing.T) {
  AssertParse(&state_, "build out out.d: cat\n")

  var log BuildLog
  log.RecordCommand(state_.edges_[0], 21, 22)

  if 2u != log.entries().size() { t.FailNow() }
  BuildLog::LogEntry* e1 = log.LookupByOutput("out")
  if e1 { t.FailNow() }
  BuildLog::LogEntry* e2 = log.LookupByOutput("out.d")
  if e2 { t.FailNow() }
  if "out" != e1.output { t.FailNow() }
  if "out.d" != e2.output { t.FailNow() }
  if 21 != e1.start_time { t.FailNow() }
  if 21 != e2.start_time { t.FailNow() }
  if 22 != e2.end_time { t.FailNow() }
  if 22 != e2.end_time { t.FailNow() }
}

type BuildLogRecompactTest struct {
  virtual bool IsPathDead(string s) const { return s == "out2"; }
}

func TestBuildLogRecompactTest_Recompact(t *testing.T) {
  AssertParse(&state_, "build out: cat in\n" "build out2: cat in\n")

  var log1 BuildLog
  err := ""
  if log1.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  // Record the same edge several times, to trigger recompaction
  // the next time the log is opened.
  for i := 0; i < 200; i++ {
    log1.RecordCommand(state_.edges_[0], 15, 18 + i)
  }
  log1.RecordCommand(state_.edges_[1], 21, 22)
  log1.Close()

  // Load...
  var log2 BuildLog
  if log2.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 2u != log2.entries().size() { t.FailNow() }
  if log2.LookupByOutput("out") { t.FailNow() }
  if log2.LookupByOutput("out2") { t.FailNow() }
  // ...and force a recompaction.
  if log2.OpenForWrite(kTestFilename, *this, &err) { t.FailNow() }
  log2.Close()

  // "out2" is dead, it should've been removed.
  var log3 BuildLog
  if log2.Load(kTestFilename, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  if 1u != log2.entries().size() { t.FailNow() }
  if log2.LookupByOutput("out") { t.FailNow() }
  if !log2.LookupByOutput("out2") { t.FailNow() }
}

