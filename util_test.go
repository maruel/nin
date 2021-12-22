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
	"runtime"
	"strconv"
	"testing"
)

func TestCanonicalizePath_PathSamples(t *testing.T) {
	type row struct {
		in   string
		want string
	}
	data := []row{
		{"", ""},
		{"foo.h", "foo.h"},
		{"./foo.h", "foo.h"},
		{"./foo/./bar.h", "foo/bar.h"},
		{"./x/foo/../bar.h", "x/bar.h"},
		{"./x/foo/../../bar.h", "bar.h"},
		{"foo//bar", "foo/bar"},
		{"foo//.//..///bar", "bar"},
		{"./x/../foo/../../bar.h", "../bar.h"},
		{"foo/./.", "foo"},
		{"foo/bar/..", "foo"},
		{"foo/.hidden_bar", "foo/.hidden_bar"},
		{"/foo", "/foo"},
		{"/", ""},
		{"/foo/..", ""},
		{".", "."},
		{"./.", "."},
		{"foo/..", "."},
		{"../../foo/bar.h", "../../foo/bar.h"},
		{"test/../../foo/bar.h", "../foo/bar.h"},
	}
	if runtime.GOOS == "windows" {
		data = append(data, row{"//foo", "//foo"})
	} else {
		data = append(data, row{"//foo", "/foo"})
	}
	for i, l := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var unused uint64
			got := CanonicalizePath(l.in, &unused)
			if l.want != got {
				t.Fatalf("want: %q, got: %q", l.want, got)
			}
		})
	}
}

/*
func TestCanonicalizePath_PathSamplesWindows(t *testing.T) {
	path := ""

	CanonicalizePath(&path, &unused)
	if "" != path {
		t.Fatal("expected equal")
	}

	path = "foo.h"
	CanonicalizePath(&path, &unused)
	if "foo.h" != path {
		t.Fatal("expected equal")
	}

	path = ".\\foo.h"
	CanonicalizePath(&path, &unused)
	if "foo.h" != path {
		t.Fatal("expected equal")
	}

	path = ".\\foo\\.\\bar.h"
	CanonicalizePath(&path, &unused)
	if "foo/bar.h" != path {
		t.Fatal("expected equal")
	}

	path = ".\\x\\foo\\..\\bar.h"
	CanonicalizePath(&path, &unused)
	if "x/bar.h" != path {
		t.Fatal("expected equal")
	}

	path = ".\\x\\foo\\..\\..\\bar.h"
	CanonicalizePath(&path, &unused)
	if "bar.h" != path {
		t.Fatal("expected equal")
	}

	path = "foo\\\\bar"
	CanonicalizePath(&path, &unused)
	if "foo/bar" != path {
		t.Fatal("expected equal")
	}

	path = "foo\\\\.\\\\..\\\\\\bar"
	CanonicalizePath(&path, &unused)
	if "bar" != path {
		t.Fatal("expected equal")
	}

	path = ".\\x\\..\\foo\\..\\..\\bar.h"
	CanonicalizePath(&path, &unused)
	if "../bar.h" != path {
		t.Fatal("expected equal")
	}

	path = "foo\\.\\."
	CanonicalizePath(&path, &unused)
	if "foo" != path {
		t.Fatal("expected equal")
	}

	path = "foo\\bar\\.."
	CanonicalizePath(&path, &unused)
	if "foo" != path {
		t.Fatal("expected equal")
	}

	path = "foo\\.hidden_bar"
	CanonicalizePath(&path, &unused)
	if "foo/.hidden_bar" != path {
		t.Fatal("expected equal")
	}

	path = "\\foo"
	CanonicalizePath(&path, &unused)
	if "/foo" != path {
		t.Fatal("expected equal")
	}

	path = "\\\\foo"
	CanonicalizePath(&path, &unused)
	if "//foo" != path {
		t.Fatal("expected equal")
	}

	path = "\\"
	CanonicalizePath(&path, &unused)
	if "" != path {
		t.Fatal("expected equal")
	}
}

func TestCanonicalizePath_SlashTracking(t *testing.T) {
	path := ""
	var slash_bits uint64

	path = "foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "foo.h" != path {
		t.Fatal("expected equal")
	}
	if 0 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a/bcd/efh\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/bcd/efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 4 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\bcd/efh\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/bcd/efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 5 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\bcd\\efh\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/bcd/efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 7 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a/bcd/efh/foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/bcd/efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 0 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\./efh\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 3 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\../efh\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "efh/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\b\\c\\d\\e\\f\\g\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/b/c/d/e/f/g/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 127 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\b\\c\\..\\..\\..\\g\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "g/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\b/c\\../../..\\g\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "g/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\b/c\\./../..\\g\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/g/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 3 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\b/c\\./../..\\g/foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/g/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\\\\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a/\\\\foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 0 != slash_bits {
		t.Fatal("expected equal")
	}

	path = "a\\//foo.h"
	CanonicalizePath(&path, &slash_bits)
	if "a/foo.h" != path {
		t.Fatal("expected equal")
	}
	if 1 != slash_bits {
		t.Fatal("expected equal")
	}
}

func TestCanonicalizePath_CanonicalizeNotExceedingLen(t *testing.T) {
	// Make sure searching \/ doesn't go past supplied len.
	buf := "foo/bar\\baz.h\\" // Last \ past end.
	var slash_bits uint64
	size := 13
	CanonicalizePath(buf[:13], &slash_bits)
	if 0 != strncmp("foo/bar/baz.h", buf, size) {
		t.Fatal("expected equal")
	}
	if 2 != slash_bits {
		t.Fatal("expected equal")
	} // Not including the trailing one.
}

func TestCanonicalizePath_TooManyComponents(t *testing.T) {
	path := ""
	var slash_bits uint64

	// 64 is OK.
	path = "a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./x.h"
	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0x0 {
		t.Fatal("expected equal")
	}

	// Backslashes version.
	path =
		"a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\x.h"

	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0xffffffff {
		t.Fatal("expected equal")
	}

	// 65 is OK if #component is less than 60 after path canonicalization.
	path = "a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./x/y.h"
	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0x0 {
		t.Fatal("expected equal")
	}

	// Backslashes version.
	path =
		"a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\x\\y.h"
	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0x1ffffffff {
		t.Fatal("expected equal")
	}

	// 59 after canonicalization is OK.
	path = "a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/x/y.h"
	if 58 != count(path.begin(), path.end(), '/') {
		t.Fatal("expected equal")
	}
	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0x0 {
		t.Fatal("expected equal")
	}

	// Backslashes version.
	path =
		"a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\x\\y.h"
	if 58 != count(path.begin(), path.end(), '\\') {
		t.Fatal("expected equal")
	}
	CanonicalizePath(&path, &slash_bits)
	if slash_bits != 0x3ffffffffffffff {
		t.Fatal("expected equal")
	}
}

func TestCanonicalizePath_AbsolutePath(t *testing.T) {
	path := "/usr/include/stdio.h"
	err := ""
	CanonicalizePath(&path)
	if "/usr/include/stdio.h" != path {
		t.Fatal("expected equal")
	}
}

func TestCanonicalizePath_NotNullTerminated(t *testing.T) {
	path := ""
	var len2 uint
	var unused uint64

	path = "foo/. bar/."
	len2 = strlen("foo/.") // Canonicalize only the part before the space.
	CanonicalizePath(&path[0], &len2, &unused)
	if strlen("foo") != len2 {
		t.Fatal("expected equal")
	}
	if "foo/. bar/." != string(path) {
		t.Fatal("expected equal")
	}

	path = "foo/../file bar/."
	len2 = strlen("foo/../file")
	CanonicalizePath(&path[0], &len2, &unused)
	if strlen("file") != len2 {
		t.Fatal("expected equal")
	}
	if "file ./file bar/." != string(path) {
		t.Fatal("expected equal")
	}
}

func TestPathEscaping_TortureTest(t *testing.T) {
	result := ""

	GetWin32EscapedString("foo bar\\\"'$@d!st!c'\\path'\\", &result)
	if "\"foo bar\\\\\\\"'$@d!st!c'\\path'\\\\\"" != result {
		t.Fatal("expected equal")
	}
	result = nil

	GetShellEscapedString("foo bar\"/'$@d!st!c'/path'", &result)
	if "'foo bar\"/'\\''$@d!st!c'\\''/path'\\'''" != result {
		t.Fatal("expected equal")
	}
}

func TestPathEscaping_SensiblePathsAreNotNeedlesslyEscaped(t *testing.T) {
	path := "some/sensible/path/without/crazy/characters.c++"
	result := ""

	GetWin32EscapedString(path, &result)
	if path != result {
		t.Fatal("expected equal")
	}
	result = nil

	GetShellEscapedString(path, &result)
	if path != result {
		t.Fatal("expected equal")
	}
}

func TestPathEscaping_SensibleWin32PathsAreNotNeedlesslyEscaped(t *testing.T) {
	path := "some\\sensible\\path\\without\\crazy\\characters.c++"
	result := ""

	GetWin32EscapedString(path, &result)
	if path != result {
		t.Fatal("expected equal")
	}
}

func TestStripAnsiEscapeCodes_EscapeAtEnd(t *testing.T) {
	stripped := StripAnsiEscapeCodes("foo\x33")
	if "foo" != stripped {
		t.Fatal("expected equal")
	}

	stripped = StripAnsiEscapeCodes("foo\x33[")
	if "foo" != stripped {
		t.Fatal("expected equal")
	}
}

func TestStripAnsiEscapeCodes_StripColors(t *testing.T) {
	// An actual clang warning.
	input := "\x33[1maffixmgr.cxx:286:15: \x33[0m\x33[0;1;35mwarning: \x33[0m\x33[1musing the result... [-Wparentheses]\x33[0m"
	stripped := StripAnsiEscapeCodes(input)
	if "affixmgr.cxx:286:15: warning: using the result... [-Wparentheses]" != stripped {
		t.Fatal("expected equal")
	}
}

func TestElideMiddle_NothingToElide(t *testing.T) {
	input := "Nothing to elide in this short string."
	if input != ElideMiddle(input, 80) {
		t.Fatal("expected equal")
	}
	if input != ElideMiddle(input, 38) {
		t.Fatal("expected equal")
	}
	if "" != ElideMiddle(input, 0) {
		t.Fatal("expected equal")
	}
	if "." != ElideMiddle(input, 1) {
		t.Fatal("expected equal")
	}
	if ".." != ElideMiddle(input, 2) {
		t.Fatal("expected equal")
	}
	if "..." != ElideMiddle(input, 3) {
		t.Fatal("expected equal")
	}
}

func TestElideMiddle_ElideInTheMiddle(t *testing.T) {
	input := "01234567890123456789"
	elided := ElideMiddle(input, 10)
	if "012...789" != elided {
		t.Fatal("expected equal")
	}
	if "01234567...23456789" != ElideMiddle(input, 19) {
		t.Fatal("expected equal")
	}
}
*/
