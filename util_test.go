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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		// CanonicalizePath.UpDir:
		{"../../foo/bar.h", "../../foo/bar.h"},
		{"test/../../foo/bar.h", "../foo/bar.h"},
		// CanonicalizePath.AbsolutePath
		{"/usr/include/stdio.h", "/usr/include/stdio.h"},
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

func TestCanonicalizePath_PathSamplesWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	type row struct {
		in   string
		want string
	}
	data := []row{
		{"", ""},
		{"foo.", "foo."},
		{".\\foo.h", "foo.h"},
		{".\\foo\\.\\bar.h", "foo/bar.h"},
		{".\\x\\foo\\..\\bar.h", "x/bar.h"},
		{".\\x\\foo\\..\\..\\bar.h", "bar.h"},
		{"foo\\\\bar", "foo/bar"},
		{"foo\\\\.\\\\..\\\\\\bar", "bar"},
		{".\\x\\..\\foo\\..\\..\\bar.h", "../bar.h"},
		{"foo\\.\\.", "foo"},
		{"foo\\bar\\..", "foo"},
		{"foo\\.hidden_bar", "foo/.hidden_bar"},
		{"\\foo", "/foo"},
		{"\\\\foo", "//foo"},
		{"\\", ""},
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

func TestCanonicalizePath_SlashTracking(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	type row struct {
		in        string
		want      string
		want_bits uint64
	}
	data := []row{
		{"", "", 0},
		{"foo.h", "foo.h", 0},
		{"foo.h", "foo.h", 0},
		{"a\\foo.h", "a/foo.h", 1},
		{"a/bcd/efh\\foo.h", "a/bcd/efh/foo.h", 4},
		{"a\\bcd/efh\\foo.h", "a/bcd/efh/foo.h", 5},
		{"a\\bcd\\efh\\foo.h", "a/bcd/efh/foo.h", 7},
		{"a/bcd/efh/foo.h", "a/bcd/efh/foo.h", 0},
		{"a\\./efh\\foo.h", "a/efh/foo.h", 3},
		{"a\\../efh\\foo.h", "efh/foo.h", 1},
		{"a\\b\\c\\d\\e\\f\\g\\foo.h", "a/b/c/d/e/f/g/foo.h", 127},
		{"a\\b\\c\\..\\..\\..\\g\\foo.h", "g/foo.h", 1},
		{"a\\b/c\\../../..\\g\\foo.h", "g/foo.h", 1},
		{"a\\b/c\\./../..\\g\\foo.h", "a/g/foo.h", 3},
		{"a\\b/c\\./../..\\g/foo.h", "a/g/foo.h", 1},
		{"a\\\\\\foo.h", "a/foo.h", 1},
		{"a/\\\\foo.h", "a/foo.h", 0},
		{"a\\//foo.h", "a/foo.h", 1},
	}
	for i, l := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var slash_bits uint64
			got := CanonicalizePath(l.in, &slash_bits)
			if l.want != got {
				t.Fatalf("want: %q, got: %q", l.want, got)
			}
			if slash_bits != l.want_bits {
				t.Fatalf("want: %d, got: %d", l.want_bits, slash_bits)
			}
		})
	}
}

func TestCanonicalizePath_CanonicalizeNotExceedingLen(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	t.Skip("This test is irrelevant in Go. Remove once conversion is done")
}

func TestCanonicalizePath_TooManyComponents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	t.Skip("TODO")
	type row struct {
		in string
		//want      string
		want_bits uint64
	}
	data := []row{
		// 64 is OK.
		{
			"a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./x.h",
			0,
		},
		// Backslashes version.
		{
			"a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\x.h",
			0xffffffff,
		},
		// 65 is OK if #component is less than 60 after path canonicalization.
		{
			"a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./a/./x/y.h",
			0,
		},
		// Backslashes version.
		{
			"a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\a\\.\\x\\y.h",
			0x1ffffffff,
		},
		// 59 after canonicalization is OK.
		{
			"a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/a/x/y.h",
			0,
		},
		// Backslashes version.
		{
			"a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\a\\x\\y.h",
			0x3ffffffffffffff,
		},
	}
	// Manual check that the last 2 ones have 58 items.
	if 58 != strings.Count(data[4].in, "/") {
		t.Fatal("expected equal")
	}
	if 58 != strings.Count(data[5].in, "\\") {
		t.Fatal("expected equal")
	}

	for i, l := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var slash_bits uint64
			_ = CanonicalizePath(l.in, &slash_bits)
			if slash_bits != l.want_bits {
				t.Fatalf("want: %d, got: %d", l.want_bits, slash_bits)
			}
		})
	}
}

func TestCanonicalizePath_NotNullTerminated(t *testing.T) {
	t.Skip("This test is irrelevant in Go. Remove once conversion is done")
}

func TestPathEscaping_TortureTest(t *testing.T) {
	got := GetWin32EscapedString("foo bar\\\"'$@d!st!c'\\path'\\")
	if diff := cmp.Diff("\"foo bar\\\\\\\"'$@d!st!c'\\path'\\\\\"", got); diff != "" {
		t.Fatalf("+want, -got: %s", diff)
	}
	got = GetShellEscapedString("foo bar\"/'$@d!st!c'/path'")
	if diff := cmp.Diff("'foo bar\"/'\\''$@d!st!c'\\''/path'\\'''", got); diff != "" {
		t.Fatalf("+want, -got: %s", diff)
	}
}

func TestPathEscaping_SensiblePathsAreNotNeedlesslyEscaped(t *testing.T) {
	path := "some/sensible/path/without/crazy/characters.c++"
	got := GetWin32EscapedString(path)
	if diff := cmp.Diff(path, got); diff != "" {
		t.Fatalf("+want, -got: %s", diff)
	}
	got = GetShellEscapedString(path)
	if diff := cmp.Diff(path, got); diff != "" {
		t.Fatalf("+want, -got: %s", diff)
	}
}

func TestPathEscaping_SensibleWin32PathsAreNotNeedlesslyEscaped(t *testing.T) {
	path := "some\\sensible\\path\\without\\crazy\\characters.c++"
	result := GetWin32EscapedString(path)
	if path != result {
		t.Fatal("expected equal")
	}
}

func TestStripAnsiEscapeCodes_EscapeAtEnd(t *testing.T) {
	stripped := StripAnsiEscapeCodes("foo\x1B")
	if "foo" != stripped {
		t.Fatalf("%+q", stripped)
	}

	stripped = StripAnsiEscapeCodes("foo\x1B[")
	if "foo" != stripped {
		t.Fatalf("%+q", stripped)
	}
}

func TestStripAnsiEscapeCodes_StripColors(t *testing.T) {
	// An actual clang warning.
	input := "\x1B[1maffixmgr.cxx:286:15: \x1B[0m\x1B[0;1;35mwarning: \x1B[0m\x1B[1musing the result... [-Wparentheses]\x1B[0m"
	stripped := StripAnsiEscapeCodes(input)
	if "affixmgr.cxx:286:15: warning: using the result... [-Wparentheses]" != stripped {
		t.Fatalf("%+q", stripped)
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

var dummyBenchmarkCanonicalizePath = ""

// The C++ version is canon_perftest. It runs 2000000 iterations.
//
// On my workstation:
//
// The C++ version has an minimum of 82ms.
//
// The Go version with "go test -cpu 1 -bench=. -run BenchmarkCanonicalizePath"
// has a minimum of 157ns, which multiplied by 2000000 gives 306ms. So the code
// is nearly 4x slower. I'll have to optimize later.
func BenchmarkCanonicalizePath(b *testing.B) {
	kPath := "../../third_party/WebKit/Source/WebCore/platform/leveldb/LevelDBWriteBatch.cpp"
	var slash_bits uint64
	s := ""
	for i := 0; i < b.N; i++ {
		s = CanonicalizePath(kPath, &slash_bits)
	}
	// Use s so it's not optimized out.
	dummyBenchmarkCanonicalizePath = s
}
