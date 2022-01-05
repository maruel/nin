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

func editDistance(s1, s2 string, allowReplacements bool, maxEditDistance int) int {
	// The algorithm implemented below is the "classic"
	// dynamic-programming algorithm for computing the Levenshtein
	// distance, which is described here:
	//
	//   http://en.wikipedia.org/wiki/LevenshteinDistance
	//
	// Although the algorithm is typically described using an m x n
	// array, only one row plus one element are used at a time, so this
	// implementation just keeps one vector for the row.  To update one entry,
	// only the entries to the left, top, and top-left are needed.  The left
	// entry is in row[x-1], the top entry is what's in row[x] from the last
	// iteration, and the top-left entry is stored in previous.
	m := len(s1)
	n := len(s2)

	row := make([]int, n+1)
	for i := 1; i <= n; i++ {
		row[i] = i
	}

	for y := 1; y <= m; y++ {
		row[0] = y
		bestThisRow := row[0]

		previous := y - 1
		for x := 1; x <= n; x++ {
			oldRow := row[x]
			if allowReplacements {
				v := 0
				if s1[y-1] != s2[x-1] {
					v = 1
				}
				row[x] = min(previous+v, min(row[x-1], row[x])+1)
			} else {
				if s1[y-1] == s2[x-1] {
					row[x] = previous
				} else {
					row[x] = min(row[x-1], row[x]) + 1
				}
			}
			previous = oldRow
			bestThisRow = min(bestThisRow, row[x])
		}

		if maxEditDistance != 0 && bestThisRow > maxEditDistance {
			return maxEditDistance + 1
		}
	}

	return row[n]
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}
