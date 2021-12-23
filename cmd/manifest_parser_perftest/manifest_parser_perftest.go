// Copyright 2014 Google Inc. All Rights Reserved.
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

// Tests manifest parser performance.  Expects to be run in ninja's root
// directory.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/maruel/ginja"
)

func WriteFakeManifests(dir string) error {
	/*
		err := ""
		mtime := disk_interface.Stat(dir+"/build.ninja", err)
		if mtime != 0 { // 0 means that the file doesn't exist yet.
			return mtime != -1
		}
	*/
	fmt.Printf("Creating manifest data...")
	cmd := exec.Command("python3", filepath.Join("misc", "write_fake_manifests.py"), dir)
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Printf("done.\n")
	return nil
}

func LoadManifests(measure_command_evaluation bool) int {
	err := ""
	disk_interface := ginja.NewRealDiskInterface()
	state := ginja.NewState()
	parser := ginja.NewManifestParser(&state, &disk_interface, ginja.ManifestParserOptions{})
	if !parser.Load("build.ninja", &err, nil) {
		fmt.Fprintf(os.Stderr, "Failed to read test data: %s\n", err)
		os.Exit(1)
	}
	// Doing an empty build involves reading the manifest and evaluating all
	// commands required for the requested targets. So include command
	// evaluation in the perftest by default.
	optimization_guard := 0
	if measure_command_evaluation {
		panic("TODO export")
		//for i := 0; i < len(state.edges_); i++ {
		//}
	}
	panic("TODO export")
	//optimization_guard += len(state.edges_[i].EvaluateCommand())
	return optimization_guard
}

func mainImpl() error {
	f := flag.Bool("f", false, "only measure manifest load time, not command evaluation time")
	flag.Parse()
	if len(flag.Args()) != 0 {
		return errors.New("unexpected arguments")
	}

	kManifestDir := filepath.Join("build", "manifest_perftest")

	if err := WriteFakeManifests(kManifestDir); err != nil {
		return fmt.Errorf("failed to write test data: %s", err)
	}

	if err := os.Chdir(kManifestDir); err != nil {
		return err
	}

	kNumRepetitions := 5
	var times []time.Duration
	for i := 0; i < kNumRepetitions; i++ {
		start := time.Now()
		optimization_guard := LoadManifests(!*f)
		delta := time.Since(start)
		fmt.Printf("%s (hash: %x)\n", delta, optimization_guard)
		times = append(times, delta)
	}

	min := times[0]
	max := times[0]
	total := times[0]
	for i := 1; i < len(times); i++ {
		if min > times[i] {
			min = times[i]
		}
		if max < times[i] {
			max = times[i]
		}
		total += times[i]
	}
	fmt.Printf("min %dms  max %dms  avg %.1fms\n", min, max, total/time.Duration(len(times)))
	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "manifest_parser_perftest: %s\n", err)
		os.Exit(1)
	}
}
