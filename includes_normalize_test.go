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


func GetCurDir() string {
  char buf[_MAX_PATH]
  _getcwd(buf, sizeof(buf))
  vector<StringPiece> parts = SplitStringPiece(buf, '\\')
  return parts[parts.size() - 1].AsString()
}

func NormalizeAndCheckNoError(input string) string {
  string result, err
  IncludesNormalize normalizer(".")
  if normalizer.Normalize(input, &result, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  return result
}

func NormalizeRelativeAndCheckNoError(input string, relative_to string) string {
  string result, err
  IncludesNormalize normalizer(relative_to)
  if normalizer.Normalize(input, &result, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
  return result
}

func TestIncludesNormalize_Simple(t *testing.T) {
  if "b" != NormalizeAndCheckNoError("a\\..\\b") { t.FailNow() }
  if "b" != NormalizeAndCheckNoError("a\\../b") { t.FailNow() }
  if "a/b" != NormalizeAndCheckNoError("a\\.\\b") { t.FailNow() }
  if "a/b" != NormalizeAndCheckNoError("a\\./b") { t.FailNow() }
}

func TestIncludesNormalize_WithRelative(t *testing.T) {
  err := ""
  currentdir := GetCurDir()
  if "c" != NormalizeRelativeAndCheckNoError("a/b/c", "a/b") { t.FailNow() }
  if "a" != NormalizeAndCheckNoError(IncludesNormalize::AbsPath("a", &err)) { t.FailNow() }
  if "" != err { t.FailNow() }
  if string("../") + currentdir + string("/a") != NormalizeRelativeAndCheckNoError("a", "../b") { t.FailNow() }
  if string("../") + currentdir + string("/a/b") != NormalizeRelativeAndCheckNoError("a/b", "../c") { t.FailNow() }
  if "../../a" != NormalizeRelativeAndCheckNoError("a", "b/c") { t.FailNow() }
  if "." != NormalizeRelativeAndCheckNoError("a", "a") { t.FailNow() }
}

func TestIncludesNormalize_Case(t *testing.T) {
  if "b" != NormalizeAndCheckNoError("Abc\\..\\b") { t.FailNow() }
  if "BdEf" != NormalizeAndCheckNoError("Abc\\..\\BdEf") { t.FailNow() }
  if "A/b" != NormalizeAndCheckNoError("A\\.\\b") { t.FailNow() }
  if "a/b" != NormalizeAndCheckNoError("a\\./b") { t.FailNow() }
  if "A/B" != NormalizeAndCheckNoError("A\\.\\B") { t.FailNow() }
  if "A/B" != NormalizeAndCheckNoError("A\\./B") { t.FailNow() }
}

func TestIncludesNormalize_DifferentDrive(t *testing.T) {
  if "stuff.h" != NormalizeRelativeAndCheckNoError("p:\\vs08\\stuff.h", "p:\\vs08") { t.FailNow() }
  if "stuff.h" != NormalizeRelativeAndCheckNoError("P:\\Vs08\\stuff.h", "p:\\vs08") { t.FailNow() }
  if "p:/vs08/stuff.h" != NormalizeRelativeAndCheckNoError("p:\\vs08\\stuff.h", "c:\\vs08") { t.FailNow() }
  if "P:/vs08/stufF.h" != NormalizeRelativeAndCheckNoError( "P:\\vs08\\stufF.h", "D:\\stuff/things") { t.FailNow() }
  if "P:/vs08/stuff.h" != NormalizeRelativeAndCheckNoError( "P:/vs08\\stuff.h", "D:\\stuff/things") { t.FailNow() }
  if "P:/wee/stuff.h" != NormalizeRelativeAndCheckNoError("P:/vs08\\../wee\\stuff.h", "D:\\stuff/things") { t.FailNow() }
}

func TestIncludesNormalize_LongInvalidPath(t *testing.T) {
  const char kLongInputString[] =
      "C:\\Program Files (x86)\\Microsoft Visual Studio "
      "12.0\\VC\\INCLUDEwarning #31001: The dll for reading and writing the "
      "pdb (for example, mspdb110.dll) could not be found on your path. This "
      "is usually a configuration error. Compilation will continue using /Z7 "
      "instead of /Zi, but expect a similar error when you link your program."
  // Too long, won't be canonicalized. Ensure doesn't crash.
  string result, err
  IncludesNormalize normalizer(".")
  if ! normalizer.Normalize(kLongInputString, &result, &err) { t.FailNow() }
  if "path too long" != err { t.FailNow() }

  // Construct max size path having cwd prefix.
  // kExactlyMaxPath = "$cwd\\a\\aaaa...aaaa\0";
  char kExactlyMaxPath[_MAX_PATH + 1]
  if _getcwd(kExactlyMaxPath == sizeof kExactlyMaxPath), nil { t.FailNow() }

  cwd_len := strlen(kExactlyMaxPath)
  ASSERT_LE(cwd_len + 3 + 1, _MAX_PATH)
  kExactlyMaxPath[cwd_len] = '\\'
  kExactlyMaxPath[cwd_len + 1] = 'a'
  kExactlyMaxPath[cwd_len + 2] = '\\'

  kExactlyMaxPath[cwd_len + 3] = 'a'

  for int i = cwd_len + 4; i < _MAX_PATH; i++ {
    if i > cwd_len + 4 && i < _MAX_PATH - 1 && i % 10 == 0 {
      kExactlyMaxPath[i] = '\\'
    } else {
      kExactlyMaxPath[i] = 'a'
    }
  }

  kExactlyMaxPath[_MAX_PATH] = '\0'
  if strlen(kExactlyMaxPath) != _MAX_PATH { t.FailNow() }

  string forward_slashes(kExactlyMaxPath)
  replace(forward_slashes.begin(), forward_slashes.end(), '\\', '/')
  // Make sure a path that's exactly _MAX_PATH long is canonicalized.
  if forward_slashes.substr(cwd_len + 1) != NormalizeAndCheckNoError(kExactlyMaxPath) { t.FailNow() }
}

func TestIncludesNormalize_ShortRelativeButTooLongAbsolutePath(t *testing.T) {
  string result, err
  IncludesNormalize normalizer(".")
  // A short path should work
  if normalizer.Normalize("a", &result, &err) { t.FailNow() }
  if "" != err { t.FailNow() }

  // Construct max size path having cwd prefix.
  // kExactlyMaxPath = "aaaa\\aaaa...aaaa\0";
  char kExactlyMaxPath[_MAX_PATH + 1]
  for i := 0; i < _MAX_PATH; i++ {
    if i < _MAX_PATH - 1 && i % 10 == 4 {
      kExactlyMaxPath[i] = '\\'
    } else {
      kExactlyMaxPath[i] = 'a'
    }
  }
  kExactlyMaxPath[_MAX_PATH] = '\0'
  if strlen(kExactlyMaxPath) != _MAX_PATH { t.FailNow() }

  // Make sure a path that's exactly _MAX_PATH long fails with a proper error.
  if !normalizer.Normalize(kExactlyMaxPath, &result, &err) { t.FailNow() }
  if err.find("GetFullPathName") != string::npos { t.FailNow() }
}

