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

import "bytes"

// Information about a node in the dependency graph: the file, whether
// it's dirty, mtime, etc.
type Node struct {
	path_ string

	// Set bits starting from lowest for backslashes that were normalized to
	// forward slashes by CanonicalizePath. See |PathDecanonicalized|.
	slash_bits_ uint64

	// Possible values of mtime_:
	//   -1: file hasn't been examined
	//   0:  we looked, and file doesn't exist
	//   >0: actual file's mtime, or the latest mtime of its dependencies if it doesn't exist
	mtime_ TimeStamp

	exists_ ExistenceStatus

	// Dirty is true when the underlying file is out-of-date.
	// But note that Edge::outputs_ready_ is also used in judging which
	// edges to build.
	dirty_ bool

	// Store whether dyndep information is expected from this node but
	// has not yet been loaded.
	dyndep_pending_ bool

	// The Edge that produces this Node, or NULL when there is no
	// known edge to produce it.
	in_edge_ *Edge

	// All Edges that use this Node as an input.
	out_edges_ []*Edge

	// A dense integer id for the node, assigned and used by DepsLog.
	id_ int
}

func NewNode(path string, slash_bits uint64) *Node {
	return &Node{
		path_:       path,
		slash_bits_: slash_bits,
		mtime_:      -1,
		exists_:     ExistenceStatusUnknown,
		id_:         -1,
	}
}

/*
// Return false on error.
func (n *Node) StatIfNecessary(disk_interface DiskInterface, err *string) bool {
	if status_known() {
		return true
	}
	return Stat(disk_interface, err)
}
*/

// Mark as not-yet-stat()ed and not dirty.
func (n *Node) ResetState() {
	n.mtime_ = -1
	n.exists_ = ExistenceStatusUnknown
	n.dirty_ = false
}

// Mark the Node as already-stat()ed and missing.
func (n *Node) MarkMissing() {
	if n.mtime_ == -1 {
		n.mtime_ = 0
	}
	n.exists_ = ExistenceStatusMissing
}
func (n *Node) exists() bool {
	return n.exists_ == ExistenceStatusExists
}
func (n *Node) status_known() bool {
	return n.exists_ != ExistenceStatusUnknown
}
func (n *Node) path() string {
	return n.path_
}

// Get |path()| but use slash_bits to convert back to original slash styles.
func (n *Node) PathDecanonicalized() string {
	return PathDecanonicalized(n.path_, n.slash_bits_)
}

func (n *Node) slash_bits() uint64 {
	return n.slash_bits_
}
func (n *Node) mtime() TimeStamp {
	return n.mtime_
}
func (n *Node) dirty() bool {
	return n.dirty_
}
func (n *Node) set_dirty(dirty bool) {
	n.dirty_ = dirty
}
func (n *Node) MarkDirty() {
	n.dirty_ = true
}
func (n *Node) dyndep_pending() bool {
	return n.dyndep_pending_
}
func (n *Node) set_dyndep_pending(pending bool) {
	n.dyndep_pending_ = pending
}
func (n *Node) in_edge() *Edge {
	return n.in_edge_
}
func (n *Node) set_in_edge(edge *Edge) {
	n.in_edge_ = edge
}
func (n *Node) id() int {
	return n.id_
}
func (n *Node) set_id(id int) {
	n.id_ = id
}
func (n *Node) out_edges() []*Edge {
	return n.out_edges_
}
func (n *Node) AddOutEdge(edge *Edge) {
	n.out_edges_ = append(n.out_edges_, edge)
}

type ExistenceStatus int

const (
	// The file hasn't been examined.
	ExistenceStatusUnknown ExistenceStatus = iota
	// The file doesn't exist. mtime_ will be the latest mtime of its dependencies.
	ExistenceStatusMissing
	// The path is an actual file. mtime_ will be the file's mtime.
	ExistenceStatusExists
)

// An edge in the dependency graph; links between Nodes using Rules.
type Edge struct {
	rule_                    *Rule
	pool_                    *Pool
	inputs_                  []*Node
	outputs_                 []*Node
	dyndep_                  *Node
	env_                     *BindingEnv
	mark_                    VisitMark
	id_                      int
	outputs_ready_           bool
	deps_loaded_             bool
	deps_missing_            bool
	generated_by_dep_loader_ bool

	// There are three types of inputs.
	// 1) explicit deps, which show up as $in on the command line;
	// 2) implicit deps, which the target depends on implicitly (e.g. C headers),
	//                   and changes in them cause the target to rebuild;
	// 3) order-only deps, which are needed before the target builds but which
	//                     don't cause the target to rebuild.
	// These are stored in inputs_ in that order, and we keep counts of
	// #2 and #3 when we need to access the various subsets.
	implicit_deps_   int
	order_only_deps_ int

	// There are two types of outputs.
	// 1) explicit outs, which show up as $out on the command line;
	// 2) implicit outs, which the target generates but are not part of $out.
	// These are stored in outputs_ in that order, and we keep a count of
	// #2 to use when we need to access the various subsets.
	implicit_outs_ int
}
type VisitMark int

const (
	VisitNone VisitMark = iota
	VisitInStack
	VisitDone
)

func NewEdge() *Edge {
	return &Edge{
		rule_:   nil,
		pool_:   nil,
		dyndep_: nil,
		env_:    nil,
		mark_:   VisitNone,
	}
}
func (e *Edge) rule() *Rule {
	return e.rule_
}
func (e *Edge) pool() *Pool {
	return e.pool_
}
func (e *Edge) weight() int {
	return 1
}
func (e *Edge) outputs_ready() bool {
	return e.outputs_ready_
}
func (e *Edge) is_implicit(index int) bool {
	return index >= len(e.inputs_)-e.order_only_deps_-e.implicit_deps_ && !e.is_order_only(index)
}
func (e *Edge) is_order_only(index int) bool {
	return index >= len(e.inputs_)-e.order_only_deps_
}
func (e *Edge) is_implicit_out(index int) bool {
	return index >= len(e.outputs_)-e.implicit_outs_
}

/*
type EdgeCmp struct {
  bool operator()(const Edge* a, const Edge* b) const {
    return a.id_ < b.id_
  }
}
type EdgeSet map[Edge*, EdgeCmp]struct{}
*/

type EdgeSet map[*Edge]struct{}

// ImplicitDepLoader loads implicit dependencies, as referenced via the
// "depfile" attribute in build files.
type ImplicitDepLoader struct {
	state_                  *State
	disk_interface_         DiskInterface
	deps_log_               *DepsLog
	depfile_parser_options_ *DepfileParserOptions
}

func NewImplicitDepLoader(state *State, deps_log *DepsLog, disk_interface DiskInterface, depfile_parser_options *DepfileParserOptions) ImplicitDepLoader {
	return ImplicitDepLoader{
		state_:                  state,
		disk_interface_:         disk_interface,
		deps_log_:               deps_log,
		depfile_parser_options_: depfile_parser_options,
	}
}

func (i *ImplicitDepLoader) deps_log() *DepsLog {
	return i.deps_log_
}

// DependencyScan manages the process of scanning the files in a graph
// and updating the dirty/outputs_ready state of all the nodes and edges.
type DependencyScan struct {
	build_log_      *BuildLog
	disk_interface_ DiskInterface
	dep_loader_     ImplicitDepLoader
	dyndep_loader_  DyndepLoader
}

func NewDependencyScan(state *State, build_log *BuildLog, deps_log *DepsLog, disk_interface DiskInterface, depfile_parser_options *DepfileParserOptions) DependencyScan {
	return DependencyScan{
		build_log_:      build_log,
		disk_interface_: disk_interface,
		dep_loader_:     NewImplicitDepLoader(state, deps_log, disk_interface, depfile_parser_options),
		dyndep_loader_:  NewDyndepLoader(state, disk_interface),
	}
}

func (d *DependencyScan) build_log() *BuildLog {
	return d.build_log_
}
func (d *DependencyScan) set_build_log(log *BuildLog) {
	d.build_log_ = log
}

func (d *DependencyScan) deps_log() *DepsLog {
	return d.dep_loader_.deps_log()
}

/*
// Return false on error.
func (n *Node) Stat(disk_interface DiskInterface, err *string) bool {
	METRIC_RECORD("node stat")
	n.mtime_ = disk_interface.Stat(n.path_, err)
	if n.mtime_ == -1 {
		return false
	}
	n.exists_ = ExistenceStatusMissing
	if n.mtime_ != 0 {
		n.exists_ = ExistenceStatusExists
	}
	return true
}
*/
// If the file doesn't exist, set the mtime_ from its dependencies
func (n *Node) UpdatePhonyMtime(mtime TimeStamp) {
	if !n.exists() {
		if mtime > n.mtime_ {
			n.mtime_ = mtime
		}
	}
}

/*
// Update the |dirty_| state of the given node by inspecting its input edge.
// Examine inputs, outputs, and command lines to judge whether an edge
// needs to be re-run, and update outputs_ready_ and each outputs' |dirty_|
// state accordingly.
// Returns false on failure.
func (d *DependencyScan) RecomputeDirty(node *Node, err *string) bool {
	var stack []*Node
	return d.RecomputeDirty(node, &stack, err)
}
*/

// Update the |dirty_| state of the given node by inspecting its input edge.
// Examine inputs, outputs, and command lines to judge whether an edge
// needs to be re-run, and update outputs_ready_ and each outputs' |dirty_|
// state accordingly.
// Returns false on failure.
func (d *DependencyScan) RecomputeDirty(node *Node, stack *[]*Node, err *string) bool {
	panic("TODO")
	/*
			edge := node.in_edge()
			if edge == nil {
				// If we already visited this leaf node then we are done.
				if node.status_known() {
					return true
				}
				// This node has no in-edge; it is dirty if it is missing.
				if !node.StatIfNecessary(d.disk_interface_, err) {
					return false
				}
				if !node.exists() {
					EXPLAIN("%s has no in-edge and is missing", node.path())
				}
				node.set_dirty(!node.exists())
				return true
			}

			// If we already finished this edge then we are done.
			if edge.mark_ == VisitDone {
				return true
			}

			// If we encountered this edge earlier in the call stack we have a cycle.
			if !VerifyDAG(node, stack, err) {
				return false
			}

			// Mark the edge temporarily while in the call stack.
			edge.mark_ = VisitInStack
			*stack = append(*stack, node)

			dirty := false
			edge.outputs_ready_ = true
			edge.deps_missing_ = false

			if !edge.deps_loaded_ {
				// This is our first encounter with this edge.
				// If there is a pending dyndep file, visit it now:
				// * If the dyndep file is ready then load it now to get any
				//   additional inputs and outputs for this and other edges.
				//   Once the dyndep file is loaded it will no longer be pending
				//   if any other edges encounter it, but they will already have
				//   been updated.
				// * If the dyndep file is not ready then since is known to be an
				//   input to this edge, the edge will not be considered ready below.
				//   Later during the build the dyndep file will become ready and be
				//   loaded to update this edge before it can possibly be scheduled.
				if edge.dyndep_ && edge.dyndep_.dyndep_pending() {
					if !d.RecomputeDirty(edge.dyndep_, stack, err) {
						return false
					}

					if !edge.dyndep_.in_edge() || edge.dyndep_.in_edge().outputs_ready() {
						// The dyndep file is ready, so load it now.
						if !LoadDyndeps(edge.dyndep_, err) {
							return false
						}
					}
				}
			}

			// Load output mtimes so we can compare them to the most recent input below.
			for o := edge.outputs_.begin(); o != edge.outputs_.end(); o++ {
				if !(*o).StatIfNecessary(d.disk_interface_, err) {
					return false
				}
			}

			if !edge.deps_loaded_ {
				// This is our first encounter with this edge.  Load discovered deps.
				edge.deps_loaded_ = true
				if !d.dep_loader_.LoadDeps(edge, err) {
					if len(err) != 0 {
						return false
					}
					// Failed to load dependency info: rebuild to regenerate it.
					// LoadDeps() did EXPLAIN() already, no need to do it here.
					dirty = true
					edge.deps_missing_ = true
				}
			}

			// Visit all inputs; we're dirty if any of the inputs are dirty.
			most_recent_input := nil
			for i := edge.inputs_.begin(); i != edge.inputs_.end(); i++ {
				// Visit this input.
				if !d.RecomputeDirty(*i, stack, err) {
					return false
				}

				// If an input is not ready, neither are our outputs.
				if in_edge := i.in_edge(); in_edge != nil {
					if !in_edge.outputs_ready_ {
						edge.outputs_ready_ = false
					}
				}

				if !edge.is_order_only(i - edge.inputs_.begin()) {
					// If a regular input is dirty (or missing), we're dirty.
					// Otherwise consider mtime.
					if (*i).dirty() {
						EXPLAIN("%s is dirty", (*i).path())
						dirty = true
					} else {
						if !most_recent_input || (*i).mtime() > most_recent_input.mtime() {
							most_recent_input = *i
						}
					}
				}
			}

			// We may also be dirty due to output state: missing outputs, out of
			// date outputs, etc.  Visit all outputs and determine whether they're dirty.
			if dirty == nil {
				if !d.RecomputeOutputsDirty(edge, most_recent_input, &dirty, err) {
					return false
				}
			}

			// Finally, visit each output and update their dirty state if necessary.
			for o := edge.outputs_.begin(); o != edge.outputs_.end(); o++ {
				if dirty != nil {
					(*o).MarkDirty()
				}
			}

			// If an edge is dirty, its outputs are normally not ready.  (It's
			// possible to be clean but still not be ready in the presence of
			// order-only inputs.)
			// But phony edges with no inputs have nothing to do, so are always
			// ready.
			if dirty && !(edge.is_phony() && edge.inputs_.empty()) {
				edge.outputs_ready_ = false
			}

			// Mark the edge as finished during this walk now that it will no longer
			// be in the call stack.
			edge.mark_ = VisitDone
			if (*stack)[len(*stack)-1] != node {
				panic("oops")
			}
			stack = (*stack)[:len(*stack)-1]

			return true
		}

		func (d *DependencyScan) VerifyDAG(node *Node, stack *[]*Node, err *string) bool {
			edge := node.in_edge()
			if !edge != nil {
				panic("oops")
			}

			// If we have no temporary mark on the edge then we do not yet have a cycle.
			if edge.mark_ != VisitInStack {
				return true
			}

			// We have this edge earlier in the call stack.  Find it.
			start := -1
			for _, i := range stack {
				if stack[i].in_edge() == edge {
					start = i
					break
				}
			}
			if start == -1 {
				panic("oops")
			}

			// Make the cycle clear by reporting its start as the node at its end
			// instead of some other output of the starting edge.  For example,
			// running 'ninja b' on
			//   build a b: cat c
			//   build c: cat a
			// should report a -> c -> a instead of b -> c -> a.
			*start = node

			// Construct the error message rejecting the cycle.
			*err = "dependency cycle: "
			for i := start; i != stack.end(); i++ {
				err.append((*i).path())
				err.append(" . ")
			}
			err.append((*start).path())

			if (start+1) == stack.end() && edge.maybe_phonycycle_diagnostic() {
				// The manifest parser would have filtered out the self-referencing
				// input if it were not configured to allow the error.
				err.append(" [-w phonycycle=err]")
			}
	*/
	return false
}

// Recompute whether any output of the edge is dirty, if so sets |*dirty|.
// Returns false on failure.
func (d *DependencyScan) RecomputeOutputsDirty(edge *Edge, most_recent_input *Node, outputs_dirty *bool, err *string) bool {
	panic("TODO")
	/*
		command := edge.EvaluateCommand(true) // incl_rsp_file=
		for o := edge.outputs_.begin(); o != edge.outputs_.end(); o++ {
			if RecomputeOutputDirty(edge, most_recent_input, command, *o) {
				*outputs_dirty = true
				return true
			}
		}
	*/
	return true
}

// Recompute whether a given single output should be marked dirty.
// Returns true if so.
func (d *DependencyScan) RecomputeOutputDirty(edge *Edge, most_recent_input *Node, command string, output *Node) bool {
	panic("TODO")
	/*
		if edge.is_phony() {
			// Phony edges don't write any output.  Outputs are only dirty if
			// there are no inputs and we're missing the output.
			if edge.inputs_.empty() && !output.exists() {
				EXPLAIN("output %s of phony edge with no inputs doesn't exist", output.path())
				return true
			}

			// Update the mtime with the newest input. Dependents can thus call mtime()
			// on the fake node and get the latest mtime of the dependencies
			if most_recent_input {
				output.UpdatePhonyMtime(most_recent_input.mtime())
			}

			// Phony edges are clean, nothing to do
			return false
		}

		entry := 0

		// Dirty if we're missing the output.
		if !output.exists() {
			EXPLAIN("output %s doesn't exist", output.path())
			return true
		}

		// Dirty if the output is older than the input.
		if most_recent_input && output.mtime() < most_recent_input.mtime() {
			output_mtime := output.mtime()

			// If this is a restat rule, we may have cleaned the output with a restat
			// rule in a previous run and stored the most recent input mtime in the
			// build log.  Use that mtime instead, so that the file will only be
			// considered dirty if an input was modified since the previous run.
			used_restat := false
			if edge.GetBindingBool("restat") && build_log() {
				if entry := build_log().LookupByOutput(output.path()); entry != nil {
					output_mtime = entry.mtime
					used_restat = true
				}
			}

			if output_mtime < most_recent_input.mtime() {
				s := ""
				if used_restat {
					s = "restat of "
				}
				EXPLAIN("%soutput %s older than most recent input %s (%x vs %x)", s, output.path(), most_recent_input.path(), output_mtime, most_recent_input.mtime())
				return true
			}
		}

		if build_log() {
			generator := edge.GetBindingBool("generator")
			if entry == nil {
				entry = build_log().LookupByOutput(output.path())
			}
			if entry {
				if !generator && HashCommand(command) != entry.command_hash {
					// May also be dirty due to the command changing since the last build.
					// But if this is a generator rule, the command changing does not make us
					// dirty.
					EXPLAIN("command line changed for %s", output.path())
					return true
				}
				if most_recent_input && entry.mtime < most_recent_input.mtime() {
					// May also be dirty due to the mtime in the log being older than the
					// mtime of the most recent input.  This can occur even when the mtime
					// on disk is newer if a previous run wrote to the output file but
					// exited with an error or was interrupted.
					EXPLAIN("recorded mtime of %s older than most recent input %s (%x vs %x)", output.path(), most_recent_input.path(), entry.mtime, most_recent_input.mtime())
					return true
				}
			}
			if !entry && !generator {
				EXPLAIN("command line not found in log for %s", output.path())
				return true
			}
		}
	*/
	return false
}

/*
// Load a dyndep file from the given node's path and update the
// build graph with the new information.  One overload accepts
// a caller-owned 'DyndepFile' object in which to store the
// information loaded from the dyndep file.
func (d *DependencyScan) LoadDyndeps(node *Node, err *string) bool {
	return d.dyndep_loader_.LoadDyndeps(node, err)
}
*/

// Load a dyndep file from the given node's path and update the
// build graph with the new information.  One overload accepts
// a caller-owned 'DyndepFile' object in which to store the
// information loaded from the dyndep file.
func (d *DependencyScan) LoadDyndeps(node *Node, ddf DyndepFile, err *string) bool {
	return d.dyndep_loader_.LoadDyndeps(node, ddf, err)
}

// Return true if all inputs' in-edges are ready.
func (e *Edge) AllInputsReady() bool {
	for _, i := range e.inputs_ {
		if i.in_edge() != nil && !i.in_edge().outputs_ready() {
			return false
		}
	}
	return true
}

// An Env for an Edge, providing $in and $out.
type EdgeEnv struct {
	lookups_       []string
	edge_          *Edge
	escape_in_out_ EscapeKind
	recursive_     bool
}
type EscapeKind int

const (
	kShellEscape EscapeKind = iota
	kDoNotEscape
)

func NewEdgeEnv(edge *Edge, escape EscapeKind) EdgeEnv {
	return EdgeEnv{
		edge_:          edge,
		escape_in_out_: escape,
	}
}

/*
func (e *EdgeEnv) LookupVariable(var2 string) string {
	if var2 == "in" || var2 == "in_newline" {
		explicit_deps_count := e.edge_.inputs_.size() - e.edge_.implicit_deps_ - e.edge_.order_only_deps_
		s := "\n"
		if var2 == "in" {
			s = " "
		}
		return MakePathList(e.edge_.inputs_.data(), explicit_deps_count, s) //#else//return MakePathList(&edge_.inputs_[0], explicit_deps_count,//#endif
	} else if var2 == "out" {
		explicit_outs_count = e.edge_.outputs_.size() - e.edge_.implicit_outs_
		return MakePathList(&e.edge_.outputs_[0], explicit_outs_count, ' ')
	}

	if e.recursive_ {
		i := 0
		for ; i < len(e.lookups_); i++ {
			if e.lookups_[i] == var2 {
				break
			}
		}
		if i != len(e.lookups_) {
			cycle := ""
			for ; i < len(e.lookups_); i++ {
				cycle += e.lookups_[i] + " . "
			}
			cycle += var2
			Fatal(("cycle in rule variables: " + cycle))
		}
	}

	// See notes on BindingEnv::LookupWithFallback.
	eval := e.edge_.rule_.GetBinding(var2)
	if e.recursive_ && eval {
		e.lookups_.push_back(var2)
	}

	// In practice, variables defined on rules never use another rule variable.
	// For performance, only start checking for cycles after the first lookup.
	e.recursive_ = true
	return e.edge_.env_.LookupWithFallback(var2, eval, this)
}

// Given a span of Nodes, construct a list of paths suitable for a command
// line.
func (e *EdgeEnv) MakePathList(span **Node, size uint, sep char) string {
	result := ""
	for i := span; i != span+size; i++ {
		if len(result) != 0 {
			result = sep + result
		}
		path := i.PathDecanonicalized()
		if e.escape_in_out_ == kShellEscape {
			GetWin32EscapedString(path, &result)
		} else {
			result += path
		}
	}
	return result
}
*/
// Expand all variables in a command and return it as a string.
// If incl_rsp_file is enabled, the string will also contain the
// full contents of a response file (if applicable)
func (e *Edge) EvaluateCommand(incl_rsp_file bool) string {
	command := e.GetBinding("command")
	if incl_rsp_file {
		rspfile_content := e.GetBinding("rspfile_content")
		if rspfile_content != "" {
			command += ";rspfile=" + rspfile_content
		}
	}
	return command
}

// Returns the shell-escaped value of |key|.
func (e *Edge) GetBinding(key string) string {
	//env := NewEdgeEnv(e, kShellEscape)
	// TODO: return env.LookupVariable(key)
	return ""
}

func (e *Edge) GetBindingBool(key string) bool {
	return e.GetBinding(key) != ""
}

// Like GetBinding("depfile"), but without shell escaping.
func (e *Edge) GetUnescapedDepfile() string {
	//TODO
	//env := NewEdgeEnv(e, kDoNotEscape)
	//return env.LookupVariable("depfile")
	return ""
}

// Like GetBinding("dyndep"), but without shell escaping.
func (e *Edge) GetUnescapedDyndep() string {
	//TODO
	//env := NewEdgeEnv(e, kDoNotEscape)
	//return env.LookupVariable("dyndep")
	return ""
}

// Like GetBinding("rspfile"), but without shell escaping.
func (e *Edge) GetUnescapedRspfile() string {
	//TODO
	//env := NewEdgeEnv(e, kDoNotEscape)
	//return env.LookupVariable("rspfile")
	return ""
}

func (e *Edge) Dump(prefix string) {
	printf("%s[ ", prefix)
	for _, i := range e.inputs_ {
		if i != nil {
			printf("%s ", i.path())
		}
	}
	printf("--%s. ", e.rule_.name())
	for _, i := range e.outputs_ {
		printf("%s ", i.path())
	}
	if e.pool_ != nil {
		if e.pool_.name() != "" {
			printf("(in pool '%s')", e.pool_.name())
		}
	} else {
		printf("(null pool?)")
	}
	printf("] 0x%p\n", e)
}

func (e *Edge) is_phony() bool {
	return e.rule_ == kPhonyRule
}

func (e *Edge) use_console() bool {
	return e.pool() == kConsolePool
}

/*
func (e *Edge) maybe_phonycycle_diagnostic() bool {
	// CMake 2.8.12.x and 3.0.x produced self-referencing phony rules
	// of the form "build a: phony ... a ...".   Restrict our
	// "phonycycle" diagnostic option to the form it used.
	return is_phony() && e.outputs_.size() == 1 && e.implicit_outs_ == 0 &&
		e.implicit_deps_ == 0
}
*/

// static
func PathDecanonicalized(path string, slash_bits uint64) string {
	result := []byte(path)
	mask := uint64(1)
	c := 0
	for {
		c = bytes.IndexByte(result[c:], '/')
		if c == -1 {
			break
		}
		if slash_bits&mask != 0 {
			result[c] = '\\'
		}
		c++
		mask <<= 1
	}
	return string(result)
}

/*
func (n *Node) Dump(prefix string) {
	s := ""
	if !n.exists() {
		s = " (:missing)"
	}
	t := " clean"
	if dirty() {
		t = " dirty"
	}
	printf("%s <%s 0x%p> mtime: %x%s, (:%s), ", prefix, path(), this, mtime(), s, t)
	if in_edge() {
		in_edge().Dump("in-edge: ")
	} else {
		printf("no in-edge\n")
	}
	printf(" out edges:\n")
	for e := out_edges().begin(); e != out_edges().end() && *e != nil; e++ {
		(*e).Dump(" +- ")
	}
}
*/
// Load implicit dependencies for \a edge.
// @return false on error (without filling \a err if info is just missing
//                          or out of date).
func (i *ImplicitDepLoader) LoadDeps(edge *Edge, err *string) bool {
	deps_type := edge.GetBinding("deps")
	if len(deps_type) != 0 {
		return i.LoadDepsFromLog(edge, err)
	}

	depfile := edge.GetUnescapedDepfile()
	if len(depfile) != 0 {
		return i.LoadDepFile(edge, depfile, err)
	}

	// No deps to load.
	return true
}

/*
type matches struct {
	/*
	   bool operator()(const Node* node) const {
	     string opath = string(node.path())
	     return *i_ == opath
	   }
	* /
	i_ int
}

func Newmatches(i int) matches {
	return matches{
		i_: i,
	}
}
*/
// Load implicit dependencies for \a edge from a depfile attribute.
// @return false on error (without filling \a err if info is just missing).
func (i *ImplicitDepLoader) LoadDepFile(edge *Edge, path string, err *string) bool {
	//TODO
	/*
			METRIC_RECORD("depfile load")
			// Read depfile content.  Treat a missing depfile as empty.
			content := ""
			switch i.disk_interface_.ReadFile(path, &content, err) {
			case Okay:
				break
			case NotFound:
				err = nil
				break
			case OtherError:
				*err = "loading '" + path + "': " + *err
				return false
			}
			// On a missing depfile: return false and empty *err.
			if len(content) == 0 {
				EXPLAIN("depfile '%s' is missing", path)
				return false
			}

			x := DepfileParserOptions()
			if i.depfile_parser_options_ != nil {
				x = *i.depfile_parser_options_
			}
			depfile := NewDepfileParser(x)
			depfile_err := ""
			if !depfile.Parse(&content, &depfile_err) {
				*err = path + ": " + depfile_err
				return false
			}

			if depfile.outs_.empty() {
				*err = path + ": no outputs declared"
				return false
			}

			var unused uint64
			primary_out := depfile.outs_.begin()
			CanonicalizePath(primary_out.str_, &primary_out.len_, &unused)

			// Check that this depfile matches the edge's output, if not return false to
			// mark the edge as dirty.
			first_output := edge.outputs_[0]
			opath := string(first_output.path())
			if opath != *primary_out {
				EXPLAIN("expected depfile '%s' to mention '%s', got '%s'", path, first_output.path(), primary_out.AsString())
				return false
			}

			// Ensure that all mentioned outputs are outputs of the edge.
			for o := depfile.outs_.begin(); o != depfile.outs_.end(); o++ {
				m := matches(o)
				if find_if(edge.outputs_.begin(), edge.outputs_.end(), m) == edge.outputs_.end() {
					*err = path + ": depfile mentions '" + o.AsString() + "' as an output, but no such output was declared"
					return false
				}
			}
		return i.ProcessDepfileDeps(edge, &depfile.ins_, err)
	*/
	return false
}

// Process loaded implicit dependencies for \a edge and update the graph
// @return false on error (without filling \a err if info is just missing)
func (i *ImplicitDepLoader) ProcessDepfileDeps(edge *Edge, depfile_ins []string, err *string) bool {
	// Preallocate space in edge->inputs_ to be filled in below.
	implicit_dep := i.PreallocateSpace(edge, len(depfile_ins))

	// Add all its in-edges.
	for _, j := range depfile_ins {
		var slash_bits uint64
		j = CanonicalizePath(j, &slash_bits)
		node := i.state_.GetNode(j, slash_bits)
		//*implicit_dep = node
		node.AddOutEdge(edge)
		i.CreatePhonyInEdge(node)
		implicit_dep++
	}

	return true
}

// Load implicit dependencies for \a edge from the DepsLog.
// @return false on error (without filling \a err if info is just missing).
func (i *ImplicitDepLoader) LoadDepsFromLog(edge *Edge, err *string) bool {
	// NOTE: deps are only supported for single-target edges.
	output := edge.outputs_[0]
	var deps *Deps
	if i.deps_log_ != nil {
		deps = i.deps_log_.GetDeps(output)
	}
	if deps == nil {
		//EXPLAIN("deps for '%s' are missing", output.path())
		return false
	}

	// Deps are invalid if the output is newer than the deps.
	if output.mtime() > deps.mtime {
		//EXPLAIN("stored deps info out of date for '%s' (%x vs %x)", output.path(), deps.mtime, output.mtime())
		return false
	}

	implicit_dep := i.PreallocateSpace(edge, deps.node_count)
	for j := 0; j < deps.node_count; j++ {
		node := deps.nodes[j]
		//*implicit_dep = node
		node.AddOutEdge(edge)
		i.CreatePhonyInEdge(node)
		implicit_dep++
	}
	return true
}

// Preallocate \a count spaces in the input array on \a edge, returning
// an iterator pointing at the first new space.
func (i *ImplicitDepLoader) PreallocateSpace(edge *Edge, count int) int {
	//TODO
	/*
		edge.inputs_.insert(edge.inputs_.end()-edge.order_only_deps_, count, 0)
		edge.implicit_deps_ += count
		return edge.inputs_.end() - edge.order_only_deps_ - count
	*/
	return 0
}

// If we don't have a edge that generates this input already,
// create one; this makes us not abort if the input is missing,
// but instead will rebuild in that circumstance.
func (i *ImplicitDepLoader) CreatePhonyInEdge(node *Node) {
	if node.in_edge() != nil {
		return
	}

	phony_edge := i.state_.AddEdge(kPhonyRule)
	phony_edge.generated_by_dep_loader_ = true
	node.set_in_edge(phony_edge)
	phony_edge.outputs_ = append(phony_edge.outputs_, node)

	// RecomputeDirty might not be called for phony_edge if a previous call
	// to RecomputeDirty had caused the file to be stat'ed.  Because previous
	// invocations of RecomputeDirty would have seen this node without an
	// input edge (and therefore ready), we have to set outputs_ready_ to true
	// to avoid a potential stuck build.  If we do call RecomputeDirty for
	// this node, it will simply set outputs_ready_ to the correct value.
	phony_edge.outputs_ready_ = true
}
