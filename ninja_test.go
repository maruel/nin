// Copyright 2013 Google Inc. All Rights Reserved.
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


type RegisteredTest struct {
  testing::Test* (*factory)()
  stringname
  should_run bool
}
// This can't be a vector because tests call RegisterTest from static
// initializers and the order static initializers run it isn't specified. So
// the vector constructor isn't guaranteed to run before all of the
// RegisterTest() calls.
static RegisteredTest tests[10000]
testing::Test* g_current_test
static int ntests
static LinePrinter printer

void RegisterTest(testing::Test* (*factory)(), string name) {
  tests[ntests].factory = factory
  tests[ntests++].name = name
}

string StringPrintf(string format, ...) {
  const int N = 1024
  char buf[N]

  va_list ap
  va_start(ap, format)
  vsnprintf(buf, N, format, ap)
  va_end(ap)

  return buf
}

func Usage() {
  fprintf(stderr, "usage: ninja_tests [options]\n\noptions:\n  --gtest_filter=POSTIVE_PATTERN[-NEGATIVE_PATTERN]\n      Run tests whose names match the positive but not the negative pattern.\n      '*' matches any substring. (gtest's ':', '?' are not implemented).\n")
}

func PatternMatchesString(pattern string, str string) bool {
  switch (*pattern) {
    case '\0':
    case '-': return *str == '\0'
    case '*': return (*str != '\0' && PatternMatchesString(pattern, str + 1)) ||
                     PatternMatchesString(pattern + 1, str)
    default:  return *pattern == *str &&
                     PatternMatchesString(pattern + 1, str + 1)
  }
}

func TestMatchesFilter(test string, filter string) bool {
  // Split --gtest_filter at '-' into positive and negative filters.
  string const dash = strchr(filter, '-')
  string pos = dash == filter ? "*" : filter //Treat '-test1' as '*-test1'
  string neg = dash ? dash + 1 : ""
  return PatternMatchesString(pos, test) && !PatternMatchesString(neg, test)
}

func ReadFlags(argc *int, argv *char**, test_filter *string) bool {
  enum { OPT_GTEST_FILTER = 1 }
  const option kLongOptions[] = {
    { "gtest_filter", required_argument, nil, OPT_GTEST_FILTER },
    { nil, 0, nil, 0 }
  }

  opt := 0
  while (opt = getopt_long(*argc, *argv, "h", kLongOptions, nil)) != -1 {
    switch (opt) {
    case OPT_GTEST_FILTER:
      if strchr(optarg, '?') == nil && strchr(optarg, ':') == nil {
        *test_filter = optarg
        break
      }  // else fall through.
    default:
      Usage()
      return false
    }
  }
  *argv += optind
  *argc -= optind
  return true
}

bool testing::Test::Check(bool condition, string file, int line, string error) {
  if (!condition) {
    printer.PrintOnNewLine( StringPrintf("*** Failure in %s:%d\n%s\n", file, line, error))
    failed_ = true
  }
  return condition
}

func main(argc int, argv **char) int {
  tests_started := 0

  string test_filter = "*"
  if !ReadFlags(&argc, &argv, &test_filter) {
    return 1
  }

  nactivetests := 0
  for i := 0; i < ntests; i++ {
    if (tests[i].should_run = TestMatchesFilter(tests[i].name, test_filter)) {
  }
      nactivetests++
    }

  passed := true
  for i := 0; i < ntests; i++ {
    if !tests[i].should_run {
    	continue
    }

    tests_started++
    test := tests[i].factory()
    printer.Print( StringPrintf("[%d/%d] %s", tests_started, nactivetests, tests[i].name), LinePrinter::ELIDE)
    test.SetUp()
    test.Run()
    test.TearDown()
    if test.Failed() {
      passed = false
    }
    var test delete
  }

  printer.PrintOnNewLine(passed ? "passed\n" : "failed\n")
  return passed ? EXIT_SUCCESS : EXIT_FAILURE
}

