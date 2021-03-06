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

package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/maruel/nin"
)

// TODO(maruel): Rewrite as a native Go server anyway, no need to depend on
// python.
const browsePy = "abc"

// Run in "browse" mode, which execs a Python webserver.
// \a ninjaCommand is the command used to invoke ninja.
// \a args are the number of arguments to be passed to the Python script.
// \a argv are arguments to be passed to the Python script.
// This function does not return if it runs successfully.
func runBrowsePython(state *nin.State, ninjaCommand string, inputFile string, args []string) {
	// The original C++ code exec() python as the parent, which is super weird.
	// We cannot do this easily so do it the normal way for now.

	cmd := exec.Command("python3", "-", "--ninja-command", ninjaCommand, "-f", "input_file")
	cmd.Args = append(cmd.Args, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = strings.NewReader(browsePy)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
	}
	os.Exit(0)
}
