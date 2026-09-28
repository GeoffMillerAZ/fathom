package review

// riskBand picks the receipt's overall Band from the strongest cited
// factor — NEVER a summed score (ADR-051 decision 4; refused: CX-X1,
// "a summed risk score or a model's opinion of risk"). Ties among several
// factors at the same strongest weight are broken by the design doc's own
// row order (factors is always built in that fixed order by
// BuildRiskReceipt) so the result is deterministic: the first factor at
// the strongest weight wins the citation.
func riskBand(factors []RiskFactor, table RiskTable) (Band, string) {
	_ = table // the table tunes factor weights upstream; riskBand only picks the strongest.

	strongest := WeightNone
	reason := ""
	for _, f := range factors {
		if f.Weight > strongest {
			strongest = f.Weight
			reason = f.Reason
		}
	}

	switch strongest {
	case WeightHigh:
		return BandHigh, reason
	case WeightMedium:
		return BandMedium, reason
	default:
		return BandLow, "no factor rose above the configured thresholds"
	}
}
