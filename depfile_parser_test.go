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

func parse(t *testing.T, s string) DepfileParser {
	p := DepfileParser{}
	err := ""
	if !p.Parse([]byte(s+"\x00"), &err) || err != "" {
		t.Fatal(err)
	}
	return p
}

func TestDepfileParserTest_Basic(t *testing.T) {
	p := parse(t, "build/ninja.o: ninja.cc ninja.h eval_env.h manifest_parser.h\n")
	if 1 != len(p.outs) {
		t.Fatal(p.outs)
	}
	if "build/ninja.o" != p.outs[0] {
		t.Fatal(p.outs)
	}
	if 4 != len(p.ins) {
		t.Fatal(p.ins)
	}
}

func TestDepfileParserTest_EarlyNewlineAndWhitespace(t *testing.T) {
	_ = parse(t, " \\\n  out: in\n")
}

func TestDepfileParserTest_Continuation(t *testing.T) {
	p := parse(t, "foo.o: \\\n  bar.h baz.h\n")
	if 1 != len(p.outs) {
		t.Fatal(p.outs)
	}
	if "foo.o" != p.outs[0] {
		t.Fatal(p.outs)
	}
	if 2 != len(p.ins) {
		t.Fatal(p.ins)
	}
}

func TestDepfileParserTest_CarriageReturnContinuation(t *testing.T) {
	p := parse(t, "foo.o: \\\r\n  bar.h baz.h\r\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo.o" != p.outs[0] {
		t.FailNow()
	}
	if 2 != len(p.ins) {
		t.FailNow()
	}
}

func TestDepfileParserTest_BackSlashes(t *testing.T) {
	p := parse(t, "Project\\Dir\\Build\\Release8\\Foo\\Foo.res : \\\n  Dir\\Library\\Foo.rc \\\n  Dir\\Library\\Version\\Bar.h \\\n  Dir\\Library\\Foo.ico \\\n  Project\\Thing\\Bar.tlb \\\n")
	if 1 != len(p.outs) {
		t.Fatal(p.outs)
	}
	if "Project\\Dir\\Build\\Release8\\Foo\\Foo.res" != p.outs[0] {
		t.Fatal(p.outs)
	}
	if 4 != len(p.ins) {
		t.Fatal(p.ins)
	}
}

func TestDepfileParserTest_Spaces(t *testing.T) {
	p := parse(t, "a\\ bc\\ def:   a\\ b c d")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "a bc def" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "a b" != p.ins[0] {
		t.FailNow()
	}
	if "c" != p.ins[1] {
		t.FailNow()
	}
	if "d" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleBackslashes(t *testing.T) {
	// Successive 2N+1 backslashes followed by space (' ') are replaced by N >= 0
	// backslashes and the space. A single backslash before hash sign is removed.
	// Other backslashes remain untouched (including 2N backslashes followed by
	// space).
	p := parse(t, "a\\ b\\#c.h: \\\\\\\\\\  \\\\\\\\ \\\\share\\info\\\\#1")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "a b#c.h" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "\\\\ " != p.ins[0] {
		t.FailNow()
	}
	if "\\\\\\\\" != p.ins[1] {
		t.FailNow()
	}
	if "\\\\share\\info\\#1" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_Escapes(t *testing.T) {
	// Put backslashes before a variety of characters, see which ones make
	// it through.
	p := parse(t, "\\!\\@\\#$$\\%\\^\\&\\[\\]\\\\:")
	if 1 != len(p.outs) {
		t.Fatal(p.outs)
	}
	if diff := cmp.Diff("\\!\\@#$\\%\\^\\&\\[\\]\\\\", p.outs[0]); diff != "" {
		t.Fatal(diff)
	}
	if 0 != len(p.ins) {
		t.Fatal(p.ins)
	}
}

func TestDepfileParserTest_EscapedColons(t *testing.T) {
	// Tests for correct parsing of depfiles produced on Windows
	// by both Clang, GCC pre 10 and GCC 10
	p := parse(t, "c\\:\\gcc\\x86_64-w64-mingw32\\include\\stddef.o: \\\n c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.h \n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.o" != p.outs[0] {
		t.FailNow()
	}
	if 1 != len(p.ins) {
		t.FailNow()
	}
	if "c:\\gcc\\x86_64-w64-mingw32\\include\\stddef.h" != p.ins[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_EscapedTargetColon(t *testing.T) {
	p := parse(t, "foo1\\: x\nfoo1\\:\nfoo1\\:\r\nfoo1\\:\t\nfoo1\\:")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo1\\" != p.outs[0] {
		t.FailNow()
	}
	if 1 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_SpecialChars(t *testing.T) {
	// See filenames like istreambuf.iteratorOp!= in
	// https://github.com/google/libcxx/tree/master/test/iterators/stream.iterators/istreambuf.iterator/
	p := parse(t, "C:/Program\\ Files\\ (x86)/Microsoft\\ crtdefs.h: \\\n en@quot.header~ t+t-x!=1 \\\n openldap/slapd.d/cn=config/cn=schema/cn={0}core.ldif\\\n Fu\303\244ball\\\n a[1]b@2%c")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "C:/Program Files (x86)/Microsoft crtdefs.h" != p.outs[0] {
		t.FailNow()
	}
	if 5 != len(p.ins) {
		t.FailNow()
	}
	if "en@quot.header~" != p.ins[0] {
		t.FailNow()
	}
	if "t+t-x!=1" != p.ins[1] {
		t.FailNow()
	}
	if "openldap/slapd.d/cn=config/cn=schema/cn={0}core.ldif" != p.ins[2] {
		t.FailNow()
	}
	if "Fu\303\244ball" != p.ins[3] {
		t.FailNow()
	}
	if "a[1]b@2%c" != p.ins[4] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleOutputs(t *testing.T) {
	// check that multiple duplicate targets are properly unified
	p := parse(t, "foo foo: x y z")
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleDifferentOutputs(t *testing.T) {
	// check that multiple different outputs are accepted by the parser
	p := parse(t, "foo bar: x y z")
	if 2 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if "bar" != p.outs[1] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleEmptyRules(t *testing.T) {
	p := parse(t, "foo: x\nfoo: \nfoo:\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 1 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleRulesLF(t *testing.T) {
	p := parse(t, "foo: x\nfoo: y\nfoo \\\nfoo: z\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMultipleRulesCRLF(t *testing.T) {
	p := parse(t, "foo: x\r\nfoo: y\r\nfoo \\\r\nfoo: z\r\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMixedRulesLF(t *testing.T) {
	p := parse(t, "foo: x\\\n     y\nfoo \\\nfoo: z\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_UnifyMixedRulesCRLF(t *testing.T) {
	p := parse(t, "foo: x\\\r\n     y\r\nfoo \\\r\nfoo: z\r\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_IndentedRulesLF(t *testing.T) {
	p := parse(t, " foo: x\n foo: y\n foo: z\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_IndentedRulesCRLF(t *testing.T) {
	p := parse(t, " foo: x\r\n foo: y\r\n foo: z\r\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_TolerateMP(t *testing.T) {
	p := parse(t, "foo: x y z\nx:\ny:\nz:\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleRulesTolerateMP(t *testing.T) {
	p := parse(t, "foo: x\nx:\nfoo: y\ny:\nfoo: z\nz:\n")
	if 1 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_MultipleRulesDifferentOutputs(t *testing.T) {
	// check that multiple different outputs are accepted by the parser
	// when spread across multiple rules
	p := parse(t, "foo: x y\nbar: y z\n")
	if 2 != len(p.outs) {
		t.FailNow()
	}
	if "foo" != p.outs[0] {
		t.FailNow()
	}
	if "bar" != p.outs[1] {
		t.FailNow()
	}
	if 3 != len(p.ins) {
		t.FailNow()
	}
	if "x" != p.ins[0] {
		t.FailNow()
	}
	if "y" != p.ins[1] {
		t.FailNow()
	}
	if "z" != p.ins[2] {
		t.FailNow()
	}
}

func TestDepfileParserTest_BuggyMP(t *testing.T) {
	err := ""
	p := DepfileParser{}
	if p.Parse([]byte("foo: x y z\nx: alsoin\ny:\nz:\n\x00"), &err) {
		t.Error("unexpected Parse success")
	}
	if "inputs may not also have inputs" != err {
		t.Fatal(err)
	}
}
