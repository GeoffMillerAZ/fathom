// Package circuitbreaker — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidZeroThreshold:
//     FailureThreshold: 0 violates >=1 — must be at least 1
//
//   invalidNegativeThreshold:
//     FailureThreshold: -3 violates >=1 — must be at least 1
//
//   invalidZeroCooldown:
//     OpenCooldown: 0 violates >0 — must be positive
//
//   invalidNegativeCooldown:
//     OpenCooldown: -1000000000 violates >0 — must be positive
//
//   invalidZeroProbes:
//     HalfOpenProbes: 0 violates >=1 — must be at least 1
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package circuitbreaker

// // invalidZeroThreshold — FailureThreshold must be >= 1
// invalidZeroThreshold: #Config & {
// 	FailureThreshold: 0
// 	OpenCooldown:     5_000_000_000
// 	HalfOpenProbes:   2
// }

// // invalidNegativeThreshold — FailureThreshold must be >= 1
// invalidNegativeThreshold: #Config & {
// 	FailureThreshold: -3
// 	OpenCooldown:     5_000_000_000
// 	HalfOpenProbes:   2
// }

// // invalidZeroCooldown — OpenCooldown must be > 0
// invalidZeroCooldown: #Config & {
// 	FailureThreshold: 5
// 	OpenCooldown:     0
// 	HalfOpenProbes:   2
// }

// // invalidNegativeCooldown — OpenCooldown must be > 0
// invalidNegativeCooldown: #Config & {
// 	FailureThreshold: 5
// 	OpenCooldown:     -1_000_000_000
// 	HalfOpenProbes:   2
// }

// // invalidZeroProbes — HalfOpenProbes must be >= 1
// invalidZeroProbes: #Config & {
// 	FailureThreshold: 5
// 	OpenCooldown:     5_000_000_000
// 	HalfOpenProbes:   0
// }
