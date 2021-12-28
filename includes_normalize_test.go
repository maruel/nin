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
	"os"
	"strings"
	"testing"
)

func GetCurDir(t *testing.T) string {
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// TODO(maruel): Weird
	p := strings.Split(d, "\\")
	return p[len(p)-1]
}

func NormalizeAndCheckNoError(t *testing.T, input string) string {
	return NormalizeRelativeAndCheckNoError(t, input, ".")
}

func NormalizeRelativeAndCheckNoError(t *testing.T, input, relative_to string) string {
	result := ""
	err := ""
	normalizer := NewIncludesNormalize(relative_to)
	if !normalizer.Normalize(input, &result, &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	return result
}

func TestIncludesNormalize_Simple(t *testing.T) {
	if got := NormalizeAndCheckNoError(t, "a\\..\\b"); got != "b" {
		t.Fatalf("%q", got)
	}
	if "b" != NormalizeAndCheckNoError(t, "a\\../b") {
		t.Fatal("expected equal")
	}
	if "a/b" != NormalizeAndCheckNoError(t, "a\\.\\b") {
		t.Fatal("expected equal")
	}
	if "a/b" != NormalizeAndCheckNoError(t, "a\\./b") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_WithRelative(t *testing.T) {
	t.Skip("TODO")
	err := ""
	currentdir := GetCurDir(t)
	if "c" != NormalizeRelativeAndCheckNoError(t, "a/b/c", "a/b") {
		t.Fatal("expected equal")
	}
	if "a" != NormalizeAndCheckNoError(t, AbsPath("a", &err)) {
		t.Fatal("expected equal")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if string("../")+currentdir+string("/a") != NormalizeRelativeAndCheckNoError(t, "a", "../b") {
		t.Fatal("expected equal")
	}
	if string("../")+currentdir+string("/a/b") != NormalizeRelativeAndCheckNoError(t, "a/b", "../c") {
		t.Fatal("expected equal")
	}
	if "../../a" != NormalizeRelativeAndCheckNoError(t, "a", "b/c") {
		t.Fatal("expected equal")
	}
	if "." != NormalizeRelativeAndCheckNoError(t, "a", "a") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_Case(t *testing.T) {
	if "b" != NormalizeAndCheckNoError(t, "Abc\\..\\b") {
		t.Fatal("expected equal")
	}
	if "BdEf" != NormalizeAndCheckNoError(t, "Abc\\..\\BdEf") {
		t.Fatal("expected equal")
	}
	if "A/b" != NormalizeAndCheckNoError(t, "A\\.\\b") {
		t.Fatal("expected equal")
	}
	if "a/b" != NormalizeAndCheckNoError(t, "a\\./b") {
		t.Fatal("expected equal")
	}
	if "A/B" != NormalizeAndCheckNoError(t, "A\\.\\B") {
		t.Fatal("expected equal")
	}
	if "A/B" != NormalizeAndCheckNoError(t, "A\\./B") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_DifferentDrive(t *testing.T) {
	t.Skip("TODO")
	if "stuff.h" != NormalizeRelativeAndCheckNoError(t, "p:\\vs08\\stuff.h", "p:\\vs08") {
		t.Fatal("expected equal")
	}
	if "stuff.h" != NormalizeRelativeAndCheckNoError(t, "P:\\Vs08\\stuff.h", "p:\\vs08") {
		t.Fatal("expected equal")
	}
	if "p:/vs08/stuff.h" != NormalizeRelativeAndCheckNoError(t, "p:\\vs08\\stuff.h", "c:\\vs08") {
		t.Fatal("expected equal")
	}
	if "P:/vs08/stufF.h" != NormalizeRelativeAndCheckNoError(t, "P:\\vs08\\stufF.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
	if "P:/vs08/stuff.h" != NormalizeRelativeAndCheckNoError(t, "P:/vs08\\stuff.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
	if "P:/wee/stuff.h" != NormalizeRelativeAndCheckNoError(t, "P:/vs08\\../wee\\stuff.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_LongInvalidPath(t *testing.T) {
	kLongInputString := "C:\\Program Files (x86)\\Microsoft Visual Studio 12.0\\VC\\INCLUDEwarning #31001: The dll for reading and writing the pdb (for example, mspdb110.dll) could not be found on your path. This is usually a configuration error. Compilation will continue using /Z7 instead of /Zi, but expect a similar error when you link your program."
	// Too long, won't be canonicalized. Ensure doesn't crash.
	result := ""
	err := ""
	normalizer := NewIncludesNormalize(".")
	if normalizer.Normalize(kLongInputString, &result, &err) {
		t.Fatal("expected false")
	}
	if "path too long" != err {
		t.Fatal("expected equal")
	}

	// Construct max size path having cwd prefix.
	// kExactlyMaxPath = "$cwd\\a\\aaaa...aaaa\0";
	t.Skip("TODO")
	/*
	  char kExactlyMaxPath[_MAX_PATH + 1]
	  if _getcwd(kExactlyMaxPath == sizeof kExactlyMaxPath), nil { t.Fatal("expected different") }

	  cwd_len := len(kExactlyMaxPath)
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
	  if strlen(kExactlyMaxPath) != _MAX_PATH { t.Fatal("expected equal") }

	  string forward_slashes(kExactlyMaxPath)
	  replace(forward_slashes.begin(), forward_slashes.end(), '\\', '/')
	  // Make sure a path that's exactly _MAX_PATH long is canonicalized.
	  if forward_slashes.substr(cwd_len + 1) != NormalizeAndCheckNoError(t,kExactlyMaxPath) { t.Fatal("expected equal") }
	*/
}

func TestIncludesNormalize_ShortRelativeButTooLongAbsolutePath(t *testing.T) {
	result := ""
	err := ""
	normalizer := NewIncludesNormalize(".")
	// A short path should work
	if !normalizer.Normalize("a", &result, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}

	t.Skip("TODO")
	/*
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
	  if strlen(kExactlyMaxPath) != _MAX_PATH { t.Fatal("expected equal") }

	  // Make sure a path that's exactly _MAX_PATH long fails with a proper error.
	  if normalizer.Normalize(kExactlyMaxPath, &result, &err) { t.Fatal("expected false") }
	  if !err.find("GetFullPathName") != string::npos { t.Fatal("expected true") }
	*/
}
