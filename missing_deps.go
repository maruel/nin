// Copyright 2019 Google Inc. All Rights Reserved.
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

import "fmt"

type MissingDependencyScannerDelegate interface {
	OnMissingDep(node *Node, path string, generator *Rule)
}

type MissingDependencyPrinter struct {
}

type MissingDependencyScanner struct {
	delegate_               MissingDependencyScannerDelegate
	deps_log_               *DepsLog
	state_                  *State
	disk_interface_         DiskInterface
	seen_                   map[*Node]struct{}
	nodes_missing_deps_     map[*Node]struct{}
	generated_nodes_        map[*Node]struct{}
	generator_rules_        map[*Rule]struct{}
	missing_dep_path_count_ int

	adjacency_map_ AdjacencyMap
}
type InnerAdjacencyMap map[*Edge]bool
type AdjacencyMap map[*Edge]InnerAdjacencyMap

func (m *MissingDependencyScanner) HadMissingDeps() bool {
	return len(m.nodes_missing_deps_) != 0
}

// ImplicitDepLoader variant that stores dep nodes into the given output
// without updating graph deps like the base loader does.
type NodeStoringImplicitDepLoader struct {
	ImplicitDepLoader
	dep_nodes_output_ []*Node
}

func NewNodeStoringImplicitDepLoader(state *State, deps_log *DepsLog, disk_interface DiskInterface, depfile_parser_options *DepfileParserOptions, dep_nodes_output []*Node) NodeStoringImplicitDepLoader {
	return NodeStoringImplicitDepLoader{
		ImplicitDepLoader: NewImplicitDepLoader(state, deps_log, disk_interface, depfile_parser_options),
		dep_nodes_output_: dep_nodes_output,
	}
}

func (n *NodeStoringImplicitDepLoader) ProcessDepfileDeps(edge *Edge, depfile_ins []string, err *string) bool {
	for _, i := range depfile_ins {
		var slash_bits uint64
		i = CanonicalizePath(i, &slash_bits)
		node := n.state_.GetNode(i, slash_bits)
		n.dep_nodes_output_ = append(n.dep_nodes_output_, node)
	}
	return true
}

func (m *MissingDependencyPrinter) OnMissingDep(node *Node, path string, generator *Rule) {
	fmt.Printf("Missing dep: %s uses %s (generated by %s)\n", node.path(), path, generator.name())
}

func NewMissingDependencyScanner(delegate MissingDependencyScannerDelegate, deps_log *DepsLog, state *State, disk_interface DiskInterface) MissingDependencyScanner {
	return MissingDependencyScanner{
		delegate_:           delegate,
		deps_log_:           deps_log,
		state_:              state,
		disk_interface_:     disk_interface,
		seen_:               map[*Node]struct{}{},
		nodes_missing_deps_: map[*Node]struct{}{},
		generated_nodes_:    map[*Node]struct{}{},
		generator_rules_:    map[*Rule]struct{}{},
		adjacency_map_:      AdjacencyMap{},
	}
}

func (m *MissingDependencyScanner) ProcessNode(node *Node) {
	if node == nil {
		return
	}
	edge := node.in_edge()
	if edge == nil {
		return
	}
	if _, ok := m.seen_[node]; ok {
		return
	}
	m.seen_[node] = struct{}{}

	for _, in := range edge.inputs_ {
		m.ProcessNode(in)
	}

	deps_type := edge.GetBinding("deps")
	if len(deps_type) != 0 {
		deps := m.deps_log_.GetDeps(node)
		if deps != nil {
			m.ProcessNodeDeps(node, deps.nodes)
		}
	} else {
		var parser_opts DepfileParserOptions
		var depfile_deps []*Node
		dep_loader := NewNodeStoringImplicitDepLoader(m.state_, m.deps_log_, m.disk_interface_, &parser_opts, depfile_deps)
		err := ""
		dep_loader.LoadDeps(edge, &err)
		if len(depfile_deps) != 0 {
			m.ProcessNodeDeps(node, depfile_deps)
		}
	}
}

func (m *MissingDependencyScanner) ProcessNodeDeps(node *Node, dep_nodes []*Node) {
	edge := node.in_edge()
	deplog_edges := map[*Edge]struct{}{}
	for i := 0; i < len(dep_nodes); i++ {
		deplog_node := dep_nodes[i]
		// Special exception: A dep on build.ninja can be used to mean "always
		// rebuild this target when the build is reconfigured", but build.ninja is
		// often generated by a configuration tool like cmake or gn. The rest of
		// the build "implicitly" depends on the entire build being reconfigured,
		// so a missing dep path to build.ninja is not an actual missing dependency
		// problem.
		if deplog_node.path() == "build.ninja" {
			return
		}
		deplog_edge := deplog_node.in_edge()
		if deplog_edge != nil {
			deplog_edges[deplog_edge] = struct{}{}
		}
	}
	var missing_deps []*Edge
	for de := range deplog_edges {
		if !m.PathExistsBetween(de, edge) {
			missing_deps = append(missing_deps, de)
		}
	}

	if len(missing_deps) != 0 {
		missing_deps_rule_names := map[string]struct{}{}
		for _, ne := range missing_deps {
			if ne == nil {
				panic("M-A")
			}
			for i := 0; i < len(dep_nodes); i++ {
				if dep_nodes[i].in_edge() == nil {
					panic("M-A")
				}
				if m.delegate_ == nil {
					panic("M-A")
				}
				if dep_nodes[i].in_edge() == ne {
					m.generated_nodes_[dep_nodes[i]] = struct{}{}
					m.generator_rules_[ne.rule()] = struct{}{}
					missing_deps_rule_names[ne.rule().name()] = struct{}{}
					m.delegate_.OnMissingDep(node, dep_nodes[i].path(), ne.rule())
				}
			}
		}
		m.missing_dep_path_count_ += len(missing_deps_rule_names)
		m.nodes_missing_deps_[node] = struct{}{}
	}
}

func (m *MissingDependencyScanner) PrintStats() {
	fmt.Printf("Processed %d nodes.\n", len(m.seen_))
	if m.HadMissingDeps() {
		fmt.Printf("Error: There are %d missing dependency paths.\n", m.missing_dep_path_count_)
		fmt.Printf("%d targets had depfile dependencies on %d distinct generated inputs (from %d rules) without a non-depfile dep path to the generator.\n",
			len(m.nodes_missing_deps_), len(m.generated_nodes_), len(m.generator_rules_))
		fmt.Printf("There might be build flakiness if any of the targets listed above are built alone, or not late enough, in a clean output directory.\n")
	} else {
		fmt.Printf("No missing dependencies on generated files found.\n")
	}
}

func (m *MissingDependencyScanner) PathExistsBetween(from *Edge, to *Edge) bool {
	it, ok := m.adjacency_map_[from]
	if ok {
		inner_it, ok := it[to]
		if ok {
			return inner_it
		}
	} else {
		it = InnerAdjacencyMap{}
		m.adjacency_map_[from] = it
	}
	found := false
	for i := 0; i < len(to.inputs_); i++ {
		e := to.inputs_[i].in_edge()
		if e != nil && (e == from || m.PathExistsBetween(from, e)) {
			found = true
			break
		}
	}
	it[to] = found
	return found
}
