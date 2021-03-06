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
	"testing"
)

func TestState_Basic(t *testing.T) {
	state := NewState()

	command := EvalString{
		Parsed: []EvalStringToken{
			{"cat ", false},
			{"in", true},
			{" > ", false},
			{"out", true},
		},
	}
	if got := command.Serialize(); got != "[cat ][$in][ > ][$out]" {
		t.Fatal(got)
	}

	rule := NewRule("cat")
	rule.Bindings["command"] = &command
	state.Bindings.Rules[rule.Name] = rule

	edge := state.addEdge(rule)
	state.addIn(edge, "in1", 0)
	state.addIn(edge, "in2", 0)
	state.addOut(edge, "out", 0)

	if got := edge.EvaluateCommand(false); got != "cat in1 in2 > out" {
		t.Fatal(got)
	}

	if state.GetNode("in1", 0).Dirty {
		t.Fatal("dirty")
	}
	if state.GetNode("in2", 0).Dirty {
		t.Fatal("dirty")
	}
	if state.GetNode("out", 0).Dirty {
		t.Fatal("dirty")
	}
}
