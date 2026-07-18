// Package correctionledger — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidCorrection_BadConfidence:
//     Confidence: 7 violates >=1 & <=5 — confidence must be 1–5
//
//   invalidCorrection_EmptyOriginal:
//     Original: "" violates string & !="" — original must not be empty
//
//   invalidDismissal_EmptyReason:
//     Reason: "" violates string & !="" — reason must not be empty
//
//   invalidFinding_BadState:
//     State: "unknown" violates #State enum — must be one of the five valid states
//
//   invalidCorrectionTarget_EmptyStableTarget:
//     StableTarget: "" violates string & !="" — stable target must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package correctionledger

// // invalidCorrection_BadConfidence — Confidence 7 is out of range [1,5]
// invalidCorrection_BadConfidence: #Correction & {
// 	FindingID:   "finding-bad-confidence"
// 	Target:      { StableTarget: "doc/section/1.v1" }
// 	Original:    "old value"
// 	Corrected:   "new value"
// 	CorrectedBy: "alice@example.com"
// 	CorrectedAt: "2024-11-01T10:00:00Z"
// 	Reason:      "was wrong"
// 	Confidence:  7
// }

// // invalidCorrection_EmptyOriginal — Original must not be empty
// invalidCorrection_EmptyOriginal: #Correction & {
// 	FindingID:   "finding-empty-original"
// 	Target:      { StableTarget: "doc/section/1.v1" }
// 	Original:    ""
// 	Corrected:   "new value"
// 	CorrectedBy: "alice@example.com"
// 	CorrectedAt: "2024-11-01T10:00:00Z"
// 	Reason:      "was wrong"
// 	Confidence:  3
// }

// // invalidDismissal_EmptyReason — Reason must not be empty
// invalidDismissal_EmptyReason: #Dismissal & {
// 	FindingID:   "finding-empty-reason"
// 	DismissedBy: "bob@example.com"
// 	Reason:      ""
// 	DismissedAt: "2024-11-01T10:00:00Z"
// }

// // invalidFinding_BadState — "unknown" is not a valid #State
// invalidFinding_BadState: #Finding & {
// 	ID:          "finding-bad-state"
// 	State:       "unknown"
// 	Description: "test"
// 	CreatedAt:   "2024-11-01T09:00:00Z"
// 	UpdatedAt:   "2024-11-01T09:00:00Z"
// }

// // invalidCorrectionTarget_EmptyStableTarget — StableTarget must not be empty
// invalidCorrectionTarget_EmptyStableTarget: #CorrectionTarget & {
// 	StableTarget: ""
// }

// // invalidAuthor_EmptyIdentityID — IdentityID must not be empty
// invalidAuthor_EmptyIdentityID: #Author & {
// 	Kind:       "human"
// 	IdentityID: ""
// }

// // invalidAuthor_BadKind — Kind must be "human" or "ai"
// invalidAuthor_BadKind: #Author & {
// 	Kind:       "system"
// 	IdentityID: "someone"
// }

// // invalidDismissal_EmptyDismissedBy — DismissedBy must not be empty
// invalidDismissal_EmptyDismissedBy: #Dismissal & {
// 	FindingID:   "finding-empty-dismissedby"
// 	DismissedBy: ""
// 	Reason:      "false positive"
// 	DismissedAt: "2024-11-01T10:00:00Z"
// }

// // invalidCorrection_EmptyFindingID — FindingID must not be empty
// invalidCorrection_EmptyFindingID: #Correction & {
// 	FindingID:   ""
// 	Target:      { StableTarget: "doc/section/1.v1" }
// 	Original:    "old value"
// 	Corrected:   "new value"
// 	CorrectedBy: "alice@example.com"
// 	CorrectedAt: "2024-11-01T10:00:00Z"
// 	Reason:      "was wrong"
// 	Confidence:  3
// }
