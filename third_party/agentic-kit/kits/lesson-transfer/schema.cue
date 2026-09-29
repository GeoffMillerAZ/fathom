// Package lessontransfer defines the portable public-synthetic lesson data shape.
// Go additionally checks exact raw-byte bindings, UTF-8, duplicate keys and callback
// freshness; CUE owns only structural interchange data.
package lessontransfer

_nonempty: string & != ""
_digest: string & =~ "^[0-9a-f]{64}$"

#Origin: {workload: _nonempty, snapshot_digest: _digest, incident_digest: _digest, scenario_digest: _digest}
#Applicability: {workloads: [_nonempty, ..._nonempty], config_digests: [..._digest]}
#ReproducerRef: {schema: "lesson-reproducer/v1", version: 1, file: "reproducer.json", raw_digest: _digest, expected: "unknown" | "fail", max_bytes: int & >0 & <=262144}
#Remedy: {id: _nonempty, revision: int & >0, digest: _digest, test_digest: _digest}
#Revocation: {status: "active" | "revoked", reason?: string, recheck: true}

#Payload: {
	version: "lesson-transfer/v1"
	id: _nonempty
	revision: int & >0
	status: "candidate" | "reviewed" | "superseded" | "revoked"
	classification: "public-synthetic"
	origin: #Origin
	applicability: #Applicability
	exclusions: [..._nonempty]
	reproducer: #ReproducerRef
	remedies: [#Remedy, ...#Remedy]
	supersedes: [..._nonempty]
	revocation: #Revocation
}

#SyntheticInput: {case: "unknown-family" | "unobserved-control"}
#Reproducer: {schema: "lesson-reproducer/v1", version: 1, input: #SyntheticInput, expected: "unknown" | "fail", invariant: _nonempty}
#Attestations: {payload_digest: _digest, reproducer_digest: _digest, reviewer: _nonempty, reviewed_at: _nonempty, scope: _nonempty}
#Receiver: {workload: _nonempty, revision: _digest, config_digest: _digest}
// Go additionally bounds detail to 4096 UTF-8 bytes before receipt creation.
#Outcome: {classification: "unknown" | "fail" | "pass", evidence: _digest, detail: _nonempty}
#Receipt: {version: "lesson-transfer/v1", payload_digest: _digest, reproducer_digest: _digest, receiver: #Receiver, outcome: #Outcome, integrity: "pass", reproduction: "pass", fresh: true, checked_at: _nonempty}
#CLIOutput: {
	version: "kit-workflow/lesson/v1"
	operation: "lesson/validate" | "lesson/adoption-check"
	payload_digest: _digest
	reproducer_digest: _digest
	receiver?: #Receiver
	integrity: "bound"
	reproduction: "not-run" | "asserted-unverified"
	fresh: false
	checked_at: _nonempty
	boundary: _nonempty
}

// #Bundle is the structural interchange form used by CUE fixtures. Go binds
// each component's exact raw UTF-8 bytes before it exposes a Bundle.
#Bundle: {lesson: #Payload, reproducer: #Reproducer, attestations: #Attestations}
