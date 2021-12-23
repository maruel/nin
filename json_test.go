// Copyright 2021 Google Inc. All Rights Reserved.
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
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestJSONTest_RegularAscii(t *testing.T) {
	data := []struct {
		in   string
		want string
	}{
		{"foo bar", "foo bar"},
		{
			"\"\\\b\f\n\r\t",
			"\\\"\\\\\\b\\f\\n\\r\\t",
		},
		{"\x01\x1f", "\\u0001\\u001f"},
		{
			// "你好",
			"\xe4\xbd\xa0\xe5\xa5\xbd",
			"\xe4\xbd\xa0\xe5\xa5\xbd",
		},
	}
	for i, l := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := EncodeJSONString(l.in)
			if diff := cmp.Diff(l.want, got); diff != "" {
				t.Fatalf("+want, -got: %s", diff)
			}
		})
	}
}
