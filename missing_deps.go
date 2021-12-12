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

//go:build nobuild

package ginja


class MissingDependencyScannerDelegate {
  virtual ~MissingDependencyScannerDelegate()
}

class MissingDependencyPrinter : public MissingDependencyScannerDelegate {
}

type MissingDependencyScanner struct {
  bool HadMissingDeps() { return !nodes_missing_deps_.empty(); }

  MissingDependencyScannerDelegate* delegate_
  DepsLog* deps_log_
  State* state_
  DiskInterface* disk_interface_
  set<Node*> seen_
  set<Node*> nodes_missing_deps_
  set<Node*> generated_nodes_
  set<const Rule*> generator_rules_
  int missing_dep_path_count_

  InnerAdjacencyMap := unordered_map<Edge*, bool>
  AdjacencyMap := unordered_map<Edge*, InnerAdjacencyMap>
  typedef map<Edge*, bool> InnerAdjacencyMap
  typedef map<Edge*, InnerAdjacencyMap> AdjacencyMap
  AdjacencyMap adjacency_map_
}


// ImplicitDepLoader variant that stores dep nodes into the given output
// without updating graph deps like the base loader does.
type NodeStoringImplicitDepLoader struct {
  NodeStoringImplicitDepLoader( State* state, DepsLog* deps_log, DiskInterface* disk_interface, DepfileParserOptions const* depfile_parser_options, vector<Node*>* dep_nodes_output)
      : ImplicitDepLoader(state, deps_log, disk_interface, depfile_parser_options),
        dep_nodes_output_(dep_nodes_output) {}

  vector<Node*>* dep_nodes_output_
}

func (n *NodeStoringImplicitDepLoader) ProcessDepfileDeps(edge *Edge, depfile_ins *vector<StringPiece>, err *string) bool {
  for (vector<StringPiece>::iterator i = depfile_ins.begin(); i != depfile_ins.end(); ++i) {
    uint64_t slash_bits
    CanonicalizePath(const_cast<char*>(i.str_), &i.len_, &slash_bits)
    node := state_.GetNode(*i, slash_bits)
    dep_nodes_output_.push_back(node)
  }
  return true
}

MissingDependencyScannerDelegate::~MissingDependencyScannerDelegate() {}

func (m *MissingDependencyPrinter) OnMissingDep(node *Node, path string, generator *Rule) {
  cout << "Missing dep: " << node.path() << " uses " << path
            << " (generated by " << generator.name() << ")\n"
}

MissingDependencyScanner::MissingDependencyScanner( MissingDependencyScannerDelegate* delegate, DepsLog* deps_log, State* state, DiskInterface* disk_interface)
    : delegate_(delegate), deps_log_(deps_log), state_(state),
      disk_interface_(disk_interface), missing_dep_path_count_(0) {}

func (m *MissingDependencyScanner) ProcessNode(node *Node) {
  if node == nil {
    return
  }
  edge := node.in_edge()
  if edge == nil {
    return
  }
  if !seen_.insert(node).second {
    return
  }

  for (vector<Node*>::iterator in = edge.inputs_.begin(); in != edge.inputs_.end(); ++in) {
    ProcessNode(*in)
  }

  deps_type := edge.GetBinding("deps")
  if !deps_type.empty() {
    deps := deps_log_.GetDeps(node)
    if deps != nil {
      ProcessNodeDeps(node, deps.nodes, deps.node_count)
    }
  } else {
    DepfileParserOptions parser_opts
    vector<Node*> depfile_deps
    string err
    dep_loader.LoadDeps(edge, &err)
    if !depfile_deps.empty() {
      ProcessNodeDeps(node, &depfile_deps[0], depfile_deps.size())
    }
  }
}

func (m *MissingDependencyScanner) ProcessNodeDeps(node *Node, dep_nodes **Node, dep_nodes_count int) {
  edge := node.in_edge()
  set<Edge*> deplog_edges
  for (int i = 0; i < dep_nodes_count; ++i) {
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
    if deplog_edge {
      deplog_edges.insert(deplog_edge)
    }
  }
  vector<Edge*> missing_deps
  for (set<Edge*>::iterator de = deplog_edges.begin(); de != deplog_edges.end(); ++de) {
    if !PathExistsBetween(*de, edge) {
      missing_deps.push_back(*de)
    }
  }

  if !missing_deps.empty() {
    set<string> missing_deps_rule_names
    for (vector<Edge*>::iterator ne = missing_deps.begin(); ne != missing_deps.end(); ++ne) {
      for (int i = 0; i < dep_nodes_count; ++i) {
        if dep_nodes[i].in_edge() == *ne {
          generated_nodes_.insert(dep_nodes[i])
          generator_rules_.insert(&(*ne).rule())
          missing_deps_rule_names.insert((*ne).rule().name())
          delegate_.OnMissingDep(node, dep_nodes[i].path(), (*ne).rule())
        }
      }
    }
    missing_dep_path_count_ += missing_deps_rule_names.size()
    nodes_missing_deps_.insert(node)
  }
}

func (m *MissingDependencyScanner) PrintStats() {
  cout << "Processed " << seen_.size() << " nodes.\n"
  if HadMissingDeps() {
    cout << "Error: There are " << missing_dep_path_count_
              << " missing dependency paths.\n"
    cout << nodes_missing_deps_.size()
              << " targets had depfile dependencies on "
              << generated_nodes_.size() << " distinct generated inputs "
              << "(from " << generator_rules_.size() << " rules) "
              << " without a non-depfile dep path to the generator.\n"
    cout << "There might be build flakiness if any of the targets listed "
                 "above are built alone, or not late enough, in a clean output "
                 "directory.\n"
  } else {
    cout << "No missing dependencies on generated files found.\n"
  }
}

func (m *MissingDependencyScanner) PathExistsBetween(from *Edge, to *Edge) bool {
  it := adjacency_map_.find(from)
  if it != adjacency_map_.end() {
    inner_it := it.second.find(to)
    if inner_it != it.second.end() {
      return inner_it.second
    }
  } else {
    it = adjacency_map_.insert(make_pair(from, InnerAdjacencyMap())).first
  }
  found := false
  for (size_t i = 0; i < to.inputs_.size(); ++i) {
    e := to.inputs_[i].in_edge()
    if e && (e == from || PathExistsBetween(from, e)) {
      found = true
      break
    }
  }
  it.second.insert(make_pair(to, found))
  return found
}

