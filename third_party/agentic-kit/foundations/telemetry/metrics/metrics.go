// Package metrics provides a lightweight, stdlib-only, concurrency-safe
// in-process metrics registry that emits deterministic Prometheus
// text-format output.
//
// Two instrument types are supported:
//   - Counter: a monotonically increasing integer, optionally partitioned by
//     ordered label pairs — Registry.Counter(name, labels...).Inc() / Add(n).
//   - Observation: float64 samples accumulated as sum + count (a Prometheus
//     summary without quantiles) — Registry.Observe(name, v, labels...).
//
// Labels are passed as ordered key/value pairs ("method", "GET", "code",
// "200") and are canonicalised by sorting on key, so the same label set in
// any argument order names the same series. Expose returns a fully sorted
// snapshot (counters first, then summaries; names sorted; series within a
// family sorted by rendered label block) so golden-string comparisons in
// tests are reliable. The registry is stampless: it holds no clock and
// records no timestamps, keeping output a pure function of the recorded
// values.
//
// There is no schema.cue in this package because the contract here is
// behavioral (a registry algorithm and its text rendering), not a structural
// data shape. CUE is applied only where data shape is the primary concern.
//
// Concurrency: all Registry and Counter methods are safe for concurrent use.
// The registry maps are guarded by a sync.Mutex; counters use atomics and
// observations use a per-series mutex, so hot-path increments do not contend
// on the registry lock.
package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter is a concurrency-safe monotonic integer counter for one series
// (one name + label-set combination). Obtain one via Registry.Counter; the
// same name and label set always returns the same instance.
type Counter struct {
	n atomic.Int64
}

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.n.Add(1) }

// Add increments the counter by delta. Counters are monotonic: a negative
// or zero delta is ignored.
func (c *Counter) Add(delta int64) {
	if delta <= 0 {
		return
	}
	c.n.Add(delta)
}

// value returns the current counter value.
func (c *Counter) value() int64 { return c.n.Load() }

// observation accumulates float64 samples for one series as sum + count.
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
	return sum, count
}

// Registry holds named counter and observation families, each partitioned
// into series by canonicalised label set. All methods are safe for
// concurrent use. Create one with NewRegistry.
type Registry struct {
	mu sync.Mutex
	// counters and observations map metric name -> rendered label block
	// ("" for unlabeled, `{k="v",...}` otherwise) -> instrument.
	counters     map[string]map[string]*Counter
	observations map[string]map[string]*observation
}

// NewRegistry creates and returns a new, empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:     make(map[string]map[string]*Counter),
		observations: make(map[string]map[string]*observation),
	}
}

// Counter returns the Counter for name partitioned by the given ordered
// label pairs, creating it on first use. labels must be alternating key,
// value strings; Counter panics on an odd count (programmer error). Label
// pairs are canonicalised by sorting on key, so Counter("x", "a", "1", "b",
// "2") and Counter("x", "b", "2", "a", "1") return the same instance.
func (r *Registry) Counter(name string, labels ...string) *Counter {
	key := labelsKey(labels)
	r.mu.Lock()
	defer r.mu.Unlock()
	fam, ok := r.counters[name]
	if !ok {
		fam = make(map[string]*Counter)
		r.counters[name] = fam
	}
	c, ok := fam[key]
	if !ok {
		c = &Counter{}
		fam[key] = c
	}
	return c
}

// Observe records a float64 sample for name partitioned by the given ordered
// label pairs, creating the series on first use. Samples accumulate as
// sum + count, as in a Prometheus summary. labels follow the same rules as
// Counter (alternating key/value; panics on an odd count; canonicalised by
// sorting on key).
func (r *Registry) Observe(name string, v float64, labels ...string) {
	key := labelsKey(labels)
	r.mu.Lock()
	fam, ok := r.observations[name]
	if !ok {
		fam = make(map[string]*observation)
		r.observations[name] = fam
	}
	o, ok := fam[key]
	if !ok {
		o = &observation{}
		fam[key] = o
	}
	r.mu.Unlock()
	o.record(v)
}

// Expose returns a sorted, deterministic Prometheus text-format string.
//
// Counters render as:
//
//	# TYPE <name> counter
//	<name>{<sorted labels>} <value>
//
// Observations render as:
//
//	# TYPE <name> summary
//	<name>_sum{<sorted labels>} <sum>
//	<name>_count{<sorted labels>} <count>
//
// The label block is omitted for unlabeled series. Counters are listed
// before summaries; family names are sorted alphabetically; series within a
// family are sorted by rendered label block (unlabeled first). The output is
// a pure function of the recorded values — no timestamps are emitted.
func (r *Registry) Expose() string {
	// Snapshot counter values and observation references under the lock.
	r.mu.Lock()
	cVals := make(map[string]map[string]int64, len(r.counters))
	for name, fam := range r.counters {
		series := make(map[string]int64, len(fam))
		for key, c := range fam {
			series[key] = c.value()
		}
		cVals[name] = series
	}
	oRefs := make(map[string]map[string]*observation, len(r.observations))
	for name, fam := range r.observations {
		series := make(map[string]*observation, len(fam))
		for key, o := range fam {
			series[key] = o
		}
		oRefs[name] = series
	}
	r.mu.Unlock()

	var sb strings.Builder

	for _, name := range sortedKeys(cVals) {
		fmt.Fprintf(&sb, "# TYPE %s counter\n", name)
		series := cVals[name]
		for _, key := range sortedKeys(series) {
			fmt.Fprintf(&sb, "%s%s %d\n", name, key, series[key])
		}
	}
	for _, name := range sortedKeys(oRefs) {
		fmt.Fprintf(&sb, "# TYPE %s summary\n", name)
		series := oRefs[name]
		for _, key := range sortedKeys(series) {
			sum, count := series[key].snapshot()
			fmt.Fprintf(&sb, "%s_sum%s %s\n", name, key, formatFloat(sum))
			fmt.Fprintf(&sb, "%s_count%s %d\n", name, key, count)
		}
	}

	return sb.String()
}

// labelsKey renders ordered label pairs into a canonical Prometheus label
// block: pairs sorted by key, values escaped, e.g. `{a="1",b="2"}`. Empty
// labels yield "". Panics when labels has an odd length — labels must be
// alternating key, value strings.
func labelsKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	if len(labels)%2 != 0 {
		panic(fmt.Sprintf("metrics: odd number of label arguments (%d); want alternating key, value pairs", len(labels)))
	}
	type kv struct{ k, v string }
	pairs := make([]kv, 0, len(labels)/2)
	for i := 0; i < len(labels); i += 2 {
		pairs = append(pairs, kv{k: labels[i], v: labels[i+1]})
	}
	// Stable sort: for duplicate keys the caller-provided order is preserved
	// (last occurrence still renders; duplicates are a caller bug).
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].k < pairs[j].k })

	var sb strings.Builder
	sb.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(p.k)
		sb.WriteString(`="`)
		sb.WriteString(escapeLabelValue(p.v))
		sb.WriteByte('"')
	}
	sb.WriteByte('}')
	return sb.String()
}

// labelValueEscaper escapes backslash, double-quote, and newline per the
// Prometheus text exposition format.
var labelValueEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

// escapeLabelValue escapes a label value for the text exposition format.
func escapeLabelValue(v string) string { return labelValueEscaper.Replace(v) }

// sortedKeys returns the keys of m in ascending order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
