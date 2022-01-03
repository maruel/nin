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

	var command EvalString
	command.AddText("cat ")
	command.AddSpecial("in")
	command.AddText(" > ")
	command.AddSpecial("out")
	if got := command.Serialize(); got != "[cat ][$in][ > ][$out]" {
		t.Fatal(got)
	}

	rule := NewRule("cat")
	rule.Bindings["command"] = &command
	state.bindings.Rules[rule.Name] = rule

	edge := state.AddEdge(rule)
	state.AddIn(edge, "in1", 0)
	state.AddIn(edge, "in2", 0)
	state.AddOut(edge, "out", 0)

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
