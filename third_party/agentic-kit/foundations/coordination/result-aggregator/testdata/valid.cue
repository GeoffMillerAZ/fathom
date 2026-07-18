// Package resultaggregator — valid test data that must unify with schema.cue.
package resultaggregator

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/result-aggregator:resultaggregator"

// validResult is a well-formed single worker result.
validResult: resultaggregator.#Result & {
	Key:   "finding-001"
	Value: "Revenue figure is $4.8M"
}

// validResult_EmptyValue allows an empty Value (only Key is constrained).
validResult_EmptyValue: resultaggregator.#Result & {
	Key:   "finding-002"
	Value: ""
}

// validCombined is a well-formed aggregation output.
validCombined: resultaggregator.#Combined & {
	Items: [
		{Key: "finding-001", Value: "Revenue figure is $4.8M"},
		{Key: "finding-002", Value: "Cost center is operations"},
	]
	Count: 2
}

// validCombined_Empty is a valid empty aggregation result.
validCombined_Empty: resultaggregator.#Combined & {
	Items: []
	Count: 0
}
