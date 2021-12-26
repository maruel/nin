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

import "fmt"

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
	if p.wanted_edges_ > 0 && p.command_edges_ > 0 {
		return true
	}
	return false
}

// Number of edges with commands to run.
func (p *Plan) command_edge_count() int {
	return p.command_edges_
}

type EdgeResult bool

const (
	kEdgeFailed    EdgeResult = false
	kEdgeSucceeded EdgeResult = true
)

// Enumerate possible steps we want for an edge.
type Want int

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

/*
func (c *CommandRunner) GetActiveEdges() []*Edge {
	return nil
}
func (c *CommandRunner) Abort() {}
*/

// Options (e.g. verbosity, parallelism) passed to a build.
type BuildConfig struct {
	verbosity        Verbosity
	dry_run          bool
	parallelism      int
	failures_allowed int
	// The maximum load average we must not exceed. A negative value
	// means that we do not have any limit.
	max_load_average       float64
	depfile_parser_options DepfileParserOptions
}

func NewBuildConfig() BuildConfig {
	return BuildConfig{
		verbosity:        NORMAL,
		parallelism:      1,
		failures_allowed: 1,
		max_load_average: -0.,
	}
}

type Verbosity int

const (
	QUIET            Verbosity = iota // No output -- used when testing.
	NO_STATUS_UPDATE                  // just regular output but suppress status update
	NORMAL                            // regular output and status update
	VERBOSE
)

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

type RunningEdgeMap map[*Edge]int64

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
	edge := node.in_edge()
	if edge == nil { // Leaf node.
		if node.dirty() {
			referenced := ""
			if dependent != nil {
				referenced = ", needed by '" + dependent.path() + "',"
			}
			*err = "'" + node.path() + "'" + referenced + " missing and no known rule to make it"
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
	if node.dirty() && want == kWantNothing {
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
	//log.Printf("ScheduleWork()")
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
	for _, oe := range node.out_edges() {
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
	node.set_dirty(false)

	for _, oe := range node.out_edges() {
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
		panic("TODO")
		/*
		   vector<Node*>::iterator begin = (*oe).inputs_.begin(), end = (*oe).inputs_.end() - (*oe).order_only_deps_
		   if find_if(begin, end, MEM_FN(&Node::dirty)) == end {
		     // Recompute most_recent_input.
		     most_recent_input := nil
		     for i := begin; i != end; i++ {
		       if !most_recent_input || (*i).mtime() > most_recent_input.mtime() {
		         most_recent_input = *i
		       }
		     }

		     // Now, this edge is dirty if any of the outputs are dirty.
		     // If the edge isn't dirty, clean the outputs and mark the edge as not
		     // wanted.
		     outputs_dirty := false
		     if !scan.RecomputeOutputsDirty(*oe, most_recent_input, &outputs_dirty, err) {
		       return false
		     }
		     if !outputs_dirty {
		       for o := (*oe).outputs_.begin(); o != (*oe).outputs_.end(); o++ {
		         if !CleanNode(scan, *o, err) {
		           return false
		         }
		       }

		       want_e.second = kWantNothing
		       p.wanted_edges_--
		       if !(*oe).is_phony() {
		         p.command_edges_--
		       }
		     }
		   }
		*/
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

	panic("TODO")
	/*
	  // Find edges in the the build plan for which we have new dyndep info.
	  var dyndep_roots []DyndepFile::const_iterator
	  for oe := ddf.begin(); oe != ddf.end(); oe++ {
	    edge := oe.first

	    // If the edge outputs are ready we do not need to consider it here.
	    if edge.outputs_ready() {
	      continue
	    }

	    want_e := p.want_.find(edge)

	    // If the edge has not been encountered before then nothing already in the
	    // plan depends on it so we do not need to consider the edge yet either.
	    if want_e == p.want_.end() {
	      continue
	    }

	    // This edge is already in the plan so queue it for the walk.
	    dyndep_roots.push_back(oe)
	  }

	  // Walk dyndep-discovered portion of the graph to add it to the build plan.
	  var dyndep_walk map[*Edge]struct{}
	  for oei := dyndep_roots.begin(); oei != dyndep_roots.end(); oei++ {
	    oe := *oei
	    for i := oe.second.implicit_inputs_.begin(); i != oe.second.implicit_inputs_.end(); i++ {
	      if !AddSubTarget(*i, oe.first.outputs_[0], err, &dyndep_walk) && !err.empty() {
	        return false
	      }
	    }
	  }

	  // Add out edges from this node that are in the plan (just as
	  // NodeFinished would have without taking the dyndep code path).
	  for oe := node.out_edges().begin(); oe != node.out_edges().end(); oe++ {
	    want_e := p.want_.find(*oe)
	    if want_e == p.want_.end() {
	      continue
	    }
	    dyndep_walk.insert(want_e.first)
	  }

	  // See if any encountered edges are now ready.
	  for wi := dyndep_walk.begin(); wi != dyndep_walk.end(); wi++ {
	    want_e := p.want_.find(*wi)
	    if want_e == p.want_.end() {
	      continue
	    }
	    if !EdgeMaybeReady(want_e, err) {
	      return false
	    }
	  }
	*/
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
		if !scan.RecomputeDirty(n, nil, err) {
			return false
		}
		if !n.dirty() {
			continue
		}

		// This edge was encountered before.  However, we may not have wanted to
		// build it if the outputs were not known to be dirty.  With dyndep
		// information an output is now known to be dirty, so we want the edge.
		edge := n.in_edge()
		if edge == nil || !edge.outputs_ready() {
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
	for _, edge := range node.out_edges() {
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
			printf("want ")
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

type RealCommandRunner struct {
	config_          *BuildConfig
	subprocs_        SubprocessSet
	subproc_to_edge_ map[*Subprocess]*Edge
}

func NewRealCommandRunner(config *BuildConfig) *RealCommandRunner {
	return &RealCommandRunner{
		config_: config,
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
	panic("TODO")
	//r.subprocs_.Clear()
}

func (r *RealCommandRunner) CanRunMore() bool {
	panic("TODO")
	/*
		subproc_number := len(r.subprocs_.running_) + len(r.subprocs_.finished_)
		return subproc_number < r.config_.parallelism && ((r.subprocs_.running_.empty() || r.config_.max_load_average <= 0.) || r.GetLoadAverage() < r.config_.max_load_average)
	*/
	return false
}

func (r *RealCommandRunner) StartCommand(edge *Edge) bool {
	panic("TODO")
	/*
		command := edge.EvaluateCommand()
		subproc := r.subprocs_.Add(command, edge.use_console())
		if subproc == nil {
			return false
		}
		r.subproc_to_edge_.insert(make_pair(subproc, edge))
	*/
	return true
}

func (r *RealCommandRunner) WaitForCommand(result *Result) bool {
	panic("TODO")
	/*
		var subproc *Subprocess
		for subproc = r.subprocs_.NextFinished(); subproc == nil; subproc = r.subprocs_.NextFinished() {
			interrupted := r.subprocs_.DoWork()
			if interrupted != nil {
				return false
			}
		}

		result.status = subproc.Finish()
		result.output = subproc.GetOutput()

		e := r.subproc_to_edge_.find(subproc)
		result.edge = e.second
		r.subproc_to_edge_.erase(e)

		delete(subproc)
	*/
	return true
}

func NewBuilder(state *State, config *BuildConfig, build_log *BuildLog, deps_log *DepsLog, disk_interface DiskInterface, status Status, start_time_millis int64) *Builder {
	b := &Builder{
		state_:             state,
		config_:            config,
		status_:            status,
		start_time_millis_: start_time_millis,
		disk_interface_:    disk_interface,
	}
	b.plan_ = NewPlan(b)
	b.scan_ = NewDependencyScan(state, build_log, deps_log, disk_interface, &b.config_.depfile_parser_options)
	return b
}

func (b *Builder) Destructor() {
	b.Cleanup()
}

// Clean up after interrupted commands by deleting output files.
func (b *Builder) Cleanup() {
	panic("TODO")
	/*
		if b.command_runner_.get() {
			active_edges := b.command_runner_.GetActiveEdges()
			b.command_runner_.Abort()

			for e := active_edges.begin(); e != active_edges.end(); e++ {
				depfile := (*e).GetUnescapedDepfile()
				for o := (*e).outputs_.begin(); o != (*e).outputs_.end(); o++ {
					// Only delete this output if it was actually modified.  This is
					// important for things like the generator where we don't want to
					// delete the manifest file if we can avoid it.  But if the rule
					// uses a depfile, always delete.  (Consider the case where we
					// need to rebuild an output because of a modified header file
					// mentioned in a depfile, and the command touches its depfile
					// but is interrupted before it touches its output file.)
					err := ""
					new_mtime := b.disk_interface_.Stat((*o).path(), &err)
					if new_mtime == -1 { // Log and ignore Stat() errors.
						b.status_.Error("%s", err)
					}
					if !depfile.empty() || (*o).mtime() != new_mtime {
						b.disk_interface_.RemoveFile((*o).path())
					}
				}
				if len(depfile) != 0 {
					b.disk_interface_.RemoveFile(depfile)
				}
			}
		}
	*/
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
	if !b.scan_.RecomputeDirty(target, nil, err) {
		return false
	}

	if in_edge := target.in_edge(); in_edge != nil {
		if in_edge.outputs_ready() {
			return true // Nothing to do.
		}
	}

	if !b.plan_.AddTarget(target, err) {
		return false
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
					b.scan_.build_log().Close()
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
	METRIC_RECORD("StartEdge")
	if edge.is_phony() {
		return true
	}
	panic("TODO")
	/*
		start_time_millis := GetTimeMillis() - b.start_time_millis_
		b.running_edges_[edge] = start_time_millis

		b.status_.BuildEdgeStarted(edge, start_time_millis)

		// Create directories necessary for outputs.
		// XXX: this will block; do we care?
		for _, o := range edge.outputs_ {
			if !b.disk_interface_.MakeDirs(o.path()) {
				return false
			}
		}

		// Create response file, if needed
		// XXX: this may also block; do we care?
		rspfile := edge.GetUnescapedRspfile()
		if len(rspfile) != 0 {
			content := edge.GetBinding("rspfile_content")
			if !b.disk_interface_.WriteFile(rspfile, content) {
				return false
			}
		}

		// start command computing and run it
		if !b.command_runner_.StartCommand(edge) {
			err.assign("command '" + edge.EvaluateCommand() + "' failed.")
			return false
		}
	*/
	return true
}

// Update status ninja logs following a command termination.
// @return false if the build can not proceed further due to a fatal error.
func (b *Builder) FinishCommand(result *Result, err *string) bool {
	METRIC_RECORD("FinishCommand")
	panic("TODO")
	/*
		edge := result.edge

		// First try to extract dependencies from the result, if any.
		// This must happen first as it filters the command output (we want
		// to filter /showIncludes output, even on compile failure) and
		// extraction itself can fail, which makes the command fail from a
		// build perspective.
		var deps_nodes []*Node
		deps_type := edge.GetBinding("deps")
		const string deps_prefix = edge.GetBinding("msvc_deps_prefix")
		if !deps_type.empty() {
			extract_err := ""
			if !ExtractDeps(result, deps_type, deps_prefix, &deps_nodes, &extract_err) && result.success() {
				if !result.output.empty() {
					result.output.append("\n")
				}
				result.output.append(extract_err)
				result.status = ExitFailure
			}
		}

		var start_time_millis, end_time_millis int64
		it := b.running_edges_.find(edge)
		start_time_millis = it.second
		end_time_millis = GetTimeMillis() - b.start_time_millis_
		b.running_edges_.erase(it)

		b.status_.BuildEdgeFinished(edge, end_time_millis, result.success(), result.output)

		// The rest of this function only applies to successful commands.
		if !result.success() {
			return b.plan_.EdgeFinished(edge, kEdgeFailed, err)
		}

		// Restat the edge outputs
		output_mtime := 0
		restat := edge.GetBindingBool("restat")
		if !b.config_.dry_run {
			node_cleaned := false

			for o := edge.outputs_.begin(); o != edge.outputs_.end(); o++ {
				new_mtime := b.disk_interface_.Stat((*o).path(), err)
				if new_mtime == -1 {
					return false
				}
				if new_mtime > output_mtime {
					output_mtime = new_mtime
				}
				if (*o).mtime() == new_mtime && restat {
					// The rule command did not change the output.  Propagate the clean
					// state through the build graph.
					// Note that this also applies to nonexistent outputs (mtime == 0).
					if !b.plan_.CleanNode(&b.scan_, *o, err) {
						return false
					}
					node_cleaned = true
				}
			}

			if node_cleaned {
				restat_mtime := 0
				// If any output was cleaned, find the most recent mtime of any
				// (existing) non-order-only input or the depfile.
				for i := edge.inputs_.begin(); i != edge.inputs_.end()-edge.order_only_deps_; i++ {
					input_mtime := b.disk_interface_.Stat((*i).path(), err)
					if input_mtime == -1 {
						return false
					}
					if input_mtime > restat_mtime {
						restat_mtime = input_mtime
					}
				}

				depfile := edge.GetUnescapedDepfile()
				if restat_mtime != 0 && deps_type.empty() && !depfile.empty() {
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
		if !rspfile.empty() && !g_keep_rsp {
			b.disk_interface_.RemoveFile(rspfile)
		}

		if b.scan_.build_log() {
			if !b.scan_.build_log().RecordCommand(edge, start_time_millis, end_time_millis, output_mtime) {
				*err = string("Error writing to build log: ") + strerror(errno)
				return false
			}
		}

		if !deps_type.empty() && !b.config_.dry_run {
			if edge.outputs_.empty() && "should have been rejected by parser" {
				panic("M-A")
			}
			for o := edge.outputs_.begin(); o != edge.outputs_.end(); o++ {
				deps_mtime := b.disk_interface_.Stat((*o).path(), err)
				if deps_mtime == -1 {
					return false
				}
				if !b.scan_.deps_log().RecordDeps(*o, deps_mtime, deps_nodes) {
					*err = string("Error writing to deps log: ") + strerror(errno)
					return false
				}
			}
		}
	*/
	return true
}

func (b *Builder) ExtractDeps(result *Result, deps_type string, deps_prefix string, deps_nodes []*Node, err *string) bool {
	panic("TODO")
	/*
	   if deps_type == "msvc" {
	     var parser CLParser
	     output := ""
	     if !parser.Parse(result.output, deps_prefix, &output, err) {
	       return false
	     }
	     result.output = output
	     for i := parser.includes_.begin(); i != parser.includes_.end(); i++ {
	       // ~0 is assuming that with MSVC-parsed headers, it's ok to always make
	       // all backslashes (as some of the slashes will certainly be backslashes
	       // anyway). This could be fixed if necessary with some additional
	       // complexity in IncludesNormalize::Relativize.
	       *deps_nodes = append(*deps_nodes, b.state_.GetNode(*i, 0xFFFFFFFF))
	     }
	   } else if deps_type == "gcc" {
	     depfile := result.edge.GetUnescapedDepfile()
	     if len(depfile) == 0 {
	       *err = string("edge with deps=gcc but no depfile makes no sense")
	       return false
	     }

	     // Read depfile content.  Treat a missing depfile as empty.
	     content := ""
	     switch (b.disk_interface_.ReadFile(depfile, &content, err)) {
	     case Okay:
	       break
	     case NotFound:
	       err = nil
	       break
	     case OtherError:
	       return false
	     }
	     if len(content) == 0 {
	       return true
	     }

	     DepfileParser deps(b.config_.depfile_parser_options)
	     if !deps.Parse(&content, err) {
	       return false
	     }

	     // XXX check depfile matches expected output.
	     deps_nodes.reserve(deps.ins_.size())
	     for i := deps.ins_.begin(); i != deps.ins_.end(); i++ {
	       var slash_bits uint64
	       CanonicalizePath(const_cast<char*>(i.str_), &i.len_, &slash_bits)
	       deps_nodes.push_back(b.state_.GetNode(*i, slash_bits))
	     }

	     if !g_keep_depfile {
	       if b.disk_interface_.RemoveFile(depfile) < 0 {
	         *err = string("deleting depfile: ") + strerror(errno) + string("\n")
	         return false
	       }
	     }
	   } else {
	     Fatal("unknown deps type '%s'", deps_type)
	   }
	*/
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
