// A JSON-shaped experimental benchmark evidence fixture. The import validates
// the public CUE contract rather than relying on package-local definitions.
package evalrunner

import "github.com/geoffmilleraz/agentic-kit/kits/eval-runner:evalrunner"

benchmarkEvidence: evalrunner.#BenchmarkEvidence & {
	manifest: {
		version:  1
		workload: "example-workload"
		solution: {
			code:       "0000000000000000000000000000000000000000000000000000000000000000"
			prompt:     "1111111111111111111111111111111111111111111111111111111111111111"
			model:      "2222222222222222222222222222222222222222222222222222222222222222"
			parameters: "3333333333333333333333333333333333333333333333333333333333333333"
			tools:      "4444444444444444444444444444444444444444444444444444444444444444"
			workflow:   "5555555555555555555555555555555555555555555555555555555555555555"
			retrieval:  "6666666666666666666666666666666666666666666666666666666666666666"
			context:    "7777777777777777777777777777777777777777777777777777777777777777"
		}
		evaluation: {
			dataset:       "8888888888888888888888888888888888888888888888888888888888888888"
			acceptance:    "9999999999999999999999999999999999999999999999999999999999999999"
			grader:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			protocol:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			environment:   "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			pricing:       "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
			useful_result: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		}
	}
	mode:        "live"
	observed_at: "2026-09-16T00:00:00Z"
	valid_until: "2026-09-17T00:00:00Z"
	trials: [{
		task_id:                "task-1"
		replicate_id:           "r-1"
		outcome:                "accepted"
		attempt_count:          1
		trial_spec_digest:      "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
		output_digest:          "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		acceptance_digest:      "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
		cost_usd:               0.01
		cost_basis:             "observed"
		price_digest:           "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		cost_complete:          true
		duration_ms:            120
		accepted_ms:            150
		first_visible_ms:       80
		first_useful_ms:        100
	}]
}
