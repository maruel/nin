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
	state_               *State
	config_              *BuildConfig
	dyndep_loader_       DyndepLoader
	removed_             map[string]struct{}
	cleaned_             map[*Node]struct{}
	cleaned_files_count_ int
	disk_interface_      DiskInterface
	status_              int
}

// @return the number of file cleaned.
func (c *Cleaner) cleaned_files_count() int {
	return c.cleaned_files_count_
}

// @return whether the cleaner is in verbose mode.
func (c *Cleaner) IsVerbose() bool {
	return c.config_.verbosity != QUIET && (c.config_.verbosity == VERBOSE || c.config_.dry_run)
}

func NewCleaner(state *State, config *BuildConfig, disk_interface DiskInterface) *Cleaner {
	return &Cleaner{
		state_:          state,
		config_:         config,
		dyndep_loader_:  NewDyndepLoader(state, disk_interface),
		removed_:        map[string]struct{}{},
		cleaned_:        map[*Node]struct{}{},
		disk_interface_: disk_interface,
	}
}

// Remove the file @a path.
// @return whether the file has been removed.
func (c *Cleaner) RemoveFile(path string) int {
	return c.disk_interface_.RemoveFile(path)
}

// @returns whether the file @a path exists.
func (c *Cleaner) FileExists(path string) bool {
	err := ""
	mtime := c.disk_interface_.Stat(path, &err)
	if mtime == -1 {
		Error("%s", err)
	}
	return mtime > 0 // Treat Stat() errors as "file does not exist".
}

func (c *Cleaner) Report(path string) {
	c.cleaned_files_count_++
	if c.IsVerbose() {
		fmt.Printf("Remove %s\n", path)
	}
}

// Remove the given @a path file only if it has not been already removed.
func (c *Cleaner) Remove(path string) {
	if !c.IsAlreadyRemoved(path) {
		c.removed_[path] = struct{}{}
		if c.config_.dry_run {
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
	if c.config_.verbosity == QUIET {
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
	if c.config_.verbosity == QUIET {
		return
	}
	fmt.Printf("%d files.\n", c.cleaned_files_count_)
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
		if e.is_phony() {
			continue
		}
		// Do not remove generator's files unless generator specified.
		if !generator && e.GetBindingBool("generator") {
			continue
		}
		for _, out_node := range e.outputs_ {
			c.Remove(out_node.Path)
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
		if n == nil || (n.in_edge() == nil && len(n.out_edges()) == 0) {
			c.Remove(k)
		}
	}
	c.PrintFooter()
	return c.status_
}

// Helper recursive method for CleanTarget().
func (c *Cleaner) DoCleanTarget(target *Node) {
	if e := target.in_edge(); e != nil {
		// Do not try to remove phony targets
		if !e.is_phony() {
			c.Remove(target.Path)
			c.RemoveEdgeFiles(e)
		}
		for _, next := range e.inputs_ {
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
		Error("unknown target '%s'", target)
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
	for _, target_name := range targets {
		if target_name == "" {
			Error("failed to canonicalize '': empty path")
			c.status_ = 1
			continue
		}
		var slashBits uint64
		target_name = CanonicalizePath(target_name, &slashBits)
		target := c.state_.LookupNode(target_name)
		if target != nil {
			if c.IsVerbose() {
				fmt.Printf("Target %s\n", target_name)
			}
			c.DoCleanTarget(target)
		} else {
			Error("unknown target '%s'", target_name)
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
		if e.rule().name() == rule.name() {
			for _, out_node := range e.outputs_ {
				c.Remove(out_node.Path)
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
		Error("unknown rule '%s'", rule)
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
	for _, rule_name := range rules {
		rule := c.state_.bindings_.LookupRule(rule_name)
		if rule != nil {
			if c.IsVerbose() {
				fmt.Printf("Rule %s\n", rule_name)
			}
			c.DoCleanRule(rule)
		} else {
			Error("unknown rule '%s'", rule_name)
			c.status_ = 1
		}
	}
	c.PrintFooter()
	return c.status_
}

func (c *Cleaner) Reset() {
	c.status_ = 0
	c.cleaned_files_count_ = 0
	c.removed_ = map[string]struct{}{}
	c.cleaned_ = map[*Node]struct{}{}
}

// Load dependencies from dyndep bindings.
func (c *Cleaner) LoadDyndeps() {
	// Load dyndep files that exist, before they are cleaned.
	for _, e := range c.state_.edges_ {
		if e.dyndep_ != nil {
			// Capture and ignore errors loading the dyndep file.
			// We clean as much of the graph as we know.
			err := ""
			c.dyndep_loader_.LoadDyndeps(e.dyndep_, DyndepFile{}, &err)
		}
	}
}
