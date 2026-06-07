// Package metrics provides a lightweight, concurrency-safe in-process metrics
// registry that emits Prometheus text-format output.
//
// The registry supports two instrument types:
//   - Counter: monotonically increasing integer, via Counter(name).Inc().
//   - Observation: floating-point sample accumulated as sum+count, via Observe(name, v).
//
// Expose() returns a sorted, deterministic Prometheus text-format string so
// that snapshot comparisons in tests are reliable.
//
// Example:
//
//	reg := metrics.NewRegistry()
//	reg.Counter("ingest_files_total").Inc()
//	reg.Counter("ingest_files_total").Inc()
//	reg.Observe("ingest_duration_seconds", 0.042)
//	fmt.Print(reg.Expose())
package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter is a concurrency-safe monotonic integer counter.
// Obtain one via Registry.Counter(name).
type Counter struct {
	n atomic.Int64
}

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.n.Add(1) }

// value returns the current counter value.
func (c *Counter) value() int64 { return c.n.Load() }

// observation accumulates float64 samples for a named metric.
type observation struct {
	mu    sync.Mutex
	sum   float64
	count int64
}

// record adds a sample.
func (o *observation) record(v float64) {
	o.mu.Lock()
	o.sum += v
	o.count++
	o.mu.Unlock()
}

// snapshot returns a copy of the current state.
func (o *observation) snapshot() (sum float64, count int64) {
	o.mu.Lock()
	sum, count = o.sum, o.count
	o.mu.Unlock()
	return
}

// Registry holds a set of named counters and observations.
// All methods are safe for concurrent use.
// Create one with NewRegistry.
type Registry struct {
	mu           sync.Mutex
	counters     map[string]*Counter
	observations map[string]*observation
}

// NewRegistry creates and returns a new, empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:     make(map[string]*Counter),
		observations: make(map[string]*observation),
	}
}

// Counter returns the named Counter, creating it on first call.
// Subsequent calls with the same name return the same instance.
// The returned *Counter is safe to call Inc() on from multiple goroutines.
func (r *Registry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.counters[name]
	if !ok {
		c = &Counter{}
		r.counters[name] = c
	}
	return c
}

// Observe records a floating-point sample for the named metric.
// If the metric does not exist it is created; subsequent Observe calls
// accumulate into the same metric (sum + count, as in Prometheus summaries).
func (r *Registry) Observe(name string, v float64) {
	r.mu.Lock()
	o, ok := r.observations[name]
	if !ok {
		o = &observation{}
		r.observations[name] = o
	}
	r.mu.Unlock()
	o.record(v)
}

// Expose returns a sorted, deterministic Prometheus text-format string.
//
// Counters appear as:
//
//	# TYPE <name> counter
//	<name> <value>
//
// Observations appear as:
//
//	# TYPE <name> summary
//	<name>_sum <sum>
//	<name>_count <count>
//
// All metrics are sorted alphabetically by name. Counters are listed before
// observations.
func (r *Registry) Expose() string {
	r.mu.Lock()
	// Snapshot counter names and values while holding the lock.
	cNames := make([]string, 0, len(r.counters))
	cVals := make(map[string]int64, len(r.counters))
	for n, c := range r.counters {
		cNames = append(cNames, n)
		cVals[n] = c.value()
	}
	// Snapshot observation names.
	oNames := make([]string, 0, len(r.observations))
	for n := range r.observations {
		oNames = append(oNames, n)
	}
	// Take references to observations while still under lock.
	oCopies := make(map[string]*observation, len(r.observations))
	for n, o := range r.observations {
		oCopies[n] = o
	}
	r.mu.Unlock()

	// Fetch observation snapshots outside the registry lock.
	oSums := make(map[string]float64, len(oNames))
	oCounts := make(map[string]int64, len(oNames))
	for _, n := range oNames {
		sum, count := oCopies[n].snapshot()
		oSums[n] = sum
		oCounts[n] = count
	}

	sort.Strings(cNames)
	sort.Strings(oNames)

	var sb strings.Builder

	for _, n := range cNames {
		fmt.Fprintf(&sb, "# TYPE %s counter\n", n)
		fmt.Fprintf(&sb, "%s %d\n", n, cVals[n])
	}
	for _, n := range oNames {
		fmt.Fprintf(&sb, "# TYPE %s summary\n", n)
		fmt.Fprintf(&sb, "%s_sum %s\n", n, formatFloat(oSums[n]))
		fmt.Fprintf(&sb, "%s_count %d\n", n, oCounts[n])
	}

	return sb.String()
}

// formatFloat formats a float64 in a compact, Prometheus-compatible way.
func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "+Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	return fmt.Sprintf("%g", f)
}
