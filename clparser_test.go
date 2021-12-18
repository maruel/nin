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


func TestCLParserTest_ShowIncludes(t *testing.T) {
  if "" != CLParser::FilterShowIncludes("", "") { t.FailNow() }

  if "" != CLParser::FilterShowIncludes("Sample compiler output", "") { t.FailNow() }
  if "c:\\Some Files\\foobar.h" != CLParser::FilterShowIncludes("Note: including file: c:\\Some Files\\foobar.h", "") { t.FailNow() }
  if "c:\\initspaces.h" != CLParser::FilterShowIncludes("Note: including file:    c:\\initspaces.h", "") { t.FailNow() }
  if "c:\\initspaces.h" != CLParser::FilterShowIncludes("Non-default prefix: inc file:    c:\\initspaces.h", "Non-default prefix: inc file:") { t.FailNow() }
}

func TestCLParserTest_FilterInputFilename(t *testing.T) {
  if CLParser::FilterInputFilename("foobar.cc") { t.FailNow() }
  if CLParser::FilterInputFilename("foo bar.cc") { t.FailNow() }
  if CLParser::FilterInputFilename("baz.c") { t.FailNow() }
  if CLParser::FilterInputFilename("FOOBAR.CC") { t.FailNow() }

  if !CLParser::FilterInputFilename( "src\\cl_helper.cc(166) : fatal error C1075: end of file found ...") { t.FailNow() }
}

func TestCLParserTest_ParseSimple(t *testing.T) {
  var parser CLParser
  string output, err
  if parser.Parse( "foo\r\nNote: inc file prefix:  foo.h\r\nbar\r\n", "Note: inc file prefix:", &output, &err) { t.FailNow() }

  if "foo\nbar\n" != output { t.FailNow() }
  if 1u != parser.includes_.size() { t.FailNow() }
  if "foo.h" != *parser.includes_.begin() { t.FailNow() }
}

func TestCLParserTest_ParseFilenameFilter(t *testing.T) {
  var parser CLParser
  string output, err
  if parser.Parse( "foo.cc\r\ncl: warning\r\n", "", &output, &err) { t.FailNow() }
  if "cl: warning\n" != output { t.FailNow() }
}

func TestCLParserTest_NoFilenameFilterAfterShowIncludes(t *testing.T) {
  var parser CLParser
  string output, err
  if parser.Parse( "foo.cc\r\nNote: including file: foo.h\r\nsomething something foo.cc\r\n", "", &output, &err) { t.FailNow() }
  if "something something foo.cc\n" != output { t.FailNow() }
}

func TestCLParserTest_ParseSystemInclude(t *testing.T) {
  var parser CLParser
  string output, err
  if parser.Parse( "Note: including file: c:\\Program Files\\foo.h\r\nNote: including file: d:\\Microsoft Visual Studio\\bar.h\r\nNote: including file: path.h\r\n", "", &output, &err) { t.FailNow() }
  // We should have dropped the first two includes because they look like
  // system headers.
  if "" != output { t.FailNow() }
  if 1u != parser.includes_.size() { t.FailNow() }
  if "path.h" != *parser.includes_.begin() { t.FailNow() }
}

func TestCLParserTest_DuplicatedHeader(t *testing.T) {
  var parser CLParser
  string output, err
  if parser.Parse( "Note: including file: foo.h\r\nNote: including file: bar.h\r\nNote: including file: foo.h\r\n", "", &output, &err) { t.FailNow() }
  // We should have dropped one copy of foo.h.
  if "" != output { t.FailNow() }
  if 2u != parser.includes_.size() { t.FailNow() }
}

func TestCLParserTest_DuplicatedHeaderPathConverted(t *testing.T) {
  var parser CLParser
  string output, err

  // This isn't inline in the Parse() call below because the #ifdef in
  // a macro expansion would confuse MSVC2013's preprocessor.
  const char kInput[] =
      "Note: including file: sub/./foo.h\r\nNote: including file: bar.h\r\n"
      "Note: including file: sub\\foo.h\r\n"
      "Note: including file: sub/foo.h\r\n"
  if parser.Parse(kInput, "", &output, &err) { t.FailNow() }
  // We should have dropped one copy of foo.h.
  if "" != output { t.FailNow() }
  if 2u != parser.includes_.size() { t.FailNow() }
}

