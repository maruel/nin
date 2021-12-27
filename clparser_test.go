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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCLParserTest_ShowIncludes(t *testing.T) {
	if "" != FilterShowIncludes("", "") {
		t.Fatal("expected equal")
	}

	if "" != FilterShowIncludes("Sample compiler output", "") {
		t.Fatal("expected equal")
	}
	if "c:\\Some Files\\foobar.h" != FilterShowIncludes("Note: including file: c:\\Some Files\\foobar.h", "") {
		t.Fatal("expected equal")
	}
	if "c:\\initspaces.h" != FilterShowIncludes("Note: including file:    c:\\initspaces.h", "") {
		t.Fatal("expected equal")
	}
	if "c:\\initspaces.h" != FilterShowIncludes("Non-default prefix: inc file:    c:\\initspaces.h", "Non-default prefix: inc file:") {
		t.Fatal("expected equal")
	}
}

func TestCLParserTest_FilterInputFilename(t *testing.T) {
	if !FilterInputFilename("foobar.cc") {
		t.Fatal("expected true")
	}
	if !FilterInputFilename("foo bar.cc") {
		t.Fatal("expected true")
	}
	if !FilterInputFilename("baz.c") {
		t.Fatal("expected true")
	}
	if !FilterInputFilename("FOOBAR.CC") {
		t.Fatal("expected true")
	}

	if FilterInputFilename("src\\cl_helper.cc(166) : fatal error C1075: end of file found ...") {
		t.Fatal("expected false")
	}
}

func TestCLParserTest_ParseSimple(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""
	if !parser.Parse("foo\r\nNote: inc file prefix:  foo.h\r\nbar\r\n", "Note: inc file prefix:", &output, &err) {
		t.Fatal("expected true")
	}

	if "foo\nbar\n" != output {
		t.Fatal(output)
	}
	if diff := cmp.Diff(map[string]struct{}{"foo.h": {}}, parser.includes_); diff != "" {
		t.Fatal(diff)
	}
}

func TestCLParserTest_ParseFilenameFilter(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""
	if !parser.Parse("foo.cc\r\ncl: warning\r\n", "", &output, &err) {
		t.Fatal("expected true")
	}
	if "cl: warning\n" != output {
		t.Fatal(output)
	}
}

func TestCLParserTest_NoFilenameFilterAfterShowIncludes(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""
	if !parser.Parse("foo.cc\r\nNote: including file: foo.h\r\nsomething something foo.cc\r\n", "", &output, &err) {
		t.Fatal("expected true")
	}
	if "something something foo.cc\n" != output {
		t.Fatal(output)
	}
}

func TestCLParserTest_ParseSystemInclude(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""
	if !parser.Parse("Note: including file: c:\\Program Files\\foo.h\r\nNote: including file: d:\\Microsoft Visual Studio\\bar.h\r\nNote: including file: path.h\r\n", "", &output, &err) {
		t.Fatal("expected true")
	}
	// We should have dropped the first two includes because they look like
	// system headers.
	if "" != output {
		t.Fatal("expected equal")
	}
	if diff := cmp.Diff(map[string]struct{}{"path.h": {}}, parser.includes_); diff != "" {
		t.Fatal(diff)
	}
}

func TestCLParserTest_DuplicatedHeader(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""
	if !parser.Parse("Note: including file: foo.h\r\nNote: including file: bar.h\r\nNote: including file: foo.h\r\n", "", &output, &err) {
		t.Fatal("expected true")
	}
	// We should have dropped one copy of foo.h.
	if "" != output {
		t.Fatal("expected equal")
	}
	if 2 != len(parser.includes_) {
		t.Fatal("expected equal")
	}
}

func TestCLParserTest_DuplicatedHeaderPathConverted(t *testing.T) {
	t.Skip("TODO")
	var parser CLParser
	output := ""
	err := ""

	// This isn't inline in the Parse() call below because the #ifdef in
	// a macro expansion would confuse MSVC2013's preprocessor.
	kInput := "Note: including file: sub/./foo.h\r\nNote: including file: bar.h\r\nNote: including file: sub\\foo.h\r\n"
	if !parser.Parse(kInput, "", &output, &err) {
		t.Fatal("expected true")
	}
	// We should have dropped one copy of foo.h.
	if "" != output {
		t.Fatal("expected equal")
	}
	if 2 != len(parser.includes_) {
		t.Fatal("expected equal")
	}
}
