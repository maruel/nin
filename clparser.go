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

import "strings"

// Visual Studio's cl.exe requires some massaging to work with Ninja;
// for example, it emits include information on stderr in a funny
// format when building with /showIncludes.  This class parses this
// output.
type CLParser struct {
	includes_ map[string]struct{}
}

func NewCLParser() CLParser {
	return CLParser{includes_: map[string]struct{}{}}
}

// Parse a line of cl.exe output and extract /showIncludes info.
// If a dependency is extracted, returns a nonempty string.
// Exposed for testing.
func filterShowIncludes(line string, deps_prefix string) string {
	const kDepsPrefixEnglish = "Note: including file: "
	if deps_prefix == "" {
		deps_prefix = kDepsPrefixEnglish
	}
	if strings.HasPrefix(line, deps_prefix) {
		return strings.TrimLeft(line[len(deps_prefix):], " ")
	}
	return ""
}

// Return true if a mentioned include file is a system path.
// Filtering these out reduces dependency information considerably.
func isSystemInclude(path string) bool {
	// TODO(maruel): The C++ code does it only for ASCII.
	path = strings.ToLower(path)
	// TODO: this is a heuristic, perhaps there's a better way?
	return strings.Contains(path, "program files") || strings.Contains(path, "microsoft visual studio")
}

// Parse a line of cl.exe output and return true if it looks like
// it's printing an input filename.  This is a heuristic but it appears
// to be the best we can do.
// Exposed for testing.
func filterInputFilename(line string) bool {
	// TODO(maruel): The C++ code does it only for ASCII.
	line = strings.ToLower(line)
	// TODO: other extensions, like .asm?
	return strings.HasSuffix(line, ".c") ||
		strings.HasSuffix(line, ".cc") ||
		strings.HasSuffix(line, ".cxx") ||
		strings.HasSuffix(line, ".cpp")
}

// Parse the full output of cl, filling filtered_output with the text that
// should be printed (if any). Returns true on success, or false with err
// filled. output must not be the same object as filtered_object.
func (c *CLParser) Parse(output, deps_prefix string, filtered_output *string, err *string) bool {
	defer METRIC_RECORD("CLParser::Parse")()
	// Loop over all lines in the output to process them.
	start := 0
	seen_show_includes := false
	normalizer := NewIncludesNormalize(".")
	for start < len(output) {
		end := strings.IndexAny(output[start:], "\r\n")
		if end == -1 {
			end = len(output)
		} else {
			end += start
		}
		line := output[start:end]

		include := filterShowIncludes(line, deps_prefix)
		if len(include) != 0 {
			seen_show_includes = true
			normalized := ""
			if !normalizer.Normalize(include, &normalized, err) {
				return false
			}
			if !isSystemInclude(normalized) {
				c.includes_[normalized] = struct{}{}
			}
		} else if !seen_show_includes && filterInputFilename(line) {
			// Drop it.
			// TODO: if we support compiling multiple output files in a single
			// cl.exe invocation, we should stash the filename.
		} else {
			*filtered_output += line
			*filtered_output += "\n"
		}

		if end < len(output) && output[end] == '\r' {
			end++
		}
		if end < len(output) && output[end] == '\n' {
			end++
		}
		start = end
	}
	return true
}
