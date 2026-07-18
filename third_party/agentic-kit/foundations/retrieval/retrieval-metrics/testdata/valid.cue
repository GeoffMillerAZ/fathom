// Positive CUE fixture for retrieval-metrics schema conformance.
// cue vet schema.cue testdata/valid.cue must pass.
package retrievalmetrics

// An applicable metric result.
_applicableResult: #MetricResult & {
	value:      0.75
	applicable: true
	reason:     ""
}

// A not-applicable metric result (reason is required).
_notApplicableResult: #MetricResult & {
	value:      0
	applicable: false
	reason:     "MRR undefined: gold set is empty"
}

// A canary record where the canary was found.
_foundCanary: #CanaryRecord & {
	signature_id: "concern-no-hardcoded-secrets"
	canary_found: true
}

// A canary record where the canary was NOT found (false absent case).
_missedCanary: #CanaryRecord & {
	signature_id: "concern-no-hardcoded-secrets"
	canary_found: false
}
