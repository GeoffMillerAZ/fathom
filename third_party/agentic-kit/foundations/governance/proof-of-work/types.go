// Package proofofwork provides a hashcash-style proof-of-work gate with
// HMAC-signed challenges for zero-infrastructure anti-bot protection.
//
// # Threat model and design
//
//   - Challenges are minted only by the server-side Issuer, which signs the
//     canonical encoding of (Nonce, Bits, ExpiresAt) with HMAC-SHA256 under a
//     secret key. Verify recomputes the MAC and compares it in constant time
//     (crypto/hmac, which is built on crypto/subtle), so a client cannot mint
//     its own challenge or tamper with one (e.g. setting Bits=0 or pushing
//     ExpiresAt into the far future). The canonical encoding length-prefixes
//     the nonce and uses fixed-width big-endian integers, so distinct field
//     tuples can never produce the same signed bytes.
//   - Verify additionally enforces a configured minimum difficulty floor
//     (MinBits >= 1) as defense in depth, rejects expired challenges, and
//     rejects replays by recording each redeemed challenge nonce in a SeenSet
//     until that challenge's own expiry.
//   - The bundled SeenSet is bounded: entries are evictable once their
//     challenge has expired (an expired challenge is rejected by Verify
//     anyway), and a hard MaxSeen cap applies. If the cap is reached and no
//     expired entry can be evicted, new verifications FAIL CLOSED — they are
//     rejected rather than weakening replay protection.
//   - The challenge is opaque to clients: they receive it, brute-force a
//     solution with Solve, and return both unchanged. Solving requires no
//     secret; verification does.
//
// # Design constraints
//
//   - stdlib only; no file I/O; no time.Now() in library paths — the clock is
//     injected via func() time.Time. NewSecret is the single deliberate use
//     of crypto/rand (a key-generation helper called explicitly by operators).
//   - Deterministic given the same inputs; testable without mocking.
//
// # Concurrency
//
// An Issuer is safe for concurrent use by multiple goroutines. Issue touches
// no shared mutable state. Verify serialises replay-set access through the
// SeenSet; the bundled implementation guards every TryAdd with a mutex.
// Custom SeenSet implementations must themselves be concurrency-safe.
package proofofwork

import "time"

// Challenge is issued by the server, solved by the client, and returned to
// the server verbatim. Clients must treat it as opaque.
type Challenge struct {
	// Nonce is the server-generated unique string the client must prove work on.
	Nonce string

	// Bits is the required number of leading zero bits in
	// SHA-256(Nonce+":"+solution).
	Bits int

	// ExpiresAt is the UTC deadline after which the challenge must be rejected.
	ExpiresAt time.Time

	// MAC is the hex-encoded HMAC-SHA256 tag over the canonical encoding of
	// (Nonce, Bits, ExpiresAt) under the Issuer's secret. Clients return it
	// unchanged; Verify rejects any challenge whose MAC does not match, which
	// makes self-minted or tampered challenges worthless.
	MAC string
}

// IssuerConfig holds the data-shaped configuration for an Issuer. The HMAC
// secret is deliberately NOT part of this struct — it is passed separately to
// NewIssuer so that config files, CUE fixtures, and snapshots never carry key
// material.
type IssuerConfig struct {
	// Bits is the difficulty of issued challenges (leading zero bits).
	// Must be >= MinBits and <= 256.
	Bits int

	// MinBits is the verification floor: Verify rejects any challenge whose
	// Bits is below this value, even when its MAC is valid. Must be >= 1.
	MinBits int

	// TTL is how long an issued challenge remains valid. Must be > 0.
	TTL time.Duration

	// MaxSeen is the hard cap on replay seen-set entries. When the cap is
	// reached and no expired entries can be evicted, verification fails
	// closed. Must be >= 1.
	MaxSeen int
}

// AddResult is the outcome of SeenSet.TryAdd.
type AddResult int

const (
	// AddOK means the key was recorded; it had not been seen before.
	AddOK AddResult = iota

	// AddReplay means the key was already present and unexpired.
	AddReplay

	// AddAtCapacity means the set is at its hard cap and holds no expired
	// entries to evict. Callers must fail closed (treat as rejection).
	AddAtCapacity
)

// String implements fmt.Stringer.
func (r AddResult) String() string {
	switch r {
	case AddOK:
		return "AddOK"
	case AddReplay:
		return "AddReplay"
	case AddAtCapacity:
		return "AddAtCapacity"
	default:
		return "Unknown"
	}
}

// SeenSet is a replay-protection store. Once a challenge nonce has been
// redeemed it must be remembered until the challenge can no longer be valid
// (i.e. until expiresAt); after that the entry may be evicted, because Verify
// independently rejects expired challenges. Implementations must be
// concurrency-safe and must return AddAtCapacity — never silently evict an
// unexpired entry — when they cannot record a new key.
type SeenSet interface {
	// TryAdd records key as seen until expiresAt and reports the outcome.
	TryAdd(key string, expiresAt time.Time) AddResult
}
