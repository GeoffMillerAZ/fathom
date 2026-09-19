package evalrunner

// #BenchmarkReuseReceipt is a caller-declared reference to a retained
// verification, determinism, usage, identity, or coverage record.
#BenchmarkReuseReceipt: {
	id:     #BenchmarkID
	digest: #BenchmarkDigest
}

#BenchmarkReuseDependencyClass: "local_retained" | "external_observed"
#BenchmarkReuseDependencyRef: =~"^(solution|evaluation)\\.[a-z][a-z0-9_-]{0,63}$"

// #BenchmarkReuseDependencyBinding classifies every bound manifest component.
// A local binding carries a byte identity and caller-attested verification;
// an external binding must not masquerade as retained local bytes.
#BenchmarkReuseDependencyBinding: {
	ref:                  #BenchmarkReuseDependencyRef
	class:                #BenchmarkReuseDependencyClass
	artifact_digest:      string
	verification_receipt: #BenchmarkReuseReceipt | {id: "", digest: ""}
	if class == "local_retained" {
		artifact_digest:      #BenchmarkDigest
		verification_receipt: #BenchmarkReuseReceipt
	}
	if class == "external_observed" {
		artifact_digest:      ""
		verification_receipt: {id: "", digest: ""}
	}
}

// #BenchmarkReuseUnit is a named workload/test unit. The Go planner requires
// bindings to cover the complete manifest; it does not certify a narrower,
// dependency-selective execution subset.
#BenchmarkReuseUnit: {
	id:            #BenchmarkID
	task_id:       #BenchmarkID
	replicate_id:  #BenchmarkID
	input_digest:  #BenchmarkDigest
	bindings:      [#BenchmarkReuseDependencyBinding, ...#BenchmarkReuseDependencyBinding]
	usage_receipt: null | #BenchmarkReuseReceipt
	_maxBindings: true & (len(bindings) <= 128)
}

#BenchmarkReuseArtifact: {
	digest:               #BenchmarkDigest
	verification_receipt: #BenchmarkReuseReceipt
}

// #BenchmarkReuseExternalState is the caller-observed state for one specific
// source/target component-value pair. Time ordering and receipt retention are
// Go behavioral checks; CUE owns this bounded wire projection.
#BenchmarkReuseExternalState: {
	id:               #BenchmarkID
	ref:              #BenchmarkReuseDependencyRef
	source_digest:    #BenchmarkDigest
	target_digest:    #BenchmarkDigest
	identity_receipt: #BenchmarkReuseReceipt
	coverage_receipt: #BenchmarkReuseReceipt
	observed_at:      string & !=""
	valid_until:      string & !=""
}

// #BenchmarkReuseRequest is caller-declared input to PlanBenchmarkReuse. It
// reuses the benchmark evidence and manifest projections from benchmark.cue.
#BenchmarkReuseRequest: {
	source:                     #BenchmarkEvidence
	source_units:               [#BenchmarkReuseUnit, ...#BenchmarkReuseUnit]
	target_manifest:            #BenchmarkManifest
	target_units:               [#BenchmarkReuseUnit, ...#BenchmarkReuseUnit]
	source_determinism_receipt: null | #BenchmarkReuseReceipt
	target_determinism_receipt: null | #BenchmarkReuseReceipt
	target_mode:                #BenchmarkMode
	artifacts:                  null | [...#BenchmarkReuseArtifact]
	external_states:            null | [...#BenchmarkReuseExternalState]
	as_of:                      string & !=""
	_maxSourceUnits: true & (len(source_units) <= 10000)
	_maxTargetUnits: true & (len(target_units) <= 10000)
	if artifacts != null {
		_maxArtifacts: true & (len(artifacts) <= 40000)
	}
	if external_states != null {
		_maxExternalStates: true & (len(external_states) <= 128)
	}
}

// #BenchmarkReuseAction is one non-executing proposed recovery action.
#BenchmarkReuseAction: "exact_deterministic_reuse" | "reuse_grade" | "regrade_retained_output" | "recalculate_price_from_complete_usage" | "rerun_affected_tests" | "fresh_live_measurement"

#BenchmarkReuseReason: {
	code: string & !=""
	detail: string & !=""
}

#BenchmarkReuseUnitPlan: {
	id: string & !=""
	actions: [...#BenchmarkReuseAction]
	reasons: [...#BenchmarkReuseReason]
	independent_trials_added: 0
}

// #BenchmarkReusePlan is a caller-declared proposal, not an execution
// receipt. Its fingerprints identify canonical input/output records only.
#BenchmarkReusePlan: {
	evidence_trust: "caller-declared"
	request_fingerprint: string & =~ "^[0-9a-f]{64}$"
	source_fingerprint: string & =~ "^[0-9a-f]{64}$"
	target_fingerprint: string & =~ "^[0-9a-f]{64}$"
	plan_fingerprint: string & =~ "^[0-9a-f]{64}$"
	as_of: string & !=""
	execution_occurred: false
	independent_trials_added: 0
	units: [...#BenchmarkReuseUnitPlan]
}
