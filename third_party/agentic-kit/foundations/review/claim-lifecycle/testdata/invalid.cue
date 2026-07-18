// Package claimlifecycle — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
// The negative constraints on #EvidenceTier and #Grade are now enforced by the
// evidencegrade package directly (imported in schema.cue); any upstream change
// is caught at `cue vet` time automatically.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidClaim_BadState:
//     State: "unknown" violates #ClaimState enum —
//     must be proposed|supported|refuted|uncertain|superseded|recanted
//
//   invalidClaim_EmptyID:
//     ID: "" violates string & !="" — ID must not be empty
//
//   invalidClaim_EmptyText:
//     Text: "" violates string & !="" — Text must not be empty
//
//   invalidClaim_BadEventKind:
//     ClaimEventKind: "approve" violates #ClaimEventKind enum
//
//   invalidClaimView_BadGradeTier:
//     Grade.Grade.Tier: "hearsay" violates evidencegrade.#Tier enum —
//     tier shapes are imported from evidencegrade (not copied inline), so
//     any upstream tier change is caught here automatically.
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package claimlifecycle

// // invalidClaim_BadState — "unknown" is not a valid #ClaimState
// invalidClaim_BadState: #Claim & {
// 	ID:        "claim-bad-state"
// 	Text:      "some claim"
// 	State:     "unknown"
// 	CreatedAt: "2024-11-01T09:00:00Z"
// 	UpdatedAt: "2024-11-01T09:00:00Z"
// }

// // invalidClaim_EmptyID — ID must not be empty
// invalidClaim_EmptyID: #Claim & {
// 	ID:        ""
// 	Text:      "some claim"
// 	State:     "proposed"
// 	CreatedAt: "2024-11-01T09:00:00Z"
// 	UpdatedAt: "2024-11-01T09:00:00Z"
// }

// // invalidClaim_EmptyText — Text must not be empty
// invalidClaim_EmptyText: #Claim & {
// 	ID:        "claim-empty-text"
// 	Text:      ""
// 	State:     "proposed"
// 	CreatedAt: "2024-11-01T09:00:00Z"
// 	UpdatedAt: "2024-11-01T09:00:00Z"
// }

// // invalidClaim_BadEventKind — "approve" is not a valid #ClaimEventKind
// invalidClaim_BadEventKind: #ClaimEventKind & "approve"

// // invalidClaimView_BadGradeTier — "hearsay" violates evidencegrade.#Tier.
// // Tier shapes are imported from evidencegrade (not inline copies), so any
// // upstream tier change is caught here automatically.
// invalidClaimView_BadGradeTier: #ClaimView & {
// 	ClaimID: "claim-bad-grade"
// 	Text:    "some claim"
// 	State:   "supported"
// 	Grade: {
// 		Grade: {
// 			Tier:       "hearsay"
// 			Confidence: 0.7
// 		}
// 		Promoted:                false
// 		PromotionReason:         ""
// 		FloorTier:               "hearsay"
// 		DroppedSpanIDs:          []
// 		AIContributedConfidence: 0.0
// 	}
// 	LinkedAssertions: []
// 	LinkedEvidence:   []
// 	History:          ["proposed", "supported"]
// }
