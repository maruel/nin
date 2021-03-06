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

// NewPool returns an initialized Pool.
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
func (p *Pool) shouldDelayEdge() bool {
	return p.depth != 0
}

// informs this Pool that the given edge is committed to be run.
// Pool will count this edge as using resources from this pool.
func (p *Pool) edgeScheduled(edge *Edge) {
	if p.depth != 0 {
		p.currentUse += edge.weight()
	}
}

// informs this Pool that the given edge is no longer runnable, and should
// relinquish its resources back to the pool
func (p *Pool) edgeFinished(edge *Edge) {
	if p.depth != 0 {
		p.currentUse -= edge.weight()
	}
}

// adds the given edge to this Pool to be delayed.
func (p *Pool) delayEdge(edge *Edge) {
	if p.depth == 0 {
		panic("M-A")
	}
	p.delayed.Add(edge)
}

// Pool will add zero or more edges to the readyQueue
func (p *Pool) retrieveReadyEdges(readyQueue *EdgeSet) {
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
		p.edgeScheduled(edge)
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

// Well known default pools and rules.
var (
	DefaultPool = NewPool("", 0)
	ConsolePool = NewPool("console", 1)
	PhonyRule   = NewRule("phony")
)

//

// State is the global state (file status) for a single run.
type State struct {
	// Mapping of path -> Node.
	Paths map[string]*Node

	// All the Pools used in the graph.
	Pools map[string]*Pool

	// All the Edges of the graph.
	Edges []*Edge

	Bindings *BindingEnv
	Defaults []*Node
}

//type Paths ExternalStringHashMap<Node*>::Type

// NewState returns an initialized State.
//
// It is preloaded with PhonyRule, and DefaultPool and ConsolePool.
func NewState() State {
	s := State{
		Paths:    map[string]*Node{},
		Pools:    map[string]*Pool{},
		Bindings: NewBindingEnv(nil),
	}
	s.Bindings.Rules[PhonyRule.Name] = PhonyRule
	s.Pools[DefaultPool.Name] = DefaultPool
	s.Pools[ConsolePool.Name] = ConsolePool
	return s
}

// addEdge creates a new edge with this rule on the default pool.
func (s *State) addEdge(rule *Rule) *Edge {
	edge := &Edge{
		Rule: rule,
		Pool: DefaultPool,
		Env:  s.Bindings,
		ID:   int32(len(s.Edges)),
	}
	s.Edges = append(s.Edges, edge)
	return edge
}

// GetNode returns the existing node for this path.
//
// If the node doesn't exist, create it and return it.
func (s *State) GetNode(path string, slashBits uint64) *Node {
	node := s.Paths[path]
	if node == nil {
		node = &Node{
			Path:      path,
			SlashBits: slashBits,
			MTime:     -1,
			ID:        -1,
			Exists:    ExistenceStatusUnknown,
		}
		s.Paths[node.Path] = node
	}
	return node
}

// SpellcheckNode returns the node with the closest name.
func (s *State) SpellcheckNode(path string) *Node {
	const maxValidEditDistance = 3
	minDistance := maxValidEditDistance + 1
	var result *Node
	for p, node := range s.Paths {
		distance := editDistance(p, path, true, maxValidEditDistance)
		if distance < minDistance && node != nil {
			minDistance = distance
			result = node
		}
	}
	return result
}

func (s *State) addIn(edge *Edge, path string, slashBits uint64) {
	node := s.GetNode(path, slashBits)
	edge.Inputs = append(edge.Inputs, node)
	node.OutEdges = append(node.OutEdges, edge)
}

func (s *State) addOut(edge *Edge, path string, slashBits uint64) bool {
	node := s.GetNode(path, slashBits)
	if node.InEdge != nil {
		return false
	}
	edge.Outputs = append(edge.Outputs, node)
	node.InEdge = edge
	return true
}

func (s *State) addValidation(edge *Edge, path string, slashBits uint64) {
	node := s.GetNode(path, slashBits)
	edge.Validations = append(edge.Validations, node)
	node.ValidationOutEdges = append(node.ValidationOutEdges, edge)
}

func (s *State) addDefault(path string) error {
	node := s.Paths[path]
	if node == nil {
		// TODO(maruel): Use %q for real quoting.
		return fmt.Errorf("unknown target '%s'", path)
	}
	s.Defaults = append(s.Defaults, node)
	return nil
}

// RootNodes return the root node(s) of the graph.
//
// Root nodes have no output edges.
//
// Returns an empty slice if no root node is found.
func (s *State) RootNodes() []*Node {
	var rootNodes []*Node
	// Search for nodes with no output.
	for _, e := range s.Edges {
		for _, out := range e.Outputs {
			if len(out.OutEdges) == 0 {
				rootNodes = append(rootNodes, out)
			}
		}
	}
	return rootNodes
}

// DefaultNodes returns the default nodes to build.
//
// If none are defined, returns all the root nodes.
func (s *State) DefaultNodes() []*Node {
	if len(s.Defaults) == 0 {
		return s.RootNodes()
	}
	return s.Defaults
}

// Reset state. Keeps all nodes and edges, but restores them to the
// state where we haven't yet examined the disk for dirty state.
func (s *State) Reset() {
	for _, n := range s.Paths {
		n.MTime = -1
		n.Exists = ExistenceStatusUnknown
		n.Dirty = false
	}
	for _, e := range s.Edges {
		e.OutputsReady = false
		e.DepsLoaded = false
		e.Mark = VisitNone
	}
}

// Dump the nodes and Pools (useful for debugging).
func (s *State) Dump() {
	names := make([]string, 0, len(s.Paths))
	for n := range s.Paths {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		node := s.Paths[name]
		s := "unknown"
		if node.Exists != ExistenceStatusUnknown {
			s = "clean"
			if node.Dirty {
				s = "dirty"
			}
		}
		fmt.Printf("%s %s [id:%d]\n", node.Path, s, node.ID)
	}
	if len(s.Pools) != 0 {
		fmt.Printf("resource_pools:\n")
		for _, p := range s.Pools {
			if p.Name != "" {
				p.Dump()
			}
		}
	}
}
