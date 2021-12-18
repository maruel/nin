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

//go:build nobuild

package ginja


type Cleaner struct {

  // @return the number of file cleaned.

  // @return whether the cleaner is in verbose mode.

  state_ *State
  config_ *BuildConfig
  dyndep_loader_ DyndepLoader
  removed_ map[string]struct{}
  cleaned_ map[*Node]struct{}
  cleaned_files_count_ int
  disk_interface_ *DiskInterface
  status_ int
}
// @return the number of file cleaned.
func (c *Cleaner) cleaned_files_count() int {
  return cleaned_files_count_
}
// @return whether the cleaner is in verbose mode.
func (c *Cleaner) IsVerbose() bool {
  return (config_.verbosity != BuildConfig::QUIET && (config_.verbosity == BuildConfig::VERBOSE || config_.dry_run))
}


Cleaner::Cleaner(State* state, const BuildConfig& config, DiskInterface* disk_interface)
  : state_(state),
    config_(config),
    dyndep_loader_(state, disk_interface),
    cleaned_files_count_(0),
    disk_interface_(disk_interface),
    status_(0) {
}

// Remove the file @a path.
// @return whether the file has been removed.
func (c *Cleaner) RemoveFile(path string) int {
  return disk_interface_.RemoveFile(path)
}

// @returns whether the file @a path exists.
func (c *Cleaner) FileExists(path string) bool {
  err := ""
  mtime := disk_interface_.Stat(path, &err)
  if mtime == -1 {
    Error("%s", err)
  }
  return mtime > 0  // Treat Stat() errors as "file does not exist".
}

func (c *Cleaner) Report(path string) {
  cleaned_files_count_++
  if IsVerbose() {
    printf("Remove %s\n", path)
  }
}

// Remove the given @a path file only if it has not been already removed.
func (c *Cleaner) Remove(path string) {
  if !IsAlreadyRemoved(path) {
    removed_.insert(path)
    if config_.dry_run {
      if FileExists(path) {
        Report(path)
      }
    } else {
      ret := RemoveFile(path)
      if ret == 0 {
        Report(path)
      } else if ret == -1 {
        status_ = 1
      }
    }
  }
}

// @return whether the given @a path has already been removed.
func (c *Cleaner) IsAlreadyRemoved(path string) bool {
  i := removed_.find(path)
  return (i != removed_.end())
}

// Remove the depfile and rspfile for an Edge.
func (c *Cleaner) RemoveEdgeFiles(edge *Edge) {
  depfile := edge.GetUnescapedDepfile()
  if len(depfile) != 0 {
    Remove(depfile)
  }

  rspfile := edge.GetUnescapedRspfile()
  if len(rspfile) != 0 {
    Remove(rspfile)
  }
}

func (c *Cleaner) PrintHeader() {
  if config_.verbosity == BuildConfig::QUIET {
    return
  }
  printf("Cleaning...")
  if IsVerbose() {
    printf("\n")
  } else {
    printf(" ")
  }
  fflush(stdout)
}

func (c *Cleaner) PrintFooter() {
  if config_.verbosity == BuildConfig::QUIET {
    return
  }
  printf("%d files.\n", cleaned_files_count_)
}

// Clean all built files, except for files created by generator rules.
// @param generator If set, also clean files created by generator rules.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanAll(generator bool) int {
  Reset()
  PrintHeader()
  LoadDyndeps()
  for e := state_.edges_.begin(); e != state_.edges_.end(); e++ {
    // Do not try to remove phony targets
    if (*e).is_phony() {
      continue
    }
    // Do not remove generator's files unless generator specified.
    if !generator && (*e).GetBindingBool("generator") {
      continue
    }
    for out_node := (*e).outputs_.begin(); out_node != (*e).outputs_.end(); out_node++ {
      Remove((*out_node).path())
    }

    RemoveEdgeFiles(*e)
  }
  PrintFooter()
  return status_
}

// Clean the files produced by previous builds that are no longer in the
// manifest.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanDead(entries *BuildLog::Entries) int {
  Reset()
  PrintHeader()
  for i := entries.begin(); i != entries.end(); i++ {
    n := state_.LookupNode(i.first)
    // Detecting stale outputs works as follows:
    //
    // - If it has no Node, it is not in the build graph, or the deps log
    //   anymore, hence is stale.
    //
    // - If it isn't an output or input for any edge, it comes from a stale
    //   entry in the deps log, but no longer referenced from the build
    //   graph.
    //
    if !n || (!n.in_edge() && n.out_edges().empty()) {
      Remove(i.first.AsString())
    }
  }
  PrintFooter()
  return status_
}

// Helper recursive method for CleanTarget().
func (c *Cleaner) DoCleanTarget(target *Node) {
  if Edge* e = target.in_edge() {
    // Do not try to remove phony targets
    if !e.is_phony() {
      Remove(target.path())
      RemoveEdgeFiles(e)
    }
    for n := e.inputs_.begin(); n != e.inputs_.end(); n++ {
      next := *n
      // call DoCleanTarget recursively if this node has not been visited
      if cleaned_.count(next) == 0 {
        DoCleanTarget(next)
      }
    }
  }

  // mark this target to be cleaned already
  cleaned_.insert(target)
}

// Clean the given target @a target.
// @return non-zero if an error occurs.
// Clean the given @a target and all the file built for it.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTarget(target *Node) int {
  if !target { panic("oops") }

  Reset()
  PrintHeader()
  LoadDyndeps()
  DoCleanTarget(target)
  PrintFooter()
  return status_
}

// Clean the given target @a target.
// @return non-zero if an error occurs.
// Clean the given @a target and all the file built for it.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTarget(target string) int {
  if !target { panic("oops") }

  Reset()
  node := state_.LookupNode(target)
  if node != nil {
    CleanTarget(node)
  } else {
    Error("unknown target '%s'", target)
    status_ = 1
  }
  return status_
}

// Clean the given target @a targets.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanTargets(target_count int, targets []*char) int {
  Reset()
  PrintHeader()
  LoadDyndeps()
  for i := 0; i < target_count; i++ {
    target_name := targets[i]
    if target_name.empty() {
      Error("failed to canonicalize '': empty path")
      status_ = 1
      continue
    }
    var slash_bits uint64
    CanonicalizePath(&target_name, &slash_bits)
    target := state_.LookupNode(target_name)
    if target != nil {
      if IsVerbose() {
        printf("Target %s\n", target_name)
      }
      DoCleanTarget(target)
    } else {
      Error("unknown target '%s'", target_name)
      status_ = 1
    }
  }
  PrintFooter()
  return status_
}

func (c *Cleaner) DoCleanRule(rule *Rule) {
  if !rule { panic("oops") }

  for e := state_.edges_.begin(); e != state_.edges_.end(); e++ {
    if (*e).rule().name() == rule.name() {
      for out_node := (*e).outputs_.begin(); out_node != (*e).outputs_.end(); out_node++ {
        Remove((*out_node).path())
        RemoveEdgeFiles(*e)
      }
    }
  }
}

// Clean the file produced by the given @a rule.
// @return non-zero if an error occurs.
// Clean all the file built with the given rule @a rule.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRule(rule *Rule) int {
  if !rule { panic("oops") }

  Reset()
  PrintHeader()
  LoadDyndeps()
  DoCleanRule(rule)
  PrintFooter()
  return status_
}

// Clean the file produced by the given @a rule.
// @return non-zero if an error occurs.
// Clean all the file built with the given rule @a rule.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRule(rule string) int {
  if !rule { panic("oops") }

  Reset()
  r := state_.bindings_.LookupRule(rule)
  if r != nil {
    CleanRule(r)
  } else {
    Error("unknown rule '%s'", rule)
    status_ = 1
  }
  return status_
}

// Clean the file produced by the given @a rules.
// @return non-zero if an error occurs.
func (c *Cleaner) CleanRules(rule_count int, rules []*char) int {
  if !rules { panic("oops") }

  Reset()
  PrintHeader()
  LoadDyndeps()
  for i := 0; i < rule_count; i++ {
    rule_name := rules[i]
    rule := state_.bindings_.LookupRule(rule_name)
    if rule != nil {
      if IsVerbose() {
        printf("Rule %s\n", rule_name)
      }
      DoCleanRule(rule)
    } else {
      Error("unknown rule '%s'", rule_name)
      status_ = 1
    }
  }
  PrintFooter()
  return status_
}

func (c *Cleaner) Reset() {
  status_ = 0
  cleaned_files_count_ = 0
  removed_ = nil
  cleaned_ = nil
}

// Load dependencies from dyndep bindings.
func (c *Cleaner) LoadDyndeps() {
  // Load dyndep files that exist, before they are cleaned.
  for e := state_.edges_.begin(); e != state_.edges_.end(); e++ {
    if Node* dyndep = (*e).dyndep_ {
      // Capture and ignore errors loading the dyndep file.
      // We clean as much of the graph as we know.
      err := ""
      dyndep_loader_.LoadDyndeps(dyndep, &err)
    }
  }
}

