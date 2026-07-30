package metrics

// tiering.go — Q-14 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §3): FATHOM_METRICS off/basic/full, default
// basic. Read-only reporting, not a new gate — no code in this phase
// branches production behavior on the tier; `full`'s payload-sampling and
// per-section compile diagnostics BEYOND what Q-1..Q-4 already capture
// unconditionally are deferred (docs/plans/expert-suite-q.md's own
// Deferred section), pending a real tuning window to design against.
//
// The honest headline (design §3): NO tier of this design spends model
// tokens to measure model tokens — off/basic/full only change how much
// arithmetic-on-already-in-hand-data and disk/privacy surface this
// instrumentation costs, never LLM spend.

import "os"

// MetricsTier is one of the three FATHOM_METRICS values.
type MetricsTier string

const (
	MetricsTierOff   MetricsTier = "off"
	MetricsTierBasic MetricsTier = "basic"
	MetricsTierFull  MetricsTier = "full"
)

// ResolveMetricsTier reads FATHOM_METRICS: "off" or "full" select that
// tier; unset/empty defaults to "basic" (the design's own default); any
// OTHER value falls back to "basic" too, but with an observable note
// naming the invalid value — never a silent, unexplained fallback.
func ResolveMetricsTier() (tier MetricsTier, note string) {
	v := os.Getenv("FATHOM_METRICS")
	switch v {
	case "", string(MetricsTierBasic):
		return MetricsTierBasic, ""
	case string(MetricsTierOff):
		return MetricsTierOff, ""
	case string(MetricsTierFull):
		return MetricsTierFull, ""
	default:
		return MetricsTierBasic, "FATHOM_METRICS=" + v + " is not a recognized tier (want off|basic|full) — falling back to basic"
	}
}
