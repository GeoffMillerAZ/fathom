// Package reliabilitystats — valid test data that must unify with schema.cue.
package reliabilitystats

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats:reliabilitystats"

// validObservationGain is one trial where the treatment scored higher.
validObservationGain: reliabilitystats.#Observation & {
	task: "scenario-001"
	diff: 0.25
}

// validObservationLoss is one trial where the treatment scored lower.
validObservationLoss: reliabilitystats.#Observation & {
	task: "scenario-002"
	diff: -0.1
}

// validIntervalPositive is an interval that excludes zero.
validIntervalPositive: reliabilitystats.#Interval & {
	low:   0.05
	high:  0.31
	point: 0.18
	tasks: 12
}

// validIntervalDegenerate is a single-task interval collapsed to its point.
validIntervalDegenerate: reliabilitystats.#Interval & {
	low:   0.4
	high:  0.4
	point: 0.4
	tasks: 1
}

// validWilcoxonExact is an exact (n <= 25) two-sided result.
validWilcoxonExact: reliabilitystats.#WilcoxonResult & {
	w:       8
	w_plus:  47
	w_minus: 8
	n:       10
	zeros:   0
	p:       0.048828125
	exact:   true
}

// validWilcoxonApprox is a normal-approximation result with dropped zeros.
validWilcoxonApprox: reliabilitystats.#WilcoxonResult & {
	w:       120.5
	w_plus:  344.5
	w_minus: 120.5
	n:       30
	zeros:   2
	p:       0.0213
	exact:   false
}
