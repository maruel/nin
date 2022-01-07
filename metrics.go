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

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

func emptyFunc() {
}

// metricRecord is the primary interface to metrics.
//
// Use defer metricRecord("foobar")() at the top of a function to get timing
// stats recorded for each call of the function.
func metricRecord(name string) func() {
	// TODO(maruel): Use runtime/trace.StartRegion() instead.
	if Metrics.metrics == nil {
		return emptyFunc
	}
	m := Metrics.getMetric(name)
	start := time.Now()
	return func() {
		m.count++
		m.sum += time.Since(start)
	}
}

// A single metrics we're tracking, like "depfile load time".
type metric struct {
	name string
	// Number of times we've hit the code path.
	count int
	// Total time we've spent on the code path.
	sum time.Duration
}

// MetricsCollection collects metrics.
type MetricsCollection struct {
	mu      sync.Mutex
	metrics map[string]*metric
}

// Metrics is the singleton that stores metrics for this package.
var Metrics MetricsCollection

// Enable enables metrics collection.
//
// Must be called before using any other functionality in this package.
func (m *MetricsCollection) Enable() {
	Metrics.metrics = map[string]*metric{}
}

func (m *MetricsCollection) getMetric(name string) *metric {
	m.mu.Lock()
	met := m.metrics[name]
	if met == nil {
		met = &metric{name: name}
		m.metrics[name] = met
	}
	m.mu.Unlock()
	return met
}

// Report prints a summary report to stdout.
func (m *MetricsCollection) Report() {
	width := 0
	names := make([]string, 0, len(m.metrics))
	for name := range m.metrics {
		if j := len(name); j > width {
			width = j
		}
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("%-*s\t%-6s\t%-9s\t%s\n", width, "metric", "count", "avg", "total")
	for _, name := range names {
		metric := m.metrics[name]
		avg := metric.sum / time.Duration(metric.count)
		fmt.Printf("%-*s\t%-6d\t%-10s\t%-10s\n", width, name, metric.count, avg.Round(time.Microsecond), metric.sum.Round(time.Microsecond))
	}
}

// GetTimeMillis gets the current time as relative to some epoch.
//
// Epoch varies between platforms; only useful for measuring elapsed time.
func GetTimeMillis() int64 {
	// TODO(maruel): Standardize on time.Time.
	return time.Now().UnixMilli()
}
