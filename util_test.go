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

//go:build nobuild

package ginja


func CanonicalizePath(path *string) {
  uint64_t unused
  ::CanonicalizePath(path, &unused)
}

TEST(CanonicalizePath, PathSamples) {
  string path

  CanonicalizePath(&path)
  if "" != path { t.FailNow() }

  path = "foo.h"
  CanonicalizePath(&path)
  if "foo.h" != path { t.FailNow() }

  path = "./foo.h"
  CanonicalizePath(&path)
  if "foo.h" != path { t.FailNow() }

  path = "./foo/./bar.h"
  CanonicalizePath(&path)
  if "foo/bar.h" != path { t.FailNow() }

  path = "./x/foo/../bar.h"
  CanonicalizePath(&path)
  if "x/bar.h" != path { t.FailNow() }

  path = "./x/foo/../../bar.h"
  CanonicalizePath(&path)
  if "bar.h" != path { t.FailNow() }

  path = "foo//bar";
  CanonicalizePath(&path)
  if "foo/bar" != path { t.FailNow() }

  path = "foo//.//..///bar";
  CanonicalizePath(&path)
  if "bar" != path { t.FailNow() }

  path = "./x/../foo/../../bar.h"
  CanonicalizePath(&path)
  if "../bar.h" != path { t.FailNow() }

  path = "foo/./."
  CanonicalizePath(&path)
  if "foo" != path { t.FailNow() }

  path = "foo/bar/.."
  CanonicalizePath(&path)
  if "foo" != path { t.FailNow() }

  path = "foo/.hidden_bar"
  CanonicalizePath(&path)
  if "foo/.hidden_bar" != path { t.FailNow() }

  path = "/foo"
  CanonicalizePath(&path)
  if "/foo" != path { t.FailNow() }

  path = "//foo";
  CanonicalizePath(&path)
