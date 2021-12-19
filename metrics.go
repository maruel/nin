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

import "time"

// The Metrics module is used for the debug mode that dumps timing stats of
// various actions.  To use, see METRIC_RECORD below.

/// The primary interface to metrics.  Use METRIC_RECORD("foobar") at the top
/// of a function to get timing stats recorded for each call of the function.
func METRIC_RECORD(name string) {
	/*
		if g_metrics != nil {
			metrics_h_metric := g_metrics.NewMetric(name)
		}
		metrics_h_scoped := ScopedMetric(metrics_h_metric)
	*/
}

// A single metrics we're tracking, like "depfile load time".
type Metric struct {
	name string
	// Number of times we've hit the code path.
	count int
	// Total time (in micros) we've spent on the code path.
	sum int64
}

// A scoped object for recording a metric across the body of a function.
// Used by the METRIC_RECORD macro.
type ScopedMetric struct {
	metric_ *Metric
	// Timestamp when the measurement started.
	// Value is platform-dependent.
	start_ int64
}

// The singleton that stores metrics and prints the report.
type Metrics struct {
	metrics_ []*Metric
}

// A simple stopwatch which returns the time
// in seconds since Restart() was called.
type Stopwatch struct {
	started_ int64
}

func NewStopwatch() Stopwatch {
	return Stopwatch{}
}

// Seconds since Restart() call.
func (s *Stopwatch) Elapsed() float64 {
	return 1e-6 * float64(s.Now()-s.started_)
}
func (s *Stopwatch) Restart() {
	s.started_ = s.Now()
}

// The primary interface to metrics.  Use METRIC_RECORD("foobar") at the top
// of a function to get timing stats recorded for each call of the function.

//extern Metrics* g_metrics

var g_metrics *Metrics

// Compute a platform-specific high-res timer value that fits into an int64.
func HighResTimer() int64 {
	/*
	  var tv timeval
	  if gettimeofday(&tv, nil) < 0 {
	    Fatal("gettimeofday: %s", strerror(errno))
	  }
	  return tv.tv_sec * 1000*1000 + tv.tv_usec
	*/
	// TODO:
	return time.Now().UnixNano()
}

// Convert a delta of HighResTimer() values to microseconds.
func TimerToMicros(dt int64) int64 {
	// No conversion necessary.
	return dt
}

func NewScopedMetric(metric *Metric) *ScopedMetric {
	return &ScopedMetric{
		metric_: metric,
		start_:  HighResTimer(),
	}
}

/*
ScopedMetric::~ScopedMetric() {
  if (!metric_)
    return
  metric_.count++
  int64_t dt = TimerToMicros(HighResTimer() - start_)
  metric_.sum += dt
}
*/
func (m *Metrics) NewMetric(name string) *Metric {
	metric := &Metric{name: name}
	m.metrics_ = append(m.metrics_, metric)
	return metric
}

// Print a summary report to stdout.
func (m *Metrics) Report() {
	width := 0
	for _, i := range m.metrics_ {
		if j := len(i.name); j > width {
			width = j
		}
	}

	printf("%-*s\t%-6s\t%-9s\t%s\n", width, "metric", "count", "avg (us)", "total (ms)")
	for _, metric := range m.metrics_ {
		total := float64(metric.sum) / 1000.
		avg := float64(metric.sum) / float64(metric.count)
		printf("%-*s\t%-6d\t%-8.1f\t%.1f\n", width, metric.name, metric.count, avg, total)
	}
}

func (s *Stopwatch) Now() int64 {
	return TimerToMicros(HighResTimer())
}

// Get the current time as relative to some epoch.
// Epoch varies between platforms; only useful for measuring elapsed time.
func GetTimeMillis() int64 {
	return TimerToMicros(HighResTimer()) / 1000
}
