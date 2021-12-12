// Copyright 2015 Google Inc. All Rights Reserved.
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


// Store dynamically-discovered dependency information for one edge.
type Dyndeps struct {
  Dyndeps() : used_(false), restat_(false) {}
  bool used_
  bool restat_
  vector<Node*> implicit_inputs_
  vector<Node*> implicit_outputs_
}

// Store data loaded from one dyndep file.  Map from an edge
// to its dynamically-discovered dependency information.
// This is a struct rather than a typedef so that we can
// forward-declare it in other headers.
struct DyndepFile: public map<Edge*, Dyndeps> {}

// DyndepLoader loads dynamically discovered dependencies, as
// referenced via the "dyndep" attribute in build files.
type DyndepLoader struct {
  DyndepLoader(State* state, DiskInterface* disk_interface)
      : state_(state), disk_interface_(disk_interface) {}

  State* state_
  DiskInterface* disk_interface_
}


// Load a dyndep file from the given node's path and update the
// build graph with the new information.  One overload accepts
// a caller-owned 'DyndepFile' object in which to store the
// information loaded from the dyndep file.
func (d *DyndepLoader) LoadDyndeps(node *Node, err *string) bool {
  DyndepFile ddf
}

// Load a dyndep file from the given node's path and update the
// build graph with the new information.  One overload accepts
// a caller-owned 'DyndepFile' object in which to store the
// information loaded from the dyndep file.
func (d *DyndepLoader) LoadDyndeps(node *Node, ddf *DyndepFile, err *string) bool {
  // We are loading the dyndep file now so it is no longer pending.
  node.set_dyndep_pending(false)

  // Load the dyndep information from the file.
  EXPLAIN("loading dyndep file '%s'", node.path())
  if !LoadDyndepFile(node, ddf, err) {
    return false
  }

  // Update each edge that specified this node as its dyndep binding.
  vector<Edge*> const& out_edges = node.out_edges()
  for (vector<Edge*>::const_iterator oe = out_edges.begin(); oe != out_edges.end(); ++oe) {
    Edge* const edge = *oe
    if edge.dyndep_ != node {
      continue
    }

    ddi := ddf.find(edge)
    if ddi == ddf.end() {
      *err = ("'" + edge.outputs_[0].path() + "' " "not mentioned in its dyndep file " "'" + node.path() + "'")
      return false
    }

    ddi.second.used_ = true
    Dyndeps const& dyndeps = ddi.second
    if !UpdateEdge(edge, &dyndeps, err) {
      return false
    }
  }

  // Reject extra outputs in dyndep file.
  for (DyndepFile::const_iterator oe = ddf.begin(); oe != ddf.end(); ++oe) {
    if !oe.second.used_ {
      Edge* const edge = oe.first
      *err = ("dyndep file '" + node.path() + "' mentions output " "'" + edge.outputs_[0].path() + "' whose build statement " "does not have a dyndep binding for the file")
      return false
    }
  }

  return true
}

func (d *DyndepLoader) UpdateEdge(edge *Edge, dyndeps *Dyndeps const, err *string) bool {
  // Add dyndep-discovered bindings to the edge.
  // We know the edge already has its own binding
  // scope because it has a "dyndep" binding.
  if dyndeps.restat_ {
    edge.env_.AddBinding("restat", "1")
  }

  // Add the dyndep-discovered outputs to the edge.
  edge.outputs_.insert(edge.outputs_.end(), dyndeps.implicit_outputs_.begin(), dyndeps.implicit_outputs_.end())
  edge.implicit_outs_ += dyndeps.implicit_outputs_.size()

  // Add this edge as incoming to each new output.
  for (vector<Node*>::const_iterator i = dyndeps.implicit_outputs_.begin(); i != dyndeps.implicit_outputs_.end(); ++i) {
    if Edge* old_in_edge = (*i).in_edge() {
      // This node already has an edge producing it.  Fail with an error
      // unless the edge was generated by ImplicitDepLoader, in which
      // case we can replace it with the now-known real producer.
      if !old_in_edge.generated_by_dep_loader_ {
        *err = "multiple rules generate " + (*i).path()
        return false
      }
      old_in_edge.outputs_ = nil
    }
    (*i).set_in_edge(edge)
  }

  // Add the dyndep-discovered inputs to the edge.
  edge.inputs_.insert(edge.inputs_.end() - edge.order_only_deps_, dyndeps.implicit_inputs_.begin(), dyndeps.implicit_inputs_.end())
  edge.implicit_deps_ += dyndeps.implicit_inputs_.size()

  // Add this edge as outgoing from each new input.
  for (vector<Node*>::const_iterator i = dyndeps.implicit_inputs_.begin(); i != dyndeps.implicit_inputs_.end(); ++i)
    (*i).AddOutEdge(edge)

  return true
}

func (d *DyndepLoader) LoadDyndepFile(file *Node, ddf *DyndepFile, err *string) bool {
  return parser.Load(file.path(), err)
}

