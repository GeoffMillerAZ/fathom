package semantic

// A well-formed second layer definition (what layer 2 would look like).
failureMode: #Layer & {
	name:           "failure-mode"
	version:        "v1"
	kind:           "failure-mode"
	corpus: include: ["docs/dogfood-observations.md", "docs/postmortems/**"]
	prompt_version: "failure-mode/v1"
	prompt_file:    "schema/semantic/layers/failure-mode/prompt.md"
	tier:           "sme-stated"
	budgets: {
		max_tokens_per_run: 100000
		max_usd_per_run:    0.5
	}
}

// Every #FindingClass value validates (docs/design/findings-as-evidence.md
// §3.2, §6 item 2) — a positive instance per vocabulary member, kept in
// findingclass.cue's own declared order.
findingClassCorrectness:        #FindingClass & "correctness"
findingClassContractViolation:  #FindingClass & "contract-violation"
findingClassResourceLeak:       #FindingClass & "resource-leak"
findingClassErrorHandling:      #FindingClass & "error-handling"
findingClassConcurrency:        #FindingClass & "concurrency"
findingClassSecurity:           #FindingClass & "security"
findingClassConfigDrift:        #FindingClass & "config-drift"
findingClassTestGap:            #FindingClass & "test-gap"
findingClassPerformance:        #FindingClass & "performance"
findingClassConformance:        #FindingClass & "conformance"
