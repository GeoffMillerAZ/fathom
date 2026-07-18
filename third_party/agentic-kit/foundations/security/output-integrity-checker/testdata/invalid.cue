// invalid.cue — examples that do NOT satisfy the output-integrity-checker schema.
// All examples are commented out. Uncomment any block to observe the cue vet error.
package outputintegritychecker

// BadKind: demonstrates a Violation with an invalid Kind.
// cue vet error: Kind: 2 errors in empty disjunction:
//   conflicting values "policy_violation" and "missing_disclaimer" ...
//
// BadViolation: {
// 	RuleID:  "RD-001"
// 	Kind:    "policy_violation"  // ERROR: must be "missing_disclaimer" or "banned_token"
// 	Detail:  "something"
// 	Excerpt: ""
// }

// EmptyRuleID: demonstrates a Violation with an empty RuleID.
// cue vet error: RuleID: invalid value "" (does not satisfy != "")
//
// EmptyRuleIDViolation: {
// 	RuleID:  ""  // ERROR: must be non-empty
// 	Kind:    "banned_token"
// 	Detail:  "some detail"
// 	Excerpt: "matched text"
// }

// EmptyPolicyName: demonstrates an IntegrityPolicy with an empty Name.
// cue vet error: Name: invalid value "" (does not satisfy != "")
//
// EmptyNamePolicy: {
// 	Name:                ""  // ERROR: must be non-empty
// 	Version:             "1.0.0"
// 	RequiredDisclaimers: []
// 	BannedTokens:        []
// }
