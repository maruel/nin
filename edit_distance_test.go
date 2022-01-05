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

import "testing"

func TestEditDistanceTest_TestEmpty(t *testing.T) {
	if 5 != editDistance("", "ninja", true, 0) {
		t.Fatal("expected equal")
	}
	if 5 != editDistance("ninja", "", true, 0) {
		t.Fatal("expected equal")
	}
	if 0 != editDistance("", "", true, 0) {
		t.Fatal("expected equal")
	}
}

func TestEditDistanceTest_TestMaxDistance(t *testing.T) {
	allowReplacements := true
	for maxDistance := 1; maxDistance < 7; maxDistance++ {
		if maxDistance+1 != editDistance("abcdefghijklmnop", "ponmlkjihgfedcba", allowReplacements, maxDistance) {
			t.Fatal("expected equal")
		}
	}
}

func TestEditDistanceTest_TestAllowReplacements(t *testing.T) {
	allowReplacements := true
	if 1 != editDistance("ninja", "njnja", allowReplacements, 0) {
		t.Fatal("expected equal")
	}
	if 1 != editDistance("njnja", "ninja", allowReplacements, 0) {
		t.Fatal("expected equal")
	}

	allowReplacements = false
	if 2 != editDistance("ninja", "njnja", allowReplacements, 0) {
		t.Fatal("expected equal")
	}
	if 2 != editDistance("njnja", "ninja", allowReplacements, 0) {
		t.Fatal("expected equal")
	}
}

func TestEditDistanceTest_TestBasics(t *testing.T) {
	if 0 != editDistance("browser_tests", "browser_tests", true, 0) {
		t.Fatal("expected equal")
	}
	if 1 != editDistance("browser_test", "browser_tests", true, 0) {
		t.Fatal("expected equal")
	}
	if 1 != editDistance("browser_tests", "browser_test", true, 0) {
		t.Fatal("expected equal")
	}
}
