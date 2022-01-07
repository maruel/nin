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

package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/maruel/nin"
)

func random(low, high int) int {
	return rand.Intn(high-low+1) + low
}

func randomCommand() string {
	len2 := random(5, 100)
	s := make([]byte, len2)
	for i := 0; i < len2; i++ {
		s[i] = byte(random(32, 127))
	}
	return string(s)
}

type item struct {
	hash uint64
	i    int
}

func main() {
	const N = 20 * 1000 * 1000

	commands := [N]string{}
	hashes := [N]item{}

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < N; i++ {
		commands[i] = randomCommand()
		hashes[i] = item{nin.HashCommand(commands[i]), i}
	}

	sort.Slice(hashes[:], func(i, j int) bool {
		return hashes[i].hash < hashes[j].hash
	})

	collisionCount := 0
	for i := 1; i < N; i++ {
		if hashes[i-1].hash == hashes[i].hash {
			lhs := commands[hashes[i-1].i]
			rhs := commands[hashes[i].i]
			if lhs != rhs {
				fmt.Printf("collision!\n  string 1: '%s'\n  string 2: '%s'\n", lhs, rhs)
				collisionCount++
			}
		}
	}
	fmt.Printf("\n\n%d collisions after %d runs\n", collisionCount, N)
}
