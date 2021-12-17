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

import "testing"

func TestDepfileParserTest_Basic(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("build/ninja.o: ninja.cc ninja.h eval_env.h manifest_parser.h\n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.Fatal(err)
	}
	if 1 != len(p.outs_) {
		t.Fatal(p.outs_)
	}
	if "build/ninja.o" != p.outs_[0] {
		t.Fatal(p.outs_)
	}
	if 4 != len(p.ins_) {
		t.Fatal(p.ins_)
	}
}

func TestDepfileParserTest_EarlyNewlineAndWhitespace(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte(" \\\n  out: in\n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.Fatal(err)
	}
}

func TestDepfileParserTest_Continuation(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo.o: \\\n  bar.h baz.h\n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.Fatal(err)
	}
	if 1 != len(p.outs_) {
		t.Fatal(p.outs_)
	}
	if "foo.o" != p.outs_[0] {
		t.Fatal(p.outs_)
	}
	if 2 != len(p.ins_) {
		t.Fatal(p.ins_)
	}
}

func TestDepfileParserTest_CarriageReturnContinuation(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo.o: \\\r\n  bar.h baz.h\r\n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo.o" != p.outs_[0] {
		t.FailNow()
	}
	if 2 != len(p.ins_) {
		t.FailNow()
	}
}

func TestDepfileParserTest_BackSlashes(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("Project\\Dir\\Build\\Release8\\Foo\\Foo.res : \\\n  Dir\\Library\\Foo.rc \\\n  Dir\\Library\\Version\\Bar.h \\\n  Dir\\Library\\Foo.ico \\\n  Project\\Thing\\Bar.tlb \\\n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.Fatal(err)
	}
	if 1 != len(p.outs_) {
		t.Fatal(p.outs_)
	}
	if "Project\\Dir\\Build\\Release8\\Foo\\Foo.res" != p.outs_[0] {
		t.Fatal(p.outs_)
	}
	if 4 != len(p.ins_) {
		t.Fatal(p.ins_)
	}
}

func TestDepfileParserTest_Spaces(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("a\\ bc\\ def:   a\\ b c d"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "a bc def" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "a b" != p.ins_[0] {
		t.FailNow()
	}
	if "c" != p.ins_[1] {
		t.FailNow()
	}
	if "d" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleBackslashes(t *testing.T) {
	// Successive 2N+1 backslashes followed by space (' ') are replaced by N >= 0
	// backslashes and the space. A single backslash before hash sign is removed.
	// Other backslashes remain untouched (including 2N backslashes followed by
	// space).
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("a\\ b\\#c.h: \\\\\\\\\\  \\\\\\\\ \\\\share\\info\\\\#1"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "a b#c.h" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "\\\\ " != p.ins_[0] {
		t.FailNow()
	}
	if "\\\\\\\\" != p.ins_[1] {
		t.FailNow()
	}
	if "\\\\share\\info\\#1" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_Escapes(t *testing.T) {
	// Put backslashes before a variety of characters, see which ones make
	// it through.
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("\\!\\@\\#$$\\%\\^\\&\\[\\]\\\\:"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.Fatal(err)
	}
	if 1 != len(p.outs_) {
		t.Fatal(p.outs_)
	}
	if "\\!\\@#$\\%\\^\\&\\[\\]\\\\" != p.outs_[0] {
		t.Fatal(p.outs_)
	}
	if 0 != len(p.ins_) {
		t.Fatal(p.ins_)
	}
}

func TestDepfileParserTest_EscapedColons(t *testing.T) {
	err := ""
	p := DepfileParser{}
	// Tests for correct parsing of depfiles produced on Windows
	// by both Clang, GCC pre 10 and GCC 10
	if !p.Parse([]byte("c\\:\\gcc\\x86_64-w64-mingw32\\include\\stddef.o: \\\n c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.h \n"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.o" != p.outs_[0] {
		t.FailNow()
	}
	if 1 != len(p.ins_) {
		t.FailNow()
	}
	if "c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.h" != p.ins_[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_EscapedTargetColon(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo1\\: x\nfoo1\\:\nfoo1\\:\r\nfoo1\\:\t\nfoo1\\:"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo1\\" != p.outs_[0] {
		t.FailNow()
	}
	if 1 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_SpecialChars(t *testing.T) {
	// See filenames like istreambuf.iterator_op!= in
	// https://github.com/google/libcxx/tree/master/test/iterators/stream.iterators/istreambuf.iterator/
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("C:/Program\\ Files\\ (x86)/Microsoft\\ crtdefs.h: \\\n en@quot.header~ t+t-x!=1 \\\n openldap/slapd.d/cn=config/cn=schema/cn={0}core.ldif\\\n Fu\303\244ball\\\n a[1]b@2%c"), &err) {
		t.Error("Parse failure")
	}
	if "" != err {
		t.FailNow()
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "C:/Program Files (x86)/Microsoft crtdefs.h" != p.outs_[0] {
		t.FailNow()
	}
	if 5 != len(p.ins_) {
		t.FailNow()
	}
	if "en@quot.header~" != p.ins_[0] {
		t.FailNow()
	}
	if "t+t-x!=1" != p.ins_[1] {
		t.FailNow()
	}
	if "openldap/slapd.d/cn=config/cn=schema/cn={0}core.ldif" != p.ins_[2] {
		t.FailNow()
	}
	if "Fu\303\244ball" != p.ins_[3] {
		t.FailNow()
	}
	if "a[1]b@2%c" != p.ins_[4] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleOutputs(t *testing.T) {
	// check that multiple duplicate targets are properly unified
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo foo: x y z"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleDifferentOutputs(t *testing.T) {
	// check that multiple different outputs are accepted by the parser
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo bar: x y z"), &err) {
		t.Error("Parse failure")
	}
	if 2 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if "bar" != p.outs_[1] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleEmptyRules(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\nfoo: \nfoo:\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 1 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleRulesLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\nfoo: y\nfoo \\\nfoo: z\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleRulesCRLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\r\nfoo: y\r\nfoo \\\r\nfoo: z\r\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMixedRulesLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\\\n     y\nfoo \\\nfoo: z\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMixedRulesCRLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\\\r\n     y\r\nfoo \\\r\nfoo: z\r\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_IndentedRulesLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte(" foo: x\n foo: y\n foo: z\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_IndentedRulesCRLF(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte(" foo: x\r\n foo: y\r\n foo: z\r\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_TolerateMP(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x y z\nx:\ny:\nz:\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleRulesTolerateMP(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x\nx:\nfoo: y\ny:\nfoo: z\nz:\n"), &err) {
		t.Error("Parse failure")
	}
	if 1 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleRulesDifferentOutputs(t *testing.T) {
	// check that multiple different outputs are accepted by the parser
	// when spread across multiple rules
	err := ""
	p := DepfileParser{}
	if !p.Parse([]byte("foo: x y\nbar: y z\n"), &err) {
		t.Error("Parse failure")
	}
	if 2 != len(p.outs_) {
		t.FailNow()
	}
	if "foo" != p.outs_[0] {
		t.FailNow()
	}
	if "bar" != p.outs_[1] {
		t.FailNow()
	}
	if 3 != len(p.ins_) {
		t.FailNow()
	}
	if "x" != p.ins_[0] {
		t.FailNow()
	}
	if "y" != p.ins_[1] {
		t.FailNow()
	}
	if "z" != p.ins_[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_BuggyMP(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if p.Parse([]byte("foo: x y z\nx: alsoin\ny:\nz:\n"), &err) {
		t.Error("unexpected Parse success")
	}
	if "inputs may not also have inputs" != err {
		t.Fatal(err)
	}
}
