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

# Credits:
# https://gist.github.com/CAFxX/332b425634f12ccbb7a1eef074da19bf

# - Inserts an empty line after unconditional control-flow modifying instructions (JMP, RET, UD2)
# - Unindent the function body
#
# Colors:
# - Green:  calls/returns
# - Red:    traps (UD2)
# - Blue:   jumps (both conditional and unconditional)
# - Violet: padding/nops
# - Yellow: function name

set -eu

if [ $# != 1 ]; then
  echo "usage: ./dump_asm.sh <symbol regex>"
  echo ""
  echo "example:"
  echo "  ./dump_asm.sh 'nin.CanonicalizePath$'"
  exit 1
fi

go build ./cmd/nin

go tool objdump -s "$1" ./nin |
  sed -E "
    s/^  ([^\t]+)(.*)/\1  \2/
    s,^(TEXT )([^ ]+)(.*),$(tput setaf 3)\\1$(tput bold)\\2$(tput sgr0)$(tput setaf 3)\\3$(tput sgr0),
    s/((JMP|RET|UD2).*)\$/\1\n/
    s,.*(CALL |RET).*,$(tput setaf 2)&$(tput sgr0),
    s,.*UD2.*,$(tput setaf 1)&$(tput sgr0),
    s,.*J[A-Z]+.*,$(tput setaf 4)&$(tput sgr0),
    s,.*(INT \\\$0x3|NOP).*,$(tput setaf 5)&$(tput sgr0),
    "

rm nin
