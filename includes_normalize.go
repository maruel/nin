// Copyright 2012 Google Inc. All Rights Reserved.
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

// Utility functions for normalizing include paths on Windows.
// TODO: this likely duplicates functionality of CanonicalizePath; refactor.
type IncludesNormalize struct {
	relative_to_       string
	split_relative_to_ []string
}

/// Normalize by fixing slashes style, fixing redundant .. and . and makes the
/// path |input| relative to |this->relative_to_| and store to |result|.
func (i *IncludesNormalize) Normalize(input string, result, err *string) bool {
	panic("TODO")
	return false
}

// Internal utilities made available for testing, maybe useful otherwise.
func AbsPath(s string, err *string) string {
	panic("TODO")
	return ""
}

func Relativize(path string, start_list []string, err *string) string {
	panic("TODO")
	return ""
}
