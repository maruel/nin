#!/bin/bash

#TODO(maruel): Figure out why go generate doesn't work well.

set -eu

#TODO(maruel): Measure impact and usefulness of -8.

if ! re2go -help > /dev/null; then
  echo "Get sources from https://github.com/skvadrik/re2c/releases"
  echo "then run the following:"
  echo ""
  echo "tax xvf the file"
  echo "cd re2c"
  echo "mkdir out"
  echo "cd out"
  echo "cmake .."
  echo "cmake --buid ."
fi

re2go depfile_parser.in.go -o depfile_parser.go -i --no-generation-date
sed -i -e "/go.+nobuild/d" depfile_parser.go
gofmt -s -w depfile_parser.go

re2go lexer.in.go -o lexer.go -i --no-generation-date
sed -i -e "/go.+nobuild/d" lexer.go
gofmt -s -w lexer.go
