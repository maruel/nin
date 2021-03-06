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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var concurrencyVals = []ParseManifestConcurrency{
	ParseManifestSerial,
	ParseManifestPrewarmSubninja,
	ParseManifestConcurrentParsing,
}

type ParserTest struct {
	t           *testing.T
	state       State
	fs          VirtualFileSystem
	Concurrency ParseManifestConcurrency
}

func NewParserTest(t *testing.T, c ParseManifestConcurrency) ParserTest {
	return ParserTest{
		t:           t,
		state:       NewState(),
		fs:          NewVirtualFileSystem(),
		Concurrency: c,
	}
}

func (p *ParserTest) assertParse(input string) {
	opts := ParseManifestOpts{
		Quiet:       true,
		Concurrency: p.Concurrency,
	}
	if err := p.parseTest(input, opts); err != nil {
		p.t.Helper()
		p.t.Fatal(err)
	}
	verifyGraph(p.t, &p.state)
}

func (p *ParserTest) parseTest(input string, opts ParseManifestOpts) error {
	return ParseManifest(&p.state, &p.fs, opts, "input", []byte(input+"\x00"))
}

func TestParserTest_Empty(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("")
		})
	}
}

func TestParserTest_Rules(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\n\nrule date\n  command = date > $out\n\nbuild result: cat in_1.cc in-2.O\n")

			if 3 != len(p.state.Bindings.Rules) {
				t.Fatal("expected equal")
			}
			rule := p.state.Bindings.Rules["cat"]
			if got := rule.Name; got != "cat" {
				t.Fatal(got)
			}
			// The C++ version of EvalString concatenates text to reduce the array slice.
			// This is slower in Go in practice.
			// Original: "[cat ][$in][ > ][$out]"
			if got := rule.Bindings["command"].Serialize(); got != "[cat][ ][$in][ ][>][ ][$out]" {
				t.Fatal(got)
			}
		})
	}
}

func TestParserTest_RuleAttributes(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			// Check that all of the allowed rule attributes are parsed ok.
			p.assertParse("rule cat\n  command = a\n  depfile = a\n  deps = a\n  description = a\n  generator = a\n  restat = a\n  rspfile = a\n  rspfile_content = a\n")
		})
	}
}

func TestParserTest_IgnoreIndentedComments(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("  #indented comment\nrule cat\n  command = cat $in > $out\n  #generator = 1\n  restat = 1 # comment\n  #comment\nbuild result: cat in_1.cc in-2.O\n  #comment\n")

			if 2 != len(p.state.Bindings.Rules) {
				t.Fatal("expected equal")
			}
			rule := p.state.Bindings.Rules["cat"]
			if "cat" != rule.Name {
				t.Fatal("expected equal")
			}
			edge := p.state.GetNode("result", 0).InEdge
			if edge.GetBinding("restat") == "" {
				t.Fatal("expected true")
			}
			if edge.GetBinding("generator") != "" {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_IgnoreIndentedBlankLines(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			// the indented blanks used to cause parse errors
			p.assertParse("  \nrule cat\n  command = cat $in > $out\n  \nbuild result: cat in_1.cc in-2.O\n  \nvariable=1\n")

			// the variable must be in the top level environment
			if "1" != p.state.Bindings.LookupVariable("variable") {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_ResponseFiles(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat_rsp\n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $in\n\nbuild out: cat_rsp in\n  rspfile=out.rsp\n")

			if 2 != len(p.state.Bindings.Rules) {
				t.Fatal("expected equal")
			}
			rule := p.state.Bindings.Rules["cat_rsp"]
			if "cat_rsp" != rule.Name {
				t.Fatal("expected equal")
			}
			// The C++ version of EvalString concatenates text to reduce the array slice.
			// This is slower in Go in practice.
			// Original: "[cat ][$rspfile][ > ][$out]"
			if got := rule.Bindings["command"].Serialize(); got != "[cat][ ][$rspfile][ ][>][ ][$out]" {
				t.Fatal(got)
			}
			if "[$rspfile]" != rule.Bindings["rspfile"].Serialize() {
				t.Fatal("expected equal")
			}
			if "[$in]" != rule.Bindings["rspfile_content"].Serialize() {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_InNewline(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat_rsp\n  command = cat $in_newline > $out\n\nbuild out: cat_rsp in in2\n  rspfile=out.rsp\n")

			if 2 != len(p.state.Bindings.Rules) {
				t.Fatal("expected equal")
			}
			rule := p.state.Bindings.Rules["cat_rsp"]
			if "cat_rsp" != rule.Name {
				t.Fatal("expected equal")
			}
			// The C++ version of EvalString concatenates text to reduce the array slice.
			// This is slower in Go in practice.
			// Original: "[cat ][$in_newline][ > ][$out]"
			if got := rule.Bindings["command"].Serialize(); got != "[cat][ ][$in_newline][ ][>][ ][$out]" {
				t.Fatal(got)
			}

			edge := p.state.Edges[0]
			if "cat in\nin2 > out" != edge.EvaluateCommand(false) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_Variables(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("l = one-letter-test\nrule link\n  command = ld $l $extra $with_under -o $out $in\n\nextra = -pthread\nwith_under = -under\nbuild a: link b c\nnested1 = 1\nnested2 = $nested1/2\nbuild supernested: link x\n  extra = $nested2/3\n")

			if 2 != len(p.state.Edges) {
				t.Fatalf("%v", p.state.Edges)
			}
			edge := p.state.Edges[0]
			if got := edge.EvaluateCommand(false); "ld one-letter-test -pthread -under -o a b c" != got {
				t.Fatal(got)
			}
			if "1/2" != p.state.Bindings.LookupVariable("nested2") {
				t.Fatal("expected equal")
			}

			edge = p.state.Edges[1]
			if "ld one-letter-test 1/2/3 -under -o supernested x" != edge.EvaluateCommand(false) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_VariableScope(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("foo = bar\nrule cmd\n  command = cmd $foo $in $out\n\nbuild inner: cmd a\n  foo = baz\nbuild outer: cmd b\n\n") // Extra newline after build line tickles a regression.

			if 2 != len(p.state.Edges) {
				t.Fatal("expected equal")
			}
			if "cmd baz a inner" != p.state.Edges[0].EvaluateCommand(false) {
				t.Fatal("expected equal")
			}
			if "cmd bar b outer" != p.state.Edges[1].EvaluateCommand(false) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_Continuation(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule link\n  command = foo bar $\n    baz\n\nbuild a: link c $\n d e f\n")

			if 2 != len(p.state.Bindings.Rules) {
				t.Fatal("expected equal")
			}
			rule := p.state.Bindings.Rules["link"]
			if "link" != rule.Name {
				t.Fatal("expected equal")
			}
			// The C++ version of EvalString concatenates text to reduce the array slice.
			// This is slower in Go in practice.
			// Original: "[foo bar baz]"
			if got := rule.Bindings["command"].Serialize(); got != "[foo][ ][bar][ ][baz]" {
				t.Fatal(got)
			}
		})
	}
}

func TestParserTest_Backslash(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("foo = bar\\baz\nfoo2 = bar\\ baz\n")
			if "bar\\baz" != p.state.Bindings.LookupVariable("foo") {
				t.Fatal("expected equal")
			}
			if "bar\\ baz" != p.state.Bindings.LookupVariable("foo2") {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_Comment(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("# this is a comment\nfoo = not # a comment\n")
			if "not # a comment" != p.state.Bindings.LookupVariable("foo") {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_Dollars(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule foo\n  command = ${out}bar$$baz$$$\nblah\nx = $$dollar\nbuild $x: foo y\n")
			if "$dollar" != p.state.Bindings.LookupVariable("x") {
				t.Fatal("expected equal")
			}
			want := "'$dollar'bar$baz$blah"
			if runtime.GOOS == "windows" {
				want = "$dollarbar$baz$blah"
			}
			if want != p.state.Edges[0].EvaluateCommand(false) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_EscapeSpaces(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule spaces\n  command = something\nbuild foo$ bar: spaces $$one two$$$ three\n")
			if p.state.Paths["foo bar"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Edges[0].Outputs[0].Path != "foo bar" {
				t.Fatal("expected equal")
			}
			if p.state.Edges[0].Inputs[0].Path != "$one" {
				t.Fatal("expected equal")
			}
			if p.state.Edges[0].Inputs[1].Path != "two$ three" {
				t.Fatal("expected equal")
			}
			if p.state.Edges[0].EvaluateCommand(false) != "something" {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_CanonicalizeFile(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in/1 in//2\nbuild in/1: cat\nbuild in/2: cat\n")

			if p.state.Paths["in/1"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["in/2"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["in//1"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["in//2"] != nil {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_CanonicalizeFileBackslashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in\\1 in\\\\2\nbuild in\\1: cat\nbuild in\\2: cat\n")

			node := p.state.Paths["in/1"]
			if node == nil {
				t.Fatal("expected true")
			}
			if 1 != node.SlashBits {
				t.Fatal("expected equal")
			}
			node = p.state.Paths["in/2"]
			if node == nil {
				t.Fatal("expected true")
			}
			if 1 != node.SlashBits {
				t.Fatal("expected equal")
			}
			if p.state.Paths["in//1"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["in//2"] != nil {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_PathVariables(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\ndir = out\nbuild $dir/exe: cat src\n")

			if p.state.Paths["$dir/exe"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["out/exe"] == nil {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_CanonicalizePaths(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild ./out.o: cat ./bar/baz/../foo.cc\n")

			if p.state.Paths["./out.o"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["out.o"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["./bar/baz/../foo.cc"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["bar/foo.cc"] == nil {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_CanonicalizePathsBackslashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild ./out.o: cat ./bar/baz/../foo.cc\nbuild .\\out2.o: cat .\\bar/baz\\..\\foo.cc\nbuild .\\out3.o: cat .\\bar\\baz\\..\\foo3.cc\n")

			if p.state.Paths["./out.o"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths[".\\out2.o"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths[".\\out3.o"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths["out.o"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["out2.o"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["out3.o"] == nil {
				t.Fatal("expected true")
			}
			if p.state.Paths["./bar/baz/../foo.cc"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths[".\\bar/baz\\..\\foo.cc"] != nil {
				t.Fatal("expected false")
			}
			if p.state.Paths[".\\bar/baz\\..\\foo3.cc"] != nil {
				t.Fatal("expected false")
			}
			node := p.state.Paths["bar/foo.cc"]
			if node == nil {
				t.Fatal("expected true")
			}
			if 0 != node.SlashBits {
				t.Fatal("expected equal")
			}
			node = p.state.Paths["bar/foo3.cc"]
			if node == nil {
				t.Fatal("expected true")
			}
			if 1 != node.SlashBits {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DuplicateEdgeWithMultipleOutputs(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n")
			// assertParse() checks that the generated build graph is self-consistent.
			// That's all the checking that this test needs.
		})
	}
}

func TestParserTest_NoDeadPointerFromDuplicateEdge(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild out: cat in\nbuild out: cat in\n")
			// assertParse() checks that the generated build graph is self-consistent.
			// That's all the checking that this test needs.
		})
	}
}

func TestParserTest_DuplicateEdgeWithMultipleOutputsError(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			input := "rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n"
			opts := ParseManifestOpts{
				ErrOnDupeEdge: true,
				Concurrency:   p.Concurrency,
			}
			if err := p.parseTest(input, opts); err == nil {
				t.Fatal("expected false")
			} else if err.Error() != "input:5: multiple rules generate out1\n" {
				t.Fatal(err)
			}
		})
	}
}

func TestParserTest_DuplicateEdgeInIncludedFile(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("sub.ninja", "rule cat\n  command = cat $in > $out\nbuild out1 out2: cat in1\nbuild out1: cat in2\nbuild final: cat out1\n")
			input := "subninja sub.ninja\n"
			opts := ParseManifestOpts{
				ErrOnDupeEdge: true,
				Concurrency:   p.Concurrency,
			}
			if err := p.parseTest(input, opts); err == nil {
				t.Fatal("expected false")
			} else if err.Error() != "sub.ninja:5: multiple rules generate out1\n" {
				t.Fatalf("%q", err)
			}
		})
	}
}

func TestParserTest_PhonySelfReferenceIgnored(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("build a: phony a\n")

			node := p.state.Paths["a"]
			edge := node.InEdge
			if len(edge.Inputs) != 0 {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_PhonySelfReferenceKept(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			input := "build a: phony a\n"
			opts := ParseManifestOpts{
				ErrOnPhonyCycle: true,
				Concurrency:     p.Concurrency,
			}
			if err := p.parseTest(input, opts); err != nil {
				t.Fatal(err)
			}

			node := p.state.Paths["a"]
			edge := node.InEdge
			if len(edge.Inputs) != 1 {
				t.Fatal("expected equal")
			}
			if edge.Inputs[0] != node {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_ReservedWords(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule build\n  command = rule run $out\nbuild subninja: build include default foo.cc\ndefault subninja\n")
		})
	}
}

func TestParserTest_Errors(t *testing.T) {
	data := []struct {
		in   string
		want string
	}{
		{"subn", "input:1: expected '=', got eof\nsubn\n    ^ near here"},

		{"foobar", "input:1: expected '=', got eof\nfoobar\n      ^ near here"},
		{"x 3", "input:1: expected '=', got identifier\nx 3\n  ^ near here"},
		{"x = 3", "input:1: unexpected EOF\nx = 3\n     ^ near here"},
		{
			"x = 3\ny 2",
			"input:2: expected '=', got identifier\ny 2\n  ^ near here",
		},
		{
			"x = $",
			"input:1: bad $-escape (literal $ must be written as $$)\nx = $\n    ^ near here",
		},
		{
			"x = $\n $[\n",
			"input:2: bad $-escape (literal $ must be written as $$)\n $[\n ^ near here",
		},
		{
			"x = a$\n b$\n $\n",
			"input:4: unexpected EOF\n",
		},
		{
			"build\n",
			"input:1: expected path\nbuild\n     ^ near here",
		},
		{
			"build x: y z\n",
			"input:1: unknown build rule 'y'\nbuild x: y z\n         ^ near here",
		},
		{
			"build x:: y z\n",
			"input:1: expected build command name\nbuild x:: y z\n        ^ near here",
		},
		{
			"rule cat\n  command = cat ok\nbuild x: cat $\n :\n",
			"input:4: expected newline, got ':'\n :\n ^ near here",
		},
		{
			"rule cat\n",
			"input:2: expected 'command =' line\n",
		},
		{
			"rule cat\n  command = echo\nrule cat\n  command = echo\n",
			"input:3: duplicate rule 'cat'\nrule cat\n        ^ near here",
		},
		{
			"rule cat\n  command = echo\n  rspfile = cat.rsp\n",
			"input:4: rspfile and rspfile_content need to be both specified\n",
		},
		{
			"rule cat\n  command = ${fafsd\nfoo = bar\n",
			"input:2: bad $-escape (literal $ must be written as $$)\n  command = ${fafsd\n            ^ near here",
		},
		{
			"rule cat\n  command = cat\nbuild $.: cat foo\n",
			"input:3: bad $-escape (literal $ must be written as $$)\nbuild $.: cat foo\n      ^ near here",
		},
		{
			"rule cat\n  command = cat\nbuild $: cat foo\n",
			"input:3: expected ':', got newline ($ also escapes ':')\nbuild $: cat foo\n                ^ near here",
		},
		{
			"rule %foo\n",
			"input:1: expected rule name\nrule %foo\n     ^ near here",
		},
		{
			"rule cc\n  command = foo\n  othervar = bar\n",
			"input:3: unexpected variable 'othervar'\n  othervar = bar\n                ^ near here",
		},
		{
			"rule cc\n  command = foo\nbuild $.: cc bar.cc\n",
			"input:3: bad $-escape (literal $ must be written as $$)\nbuild $.: cc bar.cc\n      ^ near here",
		},
		{
			"rule cc\n  command = foo\n  && bar",
			"input:3: expected variable name\n  && bar\n  ^ near here",
		},
		{
			"rule cc\n  command = foo\nbuild $: cc bar.cc\n",
			"input:3: expected ':', got newline ($ also escapes ':')\nbuild $: cc bar.cc\n                  ^ near here",
		},
		{
			"default\n",
			"input:1: expected target name\ndefault\n       ^ near here",
		},
		{
			"default nonexistent\n",
			"input:1: unknown target 'nonexistent'\ndefault nonexistent\n                   ^ near here",
		},
		{
			"rule r\n  command = r\nbuild b: r\ndefault b:\n",
			"input:4: expected newline, got ':'\ndefault b:\n         ^ near here",
		},
		{
			"default $a\n",
			"input:1: empty path\ndefault $a\n          ^ near here",
		},
		{
			"rule r\n  command = r\nbuild $a: r $c\n",
			// XXX the line number is wrong; we should evaluate paths in ParseEdge
			// as we see them, not after we've read them all!
			"input:4: empty path\n",
		},
		{
			// the indented blank line must terminate the rule
			// this also verifies that "unexpected (token)" errors are correct
			"rule r\n  command = r\n  \n  generator = 1\n",
			"input:4: unexpected indent\n",
		},
		{
			"pool\n",
			"input:1: expected pool name\npool\n    ^ near here",
		},
		{
			"pool foo\n",
			"input:2: expected 'depth =' line\n",
		},
		{
			"pool foo\n  depth = -1\n",
			"input:2: invalid pool depth\n  depth = -1\n            ^ near here",
		},
		{
			"pool foo\n  bar = 1\n",
			"input:2: unexpected variable 'bar'\n  bar = 1\n         ^ near here",
		},
		{
			// Pool names are dereferenced at edge parsing time.
			"rule run\n  command = echo\n  pool = unnamed_pool\nbuild out: run in\n",
			"input:5: unknown pool name 'unnamed_pool'\n",
		},
		// New test not in C++.
		{
			// MissingIncluded
			"include missing.ninja\n",
			"input:1: loading 'missing.ninja': file does not exist\ninclude missing.ninja\n                     ^ near here",
		},
		{
			// MissingSubninja
			"subninja foo.ninja\n",
			"input:1: loading 'foo.ninja': file does not exist\nsubninja foo.ninja\n                  ^ near here",
		},
		{
			// DyndepNotInput
			"rule touch\n  command = touch $out\nbuild result: touch\n  dyndep = notin\n",
			"input:5: dyndep 'notin' is not an input\n",
		},
	}
	for i, line := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for _, c := range concurrencyVals {
				t.Run(c.String(), func(t *testing.T) {
					p := NewParserTest(t, c)
					opts := ParseManifestOpts{
						Concurrency: p.Concurrency,
					}
					if err := p.parseTest(line.in, opts); err == nil {
						t.Fatal("expected error")
					} else if err.Error() != line.want {
						t.Fatal(cmp.Diff(line.want, err.Error()))
					}
				})
			}
		})
	}

	t.Run("pool", func(t *testing.T) {
		for _, c := range concurrencyVals {
			t.Run(c.String(), func(t *testing.T) {
				p := NewParserTest(t, c)
				opts := ParseManifestOpts{
					Concurrency: p.Concurrency,
				}
				// When parsing in concurrent mode, the full pool is processed before
				// duplication check is done.
				in := "pool foo\n  depth = 4\npool foo\n"
				if c == ParseManifestConcurrentParsing {
					in = "pool foo\n  depth = 4\npool foo\n  depth = 4\n"
				}
				want := "input:3: duplicate pool 'foo'\npool foo\n        ^ near here"
				if err := p.parseTest(in, opts); err == nil {
					t.Fatal("expected error")
				} else if err.Error() != want {
					t.Fatal(cmp.Diff(want, err.Error()))
				}
			})
		}
	})

}

func TestParserTest_MultipleOutputs(t *testing.T) {
	// The original C++ test uses a local state, not clear why.
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cc\n  command = foo\n  depfile = bar\nbuild a.o b.o: cc c.cc\n")
		})
	}
}

func TestParserTest_MultipleOutputsWithDeps(t *testing.T) {
	// The original C++ test uses a local state, not clear why.
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cc\n  command = foo\n  deps = gcc\nbuild a.o b.o: cc c.cc\n")
		})
	}
}

func TestParserTest_SubNinja(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("test.ninja", "var2 = inner\nbuild $builddir/inner: varref\n")
			p.assertParse("builddir = some_dir/\nrule varref\n  command = varref $var2\nvar2 = outer\nbuild $builddir/outer: varref\nsubninja test.ninja\nbuild $builddir/outer2: varref\n")
			if 1 != len(p.fs.filesRead) {
				t.Fatal("expected equal")
			}

			if "test.ninja" != p.fs.filesRead[0] {
				t.Fatal("expected equal")
			}
			if p.state.Paths["some_dir/outer"] == nil {
				t.Fatal("expected true")
			}
			// Verify our builddir setting is inherited.
			if p.state.Paths["some_dir/inner"] == nil {
				t.Fatal("expected true")
			}

			if 3 != len(p.state.Edges) {
				t.Fatal("expected equal")
			}
			if c == ParseManifestSerial {
				// Serial execution, ensure determinism.
				if got := p.state.Edges[0].EvaluateCommand(false); got != "varref outer" {
					t.Fatal(got)
				}
				if got := p.state.Edges[1].EvaluateCommand(false); got != "varref inner" {
					t.Fatal(got)
				}
				if got := p.state.Edges[2].EvaluateCommand(false); got != "varref outer" {
					t.Fatal(got)
				}
			} else {
				// The order of the edges can be non-deterministic with parallel subninja
				// execution.
				found := []string{
					p.state.Edges[0].EvaluateCommand(false),
					p.state.Edges[1].EvaluateCommand(false),
					p.state.Edges[2].EvaluateCommand(false),
				}
				sort.Strings(found)
				want := []string{"varref inner", "varref outer", "varref outer"}
				if diff := cmp.Diff(want, found); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestParserTest_DuplicateRuleInDifferentSubninjas(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			// Test that rules are scoped to subninjas.
			p.fs.Create("test.ninja", "rule cat\n  command = cat\n")
			p.assertParse("rule cat\n  command = cat\nsubninja test.ninja\n")
		})
	}
}

func TestParserTest_DuplicateRuleInDifferentSubninjasWithInclude(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			// Test that rules are scoped to subninjas even with includes.
			p.fs.Create("rules.ninja", "rule cat\n  command = cat\n")
			p.fs.Create("test.ninja", "include rules.ninja\nbuild x : cat\n")
			p.assertParse("include rules.ninja\nsubninja test.ninja\nbuild y : cat\n")
		})
	}
}

func TestParserTest_SubNinjaGrandChildren(t *testing.T) {
	// A more complicated version of TestParserTest_SubNinja.
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("child.ninja", "var2 = inner\nsubninja $grand\n")
			p.fs.Create("grandchild.ninja", "build $builddir/inner: varref\n")
			p.assertParse("builddir = some_dir/\nrule varref\n  command = varref $var2\nvar2 = outer\ngrand = grandchild.ninja\nbuild $builddir/outer: varref\nsubninja child.ninja\nbuild $builddir/outer2: varref\n")

			want := []string{"child.ninja", "grandchild.ninja"}
			if diff := cmp.Diff(want, p.fs.filesRead); diff != "" {
				t.Error(diff)
			}
			if p.state.Paths["some_dir/outer"] == nil {
				t.Fatal("expected true")
			}
			// Verify our builddir setting is inherited.
			if p.state.Paths["some_dir/inner"] == nil {
				t.Fatal("expected true")
			}

			if c == ParseManifestSerial {
				// Serial execution, ensure determinism.
				if got := p.state.Edges[0].EvaluateCommand(false); got != "varref outer" {
					t.Fatal(got)
				}
				if got := p.state.Edges[1].EvaluateCommand(false); got != "varref inner" {
					t.Fatal(got)
				}
				if got := p.state.Edges[2].EvaluateCommand(false); got != "varref outer" {
					t.Fatal(got)
				}
			} else {
				// The order of the edges can be non-deterministic with parallel subninja
				// execution.
				found := []string{
					p.state.Edges[0].EvaluateCommand(false),
					p.state.Edges[1].EvaluateCommand(false),
					p.state.Edges[2].EvaluateCommand(false),
				}
				sort.Strings(found)
				want := []string{"varref inner", "varref outer", "varref outer"}
				if diff := cmp.Diff(want, found); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

// Test not in C++.
func TestParserTest_Grandchild_SubNinjaError(t *testing.T) {
	// Ensure thats subninja->include->subninja outputs the error on the right file.
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("child.ninja", "include grandchild.ninja\n")
			p.fs.Create("grandchild.ninja", "var_log = bars\nsubninja missing_grandchild.ninja\n")
			opts := ParseManifestOpts{
				Concurrency: p.Concurrency,
			}
			want := "grandchild.ninja:2: loading 'missing_grandchild.ninja': file does not exist\n" +
				"subninja missing_grandchild.ninja\n" +
				"                                 ^ near here"
			if err := p.parseTest("var1 = foo\nsubninja child.ninja\n", opts); err == nil {
				t.Fatal("expected error")
			} else if err.Error() != want {
				t.Fatal(cmp.Diff(want, err.Error()))
			}
		})
	}
}

// Test not in C++.
func TestParserTest_Grandchild_IncludeError(t *testing.T) {
	// Ensure thats include->subninja->include outputs the error on the right file.
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("child.ninja", "subninja grandchild.ninja\n")
			p.fs.Create("grandchild.ninja", "var_log = bars\ninclude missing_grandchild.ninja\n")
			opts := ParseManifestOpts{
				Concurrency: p.Concurrency,
			}
			want := "grandchild.ninja:2: loading 'missing_grandchild.ninja': file does not exist\n" +
				"include missing_grandchild.ninja\n" +
				"                                ^ near here"
			if err := p.parseTest("var1 = foo\ninclude child.ninja\n", opts); err == nil {
				t.Fatal("expected error")
			} else if err.Error() != want {
				t.Fatal(cmp.Diff(want, err.Error()))
			}
		})
	}
}

func TestParserTest_Include(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("include.ninja", "var2 = inner\n")
			p.assertParse("var2 = outer\ninclude include.ninja\n")

			if 1 != len(p.fs.filesRead) {
				t.Fatal("expected equal")
			}
			if "include.ninja" != p.fs.filesRead[0] {
				t.Fatal("expected equal")
			}
			if "inner" != p.state.Bindings.LookupVariable("var2") {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_BrokenInclude(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.fs.Create("include.ninja", "build\n")
			opts := ParseManifestOpts{
				Concurrency: p.Concurrency,
			}
			if err := p.parseTest("include include.ninja\n", opts); err == nil {
				t.Fatal("expected false")
			} else if err.Error() != "include.ninja:1: expected path\nbuild\n     ^ near here" {
				t.Fatalf("%q", err)
			}
		})
	}
}

func TestParserTest_Implicit(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo: cat bar | baz\n")

			edge := p.state.Paths["foo"].InEdge
			if !edge.IsImplicit(1) {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_OrderOnly(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo: cat bar || baz\n")

			edge := p.state.Paths["foo"].InEdge
			if !edge.IsOrderOnly(1) {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_Validations(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo: cat bar |@ baz\n")

			edge := p.state.Paths["foo"].InEdge
			if len(edge.Validations) != 1 {
				t.Fatal(edge.Validations)
			}
			if edge.Validations[0].Path != "baz" {
				t.Fatal(edge.Validations[0].Path)
			}
		})
	}
}

func TestParserTest_ImplicitOutput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo | imp: cat bar\n")

			edge := p.state.Paths["imp"].InEdge
			if len(edge.Outputs) != 2 {
				t.Fatal("expected equal")
			}
			if !edge.isImplicitOut(1) {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_ImplicitOutputEmpty(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo | : cat bar\n")

			edge := p.state.Paths["foo"].InEdge
			if len(edge.Outputs) != 1 {
				t.Fatal("expected equal")
			}
			if edge.isImplicitOut(0) {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_ImplicitOutputDupe(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo baz | foo baq foo: cat bar\n")

			edge := p.state.Paths["foo"].InEdge
			if len(edge.Outputs) != 3 {
				t.Fatal("expected equal")
			}
			if edge.isImplicitOut(0) {
				t.Fatal("expected false")
			}
			if edge.isImplicitOut(1) {
				t.Fatal("expected false")
			}
			if !edge.isImplicitOut(2) {
				t.Fatal("expected true")
			}
		})
	}
}

func TestParserTest_ImplicitOutputDupes(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild foo foo foo | foo foo foo foo: cat bar\n")

			edge := p.state.Paths["foo"].InEdge
			if len(edge.Outputs) != 1 {
				t.Fatal("expected equal")
			}
			if edge.isImplicitOut(0) {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_NoExplicitOutput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild | imp : cat bar\n")
		})
	}
}

func TestParserTest_DefaultDefault(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild a: cat foo\nbuild b: cat foo\nbuild c: cat foo\nbuild d: cat foo\n")

			if 4 != len(p.state.DefaultNodes()) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DefaultDefaultCycle(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild a: cat a\n")

			if 0 != len(p.state.DefaultNodes()) {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DefaultStatements(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild a: cat foo\nbuild b: cat foo\nbuild c: cat foo\nbuild d: cat foo\nthird = c\ndefault a b\ndefault $third\n")

			nodes := p.state.DefaultNodes()
			if 3 != len(nodes) {
				t.Fatal("expected equal")
			}
			if nodes[0].Path != "a" || nodes[1].Path != "b" || nodes[2].Path != "c" {
				t.Fatal(nodes[0].Path, nodes[1].Path, nodes[2].Path)
			}
		})
	}
}

func TestParserTest_UTF8(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule utf8\n  command = true\n  description = compilaci\xC3\xB3\n")
		})
	}
}

func TestParserTest_CRLF(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("# comment with crlf\r\n")
			p.assertParse("foo = foo\nbar = bar\r\n")
			p.assertParse("pool link_pool\r\n  depth = 15\r\n\r\nrule xyz\r\n  command = something$expand \r\n  description = YAY!\r\n")
		})
	}
}

func TestParserTest_DyndepNotSpecified(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in\n")
			edge := p.state.GetNode("result", 0).InEdge
			if edge.Dyndep != nil {
				t.Fatal("expected false")
			}
		})
	}
}

func TestParserTest_DyndepExplicitInput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in\n  dyndep = in\n")
			edge := p.state.GetNode("result", 0).InEdge
			if edge.Dyndep == nil {
				t.Fatal("expected true")
			}
			if !edge.Dyndep.DyndepPending {
				t.Fatal("expected true")
			}
			if edge.Dyndep.Path != "in" {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DyndepImplicitInput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in | dd\n  dyndep = dd\n")
			edge := p.state.GetNode("result", 0).InEdge
			if edge.Dyndep == nil {
				t.Fatal("expected true")
			}
			if !edge.Dyndep.DyndepPending {
				t.Fatal("expected true")
			}
			if edge.Dyndep.Path != "dd" {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DyndepOrderOnlyInput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\nbuild result: cat in || dd\n  dyndep = dd\n")
			edge := p.state.GetNode("result", 0).InEdge
			if edge.Dyndep == nil {
				t.Fatal("expected true")
			}
			if !edge.Dyndep.DyndepPending {
				t.Fatal("expected true")
			}
			if edge.Dyndep.Path != "dd" {
				t.Fatal("expected equal")
			}
		})
	}
}

func TestParserTest_DyndepRuleInput(t *testing.T) {
	for _, c := range concurrencyVals {
		t.Run(c.String(), func(t *testing.T) {
			p := NewParserTest(t, c)
			p.assertParse("rule cat\n  command = cat $in > $out\n  dyndep = $in\nbuild result: cat in\n")
			edge := p.state.GetNode("result", 0).InEdge
			if edge.Dyndep == nil {
				t.Fatal("expected true")
			}
			if !edge.Dyndep.DyndepPending {
				t.Fatal("expected true")
			}
			if edge.Dyndep.Path != "in" {
				t.Fatal("expected equal")
			}
		})
	}
}

func writeFakeManifests(t testing.TB, dir string) {
	if _, err := os.Stat(filepath.Join(dir, "build.ninja")); err == nil {
		return
	}
	t.Logf("Creating manifest data...")
	cmd := exec.Command("python3", filepath.Join("misc", "write_fake_manifests.py"), dir)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkLoadManifest(b *testing.B) {
	manifestDir := filepath.Join("build", "manifest_perftest")
	writeFakeManifests(b, manifestDir)
	old, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	if err = os.Chdir(manifestDir); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err2 := os.Chdir(old); err2 != nil {
			b.Error(err2)
		}
	})
	di := RealDiskInterface{}
	// Don't benchmark the initial file read.
	contents, err := di.ReadFile("build.ninja")
	if err != nil {
		b.Fatal(err)
	}
	for _, c := range concurrencyVals {
		b.Run(c.String(), func(b *testing.B) {
			b.ReportAllocs()
			optimizationGuard := 0
			opts := ParseManifestOpts{
				Concurrency: c,
			}
			for i := 0; i < b.N; i++ {
				state := NewState()
				if err = ParseManifest(&state, &di, opts, "build.ninja", contents); err != nil {
					b.Fatal("Failed to read test data: ", err)
				}
				// Doing an empty build involves reading the manifest and evaluating all
				// commands required for the requested targets. So include command
				// evaluation in the perftest by default.
				for _, e := range state.Edges {
					optimizationGuard += len(e.EvaluateCommand(false))
				}
			}
		})
	}
}
