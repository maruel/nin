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
	"sort"
	"time"
)

// The Metrics module is used for the debug mode that dumps timing stats of
// various actions.  To use, see METRIC_RECORD below.

func emptyFunc() {
}

/// The primary interface to metrics.  Use METRIC_RECORD("foobar") at the top
/// of a function to get timing stats recorded for each call of the function.
func METRIC_RECORD(name string) func() {
	// TODO(maruel): Use runtime/trace.StartRegion() instead.
	if g_metrics == nil {
		return emptyFunc
	}
	m := g_metrics.GetMetric(name)
	start := time.Now()
	return func() {
		m.count++
		m.sum += time.Since(start)
	}
}

// A single metrics we're tracking, like "depfile load time".
type Metric struct {
	name string
	// Number of times we've hit the code path.
	count int
	// Total time we've spent on the code path.
	sum time.Duration
}

// The singleton that stores metrics and prints the report.
type Metrics struct {
	metrics_ map[string]*Metric
}

func NewMetrics() *Metrics {
	return &Metrics{
		metrics_: map[string]*Metric{},
	}
}

// The primary interface to metrics.  Use METRIC_RECORD("foobar") at the top
// of a function to get timing stats recorded for each call of the function.
var g_metrics *Metrics

func (m *Metrics) GetMetric(name string) *Metric {
	if m.metrics_ == nil {
		m.metrics_ = map[string]*Metric{}
	}
	metric, ok := m.metrics_[name]
	if !ok {
		metric = &Metric{name: name}
		m.metrics_[name] = metric
	}
	return metric
}

// Print a summary report to stdout.
func (m *Metrics) Report() {
	width := 0
	names := make([]string, 0, len(m.metrics_))
	for name := range m.metrics_ {
		if j := len(name); j > width {
			width = j
		}
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("%-*s\t%-6s\t%-9s\t%s\n", width, "metric", "count", "avg", "total")
	for _, name := range names {
		metric := m.metrics_[name]
		avg := metric.sum / time.Duration(metric.count)
		fmt.Printf("%-*s\t%-6d\t%-10s\t%-10s\n", width, name, metric.count, avg.Round(time.Microsecond), metric.sum.Round(time.Microsecond))
	}
}

// Get the current time as relative to some epoch.
// Epoch varies between platforms; only useful for measuring elapsed time.
func GetTimeMillis() int64 {
	return time.Now().UnixMilli()
}
