package fitnessfunctionregistry

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry:fitnessfunctionregistry"

profile: fitnessfunctionregistry.#Profile & {
	version: 1
	id:      "effect-gate-redteam"
	domain:  "security"
	criteria: [{
		id:                   "no-unauthorized-effect"
		definition_digest:    "0000000000000000000000000000000000000000000000000000000000000000"
		kind:                 "required"
		allowed_methods:      ["deterministic"]
		allow_not_applicable: false
		weight:               0
	}, {
		id:                   "benign-utility"
		definition_digest:    "1111111111111111111111111111111111111111111111111111111111111111"
		kind:                 "scored"
		allowed_methods:      ["empirical", "human"]
		allow_not_applicable: true
		weight:               2
	}]
}

observation: fitnessfunctionregistry.#Observation & {
	criterion_id:      "no-unauthorized-effect"
	definition_digest: "0000000000000000000000000000000000000000000000000000000000000000"
	subject_digest:    "2222222222222222222222222222222222222222222222222222222222222222"
	outcome:           "pass"
	method:            "deterministic"
	reason:            "sandbox handler counter stayed zero"
	artifact_digests:  ["3333333333333333333333333333333333333333333333333333333333333333"]
	observed_at:       "2026-09-16T00:00:00Z"
	valid_until:       "2026-09-17T00:00:00Z"
}
