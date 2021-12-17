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

//go:build nobuild

package ginja


// MurmurHash2, by Austin Appleby
static inline
unsigned int MurmurHash2(const void* key, size_t len) {
  static const unsigned int seed = 0xDECAFBAD
  const unsigned int m = 0x5bd1e995
  const int r = 24
  unsigned int h = seed ^ len
  const unsigned char* data = (const unsigned char*)key
  while (len >= 4) {
    unsigned int k
    memcpy(&k, data, sizeof k)
    k *= m
    k ^= k >> r
    k *= m
    h *= m
    h ^= k
    data += 4
    len -= 4
  }
  switch (len) {
  case 3: h ^= data[2] << 16
          NINJA_FALLTHROUGH
  case 2: h ^= data[1] << 8
          NINJA_FALLTHROUGH
  case 1: h ^= data[0]
    h *= m
  }
  h ^= h >> 13
  h *= m
  h ^= h >> 15
  return h
}

template<>
type hash struct {
  typedef StringPiece argument_type
  typedef size_t result_type

  size_t operator()(StringPiece key) const {
    return MurmurHash2(key.str_, key.len_)
  }
}

using stdext::hash_map
using stdext::hash_compare

type StringPieceCmp struct {
  size_t operator()(const StringPiece& key) const {
    return MurmurHash2(key.str_, key.len_)
  }
  bool operator()(const StringPiece& a, const StringPiece& b) const {
    int cmp = memcmp(a.str_, b.str_, min(a.len_, b.len_))
    if (cmp < 0) {
      return true
    } else if (cmp > 0) {
      return false
    } else {
      return a.len_ < b.len_
    }
  }
}

using __gnu_cxx::hash_map

template<>
type hash struct {
  size_t operator()(StringPiece key) const {
    return MurmurHash2(key.str_, key.len_)
  }
}

// A template for hash_maps keyed by a StringPiece whose string is
// owned externally (typically by the values).  Use like:
// ExternalStringHash<Foo*>::Type foos; to make foos into a hash
// mapping StringPiece => Foo*.
template<typename V>
type ExternalStringHashMap struct {
  typedef unordered_map<StringPiece, V> Type
  typedef hash_map<StringPiece, V, StringPieceCmp> Type
  typedef hash_map<StringPiece, V> Type
}

