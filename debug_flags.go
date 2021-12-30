// Copyright 2012 Google Inc. All Rights Reserved.
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

package nin

import (
	"fmt"
	"os"
)

var (
	g_explaining             = false
	g_keep_depfile           = false
	g_keep_rsp               = false
	g_experimental_statcache = true
)

func EXPLAIN(f string, i ...interface{}) {
	if g_explaining {
		fmt.Fprintf(os.Stderr, "ninja explain: "+f+"\n", i...)
	}
}
