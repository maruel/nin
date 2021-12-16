#!/bin/bash

#TODO(maruel): Figure out why go generate doesn't work well.

set -eu

#TODO(maruel): Measure impact and usefulness of -8.

if ! re2go -help &> /dev/null; then
  echo "re2go is needed to generate the high performance regexp."
  echo "Install with:"
  echo ""
  echo "git clone -b 2.2 https://github.com/skvadrik/re2c"
  echo "cd re2c"
  echo "mkdir out"
  echo "cd out"
  echo "cmake .."
  echo "cmake --build . -j 16"
  echo "cd ../.."
  echo 'export PATH=$PATH:$(pwd)/re2c/out'
  echo ""
  echo "then run this command again."
  exit 1
fi

echo "Generating depfile_parser.go"
re2go depfile_parser.in.go -o depfile_parser.go -i --no-generation-date
sed -i -e "/go.\+neverbuild/d" depfile_parser.go
gofmt -s -w depfile_parser.go

echo "Generating lexer.go"
re2go lexer.in.go -o lexer.go -i --no-generation-date
sed -i -e "/go.\+neverbuild/d" lexer.go
gofmt -s -w lexer.go
