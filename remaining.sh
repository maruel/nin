#!/bin/bash

# Remove this script when conversion is done.

set -eu

ls -laS $(git grep --name-only "nobuild" -- "*.go")
