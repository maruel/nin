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

//go:build nobuild

package ginga


inline char ToLowerASCII(char c) {
  return (c >= 'A' && c <= 'Z') ? (c + ('a' - 'A')) : c
}


func SplitStringPiece(input StringPiece, sep char) vector<StringPiece> {
  vector<StringPiece> elems
  elems.reserve(count(input.begin(), input.end(), sep) + 1)

  StringPiece::const_iterator pos = input.begin()

  for (;;) {
    next_pos := find(pos, input.end(), sep)
    if next_pos == input.end() {
      elems.push_back(StringPiece(pos, input.end() - pos))
      break
    }
    elems.push_back(StringPiece(pos, next_pos - pos))
    pos = next_pos + 1
  }

  return elems
}

func JoinStringPiece(list *vector<StringPiece>, sep char) string {
  if len(list) == 0 {
    return ""
  }

  string ret

  {
    size_t cap = list.size() - 1
    for (size_t i = 0; i < list.size(); ++i) {
      cap += list[i].len_
    }
    ret.reserve(cap)
  }

  for (size_t i = 0; i < list.size(); ++i) {
    if i != 0 {
      ret += sep
    }
    ret.append(list[i].str_, list[i].len_)
  }

  return ret
}

func EqualsCaseInsensitiveASCII(a StringPiece, b StringPiece) bool {
  if a.len_ != b.len_ {
    return false
  }

  for (size_t i = 0; i < a.len_; ++i) {
    if ToLowerASCII(a.str_[i]) != ToLowerASCII(b.str_[i]) {
      return false
    }
  }

  return true
}

