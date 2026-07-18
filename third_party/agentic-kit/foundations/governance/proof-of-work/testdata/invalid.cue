// Package proofofwork — invalid test data showing what the schema rejects.
// This file is NOT vetted with `cue vet` (it is intentionally invalid).
// It is kept here as documentation of the constraints enforced by schema.cue.
//
// The examples below would fail `cue vet` with the following errors:
//
//   invalidChallenge_EmptyNonce:
//     Nonce: "" violates string & !="" — Nonce must not be empty
//
//   invalidChallenge_ZeroBits:
//     Bits: 0 violates >=1 — issued challenges always carry at least 1 bit;
//     a zero-difficulty challenge was the original forgery vector and is
//     invalid by construction
//
//   invalidChallenge_NegativeBits:
//     Bits: -1 violates >=1 — Bits must be positive
//
//   invalidChallenge_EmptyExpiresAt:
//     ExpiresAt: "" violates string & !="" — ExpiresAt must not be empty
//
//   invalidChallenge_MissingMAC:
//     MAC absent — every challenge must carry the issuer's HMAC tag
//
//   invalidChallenge_MalformedMAC:
//     MAC: "not-hex" violates =~"^[0-9a-f]{64}$" — MAC must be 64 lowercase
//     hex chars (a 32-byte HMAC-SHA256 tag)
//
//   invalidIssuerConfig_ZeroMinBits:
//     MinBits: 0 violates >=1 — the verification floor must be positive
//
//   invalidIssuerConfig_BitsBelowFloor:
//     Bits: 4 with MinBits: 8 violates >=MinBits — issued difficulty cannot
//     undercut the verification floor
//
//   invalidIssuerConfig_ZeroMaxSeen:
//     MaxSeen: 0 violates >=1 — the replay set must hold at least one entry
//
//   invalidIssuerConfig_ZeroTTL:
//     TTLSeconds: 0 violates >=1 — challenges must have a finite, positive TTL
//
// These are intentionally NOT unified with the schema definitions above.
// To verify the schema rejects them, temporarily uncomment each block and run:
//   cue vet ./...   (expect errors)

package proofofwork

// // invalidChallenge_EmptyNonce — Nonce must not be empty
// invalidChallenge_EmptyNonce: #Challenge & {
// 	Nonce:     ""
// 	Bits:      8
// 	ExpiresAt: "2024-01-01T00:05:00Z"
// 	MAC:       "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
// }

// // invalidChallenge_ZeroBits — Bits must be >= 1 (forgery vector)
// invalidChallenge_ZeroBits: #Challenge & {
// 	Nonce:     "abc:123"
// 	Bits:      0
// 	ExpiresAt: "2024-01-01T00:05:00Z"
// 	MAC:       "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
// }

// // invalidChallenge_NegativeBits — Bits must be >= 1
// invalidChallenge_NegativeBits: #Challenge & {
// 	Nonce:     "abc:123"
// 	Bits:      -1
// 	ExpiresAt: "2024-01-01T00:05:00Z"
// 	MAC:       "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
// }

// // invalidChallenge_EmptyExpiresAt — ExpiresAt must not be empty
// invalidChallenge_EmptyExpiresAt: #Challenge & {
// 	Nonce:     "abc:123"
// 	Bits:      8
// 	ExpiresAt: ""
// 	MAC:       "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
// }

// // invalidChallenge_MissingMAC — MAC is required
// invalidChallenge_MissingMAC: #Challenge & {
// 	Nonce:     "abc:123"
// 	Bits:      8
// 	ExpiresAt: "2024-01-01T00:05:00Z"
// }

// // invalidChallenge_MalformedMAC — MAC must be 64 lowercase hex chars
// invalidChallenge_MalformedMAC: #Challenge & {
// 	Nonce:     "abc:123"
// 	Bits:      8
// 	ExpiresAt: "2024-01-01T00:05:00Z"
// 	MAC:       "not-hex"
// }

// // invalidIssuerConfig_ZeroMinBits — MinBits must be >= 1
// invalidIssuerConfig_ZeroMinBits: #IssuerConfig & {
// 	MinBits:    0
// 	Bits:       8
// 	TTLSeconds: 300
// 	MaxSeen:    1024
// }

// // invalidIssuerConfig_BitsBelowFloor — Bits must be >= MinBits
// invalidIssuerConfig_BitsBelowFloor: #IssuerConfig & {
// 	MinBits:    8
// 	Bits:       4
// 	TTLSeconds: 300
// 	MaxSeen:    1024
// }

// // invalidIssuerConfig_ZeroMaxSeen — MaxSeen must be >= 1
// invalidIssuerConfig_ZeroMaxSeen: #IssuerConfig & {
// 	MinBits:    16
// 	Bits:       18
// 	TTLSeconds: 300
// 	MaxSeen:    0
// }

// // invalidIssuerConfig_ZeroTTL — TTLSeconds must be >= 1
// invalidIssuerConfig_ZeroTTL: #IssuerConfig & {
// 	MinBits:    16
// 	Bits:       18
// 	TTLSeconds: 0
// 	MaxSeen:    1024
// }
