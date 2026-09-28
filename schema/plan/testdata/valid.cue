// valid.cue — positive fixture for the plan schema.
// This file must pass `cue vet ./...` from schema/plan.
package plan

plan: #Plan & {
	service:  "my-service"
	baseline: "8c1e4f2a9b3d7e6501234567890abcdef123456"
	tasks: [
		{
			id:    "T-1"
			title: "ValidateToken must exist and be tested"
			expect: {
				symbol: "ValidateToken"
			}
		},
		{
			id:      "T-2"
			title:   "US-042: charge flow covered"
			spec_ids: ["[US-042]"]
		},
		{
			// grammar v2: authored state + typed disposition (ADR-018).
			id:          "T-3"
			title:       "cmd oracle"
			state:       "skipped"
			disposition: "deferred"
			reason:      "O6 just landed; measure skip-share first"
			trigger:     "skip_test share still >30% after 4 weeks"
		},
		{
			id:    "T-4"
			title: "bare name disambiguated + justified skip"
			state: "claimed"
			expect: {
				symbol:      "New"
				package:     "internal/verify"
				skip_test:   true
				skip_reason: "reached only via methods until the cmd oracle lands"
			}
		},
		{
			// ADR-030: a non-code row proven by a lexical oracle.
			id:    "T-5"
			title: "measurement recorded durably"
			state: "claimed"
			expect: {
				oracle:  "lexical"
				path:    "docs/measurements/2026-08-27-obligation-ceiling.md"
				pattern: "p95"
			}
		},
		{
			// ADR-044 §3.1/§2.1: the refactor-conservation grammar (CV-25).
			id:    "T-6"
			title: "OldFunc is gone"
			state: "claimed"
			gone:  "callers OldFunc"
			tests: "allowed"
			expect: {
				package: "internal/old"
			}
		},
		{
			id:             "T-7"
			title:          "surface retirement declared"
			retires: ["api:foo.Old*"]
			surface_change: ["json:foo.Bar.tag"]
		},
		{
			// ADR-049 (SI-7/SI-8): weight and accept, both non-gating.
			id:     "T-7a"
			title:  "payment client retries"
			weight: 3
			accept: "WHEN the client times out THE SYSTEM SHALL retry"
		},
		{
			// CV-29: bounded lexical oracle.
			id:    "T-8"
			title: "at most 2 references remain"
			state: "claimed"
			expect: {
				oracle:  "lexical"
				path:    "internal/foo/bar.go"
				pattern: "OldThing"
				max:     2
			}
		},
	]
}
