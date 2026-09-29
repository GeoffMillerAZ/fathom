// A JSON-shaped proposal fixture. Go tests also marshal an actual plan and
// vet it against #BenchmarkReusePlan, keeping the public wire shape aligned.
package evalrunner

import "github.com/geoffmilleraz/agentic-kit/kits/eval-runner:evalrunner"

reusePlan: evalrunner.#BenchmarkReusePlan & {
	evidence_trust: "caller-declared"
	request_fingerprint: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	source_fingerprint: "0000000000000000000000000000000000000000000000000000000000000000"
	target_fingerprint: "1111111111111111111111111111111111111111111111111111111111111111"
	plan_fingerprint: "2222222222222222222222222222222222222222222222222222222222222222"
	as_of: "2026-09-16T00:00:00Z"
	execution_occurred: false
	independent_trials_added: 0
	units: [{
		id: "example-unit"
		actions: ["exact_deterministic_reuse", "reuse_grade"]
		reasons: [{code: "deterministic_exact", detail: "fixture"}]
		independent_trials_added: 0
	}]
}
