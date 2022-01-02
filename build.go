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
	"time"
)

type EdgeResult bool

const (
	kEdgeFailed    EdgeResult = false
	kEdgeSucceeded EdgeResult = true
)

// Enumerate possible steps we want for an edge.
type Want int32

const (
	// We do not want to build the edge, but we might want to build one of
	// its dependents.
	kWantNothing Want = iota
	// We want to build the edge, but have not yet scheduled it.
	kWantToStart
	// We want to build the edge, have scheduled it, and are waiting
	// for it to complete.
	kWantToFinish
)

// CommandRunner is an interface that wraps running the build
// subcommands.  This allows tests to abstract out running commands.
// RealCommandRunner is an implementation that actually runs commands.
type CommandRunner interface {
	CanRunMore() bool
	StartCommand(edge *Edge) bool

	/// Wait for a command to complete, or return false if interrupted.
	WaitForCommand(result *Result) bool

	GetActiveEdges() []*Edge
	Abort()
}

// The result of waiting for a command.
type Result struct {
	edge   *Edge
	status ExitStatus
	output string
}

func NewResult() Result {
	return Result{
		edge: nil,
	}
}

func (r *Result) success() bool {
	return r.status == ExitSuccess
}

// Options (e.g. verbosity, parallelism) passed to a build.
type BuildConfig struct {
	verbosity        Verbosity
	dry_run          bool
	parallelism      int
	failures_allowed int
	// The maximum load average we must not exceed. A negative or zero value
	// means that we do not have any limit.
	max_load_average       float64
	depfile_parser_options DepfileParserOptions
}

func NewBuildConfig() BuildConfig {
	return BuildConfig{
		verbosity:        NORMAL,
		parallelism:      1,
		failures_allowed: 1,
	}
}

type Verbosity int32

const (
	QUIET            Verbosity = iota // No output -- used when testing.
	NO_STATUS_UPDATE                  // just regular output but suppress status update
	NORMAL                            // regular output and status update
	VERBOSE
)

type RunningEdgeMap map[*Edge]int32

// A CommandRunner that doesn't actually run the commands.
type DryRunCommandRunner struct {
	finished_ []*Edge
}

// Overridden from CommandRunner:
func (d *DryRunCommandRunner) CanRunMore() bool {
	return true
}

func (d *DryRunCommandRunner) StartCommand(edge *Edge) bool {
	// In C++ it's a queue. In Go it's a bit less efficient but it shouldn't be
	// performance critical.
	// TODO(maruel): Move items when cap() is significantly larger than len().
	d.finished_ = append([]*Edge{edge}, d.finished_...)
	return true
}

func (d *DryRunCommandRunner) WaitForCommand(result *Result) bool {
	if len(d.finished_) == 0 {
		return false
	}

	result.status = ExitSuccess
	result.edge = d.finished_[len(d.finished_)-1]
	d.finished_ = d.finished_[:len(d.finished_)-1]
	return true
}

func (d *DryRunCommandRunner) GetActiveEdges() []*Edge {
	return nil
}

func (d *DryRunCommandRunner) Abort() {
}

type RealCommandRunner struct {
	config_          *BuildConfig
	subprocs_        SubprocessSet
	subproc_to_edge_ map[Subprocess]*Edge
}

func NewRealCommandRunner(config *BuildConfig) *RealCommandRunner {
	return &RealCommandRunner{
		config_:          config,
		subprocs_:        NewSubprocessSet(),
		subproc_to_edge_: map[Subprocess]*Edge{},
	}
}

func (r *RealCommandRunner) GetActiveEdges() []*Edge {
	var edges []*Edge
	for _, e := range r.subproc_to_edge_ {
		edges = append(edges, e)
	}
	return edges
}

func (r *RealCommandRunner) Abort() {
	r.subprocs_.Clear()
}

func (r *RealCommandRunner) CanRunMore() bool {
	subproc_number := r.subprocs_.Running() + r.subprocs_.Finished()
	more := subproc_number < r.config_.parallelism
	load := r.subprocs_.Running() == 0 || r.config_.max_load_average <= 0. || GetLoadAverage() < r.config_.max_load_average
	return more && load
}

func (r *RealCommandRunner) StartCommand(edge *Edge) bool {
	command := edge.EvaluateCommand(false)
	subproc := r.subprocs_.Add(command, edge.use_console())
	if subproc == nil {
		return false
	}
	r.subproc_to_edge_[subproc] = edge
	return true
}

func (r *RealCommandRunner) WaitForCommand(result *Result) bool {
	var subproc Subprocess
	for {
		subproc = r.subprocs_.NextFinished()
		if subproc != nil {
			break
		}
		if r.subprocs_.DoWork() {
			return false
		}
	}

	result.status = subproc.Finish()
	result.output = subproc.GetOutput()

	e := r.subproc_to_edge_[subproc]
	result.edge = e
	delete(r.subproc_to_edge_, subproc)
	return true
}

//

// Plan stores the state of a build plan: what we intend to build,
// which steps we're ready to execute.
type Plan struct {
	// Keep track of which edges we want to build in this plan.  If this map does
	// not contain an entry for an edge, we do not want to build the entry or its
	// dependents.  If it does contain an entry, the enumeration indicates what
	// we want for the edge.
	want_ map[*Edge]Want

	ready_ *EdgeSet

	builder_ *Builder

	// Total number of edges that have commands (not phony).
	command_edges_ int

	// Total remaining number of wanted edges.
	wanted_edges_ int
}

// Returns true if there's more work to be done.
func (p *Plan) more_to_do() bool {
	return p.wanted_edges_ > 0 && p.command_edges_ > 0
}

// Number of edges with commands to run.
func (p *Plan) command_edge_count() int {
	return p.command_edges_
}

func NewPlan(builder *Builder) Plan {
	return Plan{
		want_:    map[*Edge]Want{},
		ready_:   NewEdgeSet(),
		builder_: builder,
	}
}

// Reset state.  Clears want and ready sets.
func (p *Plan) Reset() {
	p.command_edges_ = 0
	p.wanted_edges_ = 0
	p.want_ = map[*Edge]Want{}
	p.ready_ = NewEdgeSet()
}

// Add a target to our plan (including all its dependencies).
// Returns false if we don't need to build this target; may
// fill in |err| with an error message if there's a problem.
func (p *Plan) AddTarget(target *Node, err *string) bool {
	return p.AddSubTarget(target, nil, err, nil)
}

func (p *Plan) AddSubTarget(node *Node, dependent *Node, err *string, dyndep_walk map[*Edge]struct{}) bool {
	edge := node.InEdge
	if edge == nil { // Leaf node.
		if node.Dirty {
			referenced := ""
			if dependent != nil {
				referenced = ", needed by '" + dependent.Path + "',"
			}
			*err = "'" + node.Path + "'" + referenced + " missing and no known rule to make it"
		}
		return false
	}

	if edge.outputs_ready() {
		return false // Don't need to do anything.
	}

	// If an entry in want_ does not already exist for edge, create an entry which
	// maps to kWantNothing, indicating that we do not want to build this entry itself.
	want, ok := p.want_[edge]
	if !ok {
		p.want_[edge] = kWantNothing
	} else if len(dyndep_walk) != 0 && want == kWantToFinish {
		return false // Don't need to do anything with already-scheduled edge.
	}

	// If we do need to build edge and we haven't already marked it as wanted,
	// mark it now.
	if node.Dirty && want == kWantNothing {
		want = kWantToStart
		p.want_[edge] = want
		p.EdgeWanted(edge)
		if len(dyndep_walk) == 0 && edge.AllInputsReady() {
			p.ScheduleWork(edge, want)
		}
	}

	if len(dyndep_walk) != 0 {
		dyndep_walk[edge] = struct{}{}
	}

	if ok {
		return true // We've already processed the inputs.
	}

	for _, i := range edge.inputs_ {
		if !p.AddSubTarget(i, node, err, dyndep_walk) && *err != "" {
			return false
		}
	}
	return true
}

func (p *Plan) EdgeWanted(edge *Edge) {
	p.wanted_edges_++
	if !edge.is_phony() {
		p.command_edges_++
	}
}

// Pop a ready edge off the queue of edges to build.
// Returns NULL if there's no work to do.
func (p *Plan) FindWork() *Edge {
	return p.ready_.Pop()
}

// Submits a ready edge as a candidate for execution.
// The edge may be delayed from running, for example if it's a member of a
// currently-full pool.
func (p *Plan) ScheduleWork(edge *Edge, want Want) {
	if want == kWantToFinish {
		// This edge has already been scheduled.  We can get here again if an edge
		// and one of its dependencies share an order-only input, or if a node
		// duplicates an out edge (see https://github.com/ninja-build/ninja/pull/519).
		// Avoid scheduling the work again.
		return
	}
	if want != kWantToStart {
		panic("M-A")
	}
	p.want_[edge] = kWantToFinish

	pool := edge.pool()
	if pool.ShouldDelayEdge() {
		pool.DelayEdge(edge)
		pool.RetrieveReadyEdges(p.ready_)
	} else {
		pool.EdgeScheduled(edge)
		p.ready_.Add(edge)
	}
}

// Mark an edge as done building (whether it succeeded or failed).
// If any of the edge's outputs are dyndep bindings of their dependents,
// this loads dynamic dependencies from the nodes' paths.
// Returns 'false' if loading dyndep info fails and 'true' otherwise.
func (p *Plan) EdgeFinished(edge *Edge, result EdgeResult, err *string) bool {
	want, ok := p.want_[edge]
	if !ok {
		panic("M-A")
	}
	directly_wanted := want != kWantNothing

	// See if this job frees up any delayed jobs.
	if directly_wanted {
		edge.pool().EdgeFinished(edge)
	}
	edge.pool().RetrieveReadyEdges(p.ready_)

	// The rest of this function only applies to successful commands.
	if result != kEdgeSucceeded {
		return true
	}

	if directly_wanted {
		p.wanted_edges_--
	}
	delete(p.want_, edge)
	edge.outputs_ready_ = true

	// Check off any nodes we were waiting for with this edge.
	for _, o := range edge.outputs_ {
		if !p.NodeFinished(o, err) {
			return false
		}
	}
	return true
}

// Update plan with knowledge that the given node is up to date.
// If the node is a dyndep binding on any of its dependents, this
// loads dynamic dependencies from the node's path.
// Returns 'false' if loading dyndep info fails and 'true' otherwise.
func (p *Plan) NodeFinished(node *Node, err *string) bool {
	// If this node provides dyndep info, load it now.
	if node.dyndep_pending() {
		if p.builder_ == nil {
			panic("dyndep requires Plan to have a Builder")
		}
		// Load the now-clean dyndep file.  This will also update the
		// build plan and schedule any new work that is ready.
		return p.builder_.LoadDyndeps(node, err)
	}

	// See if we we want any edges from this node.
	for _, oe := range node.OutEdges {
		want, ok := p.want_[oe]
		if !ok {
			continue
		}

		// See if the edge is now ready.
		if !p.EdgeMaybeReady(oe, want, err) {
			return false
		}
	}
	return true
}

func (p *Plan) EdgeMaybeReady(edge *Edge, want Want, err *string) bool {
	if edge.AllInputsReady() {
		if want != kWantNothing {
			p.ScheduleWork(edge, want)
		} else {
			// We do not need to build this edge, but we might need to build one of
			// its dependents.
			if !p.EdgeFinished(edge, kEdgeSucceeded, err) {
				return false
			}
		}
	}
	return true
}

// Clean the given node during the build.
// Return false on error.
func (p *Plan) CleanNode(scan *DependencyScan, node *Node, err *string) bool {
	node.Dirty = false

	for _, oe := range node.OutEdges {
		// Don't process edges that we don't actually want.
		want, ok := p.want_[oe]
		if !ok || want == kWantNothing {
			continue
		}

		// Don't attempt to clean an edge if it failed to load deps.
		if oe.deps_missing_ {
			continue
		}

		// If all non-order-only inputs for this edge are now clean,
		// we might have changed the dirty state of the outputs.
		end := len(oe.inputs_) - int(oe.order_only_deps_)
		found := false
		for i := 0; i < end; i++ {
			if oe.inputs_[i].Dirty {
				found = true
				break
			}
		}
		if !found {
			// Recompute most_recent_input.
			most_recent_input := -1
			for i := 0; i != end; i++ {
				if most_recent_input == -1 || oe.inputs_[i].mtime() > oe.inputs_[most_recent_input].mtime() {
					most_recent_input = i
				}
			}

			// Now, this edge is dirty if any of the outputs are dirty.
			// If the edge isn't dirty, clean the outputs and mark the edge as not
			// wanted.
			outputs_dirty := false
			if !scan.RecomputeOutputsDirty(oe, oe.inputs_[most_recent_input], &outputs_dirty, err) {
				return false
			}
			if !outputs_dirty {
				for _, o := range oe.outputs_ {
					if !p.CleanNode(scan, o, err) {
						return false
					}
				}

				p.want_[oe] = kWantNothing
				p.wanted_edges_--
				if !oe.is_phony() {
					p.command_edges_--
				}
			}
		}
	}
	return true
}

// Update the build plan to account for modifications made to the graph
// by information loaded from a dyndep file.
func (p *Plan) DyndepsLoaded(scan *DependencyScan, node *Node, ddf DyndepFile, err *string) bool {
	// Recompute the dirty state of all our direct and indirect dependents now
	// that our dyndep information has been loaded.
	if !p.RefreshDyndepDependents(scan, node, err) {
		return false
	}

	// We loaded dyndep information for those out_edges of the dyndep node that
	// specify the node in a dyndep binding, but they may not be in the plan.
	// Starting with those already in the plan, walk newly-reachable portion
	// of the graph through the dyndep-discovered dependencies.

	// Find edges in the the build plan for which we have new dyndep info.
	var dyndep_roots []*Edge
	for edge := range ddf {
		// If the edge outputs are ready we do not need to consider it here.
		if edge.outputs_ready() {
			continue
		}

		// If the edge has not been encountered before then nothing already in the
		// plan depends on it so we do not need to consider the edge yet either.
		if _, ok := p.want_[edge]; !ok {
			continue
		}

		// This edge is already in the plan so queue it for the walk.
		dyndep_roots = append(dyndep_roots, edge)
	}

	// Walk dyndep-discovered portion of the graph to add it to the build plan.
	dyndep_walk := map[*Edge]struct{}{}
	for _, oe := range dyndep_roots {
		for _, i := range ddf[oe].implicit_inputs_ {
			if !p.AddSubTarget(i, oe.outputs_[0], err, dyndep_walk) && *err != "" {
				return false
			}
		}
	}

	// Add out edges from this node that are in the plan (just as
	// NodeFinished would have without taking the dyndep code path).
	for _, oe := range node.OutEdges {
		if _, ok := p.want_[oe]; !ok {
			continue
		}
		dyndep_walk[oe] = struct{}{}
	}

	// See if any encountered edges are now ready.
	for wi := range dyndep_walk {
		want, ok := p.want_[wi]
		if !ok {
			continue
		}
		if !p.EdgeMaybeReady(wi, want, err) {
			return false
		}
	}
	return true
}

func (p *Plan) RefreshDyndepDependents(scan *DependencyScan, node *Node, err *string) bool {
	// Collect the transitive closure of dependents and mark their edges
	// as not yet visited by RecomputeDirty.
	dependents := map[*Node]struct{}{}
	p.UnmarkDependents(node, dependents)

	// Update the dirty state of all dependents and check if their edges
	// have become wanted.
	for n := range dependents {
		// Check if this dependent node is now dirty.  Also checks for new cycles.
		var validation_nodes []*Node
		if !scan.RecomputeDirty(n, &validation_nodes, err) {
			return false
		}

		// Add any validation nodes found during RecomputeDirty as new top level
		// targets.
		for _, v := range validation_nodes {
			if in_edge := v.InEdge; in_edge != nil {
				if !in_edge.outputs_ready() && !p.AddTarget(v, err) {
					return false
				}
			}
		}
		if !n.Dirty {
			continue
		}

		// This edge was encountered before.  However, we may not have wanted to
		// build it if the outputs were not known to be dirty.  With dyndep
		// information an output is now known to be dirty, so we want the edge.
		edge := n.InEdge
		if edge == nil || edge.outputs_ready() {
			panic("M-A")
		}
		want_e, ok := p.want_[edge]
		if !ok {
			panic("M-A")
		}
		if want_e == kWantNothing {
			p.want_[edge] = kWantToStart
			p.EdgeWanted(edge)
		}
	}
	return true
}

func (p *Plan) UnmarkDependents(node *Node, dependents map[*Node]struct{}) {
	for _, edge := range node.OutEdges {
		_, ok := p.want_[edge]
		if !ok {
			continue
		}

		if edge.mark_ != VisitNone {
			edge.mark_ = VisitNone
			for _, o := range edge.outputs_ {
				_, ok := dependents[o]
				if ok {
					p.UnmarkDependents(o, dependents)
				} else {
					dependents[o] = struct{}{}
				}
			}
		}
	}
}

// Dumps the current state of the plan.
func (p *Plan) Dump() {
	fmt.Printf("pending: %d\n", len(p.want_))
	for e, w := range p.want_ {
		if w != kWantNothing {
			fmt.Printf("want ")
		}
		e.Dump("")
	}
	// TODO(maruel): Uses inner knowledge
	fmt.Printf("ready:\n")
	p.ready_.recreate()
	for i := range p.ready_.sorted {
		fmt.Printf("\t")
		p.ready_.sorted[len(p.ready_.sorted)-1-i].Dump("")
	}
}

//

// Builder wraps the build process: starting commands, updating status.
type Builder struct {
	state_          *State
	config_         *BuildConfig
	plan_           Plan
	command_runner_ CommandRunner
	status_         Status

	// Map of running edge to time the edge started running.
	running_edges_ RunningEdgeMap

	// Time the build started.
	start_time_millis_ int64

	disk_interface_ DiskInterface
	scan_           DependencyScan
}

// Used for tests.
func (b *Builder) SetBuildLog(log *BuildLog) {
	b.scan_.set_build_log(log)
}

func NewBuilder(state *State, config *BuildConfig, build_log *BuildLog, deps_log *DepsLog, disk_interface DiskInterface, status Status, start_time_millis int64) *Builder {
	b := &Builder{
		state_:             state,
		config_:            config,
		status_:            status,
		running_edges_:     RunningEdgeMap{},
		start_time_millis_: start_time_millis,
		disk_interface_:    disk_interface,
	}
	b.plan_ = NewPlan(b)
	b.scan_ = NewDependencyScan(state, build_log, deps_log, disk_interface, &b.config_.depfile_parser_options)
	return b
}

// TODO(maruel): Make sure it's always called where important.
func (b *Builder) Destructor() {
	b.Cleanup()
}

// Clean up after interrupted commands by deleting output files.
func (b *Builder) Cleanup() {
	if b.command_runner_ != nil {
		active_edges := b.command_runner_.GetActiveEdges()
		b.command_runner_.Abort()

		for _, e := range active_edges {
			depfile := e.GetUnescapedDepfile()
			for _, o := range e.outputs_ {
				// Only delete this output if it was actually modified.  This is
				// important for things like the generator where we don't want to
				// delete the manifest file if we can avoid it.  But if the rule
				// uses a depfile, always delete.  (Consider the case where we
				// need to rebuild an output because of a modified header file
				// mentioned in a depfile, and the command touches its depfile
				// but is interrupted before it touches its output file.)
				err := ""
				new_mtime := b.disk_interface_.Stat(o.Path, &err)
				if new_mtime == -1 { // Log and ignore Stat() errors.
					b.status_.Error("%s", err)
				}
				if depfile != "" || o.mtime() != new_mtime {
					b.disk_interface_.RemoveFile(o.Path)
				}
			}
			if len(depfile) != 0 {
				b.disk_interface_.RemoveFile(depfile)
			}
		}
	}
}

// Add a target to the build, scanning dependencies.
// @return false on error.
func (b *Builder) AddTargetName(name string, err *string) *Node {
	node := b.state_.LookupNode(name)
	if node == nil {
		*err = "unknown target: '" + name + "'"
		return nil
	}
	if !b.AddTarget(node, err) {
		return nil
	}
	return node
}

// Add a target to the build, scanning dependencies.
// @return false on error.
func (b *Builder) AddTarget(target *Node, err *string) bool {
	var validation_nodes []*Node
	if !b.scan_.RecomputeDirty(target, &validation_nodes, err) {
		return false
	}

	in_edge := target.InEdge
	if in_edge == nil || !in_edge.outputs_ready() {
		if !b.plan_.AddTarget(target, err) {
			return false
		}
	}

	// Also add any validation nodes found during RecomputeDirty as top level
	// targets.
	for _, n := range validation_nodes {
		if validation_in_edge := n.InEdge; validation_in_edge != nil {
			if !validation_in_edge.outputs_ready() && !b.plan_.AddTarget(n, err) {
				return false
			}
		}
	}

	return true
}

// Returns true if the build targets are already up to date.
func (b *Builder) AlreadyUpToDate() bool {
	return !b.plan_.more_to_do()
}

// Run the build.  Returns false on error.
// It is an error to call this function when AlreadyUpToDate() is true.
func (b *Builder) Build(err *string) bool {
	if b.AlreadyUpToDate() {
		panic("M-A")
	}

	b.status_.PlanHasTotalEdges(b.plan_.command_edge_count())
	pending_commands := 0
	failures_allowed := b.config_.failures_allowed

	// Set up the command runner if we haven't done so already.
	if b.command_runner_ == nil {
		if b.config_.dry_run {
			b.command_runner_ = &DryRunCommandRunner{}
		} else {
			b.command_runner_ = NewRealCommandRunner(b.config_)
		}
	}

	// We are about to start the build process.
	b.status_.BuildStarted()

	// This main loop runs the entire build process.
	// It is structured like this:
	// First, we attempt to start as many commands as allowed by the
	// command runner.
	// Second, we attempt to wait for / reap the next finished command.
	for b.plan_.more_to_do() {
		// See if we can start any more commands.
		if failures_allowed != 0 && b.command_runner_.CanRunMore() {
			if edge := b.plan_.FindWork(); edge != nil {
				if edge.GetBindingBool("generator") {
					_ = b.scan_.build_log().Close()
				}

				if !b.StartEdge(edge, err) {
					b.Cleanup()
					b.status_.BuildFinished()
					return false
				}

				if edge.is_phony() {
					if !b.plan_.EdgeFinished(edge, kEdgeSucceeded, err) {
						b.Cleanup()
						b.status_.BuildFinished()
						return false
					}
				} else {
					pending_commands++
				}

				// We made some progress; go back to the main loop.
				continue
			}
		}

		// See if we can reap any finished commands.
		if pending_commands != 0 {
			var result Result
			if !b.command_runner_.WaitForCommand(&result) || result.status == ExitInterrupted {
				b.Cleanup()
				b.status_.BuildFinished()
				*err = "interrupted by user"
				return false
			}

			pending_commands--
			if !b.FinishCommand(&result, err) {
				b.Cleanup()
				b.status_.BuildFinished()
				return false
			}

			if !result.success() {
				if failures_allowed != 0 {
					failures_allowed--
				}
			}

			// We made some progress; start the main loop over.
			continue
		}

		// If we get here, we cannot make any more progress.
		b.status_.BuildFinished()
		if failures_allowed == 0 {
			if b.config_.failures_allowed > 1 {
				*err = "subcommands failed"
			} else {
				*err = "subcommand failed"
			}
		} else if failures_allowed < b.config_.failures_allowed {
			*err = "cannot make progress due to previous errors"
		} else {
			*err = "stuck [this is a bug]"
		}

		return false
	}
	b.status_.BuildFinished()
	return true
}

func (b *Builder) StartEdge(edge *Edge, err *string) bool {
	defer METRIC_RECORD("StartEdge")()
	if edge.is_phony() {
		return true
	}
	start_time_millis := int32(time.Now().UnixMilli() - b.start_time_millis_)
	b.running_edges_[edge] = start_time_millis

	b.status_.BuildEdgeStarted(edge, start_time_millis)

	// Create directories necessary for outputs.
	// XXX: this will block; do we care?
	for _, o := range edge.outputs_ {
		if !MakeDirs(b.disk_interface_, o.Path) {
			*err = fmt.Sprintf("Can't make dir %q", o.Path)
			return false
		}
	}

	// Create response file, if needed
	// XXX: this may also block; do we care?
	rspfile := edge.GetUnescapedRspfile()
	if len(rspfile) != 0 {
		content := edge.GetBinding("rspfile_content")
		if !b.disk_interface_.WriteFile(rspfile, content) {
			*err = fmt.Sprintf("Can't write file %q", rspfile)
			return false
		}
	}

	// start command computing and run it
	if !b.command_runner_.StartCommand(edge) {
		*err = "command '" + edge.EvaluateCommand(len(rspfile) != 0) + "' failed."
		return false
	}
	return true
}

// Update status ninja logs following a command termination.
// @return false if the build can not proceed further due to a fatal error.
func (b *Builder) FinishCommand(result *Result, err *string) bool {
	defer METRIC_RECORD("FinishCommand")()
	edge := result.edge

	// First try to extract dependencies from the result, if any.
	// This must happen first as it filters the command output (we want
	// to filter /showIncludes output, even on compile failure) and
	// extraction itself can fail, which makes the command fail from a
	// build perspective.
	var deps_nodes []*Node
	deps_type := edge.GetBinding("deps")
	deps_prefix := edge.GetBinding("msvc_deps_prefix")
	if deps_type != "" {
		extract_err := ""
		if !b.ExtractDeps(result, deps_type, deps_prefix, &deps_nodes, &extract_err) && result.success() {
			if result.output != "" {
				result.output += "\n"
			}
			result.output += extract_err
			result.status = ExitFailure
		}
	}

	var start_time_millis, end_time_millis int32
	start_time_millis = b.running_edges_[edge]
	end_time_millis = int32(time.Now().UnixMilli() - b.start_time_millis_)
	delete(b.running_edges_, edge)

	b.status_.BuildEdgeFinished(edge, end_time_millis, result.success(), result.output)

	// The rest of this function only applies to successful commands.
	if !result.success() {
		return b.plan_.EdgeFinished(edge, kEdgeFailed, err)
	}
	// Restat the edge outputs
	output_mtime := TimeStamp(0)
	restat := edge.GetBindingBool("restat")
	if !b.config_.dry_run {
		node_cleaned := false

		for _, o := range edge.outputs_ {
			new_mtime := b.disk_interface_.Stat(o.Path, err)
			if new_mtime == -1 {
				return false
			}
			if new_mtime > output_mtime {
				output_mtime = new_mtime
			}
			if o.mtime() == new_mtime && restat {
				// The rule command did not change the output.  Propagate the clean
				// state through the build graph.
				// Note that this also applies to nonexistent outputs (mtime == 0).
				if !b.plan_.CleanNode(&b.scan_, o, err) {
					return false
				}
				node_cleaned = true
			}
		}

		if node_cleaned {
			restat_mtime := TimeStamp(0)
			// If any output was cleaned, find the most recent mtime of any
			// (existing) non-order-only input or the depfile.
			for _, i := range edge.inputs_[:len(edge.inputs_)-int(edge.order_only_deps_)] {
				input_mtime := b.disk_interface_.Stat(i.Path, err)
				if input_mtime == -1 {
					return false
				}
				if input_mtime > restat_mtime {
					restat_mtime = input_mtime
				}
			}

			depfile := edge.GetUnescapedDepfile()
			if restat_mtime != 0 && deps_type == "" && depfile != "" {
				depfile_mtime := b.disk_interface_.Stat(depfile, err)
				if depfile_mtime == -1 {
					return false
				}
				if depfile_mtime > restat_mtime {
					restat_mtime = depfile_mtime
				}
			}

			// The total number of edges in the plan may have changed as a result
			// of a restat.
			b.status_.PlanHasTotalEdges(b.plan_.command_edge_count())

			output_mtime = restat_mtime
		}
	}

	if !b.plan_.EdgeFinished(edge, kEdgeSucceeded, err) {
		return false
	}

	// Delete any left over response file.
	rspfile := edge.GetUnescapedRspfile()
	if rspfile != "" && !g_keep_rsp {
		b.disk_interface_.RemoveFile(rspfile)
	}

	if b.scan_.build_log() != nil {
		if !b.scan_.build_log().RecordCommand(edge, start_time_millis, end_time_millis, output_mtime) {
			*err = "Error writing to build log: " // + err
			return false
		}
	}

	if deps_type != "" && !b.config_.dry_run {
		if len(edge.outputs_) == 0 {
			panic("should have been rejected by parser")
		}
		for _, o := range edge.outputs_ {
			deps_mtime := b.disk_interface_.Stat(o.Path, err)
			if deps_mtime == -1 {
				return false
			}
			if !b.scan_.deps_log().RecordDeps(o, deps_mtime, deps_nodes) {
				*err = "Error writing to deps log: " // + err
				return false
			}
		}
	}

	return true
}

func (b *Builder) ExtractDeps(result *Result, deps_type string, deps_prefix string, deps_nodes *[]*Node, err *string) bool {
	if deps_type == "msvc" {
		parser := NewCLParser()
		output := ""
		if !parser.Parse(result.output, deps_prefix, &output, err) {
			return false
		}
		result.output = output
		for i := range parser.includes_ {
			// ~0 is assuming that with MSVC-parsed headers, it's ok to always make
			// all backslashes (as some of the slashes will certainly be backslashes
			// anyway). This could be fixed if necessary with some additional
			// complexity in IncludesNormalize::Relativize.
			*deps_nodes = append(*deps_nodes, b.state_.GetNode(i, 0xFFFFFFFF))
		}
	} else if deps_type == "gcc" {
		depfile := result.edge.GetUnescapedDepfile()
		if len(depfile) == 0 {
			*err = "edge with deps=gcc but no depfile makes no sense"
			return false
		}

		// Read depfile content.  Treat a missing depfile as empty.
		content := ""
		switch b.disk_interface_.ReadFile(depfile, &content, err) {
		case Okay:
		case NotFound:
			err = nil
		case OtherError:
			return false
		}
		if len(content) == 0 {
			return true
		}

		deps := NewDepfileParser(b.config_.depfile_parser_options)
		// TODO(maruel): Memory copy.
		if !deps.Parse([]byte(content), err) {
			return false
		}

		// XXX check depfile matches expected output.
		//deps_nodes.reserve(deps.ins_.size())
		for _, i := range deps.ins_ {
			*deps_nodes = append(*deps_nodes, b.state_.GetNode(CanonicalizePathBits(i)))
		}

		if !g_keep_depfile {
			if b.disk_interface_.RemoveFile(depfile) < 0 {
				*err = "deleting depfile: TODO\n"
				return false
			}
		}
	} else {
		Fatal("unknown deps type '%s'", deps_type)
	}
	return true
}

// Load the dyndep information provided by the given node.
func (b *Builder) LoadDyndeps(node *Node, err *string) bool {
	b.status_.BuildLoadDyndeps()

	// Load the dyndep information provided by this node.
	ddf := DyndepFile{}
	if !b.scan_.LoadDyndeps(node, ddf, err) {
		return false
	}

	// Update the build plan to account for dyndep modifications to the graph.
	if !b.plan_.DyndepsLoaded(&b.scan_, node, ddf, err) {
		return false
	}

	// New command edges may have been added to the plan.
	b.status_.PlanHasTotalEdges(b.plan_.command_edge_count())

	return true
}
