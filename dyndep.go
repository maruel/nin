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

package nin

import (
	"errors"
	"fmt"
)

// Dyndeps stores dynamically-discovered dependency information for one edge.
type Dyndeps struct {
	used            bool
	restat          bool
	implicitInputs  []*Node
	implicitOutputs []*Node
}

func (d *Dyndeps) String() string {
	out := "Dyndeps{in:"
	for i, n := range d.implicitInputs {
		if i != 0 {
			out += ","
		}
		out += n.Path
	}
	out += "; out:"
	for i, n := range d.implicitOutputs {
		if i != 0 {
			out += ","
		}
		out += n.Path
	}
	return out + "}"
}

// DyndepFile stores data loaded from one dyndep file.
//
// Map from an edge to its dynamically-discovered dependency information.
type DyndepFile map[*Edge]*Dyndeps

// DyndepLoader loads dynamically discovered dependencies, as
// referenced via the "dyndep" attribute in build files.
type DyndepLoader struct {
	state *State
	di    DiskInterface
}

// NewDyndepLoader returns an initialized DyndepLoader.
func NewDyndepLoader(state *State, di DiskInterface) DyndepLoader {
	return DyndepLoader{
		state: state,
		di:    di,
	}
}

// LoadDyndeps loads a dyndep file from the given node's path and update the
// build graph with the new information.
//
// Caller can optionally provide a 'DyndepFile' object in which to store the
// information loaded from the dyndep file.
func (d *DyndepLoader) LoadDyndeps(node *Node, ddf DyndepFile) error {
	// We are loading the dyndep file now so it is no longer pending.
	node.DyndepPending = false

	// Load the dyndep information from the file.
	explain("loading dyndep file '%s'", node.Path)
	if err := d.loadDyndepFile(node, ddf); err != nil {
		return err
	}

	// Update each edge that specified this node as its dyndep binding.
	outEdges := node.OutEdges
	for _, oe := range outEdges {
		edge := oe
		if edge.Dyndep != node {
			continue
		}

		ddi, ok := ddf[edge]
		if !ok {
			return errors.New("'" + edge.Outputs[0].Path + "' not mentioned in its dyndep file '" + node.Path + "'")
		}

		ddi.used = true
		dyndeps := ddi
		if err := d.updateEdge(edge, dyndeps); err != nil {
			return err
		}
	}

	// Reject extra outputs in dyndep file.
	for edge, oe := range ddf {
		if !oe.used {
			return errors.New("dyndep file '" + node.Path + "' mentions output '" + edge.Outputs[0].Path + "' whose build statement does not have a dyndep binding for the file")
		}
	}
	return nil
}

func (d *DyndepLoader) updateEdge(edge *Edge, dyndeps *Dyndeps) error {
	// Add dyndep-discovered bindings to the edge.
	// We know the edge already has its own binding
	// scope because it has a "dyndep" binding.
	if dyndeps.restat {
		edge.Env.Bindings["restat"] = "1"
	}

	// Add the dyndep-discovered outputs to the edge.
	edge.Outputs = append(edge.Outputs, dyndeps.implicitOutputs...)
	edge.ImplicitOuts += int32(len(dyndeps.implicitOutputs))

	// Add this edge as incoming to each new output.
	for _, i := range dyndeps.implicitOutputs {
		if oldInEdge := i.InEdge; oldInEdge != nil {
			// This node already has an edge producing it.  Fail with an error
			// unless the edge was generated by ImplicitDepLoader, in which
			// case we can replace it with the now-known real producer.
			if !oldInEdge.GeneratedByDepLoader {
				return errors.New("multiple rules generate " + i.Path)
			}
			oldInEdge.Outputs = nil
		}
		i.InEdge = edge
	}

	// Add the dyndep-discovered inputs to the edge.
	old := edge.Inputs
	offset := len(edge.Inputs) - int(edge.OrderOnlyDeps)
	edge.Inputs = make([]*Node, len(edge.Inputs)+len(dyndeps.implicitInputs))
	copy(edge.Inputs, old[:offset])
	copy(edge.Inputs[offset:], dyndeps.implicitInputs)
	copy(edge.Inputs[offset+len(dyndeps.implicitInputs):], old[offset:])
	edge.ImplicitDeps += int32(len(dyndeps.implicitInputs))

	// Add this edge as outgoing from each new input.
	for _, n := range dyndeps.implicitInputs {
		n.OutEdges = append(n.OutEdges, edge)
	}
	return nil
}

func (d *DyndepLoader) loadDyndepFile(file *Node, ddf DyndepFile) error {
	contents, err := d.di.ReadFile(file.Path)
	if err != nil {
		return fmt.Errorf("loading '%s': %w", file.Path, err)
	}
	return ParseDyndep(d.state, ddf, file.Path, contents)
}
