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

// A pool for delayed edges.
// Pools are scoped to a State. Edges within a State will share Pools. A Pool
// will keep a count of the total 'weight' of the currently scheduled edges. If
// a Plan attempts to schedule an Edge which would cause the total weight to
// exceed the depth of the Pool, the Pool will enqueue the Edge instead of
// allowing the Plan to schedule it. The Pool will relinquish queued Edges when
// the total scheduled weight diminishes enough (i.e. when a scheduled edge
// completes).
type Pool struct {
	name_ string

	// |current_use_| is the total of the weights of the edges which are
	// currently scheduled in the Plan (i.e. the edges in Plan::ready_).
	current_use_ int
	depth_       int

	delayed_ DelayedEdges
}

func NewPool(name string, depth int) *Pool {
	return &Pool{
		name_:    name,
		depth_:   depth,
		delayed_: DelayedEdges{},
	}
}

// A depth of 0 is infinite
func (p *Pool) is_valid() bool {
	return p.depth_ >= 0
}
func (p *Pool) depth() int {
	return p.depth_
}
func (p *Pool) name() string {
	return p.name_
}
func (p *Pool) current_use() int {
	return p.current_use_
}

// true if the Pool might delay this edge
func (p *Pool) ShouldDelayEdge() bool {
	return p.depth_ != 0
}

/*
type WeightedEdgeCmp struct {
  bool operator()(const Edge* a, const Edge* b) const {
    if (!a) return b
    if (!b) return false
    int weight_diff = a.weight() - b.weight()
    return ((weight_diff < 0) || (weight_diff == 0 && EdgeCmp()(a, b)))
  }
  }
*/
//type DelayedEdges map[Edge*, WeightedEdgeCmp]struct{}
type DelayedEdges map[*Edge]struct{}

// Global state (file status) for a single run.
type State struct {
	kDefaultPool Pool
	kConsolePool Pool
	kPhonyRule   Rule

	// Mapping of path -> Node.
	paths_ Paths

	// All the pools used in the graph.
	pools_ map[string]*Pool

	// All the edges of the graph.
	edges_ []*Edge

	bindings_ BindingEnv
	defaults_ []*Node
}

//type Paths ExternalStringHashMap<Node*>::Type
type Paths map[string]*Node

// informs this Pool that the given edge is committed to be run.
// Pool will count this edge as using resources from this pool.
func (p *Pool) EdgeScheduled(edge *Edge) {
	if p.depth_ != 0 {
		p.current_use_ += edge.weight()
	}
}

// informs this Pool that the given edge is no longer runnable, and should
// relinquish its resources back to the pool
func (p *Pool) EdgeFinished(edge *Edge) {
	if p.depth_ != 0 {
		p.current_use_ -= edge.weight()
	}
}

// adds the given edge to this Pool to be delayed.
func (p *Pool) DelayEdge(edge *Edge) {
	if p.depth_ == 0 {
		panic("oops")
	}
	p.delayed_[edge] = struct{}{}
}

/*
// Pool will add zero or more edges to the ready_queue
func (p *Pool) RetrieveReadyEdges(ready_queue *EdgeSet) {
	it := p.delayed_.begin()
	for it != p.delayed_.end() {
		edge := *it
		if p.current_use_+edge.weight() > p.depth_ {
			break
		}
		ready_queue.insert(edge)
		EdgeScheduled(*edge)
		it++
	}
	p.delayed_.erase(p.delayed_.begin(), it)
}
*/

// Dump the Pool and its edges (useful for debugging).
func (p *Pool) Dump() {
	printf("%s (%d/%d) .\n", p.name_, p.current_use_, p.depth_)
	for it := range p.delayed_ {
		printf("\t")
		it.Dump("")
	}
}

var (
	kDefaultPool Pool //("", 0)
	kConsolePool Pool //("console", 1)
	kPhonyRule   Rule //("phony")
)

func NewState() State {
	s := State{}
	s.bindings_.AddRule(&kPhonyRule)
	s.AddPool(&kDefaultPool)
	s.AddPool(&kConsolePool)
	return s
}

func (s *State) AddPool(pool *Pool) {
	if s.LookupPool(pool.name()) != nil {
		panic("oops")
	}
	s.pools_[pool.name()] = pool
}

func (s *State) LookupPool(pool_name string) *Pool {
	/*
		i := s.pools_.find(pool_name)
		if i == s.pools_.end() {
			return nil
		}
		return i.second
	*/
	panic(nil)
	return nil
}

func (s *State) AddEdge(rule *Rule) *Edge {
	edge := NewEdge()
	edge.rule_ = rule
	edge.pool_ = &kDefaultPool
	edge.env_ = &s.bindings_
	edge.id_ = len(s.edges_)
	s.edges_ = append(s.edges_, edge)
	return edge
}

func (s *State) GetNode(path string, slash_bits uint64) *Node {
	node := s.LookupNode(path)
	if node != nil {
		return node
	}
	node = NewNode(path, slash_bits)
	s.paths_[node.path()] = node
	return node
}

func (s *State) LookupNode(path string) *Node {
	p, _ := s.paths_[path]
	return p
}

/*
func (s *State) SpellcheckNode(path string) *Node {
	kAllowReplacements := true
	kMaxValidEditDistance := 3

	min_distance := kMaxValidEditDistance + 1
	result := nil
	for i := s.paths_.begin(); i != s.paths_.end(); i++ {
		distance := EditDistance(i.first, path, kAllowReplacements, kMaxValidEditDistance)
		if distance < min_distance && i.second {
			min_distance = distance
			result = i.second
		}
	}
	return result
}
*/
func (s *State) AddIn(edge *Edge, path string, slash_bits uint64) {
	node := s.GetNode(path, slash_bits)
	edge.inputs_ = append(edge.inputs_, node)
	node.AddOutEdge(edge)
}

func (s *State) AddOut(edge *Edge, path string, slash_bits uint64) bool {
	node := s.GetNode(path, slash_bits)
	if node.in_edge() != nil {
		return false
	}
	edge.outputs_ = append(edge.outputs_, node)
	node.set_in_edge(edge)
	return true
}

func (s *State) AddDefault(path string, err *string) bool {
	node := s.LookupNode(path)
	if node == nil {
		*err = "unknown target '" + path + "'"
		return false
	}
	s.defaults_ = append(s.defaults_, node)
	return true
}

// @return the root node(s) of the graph. (Root nodes have no output edges).
// @param error where to write the error message if somethings went wrong.
func (s *State) RootNodes(err *string) []*Node {
	var root_nodes []*Node
	// Search for nodes with no output.
	for _, e := range s.edges_ {
		for _, out := range e.outputs_ {
			if len(out.out_edges()) == 0 {
				root_nodes = append(root_nodes, out)
			}
		}
	}

	if len(s.edges_) != 0 && len(root_nodes) == 0 {
		*err = "could not determine root nodes of build graph"
	}

	return root_nodes
}

func (s *State) DefaultNodes(err *string) []*Node {
	if len(s.defaults_) == 0 {
		return s.RootNodes(err)
	}
	return s.defaults_
}

// Reset state.  Keeps all nodes and edges, but restores them to the
// state where we haven't yet examined the disk for dirty state.
func (s *State) Reset() {
	for _, p := range s.paths_ {
		p.ResetState()
	}
	for _, e := range s.edges_ {
		e.outputs_ready_ = false
		e.deps_loaded_ = false
		e.mark_ = VisitNone
	}
}

// Dump the nodes and Pools (useful for debugging).
func (s *State) Dump() {
	for _, node := range s.paths_ {
		s := "unknown"
		if node.status_known() {
			s = "clean"
			if node.dirty() {
				s = "dirty"
			}
		}
		printf("%s %s [id:%d]\n", node.path(), s, node.id())
	}
	if len(s.pools_) != 0 {
		printf("resource_pools:\n")
		for _, p := range s.pools_ {
			if p.name() != "" {
				p.Dump()
			}
		}
	}
}
