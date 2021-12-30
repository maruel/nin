// Copyright 2017 Google Inc. All Rights Reserved.
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

func ToLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func SplitStringPiece(input string, sep byte) []string {
	return strings.Split(input, string(sep))
}

func JoinStringPiece(list []string, sep byte) string {
	return strings.Join(list, string(sep))
}

func EqualsCaseInsensitiveASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	// TODO(maruel): Benchmark if it is a performance optimization or useless.
	_ = b[len(a)-1]
	for i := range a {
		if ToLowerASCII(a[i]) != ToLowerASCII(b[i]) {
			return false
		}
	}
	return true
}
