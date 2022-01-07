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

package main

import (
	"errors"
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

	"github.com/maruel/nin"
)

// Command-line options.
type options struct {
	// Build file to load.
	inputFile string

	// Directory to change into before running.
	workingDir string

	// tool to run rather than building.
	tool *tool

	// build.ninja parsing options.
	parserOpts nin.ManifestParserOptions

	cpuprofile string
	memprofile string
	trace      string
}

// The Ninja main() loads up a series of data structures; various tools need
// to poke into these, so store them as fields on an object.
type ninjaMain struct {
	// Command line used to run Ninja.
	ninjaCommand string

	// Build configuration set from flags (e.g. parallelism).
	config *nin.BuildConfig

	// Loaded state (rules, nodes).
	state nin.State

	// Functions for accessing the disk.
	di nin.RealDiskInterface

	// The build directory, used for storing the build log etc.
	buildDir string

	buildLog nin.BuildLog
	depsLog  nin.DepsLog

	// The type of functions that are the entry points to tools (subcommands).

	startTimeMillis int64
}

func newNinjaMain(ninjaCommand string, config *nin.BuildConfig) ninjaMain {
	return ninjaMain{
		ninjaCommand:    ninjaCommand,
		config:          config,
		state:           nin.NewState(),
		buildLog:        nin.NewBuildLog(),
		startTimeMillis: nin.GetTimeMillis(),
	}
}

func (n *ninjaMain) Close() error {
	// TODO(maruel): Ensure the file handle is cleanly closed.
	err1 := n.depsLog.Close()
	err2 := n.buildLog.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type toolFunc func(*ninjaMain, *options, []string) int

func (n *ninjaMain) IsPathDead(s string) bool {
	nd := n.state.Paths[s]
	if nd != nil && nd.InEdge != nil {
		return false
	}
	// Just checking nd isn't enough: If an old output is both in the build log
	// and in the deps log, it will have a Node object in state.  (It will also
	// have an in edge if one of its inputs is another output that's in the deps
	// log, but having a deps edge product an output that's input to another deps
	// edge is rare, and the first recompaction will delete all old outputs from
	// the deps log, and then a second recompaction will clear the build log,
	// which seems good enough for this corner case.)
	// Do keep entries around for files which still exist on disk, for
	// generators that want to use this information.
	mtime, err := n.di.Stat(s)
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
func (n *ninjaMain) RebuildManifest(inputFile string, status nin.Status) (bool, error) {
	path := inputFile
	if len(path) == 0 {
		return false, errors.New("empty path")
	}
	node := n.state.Paths[nin.CanonicalizePath(path)]
	if node == nil {
		return false, errors.New("path not found")
	}

	builder := nin.NewBuilder(&n.state, n.config, &n.buildLog, &n.depsLog, &n.di, status, n.startTimeMillis)
	err2 := ""
	if !builder.AddTarget(node, &err2) {
		return false, errors.New(err2)
	}

	if builder.AlreadyUpToDate() {
		return false, nil // Not an error, but we didn't rebuild.
	}

	if err := builder.Build(); err != nil {
		return false, err
	}

	// The manifest was only rebuilt if it is now dirty (it may have been cleaned
	// by a restat).
	if !node.Dirty {
		// Reset the state to prevent problems like
		// https://github.com/ninja-build/ninja/issues/874
		n.state.Reset()
		return false, nil
	}

	return true, nil
}

// Get the Node for a given command-line path, handling features like
// spell correction.
func (n *ninjaMain) CollectTarget(cpath string, err *string) *nin.Node {
	path := cpath
	if len(path) == 0 {
		*err = "empty path"
		return nil
	}
	path, slashBits := nin.CanonicalizePathBits(path)

	// Special syntax: "foo.cc^" means "the first output of foo.cc".
	firstDependent := false
	if path != "" && path[len(path)-1] == '^' {
		path = path[:len(path)-1]
		firstDependent = true
	}

	node := n.state.Paths[path]
	if node != nil {
		if firstDependent {
			if len(node.OutEdges) == 0 {
				revDeps := n.depsLog.GetFirstReverseDepsNode(node)
				if revDeps == nil {
					*err = "'" + path + "' has no out edge"
					return nil
				}
				node = revDeps
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
	*err = "unknown target '" + nin.PathDecanonicalized(path, slashBits) + "'"
	if path == "clean" {
		*err += ", did you mean 'nin -t clean'?"
	} else if path == "help" {
		*err += ", did you mean 'nin -h'?"
	} else {
		suggestion := n.state.SpellcheckNode(path)
		if suggestion != nil {
			*err += ", did you mean '" + suggestion.Path + "'?"
		}
	}
	return nil
}

// CollectTarget for all command-line arguments, filling in \a targets.
func (n *ninjaMain) CollectTargetsFromArgs(args []string, targets *[]*nin.Node, err *string) bool {
	if len(args) == 0 {
		*targets = n.state.DefaultNodes()
		if len(*targets) == 0 {
			*err = "could not determine root nodes of build graph"
		}
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
	var nodes []*nin.Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}

	graph := nin.NewGraphViz(&n.state, &n.di)
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

	dyndepLoader := nin.NewDyndepLoader(&n.state, &n.di)

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
				if err := dyndepLoader.LoadDyndeps(edge.Dyndep, nin.DyndepFile{}); err != nil {
					warningf("%s\n", err)
				}
			}
			fmt.Printf("  input: %s\n", edge.Rule.Name)
			for in := 0; in < len(edge.Inputs); in++ {
				label := ""
				if edge.IsImplicit(in) {
					label = "| "
				} else if edge.IsOrderOnly(in) {
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
		validationEdges := node.ValidationOutEdges
		if len(validationEdges) != 0 {
			fmt.Printf("  validation for:\n")
			for _, edge := range validationEdges {
				for _, out := range edge.Outputs {
					fmt.Printf("    %s\n", out.Path)
				}
			}
		}
	}
	return 0
}

func toolBrowse(n *ninjaMain, opts *options, args []string) int {
	runBrowsePython(&n.state, n.ninjaCommand, opts.inputFile, args)
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

func toolTargetsListNodes(nodes []*nin.Node, depth int, indent int) int {
	for _, n := range nodes {
		for i := 0; i < indent; i++ {
			fmt.Printf("  ")
		}
		target := n.Path
		if n.InEdge != nil {
			fmt.Printf("%s: %s\n", target, n.InEdge.Rule.Name)
			if depth > 1 || depth <= 0 {
				toolTargetsListNodes(n.InEdge.Inputs, depth-1, indent+1)
			}
		} else {
			fmt.Printf("%s\n", target)
		}
	}
	return 0
}

func toolTargetsSourceList(state *nin.State) int {
	for _, e := range state.Edges {
		for _, inps := range e.Inputs {
			if inps.InEdge == nil {
				fmt.Printf("%s\n", inps.Path)
			}
		}
	}
	return 0
}

func toolTargetsListRule(state *nin.State, ruleName string) int {
	rules := map[string]struct{}{}

	// Gather the outputs.
	for _, e := range state.Edges {
		if e.Rule.Name == ruleName {
			for _, outNode := range e.Outputs {
				rules[outNode.Path] = struct{}{}
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

func toolTargetsList(state *nin.State) int {
	for _, e := range state.Edges {
		for _, outNode := range e.Outputs {
			fmt.Printf("%s: %s\n", outNode.Path, e.Rule.Name)
		}
	}
	return 0
}

func toolDeps(n *ninjaMain, opts *options, args []string) int {
	var nodes []*nin.Node
	if len(args) == 0 {
		for _, ni := range n.depsLog.Nodes {
			if n.depsLog.IsDepsEntryLiveFor(ni) {
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

	di := nin.NewRealDiskInterface()
	for _, it := range nodes {
		deps := n.depsLog.GetDeps(it)
		if deps == nil {
			fmt.Printf("%s: deps not found\n", it.Path)
			continue
		}

		mtime, err := di.Stat(it.Path)
		if mtime == -1 {
			errorf("%s", err) // Log and ignore Stat() errors;
		}
		s := "VALID"
		if mtime == 0 || mtime > deps.MTime {
			s = "STALE"
		}
		fmt.Printf("%s: #deps %d, deps mtime %d (%s)\n", it.Path, len(deps.Nodes), deps.MTime, s)
		for _, n := range deps.Nodes {
			fmt.Printf("    %s\n", n.Path)
		}
		fmt.Printf("\n")
	}
	return 0
}

func toolMissingDeps(n *ninjaMain, opts *options, args []string) int {
	var nodes []*nin.Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}
	di := nin.NewRealDiskInterface()
	printer := MissingDependencyPrinter{}
	scanner := nin.NewMissingDependencyScanner(&printer, &n.depsLog, &n.state, &di)
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
				return toolTargetsSourceList(&n.state)
			}
			return toolTargetsListRule(&n.state, rule)
		}
		if mode == "depth" {
			if len(args) > 1 {
				// TODO(maruel): Handle error.
				depth, _ = strconv.Atoi(args[1])
			}
		} else if mode == "all" {
			return toolTargetsList(&n.state)
		} else {
			suggestion := nin.SpellcheckString(mode, "rule", "depth", "all")
			if suggestion != "" {
				errorf("unknown target tool mode '%s', did you mean '%s'?", mode, suggestion)
			} else {
				errorf("unknown target tool mode '%s'", mode)
			}
			return 1
		}
	}

	if rootNodes := n.state.RootNodes(); len(rootNodes) != 0 {
		return toolTargetsListNodes(rootNodes, depth, 0)
	}
	errorf("could not determine root nodes of build graph")
	return 1
}

func toolRules(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse one additional flag.
	//fmt.Printf("usage: nin -t rules [options]\n\noptions:\n  -d     also print the description of the rule\n  -h     print this message\n")
	printDescription := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-d" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			printDescription = true
		}
	}

	rules := n.state.Bindings.Rules
	names := make([]string, 0, len(rules))
	for n := range rules {
		names = append(names, n)
	}
	sort.Strings(names)

	// Print rules
	for _, name := range names {
		fmt.Printf("%s", name)
		if printDescription {
			rule := rules[name]
			description := rule.Bindings["description"]
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

func printCommands(edge *nin.Edge, seen map[*nin.Edge]struct{}, mode printCommandMode) {
	if edge == nil {
		return
	}
	if _, ok := seen[edge]; ok {
		return
	}
	seen[edge] = struct{}{}

	if mode == pcmAll {
		for _, in := range edge.Inputs {
			printCommands(in.InEdge, seen, mode)
		}
	}

	if edge.Rule != nin.PhonyRule {
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

	var nodes []*nin.Node
	err := ""
	if !n.CollectTargetsFromArgs(args, &nodes, &err) {
		errorf("%s", err)
		return 1
	}

	seen := map[*nin.Edge]struct{}{}
	for _, in := range nodes {
		printCommands(in.InEdge, seen, mode)
	}
	return 0
}

func toolClean(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse two additional flags.
	// fmt.Printf("usage: nin -t clean [options] [targets]\n\noptions:\n  -g     also clean files marked as ninja generator output\n  -r     interpret targets as a list of rules to clean instead\n" )
	generator := false
	cleanRules := false
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
			cleanRules = true
		}
	}

	if cleanRules && len(args) == 0 {
		errorf("expected a rule to clean")
		return 1
	}

	cleaner := nin.NewCleaner(&n.state, n.config, &n.di)
	if len(args) >= 1 {
		if cleanRules {
			return cleaner.CleanRules(args)
		}
		return cleaner.CleanTargets(args)
	}
	return cleaner.CleanAll(generator)
}

func toolCleanDead(n *ninjaMain, opts *options, args []string) int {
	cleaner := nin.NewCleaner(&n.state, n.config, &n.di)
	return cleaner.CleanDead(n.buildLog.Entries)
}

type evaluateCommandMode bool

const (
	ecmNormal        evaluateCommandMode = false
	ecmExpandRSPFile evaluateCommandMode = true
)

func evaluateCommandWithRspfile(edge *nin.Edge, mode evaluateCommandMode) string {
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
			rspfileContent := edge.GetBinding("rspfile_content")
		  newlineIndex := 0
		  for (newlineIndex = rspfileContent.find('\n', newlineIndex)) != string::npos {
		    rspfileContent.replace(newlineIndex, 1, 1, ' ')
		    newlineIndex++
		  }
		  command.replace(index - 1, rspfile.length() + 1, rspfileContent)
		  return command
	*/
}

func printCompdb(directory string, edge *nin.Edge, evalMode evaluateCommandMode) {
	fmt.Printf("\n  {\n    \"directory\": \"")
	printJSONString(directory)
	fmt.Printf("\",\n    \"command\": \"")
	printJSONString(evaluateCommandWithRspfile(edge, evalMode))
	fmt.Printf("\",\n    \"file\": \"")
	printJSONString(edge.Inputs[0].Path)
	fmt.Printf("\",\n    \"output\": \"")
	printJSONString(edge.Outputs[0].Path)
	fmt.Printf("\"\n  }")
}

func toolCompilationDatabase(n *ninjaMain, opts *options, args []string) int {
	// HACK: parse one additional flag.
	// fmt.Printf( "usage: nin -t compdb [options] [rules]\n\noptions:\n  -x     expand @rspfile style response file invocations\n" )
	evalMode := ecmNormal
	for i := 0; i < len(args); i++ {
		if args[i] == "-x" {
			if i != len(args)-1 {
				copy(args[i:], args[i+1:])
				args = args[:len(args)-1]
			}
			evalMode = ecmExpandRSPFile
		}
	}

	first := true
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Printf("[")
	for _, e := range n.state.Edges {
		if len(e.Inputs) == 0 {
			continue
		}
		if len(args) == 0 {
			if !first {
				fmt.Printf(",")
			}
			printCompdb(cwd, e, evalMode)
			first = false
		} else {
			for i := 0; i != len(args); i++ {
				if e.Rule.Name == args[i] {
					if !first {
						fmt.Printf(",")
					}
					printCompdb(cwd, e, evalMode)
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

	// recompactOnly
	if !n.OpenBuildLog(true) || !n.OpenDepsLog(true) {
		return 1
	}

	return 0
}

func toolRestat(n *ninjaMain, opts *options, args []string) int {
	if !n.EnsureBuildDirExists() {
		return 1
	}

	logPath := ".ninja_log"
	if n.buildDir != "" {
		logPath = filepath.Join(n.buildDir, logPath)
	}

	err := ""
	status := n.buildLog.Load(logPath, &err)
	if status == nin.LoadError {
		errorf("loading build log %s: %s", logPath, err)
		return nin.ExitFailure
	}
	if status == nin.LoadNotFound {
		// Nothing to restat, ignore this
		return nin.ExitSuccess
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if !n.buildLog.Restat(logPath, &n.di, args, &err) {
		errorf("failed recompaction: %s", err)
		return nin.ExitFailure
	}

	if !n.config.DryRun {
		if !n.buildLog.OpenForWrite(logPath, n, &err) {
			errorf("opening build log: %s", err)
			return nin.ExitFailure
		}
	}

	return nin.ExitSuccess
}

// Find the function to execute for \a toolName and return it via \a func.
// Returns a Tool, or NULL if Ninja should exit.
func chooseTool(toolName string) *tool {
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
	if toolName == "list" {
		fmt.Printf("nin subtools:\n")
		for _, t := range tools {
			if t.desc != "" {
				fmt.Printf("%11s  %s\n", t.name, t.desc)
			}
		}
		return nil
	}

	for _, t := range tools {
		if t.name == toolName {
			return t
		}
	}

	var words []string
	for _, t := range tools {
		words = append(words, t.name)
	}
	suggestion := nin.SpellcheckString(toolName, words...)
	if suggestion != "" {
		fatalf("unknown tool '%s', did you mean '%s'?", toolName, suggestion)
	} else {
		fatalf("unknown tool '%s'", toolName)
	}
	return nil // Not reached.
}

var (
	disableExperimentalStatcache bool
	metricsEnabled               bool
)

// Enable a debugging mode.  Returns false if Ninja should exit instead
// of continuing.
func debugEnable(name string) bool {
	if name == "list" {
		fmt.Printf("debugging modes:\n  stats        print operation counts/timing info\n  explain      explain what caused a command to execute\n  keepdepfile  don't delete depfiles after they're read by ninja\n  keeprsp      don't delete @response files on success\n  nostatcache  don't batch stat() calls per directory and cache them\nmultiple modes can be enabled via -d FOO -d BAR\n")
		//#ifdef _WIN32//#endif
		return false
	} else if name == "stats" {
		metricsEnabled = true
		nin.Metrics.Enable()
		return true
	} else if name == "explain" {
		nin.Debug.Explaining = true
		return true
	} else if name == "keepdepfile" {
		nin.Debug.KeepDepfile = true
		return true
	} else if name == "keeprsp" {
		nin.Debug.KeepRsp = true
		return true
	} else if name == "nostatcache" {
		disableExperimentalStatcache = true
		return true
	} else {
		suggestion := nin.SpellcheckString(name, "stats", "explain", "keepdepfile", "keeprsp", "nostatcache")
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
		opts.parserOpts.ErrOnDupeEdge = true
		return true
	} else if name == "dupbuild=warn" {
		opts.parserOpts.ErrOnDupeEdge = false
		return true
	} else if name == "phonycycle=err" {
		opts.parserOpts.ErrOnPhonyCycle = true
		return true
	} else if name == "phonycycle=warn" {
		opts.parserOpts.ErrOnPhonyCycle = false
		return true
	} else if name == "depfilemulti=err" || name == "depfilemulti=warn" {
		warningf("deprecated warning 'depfilemulti'")
		return true
	} else {
		suggestion := nin.SpellcheckString(name, "dupbuild=err", "dupbuild=warn", "phonycycle=err", "phonycycle=warn")
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
func (n *ninjaMain) OpenBuildLog(recompactOnly bool) bool {
	logPath := ".ninja_log"
	if n.buildDir != "" {
		logPath = n.buildDir + "/" + logPath
	}

	err := ""
	status := n.buildLog.Load(logPath, &err)
	if status == nin.LoadError {
		errorf("loading build log %s: %s", logPath, err)
		return false
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if recompactOnly {
		if status == nin.LoadNotFound {
			return true
		}
		success := n.buildLog.Recompact(logPath, n, &err)
		if !success {
			errorf("failed recompaction: %s", err)
		}
		return success
	}

	if !n.config.DryRun {
		if !n.buildLog.OpenForWrite(logPath, n, &err) {
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
func (n *ninjaMain) OpenDepsLog(recompactOnly bool) bool {
	path := ".ninja_deps"
	if n.buildDir != "" {
		path = n.buildDir + "/" + path
	}

	err := ""
	status := n.depsLog.Load(path, &n.state, &err)
	if status == nin.LoadError {
		errorf("loading deps log %s: %s", path, err)
		return false
	}
	if len(err) != 0 {
		// Hack: Load() can return a warning via err by returning LOAD_SUCCESS.
		warningf("%s", err)
		err = ""
	}

	if recompactOnly {
		if status == nin.LoadNotFound {
			return true
		}
		success := n.depsLog.Recompact(path, &err)
		if !success {
			errorf("failed recompaction: %s", err)
		}
		return success
	}

	if !n.config.DryRun {
		if !n.depsLog.OpenForWrite(path, &err) {
			errorf("opening deps log: %s", err)
			return false
		}
	}

	return true
}

// Dump the output requested by '-d stats'.
func (n *ninjaMain) DumpMetrics() {
	nin.Metrics.Report()

	fmt.Printf("\n")
	// There's no such concept in Go's map.
	//count := len(n.state.paths)
	//buckets := len(n.state.paths)
	//fmt.Printf("path.node hash load %.2f (%d entries / %d buckets)\n", count/float64(buckets), count, buckets)
}

// Ensure the build directory exists, creating it if necessary.
// @return false on error.
func (n *ninjaMain) EnsureBuildDirExists() bool {
	n.buildDir = n.state.Bindings.LookupVariable("builddir")
	if n.buildDir != "" && !n.config.DryRun {
		if err := nin.MakeDirs(&n.di, filepath.Join(n.buildDir, ".")); err != nil {
			errorf("creating build directory %s", n.buildDir)
			return false
		}
	}
	return true
}

// Build the targets listed on the command line.
// @return an exit code.
func (n *ninjaMain) RunBuild(args []string, status nin.Status) int {
	err := ""
	var targets []*nin.Node
	if !n.CollectTargetsFromArgs(args, &targets, &err) {
		status.Error("%s", err)
		return 1
	}

	n.di.AllowStatCache(!disableExperimentalStatcache)

	builder := nin.NewBuilder(&n.state, n.config, &n.buildLog, &n.depsLog, &n.di, status, n.startTimeMillis)
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
	n.di.AllowStatCache(false)

	if builder.AlreadyUpToDate() {
		status.Info("no work to do.")
		return 0
	}

	if err := builder.Build(); err != nil {
		status.Info("build stopped: %s.", err)
		if strings.Contains(err.Error(), "interrupted by user") {
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
func readFlags(opts *options, config *nin.BuildConfig) int {
	// TODO(maruel): For now just do something simple to get started but we'll
	// have to make it custom if we want it to be drop-in replacement.
	// It's funny how "opts" and "config" is a bit mixed up here.
	flag.StringVar(&opts.inputFile, "f", "build.ninja", "specify input build file")
	flag.StringVar(&opts.workingDir, "C", "", "change to DIR before doing anything else")
	opts.parserOpts.ErrOnDupeEdge = true
	flag.StringVar(&opts.cpuprofile, "cpuprofile", "", "activate the CPU sampling profiler")
	flag.StringVar(&opts.memprofile, "memprofile", "", "snapshot a heap dump at the end")
	flag.StringVar(&opts.trace, "trace", "", "capture a runtime trace")

	flag.IntVar(&config.Parallelism, "j", guessParallelism(), "run N jobs in parallel (0 means infinity)")
	flag.IntVar(&config.FailuresAllowed, "k", 1, "keep going until N jobs fail (0 means infinity)")
	flag.Float64Var(&config.MaxLoadAvg, "l", 0, "do not start new jobs if the load average is greater than N")
	flag.BoolVar(&config.DryRun, "n", false, "dry run (don't run commands but act like they succeeded)")

	// TODO(maruel): terminates toplevel options; further flags are passed to the tool
	t := flag.String("t", "", "run a subtool (use '-t list' to list subtools)")
	// TODO(maruel): It's supposed to be accumulative.
	dbgEnable := flag.String("d", "", "enable debugging (use '-d list' to list modes)")
	verbose := flag.Bool("v", false, "show all command lines while building")
	flag.BoolVar(verbose, "verbose", false, "show all command lines while building")
	quiet := flag.Bool("quiet", false, "don't show progress status, just command output")
	warning := flag.String("w", "", "adjust warnings (use '-w list' to list warnings)")
	version := flag.Bool("version", false, fmt.Sprintf("print nin version (%q)", nin.NinjaVersion))

	flag.Usage = usage
	flag.Parse()

	if *verbose && *quiet {
		fmt.Fprintf(os.Stderr, "can't use both -v and --quiet\n")
		return 2
	}
	if *verbose {
		config.Verbosity = nin.Verbose
	}
	if *quiet {
		config.Verbosity = nin.NoStatusUpdate
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
		fmt.Printf("%s\n", nin.NinjaVersion)
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
			   option longOptions[] = {
			     { "help", noArgument, nil, 'h' },
			     { "version", noArgument, nil, OPT_VERSION },
			     { "verbose", noArgument, nil, 'v' },
			     { "quiet", noArgument, nil, OPT_QUIET },
			     { nil, 0, nil, 0 }
			   }

			   for opts.tool ==nil {
					 opt := getoptLong(*argc, *argv, "d:f:j:k:l:nt:vw:C:h", longOptions, nil))
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
			         opts.inputFile = optarg
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
			         config.failuresAllowed = value > 0 ? value : INT_MAX
			         break
			       }
			       case 'l': {
			         var end *char
			         value := strtod(optarg, &end)
			         if end == optarg {
			           Fatal("-l parameter not numeric: did you mean -l 0.0?")
			         }
			         config.maxLoadAverage = value
			         break
			       }
			       case 'n':
			         config.dryRun = true
			         break
			       case 't':
			         opts.tool = chooseTool(optarg)
			         if !opts.tool {
			           return 0
			         }
			         break
			       case 'v':
			         config.verbosity = Verbose
			         break
			       case OPT_QUIET:
			         config.verbosity = nin.NoStatusUpdate
			         break
			       case 'w':
			         if !warningEnable(optarg, opts) {
			           return 1
			         }
			         break
			       case 'C':
			         opts.workingDir = optarg
			         break
			       case OPT_VERSION:
			         fmt.Printf("%s\n", nin.NinjaVersion)
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
	config := nin.NewBuildConfig()
	opts := options{}

	//setvbuf(stdout, nil, _IOLBF, BUFSIZ)
	ninjaCommand := os.Args[0]
	exitCode := readFlags(&opts, &config)
	if exitCode >= 0 {
		return exitCode
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
	if opts.workingDir != "" {
		// The formatting of this string, complete with funny quotes, is
		// so Emacs can properly identify that the cwd has changed for
		// subsequent commands.
		// Don't print this if a tool is being used, so that tool output
		// can be piped into a file without this string showing up.
		if opts.tool == nil && config.Verbosity != nin.NoStatusUpdate {
			status.Info("Entering directory `%s'", opts.workingDir)
		}
		if err := os.Chdir(opts.workingDir); err != nil {
			fatalf("chdir to '%s' - %s", opts.workingDir, err)
		}
	}

	if opts.tool != nil && opts.tool.when == runAfterFlags {
		// None of the runAfterFlags actually use a ninjaMain, but it's needed
		// by other tools.
		ninja := newNinjaMain(ninjaCommand, &config)
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
	const cycleLimit = 100
	for cycle := 1; cycle <= cycleLimit; cycle++ {
		ninja := newNinjaMain(ninjaCommand, &config)
		input, err2 := ninja.di.ReadFile(opts.inputFile)
		if err2 != nil {
			status.Error("%s", err2)
			return 1
		}
		parser := nin.NewManifestParser(&ninja.state, &ninja.di, opts.parserOpts)
		if err := parser.Parse(opts.inputFile, input); err != nil {
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
		if rebuilt, err := ninja.RebuildManifest(opts.inputFile, status); rebuilt {
			// In dryRun mode the regeneration will succeed without changing the
			// manifest forever. Better to return immediately.
			if config.DryRun {
				return 0
			}
			// Start the build over with the new manifest.
			continue
		} else if err != nil {
			status.Error("rebuilding '%s': %s", opts.inputFile, err)
			return 1
		}

		result := ninja.RunBuild(args, status)
		if metricsEnabled {
			ninja.DumpMetrics()
		}
		return result
	}

	status.Error("manifest '%s' still dirty after %d tries", opts.inputFile, cycleLimit)
	return 1
}
