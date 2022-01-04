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

import "unsafe"

// murmurHash2, by Austin Appleby
func murmurHash2(key []byte, len2 uint32) uint32 {
	const seed = uint32(0xDECAFBAD)
	const m = uint32(0x5bd1e995)
	const r = 24
	h := uint32(seed ^ len2)
	data := 0
	for len2 >= 4 {
		k := *(*uint32)(unsafe.Pointer(&key[data*4]))
		k *= m
		k ^= k >> r
		k *= m
		h *= m
		h ^= k
		data += 4
		len2 -= 4
	}
	switch len2 {
	case 3:
		h ^= uint32(key[data*4+2]) << 16
		fallthrough
	case 2:
		h ^= uint32(key[data*4+1]) << 8
		fallthrough
	case 1:
		h ^= uint32(key[data*4])
		h *= m
	}
	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}

/*
template<>
type hash struct {

  sizeT operator()(string key) const {
    return MurmurHash2(key.str, key.len)
  }

type argumentType string
type resultType uint

using stdext::hashMap
using stdext::hashCompare

type StringPieceCmp struct {
  sizeT operator()(const string& key) const {
    return MurmurHash2(key.str, key.len)
  }
  bool operator()(const string& a, const string& b) const {
    int cmp = memcmp(a.str, b.str, min(a.len, b.len))
    if (cmp < 0) {
      return true
    } else if (cmp > 0) {
      return false
    } else {
      return a.len < b.len
    }
  }
}

// A template for hashMaps keyed by a StringPiece whose string is
// owned externally (typically by the values).  Use like:
// ExternalStringHash<Foo*>::Type foos; to make foos into a hash
// mapping StringPiece => Foo*.
template<typename V>
type ExternalStringHashMap struct {
}
type Type unorderedMap<string, V>
type Type hashMap<string, V, StringPieceCmp>
*/
