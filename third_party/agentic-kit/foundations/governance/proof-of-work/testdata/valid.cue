// Package proofofwork — valid test data that must unify with schema.cue.
//
// The MAC values below are placeholder hex digests for shape validation only;
// they are NOT real tags and no secret was used to produce them. Never put
// real key material or real MACs minted from a production secret in fixtures.
package proofofwork

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/proof-of-work:proofofwork"

// validChallenge demonstrates a well-formed signed challenge with 8-bit difficulty.
validChallenge: proofofwork.#Challenge & {
	Nonce:     "deadbeefcafe0123456789abcdef0123:1704067200000000000"
	Bits:      8
	ExpiresAt: "2024-01-01T00:05:00Z"
	MAC:       "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
}

// validMinimalChallenge demonstrates the lowest legal difficulty (Bits=1 —
// zero-difficulty challenges are invalid by construction).
validMinimalChallenge: proofofwork.#Challenge & {
	Nonce:     "abc123:1704067200000000000"
	Bits:      1
	ExpiresAt: "2024-01-01T00:01:00Z"
	MAC:       "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
}

// validHighDifficultyChallenge demonstrates a 20-bit challenge for high-value gates.
validHighDifficultyChallenge: proofofwork.#Challenge & {
	Nonce:     "f1a2b3c4d5e6f7a8:1704067200000000000"
	Bits:      20
	ExpiresAt: "2024-01-01T00:10:00Z"
	MAC:       "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"
}

// validIssuerConfig demonstrates a production issuer configuration.
// Note: no secret here — it is a constructor argument, never config data.
validIssuerConfig: proofofwork.#IssuerConfig & {
	MinBits:    16
	Bits:       18
	TTLSeconds: 300
	MaxSeen:    65536
}

// validIssuerConfigTestTuned demonstrates a fast, test-friendly configuration.
validIssuerConfigTestTuned: proofofwork.#IssuerConfig & {
	MinBits:    1
	Bits:       4
	TTLSeconds: 60
	MaxSeen:    128
}
