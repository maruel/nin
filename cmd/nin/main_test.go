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

import "testing"

func TestStripAnsiEscapeCodes_EscapeAtEnd(t *testing.T) {
	stripped := stripAnsiEscapeCodes("foo\x1B")
	if "foo" != stripped {
		t.Fatalf("%+q", stripped)
	}

	stripped = stripAnsiEscapeCodes("foo\x1B[")
	if "foo" != stripped {
		t.Fatalf("%+q", stripped)
	}
}

func TestStripAnsiEscapeCodes_StripColors(t *testing.T) {
	// An actual clang warning.
	input := "\x1B[1maffixmgr.cxx:286:15: \x1B[0m\x1B[0;1;35mwarning: \x1B[0m\x1B[1musing the result... [-Wparentheses]\x1B[0m"
	stripped := stripAnsiEscapeCodes(input)
	if "affixmgr.cxx:286:15: warning: using the result... [-Wparentheses]" != stripped {
		t.Fatalf("%+q", stripped)
	}
}
