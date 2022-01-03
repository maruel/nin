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
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type CleanTest struct {
	StateTestWithBuiltinRules
	fs     VirtualFileSystem
	config BuildConfig
}

func NewCleanTest(t *testing.T) *CleanTest {
	c := &CleanTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		fs:                        NewVirtualFileSystem(),
		config:                    NewBuildConfig(),
	}
	c.config.verbosity = Quiet
	return c
}

func TestCleanTest_CleanAll(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 4 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	if mtime, err := c.fs.Stat("in1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanAllDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	c.config.dryRun = true
	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	if mtime, err := c.fs.Stat("in1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanTarget(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	if mtime, err := c.fs.Stat("in1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanTargetDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	c.config.dryRun = true
	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	if mtime, err := c.fs.Stat("in1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRule(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cat_e\n  command = cat -e $in > $out\nbuild in1: cat_e src1\nbuild out1: cat in1\nbuild in2: cat_e src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	if mtime, err := c.fs.Stat("in1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRuleDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cat_e\n  command = cat -e $in > $out\nbuild in1: cat_e src1\nbuild out1: cat in1\nbuild in2: cat_e src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	c.config.dryRun = true
	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	if mtime, err := c.fs.Stat("in1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	c.fs.filesRemoved = nil

	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRuleGenerator(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule regen\n  command = cat $in > $out\n  generator = 1\nbuild out1: cat in1\nbuild out2: regen in2\n", ManifestParserOptions{})
	c.fs.Create("out1", "")
	c.fs.Create("out2", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	c.fs.Create("out1", "")

	// generator=true
	if 0 != cleaner.CleanAll(true) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFile(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs.Create("out1", "")
	c.fs.Create("out1.d", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFileOnCleanTarget(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs.Create("out1", "")
	c.fs.Create("out1.d", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFileOnCleanRule(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs.Create("out1", "")
	c.fs.Create("out1.d", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanRuleName("cc") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDyndep(t *testing.T) {
	c := NewCleanTest(t)
	// Verify that a dyndep file can be loaded to discover a new output
	// to be cleaned.
	c.AssertParse(&c.state, "build out: cat in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	c.fs.Create("in", "")
	c.fs.Create("dd", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep\n")
	c.fs.Create("out", "")
	c.fs.Create("out.imp", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	if mtime, err := c.fs.Stat("out"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out.imp"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestCleanTest_CleanDyndepMissing(t *testing.T) {
	c := NewCleanTest(t)
	// Verify that a missing dyndep file is tolerated.
	c.AssertParse(&c.state, "build out: cat in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	c.fs.Create("in", "")
	c.fs.Create("out", "")
	c.fs.Create("out.imp", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)

	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	if mtime, err := c.fs.Stat("out"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out.imp"); mtime != 1 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestCleanTest_CleanRspFile(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cc\n  command = cc $in > $out\n  rspfile = $rspfile\n  rspfile_content=$in\nbuild out1: cc in1\n  rspfile = cc1.rsp\n", ManifestParserOptions{})
	c.fs.Create("out1", "")
	c.fs.Create("cc1.rsp", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRsp(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cat_rsp \n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $in\nbuild in1: cat src1\nbuild out1: cat in1\nbuild in2: cat_rsp src2\n  rspfile=in2.rsp\nbuild out2: cat_rsp in2\n  rspfile=out2.rsp\n", ManifestParserOptions{})
	c.fs.Create("in1", "")
	c.fs.Create("out1", "")
	c.fs.Create("in2.rsp", "")
	c.fs.Create("out2.rsp", "")
	c.fs.Create("in2", "")
	c.fs.Create("out2", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("in2") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_rsp") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}

	if 6 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	if mtime, err := c.fs.Stat("in1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("in2.rsp"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2.rsp"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestCleanTest_CleanFailure(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build dir: cat src1\n", ManifestParserOptions{})
	c.fs.MakeDir("dir")
	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 == cleaner.CleanAll(false) {
		t.Fatal("expected different")
	}
}

func TestCleanTest_CleanPhony(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "build phony: phony t1 t2\nbuild t1: cat\nbuild t2: cat\n", ManifestParserOptions{})

	c.fs.Create("phony", "")
	c.fs.Create("t1", "")
	c.fs.Create("t2", "")

	// Check that CleanAll does not remove "phony".
	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("phony"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	c.fs.Create("t1", "")
	c.fs.Create("t2", "")

	// Check that CleanTarget does not remove "phony".
	if 0 != cleaner.CleanTarget("phony") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("phony"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

func TestCleanTest_CleanDepFileAndRspFileWithSpaces(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state, "rule cc_dep\n  command = cc $in > $out\n  depfile = $out.d\nrule cc_rsp\n  command = cc $in > $out\n  rspfile = $out.rsp\n  rspfile_content = $in\nbuild out$ 1: cc_dep in$ 1\nbuild out$ 2: cc_rsp in$ 1\n", ManifestParserOptions{})
	c.fs.Create("out 1", "")
	c.fs.Create("out 2", "")
	c.fs.Create("out 1.d", "")
	c.fs.Create("out 2.rsp", "")

	cleaner := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 4 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}

	if mtime, err := c.fs.Stat("out 1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out 2"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out 1.d"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out 2.rsp"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
}

type CleanDeadTest struct {
	*CleanTest
}

func NewCleanDeadTest(t *testing.T) *CleanDeadTest {
	// In case a crashing test left a stale file behind.
	return &CleanDeadTest{NewCleanTest(t)}
}

func (c *CleanDeadTest) IsPathDead(string) bool {
	return false
}

func TestCleanDeadTest_CleanDead(t *testing.T) {
	t.Skip("TODO")
	kTestFilename := filepath.Join(t.TempDir(), "CleanTest-tempfile")
	c := NewCleanDeadTest(t)
	state := NewState()
	c.AssertParse(&state, "rule cat\n  command = cat $in > $out\nbuild out1: cat in\nbuild out2: cat in\n", ManifestParserOptions{})
	c.AssertParse(&c.state, "build out2: cat in\n", ManifestParserOptions{})
	c.fs.Create("in", "")
	c.fs.Create("out1", "")
	c.fs.Create("out2", "")

	log1 := NewBuildLog()
	err := ""
	if !log1.OpenForWrite(kTestFilename, c, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log1.RecordCommand(state.edges[0], 15, 18, 0)
	log1.RecordCommand(state.edges[1], 20, 25, 0)
	log1.Close()

	log2 := NewBuildLog()
	if log2.Load(kTestFilename, &err) != LoadSuccess {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.entries) {
		t.Fatal("expected equal")
	}
	if log2.LookupByOutput("out1") == nil {
		t.Fatal("expected true")
	}
	if log2.LookupByOutput("out2") == nil {
		t.Fatal("expected true")
	}

	// First use the manifest that describe how to build out1.
	cleaner1 := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner1.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner1.cleanedFilesCount {
		t.Fatal(cleaner1.cleanedFilesCount)
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// Then use the manifest that does not build out1 anymore.
	cleaner2 := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner2.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner2.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if diff := cmp.Diff(map[string]struct{}{"out1": {}}, c.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// Nothing to do now.
	if 0 != cleaner2.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
	if diff := cmp.Diff(map[string]struct{}{"out1": {}}, c.fs.filesRemoved); diff != "" {
		t.Fatal(diff)
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime != 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	log2.Close()
}

func TestCleanDeadTest_CleanDeadPreservesInputs(t *testing.T) {
	kTestFilename := filepath.Join(t.TempDir(), "CleanTest-tempfile")
	c := NewCleanDeadTest(t)
	state := NewState()
	c.AssertParse(&state, "rule cat\n  command = cat $in > $out\nbuild out1: cat in\nbuild out2: cat in\n", ManifestParserOptions{})
	// This manifest does not build out1 anymore, but makes
	// it an implicit input. CleanDead should detect this
	// and preserve it.
	c.AssertParse(&c.state, "build out2: cat in | out1\n", ManifestParserOptions{})
	c.fs.Create("in", "")
	c.fs.Create("out1", "")
	c.fs.Create("out2", "")

	log1 := NewBuildLog()
	err := ""
	if !log1.OpenForWrite(kTestFilename, c, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log1.RecordCommand(state.edges[0], 15, 18, 0)
	log1.RecordCommand(state.edges[1], 20, 25, 0)
	log1.Close()

	log2 := NewBuildLog()
	if log2.Load(kTestFilename, &err) != LoadSuccess {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.entries) {
		t.Fatal("expected equal")
	}
	if log2.LookupByOutput("out1") == nil {
		t.Fatal("expected true")
	}
	if log2.LookupByOutput("out2") == nil {
		t.Fatal("expected true")
	}

	// First use the manifest that describe how to build out1.
	cleaner1 := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner1.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner1.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// Then use the manifest that does not build out1 anymore.
	cleaner2 := NewCleaner(&c.state, &c.config, &c.fs)
	if 0 != cleaner2.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}

	// Nothing to do now.
	if 0 != cleaner2.CleanDead(log2.entries) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleanedFilesCount {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs.filesRemoved) {
		t.Fatal("expected equal")
	}
	if mtime, err := c.fs.Stat("in"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out1"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	if mtime, err := c.fs.Stat("out2"); mtime <= 0 || err != nil {
		t.Fatal(mtime, err)
	}
	log2.Close()
}
