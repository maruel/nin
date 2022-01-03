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

import "fmt"

type Cleaner struct {
	state_             *State
	config_            *BuildConfig
	dyndepLoader_      DyndepLoader
	removed_           map[string]struct{}
	cleaned_           map[*Node]struct{}
	cleanedFilesCount_ int
	diskInterface_     DiskInterface
	status_            int
}

// @return the number of file cleaned.
func (c *Cleaner) cleanedFilesCount() int {
	return c.cleanedFilesCount_
}

// @return whether the cleaner is in verbose mode.
func (c *Cleaner) IsVerbose() bool {
	return c.config_.verbosity != Quiet && (c.config_.verbosity == Verbose || c.config_.dryRun)
}

func NewCleaner(state *State, config *BuildConfig, diskInterface DiskInterface) *Cleaner {
	return &Cleaner{
		state_:         state,
		config_:        config,
		dyndepLoader_:  NewDyndepLoader(state, diskInterface),
		removed_:       map[string]struct{}{},
		cleaned_:       map[*Node]struct{}{},
		diskInterface_: diskInterface,
	}
}

// Remove the file @a path.
// @return whether the file has been removed.
func (c *Cleaner) RemoveFile(path string) int {
	return c.diskInterface_.RemoveFile(path)
}

// @returns whether the file @a path exists.
func (c *Cleaner) FileExists(path string) bool {
	err := ""
	mtime := c.diskInterface_.Stat(path, &err)
	if mtime == -1 {
		errorf("%s", err)
	}
	return mtime > 0 // Treat Stat() errors as "file does not exist".
}

func (c *Cleaner) Report(path string) {
	c.cleanedFilesCount_++
	if c.IsVerbose() {
		fmt.Printf("Remove %s\n", path)
	}
}

// Remove the given @a path file only if it has not been already removed.
func (c *Cleaner) Remove(path string) {
	if !c.IsAlreadyRemoved(path) {
		c.removed_[path] = struct{}{}
		if c.config_.dryRun {
			if c.FileExists(path) {
				c.Report(path)
			}
		} else {
			ret := c.RemoveFile(path)
			if ret == 0 {
				c.Report(path)
			} else if ret == -1 {
				c.status_ = 1
			}
		}
	}
}

// @return whether the given @a path has already been removed.
func (c *Cleaner) IsAlreadyRemoved(path string) bool {
	_, ok := c.removed_[path]
	return ok
}

// Remove the depfile and rspfile for an Edge.
func (c *Cleaner) RemoveEdgeFiles(edge *Edge) {
	depfile := edge.GetUnescapedDepfile()
	if len(depfile) != 0 {
		c.Remove(depfile)
	}

	rspfile := edge.GetUnescapedRspfile()
	if len(rspfile) != 0 {
		c.Remove(rspfile)
	}
}

func (c *Cleaner) PrintHeader() {
	if c.config_.verbosity == Quiet {
		return
	}
	fmt.Printf("Cleaning...")
	if c.IsVerbose() {
		fmt.Printf("\n")
	} else {
		fmt.Printf(" ")
	}
	// TODO(maruel): fflush(stdout)
}

func (c *Cleaner) PrintFooter() {
	if c.config_.verbosity == Quiet {
		return
	}
	fmt.Printf("%d files.\n", c.cleanedFilesCount_)
}

// Clean all built files, except for files created by generator rules.
// @param generator If set, also clean files created by generator rules.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanAll(generator bool) int {
	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	for _, e := range c.state_.edges_ {
		// Do not try to remove phony targets
		if e.Rule == PhonyRule {
			continue
		}
		// Do not remove generator's files unless generator specified.
		if !generator && e.GetBinding("generator") != "" {
			continue
		}
		for _, outNode := range e.Outputs {
			c.Remove(outNode.Path)
		}

		c.RemoveEdgeFiles(e)
	}
	c.PrintFooter()
	return c.status_
}

// Clean the files produced by previous builds that are no longer in the
// manifest.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanDead(entries Entries) int {
	c.Reset()
	c.PrintHeader()
	for k := range entries {
		n := c.state_.LookupNode(k)
		// Detecting stale outputs works as follows:
		//
		// - If it has no Node, it is not in the build graph, or the deps log
		//   anymore, hence is stale.
		//
		// - If it isn't an output or input for any edge, it comes from a stale
		//   entry in the deps log, but no longer referenced from the build
		//   graph.
		//
		if n == nil || (n.InEdge == nil && len(n.OutEdges) == 0) {
			c.Remove(k)
		}
	}
	c.PrintFooter()
	return c.status_
}

// Helper recursive method for CleanTarget().
func (c *Cleaner) DoCleanTarget(target *Node) {
	if e := target.InEdge; e != nil {
		// Do not try to remove phony targets
		if e.Rule != PhonyRule {
			c.Remove(target.Path)
			c.RemoveEdgeFiles(e)
		}
		for _, next := range e.Inputs {
			// call DoCleanTarget recursively if this node has not been visited
			if _, ok := c.cleaned_[next]; !ok {
				c.DoCleanTarget(next)
			}
		}
	}

	// mark this target to be cleaned already
	c.cleaned_[target] = struct{}{}
}

// Clean the given target @a target.
// @return non-zero if an error occurs.
// Clean the given @a target and all the file built for it.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTargetNode(target *Node) int {
	if target == nil {
		panic("oops")
	}

	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	c.DoCleanTarget(target)
	c.PrintFooter()
	return c.status_
}

// Clean the given target @a target.
// @return non-zero if an error occurs.
// Clean the given @a target and all the file built for it.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTarget(target string) int {
	if target == "" {
		panic("oops")
	}

	c.Reset()
	node := c.state_.LookupNode(target)
	if node != nil {
		c.CleanTargetNode(node)
	} else {
		errorf("unknown target '%s'", target)
		c.status_ = 1
	}
	return c.status_
}

// Clean the given target @a targets.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTargets(targets []string) int {
	// TODO(maruel): Not unit tested.
	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	for _, targetName := range targets {
		if targetName == "" {
			errorf("failed to canonicalize '': empty path")
			c.status_ = 1
			continue
		}
		targetName = CanonicalizePath(targetName)
		target := c.state_.LookupNode(targetName)
		if target != nil {
			if c.IsVerbose() {
				fmt.Printf("Target %s\n", targetName)
			}
			c.DoCleanTarget(target)
		} else {
			errorf("unknown target '%s'", targetName)
			c.status_ = 1
		}
	}
	c.PrintFooter()
	return c.status_
}

func (c *Cleaner) DoCleanRule(rule *Rule) {
	if rule == nil {
		panic("oops")
	}

	for _, e := range c.state_.edges_ {
		if e.Rule.name() == rule.name() {
			for _, outNode := range e.Outputs {
				c.Remove(outNode.Path)
				c.RemoveEdgeFiles(e)
			}
		}
	}
}

// Clean the file produced by the given @a rule.
// @return non-zero if an error occurs.
// Clean all the file built with the given rule @a rule.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRule(rule *Rule) int {
	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	c.DoCleanRule(rule)
	c.PrintFooter()
	return c.status_
}

// Clean the file produced by the given @a rule.
// @return non-zero if an error occurs.
// Clean all the file built with the given rule @a rule.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRuleName(rule string) int {
	if rule == "" {
		panic("oops")
	}

	c.Reset()
	r := c.state_.bindings_.LookupRule(rule)
	if r != nil {
		c.CleanRule(r)
	} else {
		errorf("unknown rule '%s'", rule)
		c.status_ = 1
	}
	return c.status_
}

// Clean the file produced by the given @a rules.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRules(rules []string) int {
	// TODO(maruel): Not unit tested.
	if len(rules) == 0 {
		panic("oops")
	}

	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	for _, ruleName := range rules {
		rule := c.state_.bindings_.LookupRule(ruleName)
		if rule != nil {
			if c.IsVerbose() {
				fmt.Printf("Rule %s\n", ruleName)
			}
			c.DoCleanRule(rule)
		} else {
			errorf("unknown rule '%s'", ruleName)
			c.status_ = 1
		}
	}
	c.PrintFooter()
	return c.status_
}

func (c *Cleaner) Reset() {
	c.status_ = 0
	c.cleanedFilesCount_ = 0
	c.removed_ = map[string]struct{}{}
	c.cleaned_ = map[*Node]struct{}{}
}

// Load dependencies from dyndep bindings.
func (c *Cleaner) LoadDyndeps() {
	// Load dyndep files that exist, before they are cleaned.
	for _, e := range c.state_.edges_ {
		if e.Dyndep != nil {
			// Capture and ignore errors loading the dyndep file.
			// We clean as much of the graph as we know.
			err := ""
			c.dyndepLoader_.LoadDyndeps(e.Dyndep, DyndepFile{}, &err)
		}
	}
}
