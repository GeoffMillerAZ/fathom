package bench

// Synthetic public subject; the repo entry keeps corpus.d's field names.
subject: #Subject & {
	name:    "sample/subject"
	url:     "https://example.org/sample/subject"
	ref:     "v1.2.3"
	sha:     "0123456789abcdef0123456789abcdef01234567"
	licence: "MIT"
	subpath: "lib"
	bench: {
		toolchain: {
			versions: {go: "1.26.2"}
			lockfile_sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		}
		setup: ["go mod download"]
		test: ["go test ./..."]
		suite_seconds: 12.5
		flake_runs:    3
		l0_reading:    "workspace/bench/l0/subject.json"
		split:         "dev"
		task_sources: [{dataset: "synthetic-fixes", revision: "v1", licence: "CC0-1.0"}]
		contamination: {age: 365, popularity_band: "low", fresh_after: "2026-01-01"}
	}
}

// Prior reads are concrete evidence and keep the subject in the dev split.
priorReadSubject: subject & {
	bench: {
		split: "dev"
		contamination: prior_reads: ["docs/bench/scenario-runs.md"]
	}
}

// No split until controller vetting (D25).
unassigned: #Subject & {
	name:    "sample/subject"
	url:     "https://example.org/sample/subject"
	ref:     "v1.2.3"
	sha:     "0123456789abcdef0123456789abcdef01234567"
	licence: "MIT"
	subpath: "lib"
	bench: {
		toolchain: {
			versions: {go: "1.26.2"}
			lockfile_sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		}
		setup: ["go mod download"]
		test: ["go test ./..."]
		suite_seconds: 12.5
		flake_runs:    3
		l0_reading:    "workspace/bench/l0/subject.json"
		task_sources: [{dataset: "synthetic-fixes", revision: "v1", licence: "CC0-1.0"}]
		contamination: {age: 365, popularity_band: "low", fresh_after: "2026-01-01"}
	}
}

// A win and a predicted null, fixed before data. Each primary fixes its n.
prereg: #Prereg & {
	id: "localization-v1"
	strata: ["non-lexical", "lexical"]
	arms: ["A1", "A2"]
	baseline_arm: "A1"
	hypotheses: [{
		id:        "H4"
		statement: "The surface increases localization accuracy on non-lexical tasks."
		predictions: [
			{stratum: "non-lexical", arm: "A2", direction: "increase"},
			{stratum: "lexical", arm: "A2", direction: "null", equivalence_margin: 0.05},
		]
		primary_metric:             "hit@3"
		minimum_effect_of_interest: 0.10
		n:                          120
		test:                       "paired-mean-difference"
	}]
	alpha: 0.05
	multiplicity_correction: {method: "holm", family: ["H4"]}
	three_outcome_rule: {
		confirmed:    "lower_bound_above_zero"
		refuted:      "upper_bound_below_mei"
		inconclusive: "otherwise"
		tie:          "refuted"
		predicted_null: {
			equivalent:   "90_percent_interval_inside_symmetric_margin"
			harm:         "95_percent_upper_bound_below_zero_counts_as_loss"
			inconclusive: "otherwise"
			above_delta:  "unexpected_effect_not_win"
		}
	}
	turn_cap:     30
	cost_cap_usd: 100
	seed:         42
	exclusions: ["A0 hit@3 in at least two of three trials"]
	deviation_log: []
}

// D28 counts leaf tests; one exclusion is exactly two percent of fifty.
exclusionSubject: subject & {
	bench: {
		suite_tests: 50
		excluded: [{test: "TestService/offline", reason: "missing host service", log: "dial tcp: connection refused"}]
	}
}
