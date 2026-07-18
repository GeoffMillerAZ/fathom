package untrustedcontentwrapper

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// Wrapper produces Wrapped artifacts from a fixed config and an injected
// randomness source. It is immutable after construction and safe for
// concurrent use provided randHex is concurrency-safe (CryptoRandHex is).
type Wrapper struct {
	cfg     WrapConfig
	randHex func(nBytes int) string
}

// CryptoRandHex returns nBytes cryptographically random bytes hex-encoded
// (2*nBytes lowercase hex characters). It is the default randomness source
// and is safe for concurrent use. crypto/rand.Read never returns an error
// (Go >= 1.24); it crashes the program if the kernel entropy source is
// unavailable.
func CryptoRandHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewWrapper returns a Wrapper using cfg and randHex. Pass randHex == nil to
// use crypto/rand (CryptoRandHex); an injected randHex must return exactly
// 2*nBytes lowercase hex characters — Wrap rejects malformed nonces.
// Returns an error if cfg is invalid.
func NewWrapper(cfg WrapConfig, randHex func(nBytes int) string) (*Wrapper, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if randHex == nil {
		randHex = CryptoRandHex
	}
	return &Wrapper{cfg: cfg, randHex: randHex}, nil
}

// Wrap fences content between fresh nonce fences and returns the Wrapped
// artifact carrying the exact fences used. Content is never mutated: the
// Unwrap round trip is byte-exact. Each call draws a new nonce, so two wraps
// of the same content produce different fences.
//
// Wrap returns an error only if the injected randHex produces a malformed
// nonce (wrong length or non-lowercase-hex characters).
func (w *Wrapper) Wrap(content string) (Wrapped, error) {
	nBytes := w.cfg.NonceBytes
	if nBytes == 0 {
		nBytes = defaultNonceBytes
	}
	nonce := w.randHex(nBytes)
	if err := checkNonce(nonce, nBytes); err != nil {
		return Wrapped{}, err
	}

	openFence := "<<<" + w.cfg.OpenTag + ":" + nonce + ">>>"
	closeFence := "<<<" + w.cfg.CloseTag + ":" + nonce + ">>>"

	var b strings.Builder
	b.Grow(len(openFence) + len(closeFence) + len(content) + len(w.cfg.Label) + 8)
	b.WriteString(openFence)
	b.WriteByte('\n')
	if w.cfg.Label != "" {
		b.WriteByte('[')
		b.WriteString(w.cfg.Label)
		b.WriteString("]\n")
	}
	b.WriteString(content)
	b.WriteByte('\n')
	b.WriteString(closeFence)

	return Wrapped{
		Text:       b.String(),
		Nonce:      nonce,
		OpenFence:  openFence,
		CloseFence: closeFence,
		Label:      w.cfg.Label,
	}, nil
}

// checkNonce verifies that nonce is exactly 2*nBytes lowercase hex
// characters, guarding the fence grammar against a faulty injected randHex.
func checkNonce(nonce string, nBytes int) error {
	if len(nonce) != 2*nBytes {
		return fmt.Errorf("untrustedcontentwrapper: randHex returned %d characters, want %d", len(nonce), 2*nBytes)
	}
	for i := 0; i < len(nonce); i++ {
		c := nonce[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("untrustedcontentwrapper: randHex returned non-lowercase-hex character %q at index %d", c, i)
		}
	}
	return nil
}

// WrapWithNonce fences content using a caller-supplied nonce instead of
// drawing one from the injected randHex source. The nonce must satisfy the
// same grammar as a randomly-generated one: exactly 2*NonceBytes lowercase
// hex characters (where NonceBytes is the config value, defaulting to 16).
//
// Use WrapWithNonce when you need reproducible fences for the same content —
// for example, to produce a stable PromptHash over prompts that include
// wrapped untrusted content. A content-derived nonce (e.g. a truncated
// SHA-256 of the content) achieves this while preventing the content from
// containing the exact closing fence it will receive.
//
// # Security trade-off — read before use
//
// With a caller-supplied nonce, the caller owns unpredictability. A
// predictable nonce weakens the fence's injection-resistance guarantee:
//
//   - The fence works because the content is fixed BEFORE the nonce is
//     drawn, so an attacker who can inject text into the content cannot know
//     the nonce in advance and therefore cannot forge a closing fence.
//   - If the nonce is content-derived (e.g. SHA-256(content)), an attacker
//     who controls the content can compute the nonce, craft a forged closing
//     fence, inject it, and escape the fence.
//
// Only use WrapWithNonce when content comes from a fully-trusted source and
// reproducibility is more important than injection resistance (e.g. compiling
// a prompt for hashing purposes, not for sending to a model). For prompts
// sent to a model with untrusted content, use Wrap (random nonce) instead.
func (w *Wrapper) WrapWithNonce(content, nonce string) (Wrapped, error) {
	nBytes := w.cfg.NonceBytes
	if nBytes == 0 {
		nBytes = defaultNonceBytes
	}
	if err := checkNonce(nonce, nBytes); err != nil {
		return Wrapped{}, err
	}

	openFence := "<<<" + w.cfg.OpenTag + ":" + nonce + ">>>"
	closeFence := "<<<" + w.cfg.CloseTag + ":" + nonce + ">>>"

	var b strings.Builder
	b.Grow(len(openFence) + len(closeFence) + len(content) + len(w.cfg.Label) + 8)
	b.WriteString(openFence)
	b.WriteByte('\n')
	if w.cfg.Label != "" {
		b.WriteByte('[')
		b.WriteString(w.cfg.Label)
		b.WriteString("]\n")
	}
	b.WriteString(content)
	b.WriteByte('\n')
	b.WriteString(closeFence)

	return Wrapped{
		Text:       b.String(),
		Nonce:      nonce,
		OpenFence:  openFence,
		CloseFence: closeFence,
		Label:      w.cfg.Label,
	}, nil
}

// Wrap fences content using DefaultConfig and crypto/rand. Each call draws a
// fresh nonce. Use the returned Wrapped artifact (and its Unwrap method) to
// recover the content exactly.
func Wrap(content string) (Wrapped, error) {
	w, err := NewWrapper(DefaultConfig(), nil)
	if err != nil {
		return Wrapped{}, err
	}
	return w.Wrap(content)
}
