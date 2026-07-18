// Package integrityevaluator — valid test data that must unify with schema.cue.
package integrityevaluator

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/integrity-evaluator:integrityevaluator"

// validReportPassed demonstrates a clean report with no violations.
validReportPassed: integrityevaluator.#Report & {
	Passed:     true
	Violations: []
}

// validReportFailed demonstrates a report with violations.
validReportFailed: integrityevaluator.#Report & {
	Passed: false
	Violations: [
		{
			Kind:   "fabricated_ref"
			Ref:    "[42]"
			Detail: "reference \"[42]\" appears in output but is not present in knownRefs"
		},
	]
}

// validViolationFabricated demonstrates a fabricated-ref violation.
validViolationFabricated: integrityevaluator.#Violation & {
	Kind:   "fabricated_ref"
	Ref:    "[99]"
	Detail: "reference \"[99]\" appears in output but is not present in knownRefs"
}

// validViolationUnresolvable demonstrates an unresolvable-ref violation.
validViolationUnresolvable: integrityevaluator.#Violation & {
	Kind:   "unresolvable_ref"
	Ref:    "doc:missing"
	Detail: "reference \"doc:missing\" is present in knownRefs but resolves to an empty value"
}

// validKinds verifies all violation kind enum values are accepted.
_k1: integrityevaluator.#ViolationKind & "fabricated_ref"
_k2: integrityevaluator.#ViolationKind & "unresolvable_ref"
