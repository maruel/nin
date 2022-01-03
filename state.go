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

// Pool is a pool for delayed edges.
//
// Pools are scoped to a State. Edges within a State will share Pools. A Pool
// will keep a count of the total 'weight' of the currently scheduled edges. If
// a Plan attempts to schedule an Edge which would cause the total weight to
// exceed the depth of the Pool, the Pool will enqueue the Edge instead of
// allowing the Plan to schedule it. The Pool will relinquish queued Edges when
// the total scheduled weight diminishes enough (i.e. when a scheduled edge
// completes).
type Pool struct {
	Name string

	// |currentUse| is the total of the weights of the edges which are
	// currently scheduled in the Plan (i.e. the edges in Plan::ready).
	currentUse int
	depth      int

	delayed *EdgeSet
}

// Note about Pool.delayed: The C++ code checks for Edge.weight() before
// checking for the id. In practice weight is hardcoded to 1!

func NewPool(name string, depth int) *Pool {
	return &Pool{
		Name:    name,
		depth:   depth,
		delayed: NewEdgeSet(),
	}
}

// A depth of 0 is infinite
func (p *Pool) isValid() bool {
	return p.depth >= 0
}

// true if the Pool might delay this edge
func (p *Pool) ShouldDelayEdge() bool {
	return p.depth != 0
}

// informs this Pool that the given edge is committed to be run.
// Pool will count this edge as using resources from this pool.
func (p *Pool) EdgeScheduled(edge *Edge) {
	if p.depth != 0 {
		p.currentUse += edge.weight()
	}
}

// informs this Pool that the given edge is no longer runnable, and should
// relinquish its resources back to the pool
func (p *Pool) EdgeFinished(edge *Edge) {
	if p.depth != 0 {
		p.currentUse -= edge.weight()
	}
}

// adds the given edge to this Pool to be delayed.
func (p *Pool) DelayEdge(edge *Edge) {
	if p.depth == 0 {
		panic("M-A")
	}
	p.delayed.Add(edge)
}

// Pool will add zero or more edges to the readyQueue
func (p *Pool) RetrieveReadyEdges(readyQueue *EdgeSet) {
	// TODO(maruel): Redo without using the internals.
	p.delayed.recreate()
	for len(p.delayed.sorted) != 0 {
		// Do a peek first, then pop.
		edge := p.delayed.sorted[len(p.delayed.sorted)-1]
		if p.currentUse+edge.weight() > p.depth {
			break
		}
		if ed := p.delayed.Pop(); ed != edge {
			panic("M-A")
		}
		readyQueue.Add(edge)
		p.EdgeScheduled(edge)
	}
}

// Dump the Pool and its edges (useful for debugging).
func (p *Pool) Dump() {
	fmt.Printf("%s (%d/%d) ->\n", p.Name, p.currentUse, p.depth)
	// TODO(maruel): Use inner knowledge
	p.delayed.recreate()
	for _, it := range p.delayed.sorted {
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

// Global state (file status) for a single run.
type State struct {
	// Mapping of path -> Node.
	paths Paths

	// All the pools used in the graph.
	pools map[string]*Pool

	// All the edges of the graph.
	edges []*Edge

	bindings *BindingEnv
	defaults []*Node
}

//type Paths ExternalStringHashMap<Node*>::Type
type Paths map[string]*Node

func NewState() State {
	s := State{
		paths:    Paths{},
		pools:    map[string]*Pool{},
		bindings: NewBindingEnv(nil),
	}
	s.bindings.Rules[PhonyRule.Name] = PhonyRule
	s.AddPool(DefaultPool)
	s.AddPool(ConsolePool)
	return s
}

func (s *State) AddPool(pool *Pool) {
	if s.LookupPool(pool.Name) != nil {
		panic(pool.Name)
	}
	s.pools[pool.Name] = pool
}

func (s *State) LookupPool(poolName string) *Pool {
	return s.pools[poolName]
}

func (s *State) AddEdge(rule *Rule) *Edge {
	edge := NewEdge()
	edge.Rule = rule
	edge.Pool = DefaultPool
	edge.Env = s.bindings
	edge.ID = int32(len(s.edges))
	s.edges = append(s.edges, edge)
	return edge
}

func (s *State) Edges() []*Edge {
	// Eventually the member will be renamed Edges.
	return s.edges
}

func (s *State) GetNode(path string, slashBits uint64) *Node {
	node := s.LookupNode(path)
	if node != nil {
		return node
	}
	node = NewNode(path, slashBits)
	s.paths[node.Path] = node
	return node
}

func (s *State) LookupNode(path string) *Node {
	return s.paths[path]
}

func (s *State) SpellcheckNode(path string) *Node {
	const maxValidEditDistance = 3
	minDistance := maxValidEditDistance + 1
	var result *Node
	for p, node := range s.paths {
		distance := EditDistance(p, path, true, maxValidEditDistance)
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
	s.defaults = append(s.defaults, node)
	return true
}

// @return the root node(s) of the graph. (Root nodes have no output edges).
// @param error where to write the error message if somethings went wrong.
func (s *State) RootNodes(err *string) []*Node {
	var rootNodes []*Node
	// Search for nodes with no output.
	for _, e := range s.edges {
		for _, out := range e.Outputs {
			if len(out.OutEdges) == 0 {
				rootNodes = append(rootNodes, out)
			}
		}
	}

	if len(s.edges) != 0 && len(rootNodes) == 0 {
		*err = "could not determine root nodes of build graph"
	}

	return rootNodes
}

func (s *State) DefaultNodes(err *string) []*Node {
	if len(s.defaults) == 0 {
		return s.RootNodes(err)
	}
	return s.defaults
}

// Reset state. Keeps all nodes and edges, but restores them to the
// state where we haven't yet examined the disk for dirty state.
func (s *State) Reset() {
	for _, n := range s.paths {
		n.MTime = -1
		n.Exists = ExistenceStatusUnknown
		n.Dirty = false
	}
	for _, e := range s.edges {
		e.OutputsReady = false
		e.DepsLoaded = false
		e.Mark = VisitNone
	}
}

// Dump the nodes and Pools (useful for debugging).
func (s *State) Dump() {
	names := make([]string, 0, len(s.paths))
	for n := range s.paths {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		node := s.paths[name]
		s := "unknown"
		if node.Exists != ExistenceStatusUnknown {
			s = "clean"
			if node.Dirty {
				s = "dirty"
			}
		}
		fmt.Printf("%s %s [id:%d]\n", node.Path, s, node.ID)
	}
	if len(s.pools) != 0 {
		fmt.Printf("resource_pools:\n")
		for _, p := range s.pools {
			if p.Name != "" {
				p.Dump()
			}
		}
	}
}
