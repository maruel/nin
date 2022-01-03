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
	"bytes"
	"fmt"
	"runtime"
	"sort"
)

type ExistenceStatus int32

const (
	// The file hasn't been examined.
	ExistenceStatusUnknown ExistenceStatus = iota
	// The file doesn't exist. MTime will be the latest mtime of its dependencies.
	ExistenceStatusMissing
	// The path is an actual file. MTime will be the file's mtime.
	ExistenceStatusExists
)

// Information about a node in the dependency graph: the file, whether
// it's dirty, mtime, etc.
type Node struct {
	// Immutable.

	// Path is the path of the file that this node represents.
	Path string

	// Set bits starting from lowest for backslashes that were normalized to
	// forward slashes by CanonicalizePathBits. See |PathDecanonicalized|.
	SlashBits uint64

	// Mutable.

	// The Edge that produces this Node, or NULL when there is no
	// known edge to produce it.
	InEdge *Edge

	// All Edges that use this Node as an input.
	OutEdges []*Edge

	// All Edges that use this Node as a validation.
	ValidationOutEdges []*Edge

	// Possible values of MTime:
	//   -1: file hasn't been examined
	//   0:  we looked, and file doesn't exist
	//   >0: actual file's mtime, or the latest mtime of its dependencies if it doesn't exist
	MTime TimeStamp

	// A dense integer id for the node, assigned and used by DepsLog.
	ID int32

	Exists ExistenceStatus

	// Dirty is true when the underlying file is out-of-date.
	// But note that Edge.OutputsReady is also used in judging which
	// edges to build.
	Dirty bool

	// Store whether dyndep information is expected from this node but
	// has not yet been loaded.
	DyndepPending bool
}

func NewNode(path string, slashBits uint64) *Node {
	return &Node{
		Path:      path,
		SlashBits: slashBits,
		MTime:     -1,
		ID:        -1,
		Exists:    ExistenceStatusUnknown,
	}
}

// Return false on error.
func (n *Node) StatIfNecessary(disk_interface DiskInterface, err *string) bool {
	if n.Exists != ExistenceStatusUnknown {
		return true
	}
	return n.Stat(disk_interface, err)
}

// Get |Path| but use SlashBits to convert back to original slash styles.
func (n *Node) PathDecanonicalized() string {
	return PathDecanonicalized(n.Path, n.SlashBits)
}

// Return false on error.
func (n *Node) Stat(disk_interface DiskInterface, err *string) bool {
	defer METRIC_RECORD("node stat")()
	n.MTime = disk_interface.Stat(n.Path, err)
	if n.MTime == -1 {
		return false
	}
	if n.MTime != 0 {
		n.Exists = ExistenceStatusExists
	} else {
		n.Exists = ExistenceStatusMissing
	}
	return true
}

// If the file doesn't exist, set the MTime from its dependencies
func (n *Node) UpdatePhonyMtime(mtime TimeStamp) {
	if n.Exists != ExistenceStatusExists {
		if mtime > n.MTime {
			n.MTime = mtime
		}
	}
}

func (n *Node) Dump(prefix string) {
	s := ""
	if n.Exists != ExistenceStatusExists {
		s = " (:missing)"
	}
	t := " clean"
	if n.Dirty {
		t = " dirty"
	}
	fmt.Printf("%s <%s 0x%p> mtime: %x%s, (:%s), ", prefix, n.Path, n, n.MTime, s, t)
	if n.InEdge != nil {
		n.InEdge.Dump("in-edge: ")
	} else {
		fmt.Printf("no in-edge\n")
	}
	fmt.Printf(" out edges:\n")
	for _, e := range n.OutEdges {
		if e == nil {
			break
		}
		e.Dump(" +- ")
	}
	if len(n.ValidationOutEdges) != 0 {
		fmt.Printf(" validation out edges:\n")
		for _, e := range n.ValidationOutEdges {
			e.Dump(" +- ")
		}
	}
}

//

type VisitMark int32

const (
	VisitNone VisitMark = iota
	VisitInStack
	VisitDone
)

// An edge in the dependency graph; links between Nodes using Rules.
type Edge struct {
	Inputs      []*Node
	Outputs     []*Node
	Validations []*Node
	Rule        *Rule
	Pool        *Pool
	Dyndep      *Node
	Env         *BindingEnv
	Mark        VisitMark
	ID          int32

	// There are three types of inputs.
	// 1) explicit deps, which show up as $in on the command line;
	// 2) implicit deps, which the target depends on implicitly (e.g. C headers),
	//                   and changes in them cause the target to rebuild;
	// 3) order-only deps, which are needed before the target builds but which
	//                     don't cause the target to rebuild.
	// These are stored in Inputs in that order, and we keep counts of
	// #2 and #3 when we need to access the various subsets.
	ImplicitDeps  int32
	OrderOnlyDeps int32

	// There are two types of outputs.
	// 1) explicit outs, which show up as $out on the command line;
	// 2) implicit outs, which the target generates but are not part of $out.
	// These are stored in Outputs in that order, and we keep a count of
	// #2 to use when we need to access the various subsets.
	ImplicitOuts int32

	OutputsReady         bool
	DepsLoaded           bool
	DepsMissing          bool
	GeneratedByDepLoader bool
}

func NewEdge() *Edge {
	return &Edge{
		Rule:   nil,
		Pool:   nil,
		Dyndep: nil,
		Env:    nil,
		Mark:   VisitNone,
	}
}

// If this ever gets changed, update DelayedEdgesSet to take this into account.
func (e *Edge) weight() int {
	return 1
}
func (e *Edge) is_implicit(index int) bool {
	return index >= len(e.Inputs)-int(e.OrderOnlyDeps)-int(e.ImplicitDeps) && !e.is_order_only(index)
}
func (e *Edge) is_order_only(index int) bool {
	return index >= len(e.Inputs)-int(e.OrderOnlyDeps)
}
func (e *Edge) is_implicit_out(index int) bool {
	return index >= len(e.Outputs)-int(e.ImplicitOuts)
}

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
	env := NewEdgeEnv(e, kShellEscape)
	return env.LookupVariable(key)
}

// Like GetBinding("depfile"), but without shell escaping.
func (e *Edge) GetUnescapedDepfile() string {
	env := NewEdgeEnv(e, kDoNotEscape)
	return env.LookupVariable("depfile")
}

// Like GetBinding("dyndep"), but without shell escaping.
func (e *Edge) GetUnescapedDyndep() string {
	env := NewEdgeEnv(e, kDoNotEscape)
	return env.LookupVariable("dyndep")
}

// Like GetBinding("rspfile"), but without shell escaping.
func (e *Edge) GetUnescapedRspfile() string {
	env := NewEdgeEnv(e, kDoNotEscape)
	return env.LookupVariable("rspfile")
}

func (e *Edge) Dump(prefix string) {
	fmt.Printf("%s[ ", prefix)
	for _, i := range e.Inputs {
		if i != nil {
			fmt.Printf("%s ", i.Path)
		}
	}
	fmt.Printf("--%s-> ", e.Rule.name())
	for _, i := range e.Outputs {
		fmt.Printf("%s ", i.Path)
	}
	if len(e.Validations) != 0 {
		fmt.Printf(" validations ")
		for _, i := range e.Validations {
			fmt.Printf("%s ", i.Path)
		}
	}
	if e.Pool != nil {
		if e.Pool.name() != "" {
			fmt.Printf("(in pool '%s')", e.Pool.name())
		}
	} else {
		fmt.Printf("(null pool?)")
	}
	fmt.Printf("] 0x%p\n", e)
}

func (e *Edge) maybe_phonycycle_diagnostic() bool {
	// CMake 2.8.12.x and 3.0.x produced self-referencing phony rules
	// of the form "build a: phony ... a ...".   Restrict our
	// "phonycycle" diagnostic option to the form it used.
	return e.Rule == PhonyRule && len(e.Outputs) == 1 && e.ImplicitOuts == 0 && e.ImplicitDeps == 0
}

// Return true if all inputs' in-edges are ready.
func (e *Edge) AllInputsReady() bool {
	for _, i := range e.Inputs {
		if i.InEdge != nil && !i.InEdge.OutputsReady {
			return false
		}
	}
	return true
}

//

// EdgeSet acts as a sorted set of *Edge, so map[*Edge]struct{} but with sorted
// pop.
type EdgeSet struct {
	edges  map[*Edge]struct{}
	dirty  bool
	sorted []*Edge
}

func NewEdgeSet() *EdgeSet {
	return &EdgeSet{
		edges: make(map[*Edge]struct{}),
	}
}

func (e *EdgeSet) IsEmpty() bool {
	return len(e.edges) == 0
}

func (e *EdgeSet) Add(ed *Edge) {
	if ed == nil {
		panic("M-A")
	}
	e.edges[ed] = struct{}{}
	e.dirty = true
}

func (e *EdgeSet) Pop() *Edge {
	e.recreate()
	if len(e.sorted) == 0 {
		return nil
	}
	// Do not set dirty.
	ed := e.sorted[len(e.sorted)-1]
	e.sorted = e.sorted[:len(e.sorted)-1]
	delete(e.edges, ed)
	return ed
}

func (e *EdgeSet) recreate() {
	if !e.dirty {
		return
	}
	e.dirty = false
	if len(e.edges) == 0 {
		if len(e.sorted) != 0 {
			e.sorted = e.sorted[:0]
		}
		return
	}
	// Resize e.sorted to be the same size as e.edges
	le := len(e.edges)
	if cap(e.sorted) < le {
		e.sorted = make([]*Edge, le)
	} else {
		delta := le - len(e.sorted)
		if delta < 0 {
			// TODO(maruel): Not sure how to tell the Go compiler to do it as a
			// single operation.
			for i := 0; i < delta; i++ {
				e.sorted = append(e.sorted, nil)
			}
		} else if delta > 0 {
			e.sorted = e.sorted[:le]
		}
	}
	i := 0
	for k := range e.edges {
		e.sorted[i] = k
		i++
	}
	// Sort in reverse order, so that Pop() removes the last (smallest) item.
	sort.Slice(e.sorted, func(i, j int) bool {
		return e.sorted[i].ID > e.sorted[j].ID
	})
}

//

type EscapeKind bool

const (
	kShellEscape EscapeKind = false
	kDoNotEscape EscapeKind = true
)

// An Env for an Edge, providing $in and $out.
type EdgeEnv struct {
	lookups_       []string
	edge_          *Edge
	escape_in_out_ EscapeKind
	recursive_     bool
}

func NewEdgeEnv(edge *Edge, escape EscapeKind) EdgeEnv {
	return EdgeEnv{
		edge_:          edge,
		escape_in_out_: escape,
	}
}

func (e *EdgeEnv) LookupVariable(var2 string) string {
	if var2 == "in" || var2 == "in_newline" {
		explicit_deps_count := len(e.edge_.Inputs) - int(e.edge_.ImplicitDeps) - int(e.edge_.OrderOnlyDeps)
		s := byte('\n')
		if var2 == "in" {
			s = ' '
		}
		return e.MakePathList(e.edge_.Inputs[:explicit_deps_count], s)
	} else if var2 == "out" {
		explicit_outs_count := len(e.edge_.Outputs) - int(e.edge_.ImplicitOuts)
		return e.MakePathList(e.edge_.Outputs[:explicit_outs_count], ' ')
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
				cycle += e.lookups_[i] + " -> "
			}
			cycle += var2
			Fatal(("cycle in rule variables: " + cycle))
		}
	}

	// See notes on BindingEnv::LookupWithFallback.
	eval := e.edge_.Rule.GetBinding(var2)
	if e.recursive_ && eval != nil {
		e.lookups_ = append(e.lookups_, var2)
	}

	// In practice, variables defined on rules never use another rule variable.
	// For performance, only start checking for cycles after the first lookup.
	e.recursive_ = true
	return e.edge_.Env.LookupWithFallback(var2, eval, e)
}

// Given a span of Nodes, construct a list of paths suitable for a command
// line.
func (e *EdgeEnv) MakePathList(span []*Node, sep byte) string {
	s := make([]string, len(span))
	total := 0
	first := false
	for i, x := range span {
		path := x.PathDecanonicalized()
		if e.escape_in_out_ == kShellEscape {
			if runtime.GOOS == "windows" {
				path = GetWin32EscapedString(path)
			} else {
				path = GetShellEscapedString(path)
			}
		}
		l := len(path)
		if !first {
			if l != 0 {
				first = true
			}
		} else {
			// For the separator.
			total++
		}
		s[i] = path
		total += l
	}

	out := make([]byte, total)
	offset := 0
	for _, x := range s {
		if offset != 0 {
			out[offset] = sep
			offset++
		}
		copy(out[offset:], x)
		offset += len(x)
	}
	return unsafeString(out)
}

func PathDecanonicalized(path string, slashBits uint64) string {
	if runtime.GOOS != "windows" {
		return path
	}
	result := []byte(path)
	mask := uint64(1)

	for c := 0; ; c++ {
		d := bytes.IndexByte(result[c:], '/')
		if d == -1 {
			break
		}
		c += d
		if slashBits&mask != 0 {
			result[c] = '\\'
		}
		mask <<= 1
	}
	return unsafeString(result)
}

//

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

// Update the |dirty_| state of the given node by transitively inspecting their
// input edges.
// Examine inputs, outputs, and command lines to judge whether an edge
// needs to be re-run, and update OutputsReady and each outputs' Dirty
// state accordingly.
// Appends any validation nodes found to the nodes parameter.
// Returns false on failure.
func (d *DependencyScan) RecomputeDirty(initial_node *Node, validation_nodes *[]*Node, err *string) bool {
	var stack, new_validation_nodes []*Node
	nodes := []*Node{initial_node} // dequeue

	// RecomputeNodeDirty might return new validation nodes that need to be
	// checked for dirty state, keep a queue of nodes to visit.
	for len(nodes) != 0 {
		node := nodes[0]
		nodes = nodes[1:]

		stack = stack[:0]
		new_validation_nodes = new_validation_nodes[:0]

		if !d.RecomputeNodeDirty(node, &stack, &new_validation_nodes, err) {
			return false
		}
		nodes = append(nodes, new_validation_nodes...)
		if len(new_validation_nodes) != 0 {
			if validation_nodes == nil {
				panic("validations require RecomputeDirty to be called with validation_nodes")
			}
			*validation_nodes = append(*validation_nodes, new_validation_nodes...)
		}
	}
	return true
}

func (d *DependencyScan) RecomputeNodeDirty(node *Node, stack *[]*Node, validation_nodes *[]*Node, err *string) bool {
	edge := node.InEdge
	if edge == nil {
		// If we already visited this leaf node then we are done.
		if node.Exists != ExistenceStatusUnknown {
			return true
		}
		// This node has no in-edge; it is dirty if it is missing.
		if !node.StatIfNecessary(d.disk_interface_, err) {
			return false
		}
		if node.Exists != ExistenceStatusExists {
			EXPLAIN("%s has no in-edge and is missing", node.Path)
		}
		node.Dirty = node.Exists != ExistenceStatusExists
		return true
	}

	// If we already finished this edge then we are done.
	if edge.Mark == VisitDone {
		return true
	}

	if stack == nil {
		stack = &[]*Node{}
	}

	// If we encountered this edge earlier in the call stack we have a cycle.
	if !d.VerifyDAG(node, *stack, err) {
		return false
	}

	// Mark the edge temporarily while in the call stack.
	edge.Mark = VisitInStack
	*stack = append(*stack, node)

	dirty := false
	edge.OutputsReady = true
	edge.DepsMissing = false

	if !edge.DepsLoaded {
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
		if edge.Dyndep != nil && edge.Dyndep.DyndepPending {
			if !d.RecomputeNodeDirty(edge.Dyndep, stack, validation_nodes, err) {
				return false
			}

			if edge.Dyndep.InEdge == nil || edge.Dyndep.InEdge.OutputsReady {
				// The dyndep file is ready, so load it now.
				if !d.LoadDyndeps(edge.Dyndep, DyndepFile{}, err) {
					return false
				}
			}
		}
	}

	// Load output mtimes so we can compare them to the most recent input below.
	for _, o := range edge.Outputs {
		if !o.StatIfNecessary(d.disk_interface_, err) {
			return false
		}
	}

	if !edge.DepsLoaded {
		// This is our first encounter with this edge.  Load discovered deps.
		edge.DepsLoaded = true
		if !d.dep_loader_.LoadDeps(edge, err) {
			if len(*err) != 0 {
				return false
			}
			// Failed to load dependency info: rebuild to regenerate it.
			// LoadDeps() did EXPLAIN() already, no need to do it here.
			dirty = true
			edge.DepsMissing = true
		}
	}

	// Store any validation nodes from the edge for adding to the initial
	// nodes.  Don't recurse into them, that would trigger the dependency
	// cycle detector if the validation node depends on this node.
	// RecomputeDirty will add the validation nodes to the initial nodes
	// and recurse into them.
	*validation_nodes = append(*validation_nodes, edge.Validations...)

	// Visit all inputs; we're dirty if any of the inputs are dirty.
	var most_recent_input *Node
	for j, i := range edge.Inputs {
		// Visit this input.
		if !d.RecomputeNodeDirty(i, stack, validation_nodes, err) {
			return false
		}

		// If an input is not ready, neither are our outputs.
		if in_edge := i.InEdge; in_edge != nil {
			if !in_edge.OutputsReady {
				edge.OutputsReady = false
			}
		}

		if !edge.is_order_only(j) {
			// If a regular input is dirty (or missing), we're dirty.
			// Otherwise consider mtime.
			if i.Dirty {
				EXPLAIN("%s is dirty", i.Path)
				dirty = true
			} else {
				if most_recent_input == nil || i.MTime > most_recent_input.MTime {
					most_recent_input = i
				}
			}
		}
	}

	// We may also be dirty due to output state: missing outputs, out of
	// date outputs, etc.  Visit all outputs and determine whether they're dirty.
	if !dirty {
		if !d.RecomputeOutputsDirty(edge, most_recent_input, &dirty, err) {
			return false
		}
	}

	// Finally, visit each output and update their dirty state if necessary.
	for _, o := range edge.Outputs {
		if dirty {
			o.Dirty = true
		}
	}

	// If an edge is dirty, its outputs are normally not ready.  (It's
	// possible to be clean but still not be ready in the presence of
	// order-only inputs.)
	// But phony edges with no inputs have nothing to do, so are always
	// ready.
	if dirty && !(edge.Rule == PhonyRule && len(edge.Inputs) == 0) {
		edge.OutputsReady = false
	}

	// Mark the edge as finished during this walk now that it will no longer
	// be in the call stack.
	edge.Mark = VisitDone
	if (*stack)[len(*stack)-1] != node {
		panic("M-A")
	}
	*stack = (*stack)[:len(*stack)-1]
	return true
}

func (d *DependencyScan) VerifyDAG(node *Node, stack []*Node, err *string) bool {
	edge := node.InEdge
	if edge == nil {
		panic("M-A")
	}

	// If we have no temporary mark on the edge then we do not yet have a cycle.
	if edge.Mark != VisitInStack {
		return true
	}

	// We have this edge earlier in the call stack.  Find it.
	start := -1
	for i := range stack {
		if stack[i].InEdge == edge {
			start = i
			break
		}
	}
	if start == -1 {
		panic("M-A")
	}

	// Make the cycle clear by reporting its start as the node at its end
	// instead of some other output of the starting edge.  For example,
	// running 'ninja b' on
	//   build a b: cat c
	//   build c: cat a
	// should report a -> c -> a instead of b -> c -> a.
	stack[start] = node

	// Construct the error message rejecting the cycle.
	*err = "dependency cycle: "
	for i := start; i != len(stack); i++ {
		*err += stack[i].Path
		*err += " -> "
	}
	*err += stack[start].Path

	if (start+1) == len(stack) && edge.maybe_phonycycle_diagnostic() {
		// The manifest parser would have filtered out the self-referencing
		// input if it were not configured to allow the error.
		*err += " [-w phonycycle=err]"
	}
	return false
}

// Recompute whether any output of the edge is dirty, if so sets |*dirty|.
// Returns false on failure.
func (d *DependencyScan) RecomputeOutputsDirty(edge *Edge, most_recent_input *Node, outputs_dirty *bool, err *string) bool {
	command := edge.EvaluateCommand(true) // incl_rsp_file=
	for _, o := range edge.Outputs {
		if d.RecomputeOutputDirty(edge, most_recent_input, command, o) {
			*outputs_dirty = true
			return true
		}
	}
	return true
}

// Recompute whether a given single output should be marked dirty.
// Returns true if so.
func (d *DependencyScan) RecomputeOutputDirty(edge *Edge, most_recent_input *Node, command string, output *Node) bool {
	if edge.Rule == PhonyRule {
		// Phony edges don't write any output.  Outputs are only dirty if
		// there are no inputs and we're missing the output.
		if len(edge.Inputs) == 0 && output.Exists != ExistenceStatusExists {
			EXPLAIN("output %s of phony edge with no inputs doesn't exist", output.Path)
			return true
		}

		// Update the mtime with the newest input. Dependents can thus call mtime()
		// on the fake node and get the latest mtime of the dependencies
		if most_recent_input != nil {
			output.UpdatePhonyMtime(most_recent_input.MTime)
		}

		// Phony edges are clean, nothing to do
		return false
	}

	var entry *LogEntry

	// Dirty if we're missing the output.
	if output.Exists != ExistenceStatusExists {
		EXPLAIN("output %s doesn't exist", output.Path)
		return true
	}

	// Dirty if the output is older than the input.
	if most_recent_input != nil && output.MTime < most_recent_input.MTime {
		output_mtime := output.MTime

		// If this is a restat rule, we may have cleaned the output with a restat
		// rule in a previous run and stored the most recent input mtime in the
		// build log.  Use that mtime instead, so that the file will only be
		// considered dirty if an input was modified since the previous run.
		used_restat := false
		if edge.GetBinding("restat") != "" && d.build_log() != nil {
			if entry = d.build_log().LookupByOutput(output.Path); entry != nil {
				output_mtime = entry.mtime
				used_restat = true
			}
		}

		if output_mtime < most_recent_input.MTime {
			s := ""
			if used_restat {
				s = "restat of "
			}
			EXPLAIN("%soutput %s older than most recent input %s (%x vs %x)", s, output.Path, most_recent_input.Path, output_mtime, most_recent_input.MTime)
			return true
		}
	}

	if d.build_log() != nil {
		generator := edge.GetBinding("generator") != ""
		if entry == nil {
			entry = d.build_log().LookupByOutput(output.Path)
		}
		if entry != nil {
			if !generator && HashCommand(command) != entry.command_hash {
				// May also be dirty due to the command changing since the last build.
				// But if this is a generator rule, the command changing does not make us
				// dirty.
				EXPLAIN("command line changed for %s", output.Path)
				return true
			}
			if most_recent_input != nil && entry.mtime < most_recent_input.MTime {
				// May also be dirty due to the mtime in the log being older than the
				// mtime of the most recent input.  This can occur even when the mtime
				// on disk is newer if a previous run wrote to the output file but
				// exited with an error or was interrupted.
				EXPLAIN("recorded mtime of %s older than most recent input %s (%x vs %x)", output.Path, most_recent_input.Path, entry.mtime, most_recent_input.MTime)
				return true
			}
		}
		if entry == nil && !generator {
			EXPLAIN("command line not found in log for %s", output.Path)
			return true
		}
	}
	return false
}

// Load a dyndep file from the given node's path and update the
// build graph with the new information.
//
// The 'DyndepFile' object stores the information loaded from the dyndep file.
func (d *DependencyScan) LoadDyndeps(node *Node, ddf DyndepFile, err *string) bool {
	return d.dyndep_loader_.LoadDyndeps(node, ddf, err)
}

//

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

// Load implicit dependencies for \a edge from a depfile attribute.
// @return false on error (without filling \a err if info is just missing).
func (i *ImplicitDepLoader) LoadDepFile(edge *Edge, path string, err *string) bool {
	defer METRIC_RECORD("depfile load")()
	// Read depfile content.  Treat a missing depfile as empty.
	content := ""
	switch i.disk_interface_.ReadFile(path, &content, err) {
	case Okay:
	case NotFound:
		*err = ""
	case OtherError:
		*err = "loading '" + path + "': " + *err
		return false
	}
	// On a missing depfile: return false and empty *err.
	if len(content) == 0 {
		EXPLAIN("depfile '%s' is missing", path)
		return false
	}

	x := DepfileParserOptions{}
	if i.depfile_parser_options_ != nil {
		x = *i.depfile_parser_options_
	}
	depfile := NewDepfileParser(x)
	depfile_err := ""
	if !depfile.Parse([]byte(content), &depfile_err) {
		*err = path + ": " + depfile_err
		return false
	}

	if len(depfile.outs_) == 0 {
		*err = path + ": no outputs declared"
		return false
	}

	// Check that this depfile matches the edge's output, if not return false to
	// mark the edge as dirty.
	first_output := edge.Outputs[0]
	if primaryOut := CanonicalizePath(depfile.outs_[0]); first_output.Path != primaryOut {
		EXPLAIN("expected depfile '%s' to mention '%s', got '%s'", path, first_output.Path, primaryOut)
		return false
	}

	// Ensure that all mentioned outputs are outputs of the edge.
	for _, o := range depfile.outs_ {
		found := false
		for _, n := range edge.Outputs {
			if n.Path == o {
				found = true
				break
			}
		}
		if !found {
			*err = path + ": depfile mentions '" + o + "' as an output, but no such output was declared"
			return false
		}
	}
	return i.ProcessDepfileDeps(edge, depfile.ins_, err)
}

// Process loaded implicit dependencies for \a edge and update the graph
// @return false on error (without filling \a err if info is just missing)
func (i *ImplicitDepLoader) ProcessDepfileDeps(edge *Edge, depfile_ins []string, err *string) bool {
	// Preallocate space in edge.Inputs to be filled in below.
	implicit_dep := i.PreallocateSpace(edge, len(depfile_ins))

	// Add all its in-edges.
	for _, j := range depfile_ins {
		node := i.state_.GetNode(CanonicalizePathBits(j))
		edge.Inputs[implicit_dep] = node
		node.OutEdges = append(node.OutEdges, edge)
		i.CreatePhonyInEdge(node)
		implicit_dep++
	}
	return true
}

// Load implicit dependencies for \a edge from the DepsLog.
// @return false on error (without filling \a err if info is just missing).
func (i *ImplicitDepLoader) LoadDepsFromLog(edge *Edge, err *string) bool {
	// NOTE: deps are only supported for single-target edges.
	output := edge.Outputs[0]
	var deps *Deps
	if i.deps_log_ != nil {
		deps = i.deps_log_.GetDeps(output)
	}
	if deps == nil {
		EXPLAIN("deps for '%s' are missing", output.Path)
		return false
	}

	// Deps are invalid if the output is newer than the deps.
	if output.MTime > deps.mtime {
		EXPLAIN("stored deps info out of date for '%s' (%x vs %x)", output.Path, deps.mtime, output.MTime)
		return false
	}

	implicit_dep := i.PreallocateSpace(edge, deps.node_count)
	for j := 0; j < deps.node_count; j++ {
		node := deps.nodes[j]
		edge.Inputs[implicit_dep] = node
		node.OutEdges = append(node.OutEdges, edge)
		i.CreatePhonyInEdge(node)
		implicit_dep++
	}
	return true
}

// Preallocate \a count spaces in the input array on \a edge, returning
// an iterator pointing at the first new space.
func (i *ImplicitDepLoader) PreallocateSpace(edge *Edge, count int) int {
	offset := len(edge.Inputs) - int(edge.OrderOnlyDeps)
	old := edge.Inputs
	edge.Inputs = make([]*Node, len(old)+count)
	copy(edge.Inputs, old[:offset])
	copy(edge.Inputs[offset+count:], old[offset:])
	edge.ImplicitDeps += int32(count)
	return len(edge.Inputs) - int(edge.OrderOnlyDeps) - count
}

// If we don't have a edge that generates this input already,
// create one; this makes us not abort if the input is missing,
// but instead will rebuild in that circumstance.
func (i *ImplicitDepLoader) CreatePhonyInEdge(node *Node) {
	if node.InEdge != nil {
		return
	}

	phony_edge := i.state_.AddEdge(PhonyRule)
	phony_edge.GeneratedByDepLoader = true
	node.InEdge = phony_edge
	phony_edge.Outputs = append(phony_edge.Outputs, node)

	// RecomputeDirty might not be called for phony_edge if a previous call
	// to RecomputeDirty had caused the file to be stat'ed.  Because previous
	// invocations of RecomputeDirty would have seen this node without an
	// input edge (and therefore ready), we have to set OutputsReady to true
	// to avoid a potential stuck build.  If we do call RecomputeDirty for
	// this node, it will simply set OutputsReady to the correct value.
	phony_edge.OutputsReady = true
}
