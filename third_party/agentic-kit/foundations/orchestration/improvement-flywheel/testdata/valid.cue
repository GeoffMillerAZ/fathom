// Package improvementflywheel — valid test data that must unify with schema.cue.
package improvementflywheel

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/improvement-flywheel:improvementflywheel"

// validSignal — a well-formed false-positive signal.
validSignal: improvementflywheel.#Signal & {
	type:        "false_positive"
	key:         "revenue/q4-concern"
	workflow_id: "wf-001"
	detail:      "Dismissed: within expected variance"
}

// validObservation — a repeated pattern seen 5 times.
validObservation: improvementflywheel.#PatternObservation & {
	type:         "false_positive"
	key:          "revenue/q4-concern"
	frequency:    5
	workflow_ids: ["wf-001", "wf-002", "wf-003"]
	examples:     ["example 1", "example 2"]
}

// validProposal — a proposal derived from repeated observations.
validProposal: improvementflywheel.#Proposal & {
	id:                      "proposal-false_positive-revenue/q4-concern"
	observation_key:         "false_positive/revenue/q4-concern"
	pattern_type:            "false_positive"
	change_target:           "catalog_entry"
	summary:                 "Pattern 'revenue/q4-concern' (false_positive) observed 5 time(s)"
	frequency:               5
	supporting_workflow_ids: ["wf-001", "wf-002", "wf-003"]
}

// validAllPatternTypes — verify all enum values.
_p1: improvementflywheel.#PatternType & "false_positive"
_p2: improvementflywheel.#PatternType & "missed_claim"
_p3: improvementflywheel.#PatternType & "repeated_correction"
_p4: improvementflywheel.#PatternType & "operator_friction"

// validAllChangeTargets — verify all enum values.
_c1: improvementflywheel.#ChangeTarget & "catalog_entry"
_c2: improvementflywheel.#ChangeTarget & "prose_policy"
_c3: improvementflywheel.#ChangeTarget & "prompt_section"
_c4: improvementflywheel.#ChangeTarget & "cue_constraint"
_c5: improvementflywheel.#ChangeTarget & "foundation_behavior"
