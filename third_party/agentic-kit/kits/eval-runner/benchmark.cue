// Experimental benchmark-evidence data contract. This owns the serialized
// record shape; benchmark.go owns validation, hashing, and statistics.
// Values are caller evidence, never a provider catalog, price table, or
// default model selection.
package evalrunner

#BenchmarkDigest: string & =~"^[0-9a-f]{64}$"

#BenchmarkOutcome: "accepted" | "rejected" | "error" | "timeout" | "canceled" | "abandoned" | "unknown"
#BenchmarkCostBasis: "observed" | "usage" | "estimated" | "unknown"
#BenchmarkMode: "live" | "replay" | "synthetic"

#BenchmarkID: string & =~"^[ -~]{1,256}$" & =~"[!-~]"

#BenchmarkComponentMap: {
	[=~"^[a-z][a-z0-9_-]{0,63}$"]: #BenchmarkDigest
}

#BenchmarkSolution: #BenchmarkComponentMap & {
	code:       #BenchmarkDigest
	prompt:     #BenchmarkDigest
	model:      #BenchmarkDigest
	parameters: #BenchmarkDigest
	tools:      #BenchmarkDigest
	workflow:   #BenchmarkDigest
	retrieval:  #BenchmarkDigest
	context:    #BenchmarkDigest
}

#BenchmarkEvaluation: #BenchmarkComponentMap & {
	dataset:       #BenchmarkDigest
	acceptance:    #BenchmarkDigest
	grader:        #BenchmarkDigest
	protocol:      #BenchmarkDigest
	environment:   #BenchmarkDigest
	pricing:       #BenchmarkDigest
	useful_result: #BenchmarkDigest
}

#BenchmarkManifest: {
	version:  1
	workload: #BenchmarkID
	solution: #BenchmarkSolution
	evaluation: #BenchmarkEvaluation
	_maxSolutionItems:   true & (len(solution) <= 64)
	_maxEvaluationItems: true & (len(evaluation) <= 64)
}

// Pointer fields are deliberately required and nullable: Go serializes each
// optional pointer as a JSON null when the datum was not observed.
#BenchmarkTrial: {
	task_id:                #BenchmarkID
	replicate_id:           #BenchmarkID
	outcome:                #BenchmarkOutcome
	attempt_count:          int & >=1 & <=10000
	trial_spec_digest:      #BenchmarkDigest
	output_digest:          null | #BenchmarkDigest
	acceptance_digest:      null | #BenchmarkDigest
	cost_usd:               null | (number & >=0)
	cost_basis:             #BenchmarkCostBasis
	price_digest:           null | #BenchmarkDigest
	cost_complete:          bool
	duration_ms:            number & >=0
	accepted_ms:            null | (number & >=0)
	first_visible_ms:       null | (number & >=0)
	first_useful_ms:        null | (number & >=0)
	if outcome == "accepted" {
		output_digest: #BenchmarkDigest
		acceptance_digest: #BenchmarkDigest
		accepted_ms:   number & >=duration_ms
		first_visible_ms: null | (number & >=0 & <=accepted_ms)
		first_useful_ms:  null | (number & >=0 & <=accepted_ms)
	}
	if outcome != "accepted" {
		accepted_ms: null
		first_visible_ms: null | (number & >=0 & <=duration_ms)
		first_useful_ms:  null | (number & >=0 & <=duration_ms)
	}
	if outcome == "rejected" {
		acceptance_digest: #BenchmarkDigest
	}
	if outcome != "accepted" && outcome != "rejected" {
		acceptance_digest: null
	}
	if cost_basis == "unknown" {
		cost_usd:      null
		price_digest:  null
		cost_complete: false
	}
	if cost_basis != "unknown" {
		cost_usd:      number & >=0
		price_digest:  #BenchmarkDigest
	}
}

#BenchmarkEvidence: {
	manifest:    #BenchmarkManifest
	mode:        #BenchmarkMode
	observed_at: string & !=""
	valid_until: string & !=""
	trials: [...#BenchmarkTrial]
	_maxTrials: true & (len(trials) <= 10000)
}

// #BenchmarkSummary mirrors the JSON emitted by SummarizeBenchmark. Null
// metrics are intentionally distinct from a measured zero.
#BenchmarkSummary: {
	evidence_trust:                "caller-declared"
	mode:                          #BenchmarkMode
	observed_at:                   string & !=""
	valid_until:                   string & !=""
	manifest_fingerprint:          #BenchmarkDigest
	evidence_fingerprint:          #BenchmarkDigest
	trials:                        int & >=0
	attempts:                      int & >=0
	outcomes:                      [string]: int & >=0
	first_attempt_accepted:        int & >=0
	eventual_accepted:             int & >=0
	first_attempt_success_rate:    null | (number & >=0 & <=1)
	eventual_success_rate:         null | (number & >=0 & <=1)
	known_cost_trials:             int & >=0
	missing_cost_trials:           int & >=0
	total_known_cost_usd:          number & >=0
	cost_basis_counts:             [string]: int & >=0
	total_cost_by_basis_usd:       [string]: number & >=0
	cost_coverage_complete:        bool
	cost_lower_bound:              bool
	cost_warning:                  string
	cost_per_attempted_task_usd:   null | (number & >=0)
	cost_per_accepted_usd:         null | (number & >=0)
	p50_duration_ms:               null | (number & >=0)
	p95_duration_ms:               null | (number & >=0)
	accepted_trials:               int & >=0
	p50_accepted_ms:               null | (number & >=0)
	p95_accepted_ms:               null | (number & >=0)
}

// #BenchmarkComparison mirrors the JSON emitted by CompareBenchmarks. It is
// a conditional paired calculation, never an observed-live or independence
// assertion.
#BenchmarkComparison: {
	evidence_trust:        "caller-declared"
	baseline_mode:         #BenchmarkMode
	candidate_mode:        #BenchmarkMode
	baseline_observed_at:  string & !=""
	candidate_observed_at: string & !=""
	baseline_valid_until:  string & !=""
	candidate_valid_until: string & !=""
	baseline_fingerprint:  #BenchmarkDigest
	candidate_fingerprint: #BenchmarkDigest
	baseline_evidence_fingerprint: #BenchmarkDigest
	candidate_evidence_fingerprint: #BenchmarkDigest
	as_of:                 string & !=""
	seed:                  int
	resamples:             int & >=100 & <=10000
	level:                 number & >0 & <1
	difference: {
		low:   number
		high:  number
		point: number
		tasks: int & >=0
	}
	pairs:              int & >=0
	baseline_accepted:  int & >=0
	candidate_accepted: int & >=0
	warning:            string
}
