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

#TODO(maruel): Figure out why go generate doesn't work well.

set -eu

#TODO(maruel): Measure impact and usefulness of -8.

if ! re2go -help &> /dev/null; then
  if [ ! -f ../re2c/out/re2go ]; then
    echo "re2go is needed to generate the high performance regexp."
    echo "Installing"
    echo ""
    git clone -b 2.2 https://github.com/skvadrik/re2c ../re2c
    mkdir ../re2c/out
    cd ../re2c/out
    cmake ..
    cmake --build . -j 4
    cd -
  fi
  export PATH=$PATH:$(pwd)/../re2c/out
elif [[ -f ../re2c/out/re2go ]]; then
  export PATH=$PATH:$(pwd)/../re2c/out
fi

echo "Generating depfile_parser.go"
re2go depfile_parser.in.go -o depfile_parser.go -i --no-generation-date
sed -i -e "/go.\+neverbuild/d" depfile_parser.go
gofmt -s -w depfile_parser.go

echo "Generating lexer.go"
re2go lexer.in.go -o lexer.go -i --no-generation-date
sed -i -e "/go.\+neverbuild/d" lexer.go
gofmt -s -w lexer.go
