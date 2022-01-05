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
	"strings"
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

// Log an informational message.
func infof(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stdout, "nin: ")
	fmt.Fprintf(os.Stdout, msg, s...)
	fmt.Fprintf(os.Stdout, "\n")
}

func islatinalpha(c byte) bool {
	// isalpha() is locale-dependent.
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Removes all Ansi escape codes (http://www.termsys.demon.co.uk/vtansi.htm).
func stripAnsiEscapeCodes(in string) string {
	if strings.IndexByte(in, '\x1B') == -1 {
		return in
	}
	stripped := ""
	//stripped.reserve(in.size())

	for i := 0; i < len(in); i++ {
		if in[i] != '\x1B' {
			// Not an escape code.
			stripped += string(in[i])
			continue
		}

		// Only strip CSIs for now.
		if i+1 >= len(in) {
			break
		}
		if in[i+1] != '[' { // Not a CSI.
			continue
		}
		i += 2

		// Skip everything up to and including the next [a-zA-Z].
		for i < len(in) && !islatinalpha(in[i]) {
			i++
		}
	}
	return stripped
}
