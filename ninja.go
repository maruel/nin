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


// Command-line options.
type Options struct {
  // Build file to load.
  string input_file

  // Directory to change into before running.
  string working_dir

  // Tool to run rather than building.
  const Tool* tool

  // Whether duplicate rules for one target should warn or print an error.
  bool dupe_edges_should_err

  // Whether phony cycles should warn or print an error.
  bool phony_cycle_should_err
}

// The Ninja main() loads up a series of data structures; various tools need
// to poke into these, so store them as fields on an object.
type NinjaMain struct {
  NinjaMain(string ninja_command, const BuildConfig& config) :
      ninja_command_(ninja_command), config_(config),
      start_time_millis_(GetTimeMillis()) {}

  // Command line used to run Ninja.
  string ninja_command_

  // Build configuration set from flags (e.g. parallelism).
  const BuildConfig& config_

  // Loaded state (rules, nodes).
  State state_

  // Functions for accessing the disk.
  RealDiskInterface disk_interface_

  // The build directory, used for storing the build log etc.
  string build_dir_

  BuildLog build_log_
  DepsLog deps_log_

  // The type of functions that are the entry points to tools (subcommands).
  typedef int (NinjaMain::*ToolFunc)(const Options*, int, char**)

  func (n *NinjaMain) IsPathDead(s StringPiece) bool {
    n := state_.LookupNode(s)
    if n && n.in_edge() {
      return false
    }
    // Just checking n isn't enough: If an old output is both in the build log
    // and in the deps log, it will have a Node object in state_.  (It will also
    // have an in edge if one of its inputs is another output that's in the deps
    // log, but having a deps edge product an output that's input to another deps
    // edge is rare, and the first recompaction will delete all old outputs from
    // the deps log, and then a second recompaction will clear the build log,
    // which seems good enough for this corner case.)
    // Do keep entries around for files which still exist on disk, for
    // generators that want to use this information.
    string err
    mtime := disk_interface_.Stat(s.AsString(), &err)
    if mtime == -1 {
      Error("%s", err)  // Log and ignore Stat() errors.
    }
    return mtime == 0
  }

  int64_t start_time_millis_
}

// Subtools, accessible via "-t foo".
type Tool struct {
  // Short name of the tool.
  string name

  // Description (shown in "-t list").
  string desc

  // When to run the tool.
  enum {
    // Run after parsing the command-line flags and potentially changing
    // the current working directory (as early as possible).
    RUN_AFTER_FLAGS,

    // Run after loading build.ninja.
    RUN_AFTER_LOAD,

    // Run after loading the build/deps logs.
    RUN_AFTER_LOGS,
  } when

  // Implementation of the tool.
  NinjaMain::ToolFunc func
}

// Print usage information.
func Usage(config *BuildConfig) {
  fprintf(stderr, "usage: ninja [options] [targets...]\n" "\n" "if targets are unspecified, builds the 'default' target (see manual).\n" "\n" "options:\n" "  --version      print ninja version (\"%s\")\n" "  -v, --verbose  show all command lines while building\n" "  --quiet        don't show progress status, just command output\n" "\n" "  -C DIR   change to DIR before doing anything else\n" "  -f FILE  specify input build file [default=build.ninja]\n" "\n" "  -j N     run N jobs in parallel (0 means infinity) [default=%d on this system]\n" "  -k N     keep going until N jobs fail (0 means infinity) [default=1]\n" "  -l N     do not start new jobs if the load average is greater than N\n" "  -n       dry run (don't run commands but act like they succeeded)\n" "\n" "  -d MODE  enable debugging (use '-d list' to list modes)\n" "  -t TOOL  run a subtool (use '-t list' to list subtools)\n" "    terminates toplevel options; further flags are passed to the tool\n" "  -w FLAG  adjust warnings (use '-w list' to list warnings)\n", kNinjaVersion, config.parallelism)
}

// Choose a default value for the -j (parallelism) flag.
func GuessParallelism() int {
  switch (int processors = GetProcessorCount()) {
  case 0:
  case 1:
    return 2
  case 2:
    return 3
  default:
    return processors + 2
  }
}

// Rebuild the build manifest, if necessary.
// Returns true if the manifest was rebuilt.
// Rebuild the manifest, if necessary.
// Fills in \a err on error.
// @return true if the manifest was rebuilt.
func (n *NinjaMain) RebuildManifest(input_file string, err *string, status *Status) bool {
  path := input_file
  if len(path) == 0 {
    *err = "empty path"
    return false
  }
  uint64_t slash_bits  // Unused because this path is only used for lookup.
  CanonicalizePath(&path, &slash_bits)
  node := state_.LookupNode(path)
  if node == nil {
    return false
  }

  Builder builder(&state_, config_, &build_log_, &deps_log_, &disk_interface_, status, start_time_millis_)
  if !builder.AddTarget(node, err) {
    return false
  }

  if builder.AlreadyUpToDate() {
    return false  // Not an error, but we didn't rebuild.
  }

  if !builder.Build(err) {
    return false
  }

  // The manifest was only rebuilt if it is now dirty (it may have been cleaned
  // by a restat).
  if !node.dirty() {
    // Reset the state to prevent problems like
    // https://github.com/ninja-build/ninja/issues/874
    state_.Reset()
    return false
  }

  return true
}

// Get the Node for a given command-line path, handling features like
// spell correction.
func (n *NinjaMain) CollectTarget(cpath string, err *string) Node* {
  path := cpath
  if len(path) == 0 {
    *err = "empty path"
    return nil
  }
  uint64_t slash_bits
  CanonicalizePath(&path, &slash_bits)

  // Special syntax: "foo.cc^" means "the first output of foo.cc".
  first_dependent := false
  if !path.empty() && path[path.size() - 1] == '^' {
    path.resize(path.size() - 1)
    first_dependent = true
  }

  node := state_.LookupNode(path)
  if node != nil {
    if first_dependent {
      if node.out_edges().empty() {
        rev_deps := deps_log_.GetFirstReverseDepsNode(node)
        if !rev_deps {
          *err = "'" + path + "' has no out edge"
          return nil
        }
        node = rev_deps
      } else {
        edge := node.out_edges()[0]
        if edge.outputs_.empty() {
          edge.Dump()
          Fatal("edge has no outputs")
        }
        node = edge.outputs_[0]
      }
    }
    return node
  } else {
    *err =
        "unknown target '" + Node::PathDecanonicalized(path, slash_bits) + "'"
    if path == "clean" {
      *err += ", did you mean 'ninja -t clean'?"
    } else if path == "help" {
      *err += ", did you mean 'ninja -h'?"
    } else {
      suggestion := state_.SpellcheckNode(path)
      if suggestion != nil {
        *err += ", did you mean '" + suggestion.path() + "'?"
      }
    }
    return nil
  }
}

// CollectTarget for all command-line arguments, filling in \a targets.
func (n *NinjaMain) CollectTargetsFromArgs(argc int, argv []*char, targets *vector<Node*>, err *string) bool {
  if argc == 0 {
    *targets = state_.DefaultNodes(err)
    return err.empty()
  }

  for i := 0; i < argc; i++ {
    node := CollectTarget(argv[i], err)
    if node == nil {
      return false
    }
    targets.push_back(node)
  }
  return true
}

// The various subcommands, run via "-t XXX".
func (n *NinjaMain) ToolGraph(options *const Options, argc int, argv []*char) int {
  vector<Node*> nodes
  string err
  if !CollectTargetsFromArgs(argc, argv, &nodes, &err) {
    Error("%s", err)
    return 1
  }

  GraphViz graph(&state_, &disk_interface_)
  graph.Start()
  for n := nodes.begin(); n != nodes.end(); n++ {
    graph.AddTarget(*n)
  }
  graph.Finish()

  return 0
}

func (n *NinjaMain) ToolQuery(options *const Options, argc int, argv []*char) int {
  if argc == 0 {
    Error("expected a target to query")
    return 1
  }

  DyndepLoader dyndep_loader(&state_, &disk_interface_)

  for i := 0; i < argc; i++ {
    string err
    node := CollectTarget(argv[i], &err)
    if node == nil {
      Error("%s", err)
      return 1
    }

    printf("%s:\n", node.path())
    if Edge* edge = node.in_edge() {
      if edge.dyndep_ && edge.dyndep_.dyndep_pending() {
        if !dyndep_loader.LoadDyndeps(edge.dyndep_, &err) {
          Warning("%s\n", err)
        }
      }
      printf("  input: %s\n", edge.rule_.name())
      for in := 0; in < (int)edge.inputs_.size(); in++ {
        string label = ""
        if edge.is_implicit(in) {
          label = "| "
        } else if edge.is_order_only(in) {
          label = "|| "
        }
        printf("    %s%s\n", label, edge.inputs_[in].path())
      }
    }
    printf("  outputs:\n")
    for edge := node.out_edges().begin(); edge != node.out_edges().end(); edge++ {
      for out := (*edge).outputs_.begin(); out != (*edge).outputs_.end(); out++ {
        printf("    %s\n", (*out).path())
      }
    }
  }
  return 0
}

func (n *NinjaMain) ToolBrowse(options *const Options, argc int, argv []*char) int {
  RunBrowsePython(&state_, ninja_command_, options.input_file, argc, argv)
  // If we get here, the browse failed.
  return 1
}
func (n *NinjaMain) ToolBrowse(Options* const, int, char**) int {
  Fatal("browse tool not supported on this platform")
  return 1
}

func (n *NinjaMain) ToolMSVC(options *const Options, argc int, argv []*char) int {
  // Reset getopt: push one argument onto the front of argv, reset optind.
  argc++
  argv--
  optind = 0
  return MSVCHelperMain(argc, argv)
}

func ToolTargetsList(nodes *vector<Node*>, depth int, indent int) int {
  for n := nodes.begin(); n != nodes.end(); n++ {
    for i := 0; i < indent; i++ {
      printf("  ")
    }
    target := (*n).path()
    if (*n).in_edge() {
      printf("%s: %s\n", target, (*n).in_edge().rule_.name())
      if depth > 1 || depth <= 0 {
        ToolTargetsList((*n).in_edge().inputs_, depth - 1, indent + 1)
      }
    } else {
      printf("%s\n", target)
    }
  }
  return 0
}

func ToolTargetsSourceList(state *State) int {
  for e := state.edges_.begin(); e != state.edges_.end(); e++ {
    for inps := (*e).inputs_.begin(); inps != (*e).inputs_.end(); inps++ {
      if !(*inps).in_edge() {
        printf("%s\n", (*inps).path())
      }
    }
  }
  return 0
}

func ToolTargetsList(state *State, rule_name string) int {
  set<string> rules

  // Gather the outputs.
  for e := state.edges_.begin(); e != state.edges_.end(); e++ {
    if (*e).rule_.name() == rule_name {
      for out_node := (*e).outputs_.begin(); out_node != (*e).outputs_.end(); out_node++ {
        rules.insert((*out_node).path())
      }
    }
  }

  // Print them.
  for i := rules.begin(); i != rules.end(); i++ {
    printf("%s\n", (*i))
  }

  return 0
}

func ToolTargetsList(state *State) int {
  for e := state.edges_.begin(); e != state.edges_.end(); e++ {
    for out_node := (*e).outputs_.begin(); out_node != (*e).outputs_.end(); out_node++ {
      printf("%s: %s\n", (*out_node).path(), (*e).rule_.name())
    }
  }
  return 0
}

func (n *NinjaMain) ToolDeps(options *const Options, argc int, argv **char) int {
  vector<Node*> nodes
  if argc == 0 {
    for ni := deps_log_.nodes().begin(); ni != deps_log_.nodes().end(); ni++ {
      if deps_log_.IsDepsEntryLiveFor(*ni) {
        nodes.push_back(*ni)
      }
    }
  } else {
    string err
    if !CollectTargetsFromArgs(argc, argv, &nodes, &err) {
      Error("%s", err)
      return 1
    }
  }

  RealDiskInterface disk_interface
  for vector<Node*>::iterator it = nodes.begin(), end = nodes.end(); it != end; it++ {
    deps := deps_log_.GetDeps(*it)
    if deps == nil {
      printf("%s: deps not found\n", (*it).path())
      continue
    }

    string err
    mtime := disk_interface.Stat((*it).path(), &err)
    if mtime == -1 {
      Error("%s", err)  // Log and ignore Stat() errors;
    }
    printf("%s: #deps %d, deps mtime %" PRId64 " (%s)\n", (*it).path(), deps.node_count, deps.mtime, (!mtime || mtime > deps.mtime ? "STALE":"VALID"))
    for i := 0; i < deps.node_count; i++ {
      printf("    %s\n", deps.nodes[i].path())
    }
    printf("\n")
  }

  return 0
}

func (n *NinjaMain) ToolMissingDeps(options *const Options, argc int, argv **char) int {
  vector<Node*> nodes
  string err
  if !CollectTargetsFromArgs(argc, argv, &nodes, &err) {
    Error("%s", err)
    return 1
  }
  RealDiskInterface disk_interface
  MissingDependencyPrinter printer
  MissingDependencyScanner scanner(&printer, &deps_log_, &state_, &disk_interface)
  for it := nodes.begin(); it != nodes.end(); it++ {
    scanner.ProcessNode(*it)
  }
  scanner.PrintStats()
  if scanner.HadMissingDeps() {
    return 3
  }
  return 0
}

func (n *NinjaMain) ToolTargets(options *const Options, argc int, argv []*char) int {
  depth := 1
  if argc >= 1 {
    mode := argv[0]
    if mode == "rule" {
      string rule
      if argc > 1 {
        rule = argv[1]
      }
      if len(rule) == 0 {
        return ToolTargetsSourceList(&state_)
      } else {
        return ToolTargetsList(&state_, rule)
      }
    } else if mode == "depth" {
      if argc > 1 {
        depth = atoi(argv[1])
      }
    } else if mode == "all" {
      return ToolTargetsList(&state_)
    } else {
      string suggestion =
          SpellcheckString(mode, "rule", "depth", "all", nil)
      if suggestion != nil {
        Error("unknown target tool mode '%s', did you mean '%s'?", mode, suggestion)
      } else {
        Error("unknown target tool mode '%s'", mode)
      }
      return 1
    }
  }

  string err
  root_nodes := state_.RootNodes(&err)
  if len(err) == 0 {
    return ToolTargetsList(root_nodes, depth, 0)
  } else {
    Error("%s", err)
    return 1
  }
}

func (n *NinjaMain) ToolRules(options *const Options, argc int, argv []*char) int {
  // Parse options.

  // The rules tool uses getopt, and expects argv[0] to contain the name of
  // the tool, i.e. "rules".
  argc++
  argv--

  print_description := false

  optind = 1
  int opt
  while (opt = getopt(argc, argv, const_cast<char*>("hd"))) != -1 {
    switch (opt) {
    case 'd':
      print_description = true
      break
    case 'h':
    default:
      printf("usage: ninja -t rules [options]\n" "\n" "options:\n" "  -d     also print the description of the rule\n" "  -h     print this message\n" )
    return 1
    }
  }
  argv += optind
  argc -= optind

  // Print rules

  typedef map<string, const Rule*> Rules
  rules := state_.bindings_.GetRules()
  for i := rules.begin(); i != rules.end(); i++ {
    printf("%s", i.first)
    if print_description {
      rule := i.second
      const EvalString* description = rule.GetBinding("description")
      if description != nil {
        printf(": %s", description.Unparse())
      }
    }
    printf("\n")
  }
  return 0
}

func (n *NinjaMain) ToolWinCodePage(options *const Options, argc int, argv []*char) int {
  if argc != 0 {
    printf("usage: ninja -t wincodepage\n")
    return 1
  }
  printf("Build file encoding: %s\n", GetACP() == CP_UTF8? "UTF-8" : "ANSI")
  return 0
}

enum PrintCommandMode { PCM_Single, PCM_All }
func PrintCommands(edge *Edge, seen *EdgeSet, mode PrintCommandMode) {
  if edge == nil {
    return
  }
  if !seen.insert(edge).second {
    return
  }

  if mode == PCM_All {
    for in := edge.inputs_.begin(); in != edge.inputs_.end(); in++ {
      PrintCommands((*in).in_edge(), seen, mode)
    }
  }

  if !edge.is_phony() {
    puts(edge.EvaluateCommand())
  }
}

func (n *NinjaMain) ToolCommands(options *const Options, argc int, argv []*char) int {
  // The clean tool uses getopt, and expects argv[0] to contain the name of
  // the tool, i.e. "commands".
  argc++
  argv--

  mode := PCM_All

  optind = 1
  int opt
  while (opt = getopt(argc, argv, const_cast<char*>("hs"))) != -1 {
    switch (opt) {
    case 's':
      mode = PCM_Single
      break
    case 'h':
    default:
      printf("usage: ninja -t commands [options] [targets]\n" "\n" "options:\n" "  -s     only print the final command to build [target], not the whole chain\n" )
    return 1
    }
  }
  argv += optind
  argc -= optind

  vector<Node*> nodes
  string err
  if !CollectTargetsFromArgs(argc, argv, &nodes, &err) {
    Error("%s", err)
    return 1
  }

  EdgeSet seen
  for in := nodes.begin(); in != nodes.end(); in++ {
    PrintCommands((*in).in_edge(), &seen, mode)
  }

  return 0
}

func (n *NinjaMain) ToolClean(options *const Options, argc int, argv []*char) int {
  // The clean tool uses getopt, and expects argv[0] to contain the name of
  // the tool, i.e. "clean".
  argc++
  argv--

  generator := false
  clean_rules := false

  optind = 1
  int opt
  while (opt = getopt(argc, argv, const_cast<char*>("hgr"))) != -1 {
    switch (opt) {
    case 'g':
      generator = true
      break
    case 'r':
      clean_rules = true
      break
    case 'h':
    default:
      printf("usage: ninja -t clean [options] [targets]\n" "\n" "options:\n" "  -g     also clean files marked as ninja generator output\n" "  -r     interpret targets as a list of rules to clean instead\n" )
    return 1
    }
  }
  argv += optind
  argc -= optind

  if clean_rules && argc == 0 {
    Error("expected a rule to clean")
    return 1
  }

  Cleaner cleaner(&state_, config_, &disk_interface_)
  if argc >= 1 {
    if clean_rules {
      return cleaner.CleanRules(argc, argv)
    } else {
      return cleaner.CleanTargets(argc, argv)
    }
  } else {
    return cleaner.CleanAll(generator)
  }
}

func (n *NinjaMain) ToolCleanDead(options *const Options, argc int, argv []*char) int {
  Cleaner cleaner(&state_, config_, &disk_interface_)
  return cleaner.CleanDead(build_log_.entries())
}

enum EvaluateCommandMode {
  ECM_NORMAL,
  ECM_EXPAND_RSPFILE
}
func EvaluateCommandWithRspfile(edge *const Edge, mode EvaluateCommandMode) string {
  command := edge.EvaluateCommand()
  if mode == ECM_NORMAL {
    return command
  }

  rspfile := edge.GetUnescapedRspfile()
  if len(rspfile) == 0 {
    return command
  }

  index := command.find(rspfile)
  if index == 0 || index == string::npos || command[index - 1] != '@' {
    return command
  }

  string rspfile_content = edge.GetBinding("rspfile_content")
  newline_index := 0
  while (newline_index = rspfile_content.find('\n', newline_index)) != string::npos {
    rspfile_content.replace(newline_index, 1, 1, ' ')
    newline_index++
  }
  command.replace(index - 1, rspfile.length() + 1, rspfile_content)
  return command
}

func printCompdb(directory string const, edge Edge* const, eval_mode EvaluateCommandMode) {
  printf("\n  {\n    \"directory\": \"")
  PrintJSONString(directory)
  printf("\",\n    \"command\": \"")
  PrintJSONString(EvaluateCommandWithRspfile(edge, eval_mode))
  printf("\",\n    \"file\": \"")
  PrintJSONString(edge.inputs_[0].path())
  printf("\",\n    \"output\": \"")
  PrintJSONString(edge.outputs_[0].path())
  printf("\"\n  }")
}

func (n *NinjaMain) ToolCompilationDatabase(options *const Options, argc int, argv []*char) int {
  // The compdb tool uses getopt, and expects argv[0] to contain the name of
  // the tool, i.e. "compdb".
  argc++
  argv--

  eval_mode := ECM_NORMAL

  optind = 1
  int opt
  while (opt = getopt(argc, argv, const_cast<char*>("hx"))) != -1 {
    switch(opt) {
      case 'x':
        eval_mode = ECM_EXPAND_RSPFILE
        break

      case 'h':
      default:
        printf( "usage: ninja -t compdb [options] [rules]\n" "\n" "options:\n" "  -x     expand @rspfile style response file invocations\n" )
        return 1
    }
  }
  argv += optind
  argc -= optind

  first := true
  vector<char> cwd
  success := nil

  do {
    cwd.resize(cwd.size() + 1024)
    errno = 0
    success = getcwd(&cwd[0], cwd.size())
  } while (!success && errno == ERANGE)
  if success == nil {
    Error("cannot determine working directory: %s", strerror(errno))
    return 1
  }

  putchar('[')
  for e := state_.edges_.begin(); e != state_.edges_.end(); e++ {
    if (*e).inputs_.empty() {
      continue
    }
    if argc == 0 {
      if first == nil {
        putchar(',')
      }
      printCompdb(&cwd[0], *e, eval_mode)
      first = false
    } else {
      for i := 0; i != argc; i++ {
        if (*e).rule_.name() == argv[i] {
          if first == nil {
            putchar(',')
          }
          printCompdb(&cwd[0], *e, eval_mode)
          first = false
        }
      }
    }
  }

  puts("\n]")
  return 0
}

func (n *NinjaMain) ToolRecompact(options *const Options, argc int, argv []*char) int {
  if !EnsureBuildDirExists() {
    return 1
  }

  if !OpenBuildLog(/*recompact_only=*/true) || !OpenDepsLog(/*recompact_only=*/true) {
    return 1
  }

  return 0
}

func (n *NinjaMain) ToolRestat(options *const Options, argc int, argv []*char) int {
  // The restat tool uses getopt, and expects argv[0] to contain the name of the
  // tool, i.e. "restat"
  argc++
  argv--

  optind = 1
  int opt
  while (opt = getopt(argc, argv, const_cast<char*>("h"))) != -1 {
    switch (opt) {
    case 'h':
    default:
      printf("usage: ninja -t restat [outputs]\n")
      return 1
    }
  }
  argv += optind
  argc -= optind

  if !EnsureBuildDirExists() {
    return 1
  }

  string log_path = ".ninja_log"
  if !build_dir_.empty() {
    log_path = build_dir_ + "/" + log_path
  }

  string err
  status := build_log_.Load(log_path, &err)
  if status == LOAD_ERROR {
    Error("loading build log %s: %s", log_path, err)
    return EXIT_FAILURE
  }
  if status == LOAD_NOT_FOUND {
    // Nothing to restat, ignore this
    return EXIT_SUCCESS
  }
  if len(err) != 0 {
    // Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
    Warning("%s", err)
    err = nil
  }

  success := build_log_.Restat(log_path, disk_interface_, argc, argv, &err)
  if success == nil {
    Error("failed recompaction: %s", err)
    return EXIT_FAILURE
  }

  if !config_.dry_run {
    if !build_log_.OpenForWrite(log_path, *this, &err) {
      Error("opening build log: %s", err)
      return EXIT_FAILURE
    }
  }

  return EXIT_SUCCESS
}

func (n *NinjaMain) ToolUrtle(options *const Options, argc int, argv **char) int {
  // RLE encoded.
  string urtle =
" 13 ,3;2!2;\n8 ,;<11!;\n5 `'<10!(2`'2!\n11 ,6;, `\\. `\\9 .,c13$ec,.\n6 " ",2;11!>; `. ,;!2> .e8$2\".2 \"?7$e.\n <:<8!'` 2.3,.2` ,3!' ;,(?7\";2!2'<" "; `?6$PF ,;,\n2 `'4!8;<!3'`2 3! ;,`'2`2'3!;4!`2.`!;2 3,2 .<!2'`).\n5 3`5" "'2`9 `!2 `4!><3;5! J2$b,`!>;2!:2!`,d?b`!>\n26 `'-;,(<9!> $F3 )3.:!.2 d\"" "2 ) !>\n30 7`2'<3!- \"=-='5 .2 `2-=\",!>\n25 .ze9$er2 .,cd16$bc.'\n22 .e"
"14$,26$.\n21 z45$c .\n20 J50$c\n20 14$P\"`?34$b\n20 14$ dbc `2\"?22$?7$c"
"\n20 ?18$c.6 4\"8?4\" c8$P\n9 .2,.8 \"20$c.3 ._14 J9$\n .2,2c9$bec,.2 `?"
"21$c.3`4%,3%,3 c8$P\"\n22$c2 2\"?21$bc2,.2` .2,c7$P2\",cb\n23$b bc,.2\"2"
"?14$2F2\"5?2\",J5$P\" ,zd3$\n24$ ?$3?%3 `2\"2?12$bcucd3$P3\"2 2=7$\n23$P"
"\" ,3;<5!>2;,. `4\"6?2\"2 ,9;, `\"?2$\n"
  count := 0
  for p := urtle; *p; p++ {
    if '0' <= *p && *p <= '9' {
      count = count*10 + *p - '0'
    } else {
      for i := 0; i < max(count, 1); i++ {
        printf("%c", *p)
      }
      count = 0
    }
  }
  return 0
}

// Find the function to execute for \a tool_name and return it via \a func.
// Returns a Tool, or NULL if Ninja should exit.
const Tool* ChooseTool(string tool_name) {
  static const Tool kTools[] = {
    { "browse", "browse dependency graph in a web browser",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolBrowse },
    { "msvc", "build helper for MSVC cl.exe (EXPERIMENTAL)",
      Tool::RUN_AFTER_FLAGS, &NinjaMain::ToolMSVC },
    { "clean", "clean built files",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolClean },
    { "commands", "list all commands required to rebuild given targets",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolCommands },
    { "deps", "show dependencies stored in the deps log",
      Tool::RUN_AFTER_LOGS, &NinjaMain::ToolDeps },
    { "missingdeps", "check deps log dependencies on generated files",
      Tool::RUN_AFTER_LOGS, &NinjaMain::ToolMissingDeps },
    { "graph", "output graphviz dot file for targets",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolGraph },
    { "query", "show inputs/outputs for a path",
      Tool::RUN_AFTER_LOGS, &NinjaMain::ToolQuery },
    { "targets",  "list targets by their rule or depth in the DAG",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolTargets },
    { "compdb",  "dump JSON compilation database to stdout",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolCompilationDatabase },
    { "recompact",  "recompacts ninja-internal data structures",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolRecompact },
    { "restat",  "restats all outputs in the build log",
      Tool::RUN_AFTER_FLAGS, &NinjaMain::ToolRestat },
    { "rules",  "list all rules",
      Tool::RUN_AFTER_LOAD, &NinjaMain::ToolRules },
    { "cleandead",  "clean built files that are no longer produced by the manifest",
      Tool::RUN_AFTER_LOGS, &NinjaMain::ToolCleanDead },
    { "urtle", nil,
      Tool::RUN_AFTER_FLAGS, &NinjaMain::ToolUrtle },
    { "wincodepage", "print the Windows code page used by ninja",
      Tool::RUN_AFTER_FLAGS, &NinjaMain::ToolWinCodePage },
    { nil, nil, Tool::RUN_AFTER_FLAGS, nil }
  }

  if tool_name == "list" {
    printf("ninja subtools:\n")
    for tool := &kTools[0]; tool.name; tool++ {
      if tool.desc {
        printf("%11s  %s\n", tool.name, tool.desc)
      }
    }
    return nil
  }

  for tool := &kTools[0]; tool.name; tool++ {
    if tool.name == tool_name {
      return tool
    }
  }

  vector<string> words
  for tool := &kTools[0]; tool.name; tool++ {
    words.push_back(tool.name)
  }
  suggestion := SpellcheckStringV(tool_name, words)
  if suggestion != nil {
    Fatal("unknown tool '%s', did you mean '%s'?", tool_name, suggestion)
  } else {
    Fatal("unknown tool '%s'", tool_name)
  }
  return nil  // Not reached.
}

// Enable a debugging mode.  Returns false if Ninja should exit instead
// of continuing.
func DebugEnable(name string) bool {
  if name == "list" {
    printf("debugging modes:\n" "  stats        print operation counts/timing info\n" "  explain      explain what caused a command to execute\n" "  keepdepfile  don't delete depfiles after they're read by ninja\n" "  keeprsp      don't delete @response files on success\n"  "  nostatcache  don't batch stat() calls per directory and cache them\n"  "multiple modes can be enabled via -d FOO -d BAR\n")//#ifdef _WIN32//#endif
    return false
  } else if name == "stats" {
    g_metrics = new Metrics
    return true
  } else if name == "explain" {
    g_explaining = true
    return true
  } else if name == "keepdepfile" {
    g_keep_depfile = true
    return true
  } else if name == "keeprsp" {
    g_keep_rsp = true
    return true
  } else if name == "nostatcache" {
    g_experimental_statcache = false
    return true
  } else {
    string suggestion =
        SpellcheckString(name, "stats", "explain", "keepdepfile", "keeprsp", "nostatcache", nil)
    if suggestion != nil {
      Error("unknown debug setting '%s', did you mean '%s'?", name, suggestion)
    } else {
      Error("unknown debug setting '%s'", name)
    }
    return false
  }
}

// Set a warning flag.  Returns false if Ninja should exit instead of
// continuing.
func WarningEnable(name string, options *Options) bool {
  if name == "list" {
    printf("warning flags:\n" "  phonycycle={err,warn}  phony build statement references itself\n" )
    return false
  } else if name == "dupbuild=err" {
    options.dupe_edges_should_err = true
    return true
  } else if name == "dupbuild=warn" {
    options.dupe_edges_should_err = false
    return true
  } else if name == "phonycycle=err" {
    options.phony_cycle_should_err = true
    return true
  } else if name == "phonycycle=warn" {
    options.phony_cycle_should_err = false
    return true
  } else if name == "depfilemulti=err" || name == "depfilemulti=warn" {
    Warning("deprecated warning 'depfilemulti'")
    return true
  } else {
    string suggestion =
        SpellcheckString(name, "dupbuild=err", "dupbuild=warn", "phonycycle=err", "phonycycle=warn", nil)
    if suggestion != nil {
      Error("unknown warning flag '%s', did you mean '%s'?", name, suggestion)
    } else {
      Error("unknown warning flag '%s'", name)
    }
    return false
  }
}

// Open the build log.
// @return false on error.
func (n *NinjaMain) OpenBuildLog(recompact_only bool) bool {
  string log_path = ".ninja_log"
  if !build_dir_.empty() {
    log_path = build_dir_ + "/" + log_path
  }

  string err
  status := build_log_.Load(log_path, &err)
  if status == LOAD_ERROR {
    Error("loading build log %s: %s", log_path, err)
    return false
  }
  if len(err) != 0 {
    // Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
    Warning("%s", err)
    err = nil
  }

  if recompact_only {
    if status == LOAD_NOT_FOUND {
      return true
    }
    success := build_log_.Recompact(log_path, *this, &err)
    if success == nil {
      Error("failed recompaction: %s", err)
    }
    return success
  }

  if !config_.dry_run {
    if !build_log_.OpenForWrite(log_path, *this, &err) {
      Error("opening build log: %s", err)
      return false
    }
  }

  return true
}

// Open the deps log: load it, then open for writing.
// @return false on error.
// Open the deps log: load it, then open for writing.
// @return false on error.
func (n *NinjaMain) OpenDepsLog(recompact_only bool) bool {
  string path = ".ninja_deps"
  if !build_dir_.empty() {
    path = build_dir_ + "/" + path
  }

  string err
  status := deps_log_.Load(path, &state_, &err)
  if status == LOAD_ERROR {
    Error("loading deps log %s: %s", path, err)
    return false
  }
  if len(err) != 0 {
    // Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
    Warning("%s", err)
    err = nil
  }

  if recompact_only {
    if status == LOAD_NOT_FOUND {
      return true
    }
    success := deps_log_.Recompact(path, &err)
    if success == nil {
      Error("failed recompaction: %s", err)
    }
    return success
  }

  if !config_.dry_run {
    if !deps_log_.OpenForWrite(path, &err) {
      Error("opening deps log: %s", err)
      return false
    }
  }

  return true
}

// Dump the output requested by '-d stats'.
func (n *NinjaMain) DumpMetrics() {
  g_metrics.Report()

  printf("\n")
  count := (int)state_.paths_.size()
  buckets := (int)state_.paths_.bucket_count()
  printf("path.node hash load %.2f (%d entries / %d buckets)\n", count / (double) buckets, count, buckets)
}

// Ensure the build directory exists, creating it if necessary.
// @return false on error.
func (n *NinjaMain) EnsureBuildDirExists() bool {
  build_dir_ = state_.bindings_.LookupVariable("builddir")
  if !build_dir_.empty() && !config_.dry_run {
    if !disk_interface_.MakeDirs(build_dir_ + "/.") && errno != EEXIST {
      Error("creating build directory %s: %s", build_dir_, strerror(errno))
      return false
    }
  }
  return true
}

// Build the targets listed on the command line.
// @return an exit code.
func (n *NinjaMain) RunBuild(argc int, argv **char, status *Status) int {
  string err
  vector<Node*> targets
  if !CollectTargetsFromArgs(argc, argv, &targets, &err) {
    status.Error("%s", err)
    return 1
  }

  disk_interface_.AllowStatCache(g_experimental_statcache)

  Builder builder(&state_, config_, &build_log_, &deps_log_, &disk_interface_, status, start_time_millis_)
  for i := 0; i < targets.size(); i++ {
    if !builder.AddTarget(targets[i], &err) {
      if len(err) != 0 {
        status.Error("%s", err)
        return 1
      } else {
        // Added a target that is already up-to-date; not really
        // an error.
      }
    }
  }

  // Make sure restat rules do not see stale timestamps.
  disk_interface_.AllowStatCache(false)

  if builder.AlreadyUpToDate() {
    status.Info("no work to do.")
    return 0
  }

  if !builder.Build(&err) {
    status.Info("build stopped: %s.", err)
    if err.find("interrupted by user") != string::npos {
      return 2
    }
    return 1
  }

  return 0
}

// This handler processes fatal crashes that you can't catch
// Test example: C++ exception in a stack-unwind-block
// Real-world example: ninja launched a compiler to process a tricky
// C++ input file. The compiler got itself into a state where it
// generated 3 GB of output and caused ninja to crash.
func TerminateHandler() {
  CreateWin32MiniDump(nil)
  Fatal("terminate handler called")
}

// On Windows, we want to prevent error dialogs in case of exceptions.
// This function handles the exception, and writes a minidump.
func ExceptionFilter(code unsigned int, ep *struct _EXCEPTION_POINTERS) int {
  Error("exception: 0x%X", code)  // e.g. EXCEPTION_ACCESS_VIOLATION
  fflush(stderr)
  CreateWin32MiniDump(ep)
  return EXCEPTION_EXECUTE_HANDLER
}

// Parse argv for command-line options.
// Returns an exit code, or -1 if Ninja should continue.
func ReadFlags(argc *int, argv ***char, options *Options, config *BuildConfig) int {
  config.parallelism = GuessParallelism()

  enum { OPT_VERSION = 1, OPT_QUIET = 2 }
  const option kLongOptions[] = {
    { "help", no_argument, nil, 'h' },
    { "version", no_argument, nil, OPT_VERSION },
    { "verbose", no_argument, nil, 'v' },
    { "quiet", no_argument, nil, OPT_QUIET },
    { nil, 0, nil, 0 }
  }

  int opt
  while !options.tool && (opt = getopt_long(*argc, *argv, "d:f:j:k:l:nt:vw:C:h", kLongOptions, nil)) != -1 {
    switch (opt) {
      case 'd':
        if !DebugEnable(optarg) {
          return 1
        }
        break
      case 'f':
        options.input_file = optarg
        break
      case 'j': {
        char* end
        value := strtol(optarg, &end, 10)
        if *end != 0 || value < 0 {
          Fatal("invalid -j parameter")
        }

        // We want to run N jobs in parallel. For N = 0, INT_MAX
        // is close enough to infinite for most sane builds.
        config.parallelism = value > 0 ? value : INT_MAX
        break
      }
      case 'k': {
        char* end
        value := strtol(optarg, &end, 10)
        if *end != 0 {
          Fatal("-k parameter not numeric; did you mean -k 0?")
        }

        // We want to go until N jobs fail, which means we should allow
        // N failures and then stop.  For N <= 0, INT_MAX is close enough
        // to infinite for most sane builds.
        config.failures_allowed = value > 0 ? value : INT_MAX
        break
      }
      case 'l': {
        char* end
        value := strtod(optarg, &end)
        if end == optarg {
          Fatal("-l parameter not numeric: did you mean -l 0.0?")
        }
        config.max_load_average = value
        break
      }
      case 'n':
        config.dry_run = true
        break
      case 't':
        options.tool = ChooseTool(optarg)
        if !options.tool {
          return 0
        }
        break
      case 'v':
        config.verbosity = BuildConfig::VERBOSE
        break
      case OPT_QUIET:
        config.verbosity = BuildConfig::NO_STATUS_UPDATE
        break
      case 'w':
        if !WarningEnable(optarg, options) {
          return 1
        }
        break
      case 'C':
        options.working_dir = optarg
        break
      case OPT_VERSION:
        printf("%s\n", kNinjaVersion)
        return 0
      case 'h':
      default:
        Usage(*config)
        return 1
    }
  }
  *argv += optind
  *argc -= optind

  return -1
}

NORETURN void real_main(int argc, char** argv) {
  // Use exit() instead of return in this function to avoid potentially
  // expensive cleanup when destructing NinjaMain.
  BuildConfig config
  Options options = {}
  options.input_file = "build.ninja"
  options.dupe_edges_should_err = true

  setvbuf(stdout, nil, _IOLBF, BUFSIZ)
  ninja_command := argv[0]

  exit_code := ReadFlags(&argc, &argv, &options, &config)
  if exit_code >= 0 {
    exit(exit_code)
  }

  status := new StatusPrinter(config)

  if options.working_dir {
    // The formatting of this string, complete with funny quotes, is
    // so Emacs can properly identify that the cwd has changed for
    // subsequent commands.
    // Don't print this if a tool is being used, so that tool output
    // can be piped into a file without this string showing up.
    if !options.tool && config.verbosity != BuildConfig::NO_STATUS_UPDATE {
      status.Info("Entering directory `%s'", options.working_dir)
    }
    if chdir(options.working_dir) < 0 {
      Fatal("chdir to '%s' - %s", options.working_dir, strerror(errno))
    }
  }

  if options.tool && options.tool.when == Tool::RUN_AFTER_FLAGS {
    // None of the RUN_AFTER_FLAGS actually use a NinjaMain, but it's needed
    // by other tools.
    NinjaMain ninja(ninja_command, config)
    exit((ninja.*options.tool.func)(&options, argc, argv))
  }

  // It'd be nice to use line buffering but MSDN says: "For some systems,
  // [_IOLBF] provides line buffering. However, for Win32, the behavior is the
  //  same as _IOFBF - Full Buffering."
  // Buffering used to be disabled in the LinePrinter constructor but that
  // now disables it too early and breaks -t deps performance (see issue #2018)
  // so we disable it here instead, but only when not running a tool.
  if !options.tool {
    setvbuf(stdout, nil, _IONBF, 0)
  }

  // Limit number of rebuilds, to prevent infinite loops.
  kCycleLimit := 100
  for cycle := 1; cycle <= kCycleLimit; cycle++ {
    NinjaMain ninja(ninja_command, config)

    ManifestParserOptions parser_opts
    if options.dupe_edges_should_err {
      parser_opts.dupe_edge_action_ = kDupeEdgeActionError
    }
    if options.phony_cycle_should_err {
      parser_opts.phony_cycle_action_ = kPhonyCycleActionError
    }
    ManifestParser parser(&ninja.state_, &ninja.disk_interface_, parser_opts)
    string err
    if !parser.Load(options.input_file, &err) {
      status.Error("%s", err)
      exit(1)
    }

    if options.tool && options.tool.when == Tool::RUN_AFTER_LOAD {
      exit((ninja.*options.tool.func)(&options, argc, argv))
    }

    if !ninja.EnsureBuildDirExists() {
      exit(1)
    }

    if !ninja.OpenBuildLog() || !ninja.OpenDepsLog() {
      exit(1)
    }

    if options.tool && options.tool.when == Tool::RUN_AFTER_LOGS {
      exit((ninja.*options.tool.func)(&options, argc, argv))
    }

    // Attempt to rebuild the manifest before building anything else
    if ninja.RebuildManifest(options.input_file, &err, status) {
      // In dry_run mode the regeneration will succeed without changing the
      // manifest forever. Better to return immediately.
      if config.dry_run {
        exit(0)
      }
      // Start the build over with the new manifest.
      continue
    } else if len(err) != 0 {
      status.Error("rebuilding '%s': %s", options.input_file, err)
      exit(1)
    }

    result := ninja.RunBuild(argc, argv, status)
    if g_metrics {
      ninja.DumpMetrics()
    }
    exit(result)
  }

  status.Error("manifest '%s' still dirty after %d tries", options.input_file, kCycleLimit)
  exit(1)
}

func main(argc int, argv **char) int {
  // Set a handler to catch crashes not caught by the __try..__except
  // block (e.g. an exception in a stack-unwind-block).
  set_terminate(TerminateHandler)
  __try {
    // Running inside __try ... __except suppresses any Windows error
    // dialogs for errors such as bad_alloc.
    real_main(argc, argv)
  }
  __except(ExceptionFilter(GetExceptionCode(), GetExceptionInformation())) {
    // Common error situations return exitCode=1. 2 was chosen to
    // indicate a more serious problem.
    return 2
  }
  real_main(argc, argv)
}

