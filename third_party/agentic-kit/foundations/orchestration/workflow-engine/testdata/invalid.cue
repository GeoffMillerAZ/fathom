// Package workflowengine — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidDefinition_EmptyID:
//     id: "" violates string & !="" — id must not be empty
//
//   invalidDefinition_EmptyVersion:
//     version: "" violates string & !="" — version must not be empty
//
//   invalidDefinition_NoStates:
//     states: [] violates [_, ...#WorkflowState] — at least one state required
//
//   invalidTransition_EmptyFrom:
//     from: "" violates string & !="" — from must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package workflowengine

// // invalidDefinition_EmptyID — id must not be empty
// invalidDefinition_EmptyID: #WorkflowDefinition & {
// 	id:            ""
// 	version:       "1.0.0"
// 	initial_state: "start"
// 	states: [{id: "start", terminal: true, description: "start"}]
// 	transitions: []
// }

// // invalidDefinition_EmptyVersion — version must not be empty
// invalidDefinition_EmptyVersion: #WorkflowDefinition & {
// 	id:            "workflow-x"
// 	version:       ""
// 	initial_state: "start"
// 	states: [{id: "start", terminal: true, description: "start"}]
// 	transitions: []
// }

// // invalidTransition_EmptyFrom — from must not be empty
// invalidTransition_EmptyFrom: #WorkflowTransition & {
// 	from:    ""
// 	to:      "done"
// 	on:      "finish"
// 	guards:  []
// 	actions: []
// }
