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

// TODO(maruel): Move to cmd/nin/main.go once the package is importable.

package nin

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
)

// Command-line options.
type options struct {
	// Build file to load.
	input_file string

	// Directory to change into before running.
	working_dir string

	// tool to run rather than building.
	tool *tool

	// Whether duplicate rules for one target should warn or print an error.
	dupe_edges_should_err bool

	// Whether phony cycles should warn or print an error.
	phony_cycle_should_err bool

	cpuprofile string
	memprofile string
	trace      string
}

// The Ninja main() loads up a series of data structures; various tools need
// to poke into these, so store them as fields on an object.
type ninjaMain struct {
	// Command line used to run Ninja.
	ninja_command_ string

	// Build configuration set from flags (e.g. parallelism).
	config_ *BuildConfig

	// Loaded state (rules, nodes).
	state_ State

	// Functions for accessing the disk.
	disk_interface_ RealDiskInterface

	// The build directory, used for storing the build log etc.
	build_dir_ string

	build_log_ BuildLog
	deps_log_  DepsLog

	// The type of functions that are the entry points to tools (subcommands).

	start_time_millis_ int64
}

func newNinjaMain(ninja_command string, config *BuildConfig) ninjaMain {
	return ninjaMain{
		ninja_command_:     ninja_command,
		config_:            config,
		state_:             NewState(),
		build_log_:         NewBuildLog(),
		deps_log_:          NewDepsLog(),
		start_time_millis_: GetTimeMillis(),
	}
}

func (n *ninjaMain) Close() error {
	// TODO(maruel): Ensure the file handle is cleanly closed.
	err1 := n.deps_log_.Close()
	err2 := n.build_log_.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type toolFunc func(*ninjaMain, *options, []string) int

func (n *ninjaMain) IsPathDead(s string) bool {
	nd := n.state_.LookupNode(s)
	if nd != nil && nd.InEdge != nil {
		return false
	}
	// Just checking nd isn't enough: If an old output is both in the build log
	// and in the deps log, it will have a Node object in state_.  (It will also
	// have an in edge if one of its inputs is another output that's in the deps
	// log, but having a deps edge product an output that's input to another deps
	// edge is rare, and the first recompaction will delete all old outputs from
	// the deps log, and then a second recompaction will clear the build log,
	// which seems good enough for this corner case.)
	// Do keep entries around for files which still exist on disk, for
	// generators that want to use this information.
	err := ""
	mtime := n.disk_interface_.Stat(s, &err)
	if mtime == -1 {
		errorf("%s", err) // Log and ignore Stat() errors.
	}
	return mtime == 0
}

// Subtools, accessible via "-t foo".
type tool struct {
	// Short name of the tool.
	name string

	// Description (shown in "-t list").
	desc string

	when when

	// Implementation of the tool.
	tool toolFunc
}

// when to run the tool.
type when int32

const (
	// Run after parsing the command-line flags and potentially changing
	// the current working directory (as early as possible).
	runAfterFlags when = iota

	// Run after loading build.ninja.
	runAfterLoad

	// Run after loading the build/deps logs.
	runAfterLogs
)

// Print usage information.
func usage() {
	fmt.Fprintf(os.Stderr, "usage: nin [options] [targets...]\n\n")
	fmt.Fprintf(os.Stderr, "if targets are unspecified, builds the 'default' target (see manual).\n\n")
	flag.PrintDefaults()
}

// Choose a default value for the -j (parallelism) flag.
func guessParallelism() int {
	switch processors := runtime.NumCPU(); processors {
	case 0, 1:
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
func (n *ninjaMain) RebuildManifest(input_file string, err *string, status Status) bool {
	path := input_file
	if len(path) == 0 {
		*err = "empty path"
		return false
	}
	node := n.state_.LookupNode(CanonicalizePath(path))
	if node == nil {
		return false
	}

	builder := NewBuilder(&n.state_, n.config_, &n.build_log_, &n.deps_log_, &n.disk_interface_, status, n.start_time_millis_)
	if !builder.AddTarget(node, err) {
		return false
	}

	if builder.AlreadyUpToDate() {
		return false // Not an error, but we didn't rebuild.
	}

	if !builder.Build(err) {
		return false
	}

	// The manifest was only rebuilt if it is now dirty (it may have been cleaned
	// by a restat).
	if !node.Dirty {
		// Reset the state to prevent problems like
		// https://github.com/ninja-build/ninja/issues/874
		n.state_.Reset()
		return false
	}

	return true
}

// Get the Node for a given command-line path, handling features like
// spell correction.
func (n *ninjaMain) CollectTarget(cpath string, err *string) *Node {
	path := cpath
	if len(path) == 0 {
		*err = "empty path"
		return nil
	}
	path, slashBits := CanonicalizePathBits(path)

	// Special syntax: "foo.cc^" means "the first output of foo.cc".
	first_dependent := false
	if path != "" && path[len(path)-1] == '^' {
		path = path[:len(path)-1]
		first_dependent = true
	}

	node := n.state_.LookupNode(path)
	if node != nil {
		if first_dependent {
			if len(node.OutEdges) == 0 {
				rev_deps := n.deps_log_.GetFirstReverseDepsNode(node)
				if rev_deps == nil {
					*err = "'" + path + "' has no out edge"
					return nil
				}
				node = rev_deps
			} else {
				edge := node.OutEdges[0]
				if len(edge.Outputs) == 0 {
					edge.Dump("")
					fatalf("edge has no outputs")
				}
				node = edge.Outputs[0]
			}
		}
		return node
	}
	*err = "unknown target '" + PathDecanonicalized(path, slashBits) + "'"
	if path == "clean" {
		*err += ", did you mean 'nin -t clean'?"
	} else if path == "help" {
		*err += ", did you mean 'nin -h'?"
	} else {
		suggestion := n.state_.SpellcheckNode(path)
		if suggestion != nil {
			*err += ", did you mean '" + suggestion.Path + "'?"
		}
	}
	return nil
}

// CollectTarget for all command-line arguments, filling in \a targets.
func (n *ninjaMain) CollectTargetsFromArgs(args []string, targets *[]*Node, err *string) bool {
	if len(args) == 0 {
		*targets = n.state_.DefaultNodes(err)
		return *err == ""
	}

	for i := 0; i < len(args); i++ {
		node := n.CollectTarget(args[i], err)
		if node == nil {
			return false
		}
		*targets = append(*targets, node)
	}
	return true
}

// The various subcommands, run via "-t XXX".
func toolGraph(n *ninjaMain, opts *options, args []string) int {
	var nodes []*Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}

	graph := NewGraphViz(&n.state_, &n.disk_interface_)
	graph.Start()
	for _, n := range nodes {
		graph.AddTarget(n)
	}
	graph.Finish()
	return 0
}

func toolQuery(n *ninjaMain, opts *options, args []string) int {
	if len(args) == 0 {
		errorf("expected a target to query")
		return 1
	}

	dyndep_loader := NewDyndepLoader(&n.state_, &n.disk_interface_)

	for i := 0; i < len(args); i++ {
		err := ""
		node := n.CollectTarget(args[i], &err)
		if node == nil {
			errorf("%s", err)
			return 1
		}

		fmt.Printf("%s:\n", node.Path)
		if edge := node.InEdge; edge != nil {
			if edge.Dyndep != nil && edge.Dyndep.DyndepPending {
				if !dyndep_loader.LoadDyndeps(edge.Dyndep, DyndepFile{}, &err) {
					warningf("%s\n", err)
				}
			}
			fmt.Printf("  input: %s\n", edge.Rule.name())
			for in := 0; in < len(edge.Inputs); in++ {
				label := ""
				if edge.is_implicit(in) {
					label = "| "
				} else if edge.is_order_only(in) {
					label = "|| "
				}
				fmt.Printf("    %s%s\n", label, edge.Inputs[in].Path)
			}
			if len(edge.Validations) != 0 {
				fmt.Printf("  validations:\n")
				for _, validation := range edge.Validations {
					fmt.Printf("    %s\n", validation.Path)
				}
			}
		}
		fmt.Printf("  outputs:\n")
		for _, edge := range node.OutEdges {
			for _, out := range edge.Outputs {
				fmt.Printf("    %s\n", out.Path)
			}
		}
		validation_edges := node.ValidationOutEdges
		if len(validation_edges) != 0 {
			fmt.Printf("  validation for:\n")
			for _, edge := range validation_edges {
				for _, out := range edge.Outputs {
					fmt.Printf("    %s\n", out.Path)
				}
			}
		}
	}
	return 0
}

func toolBrowse(n *ninjaMain, opts *options, args []string) int {
	RunBrowsePython(&n.state_, n.ninja_command_, opts.input_file, args)
	return 0
}

/* Only defined on Windows in C++.
func  toolMSVC(n *ninjaMain,opts *options, args []string) int {
	// Reset getopt: push one argument onto the front of argv, reset optind.
	//argc++
	//argv--
	//optind = 0
	return MSVCHelperMain(args)
}
*/

func toolTargetsListNodes(nodes []*Node, depth int, indent int) int {
	for _, n := range nodes {
		for i := 0; i < indent; i++ {
			fmt.Printf("  ")
		}
		target := n.Path
		if n.InEdge != nil {
			fmt.Printf("%s: %s\n", target, n.InEdge.Rule.name())
			if depth > 1 || depth <= 0 {
				toolTargetsListNodes(n.InEdge.Inputs, depth-1, indent+1)
			}
		} else {
			fmt.Printf("%s\n", target)
		}
	}
	return 0
}

func toolTargetsSourceList(state *State) int {
	for _, e := range state.edges_ {
		for _, inps := range e.Inputs {
			if inps.InEdge == nil {
				fmt.Printf("%s\n", inps.Path)
			}
		}
	}
	return 0
}

func toolTargetsListRule(state *State, rule_name string) int {
	rules := map[string]struct{}{}

	// Gather the outputs.
	for _, e := range state.edges_ {
		if e.Rule.name() == rule_name {
			for _, out_node := range e.Outputs {
				rules[out_node.Path] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(rules))
	for n := range rules {
		names = append(names, n)
	}
	sort.Strings(names)
	// Print them.
	for _, i := range names {
		fmt.Printf("%s\n", i)
	}
	return 0
}

func toolTargetsList(state *State) int {
	for _, e := range state.edges_ {
		for _, out_node := range e.Outputs {
			fmt.Printf("%s: %s\n", out_node.Path, e.Rule.name())
		}
	}
	return 0
}

func toolDeps(n *ninjaMain, opts *options, args []string) int {
	var nodes []*Node
	if len(args) == 0 {
		for _, ni := range n.deps_log_.nodes() {
			if n.deps_log_.IsDepsEntryLiveFor(ni) {
				nodes = append(nodes, ni)
			}
		}
	} else {
		err := ""
		if !n.CollectTargetsFromArgs(args, &nodes, &err) {
			errorf("%s", err)
			return 1
		}
	}

	disk_interface := NewRealDiskInterface()
	for _, it := range nodes {
		deps := n.deps_log_.GetDeps(it)
		if deps == nil {
			fmt.Printf("%s: deps not found\n", it.Path)
			continue
		}

		err := ""
		mtime := disk_interface.Stat(it.Path, &err)
		if mtime == -1 {
			errorf("%s", err) // Log and ignore Stat() errors;
		}
		s := "VALID"
		if mtime == 0 || mtime > deps.mtime {
			s = "STALE"
		}
		fmt.Printf("%s: #deps %d, deps mtime %d (%s)\n", it.Path, deps.node_count, deps.mtime, s)
		for i := 0; i < deps.node_count; i++ {
			fmt.Printf("    %s\n", deps.nodes[i].Path)
		}
		fmt.Printf("\n")
	}
	return 0
}

func toolMissingDeps(n *ninjaMain, opts *options, args []string) int {
	var nodes []*Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}
	disk_interface := NewRealDiskInterface()
	printer := MissingDependencyPrinter{}
	scanner := NewMissingDependencyScanner(&printer, &n.deps_log_, &n.state_, &disk_interface)
	for _, it := range nodes {
		scanner.ProcessNode(it)
	}
	scanner.PrintStats()
	if scanner.HadMissingDeps() {
		return 3
	}
	return 0
}

func toolTargets(n *ninjaMain, opts *options, args []string) int {
	depth := 1
	if len(args) >= 1 {
		mode := args[0]
		if mode == "rule" {
			rule := ""
			if len(args) > 1 {
				rule = args[1]
			}
			if len(rule) == 0 {
				return toolTargetsSourceList(&n.state_)
			}
			return toolTargetsListRule(&n.state_, rule)
		}
		if mode == "depth" {
			if len(args) > 1 {
				// TODO(maruel): Handle error.
				depth, _ = strconv.Atoi(args[1])
			}
		} else if mode == "all" {
			return toolTargetsList(&n.state_)
		} else {
			suggestion := SpellcheckString(mode, "rule", "depth", "all")
			if suggestion != "" {
				errorf("unknown target tool mode '%s', did you mean '%s'?", mode, suggestion)
			} else {
				errorf("unknown target tool mode '%s'", mode)
			}
			return 1
		}
	}

	err := ""
	root_nodes := n.state_.RootNodes(&err)
	if len(err) == 0 {
		return toolTargetsListNodes(root_nodes, depth, 0)
	}
	errorf("%s", err)
	return 1
}

func toolRules(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse one additional flag.
	//fmt.Printf("usage: nin -t rules [options]\n\noptions:\n  -d     also print the description of the rule\n  -h     print this message\n")
	print_description := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-d" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			print_description = true
		}
	}

	rules := n.state_.bindings_.GetRules()
	names := make([]string, 0, len(rules))
	for n := range rules {
		names = append(names, n)
	}
	sort.Strings(names)

	// Print rules
	for _, name := range names {
		fmt.Printf("%s", name)
		if print_description {
			rule := rules[name]
			description := rule.GetBinding("description")
			if description != nil {
				fmt.Printf(": %s", description.Unparse())
			}
		}
		fmt.Printf("\n")
	}
	return 0
}

func toolWinCodePage(n *ninjaMain, opts *options, args []string) int {
	panic("TODO") // Windows only
	/*
		if len(args) != 0 {
			fmt.Printf("usage: nin -t wincodepage\n")
			return 1
		}
		cp := "ANSI"
		if GetACP() == CP_UTF8 {
			cp = "UTF-8"
		}
		fmt.Printf("Build file encoding: %s\n", cp)
		return 0
	*/
}

type printCommandMode bool

const (
	pcmSingle printCommandMode = false
	pcmAll    printCommandMode = true
)

func printCommands(edge *Edge, seen *EdgeSet, mode printCommandMode) {
	if edge == nil {
		return
	}
	if _, ok := seen.edges[edge]; ok {
		return
	}
	seen.Add(edge)

	if mode == pcmAll {
		for _, in := range edge.Inputs {
			printCommands(in.InEdge, seen, mode)
		}
	}

	if edge.Rule != PhonyRule {
		fmt.Printf("%s\n", (edge.EvaluateCommand(false)))
	}
}

func toolCommands(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse one additional flag.
	//fmt.Printf("usage: nin -t commands [options] [targets]\n\noptions:\n  -s     only print the final command to build [target], not the whole chain\n")
	mode := pcmAll
	for i := 0; i < len(args); i++ {
		if args[i] == "-s" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			mode = pcmSingle
		}
	}

	var nodes []*Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}

	seen := NewEdgeSet()
	for _, in := range nodes {
		printCommands(in.InEdge, seen, mode)
	}
	return 0
}

func toolClean(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse two additional flags.
	// fmt.Printf("usage: nin -t clean [options] [targets]\n\noptions:\n  -g     also clean files marked as ninja generator output\n  -r     interpret targets as a list of rules to clean instead\n" )
	generator := false
	clean_rules := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-g" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			generator = true
		} else if args[i] == "-r" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			clean_rules = true
		}
	}

	if clean_rules && len(args) == 0 {
		errorf("expected a rule to clean")
		return 1
	}

	cleaner := NewCleaner(&n.state_, n.config_, &n.disk_interface_)
	if len(args) >= 1 {
		if clean_rules {
			return cleaner.CleanRules(args)
		}
		return cleaner.CleanTargets(args)
	}
	return cleaner.CleanAll(generator)
}

func toolCleanDead(n *ninjaMain, opts *options, args []string) int {
	cleaner := NewCleaner(&n.state_, n.config_, &n.disk_interface_)
	return cleaner.CleanDead(n.build_log_.entries())
}

type evaluateCommandMode bool

const (
	ecmNormal        evaluateCommandMode = false
	ecmExpandRSPFile evaluateCommandMode = true
)

func evaluateCommandWithRspfile(edge *Edge, mode evaluateCommandMode) string {
	command := edge.EvaluateCommand(false)
	if mode == ecmNormal {
		return command
	}

	rspfile := edge.GetUnescapedRspfile()
	if len(rspfile) == 0 {
		return command
	}

	index := strings.Index(command, rspfile)
	if index == 0 || index == -1 || command[index-1] != '@' {
		return command
	}

	panic("TODO")
	/*
			rspfile_content := edge.GetBinding("rspfile_content")
		  newline_index := 0
		  for (newline_index = rspfile_content.find('\n', newline_index)) != string::npos {
		    rspfile_content.replace(newline_index, 1, 1, ' ')
		    newline_index++
		  }
		  command.replace(index - 1, rspfile.length() + 1, rspfile_content)
		  return command
	*/
}

func printCompdb(directory string, edge *Edge, eval_mode evaluateCommandMode) {
	fmt.Printf("\n  {\n    \"directory\": \"")
	PrintJSONString(directory)
	fmt.Printf("\",\n    \"command\": \"")
	PrintJSONString(evaluateCommandWithRspfile(edge, eval_mode))
	fmt.Printf("\",\n    \"file\": \"")
	PrintJSONString(edge.Inputs[0].Path)
	fmt.Printf("\",\n    \"output\": \"")
	PrintJSONString(edge.Outputs[0].Path)
	fmt.Printf("\"\n  }")
}

func toolCompilationDatabase(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse one additional flag.
	// fmt.Printf( "usage: nin -t compdb [options] [rules]\n\noptions:\n  -x     expand @rspfile style response file invocations\n" )
	eval_mode := ecmNormal
	for i := 0; i < len(args); i++ {
		if args[i] == "-x" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			eval_mode = ecmExpandRSPFile
		}
	}

	first := true
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Printf("[")
	for _, e := range n.state_.edges_ {
		if len(e.Inputs) == 0 {
			continue
		}
		if len(args) == 0 {
			if !first {
				fmt.Printf(",")
			}
			printCompdb(cwd, e, eval_mode)
			first = false
		} else {
			for i := 0; i != len(args); i++ {
				if e.Rule.name() == args[i] {
					if !first {
						fmt.Printf(",")
					}
					printCompdb(cwd, e, eval_mode)
					first = false
				}
			}
		}
	}

	fmt.Printf("\n]")
	return 0
}

func toolRecompact(n *ninjaMain, opts *options, args []string) int {
	if !n.EnsureBuildDirExists() {
		return 1
	}

	// recompact_only
	if !n.OpenBuildLog(true) || !n.OpenDepsLog(true) {
		return 1
	}

	return 0
}

func toolRestat(n *ninjaMain, opts *options, args []string) int {
	if !n.EnsureBuildDirExists() {
		return 1
	}

	log_path := ".ninja_log"
	if n.build_dir_ != "" {
		log_path = filepath.Join(n.build_dir_, log_path)
	}

	err := ""
	status := n.build_log_.Load(log_path, &err)
	if status == LOAD_ERROR {
		errorf("loading build log %s: %s", log_path, err)
		return ExitFailure
	}
	if status == LOAD_NOT_FOUND {
		// Nothing to restat, ignore this
		return ExitSuccess
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if !n.build_log_.Restat(log_path, &n.disk_interface_, args, &err) {
		errorf("failed recompaction: %s", err)
		return ExitFailure
	}

	if !n.config_.dry_run {
		if !n.build_log_.OpenForWrite(log_path, n, &err) {
			errorf("opening build log: %s", err)
			return ExitFailure
		}
	}

	return ExitSuccess
}

// Find the function to execute for \a tool_name and return it via \a func.
// Returns a Tool, or NULL if Ninja should exit.
func chooseTool(tool_name string) *tool {
	tools := []*tool{
		{"browse", "browse dependency graph in a web browser", runAfterLoad, toolBrowse},
		//{"msvc", "build helper for MSVC cl.exe (EXPERIMENTAL)",runAfterFlags, toolMSVC},
		{"clean", "clean built files", runAfterLoad, toolClean},
		{"commands", "list all commands required to rebuild given targets", runAfterLoad, toolCommands},
		{"deps", "show dependencies stored in the deps log", runAfterLogs, toolDeps},
		{"missingdeps", "check deps log dependencies on generated files", runAfterLogs, toolMissingDeps},
		{"graph", "output graphviz dot file for targets", runAfterLoad, toolGraph},
		{"query", "show inputs/outputs for a path", runAfterLogs, toolQuery},
		{"targets", "list targets by their rule or depth in the DAG", runAfterLoad, toolTargets},
		{"compdb", "dump JSON compilation database to stdout", runAfterLoad, toolCompilationDatabase},
		{"recompact", "recompacts ninja-internal data structures", runAfterLoad, toolRecompact},
		{"restat", "restats all outputs in the build log", runAfterFlags, toolRestat},
		{"rules", "list all rules", runAfterLoad, toolRules},
		{"cleandead", "clean built files that are no longer produced by the manifest", runAfterLogs, toolCleanDead},
		//{"wincodepage", "print the Windows code page used by nin", runAfterFlags, toolWinCodePage},
	}
	if tool_name == "list" {
		fmt.Printf("nin subtools:\n")
		for _, t := range tools {
			if t.desc != "" {
				fmt.Printf("%11s  %s\n", t.name, t.desc)
			}
		}
		return nil
	}

	for _, t := range tools {
		if t.name == tool_name {
			return t
		}
	}

	var words []string
	for _, t := range tools {
		words = append(words, t.name)
	}
	suggestion := SpellcheckString(tool_name, words...)
	if suggestion != "" {
		fatalf("unknown tool '%s', did you mean '%s'?", tool_name, suggestion)
	} else {
		fatalf("unknown tool '%s'", tool_name)
	}
	return nil // Not reached.
}

// Enable a debugging mode.  Returns false if Ninja should exit instead
// of continuing.
func debugEnable(name string) bool {
	if name == "list" {
		fmt.Printf("debugging modes:\n  stats        print operation counts/timing info\n  explain      explain what caused a command to execute\n  keepdepfile  don't delete depfiles after they're read by ninja\n  keeprsp      don't delete @response files on success\n  nostatcache  don't batch stat() calls per directory and cache them\nmultiple modes can be enabled via -d FOO -d BAR\n")
		//#ifdef _WIN32//#endif
		return false
	} else if name == "stats" {
		g_metrics = NewMetrics()
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
		suggestion := SpellcheckString(name, "stats", "explain", "keepdepfile", "keeprsp", "nostatcache")
		if suggestion != "" {
			errorf("unknown debug setting '%s', did you mean '%s'?", name, suggestion)
		} else {
			errorf("unknown debug setting '%s'", name)
		}
		return false
	}
}

// Set a warning flag.  Returns false if Ninja should exit instead of
// continuing.
func warningEnable(name string, opts *options) bool {
	if name == "list" {
		fmt.Printf("warning flags:\n  phonycycle={err,warn}  phony build statement references itself\n")
		return false
	} else if name == "dupbuild=err" {
		opts.dupe_edges_should_err = true
		return true
	} else if name == "dupbuild=warn" {
		opts.dupe_edges_should_err = false
		return true
	} else if name == "phonycycle=err" {
		opts.phony_cycle_should_err = true
		return true
	} else if name == "phonycycle=warn" {
		opts.phony_cycle_should_err = false
		return true
	} else if name == "depfilemulti=err" || name == "depfilemulti=warn" {
		warningf("deprecated warning 'depfilemulti'")
		return true
	} else {
		suggestion := SpellcheckString(name, "dupbuild=err", "dupbuild=warn", "phonycycle=err", "phonycycle=warn")
		if suggestion != "" {
			errorf("unknown warning flag '%s', did you mean '%s'?", name, suggestion)
		} else {
			errorf("unknown warning flag '%s'", name)
		}
		return false
	}
}

// Open the build log.
// @return false on error.
func (n *ninjaMain) OpenBuildLog(recompact_only bool) bool {
	log_path := ".ninja_log"
	if n.build_dir_ != "" {
		log_path = n.build_dir_ + "/" + log_path
	}

	err := ""
	status := n.build_log_.Load(log_path, &err)
	if status == LOAD_ERROR {
		errorf("loading build log %s: %s", log_path, err)
		return false
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if recompact_only {
		if status == LOAD_NOT_FOUND {
			return true
		}
		success := n.build_log_.Recompact(log_path, n, &err)
		if !success {
			errorf("failed recompaction: %s", err)
		}
		return success
	}

	if !n.config_.dry_run {
		if !n.build_log_.OpenForWrite(log_path, n, &err) {
			errorf("opening build log: %s", err)
			return false
		}
	}

	return true
}

// Open the deps log: load it, then open for writing.
// @return false on error.
// Open the deps log: load it, then open for writing.
// @return false on error.
func (n *ninjaMain) OpenDepsLog(recompact_only bool) bool {
	path := ".ninja_deps"
	if n.build_dir_ != "" {
		path = n.build_dir_ + "/" + path
	}

	err := ""
	status := n.deps_log_.Load(path, &n.state_, &err)
	if status == LOAD_ERROR {
		errorf("loading deps log %s: %s", path, err)
		return false
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if recompact_only {
		if status == LOAD_NOT_FOUND {
			return true
		}
		success := n.deps_log_.Recompact(path, &err)
		if !success {
			errorf("failed recompaction: %s", err)
		}
		return success
	}

	if !n.config_.dry_run {
		if !n.deps_log_.OpenForWrite(path, &err) {
			errorf("opening deps log: %s", err)
			return false
		}
	}

	return true
}

// Dump the output requested by '-d stats'.
func (n *ninjaMain) DumpMetrics() {
	g_metrics.Report()

	fmt.Printf("\n")
	// There's no such concept in Go's map.
	//count := len(n.state_.paths_)
	//buckets := len(n.state_.paths_)
	//fmt.Printf("path.node hash load %.2f (%d entries / %d buckets)\n", count/float64(buckets), count, buckets)
}

// Ensure the build directory exists, creating it if necessary.
// @return false on error.
func (n *ninjaMain) EnsureBuildDirExists() bool {
	n.build_dir_ = n.state_.bindings_.LookupVariable("builddir")
	if n.build_dir_ != "" && !n.config_.dry_run {
		// TODO(maruel): We need real error.
		if !MakeDirs(&n.disk_interface_, filepath.Join(n.build_dir_, ".")) {
			errorf("creating build directory %s", n.build_dir_)
			//return false
		}
	}
	return true
}

// Build the targets listed on the command line.
// @return an exit code.
func (n *ninjaMain) RunBuild(args []string, status Status) int {
	err := ""
	var targets []*Node
	if !n.CollectTargetsFromArgs(args, &targets, &err) {
		status.Error("%s", err)
		return 1
	}

	n.disk_interface_.AllowStatCache(g_experimental_statcache)

	builder := NewBuilder(&n.state_, n.config_, &n.build_log_, &n.deps_log_, &n.disk_interface_, status, n.start_time_millis_)
	for i := 0; i < len(targets); i++ {
		if !builder.AddTarget(targets[i], &err) {
			if len(err) != 0 {
				status.Error("%s", err)
				return 1
			}
			// Added a target that is already up-to-date; not really
			// an error.
		}
	}

	// Make sure restat rules do not see stale timestamps.
	n.disk_interface_.AllowStatCache(false)

	if builder.AlreadyUpToDate() {
		status.Info("no work to do.")
		return 0
	}

	if !builder.Build(&err) {
		status.Info("build stopped: %s.", err)
		if strings.Contains(err, "interrupted by user") {
			return 2
		}
		return 1
	}
	return 0
}

/*
// This handler processes fatal crashes that you can't catch
// Test example: C++ exception in a stack-unwind-block
// Real-world example: ninja launched a compiler to process a tricky
// C++ input file. The compiler got itself into a state where it
// generated 3 GB of output and caused ninja to crash.
func terminateHandler() {
  CreateWin32MiniDump(nil)
  Fatal("terminate handler called")
}

// On Windows, we want to prevent error dialogs in case of exceptions.
// This function handles the exception, and writes a minidump.
func exceptionFilter(code unsigned int, ep *struct _EXCEPTION_POINTERS) int {
  Error("exception: 0x%X", code)  // e.g. EXCEPTION_ACCESS_VIOLATION
  fflush(stderr)
  CreateWin32MiniDump(ep)
  return EXCEPTION_EXECUTE_HANDLER
}
*/

// Parse args for command-line options.
// Returns an exit code, or -1 if Ninja should continue.
func readFlags(opts *options, config *BuildConfig) int {
	// TODO(maruel): For now just do something simple to get started but we'll
	// have to make it custom if we want it to be drop-in replacement.
	// It's funny how "opts" and "config" is a bit mixed up here.
	flag.StringVar(&opts.input_file, "f", "build.ninja", "specify input build file")
	flag.StringVar(&opts.working_dir, "C", "", "change to DIR before doing anything else")
	opts.dupe_edges_should_err = true
	flag.StringVar(&opts.cpuprofile, "cpuprofile", "", "activate the CPU sampling profiler")
	flag.StringVar(&opts.memprofile, "memprofile", "", "snapshot a heap dump at the end")
	flag.StringVar(&opts.trace, "trace", "", "capture a runtime trace")

	flag.IntVar(&config.parallelism, "j", guessParallelism(), "run N jobs in parallel (0 means infinity)")
	flag.IntVar(&config.failures_allowed, "k", 1, "keep going until N jobs fail (0 means infinity)")
	flag.Float64Var(&config.max_load_average, "l", 0, "do not start new jobs if the load average is greater than N")
	flag.BoolVar(&config.dry_run, "n", false, "dry run (don't run commands but act like they succeeded)")

	// TODO(maruel): terminates toplevel options; further flags are passed to the tool
	t := flag.String("t", "", "run a subtool (use '-t list' to list subtools)")
	// TODO(maruel): It's supposed to be accumulative.
	dbgEnable := flag.String("d", "", "enable debugging (use '-d list' to list modes)")
	verbose := flag.Bool("v", false, "show all command lines while building")
	flag.BoolVar(verbose, "verbose", false, "show all command lines while building")
	quiet := flag.Bool("quiet", false, "don't show progress status, just command output")
	warning := flag.String("w", "", "adjust warnings (use '-w list' to list warnings)")
	version := flag.Bool("version", false, fmt.Sprintf("print nin version (%q)", kNinjaVersion))

	flag.Usage = usage
	flag.Parse()

	if *verbose && *quiet {
		fmt.Fprintf(os.Stderr, "can't use both -v and --quiet\n")
		return 2
	}
	if *verbose {
		config.verbosity = VERBOSE
	}
	if *quiet {
		config.verbosity = NO_STATUS_UPDATE
	}
	if *warning != "" {
		if !warningEnable(*warning, opts) {
			return 1
		}
	}
	if *dbgEnable != "" {
		if !debugEnable(*dbgEnable) {
			return 1
		}
	}
	if *version {
		fmt.Printf("%s\n", kNinjaVersion)
		return 0
	}
	if *t != "" {
		opts.tool = chooseTool(*t)
		if opts.tool == nil {
			return 0
		}
	}
	i := 0
	if opts.cpuprofile != "" {
		i++
	}
	if opts.memprofile != "" {
		i++
	}
	if opts.trace != "" {
		i++
	}
	if i > 1 {
		fmt.Fprintf(os.Stderr, "can only use one of -cpuprofile, -memprofile or -trace at a time.\n")
		return 2
	}

	/*
		OPT_VERSION := 1
		OPT_QUIET := 2
			   option kLongOptions[] = {
			     { "help", no_argument, nil, 'h' },
			     { "version", no_argument, nil, OPT_VERSION },
			     { "verbose", no_argument, nil, 'v' },
			     { "quiet", no_argument, nil, OPT_QUIET },
			     { nil, 0, nil, 0 }
			   }

			   for opts.tool ==nil {
					 opt := getopt_long(*argc, *argv, "d:f:j:k:l:nt:vw:C:h", kLongOptions, nil))
					 if opt == -1 {
						 continue
					 }
			     switch opt {
			       case 'd':
			         if !debugEnable(optarg) {
			           return 1
			         }
			         break
			       case 'f':
			         opts.input_file = optarg
			         break
			       case 'j': {
			         var end *char
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
			         var end *char
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
			         var end *char
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
			         opts.tool = chooseTool(optarg)
			         if !opts.tool {
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
			         if !warningEnable(optarg, opts) {
			           return 1
			         }
			         break
			       case 'C':
			         opts.working_dir = optarg
			         break
			       case OPT_VERSION:
			         fmt.Printf("%s\n", kNinjaVersion)
			         return 0
			       case 'h':
			       default:
			         usage()
			         return 1
			     }
			   }
			   *argv += optind
			   *argc -= optind
	*/
	return -1
}

func Main() int {
	// Use exit() instead of return in this function to avoid potentially
	// expensive cleanup when destructing ninjaMain.
	config := NewBuildConfig()
	opts := options{}

	//setvbuf(stdout, nil, _IOLBF, BUFSIZ)
	ninja_command := os.Args[0]
	exit_code := readFlags(&opts, &config)
	if exit_code >= 0 {
		return exit_code
	}
	// TODO(maruel): Handle os.Interrupt and cancel the context cleanly.

	// Disable GC (TODO: unless running a stateful server).
	debug.SetGCPercent(-1)

	if opts.cpuprofile != "" {
		f, err := os.Create(opts.cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if opts.memprofile != "" {
		// Take all memory allocation. This significantly slows down the process.
		runtime.MemProfileRate = 1
		defer func() {
			f, err := os.Create(opts.memprofile)
			if err != nil {
				log.Fatal("could not create memory profile: ", err)
			}
			defer f.Close()
			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}
		}()
	} else {
		// No need.
		runtime.MemProfileRate = 0
	}
	if opts.trace != "" {
		f, err := os.Create(opts.trace)
		if err != nil {
			log.Fatal("could not create trace: ", err)
		}
		defer f.Close()
		// TODO(maruel): Use regions.
		if err := trace.Start(f); err != nil {
			log.Fatal("could not start trace: ", err)
		}
		defer trace.Stop()
	}

	args := flag.Args()

	status := NewStatusPrinter(&config)
	if opts.working_dir != "" {
		// The formatting of this string, complete with funny quotes, is
		// so Emacs can properly identify that the cwd has changed for
		// subsequent commands.
		// Don't print this if a tool is being used, so that tool output
		// can be piped into a file without this string showing up.
		if opts.tool == nil && config.verbosity != NO_STATUS_UPDATE {
			status.Info("Entering directory `%s'", opts.working_dir)
		}
		if err := os.Chdir(opts.working_dir); err != nil {
			fatalf("chdir to '%s' - %s", opts.working_dir, err)
		}
	}

	if opts.tool != nil && opts.tool.when == runAfterFlags {
		// None of the runAfterFlags actually use a ninjaMain, but it's needed
		// by other tools.
		ninja := newNinjaMain(ninja_command, &config)
		return opts.tool.tool(&ninja, &opts, args)
	}

	// TODO(maruel): Let's wrap stdout/stderr with our own buffer?

	/*
	  // It'd be nice to use line buffering but MSDN says: "For some systems,
	  // [_IOLBF] provides line buffering. However, for Win32, the behavior is the
	  //  same as _IOFBF - Full Buffering."
	  // Buffering used to be disabled in the LinePrinter constructor but that
	  // now disables it too early and breaks -t deps performance (see issue #2018)
	  // so we disable it here instead, but only when not running a tool.
	  if !opts.tool {
	    setvbuf(stdout, nil, _IONBF, 0)
	  }
	*/
	// Limit number of rebuilds, to prevent infinite loops.
	kCycleLimit := 100
	for cycle := 1; cycle <= kCycleLimit; cycle++ {
		ninja := newNinjaMain(ninja_command, &config)

		var parser_opts ManifestParserOptions
		if opts.dupe_edges_should_err {
			parser_opts.dupe_edge_action_ = kDupeEdgeActionError
		}
		if opts.phony_cycle_should_err {
			parser_opts.phony_cycle_action_ = kPhonyCycleActionError
		}
		parser := NewManifestParser(&ninja.state_, &ninja.disk_interface_, parser_opts)
		err := ""
		if !parser.Load(opts.input_file, &err, nil) {
			status.Error("%s", err)
			return 1
		}

		if opts.tool != nil && opts.tool.when == runAfterLoad {
			return opts.tool.tool(&ninja, &opts, args)
		}

		if !ninja.EnsureBuildDirExists() {
			return 1
		}

		if !ninja.OpenBuildLog(false) || !ninja.OpenDepsLog(false) {
			return 1
		}

		if opts.tool != nil && opts.tool.when == runAfterLogs {
			return opts.tool.tool(&ninja, &opts, args)
		}

		// Attempt to rebuild the manifest before building anything else
		if ninja.RebuildManifest(opts.input_file, &err, status) {
			// In dry_run mode the regeneration will succeed without changing the
			// manifest forever. Better to return immediately.
			if config.dry_run {
				return 0
			}
			// Start the build over with the new manifest.
			continue
		} else if len(err) != 0 {
			status.Error("rebuilding '%s': %s", opts.input_file, err)
			return 1
		}

		result := ninja.RunBuild(args, status)
		if g_metrics != nil {
			ninja.DumpMetrics()
		}
		return result
	}

	status.Error("manifest '%s' still dirty after %d tries", opts.input_file, kCycleLimit)
	return 1
}
