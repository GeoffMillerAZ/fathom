// Package budgetguard — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidZeroReqPerMin:
//     MaxReqPerMin: 0 violates >=1 — must be at least 1
//
//   invalidNegativeReqPerDay:
//     MaxReqPerDay: -1 violates >=1 — must be at least 1
//
//   invalidZeroUSD:
//     MaxUSDPerDay: 0 violates >0 — must be strictly positive
//
//   invalidNegativeUSD:
//     MaxUSDPerDay: -5.0 violates >0 — must be strictly positive
//
//   invalidFractionalReqPerMin:
//     MaxReqPerMin: 2.5 violates int — must be an integer
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package budgetguard

// // invalidZeroReqPerMin — MaxReqPerMin must be >= 1
// invalidZeroReqPerMin: #Config & {
// 	MaxReqPerMin: 0
// 	MaxReqPerDay: 100
// 	MaxUSDPerDay: 10.0
// }

// // invalidNegativeReqPerDay — MaxReqPerDay must be >= 1
// invalidNegativeReqPerDay: #Config & {
// 	MaxReqPerMin: 10
// 	MaxReqPerDay: -1
// 	MaxUSDPerDay: 10.0
// }

// // invalidZeroUSD — MaxUSDPerDay must be > 0
// invalidZeroUSD: #Config & {
// 	MaxReqPerMin: 10
// 	MaxReqPerDay: 100
// 	MaxUSDPerDay: 0
// }

// // invalidNegativeUSD — MaxUSDPerDay must be > 0
// invalidNegativeUSD: #Config & {
// 	MaxReqPerMin: 10
// 	MaxReqPerDay: 100
// 	MaxUSDPerDay: -5.0
// }

// // invalidFractionalReqPerMin — MaxReqPerMin must be an integer
// invalidFractionalReqPerMin: #Config & {
// 	MaxReqPerMin: 2.5
// 	MaxReqPerDay: 100
// 	MaxUSDPerDay: 10.0
// }
