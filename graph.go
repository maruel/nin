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
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
)

// ExistenceStatus represents the knowledge of the file's existence.
type ExistenceStatus int32

const (
	// ExistenceStatusUnknown means the file hasn't been examined.
	ExistenceStatusUnknown ExistenceStatus = iota
	// ExistenceStatusMissing means the file doesn't exist. MTime will be the
	// latest mtime of its dependencies.
	ExistenceStatusMissing
	// ExistenceStatusExists means the path is an actual file. MTime will be the
	// file's mtime.
	ExistenceStatusExists
)

// Node represents information about a node in the dependency graph: the file,
// whether it's dirty, mtime, etc.
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

func (n *Node) statIfNecessary(di DiskInterface) error {
	if n.Exists != ExistenceStatusUnknown {
		return nil
	}
	return n.Stat(di)
}

// PathDecanonicalized return |Path| but use SlashBits to convert back to
// original slash styles.
func (n *Node) PathDecanonicalized() string {
	return PathDecanonicalized(n.Path, n.SlashBits)
}

// Stat stat's the file.
func (n *Node) Stat(di DiskInterface) error {
	defer metricRecord("node stat")()
	mtime, err := di.Stat(n.Path)
	n.MTime = mtime
	if mtime == -1 {
		return err
	}
	if n.MTime != 0 {
		n.Exists = ExistenceStatusExists
	} else {
		n.Exists = ExistenceStatusMissing
	}
	return nil
}

// If the file doesn't exist, set the MTime from its dependencies
func (n *Node) updatePhonyMtime(mtime TimeStamp) {
	if n.Exists != ExistenceStatusExists {
		if mtime > n.MTime {
			n.MTime = mtime
		}
	}
}

// Dump prints out Node's details to stdout.
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

// VisitMark is a market to determine if an edge is visited.
type VisitMark int32

// Valid VisitMark values.
const (
	VisitNone VisitMark = iota
	VisitInStack
	VisitDone
)

// Edge is an edge in the dependency graph; links between Nodes using Rules.
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

// If this ever gets changed, update DelayedEdgesSet to take this into account.
func (e *Edge) weight() int {
	return 1
}

// IsImplicit returns if the inputs at the specified index is implicit and not
// for ordering only.
func (e *Edge) IsImplicit(index int) bool {
	return index >= len(e.Inputs)-int(e.OrderOnlyDeps)-int(e.ImplicitDeps) && !e.IsOrderOnly(index)
}

// IsOrderOnly returns if the input at the specified index is only used for
// ordering.
func (e *Edge) IsOrderOnly(index int) bool {
	return index >= len(e.Inputs)-int(e.OrderOnlyDeps)
}

// isImplicitOut is only used in unit tests.
func (e *Edge) isImplicitOut(index int) bool {
	return index >= len(e.Outputs)-int(e.ImplicitOuts)
}

// EvaluateCommand expands all variables in a command and return it as a string.
//
// If inclRspFile is enabled, the string will also contain the
// full contents of a response file (if applicable)
func (e *Edge) EvaluateCommand(inclRspFile bool) string {
	command := e.GetBinding("command")
	if inclRspFile {
		rspfileContent := e.GetBinding("rspfile_content")
		if rspfileContent != "" {
			command += ";rspfile=" + rspfileContent
		}
	}
	return command
}

// GetBinding returns the shell-escaped value of |key|.
func (e *Edge) GetBinding(key string) string {
	env := edgeEnv{
		edge:        e,
		escapeInOut: shellEscape,
	}
	return env.LookupVariable(key)
}

// GetUnescapedDepfile returns like GetBinding("depfile"), but without shell
// escaping.
func (e *Edge) GetUnescapedDepfile() string {
	env := edgeEnv{
		edge:        e,
		escapeInOut: doNotEscape,
	}
	return env.LookupVariable("depfile")
}

// GetUnescapedDyndep returns like GetBinding("dyndep"), but without shell
// escaping.
func (e *Edge) GetUnescapedDyndep() string {
	env := edgeEnv{
		edge:        e,
		escapeInOut: doNotEscape,
	}
	return env.LookupVariable("dyndep")
}

// GetUnescapedRspfile returns like GetBinding("rspfile"), but without shell
// escaping.
func (e *Edge) GetUnescapedRspfile() string {
	env := edgeEnv{
		edge:        e,
		escapeInOut: doNotEscape,
	}
	return env.LookupVariable("rspfile")
}

// Dump prints the Edge details to stdout.
func (e *Edge) Dump(prefix string) {
	fmt.Printf("%s[ ", prefix)
	for _, i := range e.Inputs {
		if i != nil {
			fmt.Printf("%s ", i.Path)
		}
	}
	fmt.Printf("--%s-> ", e.Rule.Name)
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
		if e.Pool.Name != "" {
			fmt.Printf("(in pool '%s')", e.Pool.Name)
		}
	} else {
		fmt.Printf("(null pool?)")
	}
	fmt.Printf("] 0x%p\n", e)
}

func (e *Edge) maybePhonycycleDiagnostic() bool {
	// CMake 2.8.12.x and 3.0.x produced self-referencing phony rules
	// of the form "build a: phony ... a ...".   Restrict our
	// "phonycycle" diagnostic option to the form it used.
	return e.Rule == PhonyRule && len(e.Outputs) == 1 && e.ImplicitOuts == 0 && e.ImplicitDeps == 0
}

// Return true if all inputs' in-edges are ready.
func (e *Edge) allInputsReady() bool {
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

// NewEdgeSet returns an initialized EdgeSet.
func NewEdgeSet() *EdgeSet {
	return &EdgeSet{
		edges: make(map[*Edge]struct{}),
	}
}

// IsEmpty return true if the set is empty.
func (e *EdgeSet) IsEmpty() bool {
	return len(e.edges) == 0
}

// Add the edge to the set.
func (e *EdgeSet) Add(ed *Edge) {
	e.edges[ed] = struct{}{}
	e.dirty = true
}

// Pop returns the lowest ID.
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

type escapeKind bool

const (
	shellEscape escapeKind = false
	doNotEscape escapeKind = true
)

// An Env for an Edge, providing $in and $out.
type edgeEnv struct {
	lookups     []string
	edge        *Edge
	escapeInOut escapeKind
	recursive   bool
}

func (e *edgeEnv) LookupVariable(v string) string {
	edge := e.edge
	switch v {
	case "in":
		explicitDepsCount := len(edge.Inputs) - int(edge.ImplicitDeps) - int(edge.OrderOnlyDeps)
		return makePathList(edge.Inputs[:explicitDepsCount], ' ', e.escapeInOut)
	case "in_newline":
		explicitDepsCount := len(edge.Inputs) - int(edge.ImplicitDeps) - int(edge.OrderOnlyDeps)
		return makePathList(edge.Inputs[:explicitDepsCount], '\n', e.escapeInOut)
	case "out":
		explicitOutsCount := len(edge.Outputs) - int(edge.ImplicitOuts)
		return makePathList(edge.Outputs[:explicitOutsCount], ' ', e.escapeInOut)
	default:
		// TODO(maruel): Remove here and move to a post parsing evaluation in a
		// separate goroutine.
		for i := 0; i < len(e.lookups); i++ {
			if e.lookups[i] == v {
				cycle := ""
				for ; i < len(e.lookups); i++ {
					cycle += e.lookups[i] + " -> "
				}
				cycle += v
				fatalf("cycle in rule variables: " + cycle)
			}
		}

		// See notes on BindingEnv.lookupWithFallback.
		eval := edge.Rule.Bindings[v]
		if e.recursive {
			if eval != nil {
				e.lookups = append(e.lookups, v)
			}
		} else {
			// In practice, variables defined on rules never use another rule variable.
			e.recursive = true
		}
		return edge.Env.lookupWithFallback(v, eval, e)
	}
}

// Given a span of Nodes, construct a list of paths suitable for a command
// line.
func makePathList(span []*Node, sep byte, escapeInOut escapeKind) string {
	var z [64]string
	var s []string
	if l := len(span); l <= cap(z) {
		s = z[:l]
	} else {
		s = make([]string, l)
	}
	total := 0
	first := false
	for i, x := range span {
		path := x.PathDecanonicalized()
		if escapeInOut == shellEscape {
			if runtime.GOOS == "windows" {
				path = getWin32EscapedString(path)
			} else {
				path = getShellEscapedString(path)
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

// PathDecanonicalized does the reverse process of CanonicalizePath().
//
// Only does anything on Windows.
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
// and updating the dirty/outputsReady state of all the nodes and edges.
type DependencyScan struct {
	buildLog     *BuildLog
	di           DiskInterface
	depLoader    implicitDepLoader
	dyndepLoader DyndepLoader
}

// NewDependencyScan returns an initialized DependencyScan.
func NewDependencyScan(state *State, buildLog *BuildLog, depsLog *DepsLog, di DiskInterface) DependencyScan {
	return DependencyScan{
		buildLog:     buildLog,
		di:           di,
		depLoader:    newImplicitDepLoader(state, depsLog, di),
		dyndepLoader: NewDyndepLoader(state, di),
	}
}

func (d *DependencyScan) depsLog() *DepsLog {
	return d.depLoader.depsLog
}

// RecomputeDirty updates the |dirty| state of the given Node by transitively
// inspecting their input edges.
//
// Examine inputs, outputs, and command lines to judge whether an edge
// needs to be re-run, and update OutputsReady and each outputs' Dirty
// state accordingly.
//
// Appends any validation nodes found to the nodes parameter.
func (d *DependencyScan) RecomputeDirty(initialNode *Node) ([]*Node, error) {
	var stack, validationNodes, newValidationNodes []*Node
	// The C++ code uses a dequeue.
	nodes := []*Node{initialNode}

	// recomputeNodeDirty might return new validation nodes that need to be
	// checked for dirty state, keep a queue of nodes to visit.
	for len(nodes) != 0 {
		node := nodes[0]
		nodes = nodes[1:]
		// Reuse slices to reduce overall memory allocations.
		stack = stack[:0]
		newValidationNodes = newValidationNodes[:0]
		var err error
		stack, newValidationNodes, err = d.recomputeNodeDirty(node, stack, newValidationNodes)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, newValidationNodes...)
		validationNodes = append(validationNodes, newValidationNodes...)
	}
	return validationNodes, nil
}

// recomputeNodeDirty updates Node.Dirty.
//
// It is recursive.
func (d *DependencyScan) recomputeNodeDirty(node *Node, stack, validationNodes []*Node) ([]*Node, []*Node, error) {
	edge := node.InEdge
	if edge == nil {
		// If we already visited this leaf node then we are done.
		if node.Exists != ExistenceStatusUnknown {
			return stack, validationNodes, nil
		}
		// This node has no in-edge; it is dirty if it is missing.
		if err := node.statIfNecessary(d.di); err != nil {
			return stack, validationNodes, err
		}
		if node.Exists != ExistenceStatusExists {
			explain("%s has no in-edge and is missing", node.Path)
		}
		node.Dirty = node.Exists != ExistenceStatusExists
		return stack, validationNodes, nil
	}

	// If we already finished this edge then we are done.
	if edge.Mark == VisitDone {
		return stack, validationNodes, nil
	}

	// If we encountered this edge earlier in the call stack we have a cycle.
	if err := d.verifyDAG(node, stack); err != nil {
		return stack, validationNodes, err
	}

	// Mark the edge temporarily while in the call stack.
	edge.Mark = VisitInStack
	stack = append(stack, node)

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
			var err error
			stack, validationNodes, err = d.recomputeNodeDirty(edge.Dyndep, stack, validationNodes)
			if err != nil {
				return stack, validationNodes, err
			}

			if edge.Dyndep.InEdge == nil || edge.Dyndep.InEdge.OutputsReady {
				// The dyndep file is ready, so load it now.
				if err := d.LoadDyndeps(edge.Dyndep, DyndepFile{}); err != nil {
					return stack, validationNodes, err
				}
			}
		}
	}

	// Load output mtimes so we can compare them to the most recent input below.
	for _, o := range edge.Outputs {
		if err := o.statIfNecessary(d.di); err != nil {
			return stack, validationNodes, err
		}
	}

	if !edge.DepsLoaded {
		// This is our first encounter with this edge.  Load discovered deps.
		edge.DepsLoaded = true
		if found, err := d.depLoader.loadDeps(edge); err != nil {
			return stack, validationNodes, err
		} else if !found {
			// Failed to load dependency info: rebuild to regenerate it.
			// loadDeps() did Explain() already, no need to do it here.
			dirty = true
			edge.DepsMissing = true
		}
	}

	// Store any validation nodes from the edge for adding to the initial
	// nodes.  Don't recurse into them, that would trigger the dependency
	// cycle detector if the validation node depends on this node.
	// RecomputeDirty will add the validation nodes to the initial nodes
	// and recurse into them.
	validationNodes = append(validationNodes, edge.Validations...)

	// Visit all inputs; we're dirty if any of the inputs are dirty.
	var mostRecentInput *Node
	for j, i := range edge.Inputs {
		// Visit this input.
		var err error
		stack, validationNodes, err = d.recomputeNodeDirty(i, stack, validationNodes)
		if err != nil {
			return stack, validationNodes, err
		}

		// If an input is not ready, neither are our outputs.
		if inEdge := i.InEdge; inEdge != nil {
			if !inEdge.OutputsReady {
				edge.OutputsReady = false
			}
		}

		if !edge.IsOrderOnly(j) {
			// If a regular input is dirty (or missing), we're dirty.
			// Otherwise consider mtime.
			if i.Dirty {
				explain("%s is dirty", i.Path)
				dirty = true
			} else {
				if mostRecentInput == nil || i.MTime > mostRecentInput.MTime {
					mostRecentInput = i
				}
			}
		}
	}

	// We may also be dirty due to output state: missing outputs, out of
	// date outputs, etc.  Visit all outputs and determine whether they're dirty.
	if !dirty {
		// The C++ code conditions on this but I think there's a bug in there.
		dirty = d.recomputeOutputsDirty(edge, mostRecentInput)
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
	// assert(stack[len(stack)-1] == node)
	return stack[:len(stack)-1], validationNodes, nil
}

// verifyDAG checks that the node is a directed acyclic graph.
//
// Mutates stack in-place in case of error.
func (d *DependencyScan) verifyDAG(node *Node, stack []*Node) error {
	edge := node.InEdge

	// If we have no temporary mark on the edge then we do not yet have a cycle.
	if edge.Mark != VisitInStack {
		return nil
	}

	// We have this edge earlier in the call stack.  Find it.
	start := -1
	for i := range stack {
		if stack[i].InEdge == edge {
			start = i
			break
		}
	}

	// Make the cycle clear by reporting its start as the node at its end
	// instead of some other output of the starting edge.  For example,
	// running 'ninja b' on
	//   build a b: cat c
	//   build c: cat a
	// should report a -> c -> a instead of b -> c -> a.
	stack[start] = node

	// Construct the error message rejecting the cycle.
	err := "dependency cycle: "
	for i := start; i != len(stack); i++ {
		err += stack[i].Path
		err += " -> "
	}
	err += stack[start].Path

	if (start+1) == len(stack) && edge.maybePhonycycleDiagnostic() {
		// The manifest parser would have filtered out the self-referencing
		// input if it were not configured to allow the error.
		err += " [-w phonycycle=err]"
	}
	return errors.New(err)
}

// recomputeOutputsDirty recomputes whether any output of the edge is dirty.
//
// Returns true if dirty.
func (d *DependencyScan) recomputeOutputsDirty(edge *Edge, mostRecentInput *Node) bool {
	command := edge.EvaluateCommand(true) // inclRspFile=
	for _, o := range edge.Outputs {
		if d.recomputeOutputDirty(edge, mostRecentInput, command, o) {
			return true
		}
	}
	return false
}

// recomputeOutputDirty recomputes whether a given single output should be
// marked dirty.
//
// Returns true if so.
func (d *DependencyScan) recomputeOutputDirty(edge *Edge, mostRecentInput *Node, command string, output *Node) bool {
	if edge.Rule == PhonyRule {
		// Phony edges don't write any output.  Outputs are only dirty if
		// there are no inputs and we're missing the output.
		if len(edge.Inputs) == 0 && output.Exists != ExistenceStatusExists {
			explain("output %s of phony edge with no inputs doesn't exist", output.Path)
			return true
		}

		// Update the mtime with the newest input. Dependents can thus call mtime()
		// on the fake node and get the latest mtime of the dependencies
		if mostRecentInput != nil {
			output.updatePhonyMtime(mostRecentInput.MTime)
		}

		// Phony edges are clean, nothing to do.
		return false
	}

	var entry *LogEntry

	// Dirty if we're missing the output.
	if output.Exists != ExistenceStatusExists {
		explain("output %s doesn't exist", output.Path)
		return true
	}

	// Dirty if the output is older than the input.
	if mostRecentInput != nil && output.MTime < mostRecentInput.MTime {
		outputMtime := output.MTime

		// If this is a restat rule, we may have cleaned the output with a restat
		// rule in a previous run and stored the most recent input mtime in the
		// build log.  Use that mtime instead, so that the file will only be
		// considered dirty if an input was modified since the previous run.
		usedRestat := false
		if edge.GetBinding("restat") != "" && d.buildLog != nil {
			if entry = d.buildLog.Entries[output.Path]; entry != nil {
				outputMtime = entry.mtime
				usedRestat = true
			}
		}

		if outputMtime < mostRecentInput.MTime {
			s := ""
			if usedRestat {
				s = "restat of "
			}
			explain("%soutput %s older than most recent input %s (%x vs %x)", s, output.Path, mostRecentInput.Path, outputMtime, mostRecentInput.MTime)
			return true
		}
	}

	if d.buildLog != nil {
		generator := edge.GetBinding("generator") != ""
		if entry == nil {
			entry = d.buildLog.Entries[output.Path]
		}
		if entry != nil {
			if !generator && HashCommand(command) != entry.commandHash {
				// May also be dirty due to the command changing since the last build.
				// But if this is a generator rule, the command changing does not make us
				// dirty.
				explain("command line changed for %s", output.Path)
				return true
			}
			if mostRecentInput != nil && entry.mtime < mostRecentInput.MTime {
				// May also be dirty due to the mtime in the log being older than the
				// mtime of the most recent input.  This can occur even when the mtime
				// on disk is newer if a previous run wrote to the output file but
				// exited with an error or was interrupted.
				explain("recorded mtime of %s older than most recent input %s (%x vs %x)", output.Path, mostRecentInput.Path, entry.mtime, mostRecentInput.MTime)
				return true
			}
		}
		if entry == nil && !generator {
			explain("command line not found in log for %s", output.Path)
			return true
		}
	}
	return false
}

// LoadDyndeps loads a dyndep file from the given node's path and update the
// build graph with the new information.
//
// The 'DyndepFile' object stores the information loaded from the dyndep file.
func (d *DependencyScan) LoadDyndeps(node *Node, ddf DyndepFile) error {
	return d.dyndepLoader.LoadDyndeps(node, ddf)
}

//

// implicitDepLoader loads implicit dependencies, as referenced via the
// "depfile" attribute in build files.
type implicitDepLoader struct {
	state   *State
	di      DiskInterface
	depsLog *DepsLog
}

func newImplicitDepLoader(state *State, depsLog *DepsLog, di DiskInterface) implicitDepLoader {
	return implicitDepLoader{
		state:   state,
		di:      di,
		depsLog: depsLog,
	}
}

// loadDeps loads implicit dependencies for edge.
//
// Returns false if info is just missing or out of date.
func (i *implicitDepLoader) loadDeps(edge *Edge) (bool, error) {
	depsType := edge.GetBinding("deps")
	if len(depsType) != 0 {
		return i.loadDepsFromLog(edge), nil
	}

	depfile := edge.GetUnescapedDepfile()
	if len(depfile) != 0 {
		return i.loadDepFile(edge, depfile)
	}

	// No deps to load.
	return true, nil
}

// loadDepFile loads implicit dependencies for edge from a depfile attribute.
//
// Returns false if info is just missing or on error.
func (i *implicitDepLoader) loadDepFile(edge *Edge, path string) (bool, error) {
	defer metricRecord("depfile load")()
	// Read depfile content.  Treat a missing depfile as empty.
	content, err := i.di.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		// TODO(maruel): Use %q for real quoting.
		return false, fmt.Errorf("loading '%s': %w", path, err)
	}
	// On a missing depfile: return false and empty error.
	if len(content) == 0 {
		// TODO(maruel): Use %q for real quoting.
		explain("depfile '%s' is missing", path)
		return false, nil
	}

	depfile := DepfileParser{}
	if err := depfile.Parse(content); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}

	if len(depfile.outs) == 0 {
		return false, errors.New(path + ": no outputs declared")
	}

	// Check that this depfile matches the edge's output, if not return false to
	// mark the edge as dirty.
	firstOutput := edge.Outputs[0]
	if primaryOut := CanonicalizePath(depfile.outs[0]); firstOutput.Path != primaryOut {
		explain("expected depfile '%s' to mention '%s', got '%s'", path, firstOutput.Path, primaryOut)
		return false, nil
	}

	// Ensure that all mentioned outputs are outputs of the edge.
	for _, o := range depfile.outs {
		found := false
		for _, n := range edge.Outputs {
			if n.Path == o {
				found = true
				break
			}
		}
		if !found {
			// TODO(maruel): Use %q for real quoting.
			return false, fmt.Errorf("%s: depfile mentions '%s' as an output, but no such output was declared", path, o)
		}
	}
	return i.processDepfileDeps(edge, depfile.ins), nil
}

// processDepfileDeps processes loaded implicit dependencies for edge and
// update the graph.
//
// Returns false with info is just missing.
func (i *implicitDepLoader) processDepfileDeps(edge *Edge, depfileIns []string) bool {
	// Preallocate space in edge.Inputs to be filled in below.
	implicitDep := i.preallocateSpace(edge, len(depfileIns))

	// Add all its in-edges.
	for _, j := range depfileIns {
		node := i.state.GetNode(CanonicalizePathBits(j))
		edge.Inputs[implicitDep] = node
		node.OutEdges = append(node.OutEdges, edge)
		i.createPhonyInEdge(node)
		implicitDep++
	}
	return true
}

// loadDepsFromLog loads implicit dependencies for edge from the DepsLog.
//
// Returns false if info is missing.
func (i *implicitDepLoader) loadDepsFromLog(edge *Edge) bool {
	// NOTE: deps are only supported for single-target edges.
	output := edge.Outputs[0]
	var deps *Deps
	if i.depsLog != nil {
		deps = i.depsLog.GetDeps(output)
	}
	if deps == nil {
		explain("deps for '%s' are missing", output.Path)
		return false
	}

	// Deps are invalid if the output is newer than the deps.
	if output.MTime > deps.MTime {
		explain("stored deps info out of date for '%s' (%x vs %x)", output.Path, deps.MTime, output.MTime)
		return false
	}

	implicitDep := i.preallocateSpace(edge, len(deps.Nodes))
	for _, node := range deps.Nodes {
		edge.Inputs[implicitDep] = node
		node.OutEdges = append(node.OutEdges, edge)
		i.createPhonyInEdge(node)
		implicitDep++
	}
	return true
}

// preallocateSpace preallocates count spaces in the input array on edge,
// returning the index at the first new space.
func (i *implicitDepLoader) preallocateSpace(edge *Edge, count int) int {
	offset := len(edge.Inputs) - int(edge.OrderOnlyDeps)
	old := edge.Inputs
	edge.Inputs = make([]*Node, len(old)+count)
	copy(edge.Inputs, old[:offset])
	copy(edge.Inputs[offset+count:], old[offset:])
	edge.ImplicitDeps += int32(count)
	return len(edge.Inputs) - int(edge.OrderOnlyDeps) - count
}

// createPhonyInEdge creates an edge that generates this input if we don't have
// one already.
//
// This makes us not abort if the input is missing, but instead will rebuild in
// that circumstance.
func (i *implicitDepLoader) createPhonyInEdge(node *Node) {
	if node.InEdge != nil {
		return
	}

	phonyEdge := i.state.addEdge(PhonyRule)
	phonyEdge.GeneratedByDepLoader = true
	node.InEdge = phonyEdge
	phonyEdge.Outputs = append(phonyEdge.Outputs, node)

	// RecomputeDirty might not be called for phonyEdge if a previous call
	// to RecomputeDirty had caused the file to be stat'ed.  Because previous
	// invocations of RecomputeDirty would have seen this node without an
	// input edge (and therefore ready), we have to set OutputsReady to true
	// to avoid a potential stuck build.  If we do call RecomputeDirty for
	// this node, it will simply set OutputsReady to the correct value.
	phonyEdge.OutputsReady = true
}
