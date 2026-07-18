// Package improvementflywheel — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidSignal_BadType:
//     type: "typo_fix" violates #PatternType enum
//
//   invalidSignal_EmptyKey:
//     key: "" violates string & !=""
//
//   invalidObservation_ZeroFrequency:
//     frequency: 0 violates int & >=1 — must be at least 1
//
//   invalidProposal_EmptyID:
//     id: "" violates string & !="" — id must not be empty
//
//   invalidProposal_ZeroFrequency:
//     frequency: 0 violates int & >=1
//
//   invalidChangeTarget_BadValue:
//     change_target: "wiki_page" violates #ChangeTarget enum
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package improvementflywheel

// // invalidSignal_BadType — "typo_fix" is not a valid #PatternType
// invalidSignal_BadType: #Signal & {
// 	type:        "typo_fix"
// 	key:         "revenue/q4"
// 	workflow_id: "wf-001"
// 	detail:      ""
// }

// // invalidSignal_EmptyKey — key must not be empty
// invalidSignal_EmptyKey: #Signal & {
// 	type:        "false_positive"
// 	key:         ""
// 	workflow_id: "wf-001"
// 	detail:      "some detail"
// }

// // invalidObservation_ZeroFrequency — frequency must be >= 1
// invalidObservation_ZeroFrequency: #PatternObservation & {
// 	type:         "missed_claim"
// 	key:          "claim/type-a"
// 	frequency:    0
// 	workflow_ids: []
// 	examples:     []
// }

// // invalidProposal_EmptyID — id must not be empty
// invalidProposal_EmptyID: #Proposal & {
// 	id:                      ""
// 	observation_key:         "false_positive/x"
// 	pattern_type:            "false_positive"
// 	change_target:           "catalog_entry"
// 	summary:                 "some proposal"
// 	frequency:               1
// 	supporting_workflow_ids: []
// }

// // invalidChangeTarget_BadValue — "wiki_page" is not a valid #ChangeTarget
// invalidChangeTarget_BadValue: #Proposal & {
// 	id:                      "prop-x"
// 	observation_key:         "false_positive/x"
// 	pattern_type:            "false_positive"
// 	change_target:           "wiki_page"
// 	summary:                 "some proposal"
// 	frequency:               1
// 	supporting_workflow_ids: []
// }
