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
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/maruel/nin"
)

const testFilename = "BuildLogPerfTest-tempfile"

type NoDeadPaths struct {
}

func (n *NoDeadPaths) IsPathDead(string) bool {
	return false
}

func WriteTestData() error {
	log := nin.NewBuildLog()
	noDeadPaths := NoDeadPaths{}
	err := ""
	if !log.OpenForWrite(testFilename, &noDeadPaths, &err) {
		return errors.New(err)
	}

	/*
	  A histogram of command lengths in chromium. For example, 407 builds,
	  1.4% of all builds, had commands longer than 32 bytes but shorter than 64.
	       32    407   1.4%
	       64    183   0.6%
	      128   1461   5.1%
	      256    791   2.8%
	      512   1314   4.6%
	     1024   6114  21.3%
	     2048  11759  41.0%
	     4096   2056   7.2%
	     8192   4567  15.9%
	    16384     13   0.0%
	    32768      4   0.0%
	    65536      5   0.0%
	  The average command length is 4.1 kB and there were 28674 commands in total,
	  which makes for a total log size of ~120 MB (also counting output filenames).

	  Based on this, write 30000 many 4 kB long command lines.
	*/

	// ManifestParser is the only object allowed to create Rules.
	kRuleSize := 4000
	longRuleCommand := "gcc "
	for i := 0; len(longRuleCommand) < kRuleSize; i++ {
		longRuleCommand += fmt.Sprintf("-I../../and/arbitrary/but/fairly/long/path/suffixed/%d ", i)
	}
	longRuleCommand += "$in -o $out\n"

	state := nin.NewState()
	parser := nin.NewManifestParser(&state, nil, nin.ManifestParserOptions{})
	if !parser.ParseTest("rule cxx\n  command = "+longRuleCommand, &err) {
		return errors.New(err)
	}

	// Create build edges. Using ManifestParser is as fast as using the State api
	// for edge creation, so just use that.
	kNumCommands := int32(30000)
	buildRules := ""
	for i := int32(0); i < kNumCommands; i++ {
		buildRules += fmt.Sprintf("build input%d.o: cxx input%d.cc\n", i, i)
	}

	if !parser.ParseTest(buildRules, &err) {
		return errors.New(err)
	}

	for i := int32(0); i < kNumCommands; i++ {
		log.RecordCommand(state.Edges()[i] /*startTime=*/, 100*i /*endTime=*/, 100*i+1 /*mtime=*/, 0)
	}

	return nil
}

func mainImpl() error {
	if err := WriteTestData(); err != nil {
		return fmt.Errorf("failed to write test data: %w", err)
	}

	err := ""
	{
		// Read once to warm up disk cache.
		log := nin.NewBuildLog()
		if log.Load(testFilename, &err) == nin.LoadError {
			return fmt.Errorf("failed to read test data: %s", err)
		}
	}

	rnd := time.Microsecond
	var times []time.Duration
	kNumRepetitions := 5
	for i := 0; i < kNumRepetitions; i++ {
		start := time.Now()
		log := nin.NewBuildLog()
		if log.Load(testFilename, &err) == nin.LoadError {
			return fmt.Errorf("failed to read test data: %s", err)
		}
		delta := time.Since(start)
		fmt.Printf("%s\n", delta.Round(rnd))
		times = append(times, delta)
	}

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
	return os.Remove(testFilename)
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "build_log_perftest: %s\n", err)
		os.Exit(1)
	}
}
