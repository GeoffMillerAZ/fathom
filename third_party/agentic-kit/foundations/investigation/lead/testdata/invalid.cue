// Package lead — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
// Per kit convention, all examples are commented out.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidLead_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidLead_EmptyPrompt:
//     Prompt: "" violates string & !="" — Prompt must not be empty
//
//   invalidLead_BadStatus:
//     Status: "hunch" violates #LeadStatus enum —
//     must be open|pursuing|closed|abandoned
//
//   invalidLead_BadAuthorKind:
//     Author.Kind: "robot" violates the #Author disjunction —
//     must be "human" or "ai"
//
//   invalidLead_AIAuthorMissingModelID:
//     Author.AI.ModelID: "" violates _nonEmpty constraint in ai-provenance
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package lead

// // invalidLead_EmptyID — ID must not be empty
// invalidLead_EmptyID: #Lead & {
// 	ID:             ""
// 	Prompt:         "Some valid prompt"
// 	TriggerSpanIDs: []
// 	Status:         "open"
// 	Author: {Kind: "human", Human: {ID: "user-1"}}
// 	CreatedAt:     "2026-06-08T12:00:00Z"
// 	UpdatedAt:     "2026-06-08T12:00:00Z"
// 	ResultSpanIDs: []
// }

// // invalidLead_EmptyPrompt — Prompt must not be empty
// invalidLead_EmptyPrompt: #Lead & {
// 	ID:             "lead-bad"
// 	Prompt:         ""
// 	TriggerSpanIDs: []
// 	Status:         "open"
// 	Author: {Kind: "human", Human: {ID: "user-1"}}
// 	CreatedAt:     "2026-06-08T12:00:00Z"
// 	UpdatedAt:     "2026-06-08T12:00:00Z"
// 	ResultSpanIDs: []
// }

// // invalidLead_BadStatus — "hunch" is not a valid #LeadStatus
// invalidLead_BadStatus: #Lead & {
// 	ID:             "lead-bad-status"
// 	Prompt:         "Some valid prompt"
// 	TriggerSpanIDs: []
// 	Status:         "hunch"
// 	Author: {Kind: "human", Human: {ID: "user-1"}}
// 	CreatedAt:     "2026-06-08T12:00:00Z"
// 	UpdatedAt:     "2026-06-08T12:00:00Z"
// 	ResultSpanIDs: []
// }

// // invalidLead_BadAuthorKind — "robot" violates the #Author disjunction
// invalidLead_BadAuthorKind: #Lead & {
// 	ID:             "lead-bad-author"
// 	Prompt:         "Some valid prompt"
// 	TriggerSpanIDs: []
// 	Status:         "open"
// 	Author: {Kind: "robot", Human: {ID: "r2d2"}}
// 	CreatedAt:     "2026-06-08T12:00:00Z"
// 	UpdatedAt:     "2026-06-08T12:00:00Z"
// 	ResultSpanIDs: []
// }

// // invalidLead_AIAuthorMissingModelID — ModelID must not be empty
// invalidLead_AIAuthorMissingModelID: #Lead & {
// 	ID:             "lead-bad-ai"
// 	Prompt:         "AI-generated lead with incomplete provenance"
// 	TriggerSpanIDs: []
// 	Status:         "open"
// 	Author: {
// 		Kind: "ai"
// 		AI: {
// 			ModelID:      ""
// 			ModelVersion: "20260608"
// 			PromptHash:   "sha256-ph1"
// 			InputDigest:  "sha256-id1"
// 			At:           "2026-06-08T12:00:00Z"
// 		}
// 	}
// 	CreatedAt:     "2026-06-08T12:00:00Z"
// 	UpdatedAt:     "2026-06-08T12:00:00Z"
// 	ResultSpanIDs: []
// }
