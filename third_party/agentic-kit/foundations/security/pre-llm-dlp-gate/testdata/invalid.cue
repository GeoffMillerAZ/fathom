// invalid.cue — examples that do NOT satisfy #RedactPolicy.
// All examples are commented out. Uncomment any block to observe the cue vet error.
package prellmdlpgate

// EmptyLabel: demonstrates a rule with an empty Label.
// cue vet error: Label: invalid value "" (does not satisfy != "")
//
// EmptyLabelPolicy: {
// 	Name:    "bad-v1"
// 	Version: "1.0.0"
// 	Rules: [
// 		{
// 			ID:          "DLP-BAD"
// 			Pattern:     "test"
// 			Label:       ""  // ERROR: label must be non-empty
// 			Description: "Missing label."
// 		},
// 	]
// }

// EmptyRules: demonstrates a policy with no rules.
// cue vet error: Rules: length must be >= 1
//
// EmptyRulesPolicy: {
// 	Name:    "empty-v1"
// 	Version: "1.0.0"
// 	Rules:   []  // ERROR: must have at least one rule
// }
