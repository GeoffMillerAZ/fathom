// Package scenariobank defines the CUE-first contract for the scenario-bank
// foundation. Go types in types.go mirror these definitions; any drift is a
// bug. These constraints are evaluated with `cue vet ./...`.
package scenariobank

// #Provenance enumerates the permissible provenance tags for a scenario. The
// choice of provenance tag carries PII-safety implications — see Go doc for
// details.
#Provenance: "production-captured" | "production-derived-redacted" | "synthetic-adversarial"

// #PredicateKind enumerates the interpretable predicate vocabulary. Predicates
// are compiled into qa-harness RubricCriteria by RubricJudgeFrom; they are
// never Go closures in committed data.
#PredicateKind: "contains" | "regex" | "json_path_eq" | "nonempty"

// #Criterion is one weighted rubric criterion with a declarative predicate.
#Criterion: {
	// name is a non-empty human-readable identifier for this criterion.
	name: string & !=""

	// weight is the relative importance. Must be strictly positive.
	weight: number & >0

	// predicate is the interpretable machine-checkable claim.
	predicate: {
		// kind selects the check algorithm.
		kind: #PredicateKind

		// arg is the primary argument (substring, regex pattern, JSON path, or
		// empty for "nonempty").
		arg: string

		// value is the expected value for json_path_eq predicates; optional
		// for other kinds.
		value?: string
	}
}

// #Rubric is an ordered set of weighted criteria that a RubricJudge evaluates.
#Rubric: {
	// criteria is the ordered list of evaluation dimensions. May be empty (a
	// rubric with no criteria scores 0 via qa-harness semantics).
	criteria: [...#Criterion]
}

// #Scenario is one versioned evaluation scenario. Provenance gates apply:
// production-captured scenarios are rejected in committed testdata.
//
// Key invariant: decisive_eligible must be false when provenance is
// synthetic-adversarial (enforced in Go by Validate).
#Scenario: {
	// id is a non-empty stable identifier for this scenario.
	id: string & !=""

	// version is a positive integer tracking the scenario revision.
	version: int & >=1

	// provenance records the origin of the scenario data.
	provenance: #Provenance

	// decisive_eligible indicates whether records against this scenario may be
	// marked decisive. Synthetic-adversarial scenarios MUST have this false.
	decisive_eligible: bool

	// input is the scenario's input payload. Shape is caller-defined.
	input: {...}

	// expected is an optional gold-standard expected output.
	expected?: {...}

	// rubric is an optional evaluation rubric for this scenario.
	rubric?: #Rubric
}
