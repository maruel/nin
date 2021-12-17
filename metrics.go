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


// The Metrics module is used for the debug mode that dumps timing stats of
// various actions.  To use, see METRIC_RECORD below.

// A single metrics we're tracking, like "depfile load time".
type Metric struct {
  string name
  // Number of times we've hit the code path.
  int count
  // Total time (in micros) we've spent on the code path.
  int64_t sum
}

// A scoped object for recording a metric across the body of a function.
// Used by the METRIC_RECORD macro.
type ScopedMetric struct {
  ~ScopedMetric()

  Metric* metric_
  // Timestamp when the measurement started.
  // Value is platform-dependent.
  int64_t start_
}

// The singleton that stores metrics and prints the report.
type Metrics struct {

  vector<Metric*> metrics_
}

// Get the current time as relative to some epoch.
// Epoch varies between platforms; only useful for measuring elapsed time.
int64_t GetTimeMillis()

// A simple stopwatch which returns the time
// in seconds since Restart() was called.
type Stopwatch struct {
  Stopwatch() : started_(0) {}

  // Seconds since Restart() call.
  func (s *Stopwatch) Elapsed() double {
    return 1e-6 * static_cast<double>(Now() - started_)
  }

  void Restart() { started_ = Now(); }

  uint64_t started_
  uint64_t Now() const
}

// The primary interface to metrics.  Use METRIC_RECORD("foobar") at the top
// of a function to get timing stats recorded for each call of the function.

//extern Metrics* g_metrics


Metrics* g_metrics = nil

// Compute a platform-specific high-res timer value that fits into an int64.
int64_t HighResTimer() {
  timeval tv
  if (gettimeofday(&tv, nil) < 0)
    Fatal("gettimeofday: %s", strerror(errno))
  return (int64_t)tv.tv_sec * 1000*1000 + tv.tv_usec
}

// Convert a delta of HighResTimer() values to microseconds.
int64_t TimerToMicros(int64_t dt) {
  // No conversion necessary.
  return dt
}
int64_t LargeIntegerToInt64(const LARGE_INTEGER& i) {
  return ((int64_t)i.HighPart) << 32 | i.LowPart
}

int64_t HighResTimer() {
  LARGE_INTEGER counter
  if (!QueryPerformanceCounter(&counter))
    Fatal("QueryPerformanceCounter: %s", GetLastErrorString())
  return LargeIntegerToInt64(counter)
}

int64_t TimerToMicros(int64_t dt) {
  static int64_t ticks_per_sec = 0
  if (!ticks_per_sec) {
    LARGE_INTEGER freq
    if (!QueryPerformanceFrequency(&freq))
      Fatal("QueryPerformanceFrequency: %s", GetLastErrorString())
    ticks_per_sec = LargeIntegerToInt64(freq)
  }

  // dt is in ticks.  We want microseconds.
  return (dt * 1000000) / ticks_per_sec
}

ScopedMetric::ScopedMetric(Metric* metric) {
  metric_ = metric
  if (!metric_)
    return
  start_ = HighResTimer()
}
ScopedMetric::~ScopedMetric() {
  if (!metric_)
    return
  metric_.count++
  int64_t dt = TimerToMicros(HighResTimer() - start_)
  metric_.sum += dt
}

func (m *Metrics) NewMetric(name string) Metric* {
  metric := new Metric
  metric.name = name
  metric.count = 0
  metric.sum = 0
  metrics_.push_back(metric)
  return metric
}

// Print a summary report to stdout.
func (m *Metrics) Report() {
  width := 0
  for i := metrics_.begin(); i != metrics_.end(); i++ {
    width = max((int)(*i).name.size(), width)
  }

  printf("%-*s\t%-6s\t%-9s\t%s\n", width, "metric", "count", "avg (us)", "total (ms)")
  for i := metrics_.begin(); i != metrics_.end(); i++ {
    metric := *i
    double total = metric.sum / (double)1000
    double avg = metric.sum / (double)metric.count
    printf("%-*s\t%-6d\t%-8.1f\t%.1f\n", width, metric.name, metric.count, avg, total)
  }
}

uint64_t Stopwatch::Now() const {
  return TimerToMicros(HighResTimer())
}

int64_t GetTimeMillis() {
  return TimerToMicros(HighResTimer()) / 1000
}

