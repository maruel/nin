// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(Main())
}

// Log a fatalf message and exit.
func fatalf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: fatal: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
	// On Windows, some tools may inject extra threads.
	// exit() may block on locks held by those threads, so forcibly exit.
	_ = os.Stderr.Sync()
	_ = os.Stdout.Sync()
	os.Exit(1)
}

// Log a warning message.
func warningf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: warning: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}

// Log an error message.
func errorf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: error: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}
