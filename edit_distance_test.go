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

package ginja

import "testing"

func TestEditDistanceTest_TestEmpty(t *testing.T) {
	if 5 != EditDistance("", "ninja", true, 0) {
		t.Fatal("expected equal")
	}
	if 5 != EditDistance("ninja", "", true, 0) {
		t.Fatal("expected equal")
	}
	if 0 != EditDistance("", "", true, 0) {
		t.Fatal("expected equal")
	}
}

func TestEditDistanceTest_TestMaxDistance(t *testing.T) {
	allow_replacements := true
	for max_distance := 1; max_distance < 7; max_distance++ {
		if max_distance+1 != EditDistance("abcdefghijklmnop", "ponmlkjihgfedcba", allow_replacements, max_distance) {
			t.Fatal("expected equal")
		}
	}
}

func TestEditDistanceTest_TestAllowReplacements(t *testing.T) {
	allow_replacements := true
	if 1 != EditDistance("ninja", "njnja", allow_replacements, 0) {
		t.Fatal("expected equal")
	}
	if 1 != EditDistance("njnja", "ninja", allow_replacements, 0) {
		t.Fatal("expected equal")
	}

	allow_replacements = false
	if 2 != EditDistance("ninja", "njnja", allow_replacements, 0) {
		t.Fatal("expected equal")
	}
	if 2 != EditDistance("njnja", "ninja", allow_replacements, 0) {
		t.Fatal("expected equal")
	}
}

func TestEditDistanceTest_TestBasics(t *testing.T) {
	if 0 != EditDistance("browser_tests", "browser_tests", true, 0) {
		t.Fatal("expected equal")
	}
	if 1 != EditDistance("browser_test", "browser_tests", true, 0) {
		t.Fatal("expected equal")
	}
	if 1 != EditDistance("browser_tests", "browser_test", true, 0) {
		t.Fatal("expected equal")
	}
}
