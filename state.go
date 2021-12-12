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


// A pool for delayed edges.
// Pools are scoped to a State. Edges within a State will share Pools. A Pool
// will keep a count of the total 'weight' of the currently scheduled edges. If
// a Plan attempts to schedule an Edge which would cause the total weight to
// exceed the depth of the Pool, the Pool will enqueue the Edge instead of
// allowing the Plan to schedule it. The Pool will relinquish queued Edges when
// the total scheduled weight diminishes enough (i.e. when a scheduled edge
// completes).
type Pool struct {
  Pool(string name, int depth)
    : name_(name), current_use_(0), depth_(depth), delayed_() {}

  // A depth of 0 is infinite
  bool is_valid() const { return depth_ >= 0; }
  int depth() const { return depth_; }
  string name() const { return name_; }
  int current_use() const { return current_use_; }

  // true if the Pool might delay this edge
  bool ShouldDelayEdge() const { return depth_ != 0; }

  string name_

  // |current_use_| is the total of the weights of the edges which are
  // currently scheduled in the Plan (i.e. the edges in Plan::ready_).
  int current_use_
  int depth_

  type WeightedEdgeCmp struct {
    bool operator()(const Edge* a, const Edge* b) {
      if (!a) return b
      if (!b) return false
      weight_diff := a.weight() - b.weight()
      return ((weight_diff < 0) || (weight_diff == 0 && EdgeCmp()(a, b)))
    }
  }

  typedef set<Edge*, WeightedEdgeCmp> DelayedEdges
  DelayedEdges delayed_
}

// Global state (file status) for a single run.
type State struct {
  static Pool kDefaultPool
  static Pool kConsolePool
  static const Rule kPhonyRule

  Node* GetNode(StringPiece path, uint64_t slash_bits)

  void AddIn(Edge* edge, StringPiece path, uint64_t slash_bits)
  bool AddOut(Edge* edge, StringPiece path, uint64_t slash_bits)

  // Mapping of path -> Node.
  typedef ExternalStringHashMap<Node*>::Type Paths
  Paths paths_

  // All the pools used in the graph.
  map<string, Pool*> pools_

  // All the edges of the graph.
  vector<Edge*> edges_

  BindingEnv bindings_
  vector<Node*> defaults_
}


// informs this Pool that the given edge is committed to be run.
// Pool will count this edge as using resources from this pool.
func (p *Pool) EdgeScheduled(edge *Edge) {
  if depth_ != 0 {
    current_use_ += edge.weight()
  }
}

// informs this Pool that the given edge is no longer runnable, and should
// relinquish its resources back to the pool
func (p *Pool) EdgeFinished(edge *Edge) {
  if depth_ != 0 {
    current_use_ -= edge.weight()
  }
}

// adds the given edge to this Pool to be delayed.
func (p *Pool) DelayEdge(edge *Edge) {
  assert(depth_ != 0)
  delayed_.insert(edge)
}

// Pool will add zero or more edges to the ready_queue
func (p *Pool) RetrieveReadyEdges(ready_queue *EdgeSet) {
  it := delayed_.begin()
  while (it != delayed_.end()) {
    edge := *it
    if current_use_ + edge.weight() > depth_ {
      break
    }
    ready_queue.insert(edge)
    EdgeScheduled(*edge)
    ++it
  }
  delayed_.erase(delayed_.begin(), it)
}

// Dump the Pool and its edges (useful for debugging).
func (p *Pool) Dump() {
  printf("%s (%d/%d) .\n", name_, current_use_, depth_)
  for (DelayedEdges::const_iterator it = delayed_.begin(); it != delayed_.end(); ++it)
  {
    printf("\t")
    (*it).Dump()
  }
}

Pool State::kDefaultPool("", 0)
Pool State::kConsolePool("console", 1)
const Rule State::kPhonyRule("phony")

State::State() {
  bindings_.AddRule(&kPhonyRule)
  AddPool(&kDefaultPool)
  AddPool(&kConsolePool)
}

func (s *State) AddPool(pool *Pool) {
  assert(LookupPool(pool.name()) == nil)
  pools_[pool.name()] = pool
}

func (s *State) LookupPool(pool_name string) Pool* {
  map<string, Pool*>::iterator i = pools_.find(pool_name)
  if i == pools_.end() {
    return nil
  }
  return i.second
}

func (s *State) AddEdge(rule *const Rule) Edge* {
  edge := new Edge()
  edge.rule_ = rule
  edge.pool_ = &State::kDefaultPool
  edge.env_ = &bindings_
  edge.id_ = edges_.size()
  edges_.push_back(edge)
  return edge
}

Node* State::GetNode(StringPiece path, uint64_t slash_bits) {
  node := LookupNode(path)
  if node != nil {
    return node
  }
  node = new Node(path.AsString(), slash_bits)
  paths_[node.path()] = node
  return node
}

func (s *State) LookupNode(path StringPiece) Node* {
  Paths::const_iterator i = paths_.find(path)
  if i != paths_.end() {
    return i.second
  }
  return nil
}

func (s *State) SpellcheckNode(path string) Node* {
  const bool kAllowReplacements = true
  const int kMaxValidEditDistance = 3

  min_distance := kMaxValidEditDistance + 1
  result := nil
  for (Paths::iterator i = paths_.begin(); i != paths_.end(); ++i) {
    distance := EditDistance( i.first, path, kAllowReplacements, kMaxValidEditDistance)
    if distance < min_distance && i.second {
      min_distance = distance
      result = i.second
    }
  }
  return result
}

void State::AddIn(Edge* edge, StringPiece path, uint64_t slash_bits) {
  node := GetNode(path, slash_bits)
  edge.inputs_.push_back(node)
  node.AddOutEdge(edge)
}

bool State::AddOut(Edge* edge, StringPiece path, uint64_t slash_bits) {
  node := GetNode(path, slash_bits)
  if node.in_edge() {
    return false
  }
  edge.outputs_.push_back(node)
  node.set_in_edge(edge)
  return true
}

func (s *State) AddDefault(path StringPiece, err *string) bool {
  node := LookupNode(path)
  if node == nil {
    *err = "unknown target '" + path.AsString() + "'"
    return false
  }
  defaults_.push_back(node)
  return true
}

// @return the root node(s) of the graph. (Root nodes have no output edges).
// @param error where to write the error message if somethings went wrong.
func (s *State) RootNodes(err *string) vector<Node*> {
  vector<Node*> root_nodes
  // Search for nodes with no output.
  for (vector<Edge*>::const_iterator e = edges_.begin(); e != edges_.end(); ++e) {
    for (vector<Node*>::const_iterator out = (*e).outputs_.begin(); out != (*e).outputs_.end(); ++out) {
      if (*out).out_edges().empty() {
        root_nodes.push_back(*out)
      }
    }
  }

  if !edges_.empty() && root_nodes.empty() {
    *err = "could not determine root nodes of build graph"
  }

  return root_nodes
}

func (s *State) DefaultNodes(err *string) vector<Node*> {
  return defaults_.empty() ? RootNodes(err) : defaults_
}

// Reset state.  Keeps all nodes and edges, but restores them to the
// state where we haven't yet examined the disk for dirty state.
func (s *State) Reset() {
  for (Paths::iterator i = paths_.begin(); i != paths_.end(); ++i)
    i.second.ResetState()
  for (vector<Edge*>::iterator e = edges_.begin(); e != edges_.end(); ++e) {
    (*e).outputs_ready_ = false
    (*e).deps_loaded_ = false
    (*e).mark_ = Edge::VisitNone
  }
}

// Dump the nodes and Pools (useful for debugging).
func (s *State) Dump() {
  for (Paths::iterator i = paths_.begin(); i != paths_.end(); ++i) {
    node := i.second
    printf("%s %s [id:%d]\n", node.path(), node.status_known() ? (node.dirty() ? "dirty" : "clean") : "unknown", node.id())
  }
  if !pools_.empty() {
    printf("resource_pools:\n")
    for (map<string, Pool*>::const_iterator it = pools_.begin(); it != pools_.end(); ++it)
    {
      if !it.second.name().empty() {
        it.second.Dump()
      }
    }
  }
}

