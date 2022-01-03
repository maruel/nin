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
	"io"
	"os"
	"strings"
)

// Runs the process of creating GraphViz .dot file output.
type GraphViz struct {
	out            io.Writer
	dyndep_loader_ DyndepLoader
	visited_nodes_ map[*Node]struct{}
	visited_edges_ map[*Edge]struct{}
}

func NewGraphViz(state *State, disk_interface DiskInterface) GraphViz {
	return GraphViz{
		out:            os.Stdout,
		dyndep_loader_: NewDyndepLoader(state, disk_interface),
		visited_nodes_: map[*Node]struct{}{},
		visited_edges_: map[*Edge]struct{}{},
	}
}

func (g *GraphViz) AddTarget(node *Node) {
	if _, ok := g.visited_nodes_[node]; ok {
		return
	}

	fmt.Fprintf(g.out, "\"%p\" [label=\"%s\"]\n", node, strings.ReplaceAll(node.Path, "\\", "/"))
	g.visited_nodes_[node] = struct{}{}

	edge := node.InEdge

	if edge == nil {
		// Leaf node.
		// Draw as a rect?
		return
	}

	if _, ok := g.visited_edges_[edge]; ok {
		return
	}
	g.visited_edges_[edge] = struct{}{}

	if edge.Dyndep != nil && edge.Dyndep.DyndepPending {
		err := ""
		if !g.dyndep_loader_.LoadDyndeps(edge.Dyndep, DyndepFile{}, &err) {
			warningf("%s\n", err)
		}
	}

	if len(edge.Inputs) == 1 && len(edge.Outputs) == 1 {
		// Can draw simply.
		// Note extra space before label text -- this is cosmetic and feels
		// like a graphviz bug.
		fmt.Fprintf(g.out, "\"%p\" -> \"%p\" [label=\" %s\"]\n", edge.Inputs[0], edge.Outputs[0], edge.Rule.name())
	} else {
		fmt.Fprintf(g.out, "\"%p\" [label=\"%s\", shape=ellipse]\n", edge, edge.Rule.name())
		for _, out := range edge.Outputs {
			fmt.Fprintf(g.out, "\"%p\" -> \"%p\"\n", edge, out)
		}
		for i, in := range edge.Inputs {
			order_only := ""
			if edge.is_order_only(i) {
				order_only = " style=dotted"
			}
			fmt.Fprintf(g.out, "\"%p\" -> \"%p\" [arrowhead=none%s]\n", in, edge, order_only)
		}
	}

	for _, in := range edge.Inputs {
		g.AddTarget(in)
	}
}

func (g *GraphViz) Start() {
	fmt.Fprintf(g.out, "digraph ninja {\n")
	fmt.Fprintf(g.out, "rankdir=\"LR\"\n")
	fmt.Fprintf(g.out, "node [fontsize=10, shape=box, height=0.25]\n")
	fmt.Fprintf(g.out, "edge [fontsize=10]\n")
}

func (g *GraphViz) Finish() {
	fmt.Fprintf(g.out, "}\n")
}
