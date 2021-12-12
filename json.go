// Copyright 2021 Google Inc. All Rights Reserved.
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

package ginga



// Encode a string in JSON format without encolsing quotes
func EncodeJSONString(in string) string {
  static string hex_digits = "0123456789abcdef"
  string out
  out.reserve(in.length() * 1.2)
  for (string::const_iterator it = in.begin(); it != in.end(); ++it) {
    c := *it
    if c == '\b' {
      out += "\\b"
    } else if se if (c == '\f' {
      out += "\\f"
    } else if se if (c == '\n' {
      out += "\\n"
    } else if se if (c == '\r' {
      out += "\\r"
    } else if se if (c == '\t' {
      out += "\\t"
    } else if se if (0x0 <= c && c < 0x20 {
      out += "\\u00"
      out += hex_digits[c >> 4]
      out += hex_digits[c & 0xf]
    } else if se if (c == '\\' {
      out += "\\\\"
    } else if se if (c == '\"' {
      out += "\\\""
    } else {
      out += c
    }
  }
  return out
}

// Print a string in JSON format to stdout without enclosing quotes
func PrintJSONString(in string) {
  out := EncodeJSONString(in)
  fwrite(out, 1, out.length(), stdout)
}

