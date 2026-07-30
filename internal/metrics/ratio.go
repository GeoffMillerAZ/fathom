package metrics

// ratio.go — Q-15 (docs/plans/expert-suite-q.md): the ONE zero-denominator
// guard every ratio this phase computes goes through, closing docs/design/
// observability-surfaces.md §6's requirement structurally rather than
// per-call-site. Phase P's own critic finding F1 (research_report.go,
// buildDiscoverySection) found and fixed exactly this bug class once —
// total==0 with a positive numerator computing a fabricated 100%. Ratio
// makes that bug unrepresentable in every new aggregator this phase adds.

// notMeasurableZeroDenominator is the fixed note Ratio returns whenever the
// denominator is zero — never a blank, never a fabricated 0/100.
const notMeasurableZeroDenominator = "not measurable: zero denominator"

// Ratio computes numerator/denominator as a percentage (0-100), guarding
// the zero-denominator case: denominator == 0 returns (nil,
// "not measurable: zero denominator") rather than NaN/+Inf or a silently
// wrong 0%/100%. A real computation returns a non-nil percentage and an
// empty note.
func Ratio(numerator, denominator int) (pct *float64, note string) {
	if denominator == 0 {
		return nil, notMeasurableZeroDenominator
	}
	p := float64(numerator) / float64(denominator) * 100
	return &p, ""
}
