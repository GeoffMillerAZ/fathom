// Package scenariobank — valid test fixtures that must unify with schema.cue.
// Run `cue vet ./...` from the package root or `cue vet ./testdata/` to
// validate these against the schema.
package scenariobank

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/scenario-bank:scenariobank"

// validSyntheticMinimal is the simplest valid scenario: synthetic provenance,
// no rubric, no expected output.
validSyntheticMinimal: scenariobank.#Scenario & {
	id:                "scenario-synthetic-001"
	version:           1
	provenance:        "synthetic-adversarial"
	decisive_eligible: false
	input: {prompt: "What is 2+2?"}
}

// validRedactedWithRubric exercises a redacted production scenario with a
// full rubric.
validRedactedWithRubric: scenariobank.#Scenario & {
	id:                "scenario-redacted-001"
	version:           2
	provenance:        "production-derived-redacted"
	decisive_eligible: true
	input: {text: "summarise the following document"}
	expected: {summary: "a concise summary"}
	rubric: {
		criteria: [
			{
				name:   "output-nonempty"
				weight: 1.0
				predicate: {kind: "nonempty", arg: ""}
			},
			{
				name:   "contains-keyword"
				weight: 2.0
				predicate: {kind: "contains", arg: "summary"}
			},
		]
	}
}

// validRegexPredicate exercises the regex predicate kind.
validRegexPredicate: scenariobank.#Rubric & {
	criteria: [
		{
			name:   "matches-pattern"
			weight: 1.0
			predicate: {kind: "regex", arg: "^[A-Z].*\\.$"}
		},
	]
}

// validJSONPathPredicate exercises the json_path_eq predicate kind.
validJSONPathPredicate: scenariobank.#Rubric & {
	criteria: [
		{
			name:   "field-value-check"
			weight: 1.0
			predicate: {kind: "json_path_eq", arg: "/status", value: "ok"}
		},
	]
}
