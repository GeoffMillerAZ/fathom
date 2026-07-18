package evalgate

// Evaluate runs the gate over the provided checks and returns a deterministic
// Result. The algorithm is:
//
//  1. Sum all positive weights to derive the total weight.
//  2. Sum the weights of passing checks.
//  3. Score = passingWeight / totalWeight  (0 when totalWeight == 0).
//  4. Passed = Score >= Threshold, where a zero Threshold is treated as 1.0.
//
// Checks with non-positive Weight are skipped entirely (they contribute neither
// to the numerator nor the denominator) but their Name is still included in
// Failures if they did not pass.
func (g Gate) Evaluate(checks []Check) Result {
	threshold := g.Threshold
	if threshold <= 0 {
		threshold = 1.0
	}

	var totalWeight, passingWeight float64
	var failures []string

	for _, c := range checks {
		if c.Weight > 0 {
			totalWeight += c.Weight
			if c.Passed {
				passingWeight += c.Weight
			}
		}
		if !c.Passed {
			failures = append(failures, c.Name)
		}
	}

	var score float64
	if totalWeight > 0 {
		score = passingWeight / totalWeight
	}

	return Result{
		Passed:   score >= threshold,
		Score:    score,
		Failures: failures,
	}
}
