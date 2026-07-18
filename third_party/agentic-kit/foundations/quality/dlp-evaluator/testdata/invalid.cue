// invalid.cue — values that do NOT satisfy #DLPRuleSet.
// Each bad value is commented out so that `cue vet ./...` passes on this
// directory. Uncomment any block to observe the vet error it produces.
package dlpevaluator

// BadRiskBand demonstrates a rule whose RiskBand is not a member of #RiskBand.
// Uncomment to observe: cue vet will reject the value "urgent".
//
// BadRiskBandRule: dlpevaluator.#DLPRuleSet & {
// 	Name:    "bad-risk-band"
// 	Version: "1.0.0"
// 	Rules: [
// 		{
// 			ID:          "DLP-BAD-001"
// 			Pattern:     ".*"
// 			RiskBand:    "urgent"   // ERROR: not a valid #RiskBand value
// 			Description: "This rule has an invalid risk band."
// 		},
// 	]
// }

// MissingName demonstrates a DLPRuleSet with an empty Name field.
// Uncomment to observe: cue vet will reject the empty string.
//
// MissingNameRuleSet: dlpevaluator.#DLPRuleSet & {
// 	Name:    ""   // ERROR: Name must be non-empty
// 	Version: "1.0.0"
// 	Rules: [
// 		{
// 			ID:          "DLP-001"
// 			Pattern:     ".*"
// 			RiskBand:    "low"
// 			Description: "A rule."
// 		},
// 	]
// }
