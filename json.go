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

package nin

import (
	"os"
)

func EncodeJSONString(in string) string {
	hex_digits := "0123456789abcdef"
	out := ""
	//out.reserve(in.length() * 1.2)
	for _, c := range in {
		switch c {
		case '\b':
			out += "\\b"
		case '\f':
			out += "\\f"
		case '\n':
			out += "\\n"
		case '\r':
			out += "\\r"
		case '\t':
			out += "\\t"
		case '\\':
			out += "\\\\"
		case '"':
			out += "\\\""
		default:
			if 0x0 <= c && c < 0x20 {
				out += "\\u00"
				out += hex_digits[c>>4 : (c>>4)+1]
				out += hex_digits[c&0xf : (c&0xf)+1]
			} else {
				out += string(c)
			}
		}
	}
	return out
}

// Print a string in JSON format to stdout without enclosing quotes
func PrintJSONString(in string) {
	//b, _ := json.Marshal(in)
	b := EncodeJSONString(in)
	_, _ = os.Stdout.WriteString(b)
}
