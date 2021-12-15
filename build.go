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

/*
// Plan stores the state of a build plan: what we intend to build,
// which steps we're ready to execute.
type Plan struct {

  // Returns true if there's more work to be done.
  bool more_to_do() const { return wanted_edges_ > 0 && command_edges_ > 0; }

  enum EdgeResult {
    kEdgeFailed,
    kEdgeSucceeded
  }

  // Number of edges with commands to run.
  int command_edge_count() const { return command_edges_; }

  // Enumerate possible steps we want for an edge.
  enum Want
  {
    // We do not want to build the edge, but we might want to build one of
    // its dependents.
    kWantNothing,
    // We want to build the edge, but have not yet scheduled it.
    kWantToStart,
    // We want to build the edge, have scheduled it, and are waiting
    // for it to complete.
    kWantToFinish
  }

  // Keep track of which edges we want to build in this plan.  If this map does
  // not contain an entry for an edge, we do not want to build the entry or its
  // dependents.  If it does contain an entry, the enumeration indicates what
  // we want for the edge.
  map<Edge*, Want> want_

  EdgeSet ready_

  Builder* builder_

  // Total number of edges that have commands (not phony).
  int command_edges_

  // Total remaining number of wanted edges.
  int wanted_edges_
}

// CommandRunner is an interface that wraps running the build
// subcommands.  This allows tests to abstract out running commands.
// RealCommandRunner is an implementation that actually runs commands.
type CommandRunner struct {
  virtual ~CommandRunner() {}

  // The result of waiting for a command.
  type Result struct {
    Result() : edge(nil) {}
    Edge* edge
    ExitStatus status
    string output
    bool success() const { return status == ExitSuccess; }
  }

  virtual vector<Edge*> GetActiveEdges() { return vector<Edge*>(); }
  func (c *CommandRunner) Abort() {}
}
*/
type Verbosity int

const (
	// No output -- used when testing.
	QUIET Verbosity = iota
	// just regular output but suppress status update
	NO_STATUS_UPDATE
	// regular output and status update
	NORMAL
	VERBOSE
)

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

/*
// Builder wraps the build process: starting commands, updating status.
type Builder struct {
  Builder(State* state, const BuildConfig& config, BuildLog* build_log, DepsLog* deps_log, DiskInterface* disk_interface, Status* status, int64_t start_time_millis)
  ~Builder()

  // Used for tests.
  func (b *Builder) SetBuildLog(log *BuildLog) {
    scan_.set_build_log(log)
  }

  State* state_
  const BuildConfig& config_
  Plan plan_
  auto_ptr<CommandRunner> command_runner_
  unique_ptr<CommandRunner> command_runner_  // auto_ptr was removed in C++17.
  Status* status_

  // Map of running edge to time the edge started running.
  typedef map<const Edge*, int> RunningEdgeMap
  RunningEdgeMap running_edges_

  // Time the build started.
  int64_t start_time_millis_

  DiskInterface* disk_interface_
  DependencyScan scan_

  void operator=(const Builder &other) // DO NOT IMPLEMENT
}


// A CommandRunner that doesn't actually run the commands.
type DryRunCommandRunner struct {
  virtual ~DryRunCommandRunner() {}

  queue<Edge*> finished_
}

// Overridden from CommandRunner:
func (d *DryRunCommandRunner) CanRunMore() bool {
  return true
}

func (d *DryRunCommandRunner) StartCommand(edge *Edge) bool {
  finished_.push(edge)
  return true
}

func (d *DryRunCommandRunner) WaitForCommand(result *Result) bool {
   if finished_.empty() {
     return false
   }

   result.status = ExitSuccess
   result.edge = finished_.front()
   finished_.pop()
   return true
}

Plan::Plan(Builder* builder)
  : builder_(builder)
  , command_edges_(0)
  , wanted_edges_(0)
{}

// Reset state.  Clears want and ready sets.
func (p *Plan) Reset() {
  command_edges_ = 0
  wanted_edges_ = 0
  ready_ = nil
  want_ = nil
}

// Add a target to our plan (including all its dependencies).
// Returns false if we don't need to build this target; may
// fill in |err| with an error message if there's a problem.
func (p *Plan) AddTarget(target *const Node, err *string) bool {
}

func (p *Plan) AddSubTarget(node *const Node, dependent *const Node, err *string, dyndep_walk *set<Edge*>) bool {
  edge := node.in_edge()
  if edge == nil {  // Leaf node.
    if node.dirty() {
      string referenced
      if dependent != nil {
        referenced = ", needed by '" + dependent.path() + "',"
      }
      *err = "'" + node.path() + "'" + referenced + " missing "
             "and no known rule to make it"
    }
    return false
  }

  if edge.outputs_ready() {
    return false  // Don't need to do anything.
  }

  // If an entry in want_ does not already exist for edge, create an entry which
  // maps to kWantNothing, indicating that we do not want to build this entry itself.
  pair<map<Edge*, Want>::iterator, bool> want_ins =
    want_.insert(make_pair(edge, kWantNothing))
  want := want_ins.first.second

  if dyndep_walk && want == kWantToFinish {
    return false  // Don't need to do anything with already-scheduled edge.
  }

  // If we do need to build edge and we haven't already marked it as wanted,
  // mark it now.
  if node.dirty() && want == kWantNothing {
    want = kWantToStart
    EdgeWanted(edge)
    if !dyndep_walk && edge.AllInputsReady() {
      ScheduleWork(want_ins.first)
    }
  }

  if dyndep_walk {
    dyndep_walk.insert(edge)
  }

  if !want_ins.second {
    return true  // We've already processed the inputs.
  }

  for (vector<Node*>::iterator i = edge.inputs_.begin(); i != edge.inputs_.end(); ++i) {
    if !AddSubTarget(*i, node, err, dyndep_walk) && !err.empty() {
      return false
    }
  }

  return true
}

func (p *Plan) EdgeWanted(edge *const Edge) {
  ++wanted_edges_
  if !edge.is_phony() {
    ++command_edges_
  }
}

// Pop a ready edge off the queue of edges to build.
// Returns NULL if there's no work to do.
func (p *Plan) FindWork() Edge* {
  if ready_.empty() {
    return nil
  }
  e := ready_.begin()
  edge := *e
  ready_.erase(e)
  return edge
}

// Submits a ready edge as a candidate for execution.
// The edge may be delayed from running, for example if it's a member of a
// currently-full pool.
func (p *Plan) ScheduleWork(want_e map<Edge*, Want>::iterator) {
  if want_e.second == kWantToFinish {
    // This edge has already been scheduled.  We can get here again if an edge
    // and one of its dependencies share an order-only input, or if a node
    // duplicates an out edge (see https://github.com/ninja-build/ninja/pull/519).
    // Avoid scheduling the work again.
    return
  }
  assert(want_e.second == kWantToStart)
  want_e.second = kWantToFinish

  edge := want_e.first
  pool := edge.pool()
  if pool.ShouldDelayEdge() {
    pool.DelayEdge(edge)
    pool.RetrieveReadyEdges(&ready_)
  } else {
    pool.EdgeScheduled(*edge)
    ready_.insert(edge)
  }
}

// Mark an edge as done building (whether it succeeded or failed).
// If any of the edge's outputs are dyndep bindings of their dependents,
// this loads dynamic dependencies from the nodes' paths.
// Returns 'false' if loading dyndep info fails and 'true' otherwise.
func (p *Plan) EdgeFinished(edge *Edge, result EdgeResult, err *string) bool {
  map<Edge*, Want>::iterator e = want_.find(edge)
  assert(e != want_.end())
  directly_wanted := e.second != kWantNothing

  // See if this job frees up any delayed jobs.
  if directly_wanted {
    edge.pool().EdgeFinished(*edge)
  }
  edge.pool().RetrieveReadyEdges(&ready_)

  // The rest of this function only applies to successful commands.
  if result != kEdgeSucceeded {
    return true
  }

  if directly_wanted {
    --wanted_edges_
  }
  want_.erase(e)
  edge.outputs_ready_ = true

  // Check off any nodes we were waiting for with this edge.
  for (vector<Node*>::iterator o = edge.outputs_.begin(); o != edge.outputs_.end(); ++o) {
    if !NodeFinished(*o, err) {
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
    assert(builder_ && "dyndep requires Plan to have a Builder")
    // Load the now-clean dyndep file.  This will also update the
    // build plan and schedule any new work that is ready.
    return builder_.LoadDyndeps(node, err)
  }

  // See if we we want any edges from this node.
  for (vector<Edge*>::const_iterator oe = node.out_edges().begin(); oe != node.out_edges().end(); ++oe) {
    map<Edge*, Want>::iterator want_e = want_.find(*oe)
    if want_e == want_.end() {
      continue
    }

    // See if the edge is now ready.
    if !EdgeMaybeReady(want_e, err) {
      return false
    }
  }
  return true
}

func (p *Plan) EdgeMaybeReady(want_e map<Edge*, Want>::iterator, err *string) bool {
  edge := want_e.first
  if edge.AllInputsReady() {
    if want_e.second != kWantNothing {
      ScheduleWork(want_e)
    } else {
      // We do not need to build this edge, but we might need to build one of
      // its dependents.
      if !EdgeFinished(edge, kEdgeSucceeded, err) {
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

  for (vector<Edge*>::const_iterator oe = node.out_edges().begin(); oe != node.out_edges().end(); ++oe) {
    // Don't process edges that we don't actually want.
    map<Edge*, Want>::iterator want_e = want_.find(*oe)
    if want_e == want_.end() || want_e.second == kWantNothing {
      continue
    }

    // Don't attempt to clean an edge if it failed to load deps.
    if (*oe).deps_missing_ {
      continue
    }

    // If all non-order-only inputs for this edge are now clean,
    // we might have changed the dirty state of the outputs.
    vector<Node*>::iterator
        begin = (*oe).inputs_.begin(),
        end = (*oe).inputs_.end() - (*oe).order_only_deps_
    if find_if(begin, end, MEM_FN(&Node::dirty)) == end {
      // Recompute most_recent_input.
      most_recent_input := nil
      for (vector<Node*>::iterator i = begin; i != end; ++i) {
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
        for (vector<Node*>::iterator o = (*oe).outputs_.begin(); o != (*oe).outputs_.end(); ++o) {
          if !CleanNode(scan, *o, err) {
            return false
          }
        }

        want_e.second = kWantNothing
        --wanted_edges_
        if !(*oe).is_phony() {
          --command_edges_
        }
      }
    }
  }
  return true
}

// Update the build plan to account for modifications made to the graph
// by information loaded from a dyndep file.
func (p *Plan) DyndepsLoaded(scan *DependencyScan, node *const Node, ddf *DyndepFile, err *string) bool {
  // Recompute the dirty state of all our direct and indirect dependents now
  // that our dyndep information has been loaded.
  if !RefreshDyndepDependents(scan, node, err) {
    return false
  }

  // We loaded dyndep information for those out_edges of the dyndep node that
  // specify the node in a dyndep binding, but they may not be in the plan.
  // Starting with those already in the plan, walk newly-reachable portion
  // of the graph through the dyndep-discovered dependencies.

  // Find edges in the the build plan for which we have new dyndep info.
  vector<DyndepFile::const_iterator> dyndep_roots
  for (DyndepFile::const_iterator oe = ddf.begin(); oe != ddf.end(); ++oe) {
    edge := oe.first

    // If the edge outputs are ready we do not need to consider it here.
    if edge.outputs_ready() {
      continue
    }

    map<Edge*, Want>::iterator want_e = want_.find(edge)

    // If the edge has not been encountered before then nothing already in the
    // plan depends on it so we do not need to consider the edge yet either.
    if want_e == want_.end() {
      continue
    }

    // This edge is already in the plan so queue it for the walk.
    dyndep_roots.push_back(oe)
  }

  // Walk dyndep-discovered portion of the graph to add it to the build plan.
  set<Edge*> dyndep_walk
  for (vector<DyndepFile::const_iterator>::iterator oei = dyndep_roots.begin(); oei != dyndep_roots.end(); ++oei) {
    DyndepFile::const_iterator oe = *oei
    for (vector<Node*>::const_iterator i = oe.second.implicit_inputs_.begin(); i != oe.second.implicit_inputs_.end(); ++i) {
      if !AddSubTarget(*i, oe.first.outputs_[0], err, &dyndep_walk) && !err.empty() {
        return false
      }
    }
  }

  // Add out edges from this node that are in the plan (just as
  // Plan::NodeFinished would have without taking the dyndep code path).
  for (vector<Edge*>::const_iterator oe = node.out_edges().begin(); oe != node.out_edges().end(); ++oe) {
    map<Edge*, Want>::iterator want_e = want_.find(*oe)
    if want_e == want_.end() {
      continue
    }
    dyndep_walk.insert(want_e.first)
  }

  // See if any encountered edges are now ready.
  for (set<Edge*>::iterator wi = dyndep_walk.begin(); wi != dyndep_walk.end(); ++wi) {
    map<Edge*, Want>::iterator want_e = want_.find(*wi)
    if want_e == want_.end() {
      continue
    }
    if !EdgeMaybeReady(want_e, err) {
      return false
    }
  }

  return true
}

func (p *Plan) RefreshDyndepDependents(scan *DependencyScan, node *const Node, err *string) bool {
  // Collect the transitive closure of dependents and mark their edges
  // as not yet visited by RecomputeDirty.
  set<Node*> dependents
  UnmarkDependents(node, &dependents)

  // Update the dirty state of all dependents and check if their edges
  // have become wanted.
  for (set<Node*>::iterator i = dependents.begin(); i != dependents.end(); ++i) {
    n := *i

    // Check if this dependent node is now dirty.  Also checks for new cycles.
    if !scan.RecomputeDirty(n, err) {
      return false
    }
    if !n.dirty() {
      continue
    }

    // This edge was encountered before.  However, we may not have wanted to
    // build it if the outputs were not known to be dirty.  With dyndep
    // information an output is now known to be dirty, so we want the edge.
    edge := n.in_edge()
    assert(edge && !edge.outputs_ready())
    map<Edge*, Want>::iterator want_e = want_.find(edge)
    assert(want_e != want_.end())
    if want_e.second == kWantNothing {
      want_e.second = kWantToStart
      EdgeWanted(edge)
    }
  }
  return true
}

func (p *Plan) UnmarkDependents(node *const Node, dependents *set<Node*>) {
  for (vector<Edge*>::const_iterator oe = node.out_edges().begin(); oe != node.out_edges().end(); ++oe) {
    edge := *oe

    map<Edge*, Want>::iterator want_e = want_.find(edge)
    if want_e == want_.end() {
      continue
    }

    if edge.mark_ != Edge::VisitNone {
      edge.mark_ = Edge::VisitNone
      for (vector<Node*>::iterator o = edge.outputs_.begin(); o != edge.outputs_.end(); ++o) {
        if dependents.insert(*o).second {
          UnmarkDependents(*o, dependents)
        }
      }
    }
  }
}

// Dumps the current state of the plan.
func (p *Plan) Dump() {
  printf("pending: %d\n", (int)want_.size())
  for (map<Edge*, Want>::const_iterator e = want_.begin(); e != want_.end(); ++e) {
    if e.second != kWantNothing {
      printf("want ")
    }
    e.first.Dump()
  }
  printf("ready: %d\n", (int)ready_.size())
}

type RealCommandRunner struct {
  explicit RealCommandRunner(const BuildConfig& config) : config_(config) {}
  virtual ~RealCommandRunner() {}

  const BuildConfig& config_
  SubprocessSet subprocs_
  map<const Subprocess*, Edge*> subproc_to_edge_
}

func (r *RealCommandRunner) GetActiveEdges() vector<Edge*> {
  vector<Edge*> edges
  for (map<const Subprocess*, Edge*>::iterator e = subproc_to_edge_.begin(); e != subproc_to_edge_.end(); ++e)
    edges.push_back(e.second)
  return edges
}

func (r *RealCommandRunner) Abort() {
  subprocs_.Clear()
}

func (r *RealCommandRunner) CanRunMore() bool {
  size_t subproc_number =
      subprocs_.running_.size() + subprocs_.finished_.size()
  return (int)subproc_number < config_.parallelism
    && ((subprocs_.running_.empty() || config_.max_load_average <= 0.0f) || GetLoadAverage() < config_.max_load_average)
}

func (r *RealCommandRunner) StartCommand(edge *Edge) bool {
  command := edge.EvaluateCommand()
  subproc := subprocs_.Add(command, edge.use_console())
  if subproc == nil {
    return false
  }
  subproc_to_edge_.insert(make_pair(subproc, edge))

  return true
}

func (r *RealCommandRunner) WaitForCommand(result *Result) bool {
  Subprocess* subproc
  while ((subproc = subprocs_.NextFinished()) == nil) {
    interrupted := subprocs_.DoWork()
    if interrupted != nil {
      return false
    }
  }

  result.status = subproc.Finish()
  result.output = subproc.GetOutput()

  map<const Subprocess*, Edge*>::iterator e = subproc_to_edge_.find(subproc)
  result.edge = e.second
  subproc_to_edge_.erase(e)

  delete subproc
  return true
}

Builder::Builder(State* state, const BuildConfig& config, BuildLog* build_log, DepsLog* deps_log, DiskInterface* disk_interface, Status *status, int64_t start_time_millis)
    : state_(state), config_(config), plan_(this), status_(status),
      start_time_millis_(start_time_millis), disk_interface_(disk_interface),
      scan_(state, build_log, deps_log, disk_interface, &config_.depfile_parser_options) {
}

Builder::~Builder() {
  Cleanup()
}

// Clean up after interrupted commands by deleting output files.
func (b *Builder) Cleanup() {
  if command_runner_.get() {
    active_edges := command_runner_.GetActiveEdges()
    command_runner_.Abort()

    for (vector<Edge*>::iterator e = active_edges.begin(); e != active_edges.end(); ++e) {
      depfile := (*e).GetUnescapedDepfile()
      for (vector<Node*>::iterator o = (*e).outputs_.begin(); o != (*e).outputs_.end(); ++o) {
        // Only delete this output if it was actually modified.  This is
        // important for things like the generator where we don't want to
        // delete the manifest file if we can avoid it.  But if the rule
        // uses a depfile, always delete.  (Consider the case where we
        // need to rebuild an output because of a modified header file
        // mentioned in a depfile, and the command touches its depfile
        // but is interrupted before it touches its output file.)
        string err
        new_mtime := disk_interface_.Stat((*o).path(), &err)
        if new_mtime == -1 {  // Log and ignore Stat() errors.
          status_.Error("%s", err)
        }
        if !depfile.empty() || (*o).mtime() != new_mtime {
          disk_interface_.RemoveFile((*o).path())
        }
      }
      if len(depfile) != 0 {
        disk_interface_.RemoveFile(depfile)
      }
    }
  }
}

// Add a target to the build, scanning dependencies.
// @return false on error.
func (b *Builder) AddTarget(name string, err *string) Node* {
  node := state_.LookupNode(name)
  if node == nil {
    *err = "unknown target: '" + name + "'"
    return nil
  }
  if !AddTarget(node, err) {
    return nil
  }
  return node
}

// Add a target to the build, scanning dependencies.
// @return false on error.
func (b *Builder) AddTarget(target *Node, err *string) bool {
  if !scan_.RecomputeDirty(target, err) {
    return false
  }

  if Edge* in_edge = target.in_edge() {
    if in_edge.outputs_ready() {
      return true  // Nothing to do.
    }
  }

  if !plan_.AddTarget(target, err) {
    return false
  }

  return true
}

// Returns true if the build targets are already up to date.
func (b *Builder) AlreadyUpToDate() bool {
  return !plan_.more_to_do()
}

// Run the build.  Returns false on error.
// It is an error to call this function when AlreadyUpToDate() is true.
func (b *Builder) Build(err *string) bool {
  assert(!AlreadyUpToDate())

  status_.PlanHasTotalEdges(plan_.command_edge_count())
  pending_commands := 0
  failures_allowed := config_.failures_allowed

  // Set up the command runner if we haven't done so already.
  if !command_runner_.get() {
    if config_.dry_run {
      command_runner_.reset(new DryRunCommandRunner)
    } else {
      command_runner_.reset(new RealCommandRunner(config_))
    }
  }

  // We are about to start the build process.
  status_.BuildStarted()

  // This main loop runs the entire build process.
  // It is structured like this:
  // First, we attempt to start as many commands as allowed by the
  // command runner.
  // Second, we attempt to wait for / reap the next finished command.
  while (plan_.more_to_do()) {
    // See if we can start any more commands.
    if failures_allowed && command_runner_.CanRunMore() {
      if Edge* edge = plan_.FindWork() {
        if edge.GetBindingBool("generator") {
          scan_.build_log().Close()
        }

        if !StartEdge(edge, err) {
          Cleanup()
          status_.BuildFinished()
          return false
        }

        if edge.is_phony() {
          if !plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, err) {
            Cleanup()
            status_.BuildFinished()
            return false
          }
        } else {
          ++pending_commands
        }

        // We made some progress; go back to the main loop.
        continue
      }
    }

    // See if we can reap any finished commands.
    if pending_commands {
      CommandRunner::Result result
      if !command_runner_.WaitForCommand(&result) || result.status == ExitInterrupted {
        Cleanup()
        status_.BuildFinished()
        *err = "interrupted by user"
        return false
      }

      --pending_commands
      if !FinishCommand(&result, err) {
        Cleanup()
        status_.BuildFinished()
        return false
      }

      if !result.success() {
        if failures_allowed {
          failures_allowed--
        }
      }

      // We made some progress; start the main loop over.
      continue
    }

    // If we get here, we cannot make any more progress.
    status_.BuildFinished()
    if failures_allowed == 0 {
      if config_.failures_allowed > 1 {
        *err = "subcommands failed"
      } else {
        *err = "subcommand failed"
      }
    } else if failures_allowed < config_.failures_allowed {
      *err = "cannot make progress due to previous errors"
    } else {
      *err = "stuck [this is a bug]"
    }

    return false
  }

  status_.BuildFinished()
  return true
}

func (b *Builder) StartEdge(edge *Edge, err *string) bool {
  METRIC_RECORD("StartEdge")
  if edge.is_phony() {
    return true
  }

  int64_t start_time_millis = GetTimeMillis() - start_time_millis_
  running_edges_.insert(make_pair(edge, start_time_millis))

  status_.BuildEdgeStarted(edge, start_time_millis)

  // Create directories necessary for outputs.
  // XXX: this will block; do we care?
  for (vector<Node*>::iterator o = edge.outputs_.begin(); o != edge.outputs_.end(); ++o) {
    if !disk_interface_.MakeDirs((*o).path()) {
      return false
    }
  }

  // Create response file, if needed
  // XXX: this may also block; do we care?
  rspfile := edge.GetUnescapedRspfile()
  if len(rspfile) != 0 {
    content := edge.GetBinding("rspfile_content")
    if !disk_interface_.WriteFile(rspfile, content) {
      return false
    }
  }

  // start command computing and run it
  if !command_runner_.StartCommand(edge) {
    err.assign("command '" + edge.EvaluateCommand() + "' failed.")
    return false
  }

  return true
}

// Update status ninja logs following a command termination.
// @return false if the build can not proceed further due to a fatal error.
func (b *Builder) FinishCommand(result *CommandRunner::Result, err *string) bool {
  METRIC_RECORD("FinishCommand")

  edge := result.edge

  // First try to extract dependencies from the result, if any.
  // This must happen first as it filters the command output (we want
  // to filter /showIncludes output, even on compile failure) and
  // extraction itself can fail, which makes the command fail from a
  // build perspective.
  vector<Node*> deps_nodes
  deps_type := edge.GetBinding("deps")
  const string deps_prefix = edge.GetBinding("msvc_deps_prefix")
  if !deps_type.empty() {
    string extract_err
    if !ExtractDeps(result, deps_type, deps_prefix, &deps_nodes, &extract_err) && result.success() {
      if !result.output.empty() {
        result.output.append("\n")
      }
      result.output.append(extract_err)
      result.status = ExitFailure
    }
  }

  int64_t start_time_millis, end_time_millis
  it := running_edges_.find(edge)
  start_time_millis = it.second
  end_time_millis = GetTimeMillis() - start_time_millis_
  running_edges_.erase(it)

  status_.BuildEdgeFinished(edge, end_time_millis, result.success(), result.output)

  // The rest of this function only applies to successful commands.
  if !result.success() {
    return plan_.EdgeFinished(edge, Plan::kEdgeFailed, err)
  }

  // Restat the edge outputs
  output_mtime := 0
  restat := edge.GetBindingBool("restat")
  if !config_.dry_run {
    node_cleaned := false

    for (vector<Node*>::iterator o = edge.outputs_.begin(); o != edge.outputs_.end(); ++o) {
      new_mtime := disk_interface_.Stat((*o).path(), err)
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
        if !plan_.CleanNode(&scan_, *o, err) {
          return false
        }
        node_cleaned = true
      }
    }

    if node_cleaned {
      restat_mtime := 0
      // If any output was cleaned, find the most recent mtime of any
      // (existing) non-order-only input or the depfile.
      for (vector<Node*>::iterator i = edge.inputs_.begin(); i != edge.inputs_.end() - edge.order_only_deps_; ++i) {
        input_mtime := disk_interface_.Stat((*i).path(), err)
        if input_mtime == -1 {
          return false
        }
        if input_mtime > restat_mtime {
          restat_mtime = input_mtime
        }
      }

      depfile := edge.GetUnescapedDepfile()
      if restat_mtime != 0 && deps_type.empty() && !depfile.empty() {
        depfile_mtime := disk_interface_.Stat(depfile, err)
        if depfile_mtime == -1 {
          return false
        }
        if depfile_mtime > restat_mtime {
          restat_mtime = depfile_mtime
        }
      }

      // The total number of edges in the plan may have changed as a result
      // of a restat.
      status_.PlanHasTotalEdges(plan_.command_edge_count())

      output_mtime = restat_mtime
    }
  }

  if !plan_.EdgeFinished(edge, Plan::kEdgeSucceeded, err) {
    return false
  }

  // Delete any left over response file.
  rspfile := edge.GetUnescapedRspfile()
  if !rspfile.empty() && !g_keep_rsp {
    disk_interface_.RemoveFile(rspfile)
  }

  if scan_.build_log() {
    if !scan_.build_log().RecordCommand(edge, start_time_millis, end_time_millis, output_mtime) {
      *err = string("Error writing to build log: ") + strerror(errno)
      return false
    }
  }

  if !deps_type.empty() && !config_.dry_run {
    assert(!edge.outputs_.empty() && "should have been rejected by parser")
    for (vector<Node*>::const_iterator o = edge.outputs_.begin(); o != edge.outputs_.end(); ++o) {
      deps_mtime := disk_interface_.Stat((*o).path(), err)
      if deps_mtime == -1 {
        return false
      }
      if !scan_.deps_log().RecordDeps(*o, deps_mtime, deps_nodes) {
        *err = string("Error writing to deps log: ") + strerror(errno)
        return false
      }
    }
  }
  return true
}

func (b *Builder) ExtractDeps(result *CommandRunner::Result, deps_type string, deps_prefix string, deps_nodes *vector<Node*>, err *string) bool {
  if deps_type == "msvc" {
    CLParser parser
    string output
    if !parser.Parse(result.output, deps_prefix, &output, err) {
      return false
    }
    result.output = output
    for (set<string>::iterator i = parser.includes_.begin(); i != parser.includes_.end(); ++i) {
      // ~0 is assuming that with MSVC-parsed headers, it's ok to always make
      // all backslashes (as some of the slashes will certainly be backslashes
      // anyway). This could be fixed if necessary with some additional
      // complexity in IncludesNormalize::Relativize.
      deps_nodes.push_back(state_.GetNode(*i, ~0u))
    }
  } else if deps_type == "gcc" {
    depfile := result.edge.GetUnescapedDepfile()
    if len(depfile) == 0 {
      *err = string("edge with deps=gcc but no depfile makes no sense")
      return false
    }

    // Read depfile content.  Treat a missing depfile as empty.
    string content
    switch (disk_interface_.ReadFile(depfile, &content, err)) {
    case DiskInterface::Okay:
      break
    case DiskInterface::NotFound:
      err = nil
      break
    case DiskInterface::OtherError:
      return false
    }
    if len(content) == 0 {
      return true
    }

    DepfileParser deps(config_.depfile_parser_options)
    if !deps.Parse(&content, err) {
      return false
    }

    // XXX check depfile matches expected output.
    deps_nodes.reserve(deps.ins_.size())
    for (vector<StringPiece>::iterator i = deps.ins_.begin(); i != deps.ins_.end(); ++i) {
      uint64_t slash_bits
      CanonicalizePath(const_cast<char*>(i.str_), &i.len_, &slash_bits)
      deps_nodes.push_back(state_.GetNode(*i, slash_bits))
    }

    if !g_keep_depfile {
      if disk_interface_.RemoveFile(depfile) < 0 {
        *err = string("deleting depfile: ") + strerror(errno) + string("\n")
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
  status_.BuildLoadDyndeps()

  // Load the dyndep information provided by this node.
  DyndepFile ddf
  if !scan_.LoadDyndeps(node, &ddf, err) {
    return false
  }

  // Update the build plan to account for dyndep modifications to the graph.
  if !plan_.DyndepsLoaded(&scan_, node, ddf, err) {
    return false
  }

  // New command edges may have been added to the plan.
  status_.PlanHasTotalEdges(plan_.command_edge_count())

  return true
}
*/
