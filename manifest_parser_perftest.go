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

//go:build nobuild

package ginja


// Tests manifest parser performance.  Expects to be run in ninja's root
// directory.

func WriteFakeManifests(dir string, err *string) bool {
  var disk_interface RealDiskInterface
  TimeStamp mtime = disk_interface.Stat(dir + "/build.ninja", err)
  if mtime != 0 {  // 0 means that the file doesn't exist yet.
    return mtime != -1
  }

	command := "python misc/write_fake_manifests.py " + dir
  printf("Creating manifest data...")
	fflush(stdout)
  exit_code := system(command)
  printf("done.\n")
  if exit_code != 0 {
    *err = "Failed to run " + command
  }
  return exit_code == 0
}

func LoadManifests(measure_command_evaluation bool) int {
  err := ""
  var disk_interface RealDiskInterface
  var state State
  ManifestParser parser(&state, &disk_interface)
  if !parser.Load("build.ninja", &err) {
    fprintf(stderr, "Failed to read test data: %s\n", err)
    exit(1)
  }
  // Doing an empty build involves reading the manifest and evaluating all
  // commands required for the requested targets. So include command
  // evaluation in the perftest by default.
  optimization_guard := 0
  if measure_command_evaluation {
    for i := 0; i < state.edges_.size(); i++ {
  }
    }
      optimization_guard += state.edges_[i].EvaluateCommand().size()
  return optimization_guard
}

func main(argc int, argv []*char) int {
  measure_command_evaluation := true
  opt := 0
  for (opt = getopt(argc, argv, const_cast<char*>("fh"))) != -1 {
    switch (opt) {
    case 'f':
      measure_command_evaluation = false
      break
    case 'h':
    default:
      printf("usage: manifest_parser_perftest\n\noptions:\n  -f     only measure manifest load time, not command evaluation time\n" )
    return 1
    }
  }

  const char kManifestDir[] = "build/manifest_perftest"

  err := ""
  if !WriteFakeManifests(kManifestDir, &err) {
    fprintf(stderr, "Failed to write test data: %s\n", err)
    return 1
  }

  if chdir(kManifestDir) < 0 {
    Fatal("chdir: %s", strerror(errno))
  }

  kNumRepetitions := 5
  var times []int
  for i := 0; i < kNumRepetitions; i++ {
    start := GetTimeMillis()
    optimization_guard := LoadManifests(measure_command_evaluation)
    int delta = (int)(GetTimeMillis() - start)
    printf("%dms (hash: %x)\n", delta, optimization_guard)
    times.push_back(delta)
  }

  min := *min_element(times.begin(), times.end())
  max := *max_element(times.begin(), times.end())
  total := accumulate(times.begin(), times.end(), 0.0f)
  printf("min %dms  max %dms  avg %.1fms\n", min, max, total / times.size())
}

