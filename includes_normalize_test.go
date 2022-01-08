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

func getCurDir(t *testing.T) string {
	d, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// TODO(maruel): Weird
	p := strings.Split(d, "\\")
	return p[len(p)-1]
}

func normalizeAndCheckNoError(t *testing.T, input string) string {
	return normalizeRelativeAndCheckNoError(t, input, ".")
}

func normalizeRelativeAndCheckNoError(t *testing.T, input, relativeTo string) string {
	normalizer, err := newIncludesNormalize(relativeTo)
	if err != nil {
		t.Fatal(err)
	}
	result, err := normalizer.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestIncludesNormalize_Simple(t *testing.T) {
	if got := normalizeAndCheckNoError(t, "a\\..\\b"); got != "b" {
		t.Fatalf("%q", got)
	}
	if "b" != normalizeAndCheckNoError(t, "a\\../b") {
		t.Fatal("expected equal")
	}
	if "a/b" != normalizeAndCheckNoError(t, "a\\.\\b") {
		t.Fatal("expected equal")
	}
	if "a/b" != normalizeAndCheckNoError(t, "a\\./b") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_WithRelative(t *testing.T) {
	t.Skip("TODO")
	currentdir := getCurDir(t)
	if "c" != normalizeRelativeAndCheckNoError(t, "a/b/c", "a/b") {
		t.Fatal("expected equal")
	}
	a, err := absPath("a")
	if err != nil {
		t.Fatal(err)
	}
	if "a" != normalizeAndCheckNoError(t, a) {
		t.Fatal("expected equal")
	}
	if string("../")+currentdir+string("/a") != normalizeRelativeAndCheckNoError(t, "a", "../b") {
		t.Fatal("expected equal")
	}
	if string("../")+currentdir+string("/a/b") != normalizeRelativeAndCheckNoError(t, "a/b", "../c") {
		t.Fatal("expected equal")
	}
	if "../../a" != normalizeRelativeAndCheckNoError(t, "a", "b/c") {
		t.Fatal("expected equal")
	}
	if "." != normalizeRelativeAndCheckNoError(t, "a", "a") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_Case(t *testing.T) {
	if "b" != normalizeAndCheckNoError(t, "Abc\\..\\b") {
		t.Fatal("expected equal")
	}
	if "BdEf" != normalizeAndCheckNoError(t, "Abc\\..\\BdEf") {
		t.Fatal("expected equal")
	}
	if "A/b" != normalizeAndCheckNoError(t, "A\\.\\b") {
		t.Fatal("expected equal")
	}
	if "a/b" != normalizeAndCheckNoError(t, "a\\./b") {
		t.Fatal("expected equal")
	}
	if "A/B" != normalizeAndCheckNoError(t, "A\\.\\B") {
		t.Fatal("expected equal")
	}
	if "A/B" != normalizeAndCheckNoError(t, "A\\./B") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_DifferentDrive(t *testing.T) {
	t.Skip("TODO")
	if "stuff.h" != normalizeRelativeAndCheckNoError(t, "p:\\vs08\\stuff.h", "p:\\vs08") {
		t.Fatal("expected equal")
	}
	if "stuff.h" != normalizeRelativeAndCheckNoError(t, "P:\\Vs08\\stuff.h", "p:\\vs08") {
		t.Fatal("expected equal")
	}
	if "p:/vs08/stuff.h" != normalizeRelativeAndCheckNoError(t, "p:\\vs08\\stuff.h", "c:\\vs08") {
		t.Fatal("expected equal")
	}
	if "P:/vs08/stufF.h" != normalizeRelativeAndCheckNoError(t, "P:\\vs08\\stufF.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
	if "P:/vs08/stuff.h" != normalizeRelativeAndCheckNoError(t, "P:/vs08\\stuff.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
	if "P:/wee/stuff.h" != normalizeRelativeAndCheckNoError(t, "P:/vs08\\../wee\\stuff.h", "D:\\stuff/things") {
		t.Fatal("expected equal")
	}
}

func TestIncludesNormalize_LongInvalidPath(t *testing.T) {
	longInputString := "C:\\Program Files (x86)\\Microsoft Visual Studio 12.0\\VC\\INCLUDEwarning #31001: The dll for reading and writing the pdb (for example, mspdb110.dll) could not be found on your path. This is usually a configuration error. Compilation will continue using /Z7 instead of /Zi, but expect a similar error when you link your program."
	// Too long, won't be canonicalized. Ensure doesn't crash.
	normalizer, err := newIncludesNormalize(".")
	if err != nil {
		t.Fatal(err)
	}
	_, err = normalizer.Normalize(longInputString)
	if err == nil {
		t.Fatal("expected false")
	}
	if err.Error() != "path too long" {
		t.Fatal(err)
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
		  if forwardSlashes.substr(cwdLen + 1) != normalizeAndCheckNoError(t,kExactlyMaxPath) { t.Fatal("expected equal") }
	*/
}

func TestIncludesNormalize_ShortRelativeButTooLongAbsolutePath(t *testing.T) {
	normalizer, err := newIncludesNormalize(".")
	if err != nil {
		t.Fatal(err)
	}
	// A short path should work
	_, err = normalizer.Normalize("a")
	if err != nil {
		t.Fatal(err)
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
