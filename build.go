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
	"errors"
	"fmt"
	"os"
	"time"
)

type edgeResult bool

const (
	edgeFailed    edgeResult = false
	edgeSucceeded edgeResult = true
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

// commandRunner is an interface that wraps running the build
// subcommands.  This allows tests to abstract out running commands.
// RealCommandRunner is an implementation that actually runs commands.
type commandRunner interface {
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

// TODO(maruel): The build per se shouldn't have verbosity as a flag. It should
// be composed.

// BuildConfig are the options (e.g. verbosity, parallelism) passed to a build.
type BuildConfig struct {
	Verbosity       Verbosity
	DryRun          bool
	Parallelism     int
	FailuresAllowed int
	// The maximum load average we must not exceed. A negative or zero value
	// means that we do not have any limit.
	MaxLoadAvg float64
}

func NewBuildConfig() BuildConfig {
	return BuildConfig{
		Verbosity:       Normal,
		Parallelism:     1,
		FailuresAllowed: 1,
	}
}

type Verbosity int32

const (
	Quiet          Verbosity = iota // No output -- used when testing.
	NoStatusUpdate                  // just regular output but suppress status update
	Normal                          // regular output and status update
	Verbose
)

// A CommandRunner that doesn't actually run the commands.
type dryRunCommandRunner struct {
	finished []*Edge
}

// Overridden from CommandRunner:
func (d *dryRunCommandRunner) CanRunMore() bool {
	return true
}

func (d *dryRunCommandRunner) StartCommand(edge *Edge) bool {
	// In C++ it's a queue. In Go it's a bit less efficient but it shouldn't be
	// performance critical.
	// TODO(maruel): Move items when cap() is significantly larger than len().
	d.finished = append([]*Edge{edge}, d.finished...)
	return true
}

func (d *dryRunCommandRunner) WaitForCommand(result *Result) bool {
	if len(d.finished) == 0 {
		return false
	}

	result.ExitCode = ExitSuccess
	result.Edge = d.finished[len(d.finished)-1]
	d.finished = d.finished[:len(d.finished)-1]
	return true
}

func (d *dryRunCommandRunner) GetActiveEdges() []*Edge {
	return nil
}

func (d *dryRunCommandRunner) Abort() {
}

type realCommandRunner struct {
	config        *BuildConfig
	subprocs      *subprocessSet
	subprocToEdge map[*subprocess]*Edge
}

func newRealCommandRunner(config *BuildConfig) *realCommandRunner {
	return &realCommandRunner{
		config:        config,
		subprocs:      newSubprocessSet(),
		subprocToEdge: map[*subprocess]*Edge{},
	}
}

func (r *realCommandRunner) GetActiveEdges() []*Edge {
	var edges []*Edge
	for _, e := range r.subprocToEdge {
		edges = append(edges, e)
	}
	return edges
}

func (r *realCommandRunner) Abort() {
	r.subprocs.Clear()
}

func (r *realCommandRunner) CanRunMore() bool {
	subprocNumber := r.subprocs.Running() + r.subprocs.Finished()
	more := subprocNumber < r.config.Parallelism
	load := r.subprocs.Running() == 0 || r.config.MaxLoadAvg <= 0. || getLoadAverage() < r.config.MaxLoadAvg
	return more && load
}

func (r *realCommandRunner) StartCommand(edge *Edge) bool {
	command := edge.EvaluateCommand(false)
	subproc := r.subprocs.Add(command, edge.Pool == ConsolePool)
	if subproc == nil {
		return false
	}
	r.subprocToEdge[subproc] = edge
	return true
}

func (r *realCommandRunner) WaitForCommand(result *Result) bool {
	var subproc *subprocess
	for {
		subproc = r.subprocs.NextFinished()
		if subproc != nil {
			break
		}
		if r.subprocs.DoWork() {
			return false
		}
	}

	result.ExitCode = subproc.Finish()
	result.Output = subproc.GetOutput()

	e := r.subprocToEdge[subproc]
	result.Edge = e
	delete(r.subprocToEdge, subproc)
	return true
}

//

// plan stores the state of a build plan: what we intend to build,
// which steps we're ready to execute.
type plan struct {
	// Keep track of which edges we want to build in this plan.  If this map does
	// not contain an entry for an edge, we do not want to build the entry or its
	// dependents.  If it does contain an entry, the enumeration indicates what
	// we want for the edge.
	want map[*Edge]Want

	ready *EdgeSet

	builder *Builder

	// Total number of edges that have commands (not phony).
	commandEdges int

	// Total remaining number of wanted edges.
	wantedEdges int
}

// Returns true if there's more work to be done.
func (p *plan) moreToDo() bool {
	return p.wantedEdges > 0 && p.commandEdges > 0
}

func newPlan(builder *Builder) plan {
	return plan{
		want:    map[*Edge]Want{},
		ready:   NewEdgeSet(),
		builder: builder,
	}
}

// Reset state.  Clears want and ready sets.
func (p *plan) Reset() {
	p.commandEdges = 0
	p.wantedEdges = 0
	p.want = map[*Edge]Want{}
	p.ready = NewEdgeSet()
}

// Add a target to our plan (including all its dependencies).
// Returns false if we don't need to build this target; may
// fill in |err| with an error message if there's a problem.
func (p *plan) addTarget(target *Node, err *string) bool {
	return p.addSubTarget(target, nil, err, nil)
}

func (p *plan) addSubTarget(node *Node, dependent *Node, err *string, dyndepWalk map[*Edge]struct{}) bool {
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

	// If an entry in want does not already exist for edge, create an entry which
	// maps to WantNothing, indicating that we do not want to build this entry itself.
	want, ok := p.want[edge]
	if !ok {
		p.want[edge] = WantNothing
	} else if len(dyndepWalk) != 0 && want == WantToFinish {
		return false // Don't need to do anything with already-scheduled edge.
	}

	// If we do need to build edge and we haven't already marked it as wanted,
	// mark it now.
	if node.Dirty && want == WantNothing {
		want = WantToStart
		p.want[edge] = want
		p.edgeWanted(edge)
		if len(dyndepWalk) == 0 && edge.allInputsReady() {
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
		if !p.addSubTarget(i, node, err, dyndepWalk) && *err != "" {
			return false
		}
	}
	return true
}

func (p *plan) edgeWanted(edge *Edge) {
	p.wantedEdges++
	if edge.Rule != PhonyRule {
		p.commandEdges++
	}
}

// Pop a ready edge off the queue of edges to build.
// Returns NULL if there's no work to do.
func (p *plan) findWork() *Edge {
	return p.ready.Pop()
}

// Submits a ready edge as a candidate for execution.
// The edge may be delayed from running, for example if it's a member of a
// currently-full pool.
func (p *plan) ScheduleWork(edge *Edge, want Want) {
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
	p.want[edge] = WantToFinish

	pool := edge.Pool
	if pool.shouldDelayEdge() {
		pool.delayEdge(edge)
		pool.retrieveReadyEdges(p.ready)
	} else {
		pool.edgeScheduled(edge)
		p.ready.Add(edge)
	}
}

// Mark an edge as done building (whether it succeeded or failed).
// If any of the edge's outputs are dyndep bindings of their dependents,
// this loads dynamic dependencies from the nodes' paths.
// Returns 'false' if loading dyndep info fails and 'true' otherwise.
func (p *plan) edgeFinished(edge *Edge, result edgeResult, err *string) bool {
	want, ok := p.want[edge]
	if !ok {
		panic("M-A")
	}
	directlyWanted := want != WantNothing

	// See if this job frees up any delayed jobs.
	if directlyWanted {
		edge.Pool.edgeFinished(edge)
	}
	edge.Pool.retrieveReadyEdges(p.ready)

	// The rest of this function only applies to successful commands.
	if result != edgeSucceeded {
		return true
	}

	if directlyWanted {
		p.wantedEdges--
	}
	delete(p.want, edge)
	edge.OutputsReady = true

	// Check off any nodes we were waiting for with this edge.
	for _, o := range edge.Outputs {
		if !p.nodeFinished(o, err) {
			return false
		}
	}
	return true
}

// Update plan with knowledge that the given node is up to date.
// If the node is a dyndep binding on any of its dependents, this
// loads dynamic dependencies from the node's path.
// Returns 'false' if loading dyndep info fails and 'true' otherwise.
func (p *plan) nodeFinished(node *Node, err *string) bool {
	// If this node provides dyndep info, load it now.
	if node.DyndepPending {
		if p.builder == nil {
			panic("dyndep requires Plan to have a Builder")
		}
		// Load the now-clean dyndep file.  This will also update the
		// build plan and schedule any new work that is ready.
		if err2 := p.builder.loadDyndeps(node); err2 != nil {
			*err = err2.Error()
			return false
		}
		return true
	}

	// See if we we want any edges from this node.
	for _, oe := range node.OutEdges {
		want, ok := p.want[oe]
		if !ok {
			continue
		}

		// See if the edge is now ready.
		if !p.edgeMaybeReady(oe, want, err) {
			return false
		}
	}
	return true
}

func (p *plan) edgeMaybeReady(edge *Edge, want Want, err *string) bool {
	if edge.allInputsReady() {
		if want != WantNothing {
			p.ScheduleWork(edge, want)
		} else {
			// We do not need to build this edge, but we might need to build one of
			// its dependents.
			if !p.edgeFinished(edge, edgeSucceeded, err) {
				return false
			}
		}
	}
	return true
}

// Clean the given node during the build.
// Return false on error.
func (p *plan) cleanNode(scan *DependencyScan, node *Node, err *string) bool {
	node.Dirty = false

	for _, oe := range node.OutEdges {
		// Don't process edges that we don't actually want.
		want, ok := p.want[oe]
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
			var mostRecentInput *Node
			if end > 0 {
				mostRecentInput = oe.Inputs[0]
				for i := 1; i != end; i++ {
					if oe.Inputs[i].MTime > mostRecentInput.MTime {
						mostRecentInput = oe.Inputs[i]
					}
				}
			}

			// TODO(maruel): This code doesn't have unit test coverage when
			// mostRecentInput is nil.

			// Now, this edge is dirty if any of the outputs are dirty.
			// If the edge isn't dirty, clean the outputs and mark the edge as not
			// wanted.
			outputsDirty := false
			if !scan.recomputeOutputsDirty(oe, mostRecentInput, &outputsDirty, err) {
				return false
			}
			if !outputsDirty {
				for _, o := range oe.Outputs {
					if !p.cleanNode(scan, o, err) {
						return false
					}
				}

				p.want[oe] = WantNothing
				p.wantedEdges--
				if oe.Rule != PhonyRule {
					p.commandEdges--
				}
			}
		}
	}
	return true
}

// Update the build plan to account for modifications made to the graph
// by information loaded from a dyndep file.
func (p *plan) dyndepsLoaded(scan *DependencyScan, node *Node, ddf DyndepFile, err *string) bool {
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
		if _, ok := p.want[edge]; !ok {
			continue
		}

		// This edge is already in the plan so queue it for the walk.
		dyndepRoots = append(dyndepRoots, edge)
	}

	// Walk dyndep-discovered portion of the graph to add it to the build plan.
	dyndepWalk := map[*Edge]struct{}{}
	for _, oe := range dyndepRoots {
		for _, i := range ddf[oe].implicitInputs {
			if !p.addSubTarget(i, oe.Outputs[0], err, dyndepWalk) && *err != "" {
				return false
			}
		}
	}

	// Add out edges from this node that are in the plan (just as
	// NodeFinished would have without taking the dyndep code path).
	for _, oe := range node.OutEdges {
		if _, ok := p.want[oe]; !ok {
			continue
		}
		dyndepWalk[oe] = struct{}{}
	}

	// See if any encountered edges are now ready.
	for wi := range dyndepWalk {
		want, ok := p.want[wi]
		if !ok {
			continue
		}
		if !p.edgeMaybeReady(wi, want, err) {
			return false
		}
	}
	return true
}

func (p *plan) RefreshDyndepDependents(scan *DependencyScan, node *Node, err *string) bool {
	// Collect the transitive closure of dependents and mark their edges
	// as not yet visited by RecomputeDirty.
	dependents := map[*Node]struct{}{}
	p.unmarkDependents(node, dependents)

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
				if !inEdge.OutputsReady && !p.addTarget(v, err) {
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
		wantE, ok := p.want[edge]
		if !ok {
			panic("M-A")
		}
		if wantE == WantNothing {
			p.want[edge] = WantToStart
			p.edgeWanted(edge)
		}
	}
	return true
}

func (p *plan) unmarkDependents(node *Node, dependents map[*Node]struct{}) {
	for _, edge := range node.OutEdges {
		_, ok := p.want[edge]
		if !ok {
			continue
		}

		if edge.Mark != VisitNone {
			edge.Mark = VisitNone
			for _, o := range edge.Outputs {
				_, ok := dependents[o]
				if ok {
					p.unmarkDependents(o, dependents)
				} else {
					dependents[o] = struct{}{}
				}
			}
		}
	}
}

// Dumps the current state of the plan.
func (p *plan) Dump() {
	fmt.Printf("pending: %d\n", len(p.want))
	for e, w := range p.want {
		if w != WantNothing {
			fmt.Printf("want ")
		}
		e.Dump("")
	}
	// TODO(maruel): Uses inner knowledge
	fmt.Printf("ready:\n")
	p.ready.recreate()
	for i := range p.ready.sorted {
		fmt.Printf("\t")
		p.ready.sorted[len(p.ready.sorted)-1-i].Dump("")
	}
}

//

// Builder wraps the build process: starting commands, updating status.
type Builder struct {
	state         *State
	config        *BuildConfig
	plan          plan
	commandRunner commandRunner
	status        Status

	// Map of running edge to time the edge started running.
	runningEdges map[*Edge]int32

	// Time the build started.
	startTimeMillis int64

	di   DiskInterface
	scan DependencyScan
}

func NewBuilder(state *State, config *BuildConfig, buildLog *BuildLog, depsLog *DepsLog, di DiskInterface, status Status, startTimeMillis int64) *Builder {
	b := &Builder{
		state:           state,
		config:          config,
		status:          status,
		runningEdges:    map[*Edge]int32{},
		startTimeMillis: startTimeMillis,
		di:              di,
	}
	b.plan = newPlan(b)
	b.scan = NewDependencyScan(state, buildLog, depsLog, di)
	return b
}

// cleanup cleans up after interrupted commands by deleting output files.
func (b *Builder) cleanup() {
	if b.commandRunner != nil {
		activeEdges := b.commandRunner.GetActiveEdges()
		b.commandRunner.Abort()

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
				newMtime, err := b.di.Stat(o.Path)
				if newMtime == -1 { // Log and ignore Stat() errors.
					b.status.Error("%s", err)
				}
				if depfile != "" || o.MTime != newMtime {
					if err := b.di.RemoveFile(o.Path); err != nil {
						b.status.Error("%s", err)
					}
				}
			}
			if len(depfile) != 0 {
				if err := b.di.RemoveFile(depfile); err != nil {
					b.status.Error("%s", err)
				}
			}
		}
	}
}

// Add a target to the build, scanning dependencies.
// @return false on error.
func (b *Builder) addTargetName(name string, err *string) *Node {
	node := b.state.Paths[name]
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
	if !b.scan.RecomputeDirty(target, &validationNodes, err) {
		return false
	}

	inEdge := target.InEdge
	if inEdge == nil || !inEdge.OutputsReady {
		if !b.plan.addTarget(target, err) {
			return false
		}
	}

	// Also add any validation nodes found during RecomputeDirty as top level
	// targets.
	for _, n := range validationNodes {
		if validationInEdge := n.InEdge; validationInEdge != nil {
			if !validationInEdge.OutputsReady && !b.plan.addTarget(n, err) {
				return false
			}
		}
	}

	return true
}

// Returns true if the build targets are already up to date.
func (b *Builder) AlreadyUpToDate() bool {
	return !b.plan.moreToDo()
}

// Run the build.
//
// It is an error to call this function when AlreadyUpToDate() is true.
func (b *Builder) Build() error {
	if b.AlreadyUpToDate() {
		return errors.New("already up to date")
	}

	b.status.PlanHasTotalEdges(b.plan.commandEdges)
	pendingCommands := 0
	failuresAllowed := b.config.FailuresAllowed

	// Set up the command runner if we haven't done so already.
	if b.commandRunner == nil {
		if b.config.DryRun {
			b.commandRunner = &dryRunCommandRunner{}
		} else {
			b.commandRunner = newRealCommandRunner(b.config)
		}
	}

	// We are about to start the build process.
	b.status.BuildStarted()

	// This main loop runs the entire build process.
	// It is structured like this:
	// First, we attempt to start as many commands as allowed by the
	// command runner.
	// Second, we attempt to wait for / reap the next finished command.
	for b.plan.moreToDo() {
		// See if we can start any more commands.
		if failuresAllowed != 0 && b.commandRunner.CanRunMore() {
			if edge := b.plan.findWork(); edge != nil {
				if edge.GetBinding("generator") != "" {
					_ = b.scan.buildLog.Close()
				}

				err2 := ""
				if !b.startEge(edge, &err2) {
					b.cleanup()
					b.status.BuildFinished()
					return errors.New(err2)
				}

				if edge.Rule == PhonyRule {
					if !b.plan.edgeFinished(edge, edgeSucceeded, &err2) {
						b.cleanup()
						b.status.BuildFinished()
						return errors.New(err2)
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
			if !b.commandRunner.WaitForCommand(&result) || result.ExitCode == ExitInterrupted {
				b.cleanup()
				b.status.BuildFinished()
				return errors.New("interrupted by user")
			}

			pendingCommands--
			err2 := ""
			if !b.finishCommand(&result, &err2) {
				b.cleanup()
				b.status.BuildFinished()
				return errors.New(err2)
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
		b.status.BuildFinished()
		if failuresAllowed == 0 {
			if b.config.FailuresAllowed > 1 {
				return errors.New("subcommands failed")
			}
			return errors.New("subcommand failed")
		} else if failuresAllowed < b.config.FailuresAllowed {
			return errors.New("cannot make progress due to previous errors")
		}
		return errors.New("stuck [this is a bug]")
	}
	b.status.BuildFinished()
	return nil
}

func (b *Builder) startEge(edge *Edge, err *string) bool {
	defer metricRecord("StartEdge")()
	if edge.Rule == PhonyRule {
		return true
	}
	startTimeMillis := int32(time.Now().UnixMilli() - b.startTimeMillis)
	b.runningEdges[edge] = startTimeMillis

	b.status.BuildEdgeStarted(edge, startTimeMillis)

	// Create directories necessary for outputs.
	// XXX: this will block; do we care?
	for _, o := range edge.Outputs {
		if err2 := MakeDirs(b.di, o.Path); err2 != nil {
			*err = err2.Error()
			return false
		}
	}

	// Create response file, if needed
	// XXX: this may also block; do we care?
	rspfile := edge.GetUnescapedRspfile()
	if len(rspfile) != 0 {
		content := edge.GetBinding("rspfile_content")
		if err2 := b.di.WriteFile(rspfile, content); err2 != nil {
			*err = err2.Error()
			return false
		}
	}

	// start command computing and run it
	if !b.commandRunner.StartCommand(edge) {
		*err = "command '" + edge.EvaluateCommand(len(rspfile) != 0) + "' failed."
		return false
	}
	return true
}

// Update status ninja logs following a command termination.
// @return false if the build can not proceed further due to a fatal error.
func (b *Builder) finishCommand(result *Result, err *string) bool {
	defer metricRecord("FinishCommand")()
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
		var err2 error
		depsNodes, err2 = b.extractDeps(result, depsType, depsPrefix)
		if err2 != nil && result.ExitCode == ExitSuccess {
			if result.Output != "" {
				result.Output += "\n"
			}
			result.Output += err2.Error()
			result.ExitCode = ExitFailure
		}
	}

	var startTimeMillis, endTimeMillis int32
	startTimeMillis = b.runningEdges[edge]
	endTimeMillis = int32(time.Now().UnixMilli() - b.startTimeMillis)
	delete(b.runningEdges, edge)

	b.status.BuildEdgeFinished(edge, endTimeMillis, result.ExitCode == ExitSuccess, result.Output)

	// The rest of this function only applies to successful commands.
	if result.ExitCode != ExitSuccess {
		return b.plan.edgeFinished(edge, edgeFailed, err)
	}
	// Restat the edge outputs
	outputMtime := TimeStamp(0)
	restat := edge.GetBinding("restat") != ""
	if !b.config.DryRun {
		nodeCleaned := false

		for _, o := range edge.Outputs {
			newMtime, err2 := b.di.Stat(o.Path)
			if newMtime == -1 {
				*err = err2.Error()
				return false
			}
			if newMtime > outputMtime {
				outputMtime = newMtime
			}
			if o.MTime == newMtime && restat {
				// The rule command did not change the output.  Propagate the clean
				// state through the build graph.
				// Note that this also applies to nonexistent outputs (mtime == 0).
				if !b.plan.cleanNode(&b.scan, o, err) {
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
				inputMtime, err2 := b.di.Stat(i.Path)
				if inputMtime == -1 {
					*err = err2.Error()
					return false
				}
				if inputMtime > restatMtime {
					restatMtime = inputMtime
				}
			}

			depfile := edge.GetUnescapedDepfile()
			if restatMtime != 0 && depsType == "" && depfile != "" {
				depfileMtime, err2 := b.di.Stat(depfile)
				if depfileMtime == -1 {
					*err = err2.Error()
					return false
				}
				if depfileMtime > restatMtime {
					restatMtime = depfileMtime
				}
			}

			// The total number of edges in the plan may have changed as a result
			// of a restat.
			b.status.PlanHasTotalEdges(b.plan.commandEdges)

			outputMtime = restatMtime
		}
	}

	if !b.plan.edgeFinished(edge, edgeSucceeded, err) {
		return false
	}

	// Delete any left over response file.
	rspfile := edge.GetUnescapedRspfile()
	if rspfile != "" && !Debug.KeepRsp {
		if err2 := b.di.RemoveFile(rspfile); err2 != nil {
			*err = err2.Error()
		}
	}

	if b.scan.buildLog != nil {
		if !b.scan.buildLog.RecordCommand(edge, startTimeMillis, endTimeMillis, outputMtime) {
			*err = "Error writing to build log: " // + err
			return false
		}
	}

	if depsType != "" && !b.config.DryRun {
		if len(edge.Outputs) == 0 {
			panic("should have been rejected by parser")
		}
		for _, o := range edge.Outputs {
			depsMtime, err2 := b.di.Stat(o.Path)
			if depsMtime == -1 {
				*err = err2.Error()
				return false
			}
			if !b.scan.depsLog().recordDeps(o, depsMtime, depsNodes) {
				*err = "Error writing to deps log: " // + err
				return false
			}
		}
	}

	return true
}

func (b *Builder) extractDeps(result *Result, depsType string, depsPrefix string) ([]*Node, error) {
	switch depsType {
	case "msvc":
		parser := NewCLParser()
		output := ""
		err := ""
		if !parser.Parse(result.Output, depsPrefix, &output, &err) {
			return nil, errors.New(err)
		}
		result.Output = output
		depsNodes := make([]*Node, 0, len(parser.includes))
		for i := range parser.includes {
			// ~0 is assuming that with MSVC-parsed headers, it's ok to always make
			// all backslashes (as some of the slashes will certainly be backslashes
			// anyway). This could be fixed if necessary with some additional
			// complexity in IncludesNormalize.relativize.
			depsNodes = append(depsNodes, b.state.GetNode(i, 0xFFFFFFFF))
		}
		return depsNodes, nil
	case "gcc":
		depfile := result.Edge.GetUnescapedDepfile()
		if len(depfile) == 0 {
			return nil, errors.New("edge with deps=gcc but no depfile makes no sense")
		}

		// Read depfile content. Treat a missing depfile as empty.
		content, err := b.di.ReadFile(depfile)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if len(content) == 0 {
			return nil, nil
		}

		deps := DepfileParser{}
		err2 := ""
		if !deps.Parse(content, &err2) {
			return nil, errors.New(err2)
		}

		// XXX check depfile matches expected output.
		depsNodes := make([]*Node, len(deps.ins))
		for i, s := range deps.ins {
			depsNodes[i] = b.state.GetNode(CanonicalizePathBits(s))
		}

		if !Debug.KeepDepfile {
			if err := b.di.RemoveFile(depfile); err != nil {
				return depsNodes, err
			}
		}
		return depsNodes, nil
	default:
		return nil, fmt.Errorf("unknown deps type '%s'", depsType)
	}
}

// Load the dyndep information provided by the given node.
func (b *Builder) loadDyndeps(node *Node) error {
	b.status.BuildLoadDyndeps()

	// Load the dyndep information provided by this node.
	ddf := DyndepFile{}
	if err := b.scan.LoadDyndeps(node, ddf); err != nil {
		return err
	}

	// Update the build plan to account for dyndep modifications to the graph.
	err2 := ""
	if !b.plan.dyndepsLoaded(&b.scan, node, ddf, &err2) {
		return errors.New(err2)
	}

	// New command edges may have been added to the plan.
	b.status.PlanHasTotalEdges(b.plan.commandEdges)
	return nil
}
