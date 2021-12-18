// Copyright 2015 Google Inc. All Rights Reserved.
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


type DyndepParserTest struct {

  state_ State
  fs_ VirtualFileSystem
  dyndep_file_ DyndepFile
}
func (d *DyndepParserTest) AssertParse(input string) {
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if parser.ParseTest(input, &err) { t.FailNow() }
  if "" != err { t.FailNow() }
}
func (d *DyndepParserTest) SetUp() {
  ::AssertParse(&state_, "rule touch\n" "  command = touch $out\n" "build out otherout: touch\n")
}

func TestDyndepParserTest_Empty(t *testing.T) {
  const char kInput[] =
""
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: expected 'ninja_dyndep_version = ...'\n" != err { t.FailNow() }
}

func TestDyndepParserTest_Version1(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n"))
}

func TestDyndepParserTest_Version1Extra(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1-extra\n"))
}

func TestDyndepParserTest_Version1_0(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1.0\n"))
}

func TestDyndepParserTest_Version1_0Extra(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1.0-extra\n"))
}

func TestDyndepParserTest_CommentVersion(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "# comment\n" "ninja_dyndep_version = 1\n"))
}

func TestDyndepParserTest_BlankLineVersion(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "\n" "ninja_dyndep_version = 1\n"))
}

func TestDyndepParserTest_VersionCRLF(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\r\n"))
}

func TestDyndepParserTest_CommentVersionCRLF(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "# comment\r\n" "ninja_dyndep_version = 1\r\n"))
}

func TestDyndepParserTest_BlankLineVersionCRLF(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "\r\n" "ninja_dyndep_version = 1\r\n"))
}

func TestDyndepParserTest_VersionUnexpectedEOF(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1.0"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: unexpected EOF\n" "ninja_dyndep_version = 1.0\n" "                          ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_UnsupportedVersion0(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 0\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: unsupported 'ninja_dyndep_version = 0'\n" "ninja_dyndep_version = 0\n" "                        ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_UnsupportedVersion1_1(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1.1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: unsupported 'ninja_dyndep_version = 1.1'\n" "ninja_dyndep_version = 1.1\n" "                          ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_DuplicateVersion(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"ninja_dyndep_version = 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: unexpected identifier\n" != err { t.FailNow() }
}

func TestDyndepParserTest_MissingVersionOtherVar(t *testing.T) {
  const char kInput[] =
"not_ninja_dyndep_version = 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: expected 'ninja_dyndep_version = ...'\n" "not_ninja_dyndep_version = 1\n" "                            ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_MissingVersionBuild(t *testing.T) {
  const char kInput[] =
"build out: dyndep\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: expected 'ninja_dyndep_version = ...'\n" != err { t.FailNow() }
}

func TestDyndepParserTest_UnexpectedEqual(t *testing.T) {
  const char kInput[] =
"= 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: unexpected '='\n" != err { t.FailNow() }
}

func TestDyndepParserTest_UnexpectedIndent(t *testing.T) {
  const char kInput[] =
" = 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:1: unexpected indent\n" != err { t.FailNow() }
}

func TestDyndepParserTest_OutDuplicate(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep\n"
"build out: dyndep\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:3: multiple statements for 'out'\n" "build out: dyndep\n" "         ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OutDuplicateThroughOther(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep\n"
"build otherout: dyndep\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:3: multiple statements for 'otherout'\n" "build otherout: dyndep\n" "              ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_NoOutEOF(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: unexpected EOF\n" "build\n" "     ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_NoOutColon(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build :\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: expected path\n" "build :\n" "      ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OutNoStatement(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build missing: dyndep\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: no build statement exists for 'missing'\n" "build missing: dyndep\n" "             ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OutEOF(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: unexpected EOF\n" "build out\n" "         ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OutNoRule(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out:"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: expected build command name 'dyndep'\n" "build out:\n" "          ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OutBadRule(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: touch"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: expected build command name 'dyndep'\n" "build out: touch\n" "           ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_BuildEOF(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: unexpected EOF\n" "build out: dyndep\n" "                 ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_ExplicitOut(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out exp: dyndep\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: explicit outputs not supported\n" "build out exp: dyndep\n" "             ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_ExplicitIn(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep exp\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: explicit inputs not supported\n" "build out: dyndep exp\n" "                     ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_OrderOnlyIn(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep ||\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:2: order-only inputs not supported\n" "build out: dyndep ||\n" "                  ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_BadBinding(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep\n"
"  not_restat = 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:3: binding is not 'restat'\n" "  not_restat = 1\n" "                ^ near here" != err { t.FailNow() }
}

func TestDyndepParserTest_RestatTwice(t *testing.T) {
  const char kInput[] =
"ninja_dyndep_version = 1\n"
"build out: dyndep\n"
"  restat = 1\n"
"  restat = 1\n"
  DyndepParser parser(&state_, &fs_, &dyndep_file_)
  err := ""
  if !parser.ParseTest(kInput, &err) { t.FailNow() }
  if "input:4: unexpected indent\n" != err { t.FailNow() }
}

func TestDyndepParserTest_NoImplicit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out: dyndep\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_EmptyImplicit(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out | : dyndep |\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_ImplicitIn(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out: dyndep | impin\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 1u != i.second.implicit_inputs_.size() { t.FailNow() }
  if "impin" != i.second.implicit_inputs_[0].path() { t.FailNow() }
}

func TestDyndepParserTest_ImplicitIns(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out: dyndep | impin1 impin2\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 2u != i.second.implicit_inputs_.size() { t.FailNow() }
  if "impin1" != i.second.implicit_inputs_[0].path() { t.FailNow() }
  if "impin2" != i.second.implicit_inputs_[1].path() { t.FailNow() }
}

func TestDyndepParserTest_ImplicitOut(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out | impout: dyndep\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 1u != i.second.implicit_outputs_.size() { t.FailNow() }
  if "impout" != i.second.implicit_outputs_[0].path() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_ImplicitOuts(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out | impout1 impout2 : dyndep\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 2u != i.second.implicit_outputs_.size() { t.FailNow() }
  if "impout1" != i.second.implicit_outputs_[0].path() { t.FailNow() }
  if "impout2" != i.second.implicit_outputs_[1].path() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_ImplicitInsAndOuts(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out | impout1 impout2: dyndep | impin1 impin2\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 2u != i.second.implicit_outputs_.size() { t.FailNow() }
  if "impout1" != i.second.implicit_outputs_[0].path() { t.FailNow() }
  if "impout2" != i.second.implicit_outputs_[1].path() { t.FailNow() }
  if 2u != i.second.implicit_inputs_.size() { t.FailNow() }
  if "impin1" != i.second.implicit_inputs_[0].path() { t.FailNow() }
  if "impin2" != i.second.implicit_inputs_[1].path() { t.FailNow() }
}

func TestDyndepParserTest_Restat(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out: dyndep\n" "  restat = 1\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if true != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_OtherOutput(t *testing.T) {
  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build otherout: dyndep\n"))

  if 1u != dyndep_file_.size() { t.FailNow() }
  i := dyndep_file_.find(state_.edges_[0])
  if i == dyndep_file_.end() { t.FailNow() }
  if false != i.second.restat_ { t.FailNow() }
  if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
  if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
}

func TestDyndepParserTest_MultipleEdges(t *testing.T) {
    ::AssertParse(&state_, "build out2: touch\n")
  if 2u != state_.edges_.size() { t.FailNow() }
  if 1u != state_.edges_[1].outputs_.size() { t.FailNow() }
  if "out2" != state_.edges_[1].outputs_[0].path() { t.FailNow() }
  if 0u != state_.edges_[0].inputs_.size() { t.FailNow() }

  ASSERT_NO_FATAL_FAILURE(AssertParse( "ninja_dyndep_version = 1\n" "build out: dyndep\n" "build out2: dyndep\n" "  restat = 1\n"))

  if 2u != dyndep_file_.size() { t.FailNow() }
  {
    i := dyndep_file_.find(state_.edges_[0])
    if i == dyndep_file_.end() { t.FailNow() }
    if false != i.second.restat_ { t.FailNow() }
    if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
    if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
  }
  {
    i := dyndep_file_.find(state_.edges_[1])
    if i == dyndep_file_.end() { t.FailNow() }
    if true != i.second.restat_ { t.FailNow() }
    if 0u != i.second.implicit_outputs_.size() { t.FailNow() }
    if 0u != i.second.implicit_inputs_.size() { t.FailNow() }
  }
}

