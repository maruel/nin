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
	EdgeFailed    EdgeResult = false
	EdgeSucceeded EdgeResult = true
)

// Enumerate possible steps we want for an edge.
type Want int32

const (
	// We do not want to build the edge, but we might want to build one of
	// its dependents.
	WantNothing Want = iota
	// We want to build the edge, but have not yet scheduled it.
	WantToStart
	// We want to build the edge, have scheduled it, and are waiting
	// for it to complete.
	WantToFinish
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

// Result is the result of waiting for a command.
type Result struct {
	Edge     *Edge
	ExitCode ExitStatus
	Output   string
}

// Options (e.g. verbosity, parallelism) passed to a build.
type BuildConfig struct {
	verbosity       Verbosity
	dryRun          bool
	parallelism     int
	failuresAllowed int
	// The maximum load average we must not exceed. A negative or zero value
	// means that we do not have any limit.
	maxLoadAverage       float64
	depfileParserOptions DepfileParserOptions
}

func NewBuildConfig() BuildConfig {
	return BuildConfig{
		verbosity:       Normal,
		parallelism:     1,
		failuresAllowed: 1,
	}
}

type Verbosity int32

const (
	Quiet          Verbosity = iota // No output -- used when testing.
	NoStatusUpdate                  // just regular output but suppress status update
	Normal                          // regular output and status update
	Verbose
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

	result.ExitCode = ExitSuccess
	result.Edge = d.finished_[len(d.finished_)-1]
	d.finished_ = d.finished_[:len(d.finished_)-1]
	return true
}

func (d *DryRunCommandRunner) GetActiveEdges() []*Edge {
	return nil
}

func (d *DryRunCommandRunner) Abort() {
}

type RealCommandRunner struct {
	config_        *BuildConfig
	subprocs_      *SubprocessSet
	subprocToEdge_ map[*Subprocess]*Edge
}

func NewRealCommandRunner(config *BuildConfig) *RealCommandRunner {
	return &RealCommandRunner{
		config_:        config,
		subprocs_:      NewSubprocessSet(),
		subprocToEdge_: map[*Subprocess]*Edge{},
	}
}

func (r *RealCommandRunner) GetActiveEdges() []*Edge {
	var edges []*Edge
	for _, e := range r.subprocToEdge_ {
		edges = append(edges, e)
	}
	return edges
}

func (r *RealCommandRunner) Abort() {
	r.subprocs_.Clear()
}

func (r *RealCommandRunner) CanRunMore() bool {
	subprocNumber := r.subprocs_.Running() + r.subprocs_.Finished()
	more := subprocNumber < r.config_.parallelism
	load := r.subprocs_.Running() == 0 || r.config_.maxLoadAverage <= 0. || getLoadAverage() < r.config_.maxLoadAverage
	return more && load
}

func (r *RealCommandRunner) StartCommand(edge *Edge) bool {
	command := edge.EvaluateCommand(false)
	subproc := r.subprocs_.Add(command, edge.Pool == ConsolePool)
	if subproc == nil {
		return false
	}
	r.subprocToEdge_[subproc] = edge
	return true
}

func (r *RealCommandRunner) WaitForCommand(result *Result) bool {
	var subproc *Subprocess
	for {
		subproc = r.subprocs_.NextFinished()
		if subproc != nil {
			break
		}
		if r.subprocs_.DoWork() {
			return false
		}
	}

	result.ExitCode = subproc.Finish()
	result.Output = subproc.GetOutput()

	e := r.subprocToEdge_[subproc]
	result.Edge = e
	delete(r.subprocToEdge_, subproc)
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
	commandEdges_ int

	// Total remaining number of wanted edges.
	wantedEdges_ int
}

// Returns true if there's more work to be done.
func (p *Plan) moreToDo() bool {
	return p.wantedEdges_ > 0 && p.commandEdges_ > 0
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
	p.commandEdges_ = 0
	p.wantedEdges_ = 0
	p.want_ = map[*Edge]Want{}
	p.ready_ = NewEdgeSet()
}

// Add a target to our plan (including all its dependencies).
// Returns false if we don't need to build this target; may
// fill in |err| with an error message if there's a problem.
func (p *Plan) AddTarget(target *Node, err *string) bool {
	return p.AddSubTarget(target, nil, err, nil)
}

func (p *Plan) AddSubTarget(node *Node, dependent *Node, err *string, dyndepWalk map[*Edge]struct{}) bool {
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

	if edge.OutputsReady {
		return false // Don't need to do anything.
	}

	// If an entry in want_ does not already exist for edge, create an entry which
	// maps to WantNothing, indicating that we do not want to build this entry itself.
	want, ok := p.want_[edge]
	if !ok {
		p.want_[edge] = WantNothing
	} else if len(dyndepWalk) != 0 && want == WantToFinish {
		return false // Don't need to do anything with already-scheduled edge.
	}

	// If we do need to build edge and we haven't already marked it as wanted,
	// mark it now.
	if node.Dirty && want == WantNothing {
		want = WantToStart
		p.want_[edge] = want
		p.EdgeWanted(edge)
		if len(dyndepWalk) == 0 && edge.AllInputsReady() {
			p.ScheduleWork(edge, want)
		}
	}

	if len(dyndepWalk) != 0 {
		dyndepWalk[edge] = struct{}{}
	}

	if ok {
		return true // We've already processed the inputs.
	}

	for _, i := range edge.Inputs {
		if !p.AddSubTarget(i, node, err, dyndepWalk) && *err != "" {
			return false
		}
	}
	return true
}

func (p *Plan) EdgeWanted(edge *Edge) {
	p.wantedEdges_++
	if edge.Rule != PhonyRule {
		p.commandEdges_++
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
	if want == WantToFinish {
		// This edge has already been scheduled.  We can get here again if an edge
		// and one of its dependencies share an order-only input, or if a node
		// duplicates an out edge (see https://github.com/ninja-build/ninja/pull/519).
		// Avoid scheduling the work again.
		return
	}
	if want != WantToStart {
		panic("M-A")
	}
	p.want_[edge] = WantToFinish

	pool := edge.Pool
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
	directlyWanted := want != WantNothing

	// See if this job frees up any delayed jobs.
	if directlyWanted {
		edge.Pool.EdgeFinished(edge)
	}
	edge.Pool.RetrieveReadyEdges(p.ready_)

	// The rest of this function only applies to successful commands.
	if result != EdgeSucceeded {
		return true
	}

	if directlyWanted {
		p.wantedEdges_--
	}
	delete(p.want_, edge)
	edge.OutputsReady = true

	// Check off any nodes we were waiting for with this edge.
	for _, o := range edge.Outputs {
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
	if node.DyndepPending {
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
		if want != WantNothing {
			p.ScheduleWork(edge, want)
		} else {
			// We do not need to build this edge, but we might need to build one of
			// its dependents.
			if !p.EdgeFinished(edge, EdgeSucceeded, err) {
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
		if !ok || want == WantNothing {
			continue
		}

		// Don't attempt to clean an edge if it failed to load deps.
		if oe.DepsMissing {
			continue
		}

		// If all non-order-only inputs for this edge are now clean,
		// we might have changed the dirty state of the outputs.
		end := len(oe.Inputs) - int(oe.OrderOnlyDeps)
		found := false
		for i := 0; i < end; i++ {
			if oe.Inputs[i].Dirty {
				found = true
				break
			}
		}
		if !found {
			// Recompute mostRecentInput.
			mostRecentInput := -1
			for i := 0; i != end; i++ {
				if mostRecentInput == -1 || oe.Inputs[i].MTime > oe.Inputs[mostRecentInput].MTime {
					mostRecentInput = i
				}
			}

			// Now, this edge is dirty if any of the outputs are dirty.
			// If the edge isn't dirty, clean the outputs and mark the edge as not
			// wanted.
			outputsDirty := false
			if !scan.RecomputeOutputsDirty(oe, oe.Inputs[mostRecentInput], &outputsDirty, err) {
				return false
			}
			if !outputsDirty {
				for _, o := range oe.Outputs {
					if !p.CleanNode(scan, o, err) {
						return false
					}
				}

				p.want_[oe] = WantNothing
				p.wantedEdges_--
				if oe.Rule != PhonyRule {
					p.commandEdges_--
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

	// We loaded dyndep information for those outEdges of the dyndep node that
	// specify the node in a dyndep binding, but they may not be in the plan.
	// Starting with those already in the plan, walk newly-reachable portion
	// of the graph through the dyndep-discovered dependencies.

	// Find edges in the the build plan for which we have new dyndep info.
	var dyndepRoots []*Edge
	for edge := range ddf {
		// If the edge outputs are ready we do not need to consider it here.
		if edge.OutputsReady {
			continue
		}

		// If the edge has not been encountered before then nothing already in the
		// plan depends on it so we do not need to consider the edge yet either.
		if _, ok := p.want_[edge]; !ok {
			continue
		}

		// This edge is already in the plan so queue it for the walk.
		dyndepRoots = append(dyndepRoots, edge)
	}

	// Walk dyndep-discovered portion of the graph to add it to the build plan.
	dyndepWalk := map[*Edge]struct{}{}
	for _, oe := range dyndepRoots {
		for _, i := range ddf[oe].implicitInputs_ {
			if !p.AddSubTarget(i, oe.Outputs[0], err, dyndepWalk) && *err != "" {
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
		dyndepWalk[oe] = struct{}{}
	}

	// See if any encountered edges are now ready.
	for wi := range dyndepWalk {
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
		var validationNodes []*Node
		if !scan.RecomputeDirty(n, &validationNodes, err) {
			return false
		}

		// Add any validation nodes found during RecomputeDirty as new top level
		// targets.
		for _, v := range validationNodes {
			if inEdge := v.InEdge; inEdge != nil {
				if !inEdge.OutputsReady && !p.AddTarget(v, err) {
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
		if edge == nil || edge.OutputsReady {
			panic("M-A")
		}
		wantE, ok := p.want_[edge]
		if !ok {
			panic("M-A")
		}
		if wantE == WantNothing {
			p.want_[edge] = WantToStart
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

		if edge.Mark != VisitNone {
			edge.Mark = VisitNone
			for _, o := range edge.Outputs {
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
		if w != WantNothing {
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
	state_         *State
	config_        *BuildConfig
	plan_          Plan
	commandRunner_ CommandRunner
	status_        Status

	// Map of running edge to time the edge started running.
	runningEdges_ RunningEdgeMap

	// Time the build started.
	startTimeMillis_ int64

	diskInterface_ DiskInterface
	scan_          DependencyScan
}

// Used for tests.
func (b *Builder) SetBuildLog(log *BuildLog) {
	b.scan_.setBuildLog(log)
}

func NewBuilder(state *State, config *BuildConfig, buildLog *BuildLog, depsLog *DepsLog, diskInterface DiskInterface, status Status, startTimeMillis int64) *Builder {
	b := &Builder{
		state_:           state,
		config_:          config,
		status_:          status,
		runningEdges_:    RunningEdgeMap{},
		startTimeMillis_: startTimeMillis,
		diskInterface_:   diskInterface,
	}
	b.plan_ = NewPlan(b)
	b.scan_ = NewDependencyScan(state, buildLog, depsLog, diskInterface, &b.config_.depfileParserOptions)
	return b
}

// TODO(maruel): Make sure it's always called where important.
func (b *Builder) Destructor() {
	b.Cleanup()
}

// Clean up after interrupted commands by deleting output files.
func (b *Builder) Cleanup() {
	if b.commandRunner_ != nil {
		activeEdges := b.commandRunner_.GetActiveEdges()
		b.commandRunner_.Abort()

		for _, e := range activeEdges {
			depfile := e.GetUnescapedDepfile()
			for _, o := range e.Outputs {
				// Only delete this output if it was actually modified.  This is
				// important for things like the generator where we don't want to
				// delete the manifest file if we can avoid it.  But if the rule
				// uses a depfile, always delete.  (Consider the case where we
				// need to rebuild an output because of a modified header file
				// mentioned in a depfile, and the command touches its depfile
				// but is interrupted before it touches its output file.)
				err := ""
				newMtime := b.diskInterface_.Stat(o.Path, &err)
				if newMtime == -1 { // Log and ignore Stat() errors.
					b.status_.Error("%s", err)
				}
				if depfile != "" || o.MTime != newMtime {
					b.diskInterface_.RemoveFile(o.Path)
				}
			}
			if len(depfile) != 0 {
				b.diskInterface_.RemoveFile(depfile)
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
	var validationNodes []*Node
	if !b.scan_.RecomputeDirty(target, &validationNodes, err) {
		return false
	}

	inEdge := target.InEdge
	if inEdge == nil || !inEdge.OutputsReady {
		if !b.plan_.AddTarget(target, err) {
			return false
		}
	}

	// Also add any validation nodes found during RecomputeDirty as top level
	// targets.
	for _, n := range validationNodes {
		if validationInEdge := n.InEdge; validationInEdge != nil {
			if !validationInEdge.OutputsReady && !b.plan_.AddTarget(n, err) {
				return false
			}
		}
	}

	return true
}

// Returns true if the build targets are already up to date.
func (b *Builder) AlreadyUpToDate() bool {
	return !b.plan_.moreToDo()
}

// Run the build.  Returns false on error.
// It is an error to call this function when AlreadyUpToDate() is true.
func (b *Builder) Build(err *string) bool {
	if b.AlreadyUpToDate() {
		panic("M-A")
	}

	b.status_.PlanHasTotalEdges(b.plan_.commandEdges_)
	pendingCommands := 0
	failuresAllowed := b.config_.failuresAllowed

	// Set up the command runner if we haven't done so already.
	if b.commandRunner_ == nil {
		if b.config_.dryRun {
			b.commandRunner_ = &DryRunCommandRunner{}
		} else {
			b.commandRunner_ = NewRealCommandRunner(b.config_)
		}
	}

	// We are about to start the build process.
	b.status_.BuildStarted()

	// This main loop runs the entire build process.
	// It is structured like this:
	// First, we attempt to start as many commands as allowed by the
	// command runner.
	// Second, we attempt to wait for / reap the next finished command.
	for b.plan_.moreToDo() {
		// See if we can start any more commands.
		if failuresAllowed != 0 && b.commandRunner_.CanRunMore() {
			if edge := b.plan_.FindWork(); edge != nil {
				if edge.GetBinding("generator") != "" {
					_ = b.scan_.buildLog().Close()
				}

				if !b.StartEdge(edge, err) {
					b.Cleanup()
					b.status_.BuildFinished()
					return false
				}

				if edge.Rule == PhonyRule {
					if !b.plan_.EdgeFinished(edge, EdgeSucceeded, err) {
						b.Cleanup()
						b.status_.BuildFinished()
						return false
					}
				} else {
					pendingCommands++
				}

				// We made some progress; go back to the main loop.
				continue
			}
		}

		// See if we can reap any finished commands.
		if pendingCommands != 0 {
			var result Result
			if !b.commandRunner_.WaitForCommand(&result) || result.ExitCode == ExitInterrupted {
				b.Cleanup()
				b.status_.BuildFinished()
				*err = "interrupted by user"
				return false
			}

			pendingCommands--
			if !b.FinishCommand(&result, err) {
				b.Cleanup()
				b.status_.BuildFinished()
				return false
			}

			if result.ExitCode != ExitSuccess {
				if failuresAllowed != 0 {
					failuresAllowed--
				}
			}

			// We made some progress; start the main loop over.
			continue
		}

		// If we get here, we cannot make any more progress.
		b.status_.BuildFinished()
		if failuresAllowed == 0 {
			if b.config_.failuresAllowed > 1 {
				*err = "subcommands failed"
			} else {
				*err = "subcommand failed"
			}
		} else if failuresAllowed < b.config_.failuresAllowed {
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
	defer MetricRecord("StartEdge")()
	if edge.Rule == PhonyRule {
		return true
	}
	startTimeMillis := int32(time.Now().UnixMilli() - b.startTimeMillis_)
	b.runningEdges_[edge] = startTimeMillis

	b.status_.BuildEdgeStarted(edge, startTimeMillis)

	// Create directories necessary for outputs.
	// XXX: this will block; do we care?
	for _, o := range edge.Outputs {
		if !MakeDirs(b.diskInterface_, o.Path) {
			*err = fmt.Sprintf("Can't make dir %q", o.Path)
			return false
		}
	}

	// Create response file, if needed
	// XXX: this may also block; do we care?
	rspfile := edge.GetUnescapedRspfile()
	if len(rspfile) != 0 {
		content := edge.GetBinding("rspfile_content")
		if !b.diskInterface_.WriteFile(rspfile, content) {
			*err = fmt.Sprintf("Can't write file %q", rspfile)
			return false
		}
	}

	// start command computing and run it
	if !b.commandRunner_.StartCommand(edge) {
		*err = "command '" + edge.EvaluateCommand(len(rspfile) != 0) + "' failed."
		return false
	}
	return true
}

// Update status ninja logs following a command termination.
// @return false if the build can not proceed further due to a fatal error.
func (b *Builder) FinishCommand(result *Result, err *string) bool {
	defer MetricRecord("FinishCommand")()
	edge := result.Edge

	// First try to extract dependencies from the result, if any.
	// This must happen first as it filters the command output (we want
	// to filter /showIncludes output, even on compile failure) and
	// extraction itself can fail, which makes the command fail from a
	// build perspective.
	var depsNodes []*Node
	depsType := edge.GetBinding("deps")
	depsPrefix := edge.GetBinding("msvc_deps_prefix")
	if depsType != "" {
		extractErr := ""
		if !b.ExtractDeps(result, depsType, depsPrefix, &depsNodes, &extractErr) && result.ExitCode == ExitSuccess {
			if result.Output != "" {
				result.Output += "\n"
			}
			result.Output += extractErr
			result.ExitCode = ExitFailure
		}
	}

	var startTimeMillis, endTimeMillis int32
	startTimeMillis = b.runningEdges_[edge]
	endTimeMillis = int32(time.Now().UnixMilli() - b.startTimeMillis_)
	delete(b.runningEdges_, edge)

	b.status_.BuildEdgeFinished(edge, endTimeMillis, result.ExitCode == ExitSuccess, result.Output)

	// The rest of this function only applies to successful commands.
	if result.ExitCode != ExitSuccess {
		return b.plan_.EdgeFinished(edge, EdgeFailed, err)
	}
	// Restat the edge outputs
	outputMtime := TimeStamp(0)
	restat := edge.GetBinding("restat") != ""
	if !b.config_.dryRun {
		nodeCleaned := false

		for _, o := range edge.Outputs {
			newMtime := b.diskInterface_.Stat(o.Path, err)
			if newMtime == -1 {
				return false
			}
			if newMtime > outputMtime {
				outputMtime = newMtime
			}
			if o.MTime == newMtime && restat {
				// The rule command did not change the output.  Propagate the clean
				// state through the build graph.
				// Note that this also applies to nonexistent outputs (mtime == 0).
				if !b.plan_.CleanNode(&b.scan_, o, err) {
					return false
				}
				nodeCleaned = true
			}
		}

		if nodeCleaned {
			restatMtime := TimeStamp(0)
			// If any output was cleaned, find the most recent mtime of any
			// (existing) non-order-only input or the depfile.
			for _, i := range edge.Inputs[:len(edge.Inputs)-int(edge.OrderOnlyDeps)] {
				inputMtime := b.diskInterface_.Stat(i.Path, err)
				if inputMtime == -1 {
					return false
				}
				if inputMtime > restatMtime {
					restatMtime = inputMtime
				}
			}

			depfile := edge.GetUnescapedDepfile()
			if restatMtime != 0 && depsType == "" && depfile != "" {
				depfileMtime := b.diskInterface_.Stat(depfile, err)
				if depfileMtime == -1 {
					return false
				}
				if depfileMtime > restatMtime {
					restatMtime = depfileMtime
				}
			}

			// The total number of edges in the plan may have changed as a result
			// of a restat.
			b.status_.PlanHasTotalEdges(b.plan_.commandEdges_)

			outputMtime = restatMtime
		}
	}

	if !b.plan_.EdgeFinished(edge, EdgeSucceeded, err) {
		return false
	}

	// Delete any left over response file.
	rspfile := edge.GetUnescapedRspfile()
	if rspfile != "" && !gKeepRsp {
		b.diskInterface_.RemoveFile(rspfile)
	}

	if b.scan_.buildLog() != nil {
		if !b.scan_.buildLog().RecordCommand(edge, startTimeMillis, endTimeMillis, outputMtime) {
			*err = "Error writing to build log: " // + err
			return false
		}
	}

	if depsType != "" && !b.config_.dryRun {
		if len(edge.Outputs) == 0 {
			panic("should have been rejected by parser")
		}
		for _, o := range edge.Outputs {
			depsMtime := b.diskInterface_.Stat(o.Path, err)
			if depsMtime == -1 {
				return false
			}
			if !b.scan_.depsLog().RecordDeps(o, depsMtime, depsNodes) {
				*err = "Error writing to deps log: " // + err
				return false
			}
		}
	}

	return true
}

func (b *Builder) ExtractDeps(result *Result, depsType string, depsPrefix string, depsNodes *[]*Node, err *string) bool {
	if depsType == "msvc" {
		parser := NewCLParser()
		output := ""
		if !parser.Parse(result.Output, depsPrefix, &output, err) {
			return false
		}
		result.Output = output
		for i := range parser.includes_ {
			// ~0 is assuming that with MSVC-parsed headers, it's ok to always make
			// all backslashes (as some of the slashes will certainly be backslashes
			// anyway). This could be fixed if necessary with some additional
			// complexity in IncludesNormalize.relativize.
			*depsNodes = append(*depsNodes, b.state_.GetNode(i, 0xFFFFFFFF))
		}
	} else if depsType == "gcc" {
		depfile := result.Edge.GetUnescapedDepfile()
		if len(depfile) == 0 {
			*err = "edge with deps=gcc but no depfile makes no sense"
			return false
		}

		// Read depfile content.  Treat a missing depfile as empty.
		content := ""
		switch b.diskInterface_.ReadFile(depfile, &content, err) {
		case Okay:
		case NotFound:
			err = nil
		case OtherError:
			return false
		}
		if len(content) == 0 {
			return true
		}

		deps := NewDepfileParser(b.config_.depfileParserOptions)
		// TODO(maruel): Memory copy.
		if !deps.Parse([]byte(content), err) {
			return false
		}

		// XXX check depfile matches expected output.
		//depsNodes.reserve(deps.ins_.size())
		for _, i := range deps.ins_ {
			*depsNodes = append(*depsNodes, b.state_.GetNode(CanonicalizePathBits(i)))
		}

		if !gKeepDepfile {
			if b.diskInterface_.RemoveFile(depfile) < 0 {
				*err = "deleting depfile: TODO\n"
				return false
			}
		}
	} else {
		fatalf("unknown deps type '%s'", depsType)
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
	b.status_.PlanHasTotalEdges(b.plan_.commandEdges_)

	return true
}
