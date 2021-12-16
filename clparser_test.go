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


TEST(CLParserTest, ShowIncludes) {
  if "" != CLParser::FilterShowIncludes("", "") { t.FailNow() }

  if "" != CLParser::FilterShowIncludes("Sample compiler output", "") { t.FailNow() }
  ASSERT_EQ("c:\\Some Files\\foobar.h", CLParser::FilterShowIncludes("Note: including file: " "c:\\Some Files\\foobar.h", ""))
  ASSERT_EQ("c:\\initspaces.h", CLParser::FilterShowIncludes("Note: including file:    " "c:\\initspaces.h", ""))
  ASSERT_EQ("c:\\initspaces.h", CLParser::FilterShowIncludes("Non-default prefix: inc file:    " "c:\\initspaces.h", "Non-default prefix: inc file:"))
}

TEST(CLParserTest, FilterInputFilename) {
  if CLParser::FilterInputFilename("foobar.cc") { t.FailNow() }
  if CLParser::FilterInputFilename("foo bar.cc") { t.FailNow() }
  if CLParser::FilterInputFilename("baz.c") { t.FailNow() }
  if CLParser::FilterInputFilename("FOOBAR.CC") { t.FailNow() }

  ASSERT_FALSE(CLParser::FilterInputFilename( "src\\cl_helper.cc(166) : fatal error C1075: end " "of file found ..."))
}

TEST(CLParserTest, ParseSimple) {
  CLParser parser
  string output, err
  ASSERT_TRUE(parser.Parse( "foo\r\n" "Note: inc file prefix:  foo.h\r\n" "bar\r\n", "Note: inc file prefix:", &output, &err))

  if "foo\nbar\n" != output { t.FailNow() }
  if 1u != parser.includes_.size() { t.FailNow() }
  if "foo.h" != *parser.includes_.begin() { t.FailNow() }
}

TEST(CLParserTest, ParseFilenameFilter) {
  CLParser parser
  string output, err
  ASSERT_TRUE(parser.Parse( "foo.cc\r\n" "cl: warning\r\n", "", &output, &err))
  if "cl: warning\n" != output { t.FailNow() }
}

TEST(CLParserTest, NoFilenameFilterAfterShowIncludes) {
  CLParser parser
  string output, err
  ASSERT_TRUE(parser.Parse( "foo.cc\r\n" "Note: including file: foo.h\r\n" "something something foo.cc\r\n", "", &output, &err))
  if "something something foo.cc\n" != output { t.FailNow() }
}

TEST(CLParserTest, ParseSystemInclude) {
  CLParser parser
  string output, err
  ASSERT_TRUE(parser.Parse( "Note: including file: c:\\Program Files\\foo.h\r\n" "Note: including file: d:\\Microsoft Visual Studio\\bar.h\r\n" "Note: including file: path.h\r\n", "", &output, &err))
  // We should have dropped the first two includes because they look like
  // system headers.
  if "" != output { t.FailNow() }
  if 1u != parser.includes_.size() { t.FailNow() }
  if "path.h" != *parser.includes_.begin() { t.FailNow() }
}

TEST(CLParserTest, DuplicatedHeader) {
  CLParser parser
  string output, err
  ASSERT_TRUE(parser.Parse( "Note: including file: foo.h\r\n" "Note: including file: bar.h\r\n" "Note: including file: foo.h\r\n", "", &output, &err))
  // We should have dropped one copy of foo.h.
  if "" != output { t.FailNow() }
  if 2u != parser.includes_.size() { t.FailNow() }
}

TEST(CLParserTest, DuplicatedHeaderPathConverted) {
  CLParser parser
  string output, err

  // This isn't inline in the Parse() call below because the #ifdef in
  // a macro expansion would confuse MSVC2013's preprocessor.
  const char kInput[] =
      "Note: including file: sub/./foo.h\r\n"
      "Note: including file: bar.h\r\n"
      "Note: including file: sub\\foo.h\r\n"
      "Note: including file: sub/foo.h\r\n"
  if parser.Parse(kInput, "", &output, &err) { t.FailNow() }
  // We should have dropped one copy of foo.h.
  if "" != output { t.FailNow() }
  if 2u != parser.includes_.size() { t.FailNow() }
}

