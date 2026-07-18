package agenticpracticesevaluator

// Evaluate scores run against each practice in practices and returns a
// Scorecard. The result is deterministic: for the same (run, practices) input
// the output is always identical. Practices are evaluated in order; the Misses
// slice preserves that order.
//
// If practices is empty the returned Scorecard has Score=1.0 and no Misses.
func Evaluate(run AgentRun, practices []Practice) Scorecard {
	if len(practices) == 0 {
		return Scorecard{Score: 1.0}
	}

	var misses []Miss
	for _, p := range practices {
		if p.Check == nil || !p.Check(run) {
			misses = append(misses, Miss{
				PracticeName: p.Name,
				Description:  p.Description,
			})
		}
	}

	passed := len(practices) - len(misses)
	score := float64(passed) / float64(len(practices))
	return Scorecard{
		Score:  score,
		Misses: misses,
	}
}
