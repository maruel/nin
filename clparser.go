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


// Visual Studio's cl.exe requires some massaging to work with Ninja;
// for example, it emits include information on stderr in a funny
// format when building with /showIncludes.  This class parses this
// output.
type CLParser struct {
  // Parse a line of cl.exe output and extract /showIncludes info.
  // If a dependency is extracted, returns a nonempty string.
  // Exposed for testing.
  static string FilterShowIncludes(string line, string deps_prefix)

  // Return true if a mentioned include file is a system path.
  // Filtering these out reduces dependency information considerably.
  static bool IsSystemInclude(string path)

  // Parse a line of cl.exe output and return true if it looks like
  // it's printing an input filename.  This is a heuristic but it appears
  // to be the best we can do.
  // Exposed for testing.
  static bool FilterInputFilename(string line)

  set<string> includes_
}


// Return true if \a input ends with \a needle.
func EndsWith(input string, needle string) bool {
  return (input.size() >= needle.size() && input.substr(input.size() - needle.size()) == needle)
}

// static
func (c *CLParser) FilterShowIncludes(line string, deps_prefix string) string {
  const string kDepsPrefixEnglish = "Note: including file: "
  in := line
  end := in + line.size()
  prefix := deps_prefix.empty() ? kDepsPrefixEnglish : deps_prefix
  if end - in > (int)prefix.size() && memcmp(in, prefix, (int)prefix.size()) == 0 {
    in += prefix.size()
    while (*in == ' ')
      ++in
    return line.substr(in - line)
  }
  return ""
}

// static
func (c *CLParser) IsSystemInclude(path string) bool {
  transform(path.begin(), path.end(), path.begin(), ToLowerASCII)
  // TODO: this is a heuristic, perhaps there's a better way?
  return (path.find("program files") != string::npos || path.find("microsoft visual studio") != string::npos)
}

// static
func (c *CLParser) FilterInputFilename(line string) bool {
  transform(line.begin(), line.end(), line.begin(), ToLowerASCII)
  // TODO: other extensions, like .asm?
  return EndsWith(line, ".c") ||
      EndsWith(line, ".cc") ||
      EndsWith(line, ".cxx") ||
      EndsWith(line, ".cpp")
}

// static
func (c *CLParser) Parse(output string, deps_prefix string, filtered_output *string, err *string) bool {
  METRIC_RECORD("CLParser::Parse")

  // Loop over all lines in the output to process them.
  assert(&output != filtered_output)
  size_t start = 0
  seen_show_includes := false
  IncludesNormalize normalizer(".")

  while (start < output.size()) {
    size_t end = output.find_first_of("\r\n", start)
    if end == string::npos {
      end = output.size()
    }
    line := output.substr(start, end - start)

    include := FilterShowIncludes(line, deps_prefix)
    if len(include) != 0 {
      seen_show_includes = true
      string normalized
      if !normalizer.Normalize(include, &normalized, err) {
        return false
      }
      // TODO: should this make the path relative to cwd?
      normalized = include
      uint64_t slash_bits
      CanonicalizePath(&normalized, &slash_bits)
      if !IsSystemInclude(normalized) {
        includes_.insert(normalized)
      }
    } else if se if (!seen_show_includes && FilterInputFilename(line) {
      // Drop it.
      // TODO: if we support compiling multiple output files in a single
      // cl.exe invocation, we should stash the filename.
    } else {
      filtered_output.append(line)
      filtered_output.append("\n")
    }

    if end < output.size() && output[end] == '\r' {
      ++end
    }
    if end < output.size() && output[end] == '\n' {
      ++end
    }
    start = end
  }

  return true
}

