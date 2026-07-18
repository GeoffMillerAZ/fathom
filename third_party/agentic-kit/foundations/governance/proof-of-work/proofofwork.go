package proofofwork

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// MinSecretLen is the minimum accepted HMAC secret length in bytes.
const MinSecretLen = 16

// secretLen is the length of secrets produced by NewSecret.
const secretLen = 32

// maxBits is the hard upper bound on difficulty (a SHA-256 digest has 256 bits).
const maxBits = 256

// NewSecret returns a fresh 32-byte cryptographically random HMAC secret.
// This is the one place the package touches crypto/rand; call it explicitly
// at deployment or startup time and pass the result to NewIssuer. Generating
// a fresh secret on every process start is acceptable: it merely invalidates
// challenges issued before the restart, and clients simply request new ones.
func NewSecret() ([]byte, error) {
	b := make([]byte, secretLen)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("proofofwork: generating secret: %w", err)
	}
	return b, nil
}

// Issuer mints HMAC-signed challenges and verifies returned solutions.
// Construct via NewIssuer; the zero value is not usable.
// All methods are safe for concurrent use.
type Issuer struct {
	secret []byte
	cfg    IssuerConfig
	now    func() time.Time
	seen   SeenSet
}

// NewIssuer constructs an Issuer with a bundled bounded SeenSet.
//
// The secret must be at least MinSecretLen bytes (use NewSecret to generate
// one); it is copied defensively. cfg must satisfy the constraints documented
// on IssuerConfig. now is the injected clock.
func NewIssuer(secret []byte, cfg IssuerConfig, now func() time.Time) (*Issuer, error) {
	if now == nil {
		return nil, errors.New("proofofwork: now clock is required")
	}
	return NewIssuerWithSeenSet(secret, cfg, now, NewSeenSet(cfg.MaxSeen, now))
}

// NewIssuerWithSeenSet is NewIssuer with a caller-supplied SeenSet (e.g. a
// shared or persistent store). The SeenSet must be concurrency-safe and must
// honour the fail-closed contract documented on the interface.
func NewIssuerWithSeenSet(secret []byte, cfg IssuerConfig, now func() time.Time, seen SeenSet) (*Issuer, error) {
	if len(secret) < MinSecretLen {
		return nil, fmt.Errorf("proofofwork: secret must be at least %d bytes, got %d", MinSecretLen, len(secret))
	}
	if cfg.MinBits < 1 {
		return nil, fmt.Errorf("proofofwork: MinBits must be >= 1, got %d", cfg.MinBits)
	}
	if cfg.Bits < cfg.MinBits {
		return nil, fmt.Errorf("proofofwork: Bits (%d) must be >= MinBits (%d)", cfg.Bits, cfg.MinBits)
	}
	if cfg.Bits > maxBits {
		return nil, fmt.Errorf("proofofwork: Bits must be <= %d, got %d", maxBits, cfg.Bits)
	}
	if cfg.TTL <= 0 {
		return nil, fmt.Errorf("proofofwork: TTL must be > 0, got %v", cfg.TTL)
	}
	if cfg.MaxSeen < 1 {
		return nil, fmt.Errorf("proofofwork: MaxSeen must be >= 1, got %d", cfg.MaxSeen)
	}
	if now == nil {
		return nil, errors.New("proofofwork: now clock is required")
	}
	if seen == nil {
		return nil, errors.New("proofofwork: seen set is required")
	}
	sec := make([]byte, len(secret))
	copy(sec, secret)
	return &Issuer{secret: sec, cfg: cfg, now: now, seen: seen}, nil
}

// Issue creates a new signed Challenge. The nonce is constructed from the
// caller-supplied randHex string and the issue timestamp so that callers can
// pass crypto/rand-derived hex in production and a fixed string in tests.
func (i *Issuer) Issue(randHex string) Challenge {
	t := i.now()
	nonce := fmt.Sprintf("%s:%d", randHex, t.UnixNano())
	expiresAt := t.Add(i.cfg.TTL)
	return Challenge{
		Nonce:     nonce,
		Bits:      i.cfg.Bits,
		ExpiresAt: expiresAt,
		MAC:       hex.EncodeToString(i.mac(nonce, i.cfg.Bits, expiresAt)),
	}
}

// Verify returns (true, "") iff ALL of the following hold, checked in order:
//
//  1. c.MAC is a well-formed hex HMAC-SHA256 tag that matches the recomputed
//     MAC over the canonical encoding of (Nonce, Bits, ExpiresAt) under this
//     Issuer's secret. The comparison is constant-time via hmac.Equal
//     (crypto/subtle). This rejects forged and tampered challenges.
//  2. c.Bits >= MinBits (difficulty floor; defense in depth).
//  3. now() is before c.ExpiresAt (not expired).
//  4. SHA-256(c.Nonce+":"+solution) has at least c.Bits leading zero bits.
//  5. c.Nonce has not been redeemed before (replay protection). The replay
//     check is last so that failed attempts never consume the nonce. If the
//     seen set is at capacity with nothing evictable, verification FAILS
//     CLOSED.
//
// On failure it returns (false, reason); reason is a human-readable
// explanation suitable for logging (not end-user display).
func (i *Issuer) Verify(c Challenge, solution string) (bool, string) {
	// 1. Authenticity — the challenge must be one we signed.
	got, err := hex.DecodeString(c.MAC)
	if err != nil || len(got) != sha256.Size {
		return false, "malformed challenge MAC"
	}
	want := i.mac(c.Nonce, c.Bits, c.ExpiresAt)
	if !hmac.Equal(got, want) { // constant-time compare (crypto/subtle)
		return false, "challenge MAC mismatch: not server-issued or tampered"
	}

	// 2. Difficulty floor — even a validly signed challenge (e.g. issued under
	// an older, weaker config sharing this secret) must meet the current floor.
	if c.Bits < i.cfg.MinBits {
		return false, fmt.Sprintf("challenge difficulty %d below configured minimum %d", c.Bits, i.cfg.MinBits)
	}

	// 3. Expiry — fail-closed.
	if !i.now().Before(c.ExpiresAt) {
		return false, "challenge expired"
	}

	// 4. Proof of work.
	if !MeetsDifficulty(c, solution) {
		return false, fmt.Sprintf("insufficient proof of work: need %d leading zero bits", c.Bits)
	}

	// 5. Replay protection — keyed on the nonce so a challenge can be redeemed
	// at most once, even with a different valid solution. Must be last so
	// invalid attempts never consume the nonce.
	switch i.seen.TryAdd(c.Nonce, c.ExpiresAt) {
	case AddReplay:
		return false, "replay: challenge already redeemed"
	case AddAtCapacity:
		return false, "replay set at capacity: failing closed"
	}
	return true, ""
}

// mac computes HMAC-SHA256 over the canonical encoding of the signed fields.
//
// Canonical encoding (unambiguous by construction — the nonce is
// length-prefixed and the integers are fixed-width):
//
//	uint64 big-endian: len(nonce)
//	bytes:             nonce
//	uint64 big-endian: bits (two's complement of int64)
//	uint64 big-endian: expiresAt.UnixNano()
//
// UnixNano is location-independent and survives JSON round-trips of
// time.Time (RFC 3339 with nanoseconds), so a challenge serialised to a
// client and returned verbatim re-verifies. (UnixNano is undefined outside
// roughly years 1678–2262; TTL-derived expiries are always in range.)
func (i *Issuer) mac(nonce string, bits int, expiresAt time.Time) []byte {
	buf := make([]byte, 0, 8+len(nonce)+8+8)
	var w [8]byte
	binary.BigEndian.PutUint64(w[:], uint64(len(nonce)))
	buf = append(buf, w[:]...)
	buf = append(buf, nonce...)
	binary.BigEndian.PutUint64(w[:], uint64(int64(bits)))
	buf = append(buf, w[:]...)
	binary.BigEndian.PutUint64(w[:], uint64(expiresAt.UnixNano()))
	buf = append(buf, w[:]...)

	h := hmac.New(sha256.New, i.secret)
	h.Write(buf)
	return h.Sum(nil)
}

// Solve performs a brute-force search for a solution string whose
// SHA-256(c.Nonce+":"+candidate) has at least c.Bits leading zero bits.
// It increments an integer counter until the condition is met.
//
// Solve needs no secret and is deterministic: given the same Challenge it
// always returns the same solution. It is safe to compile to WASM for
// client-side solving — the work burns the client's CPU, not the server's.
func Solve(c Challenge) string {
	for i := 0; ; i++ {
		candidate := strconv.Itoa(i)
		if MeetsDifficulty(c, candidate) {
			return candidate
		}
	}
}

// MeetsDifficulty reports whether SHA-256(c.Nonce+":"+solution) has at least
// c.Bits leading zero bits.
//
// This is the client-side difficulty check only — it is NOT verification.
// Authoritative verification (MAC authenticity, difficulty floor, expiry,
// replay) happens server-side via Issuer.Verify, because only the server
// holds the HMAC secret.
func MeetsDifficulty(c Challenge, solution string) bool {
	digest := sha256.Sum256([]byte(c.Nonce + ":" + solution))
	return hasLeadingZeroBits(digest[:], c.Bits)
}

// hasLeadingZeroBits reports whether the first bits bits of b are all zero.
func hasLeadingZeroBits(b []byte, bits int) bool {
	if bits <= 0 {
		return true
	}
	// Check full zero bytes.
	fullBytes := bits / 8
	remBits := bits % 8
	for i := 0; i < fullBytes; i++ {
		if i >= len(b) || b[i] != 0 {
			return false
		}
	}
	// Check remaining bits in the next byte.
	if remBits > 0 {
		if fullBytes >= len(b) {
			return false
		}
		mask := byte(0xff << (8 - remBits))
		if b[fullBytes]&mask != 0 {
			return false
		}
	}
	return true
}

// ---- Bounded in-memory SeenSet ----------------------------------------------

// seenSet is a concurrency-safe, bounded, in-memory SeenSet. Each entry
// carries the expiry of its challenge; expired entries are evicted lazily.
// Memory is hard-bounded by max entries.
type seenSet struct {
	mu      sync.Mutex
	now     func() time.Time
	max     int
	entries map[string]time.Time // key → challenge expiry
}

// NewSeenSet returns a concurrency-safe, bounded, in-memory SeenSet.
//
// Eviction policy (never weakens security):
//   - An entry is evictable once its expiresAt has passed — the corresponding
//     challenge is rejected by Verify's expiry check regardless, so dropping
//     the entry cannot enable a replay.
//   - When the set holds maxEntries entries, TryAdd first sweeps expired
//     entries. If the set is still full — i.e. every entry is unexpired —
//     TryAdd returns AddAtCapacity and the caller FAILS CLOSED. Unexpired
//     entries are never evicted.
//
// maxEntries < 1 is clamped to 1 so the bound always holds; size maxEntries
// to at least the expected number of verifications per challenge TTL window.
func NewSeenSet(maxEntries int, now func() time.Time) SeenSet {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &seenSet{
		now:     now,
		max:     maxEntries,
		entries: make(map[string]time.Time),
	}
}

// TryAdd records key as seen until expiresAt. See SeenSet for the contract.
func (s *seenSet) TryAdd(key string, expiresAt time.Time) AddResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.now()
	if exp, ok := s.entries[key]; ok {
		if t.Before(exp) {
			return AddReplay
		}
		// The stored entry's challenge has expired; the slot is reusable.
		delete(s.entries, key)
	}
	if len(s.entries) >= s.max {
		// TTL eviction: drop entries whose challenges have expired. The O(n)
		// sweep runs only at the cap, so it is amortised across the adds that
		// filled the set.
		for k, exp := range s.entries {
			if !t.Before(exp) {
				delete(s.entries, k)
			}
		}
		if len(s.entries) >= s.max {
			// Every entry is still live — refuse to evict, fail closed.
			return AddAtCapacity
		}
	}
	s.entries[key] = expiresAt
	return AddOK
}
