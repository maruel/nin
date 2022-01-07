// Copyright 2016 Google Inc. All Rights Reserved.
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

// TODO(maruel): Create a Status (or LinePrinter?) for test cases that
// redirect to testing.T.Log().

// Status is the interface that tracks the status of a build:
// completion fraction, printing updates.
type Status interface {
	PlanHasTotalEdges(total int)
	BuildEdgeStarted(edge *Edge, startTimeMillis int32)
	BuildEdgeFinished(edge *Edge, endTimeMillis int32, success bool, output string)
	BuildLoadDyndeps()
	BuildStarted()
	BuildFinished()

	Info(msg string, i ...interface{})
	Warning(msg string, i ...interface{})
	Error(msg string, i ...interface{})
}
