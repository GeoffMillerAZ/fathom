// Package expresslane — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidDecision_BadLane:
//     lane: "medium" violates #Lane enum — must be "fast" or "full"
//
//   invalidDecision_BadReason:
//     reason: "unknown" violates #Reason enum
//
//   invalidTask_EmptyID:
//     id: "" violates string & !="" — id must not be empty
//
//   invalidConfig_ZeroBytes:
//     max_payload_bytes: 0 violates int & >=1 — must be at least 1
//
//   invalidTaskSpec_NegativeComplexity:
//     estimated_complexity: -1 violates int & >=0
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package expresslane

// // invalidDecision_BadLane — "medium" is not a valid #Lane
// invalidDecision_BadLane: #Decision & {
// 	task_id: "task-bad"
// 	lane:    "medium"
// 	reason:  "eligible"
// }

// // invalidDecision_BadReason — "unknown" is not a valid #Reason
// invalidDecision_BadReason: #Decision & {
// 	task_id: "task-bad"
// 	lane:    "fast"
// 	reason:  "unknown"
// }

// // invalidTask_EmptyID — task id must not be empty
// invalidTask_EmptyID: #Task & {
// 	id:      ""
// 	payload: ""
// 	spec: {
// 		description:           "test"
// 		estimated_complexity:  1
// 		required_capabilities: []
// 	}
// }

// // invalidConfig_ZeroBytes — max_payload_bytes must be >= 1
// invalidConfig_ZeroBytes: #Config & {
// 	max_payload_bytes: 0
// }

// // invalidTaskSpec_NegativeComplexity — estimated_complexity must be >= 0
// invalidTaskSpec_NegativeComplexity: #TaskSpec & {
// 	description:           "test"
// 	estimated_complexity:  -1
// 	required_capabilities: []
// }
