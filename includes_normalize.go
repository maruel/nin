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
	"errors"
	"strings"
)

// Utility functions for normalizing include paths on Windows.
// TODO: this likely duplicates functionality of CanonicalizePath; refactor.
type includesNormalize struct {
	relativeTo      string
	splitRelativeTo []string
}

func newIncludesNormalize(relativeTo string) (includesNormalize, error) {
	relativeTo, err := absPath(relativeTo)
	return includesNormalize{
		relativeTo:      relativeTo,
		splitRelativeTo: strings.Split(relativeTo, "/"),
	}, err
}

// Return true if paths a and b are on the same windows drive.
// Return false if this function cannot check
// whether or not on the same windows drive.
func sameDriveFast(a string, b string) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}

	if !islatinalpha(a[0]) || !islatinalpha(b[0]) {
		return false
	}

	if toLowerASCII(a[0]) != toLowerASCII(b[0]) {
		return false
	}

	if a[1] != ':' || b[1] != ':' {
		return false
	}

	return isPathSeparator(a[2]) && isPathSeparator(b[2])
}

// Return true if paths a and b are on the same Windows drive.
func sameDrive(a, b string) (bool, error) {
	if sameDriveFast(a, b) {
		return true, nil
	}

	aAbsolute, err := internalGetFullPathName(a)
	if err != nil {
		return false, err
	}
	bAbsolute, err := internalGetFullPathName(b)
	if err != nil {
		return false, err
	}
	return getDrive(aAbsolute) == getDrive(bAbsolute), nil
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
func isFullPathName(s string) bool {
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
func absPath(s string) (string, error) {
	if isFullPathName(s) {
		return strings.ReplaceAll(s, "\\", "/"), nil
	}
	result, err := internalGetFullPathName(s)
	return strings.ReplaceAll(result, "\\", "/"), err
}

func relativize(path string, startList []string) (string, error) {
	absPath, err := absPath(path)
	if err != nil {
		return "", err
	}
	pathList := strings.Split(absPath, "/")
	i := 0
	end := len(startList)
	if end2 := len(pathList); end2 < end {
		end = end2
	}
	for i = 0; i < end; i++ {
		if !equalsCaseInsensitiveASCII(startList[i], pathList[i]) {
			break
		}
	}

	relList := make([]string, 0, len(pathList)-i)
	for j := 0; j < len(startList)-i; j++ {
		relList = append(relList, "..")
	}
	for j := i; j < len(pathList); j++ {
		relList = append(relList, pathList[j])
	}
	if len(relList) == 0 {
		return ".", nil
	}
	return strings.Join(relList, "/"), nil
}

// Normalize by fixing slashes style, fixing redundant .. and . and makes the
// path input relative to relativeTo.
func (i *includesNormalize) Normalize(input string) (string, error) {
	len2 := len(input)
	if len2 >= maxPath {
		return "", errors.New("path too long")
	}
	cp := CanonicalizePath(input)
	absInput, err := absPath(cp)
	if err != nil {
		return "", err
	}

	same, err := sameDrive(absInput, i.relativeTo)
	if err != nil {
		return "", err
	}
	if !same {
		return cp, nil
	}
	return relativize(absInput, i.splitRelativeTo)
}
