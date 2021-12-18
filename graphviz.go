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


// Runs the process of creating GraphViz .dot file output.
type GraphViz struct {
  GraphViz(State* state, DiskInterface* disk_interface)
      : dyndep_loader_(state, disk_interface) {}

  dyndep_loader_ DyndepLoader
  visited_nodes_ map[*Node]struct{}
  visited_edges_ EdgeSet
}


func (g *GraphViz) AddTarget(node *Node) {
  if g.visited_nodes_.find(node) != g.visited_nodes_.end() {
    return
  }

  pathstr := node.path()
  replace(pathstr.begin(), pathstr.end(), '\\', '/')
  printf("\"%p\" [label=\"%s\"]\n", node, pathstr)
  g.visited_nodes_.insert(node)

  edge := node.in_edge()

  if edge == nil {
    // Leaf node.
    // Draw as a rect?
    return
  }

  if g.visited_edges_.find(edge) != g.visited_edges_.end() {
    return
  }
  g.visited_edges_.insert(edge)

  if edge.dyndep_ && edge.dyndep_.dyndep_pending() {
    err := ""
    if !g.dyndep_loader_.LoadDyndeps(edge.dyndep_, &err) {
      Warning("%s\n", err)
    }
  }

  if edge.inputs_.size() == 1 && edge.outputs_.size() == 1 {
    // Can draw simply.
    // Note extra space before label text -- this is cosmetic and feels
    // like a graphviz bug.
    printf("\"%p\" . \"%p\" [label=\" %s\"]\n", edge.inputs_[0], edge.outputs_[0], edge.rule_.name())
  } else {
    printf("\"%p\" [label=\"%s\", shape=ellipse]\n", edge, edge.rule_.name())
    for out := edge.outputs_.begin(); out != edge.outputs_.end(); out++ {
      printf("\"%p\" . \"%p\"\n", edge, *out)
    }
    for in := edge.inputs_.begin(); in != edge.inputs_.end(); in++ {
      string order_only = ""
      if edge.is_order_only(in - edge.inputs_.begin()) {
        order_only = " style=dotted"
      }
      printf("\"%p\" . \"%p\" [arrowhead=none%s]\n", (*in), edge, order_only)
    }
  }

  for in := edge.inputs_.begin(); in != edge.inputs_.end(); in++ {
    AddTarget(*in)
  }
}

func (g *GraphViz) Start() {
  printf("digraph ninja {\n")
  printf("rankdir=\"LR\"\n")
  printf("node [fontsize=10, shape=box, height=0.25]\n")
  printf("edge [fontsize=10]\n")
}

func (g *GraphViz) Finish() {
  printf("}\n")
}

