// Package reliabilitystats defines the CUE contract for the
// reliability-stats foundation: the shapes its estimators consume and
// emit. These constraints mirror types.go and are evaluated with
// `cue vet ./...`.
package reliabilitystats

// #Observation is one paired trial: the task it belongs to and
// the treatment-minus-baseline difference for that trial. All trials of
// one task resample together.
#Observation: {
	// task is the task id; non-empty.
	task: string & !=""

	// diff is treatment minus baseline for this trial.
	diff: number
}

// #Interval is a percentile bootstrap confidence interval around the
// mean paired difference.
#Interval: {
	// low is the lower percentile bound.
	low: number

	// high is the upper percentile bound; never below low.
	high: number & >=low

	// point is the statistic on the observed sample.
	point: number

	// tasks is the number of distinct tasks in the estimator; at least one.
	tasks: int & >=1
}

// #WilcoxonResult is the outcome of a two-sided Wilcoxon signed-rank
// test on paired differences.
#WilcoxonResult: {
	// w is min(w_plus, w_minus), the conventional T statistic.
	w: number & >=0

	// w_plus is the rank sum of the positive differences.
	w_plus: number & >=0

	// w_minus is the rank sum of the negative differences.
	w_minus: number & >=0

	// n is the pair count after zero differences are dropped; at least one.
	n: int & >=1

	// zeros is the number of zero differences dropped.
	zeros: int & >=0

	// p is the two-sided p-value in [0, 1].
	p: number & >=0 & <=1

	// exact is true when p came from the enumerated null distribution
	// (n <= 25), false when from the normal approximation.
	exact: bool
}

// #TOSTResult mirrors TOSTResult: verdict and paired mean interval.
#TOSTResult: {
	equivalent: bool
	// unexpected_effect reports Low > the caller's margin; verdict stays inconclusive.
	unexpected_effect: bool
	low:               number
	high:              number & >=low
}
