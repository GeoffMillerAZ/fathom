// Package integrityevaluator defines the CUE schema for the integrity-evaluator
// foundation. These constraints mirror the Go types and are evaluated with
// `cue vet ./...`.
package integrityevaluator

// #ViolationKind enumerates the recognised integrity violation categories.
#ViolationKind: "fabricated_ref" | "unresolvable_ref"

// #Violation is a single integrity breach found during evaluation.
#Violation: {
	// Kind identifies the category of violation.
	Kind: #ViolationKind

	// Ref is the reference string that triggered the violation.
	Ref: string & !=""

	// Detail is a non-empty human-readable explanation.
	Detail: string & !=""
}

// #Report is the result of an integrity evaluation.
#Report: {
	// Passed is true when no violations were found.
	Passed: bool

	// Violations lists every integrity breach; may be empty.
	Violations: [...#Violation]
}
