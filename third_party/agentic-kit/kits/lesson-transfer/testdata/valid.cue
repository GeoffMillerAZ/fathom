package lessontransfer

import lesson "github.com/geoffmilleraz/agentic-kit/kits/lesson-transfer:lessontransfer"

payload: lesson.#Payload & {
	version: "lesson-transfer/v1"
	id: "unknown-evidence"
	revision: 1
	status: "reviewed"
	classification: "public-synthetic"
	origin: {workload: "kits/agent-harness", snapshot_digest: "9955737c1df3ea26e823325c89c67c700428f4638703a828506e310393093af5", incident_digest: "4255c32f60fa12116995505ec0bdfaafaf700ca557175a3479a3507a95b1391e", scenario_digest: "405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13"}
	applicability: {workloads: ["resume_as_code"], config_digests: []}
	exclusions: []
	reproducer: {schema: "lesson-reproducer/v1", version: 1, file: "reproducer.json", raw_digest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", expected: "unknown", max_bytes: 262144}
	remedies: [{id: "coverage-gate", revision: 1, digest: "66248ea85be18c455a6a4440b2eade13b0a9d8bc70878f074bb26b2e1c954c4e", test_digest: "405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13"}]
	supersedes: []
	revocation: {status: "active", recheck: true}
}
reproducerData: lesson.#Reproducer & {schema: "lesson-reproducer/v1", version: 1, input: {case: "unknown-family"}, expected: "unknown", invariant: "unknown evidence cannot pass"}
attestationData: lesson.#Attestations & {payload_digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", reproducer_digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", reviewer: "fixture-review", reviewed_at: "2026-09-13T12:00:00Z", scope: "synthetic only"}

bundle: lesson.#Bundle & {lesson: payload, reproducer: reproducerData, attestations: attestationData}
