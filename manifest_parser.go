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

// ParseManifestConcurrency defines the concurrency parameters when parsing
// manifest (build.ninja files).
type ParseManifestConcurrency int32

const (
	// ParseManifestSerial parses files serially in the same way that the C++
	// ninja implementation does. It is the most compatible way.
	//
	// This reduces CPU usage, at the cost of higher latency.
	ParseManifestSerial ParseManifestConcurrency = iota
	// ParseManifestPrewarmSubninja loads files serially except that subninjas
	// are processed at the very end. This gives a latency improvement when a
	// significant number of subninjas are processed because subninja files can
	// be read from disk concurrently. This causes subninja files to be processed
	// out of order.
	ParseManifestPrewarmSubninja
	// ParseManifestConcurrentParsing parses all subninjas concurrently.
	ParseManifestConcurrentParsing
)

func (p ParseManifestConcurrency) String() string {
	switch p {
	case ParseManifestSerial:
		return "Serial"
	case ParseManifestPrewarmSubninja:
		return "PrewarmSubninja"
	case ParseManifestConcurrentParsing:
		return "Concurrent"
	default:
		return "Invalid"
	}
}

// ParseManifestOpts are the options when parsing a build.ninja file.
type ParseManifestOpts struct {
	// ErrOnDupeEdge causes duplicate rules for one target to print an error,
	// otherwise warns.
	ErrOnDupeEdge bool
	// ErrOnPhonyCycle causes phony cycles to print an error, otherwise warns.
	ErrOnPhonyCycle bool
	// Quiet silences warnings.
	Quiet bool
	// Concurrency defines the parsing concurrency.
	Concurrency ParseManifestConcurrency
}

// ParseManifest parses a manifest file (i.e. build.ninja).
//
// The input must contain a trailing terminating zero byte.
func ParseManifest(state *State, fr FileReader, options ParseManifestOpts, filename string, input []byte) error {
	m := manifestParserSerial{
		fr:      fr,
		options: options,
		state:   state,
		env:     state.Bindings,
	}
	return m.parse(filename, input)
}

// subninja is a struct used to manage parallel reading of subninja files.
type subninja struct {
	filename string
	input    []byte
	err      error
	ls       lexerState // lexer state when the subninja statement was parsed.
}

// readSubninjaAsync is the goroutine that reads the subninja file in parallel
// to the main build.ninja to reduce overall latency.
func readSubninjaAsync(fr FileReader, filename string, ch chan<- subninja, ls lexerState) {
	input, err := fr.ReadFile(filename)
	ch <- subninja{
		filename: filename,
		input:    input,
		err:      err,
		ls:       ls,
	}
}
