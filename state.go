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
	"sort"
)

// A pool for delayed edges.
// Pools are scoped to a State. Edges within a State will share Pools. A Pool
// will keep a count of the total 'weight' of the currently scheduled edges. If
// a Plan attempts to schedule an Edge which would cause the total weight to
// exceed the depth of the Pool, the Pool will enqueue the Edge instead of
// allowing the Plan to schedule it. The Pool will relinquish queued Edges when
// the total scheduled weight diminishes enough (i.e. when a scheduled edge
// completes).
type Pool struct {
	Name string

	// |currentUse_| is the total of the weights of the edges which are
	// currently scheduled in the Plan (i.e. the edges in Plan::ready_).
	currentUse_ int
	depth_      int

	delayed_ *DelayedEdges
}

func NewPool(name string, depth int) *Pool {
	return &Pool{
		Name:     name,
		depth_:   depth,
		delayed_: NewEdgeSet(),
	}
}

// A depth of 0 is infinite
func (p *Pool) isValid() bool {
	return p.depth_ >= 0
}
func (p *Pool) depth() int {
	return p.depth_
}
func (p *Pool) currentUse() int {
	return p.currentUse_
}

// true if the Pool might delay this edge
func (p *Pool) ShouldDelayEdge() bool {
	return p.depth_ != 0
}

// informs this Pool that the given edge is committed to be run.
// Pool will count this edge as using resources from this pool.
func (p *Pool) EdgeScheduled(edge *Edge) {
	if p.depth_ != 0 {
		p.currentUse_ += edge.weight()
	}
}

// informs this Pool that the given edge is no longer runnable, and should
// relinquish its resources back to the pool
func (p *Pool) EdgeFinished(edge *Edge) {
	if p.depth_ != 0 {
		p.currentUse_ -= edge.weight()
	}
}

// adds the given edge to this Pool to be delayed.
func (p *Pool) DelayEdge(edge *Edge) {
	if p.depth_ == 0 {
		panic("M-A")
	}
	p.delayed_.Add(edge)
}

// Pool will add zero or more edges to the readyQueue
func (p *Pool) RetrieveReadyEdges(readyQueue *EdgeSet) {
	// TODO(maruel): Redo without using the internals.
	p.delayed_.recreate()
	for len(p.delayed_.sorted) != 0 {
		// Do a peek first, then pop.
		edge := p.delayed_.sorted[len(p.delayed_.sorted)-1]
		if p.currentUse_+edge.weight() > p.depth_ {
			break
		}
		if ed := p.delayed_.Pop(); ed != edge {
			panic("M-A")
		}
		readyQueue.Add(edge)
		p.EdgeScheduled(edge)
	}
}

// Dump the Pool and its edges (useful for debugging).
func (p *Pool) Dump() {
	fmt.Printf("%s (%d/%d) ->\n", p.Name, p.currentUse_, p.depth_)
	// TODO(maruel): Use inner knowledge
	p.delayed_.recreate()
	for _, it := range p.delayed_.sorted {
		fmt.Printf("\t")
		it.Dump("")
	}
}

var (
	DefaultPool = NewPool("", 0)
	ConsolePool = NewPool("console", 1)
	PhonyRule   = NewRule("phony")
)

//

// The C++ code checks for Edge.weight() before checking for the id. In
// practice weight is hardcoded to 1.
type DelayedEdges = EdgeSet

// Global state (file status) for a single run.
type State struct {
	// Mapping of path -> Node.
	paths_ Paths

	// All the pools used in the graph.
	pools_ map[string]*Pool

	// All the edges of the graph.
	edges_ []*Edge

	bindings_ *BindingEnv
	defaults_ []*Node
}

//type Paths ExternalStringHashMap<Node*>::Type
type Paths map[string]*Node

func NewState() State {
	s := State{
		paths_:    Paths{},
		pools_:    map[string]*Pool{},
		bindings_: NewBindingEnv(nil),
	}
	s.bindings_.AddRule(PhonyRule)
	s.AddPool(DefaultPool)
	s.AddPool(ConsolePool)
	return s
}

func (s *State) AddPool(pool *Pool) {
	if s.LookupPool(pool.Name) != nil {
		panic(pool.Name)
	}
	s.pools_[pool.Name] = pool
}

func (s *State) LookupPool(poolName string) *Pool {
	return s.pools_[poolName]
}

func (s *State) AddEdge(rule *Rule) *Edge {
	edge := NewEdge()
	edge.Rule = rule
	edge.Pool = DefaultPool
	edge.Env = s.bindings_
	edge.ID = int32(len(s.edges_))
	s.edges_ = append(s.edges_, edge)
	return edge
}

func (s *State) Edges() []*Edge {
	// Eventually the member will be renamed Edges.
	return s.edges_
}

func (s *State) GetNode(path string, slashBits uint64) *Node {
	node := s.LookupNode(path)
	if node != nil {
		return node
	}
	node = NewNode(path, slashBits)
	s.paths_[node.Path] = node
	return node
}

func (s *State) LookupNode(path string) *Node {
	return s.paths_[path]
}

func (s *State) SpellcheckNode(path string) *Node {
	kAllowReplacements := true
	kMaxValidEditDistance := 3

	minDistance := kMaxValidEditDistance + 1
	var result *Node
	for p, node := range s.paths_ {
		distance := EditDistance(p, path, kAllowReplacements, kMaxValidEditDistance)
		if distance < minDistance && node != nil {
			minDistance = distance
			result = node
		}
	}
	return result
}

func (s *State) AddIn(edge *Edge, path string, slashBits uint64) {
	node := s.GetNode(path, slashBits)
	edge.Inputs = append(edge.Inputs, node)
	node.OutEdges = append(node.OutEdges, edge)
}

func (s *State) AddOut(edge *Edge, path string, slashBits uint64) bool {
	node := s.GetNode(path, slashBits)
	if node.InEdge != nil {
		return false
	}
	edge.Outputs = append(edge.Outputs, node)
	node.InEdge = edge
	return true
}

func (s *State) AddValidation(edge *Edge, path string, slashBits uint64) {
	node := s.GetNode(path, slashBits)
	edge.Validations = append(edge.Validations, node)
	node.ValidationOutEdges = append(node.ValidationOutEdges, edge)
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
	var rootNodes []*Node
	// Search for nodes with no output.
	for _, e := range s.edges_ {
		for _, out := range e.Outputs {
			if len(out.OutEdges) == 0 {
				rootNodes = append(rootNodes, out)
			}
		}
	}

	if len(s.edges_) != 0 && len(rootNodes) == 0 {
		*err = "could not determine root nodes of build graph"
	}

	return rootNodes
}

func (s *State) DefaultNodes(err *string) []*Node {
	if len(s.defaults_) == 0 {
		return s.RootNodes(err)
	}
	return s.defaults_
}

// Reset state. Keeps all nodes and edges, but restores them to the
// state where we haven't yet examined the disk for dirty state.
func (s *State) Reset() {
	for _, n := range s.paths_ {
		n.MTime = -1
		n.Exists = ExistenceStatusUnknown
		n.Dirty = false
	}
	for _, e := range s.edges_ {
		e.OutputsReady = false
		e.DepsLoaded = false
		e.Mark = VisitNone
	}
}

// Dump the nodes and Pools (useful for debugging).
func (s *State) Dump() {
	names := make([]string, 0, len(s.paths_))
	for n := range s.paths_ {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		node := s.paths_[name]
		s := "unknown"
		if node.Exists != ExistenceStatusUnknown {
			s = "clean"
			if node.Dirty {
				s = "dirty"
			}
		}
		fmt.Printf("%s %s [id:%d]\n", node.Path, s, node.ID)
	}
	if len(s.pools_) != 0 {
		fmt.Printf("resource_pools:\n")
		for _, p := range s.pools_ {
			if p.Name != "" {
				p.Dump()
			}
		}
	}
}
