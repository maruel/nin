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

// GraphViz is the object to initialize the parameters to create GraphViz .dot
// file output.
type GraphViz struct {
	out          io.Writer
	dyndepLoader DyndepLoader
	visitedNodes map[*Node]struct{}
	visitedEdges map[*Edge]struct{}
}

// NewGraphViz returns an initialized GraphViz.
func NewGraphViz(state *State, di DiskInterface) GraphViz {
	return GraphViz{
		out:          os.Stdout,
		dyndepLoader: NewDyndepLoader(state, di),
		visitedNodes: map[*Node]struct{}{},
		visitedEdges: map[*Edge]struct{}{},
	}
}

// AddTarget adds a node to include in the graph.
func (g *GraphViz) AddTarget(node *Node) {
	if _, ok := g.visitedNodes[node]; ok {
		return
	}

	fmt.Fprintf(g.out, "\"%p\" [label=\"%s\"]\n", node, strings.ReplaceAll(node.Path, "\\", "/"))
	g.visitedNodes[node] = struct{}{}

	edge := node.InEdge

	if edge == nil {
		// Leaf node.
		// Draw as a rect?
		return
	}

	if _, ok := g.visitedEdges[edge]; ok {
		return
	}
	g.visitedEdges[edge] = struct{}{}

	if edge.Dyndep != nil && edge.Dyndep.DyndepPending {
		if err := g.dyndepLoader.LoadDyndeps(edge.Dyndep, DyndepFile{}); err != nil {
			warningf("%s\n", err)
		}
	}

	if len(edge.Inputs) == 1 && len(edge.Outputs) == 1 {
		// Can draw simply.
		// Note extra space before label text -- this is cosmetic and feels
		// like a graphviz bug.
		fmt.Fprintf(g.out, "\"%p\" -> \"%p\" [label=\" %s\"]\n", edge.Inputs[0], edge.Outputs[0], edge.Rule.Name)
	} else {
		fmt.Fprintf(g.out, "\"%p\" [label=\"%s\", shape=ellipse]\n", edge, edge.Rule.Name)
		for _, out := range edge.Outputs {
			fmt.Fprintf(g.out, "\"%p\" -> \"%p\"\n", edge, out)
		}
		for i, in := range edge.Inputs {
			orderOnly := ""
			if edge.IsOrderOnly(i) {
				orderOnly = " style=dotted"
			}
			fmt.Fprintf(g.out, "\"%p\" -> \"%p\" [arrowhead=none%s]\n", in, edge, orderOnly)
		}
	}

	for _, in := range edge.Inputs {
		g.AddTarget(in)
	}
}

// Start prints out the header.
func (g *GraphViz) Start() {
	fmt.Fprintf(g.out, "digraph ninja {\n")
	fmt.Fprintf(g.out, "rankdir=\"LR\"\n")
	fmt.Fprintf(g.out, "node [fontsize=10, shape=box, height=0.25]\n")
	fmt.Fprintf(g.out, "edge [fontsize=10]\n")
}

// Finish prints out the footer.
func (g *GraphViz) Finish() {
	fmt.Fprintf(g.out, "}\n")
}
