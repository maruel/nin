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

AGAINST=HEAD~1
COUNT=10
BENCH=.

# Make sure the tree is checked out and pristine, otherwise we could loose the
# checkout.
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" = "HEAD" ]; then
  echo "Checkout a branch first."
  exit 1
fi

if [ "$(git status --porcelain)" != "" ]; then
  echo "The tree is modified, make sure to commit all your changes before"
  echo "running this script."
  exit 1
fi

if ! which benchstat > /dev/null ; then
  go install golang.org/x/perf/cmd/benchstat@latest
fi

echo "Running go test -bench=$BENCH -count=$COUNT on $BRANCH"
go test -count=$COUNT -bench=$BENCH -run '^$' -cpu 1 > new.txt

echo "Running go test -bench=$BENCH -count=$COUNT on $AGAINST"
git checkout -q $AGAINST
go test -count=$COUNT -bench=$BENCH -run '^$' -cpu 1 > old.txt
git checkout -q $BRANCH

benchstat old.txt new.txt
