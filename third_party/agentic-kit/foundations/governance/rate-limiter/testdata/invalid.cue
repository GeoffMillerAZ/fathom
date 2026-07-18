// Package ratelimiter — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidZeroRate:
//     RatePerSec: 0 violates >0 — rate must be strictly positive
//
//   invalidNegativeRate:
//     RatePerSec: -1 violates >0 — rate must be strictly positive
//
//   invalidZeroBurst:
//     Burst: 0 violates >=1 — burst must be at least 1
//
//   invalidNegativeBurst:
//     Burst: -5 violates >=1 — burst must be at least 1
//
//   invalidFractionalBurst:
//     Burst: 2.5 violates int — burst must be an integer
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package ratelimiter

// // invalidZeroRate — RatePerSec must be > 0
// invalidZeroRate: #Config & {
// 	RatePerSec: 0
// 	Burst:      10
// }

// // invalidNegativeRate — RatePerSec must be > 0
// invalidNegativeRate: #Config & {
// 	RatePerSec: -1
// 	Burst:      10
// }

// // invalidZeroBurst — Burst must be >= 1
// invalidZeroBurst: #Config & {
// 	RatePerSec: 1.0
// 	Burst:      0
// }

// // invalidNegativeBurst — Burst must be >= 1
// invalidNegativeBurst: #Config & {
// 	RatePerSec: 1.0
// 	Burst:      -5
// }

// // invalidFractionalBurst — Burst must be an integer
// invalidFractionalBurst: #Config & {
// 	RatePerSec: 1.0
// 	Burst:      2.5
// }
