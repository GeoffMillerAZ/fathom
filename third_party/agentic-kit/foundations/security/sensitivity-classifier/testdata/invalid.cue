// invalid.cue — examples that do NOT satisfy #Catalog or #ClassRule.
// All examples are commented out. Uncomment any block to observe the cue vet error.
package sensitivityclassifier

// BadLevel: demonstrates an unrecognised sensitivity level in a rule.
// cue vet error: Level: 2 errors in empty disjunction:
//   conflicting values "proprietary" and "public" ...
//
// BadCatalog: {
// 	Name:    "bad-level-v1"
// 	Version: "1.0.0"
// 	Rules: [
// 		{
// 			ID:          "CR-BAD"
// 			Pattern:     "test"
// 			Level:       "proprietary"  // ERROR: not a valid #SensitivityLevel
// 			Description: "Invalid level example."
// 		},
// 	]
// }

// EmptyRules: demonstrates a Catalog with no rules.
// cue vet error: Rules: length must be >= 1
//
// EmptyRulesCatalog: {
// 	Name:    "empty-v1"
// 	Version: "1.0.0"
// 	Rules:   []  // ERROR: must have at least one rule
// }

// EmptyPattern: demonstrates a rule with an empty pattern.
// cue vet error: Pattern: invalid value "" (does not satisfy != "")
//
// EmptyPatternCatalog: {
// 	Name:    "empty-pattern-v1"
// 	Version: "1.0.0"
// 	Rules: [
// 		{
// 			ID:          "CR-EP"
// 			Pattern:     ""  // ERROR: pattern must be non-empty
// 			Level:       "internal"
// 			Description: "Empty pattern example."
// 		},
// 	]
// }
