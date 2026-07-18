// Package honestygate — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidClaim_EmptyText:
//     Text: "" violates string & !="" — Text must not be empty
//
//   invalidClaim_NegativeEvidence:
//     EvidenceCount: -1 violates int & >=0 — EvidenceCount must be >= 0
//
//   invalidClaim_ConfidenceAboveOne:
//     Confidence: 1.1 violates number & >=0 & <=1 — Confidence must be in [0,1]
//
//   invalidDecision_BadVerdict:
//     Verdict: "unknown" violates #Verdict enum
//
//   invalidDecision_EmptyReason:
//     Reason: "" violates string & !="" — Reason must not be empty
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package honestygate

// // invalidClaim_EmptyText — Text must not be empty
// invalidClaim_EmptyText: #Claim & {
// 	Text:          ""
// 	EvidenceCount: 1
// 	Confidence:    0.5
// }

// // invalidClaim_NegativeEvidence — EvidenceCount must be >= 0
// invalidClaim_NegativeEvidence: #Claim & {
// 	Text:          "some claim"
// 	EvidenceCount: -1
// 	Confidence:    0.5
// }

// // invalidClaim_ConfidenceAboveOne — Confidence must be in [0,1]
// invalidClaim_ConfidenceAboveOne: #Claim & {
// 	Text:          "some claim"
// 	EvidenceCount: 2
// 	Confidence:    1.1
// }

// // invalidDecision_BadVerdict — "unknown" is not a valid #Verdict
// invalidDecision_BadVerdict: #Decision & {
// 	Verdict: "unknown"
// 	Reason:  "some reason"
// }

// // invalidDecision_EmptyReason — Reason must not be empty
// invalidDecision_EmptyReason: #Decision & {
// 	Verdict: "allowed"
// 	Reason:  ""
// }
