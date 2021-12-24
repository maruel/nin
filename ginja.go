// Copyright 2011 Google Inc. All Rights Reserved.
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

package ginja

import (
	"fmt"
	"io"
	"os"
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
)

func assert(b bool) {
	if !b {
		panic(b)
	}
}

func printf(f string, v ...interface{}) {
	fmt.Printf(f, v...)
}

func fprintf(w io.Writer, f string, v ...interface{}) {
	fmt.Fprintf(w, f, v...)
}
