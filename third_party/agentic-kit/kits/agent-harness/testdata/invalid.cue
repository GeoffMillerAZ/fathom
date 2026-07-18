// Package agentharness — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by
// schema.cue; the matching Go rejection tests live in TestNewValidation.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidZeroMaxTurns:
//     MaxTurns: 0 violates >=1 — at least one model call is required
//
//   invalidNegativeMaxTokens:
//     MaxTokensPerCall: -1 violates >=0 — token caps cannot be negative
//
//   invalidAction:
//     InjectionAction: "quarantine" is not in "off" | "annotate" | "block"
//
//   invalidSeverity:
//     InjectionMinSeverity: "extreme" is not a #Severity
//
//   invalidNegativeUSD:
//     EstimatedUSDPerCall: -0.01 violates >=0 — estimates cannot be negative
//
//   invalidFractionalTurns:
//     MaxTurns: 2.5 violates int — turn counts are integers
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package agentharness

// // invalidZeroMaxTurns — MaxTurns must be >= 1
// invalidZeroMaxTurns: #Config & {
// 	MaxTurns: 0
// }

// // invalidNegativeMaxTokens — MaxTokensPerCall must be >= 0
// invalidNegativeMaxTokens: #Config & {
// 	MaxTurns:         4
// 	MaxTokensPerCall: -1
// }

// // invalidAction — InjectionAction must be off | annotate | block
// invalidAction: #Config & {
// 	MaxTurns:        4
// 	InjectionAction: "quarantine"
// }

// // invalidSeverity — InjectionMinSeverity must be a #Severity
// invalidSeverity: #Config & {
// 	MaxTurns:             4
// 	InjectionMinSeverity: "extreme"
// }

// // invalidNegativeUSD — EstimatedUSDPerCall must be >= 0
// invalidNegativeUSD: #Config & {
// 	MaxTurns:            4
// 	EstimatedUSDPerCall: -0.01
// }

// // invalidFractionalTurns — MaxTurns must be an integer
// invalidFractionalTurns: #Config & {
// 	MaxTurns: 2.5
// }
