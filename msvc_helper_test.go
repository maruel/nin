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

package nin

import "testing"

func TestEscapeForDepfileTest_SpacesInFilename(t *testing.T) {
	if EscapeForDepfile("sub\\some sdk\\foo.h") != "sub\\some\\ sdk\\foo.h" {
		t.Fatal("expected equal")
	}
}

func TestMSVCHelperTest_EnvBlock(t *testing.T) {
	t.Skip("TODO")
	env_block := "foo=bar\x00"
	var cl CLWrapper
	cl.SetEnvBlock(env_block)
	output := ""
	cl.Run("cmd /c \"echo foo is %foo%", &output)
	if output != "foo is bar\r\n" {
		t.Fatal("expected equal")
	}
}

func TestMSVCHelperTest_NoReadOfStderr(t *testing.T) {
	t.Skip("TODO")
	var cl CLWrapper
	output := ""
	cl.Run("cmd /c \"echo to stdout&& echo to stderr 1>&2", &output)
	if output != "to stdout\r\n" {
		t.Fatal("expected equal")
	}
}
