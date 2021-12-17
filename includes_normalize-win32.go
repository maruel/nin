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

//go:build nobuild

package ginja


func InternalGetFullPathName(file_name *string, buffer *char, buffer_length size_t, err *string) bool {
  result_size := GetFullPathNameA(file_name.AsString(), buffer_length, buffer, nil)
  if result_size == 0 {
    *err = "GetFullPathNameA(" + file_name.AsString() + "): " +
        GetLastErrorString()
    return false
  } else if result_size > buffer_length {
    *err = "path too long"
    return false
  }
  return true
}

func IsPathSeparator(c char) bool {
  return c == '/' ||  c == '\\'
}

// Return true if paths a and b are on the same windows drive.
// Return false if this function cannot check
// whether or not on the same windows drive.
func SameDriveFast(a string, b string) bool {
  if a.size() < 3 || b.size() < 3 {
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

  return IsPathSeparator(a[2]) && IsPathSeparator(b[2])
}

// Return true if paths a and b are on the same Windows drive.
func SameDrive(a string, b string, err *string) bool {
  if SameDriveFast(a, b) {
    return true
  }

  char a_absolute[_MAX_PATH]
  char b_absolute[_MAX_PATH]
  if !InternalGetFullPathName(a, a_absolute, sizeof(a_absolute), err) {
    return false
  }
  if !InternalGetFullPathName(b, b_absolute, sizeof(b_absolute), err) {
    return false
  }
  char a_drive[_MAX_DIR]
  char b_drive[_MAX_DIR]
  _splitpath(a_absolute, a_drive, nil, nil, nil)
  _splitpath(b_absolute, b_drive, nil, nil, nil)
  return _stricmp(a_drive, b_drive) == 0
}

// Check path |s| is FullPath style returned by GetFullPathName.
// This ignores difference of path separator.
// This is used not to call very slow GetFullPathName API.
func IsFullPathName(s string) bool {
  if s.size() < 3 || !islatinalpha(s[0]) || s[1] != ':' || !IsPathSeparator(s[2]) {
    return false
  }

  // Check "." or ".." is contained in path.
  for i := 2; i < s.size(); i++ {
    if !IsPathSeparator(s[i]) {
      continue
    }

    // Check ".".
    if i + 1 < s.size() && s[i+1] == '.' && (i + 2 >= s.size() || IsPathSeparator(s[i+2])) {
      return false
    }

    // Check "..".
    if i + 2 < s.size() && s[i+1] == '.' && s[i+2] == '.' && (i + 3 >= s.size() || IsPathSeparator(s[i+3])) {
      return false
    }
  }

  return true
}

IncludesNormalize::IncludesNormalize(string relative_to) {
  string err
  relative_to_ = AbsPath(relative_to, &err)
  if (!err.empty()) {
    Fatal("Initializing IncludesNormalize(): %s", err)
  }
  split_relative_to_ = SplitStringPiece(relative_to_, '/')
}

func (i *IncludesNormalize) AbsPath(s string, err *string) string {
  if IsFullPathName(s) {
    result := s.AsString()
    for i := 0; i < result.size(); i++ {
      if result[i] == '\\' {
        result[i] = '/'
      }
    }
    return result
  }

  char result[_MAX_PATH]
  if !InternalGetFullPathName(s, result, sizeof(result), err) {
    return ""
  }
  for c := result; *c; c++ {
    if *c == '\\' {
  }
      *c = '/'
    }
  return result
}

func (i *IncludesNormalize) Relativize(path string, start_list *vector<string>, err *string) string {
  abs_path := AbsPath(path, err)
  if len(err) != 0 {
    return ""
  }
  vector<string> path_list = SplitStringPiece(abs_path, '/')
  i := 0
  for i = 0; i < static_cast<int>(min(start_list.size(), path_list.size())); i++ {
    if !EqualsCaseInsensitiveASCII(start_list[i], path_list[i]) {
      break
    }
  }

  var rel_list []string
  rel_list.reserve(start_list.size() - i + path_list.size() - i)
  for j := 0; j < static_cast<int>(start_list.size() - i); j++ {
    rel_list.push_back("..")
  }
  for j := i; j < static_cast<int>(path_list.size()); j++ {
    rel_list.push_back(path_list[j])
  }
  if rel_list.size() == 0 {
    return "."
  }
  return JoinStringPiece(rel_list, '/')
}

func (i *IncludesNormalize) Normalize(input string, result *string, err *string) bool {
  char copy[_MAX_PATH + 1]
  len := input.size()
  if len > _MAX_PATH {
    *err = "path too long"
    return false
  }
  strncpy(copy, input, input.size() + 1)
  var slash_bits uint64
  CanonicalizePath(copy, &len, &slash_bits)
  string partially_fixed(copy, len)
  abs_input := AbsPath(partially_fixed, err)
  if len(err) != 0 {
    return false
  }

  if !SameDrive(abs_input, relative_to_, err) {
    if len(err) != 0 {
      return false
    }
    *result = partially_fixed.AsString()
    return true
  }
  *result = Relativize(abs_input, split_relative_to_, err)
  if len(err) != 0 {
    return false
  }
  return true
}

