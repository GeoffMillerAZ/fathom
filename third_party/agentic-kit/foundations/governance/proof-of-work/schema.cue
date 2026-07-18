// Package proofofwork defines the schema for hashcash-style proof-of-work
// challenges with HMAC-signed integrity. Constraints mirror the Go types and
// are evaluated with `cue vet ./...`.
//
// SECURITY: the HMAC secret is deliberately absent from this schema. It is a
// constructor argument in Go (proofofwork.NewIssuer), not config data, so
// that config files and CUE fixtures never carry key material. CUE examples
// must never contain a real secret value.
package proofofwork

// #Challenge is issued by the server, solved by the client, and returned
// verbatim. Clients treat it as opaque.
#Challenge: {
	// Nonce is the server-generated unique string. Must not be empty.
	Nonce: string & !=""

	// Bits is the required number of leading zero bits in the SHA-256 digest.
	// Issued challenges always satisfy the issuer's MinBits floor, which is at
	// least 1, so zero-difficulty challenges are invalid by construction.
	Bits: >=1 & int

	// ExpiresAt is the RFC 3339 UTC deadline for the challenge.
	ExpiresAt: string & !=""

	// MAC is the hex-encoded HMAC-SHA256 tag (32 bytes → 64 hex chars) over
	// the canonical encoding of (Nonce, Bits, ExpiresAt) under the issuer's
	// secret. Verification rejects any challenge whose MAC does not match.
	MAC: string & =~"^[0-9a-f]{64}$"
}

// #IssuerConfig mirrors proofofwork.IssuerConfig (data-shaped fields only;
// the secret is passed separately to the constructor).
#IssuerConfig: {
	// MinBits is the verification difficulty floor. Challenges below it are
	// rejected even with a valid MAC. Must be >= 1.
	MinBits: >=1 & int

	// Bits is the difficulty of issued challenges. Must be >= MinBits and
	// <= 256 (a SHA-256 digest has 256 bits).
	Bits: >=MinBits & <=256 & int

	// TTLSeconds is the challenge validity period in seconds. Must be >= 1.
	TTLSeconds: >=1 & int

	// MaxSeen is the hard cap on replay seen-set entries. When the cap is
	// reached with no expired entries to evict, verification fails closed.
	// Must be >= 1.
	MaxSeen: >=1 & int
}
