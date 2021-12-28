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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/maruel/ginja"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s <file1> <file2...>\n", os.Args[0])
		os.Exit(1)
	}

	rnd := time.Microsecond
	var times []time.Duration
	for _, filename := range os.Args[1:] {
		for limit := 1 << 10; limit < (1 << 20); limit *= 2 {
			start := time.Now()
			for rep := 0; rep < limit; rep++ {
				buf, err := ioutil.ReadFile(filename)
				if err != nil {
					fmt.Printf("%s: %s\n", filename, err)
					os.Exit(1)
				}

				err2 := ""
				parser := ginja.NewDepfileParser(ginja.DepfileParserOptions{})
				if !parser.Parse(buf, &err2) {
					fmt.Printf("%s: %s\n", filename, err2)
					os.Exit(1)
				}
			}
			delta := time.Since(start)

			if delta > 100*time.Millisecond {
				time := delta / time.Duration(limit)
				fmt.Printf("%s: %s\n", filename, time.Round(rnd))
				times = append(times, time)
				break
			}
		}
	}

	if len(times) != 0 {
		min := times[0]
		max := times[0]
		total := time.Duration(0)
		for i := 0; i < len(times); i++ {
			total += times[i]
			if times[i] < min {
				min = times[i]
			} else if times[i] > max {
				max = times[i]
			}
		}

		avg := total / time.Duration(len(times))
		fmt.Printf("min %s  max %s  avg %s\n", min.Round(rnd), max.Round(rnd), avg.Round(rnd))
	}
}
