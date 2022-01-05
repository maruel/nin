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

package nin

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

func NormalizeRelativeAndCheckNoError(t *testing.T, input, relativeTo string) string {
	result := ""
	err := ""
	normalizer := newIncludesNormalize(relativeTo)
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
	if "a" != NormalizeAndCheckNoError(t, absPath("a", &err)) {
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
	normalizer := newIncludesNormalize(".")
	if normalizer.Normalize(kLongInputString, &result, &err) {
		t.Fatal("expected false")
	}
	if "path too long" != err {
		t.Fatal("expected equal")
	}

	// Construct max size path having cwd prefix.
	// exactlyMaxPath = "$cwd\\a\\aaaa...aaaa\0";
	t.Skip("TODO")
	/*
			cwd := os.Getwd()
		  cwdLen := len(exactlyMaxPath)
		  ASSERT_LE(cwdLen + 3 + 1, maxPath)
		  exactlyMaxPath[cwdLen] = '\\'
		  exactlyMaxPath[cwdLen + 1] = 'a'
		  exactlyMaxPath[cwdLen + 2] = '\\'

		  exactlyMaxPath[cwdLen + 3] = 'a'

		  for int i = cwdLen + 4; i < maxPath; i++ {
		    if i > cwdLen + 4 && i < maxPath - 1 && i % 10 == 0 {
		      exactlyMaxPath[i] = '\\'
		    } else {
		      exactlyMaxPath[i] = 'a'
		    }
		  }

		  exactlyMaxPath[maxPath] = '\0'
		  if strlen(kExactlyMaxPath) != maxPath { t.Fatal("expected equal") }

		  string forwardSlashes(kExactlyMaxPath)
		  replace(forwardSlashes.begin(), forwardSlashes.end(), '\\', '/')
		  // Make sure a path that's exactly maxPath long is canonicalized.
		  if forwardSlashes.substr(cwdLen + 1) != NormalizeAndCheckNoError(t,kExactlyMaxPath) { t.Fatal("expected equal") }
	*/
}

func TestIncludesNormalize_ShortRelativeButTooLongAbsolutePath(t *testing.T) {
	result := ""
	err := ""
	normalizer := newIncludesNormalize(".")
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
		  // exactlyMaxPath = "aaaa\\aaaa...aaaa\0";
			exactlyMaxPath := [maxPath + 1]byte{}
		  for i := 0; i < maxPath; i++ {
		    if i < maxPath - 1 && i % 10 == 4 {
		      exactlyMaxPath[i] = '\\'
		    } else {
		      exactlyMaxPath[i] = 'a'
		    }
		  }
		  exactlyMaxPath[maxPath] = '\0'
		  if strlen(kExactlyMaxPath) != maxPath { t.Fatal("expected equal") }

		  // Make sure a path that's exactly maxPath long fails with a proper error.
		  if normalizer.Normalize(kExactlyMaxPath, &result, &err) { t.Fatal("expected false") }
		  if !err.find("GetFullPathName") != string::npos { t.Fatal("expected true") }
	*/
}
