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
	"path/filepath"
	"testing"
)

type CleanTest struct {
	StateTestWithBuiltinRules
	fs_     VirtualFileSystem
	config_ BuildConfig
}

func NewCleanTest(t *testing.T) *CleanTest {
	return &CleanTest{
		StateTestWithBuiltinRules: NewStateTestWithBuiltinRules(t),
		fs_:                       NewVirtualFileSystem(),
		config_:                   BuildConfig{verbosity: QUIET},
	}
}

func TestCleanTest_CleanAll(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 4 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	err := ""
	if 0 != c.fs_.Stat("in1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("in2", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out2", &err) {
		t.Fatal("expected equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanAllDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	c.config_.dry_run = true
	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	err := ""
	if 0 >= c.fs_.Stat("in1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("in2", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out2", &err) {
		t.Fatal("expected less or equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanTarget(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	err := ""
	if 0 != c.fs_.Stat("in1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out1", &err) {
		t.Fatal("expected equal")
	}
	if 0 >= c.fs_.Stat("in2", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out2", &err) {
		t.Fatal("expected less or equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanTargetDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "build in1: cat src1\nbuild out1: cat in1\nbuild in2: cat src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	c.config_.dry_run = true
	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	err := ""
	if 0 >= c.fs_.Stat("in1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("in2", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out2", &err) {
		t.Fatal("expected less or equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRule(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cat_e\n  command = cat -e $in > $out\nbuild in1: cat_e src1\nbuild out1: cat in1\nbuild in2: cat_e src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	err := ""
	if 0 != c.fs_.Stat("in1", &err) {
		t.Fatal("expected equal")
	}
	if 0 >= c.fs_.Stat("out1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 != c.fs_.Stat("in2", &err) {
		t.Fatal("expected equal")
	}
	if 0 >= c.fs_.Stat("out2", &err) {
		t.Fatal("expected less or equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRuleDryRun(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cat_e\n  command = cat -e $in > $out\nbuild in1: cat_e src1\nbuild out1: cat in1\nbuild in2: cat_e src2\nbuild out2: cat in2\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	c.config_.dry_run = true
	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are not removed.
	err := ""
	if 0 >= c.fs_.Stat("in1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out1", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("in2", &err) {
		t.Fatal("expected less or equal")
	}
	if 0 >= c.fs_.Stat("out2", &err) {
		t.Fatal("expected less or equal")
	}
	c.fs_.files_removed_ = nil

	if 0 != cleaner.CleanRuleName("cat_e") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRuleGenerator(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule regen\n  command = cat $in > $out\n  generator = 1\nbuild out1: cat in1\nbuild out2: regen in2\n", ManifestParserOptions{})
	c.fs_.Create("out1", "")
	c.fs_.Create("out2", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	c.fs_.Create("out1", "")

	if 0 != cleaner.CleanAll( /*generator=*/ true) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFile(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs_.Create("out1", "")
	c.fs_.Create("out1.d", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFileOnCleanTarget(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs_.Create("out1", "")
	c.fs_.Create("out1.d", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDepFileOnCleanRule(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cc\n  command = cc $in > $out\n  depfile = $out.d\nbuild out1: cc in1\n", ManifestParserOptions{})
	c.fs_.Create("out1", "")
	c.fs_.Create("out1.d", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanRuleName("cc") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDyndep(t *testing.T) {
	c := NewCleanTest(t)
	// Verify that a dyndep file can be loaded to discover a new output
	// to be cleaned.
	c.AssertParse(&c.state_, "build out: cat in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	c.fs_.Create("in", "")
	c.fs_.Create("dd", "ninja_dyndep_version = 1\nbuild out | out.imp: dyndep\n")
	c.fs_.Create("out", "")
	c.fs_.Create("out.imp", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	err := ""
	if 0 != c.fs_.Stat("out", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out.imp", &err) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanDyndepMissing(t *testing.T) {
	c := NewCleanTest(t)
	// Verify that a missing dyndep file is tolerated.
	c.AssertParse(&c.state_, "build out: cat in || dd\n  dyndep = dd\n", ManifestParserOptions{})
	c.fs_.Create("in", "")
	c.fs_.Create("out", "")
	c.fs_.Create("out.imp", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)

	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	err := ""
	if 0 != c.fs_.Stat("out", &err) {
		t.Fatal("expected equal")
	}
	if 1 != c.fs_.Stat("out.imp", &err) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRspFile(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cc\n  command = cc $in > $out\n  rspfile = $rspfile\n  rspfile_content=$in\nbuild out1: cc in1\n  rspfile = cc1.rsp\n", ManifestParserOptions{})
	c.fs_.Create("out1", "")
	c.fs_.Create("cc1.rsp", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 2 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanRsp(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cat_rsp \n  command = cat $rspfile > $out\n  rspfile = $rspfile\n  rspfile_content = $in\nbuild in1: cat src1\nbuild out1: cat in1\nbuild in2: cat_rsp src2\n  rspfile=in2.rsp\nbuild out2: cat_rsp in2\n  rspfile=out2.rsp\n", ManifestParserOptions{})
	c.fs_.Create("in1", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("in2.rsp", "")
	c.fs_.Create("out2.rsp", "")
	c.fs_.Create("in2", "")
	c.fs_.Create("out2", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("out1") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanTarget("in2") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != cleaner.CleanRuleName("cat_rsp") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}

	if 6 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	// Check they are removed.
	err := ""
	if 0 != c.fs_.Stat("in1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("in2", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out2", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("in2.rsp", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out2.rsp", &err) {
		t.Fatal("expected equal")
	}
}

func TestCleanTest_CleanFailure(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "build dir: cat src1\n", ManifestParserOptions{})
	c.fs_.MakeDir("dir")
	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 == cleaner.CleanAll(false) {
		t.Fatal("expected different")
	}
}

func TestCleanTest_CleanPhony(t *testing.T) {
	c := NewCleanTest(t)
	err := ""
	c.AssertParse(&c.state_, "build phony: phony t1 t2\nbuild t1: cat\nbuild t2: cat\n", ManifestParserOptions{})

	c.fs_.Create("phony", "")
	c.fs_.Create("t1", "")
	c.fs_.Create("t2", "")

	// Check that CleanAll does not remove "phony".
	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 >= c.fs_.Stat("phony", &err) {
		t.Fatal("expected less or equal")
	}

	c.fs_.Create("t1", "")
	c.fs_.Create("t2", "")

	// Check that CleanTarget does not remove "phony".
	if 0 != cleaner.CleanTarget("phony") {
		t.Fatal("expected equal")
	}
	if 2 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 >= c.fs_.Stat("phony", &err) {
		t.Fatal("expected less or equal")
	}
}

func TestCleanTest_CleanDepFileAndRspFileWithSpaces(t *testing.T) {
	c := NewCleanTest(t)
	c.AssertParse(&c.state_, "rule cc_dep\n  command = cc $in > $out\n  depfile = $out.d\nrule cc_rsp\n  command = cc $in > $out\n  rspfile = $out.rsp\n  rspfile_content = $in\nbuild out$ 1: cc_dep in$ 1\nbuild out$ 2: cc_rsp in$ 1\n", ManifestParserOptions{})
	c.fs_.Create("out 1", "")
	c.fs_.Create("out 2", "")
	c.fs_.Create("out 1.d", "")
	c.fs_.Create("out 2.rsp", "")

	cleaner := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner.CleanAll(false) {
		t.Fatal("expected equal")
	}
	if 4 != cleaner.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 4 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}

	err := ""
	if 0 != c.fs_.Stat("out 1", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out 2", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out 1.d", &err) {
		t.Fatal("expected equal")
	}
	if 0 != c.fs_.Stat("out 2.rsp", &err) {
		t.Fatal("expected equal")
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
	c.AssertParse(&c.state_, "build out2: cat in\n", ManifestParserOptions{})
	c.fs_.Create("in", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("out2", "")

	var log1 BuildLog
	err := ""
	if !log1.OpenForWrite(kTestFilename, c, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log1.RecordCommand(state.edges_[0], 15, 18, 0)
	log1.RecordCommand(state.edges_[1], 20, 25, 0)
	log1.Close()

	var log2 BuildLog
	if log2.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.entries()) {
		t.Fatal("expected equal")
	}
	if log2.LookupByOutput("out1") == nil {
		t.Fatal("expected true")
	}
	if log2.LookupByOutput("out2") == nil {
		t.Fatal("expected true")
	}

	// First use the manifest that describe how to build out1.
	cleaner1 := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner1.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner1.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out1", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}

	// Then use the manifest that does not build out1 anymore.
	cleaner2 := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner2.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 1 != cleaner2.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	t.Skip("TODO")
	/*
		if "out1" != *(c.fs_.files_removed_.begin()) {
			t.Fatal("expected equal")
		}
	*/
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 != c.fs_.Stat("out1", &err) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}

	// Nothing to do now.
	if 0 != cleaner2.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 1 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	t.Skip("TODO")
	/*
		if "out1" != *(c.fs_.files_removed_.begin()) {
			t.Fatal("expected equal")
		}
	*/
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 != c.fs_.Stat("out1", &err) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}
	log2.Close()
}

func TestCleanDeadTest_CleanDeadPreservesInputs(t *testing.T) {
	t.Skip("TODO")
	kTestFilename := filepath.Join(t.TempDir(), "CleanTest-tempfile")
	c := NewCleanDeadTest(t)
	state := NewState()
	c.AssertParse(&state, "rule cat\n  command = cat $in > $out\nbuild out1: cat in\nbuild out2: cat in\n", ManifestParserOptions{})
	// This manifest does not build out1 anymore, but makes
	// it an implicit input. CleanDead should detect this
	// and preserve it.
	c.AssertParse(&c.state_, "build out2: cat in | out1\n", ManifestParserOptions{})
	c.fs_.Create("in", "")
	c.fs_.Create("out1", "")
	c.fs_.Create("out2", "")

	var log1 BuildLog
	err := ""
	if !log1.OpenForWrite(kTestFilename, c, &err) {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	log1.RecordCommand(state.edges_[0], 15, 18, 0)
	log1.RecordCommand(state.edges_[1], 20, 25, 0)
	log1.Close()

	var log2 BuildLog
	if log2.Load(kTestFilename, &err) != LOAD_SUCCESS {
		t.Fatal("expected true")
	}
	if "" != err {
		t.Fatal("expected equal")
	}
	if 2 != len(log2.entries()) {
		t.Fatal("expected equal")
	}
	if log2.LookupByOutput("out1") == nil {
		t.Fatal("expected true")
	}
	if log2.LookupByOutput("out2") == nil {
		t.Fatal("expected true")
	}

	// First use the manifest that describe how to build out1.
	cleaner1 := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner1.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner1.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out1", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}

	// Then use the manifest that does not build out1 anymore.
	cleaner2 := NewCleaner(&c.state_, &c.config_, &c.fs_)
	if 0 != cleaner2.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out1", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}

	// Nothing to do now.
	if 0 != cleaner2.CleanDead(log2.entries()) {
		t.Fatal("expected equal")
	}
	if 0 != cleaner2.cleaned_files_count() {
		t.Fatal("expected equal")
	}
	if 0 != len(c.fs_.files_removed_) {
		t.Fatal("expected equal")
	}
	if 0 == c.fs_.Stat("in", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out1", &err) {
		t.Fatal("expected different")
	}
	if 0 == c.fs_.Stat("out2", &err) {
		t.Fatal("expected different")
	}
	log2.Close()
}
