#!/bin/bash
# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu

# Disable __pycache__.
export PYTHONDONTWRITEBYTECODE=x

if [ ! -f ./ninja ]; then
  python3 ./configure.py --bootstrap
fi

# Make sure re2c is compiled before, otherwise it will affect the first run.
./regen.sh

echo "Building ninja_test from scratch with both ninja and nin."
echo ""
echo "./ninja --quiet -d stats ninja_test"
rm -rf build ninja_test
time ./ninja --quiet -d stats ninja_test

echo ""
echo "nin --quiet -d stats ninja_test"
rm -rf build ninja_test
go install ./cmd/nin
time nin --quiet -d stats ninja_test

# Build the remaining performance tests.
./ninja --quiet build_log_perftest depfile_parser_perftest manifest_parser_perftest

# Unless the version of re2c locally exactly matches what is in the source
# repositories, the version number will change.
# TODO(maruel): Fix it upstream.
git checkout HEAD src/depfile_parser.cc src/lexer.cc

echo ""
echo "Comparing build_log_perftest:"
echo "C++"
./build_log_perftest
echo "Go"
go run ./cmd/build_log_perftest

# TODO(maruel): Either commit or generate a .d file.
#echo ""
#echo "Comparing depfile_parser_perftest:"
#echo "C++"
#./depfile_parser_perftest
#echo "Go"
#go run ./cmd/depfile_parser_perftest

echo ""
echo "Comparing manifest_parser_perftest:"
echo "C++"
./manifest_parser_perftest
echo "Go"
go run ./cmd/manifest_parser_perftest
