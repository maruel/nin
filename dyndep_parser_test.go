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

package nin

import "testing"

type DyndepParserTest struct {
	t          *testing.T
	state      State
	fs         VirtualFileSystem
	dyndepFile DyndepFile
}

func NewDyndepParserTest(t *testing.T) *DyndepParserTest {
	d := &DyndepParserTest{
		t:          t,
		state:      NewState(),
		fs:         NewVirtualFileSystem(),
		dyndepFile: DyndepFile{},
	}
	assertParse(t, "rule touch\n  command = touch $out\nbuild out otherout: touch\n", &d.state)
	return d
}

func assertParse(t *testing.T, input string, state *State) {
	parser := NewManifestParser(state, nil, ManifestParserOptions{})
	err := ""
	if !parser.parseTest(input, &err) {
		t.Fatal(err)
	}
	if "" != err {
		t.Fatal(err)
	}
	VerifyGraph(t, state)
}

func (d *DyndepParserTest) AssertParse(input string) {
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if !parser.parseTest(input, &err) {
		d.t.Fatal(err)
	}
	if "" != err {
		d.t.Fatal(err)
	}
	VerifyGraph(d.t, &d.state)
}

// parseText parses a text string of input. Only used in tests.
func (d *DyndepParser) parseTest(input string, err *string) bool {
	return d.Parse("input", []byte(input+"\x00"), err)
}

func TestDyndepParserTest_Empty(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := ""
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: expected 'ninja_dyndep_version = ...'\n" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_Version1(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\n")
}

func TestDyndepParserTest_Version1Extra(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1-extra\n")
}

func TestDyndepParserTest_Version1_0(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1.0\n")
}

func TestDyndepParserTest_Version1_0Extra(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1.0-extra\n")
}

func TestDyndepParserTest_CommentVersion(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("# comment\nninja_dyndep_version = 1\n")
}

func TestDyndepParserTest_BlankLineVersion(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("\nninja_dyndep_version = 1\n")
}

func TestDyndepParserTest_VersionCRLF(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\r\n")
}

func TestDyndepParserTest_CommentVersionCRLF(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("# comment\r\nninja_dyndep_version = 1\r\n")
}

func TestDyndepParserTest_BlankLineVersionCRLF(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("\r\nninja_dyndep_version = 1\r\n")
}

func TestDyndepParserTest_VersionUnexpectedEOF(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1.0"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: unexpected EOF\nninja_dyndep_version = 1.0\n                          ^ near here" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_UnsupportedVersion0(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 0\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: unsupported 'ninja_dyndep_version = 0'\nninja_dyndep_version = 0\n                        ^ near here" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_UnsupportedVersion1_1(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1.1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: unsupported 'ninja_dyndep_version = 1.1'\nninja_dyndep_version = 1.1\n                          ^ near here" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_DuplicateVersion(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nninja_dyndep_version = 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: unexpected identifier\n" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_MissingVersionOtherVar(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "not_ninja_dyndep_version = 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: expected 'ninja_dyndep_version = ...'\nnot_ninja_dyndep_version = 1\n                            ^ near here" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_MissingVersionBuild(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "build out: dyndep\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: expected 'ninja_dyndep_version = ...'\n" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_UnexpectedEqual(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "= 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: unexpected '='\n" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_UnexpectedIndent(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := " = 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:1: unexpected indent\n" != err {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_OutDuplicate(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild out: dyndep\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:3: multiple statements for 'out'\nbuild out: dyndep\n         ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OutDuplicateThroughOther(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep\nbuild otherout: dyndep\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:3: multiple statements for 'otherout'\nbuild otherout: dyndep\n              ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_NoOutEOF(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: unexpected EOF\nbuild\n     ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_NoOutColon(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild :\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: expected path\nbuild :\n      ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OutNoStatement(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild missing: dyndep\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: no build statement exists for 'missing'\nbuild missing: dyndep\n             ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OutEOF(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: unexpected EOF\nbuild out\n         ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OutNoRule(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out:"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: expected build command name 'dyndep'\nbuild out:\n          ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OutBadRule(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: touch"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: expected build command name 'dyndep'\nbuild out: touch\n           ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_BuildEOF(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal("expected false")
	}
	if "input:2: unexpected EOF\nbuild out: dyndep\n                 ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_ExplicitOut(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out exp: dyndep\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal(err)
	}
	if "input:2: explicit outputs not supported\nbuild out exp: dyndep\n             ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_ExplicitIn(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep exp\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal(err)
	}
	if "input:2: explicit inputs not supported\nbuild out: dyndep exp\n                     ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_OrderOnlyIn(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep ||\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal(err)
	}
	if "input:2: order-only inputs not supported\nbuild out: dyndep ||\n                  ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_BadBinding(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep\n  not_restat = 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal(err)
	}
	if "input:3: binding is not 'restat'\n  not_restat = 1\n                ^ near here" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_RestatTwice(t *testing.T) {
	d := NewDyndepParserTest(t)
	kInput := "ninja_dyndep_version = 1\nbuild out: dyndep\n  restat = 1\n  restat = 1\n"
	parser := NewDyndepParser(&d.state, &d.fs, d.dyndepFile)
	err := ""
	if parser.parseTest(kInput, &err) {
		t.Fatal(err)
	}
	if "input:4: unexpected indent\n" != err {
		t.Fatal(err)
	}
}

func TestDyndepParserTest_NoImplicit(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out: dyndep\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_EmptyImplicit(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out | : dyndep |\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_ImplicitIn(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out: dyndep | impin\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 1 != len(i.implicitInputs) {
		t.Fatal(i.implicitInputs)
	}
	if "impin" != i.implicitInputs[0].Path {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_ImplicitIns(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out: dyndep | impin1 impin2\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 2 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
	if "impin1" != i.implicitInputs[0].Path {
		t.Fatal("expected equal")
	}
	if "impin2" != i.implicitInputs[1].Path {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_ImplicitOut(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out | impout: dyndep\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 1 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if "impout" != i.implicitOutputs[0].Path {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_ImplicitOuts(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out | impout1 impout2 : dyndep\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 2 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if "impout1" != i.implicitOutputs[0].Path {
		t.Fatal("expected equal")
	}
	if "impout2" != i.implicitOutputs[1].Path {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_ImplicitInsAndOuts(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out | impout1 impout2: dyndep | impin1 impin2\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 2 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if "impout1" != i.implicitOutputs[0].Path {
		t.Fatal("expected equal")
	}
	if "impout2" != i.implicitOutputs[1].Path {
		t.Fatal("expected equal")
	}
	if 2 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
	if "impin1" != i.implicitInputs[0].Path {
		t.Fatal("expected equal")
	}
	if "impin2" != i.implicitInputs[1].Path {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_Restat(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild out: dyndep\n  restat = 1\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if true != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_OtherOutput(t *testing.T) {
	d := NewDyndepParserTest(t)
	d.AssertParse("ninja_dyndep_version = 1\nbuild otherout: dyndep\n")

	if 1 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	i := d.dyndepFile[d.state.Edges[0]]
	if i == nil {
		t.Fatal("expected different")
	}
	if false != i.restat {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitOutputs) {
		t.Fatal("expected equal")
	}
	if 0 != len(i.implicitInputs) {
		t.Fatal("expected equal")
	}
}

func TestDyndepParserTest_MultipleEdges(t *testing.T) {
	d := NewDyndepParserTest(t)
	assertParse(t, "build out2: touch\n", &d.state)
	if 2 != len(d.state.Edges) {
		t.Fatal("expected equal")
	}
	if 1 != len(d.state.Edges[1].Outputs) {
		t.Fatal("expected equal")
	}
	if "out2" != d.state.Edges[1].Outputs[0].Path {
		t.Fatal("expected equal")
	}
	if 0 != len(d.state.Edges[0].Inputs) {
		t.Fatal("expected equal")
	}

	d.AssertParse("ninja_dyndep_version = 1\nbuild out: dyndep\nbuild out2: dyndep\n  restat = 1\n")

	if 2 != len(d.dyndepFile) {
		t.Fatal("expected equal")
	}
	{
		i := d.dyndepFile[d.state.Edges[0]]
		if i == nil {
			t.Fatal("expected different")
		}
		if false != i.restat {
			t.Fatal("expected equal")
		}
		if 0 != len(i.implicitOutputs) {
			t.Fatal("expected equal")
		}
		if 0 != len(i.implicitInputs) {
			t.Fatal("expected equal")
		}
	}
	{
		i := d.dyndepFile[d.state.Edges[1]]
		if i == nil {
			t.Fatal("expected different")
		}
		if true != i.restat {
			t.Fatal("expected equal")
		}
		if 0 != len(i.implicitOutputs) {
			t.Fatal("expected equal")
		}
		if 0 != len(i.implicitInputs) {
			t.Fatal("expected equal")
		}
	}
}
