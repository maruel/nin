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

package nin

import (
	"strings"
)

const _MAX_PATH = 259

// Utility functions for normalizing include paths on Windows.
// TODO: this likely duplicates functionality of CanonicalizePath; refactor.
type IncludesNormalize struct {
	relative_to_       string
	split_relative_to_ []string
}

func NewIncludesNormalize(relative_to string) IncludesNormalize {
	err := ""
	relative_to = AbsPath(relative_to, &err)
	if err != "" {
		fatalf("Initializing IncludesNormalize(): %s", err)
	}
	return IncludesNormalize{
		relative_to_:       relative_to,
		split_relative_to_: strings.Split(relative_to, "/"),
	}
}

// Return true if paths a and b are on the same windows drive.
// Return false if this function cannot check
// whether or not on the same windows drive.
func SameDriveFast(a string, b string) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}

	if !islatinalpha(a[0]) || !islatinalpha(b[0]) {
		return false
	}

	if ToLowerASCII(a[0]) != ToLowerASCII(b[0]) {
		return false
	}

	if a[1] != ':' || b[1] != ':' {
		return false
	}

	return isPathSeparator(a[2]) && isPathSeparator(b[2])
}

// Return true if paths a and b are on the same Windows drive.
func SameDrive(a string, b string, err *string) bool {
	if SameDriveFast(a, b) {
		return true
	}

	a_absolute := ""
	b_absolute := ""
	if !InternalGetFullPathName(a, &a_absolute, err) {
		return false
	}
	if !InternalGetFullPathName(b, &b_absolute, err) {
		return false
	}
	return getDrive(a_absolute) == getDrive(b_absolute)
}

func getDrive(s string) string {
	s = strings.TrimPrefix(s, "\\\\?\\")
	if len(s) >= 2 && islatinalpha(s[0]) && s[1] == ':' {
		return s[:2]
	}
	return ""
}

// Check path |s| is FullPath style returned by GetFullPathName.
// This ignores difference of path separator.
// This is used not to call very slow GetFullPathName API.
func IsFullPathName(s string) bool {
	if len(s) < 3 || !islatinalpha(s[0]) || s[1] != ':' || !isPathSeparator(s[2]) {
		return false
	}

	// Check "." or ".." is contained in path.
	for i := 2; i < len(s); i++ {
		if !isPathSeparator(s[i]) {
			continue
		}

		// Check ".".
		if i+1 < len(s) && s[i+1] == '.' && (i+2 >= len(s) || isPathSeparator(s[i+2])) {
			return false
		}

		// Check "..".
		if i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.' && (i+3 >= len(s) || isPathSeparator(s[i+3])) {
			return false
		}
	}

	return true
}

// Internal utilities made available for testing, maybe useful otherwise.
func AbsPath(s string, err *string) string {
	if IsFullPathName(s) {
		return strings.ReplaceAll(s, "\\", "/")
	}

	result := ""
	if !InternalGetFullPathName(s, &result, err) {
		return ""
	}
	return strings.ReplaceAll(result, "\\", "/")
}

func Relativize(path string, start_list []string, err *string) string {
	abs_path := AbsPath(path, err)
	if len(*err) != 0 {
		return ""
	}
	path_list := strings.Split(abs_path, "/")
	i := 0
	end := len(start_list)
	if end2 := len(path_list); end2 < end {
		end = end2
	}
	for i = 0; i < end; i++ {
		if !EqualsCaseInsensitiveASCII(start_list[i], path_list[i]) {
			break
		}
	}

	var rel_list []string
	//rel_list.reserve(len(start_list) - i + len(path_list) - i)
	for j := 0; j < len(start_list)-i; j++ {
		rel_list = append(rel_list, "..")
	}
	for j := i; j < len(path_list); j++ {
		rel_list = append(rel_list, path_list[j])
	}
	if len(rel_list) == 0 {
		return "."
	}
	return JoinStringPiece(rel_list, '/')
}

/// Normalize by fixing slashes style, fixing redundant .. and . and makes the
/// path |input| relative to |this->relative_to_| and store to |result|.
func (i *IncludesNormalize) Normalize(input string, result *string, err *string) bool {
	len2 := len(input)
	if len2 > _MAX_PATH {
		*err = "path too long"
		return false
	}
	cp := CanonicalizePath(input)
	abs_input := AbsPath(cp, err)
	if len(*err) != 0 {
		return false
	}

	if !SameDrive(abs_input, i.relative_to_, err) {
		if len(*err) != 0 {
			return false
		}
		*result = cp
		return true
	}
	*result = Relativize(abs_input, i.split_relative_to_, err)
	return len(*err) == 0
}
