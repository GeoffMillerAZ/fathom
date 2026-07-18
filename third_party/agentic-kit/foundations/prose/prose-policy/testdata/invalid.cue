// invalid.cue — demonstrates values that VIOLATE the schema. This file is
// intentionally NOT included in the `cue vet` path (the valid.cue sibling is
// the one vetted in CI). It is kept here as a documentation artifact so
// reviewers understand what the schema rejects.
//
// The examples below would fail `cue vet` with the following errors:
//
//   _badPosture:
//     "classified" is not one of "open"|"internal"|"restricted"|"secret"
//
//   _badRuleEmptyTerm:
//     Terms element "" violates _nonEmpty (string & !="")
//
//   _badPolicyBadPosture:
//     Posture: "public" violates #Posture enum
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package prosepolicy

// // _badPosture — "classified" is not a valid #Posture
// _badPosture: #Posture & "classified"

// // _badRuleEmptyTerm — empty string in Terms violates _nonEmpty
// _badRuleEmptyTerm: #PostureRule & {
// 	Threshold: "secret"
// 	Terms: [""]
// }

// // _badPolicyBadPosture — "public" is not a valid Posture
// _badPolicyBadPosture: #Policy & {
// 	Posture: "public"
// 	Rules:   []
// }
