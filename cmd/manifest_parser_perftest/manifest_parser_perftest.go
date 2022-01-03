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

	"github.com/maruel/nin"
)

func WriteFakeManifests(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "build.ninja")); err == nil {
		fmt.Printf("Creating manifest data... [SKIP]\n")
		return nil
	}
	fmt.Printf("Creating manifest data...")
	cmd := exec.Command("python3", filepath.Join("misc", "write_fake_manifests.py"), dir)
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Printf("done.\n")
	return nil
}

func LoadManifests(measureCommandEvaluation bool) int {
	err := ""
	di := nin.NewRealDiskInterface()
	state := nin.NewState()
	parser := nin.NewManifestParser(&state, &di, nin.ManifestParserOptions{})
	if !parser.Load("build.ninja", &err, nil) {
		fmt.Fprintf(os.Stderr, "Failed to read test data: %s\n", err)
		os.Exit(1)
	}
	// Doing an empty build involves reading the manifest and evaluating all
	// commands required for the requested targets. So include command
	// evaluation in the perftest by default.
	optimizationGuard := 0
	if measureCommandEvaluation {
		for _, e := range state.Edges() {
			optimizationGuard += len(e.EvaluateCommand(false))
		}
	}
	return optimizationGuard
}

func mainImpl() error {
	f := flag.Bool("f", false, "only measure manifest load time, not command evaluation time")
	flag.Parse()
	if len(flag.Args()) != 0 {
		return errors.New("unexpected arguments")
	}

	// Disable __pycache__.
	if err := os.Setenv("PYTHONDONTWRITEBYTECODE", "x"); err != nil {
		return err
	}

	kManifestDir := filepath.Join("build", "manifest_perftest")

	if err := WriteFakeManifests(kManifestDir); err != nil {
		return fmt.Errorf("failed to write test data: %s", err)
	}

	if err := os.Chdir(kManifestDir); err != nil {
		return err
	}

	rnd := time.Microsecond
	kNumRepetitions := 5
	var times []time.Duration
	for i := 0; i < kNumRepetitions; i++ {
		start := time.Now()
		optimizationGuard := LoadManifests(!*f)
		delta := time.Since(start)
		fmt.Printf("%s (hash: %x)\n", delta.Round(rnd), optimizationGuard)
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
	avg := total / time.Duration(len(times))
	fmt.Printf("min %s  max %s  avg %s\n", min.Round(rnd), max.Round(rnd), avg.Round(rnd))
	return nil
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "manifest_parser_perftest: %s\n", err)
		os.Exit(1)
	}
}
