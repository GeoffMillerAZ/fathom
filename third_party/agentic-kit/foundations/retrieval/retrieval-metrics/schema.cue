// Package retrievalmetrics defines the CUE-first contract for retrieval quality
// metrics. Go types in types.go mirror these definitions.
//
// Metric applicability by archetype:
//
//   existence  : Precision ✓  Recall ✓  MRR ✓  FalseAbsentRate ✗  CoverageFraction ✗
//   absence    : Precision ✗  Recall ✗  MRR ✗  FalseAbsentRate ✓  CoverageFraction ✓
//   threshold  : Precision ✓  Recall ✓  MRR ✓  FalseAbsentRate ✗  CoverageFraction ✗
//   contradicts: Precision ✓  Recall ✓  MRR ✓  FalseAbsentRate ✗  CoverageFraction ✗
//
// MRR is undefined (not-applicable) on empty gold sets — it must never return
// NaN. All metric functions return a MetricResult with a NotApplicable flag
// rather than a sentinel float value.
package retrievalmetrics

// #Archetype mirrors evidencesignature.Archetype for metric applicability
// declarations. Must stay in sync with evidence-signature/schema.cue.
#Archetype: "existence" | "absence" | "threshold" | "contradicts"

// #MetricResult is the outcome of a single metric computation.
//
//   - value: the metric value in [0, 1] (only meaningful when applicable == true).
//   - applicable: true iff this metric is defined for the inputs.
//   - reason: the human-readable reason when applicable == false (empty when true).
//
// Invariant: applicable == false implies reason is non-empty.
#MetricResult: {
	value:      >=0 & <=1
	applicable: bool
	reason:     string

	// When not applicable, a reason must be stated.
	_reasonRequired: applicable == true || reason != ""
}

// #CanaryRecord is one deterministic absence-quality sample: whether a canary
// that was supposed to be planted was actually found by the search engine.
// The population of these records is the ground truth for FalseAbsentRate.
//
//   - signature_id: the ConcernSignature.ID this canary belongs to.
//   - canary_found:  true if the search engine found the canary's pattern.
//
// Invariant: both fields are required (non-empty ID, explicit bool).
#CanaryRecord: {
	signature_id: string & !=""
	canary_found: bool
}
