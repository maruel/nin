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
	"fmt"
	"os"
)

type Cleaner struct {
	state             *State
	config            *BuildConfig
	dyndepLoader      DyndepLoader
	removed           map[string]struct{}
	cleaned           map[*Node]struct{}
	cleanedFilesCount int // Number of files cleaned.
	di                DiskInterface
	status            int
}

// @return whether the cleaner is in verbose mode.
func (c *Cleaner) IsVerbose() bool {
	return c.config.Verbosity != Quiet && (c.config.Verbosity == Verbose || c.config.DryRun)
}

func NewCleaner(state *State, config *BuildConfig, di DiskInterface) *Cleaner {
	return &Cleaner{
		state:        state,
		config:       config,
		dyndepLoader: NewDyndepLoader(state, di),
		removed:      map[string]struct{}{},
		cleaned:      map[*Node]struct{}{},
		di:           di,
	}
}

// @returns whether the file @a path exists.
func (c *Cleaner) FileExists(path string) bool {
	mtime, err := c.di.Stat(path)
	if mtime == -1 {
		errorf("%s", err)
	}
	return mtime > 0 // Treat Stat() errors as "file does not exist".
}

func (c *Cleaner) Report(path string) {
	c.cleanedFilesCount++
	if c.IsVerbose() {
		fmt.Printf("Remove %s\n", path)
	}
}

// Remove the given @a path file only if it has not been already removed.
func (c *Cleaner) Remove(path string) {
	if !c.IsAlreadyRemoved(path) {
		c.removed[path] = struct{}{}
		if c.config.DryRun {
			if c.FileExists(path) {
				c.Report(path)
			}
		} else {
			if err := c.di.RemoveFile(path); err == nil {
				c.Report(path)
			} else if !os.IsNotExist(err) {
				c.status = 1
			}
		}
	}
}

// @return whether the given @a path has already been removed.
func (c *Cleaner) IsAlreadyRemoved(path string) bool {
	_, ok := c.removed[path]
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
	if c.config.Verbosity == Quiet {
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
	if c.config.Verbosity == Quiet {
		return
	}
	fmt.Printf("%d files.\n", c.cleanedFilesCount)
}

// Clean all built files, except for files created by generator rules.
// @param generator If set, also clean files created by generator rules.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanAll(generator bool) int {
	c.Reset()
	c.PrintHeader()
	c.LoadDyndeps()
	for _, e := range c.state.Edges {
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
	return c.status
}

// Clean the files produced by previous builds that are no longer in the
// manifest.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanDead(entries map[string]*LogEntry) int {
	c.Reset()
	c.PrintHeader()
	for k := range entries {
		n := c.state.Paths[k]
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
	return c.status
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
			if _, ok := c.cleaned[next]; !ok {
				c.DoCleanTarget(next)
			}
		}
	}

	// mark this target to be cleaned already
	c.cleaned[target] = struct{}{}
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
	return c.status
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
	node := c.state.Paths[target]
	if node != nil {
		c.CleanTargetNode(node)
	} else {
		errorf("unknown target '%s'", target)
		c.status = 1
	}
	return c.status
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
			c.status = 1
			continue
		}
		targetName = CanonicalizePath(targetName)
		target := c.state.Paths[targetName]
		if target != nil {
			if c.IsVerbose() {
				fmt.Printf("Target %s\n", targetName)
			}
			c.DoCleanTarget(target)
		} else {
			errorf("unknown target '%s'", targetName)
			c.status = 1
		}
	}
	c.PrintFooter()
	return c.status
}

func (c *Cleaner) DoCleanRule(rule *Rule) {
	if rule == nil {
		panic("oops")
	}

	for _, e := range c.state.Edges {
		if e.Rule.Name == rule.Name {
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
	return c.status
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
	r := c.state.Bindings.LookupRule(rule)
	if r != nil {
		c.CleanRule(r)
	} else {
		errorf("unknown rule '%s'", rule)
		c.status = 1
	}
	return c.status
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
		rule := c.state.Bindings.LookupRule(ruleName)
		if rule != nil {
			if c.IsVerbose() {
				fmt.Printf("Rule %s\n", ruleName)
			}
			c.DoCleanRule(rule)
		} else {
			errorf("unknown rule '%s'", ruleName)
			c.status = 1
		}
	}
	c.PrintFooter()
	return c.status
}

func (c *Cleaner) Reset() {
	c.status = 0
	c.cleanedFilesCount = 0
	c.removed = map[string]struct{}{}
	c.cleaned = map[*Node]struct{}{}
}

// Load dependencies from dyndep bindings.
func (c *Cleaner) LoadDyndeps() {
	// Load dyndep files that exist, before they are cleaned.
	for _, e := range c.state.Edges {
		if e.Dyndep != nil {
			// Capture and ignore errors loading the dyndep file.
			// We clean as much of the graph as we know.
			err := ""
			c.dyndepLoader.LoadDyndeps(e.Dyndep, DyndepFile{}, &err)
		}
	}
}
