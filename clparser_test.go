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

package nin

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCLParserTest_ShowIncludes(t *testing.T) {
	if "" != filterShowIncludes("", "") {
		t.Fatal("expected equal")
	}

	if "" != filterShowIncludes("Sample compiler output", "") {
		t.Fatal("expected equal")
	}
	if "c:\\Some Files\\foobar.h" != filterShowIncludes("Note: including file: c:\\Some Files\\foobar.h", "") {
		t.Fatal("expected equal")
	}
	if "c:\\initspaces.h" != filterShowIncludes("Note: including file:    c:\\initspaces.h", "") {
		t.Fatal("expected equal")
	}
	if "c:\\initspaces.h" != filterShowIncludes("Non-default prefix: inc file:    c:\\initspaces.h", "Non-default prefix: inc file:") {
		t.Fatal("expected equal")
	}
}

func TestCLParserTest_FilterInputFilename(t *testing.T) {
	if !filterInputFilename("foobar.cc") {
		t.Fatal("expected true")
	}
	if !filterInputFilename("foo bar.cc") {
		t.Fatal("expected true")
	}
	if !filterInputFilename("baz.c") {
		t.Fatal("expected true")
	}
	if !filterInputFilename("FOOBAR.CC") {
		t.Fatal("expected true")
	}

	if filterInputFilename("src\\cl_helper.cc(166) : fatal error C1075: end of file found ...") {
		t.Fatal("expected false")
	}
}

func TestCLParserTest_ParseSimple(t *testing.T) {
	parser := NewCLParser()
	output := ""
	if err := parser.Parse("foo\r\nNote: inc file prefix:  foo.h\r\nbar\r\n", "Note: inc file prefix:", &output); err != nil {
		t.Fatal(err)
	}

	if "foo\nbar\n" != output {
		t.Fatal(output)
	}
	if diff := cmp.Diff(map[string]struct{}{"foo.h": {}}, parser.includes); diff != "" {
		t.Fatal(diff)
	}
}

func TestCLParserTest_ParseFilenameFilter(t *testing.T) {
	parser := NewCLParser()
	output := ""
	if err := parser.Parse("foo.cc\r\ncl: warning\r\n", "", &output); err != nil {
		t.Fatal(err)
	}
	if "cl: warning\n" != output {
		t.Fatal(output)
	}
}

func TestCLParserTest_NoFilenameFilterAfterShowIncludes(t *testing.T) {
	parser := NewCLParser()
	output := ""
	if err := parser.Parse("foo.cc\r\nNote: including file: foo.h\r\nsomething something foo.cc\r\n", "", &output); err != nil {
		t.Fatal(err)
	}
	if "something something foo.cc\n" != output {
		t.Fatal(output)
	}
}

func TestCLParserTest_ParseSystemInclude(t *testing.T) {
	parser := NewCLParser()
	output := ""
	if err := parser.Parse("Note: including file: c:\\Program Files\\foo.h\r\nNote: including file: d:\\Microsoft Visual Studio\\bar.h\r\nNote: including file: path.h\r\n", "", &output); err != nil {
		t.Fatal(err)
	}
	// We should have dropped the first two includes because they look like
	// system headers.
	if "" != output {
		t.Fatal("expected equal")
	}
	if diff := cmp.Diff(map[string]struct{}{"path.h": {}}, parser.includes); diff != "" {
		t.Fatal(diff)
	}
}

func TestCLParserTest_DuplicatedHeader(t *testing.T) {
	parser := NewCLParser()
	output := ""
	if err := parser.Parse("Note: including file: foo.h\r\nNote: including file: bar.h\r\nNote: including file: foo.h\r\n", "", &output); err != nil {
		t.Fatal(err)
	}
	// We should have dropped one copy of foo.h.
	if "" != output {
		t.Fatal("expected equal")
	}
	if 2 != len(parser.includes) {
		t.Fatal("expected equal")
	}
}

func TestCLParserTest_DuplicatedHeaderPathConverted(t *testing.T) {
	parser := NewCLParser()
	output := ""
	// This isn't inline in the Parse() call below because the #ifdef in
	// a macro expansion would confuse MSVC2013's preprocessor.
	input := "Note: including file: sub/./foo.h\r\nNote: including file: bar.h\r\nNote: including file: sub\\foo.h\r\n"
	if err := parser.Parse(input, "", &output); err != nil {
		t.Fatal(err)
	}
	// We should have dropped one copy of foo.h.
	if "" != output {
		t.Fatal("expected equal")
	}
	if 2 != len(parser.includes) {
		t.Fatal(parser.includes)
	}
}

var dummyBenchmarkCLParser = ""

const benchmarkCLParserInput = "Note: including file: C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\iostream\r\n" +
	"Note: including file:  C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\istream\r\n" +
	"Note: including file:   C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\ostream\r\n" +
	"Note: including file:    C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\ios\r\n" +
	"Note: including file:     C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xlocnum\r\n" +
	"Note: including file:      C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\climits\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\yvals.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xkeycheck.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\crtdefs.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\sal.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\ConcurrencySal.h\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vadefs.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt.h\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\use_ansi.h\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\limits.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:      C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cmath\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\math.h\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xtgmath.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xtr1common\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cstdlib\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\stdlib.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_malloc.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_search.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\stddef.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wstdlib.h\r\n" +
	"Note: including file:      C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cstdio\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\stdio.h\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wstdio.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_stdio_config.h\r\n" +
	"Note: including file:      C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\streambuf\r\n" +
	"Note: including file:       C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xiosbase\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xlocale\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cstring\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\string.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_memory.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_memcpy_s.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\errno.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime_string.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wstring.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\stdexcept\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\exception\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\type_traits\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xstddef\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cstddef\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\initializer_list\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\malloc.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime_exception.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\eh.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_terminate.h\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xstring\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xmemory0\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cstdint\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\stdint.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\limits\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\ymath.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cfloat\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\float.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cwchar\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\wchar.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wconio.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wctype.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wdirect.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wio.h\r\n" +
	"Note: including file:                C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_share.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wprocess.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\corecrt_wtime.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\sys/stat.h\r\n" +
	"Note: including file:                C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\sys/types.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\new\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime_new.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xutility\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\utility\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\iosfwd\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\crtdbg.h\r\n" +
	"Note: including file:                C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime_new_debug.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xatomic0.h\r\n" +
	"Note: including file:            C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\intrin.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\setjmp.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\immintrin.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\wmmintrin.h\r\n" +
	"Note: including file:               C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\nmmintrin.h\r\n" +
	"Note: including file:                C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\smmintrin.h\r\n" +
	"Note: including file:                 C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\tmmintrin.h\r\n" +
	"Note: including file:                  C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\pmmintrin.h\r\n" +
	"Note: including file:                   C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\emmintrin.h\r\n" +
	"Note: including file:                    C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xmmintrin.h\r\n" +
	"Note: including file:                     C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\mmintrin.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\ammintrin.h\r\n" +
	"Note: including file:             C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\mm3dnow.h\r\n" +
	"Note: including file:              C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\typeinfo\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime_typeinfo.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\vcruntime.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xlocinfo\r\n" +
	"Note: including file:          C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xlocinfo.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\ctype.h\r\n" +
	"Note: including file:           C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\locale.h\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\xfacet\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\system_error\r\n" +
	"Note: including file:         C:\\Program Files (x86)\\Microsoft Visual Studio 14.0\\VC\\INCLUDE\\cerrno\r\n" +
	"Note: including file:        C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.10240.0\\ucrt\\share.h\r\n"

// On my workstation, the C++ version takes 48µs. The Go version takes 300µs.
// So there's a lot of optimization to do here.
func BenchmarkCLParser(b *testing.B) {
	b.ReportAllocs()
	s := ""
	for i := 0; i < b.N; i++ {
		parser := NewCLParser()
		if err := parser.Parse(benchmarkCLParserInput, "", &s); err != nil {
			b.Fatal(err)
		}
	}
	// Use s so it's not optimized out.
	dummyBenchmarkValue = s
}
