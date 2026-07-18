// Package untrustedcontentwrapper fences untrusted external content (user
// documents, tool output, fetched web pages) between per-wrap nonce
// delimiters so a language model is less likely to conflate that content
// with trusted instructions.
//
// # Honest scope
//
// This is defense-in-depth, NOT a security boundary. Fencing reduces the
// chance a model treats external content as instructions, but a model can
// still be persuaded by the wrapped content itself. Do not rely on this
// package as the sole control against prompt injection; combine it with
// instruction-hierarchy enforcement, injection detection, and least-privilege
// tool capabilities.
//
// # Design
//
// Each Wrap call draws a fresh unguessable nonce and renders fences of the
// form
//
//	<<<UNTRUSTED_CONTENT_BEGIN:9f86d081884c7d659a2feaa0c55ad015>>>
//	content
//	<<<UNTRUSTED_CONTENT_END:9f86d081884c7d659a2feaa0c55ad015>>>
//
// Because the content is fixed before the nonce is drawn, the content cannot
// contain the real closing fence: an embedded delimiter forged by an attacker
// carries the wrong nonce and stays inert data. Wrap returns a Wrapped
// artifact carrying the exact fences used, and Unwrap is a method on that
// artifact — there is deliberately no stateless "IsWrapped(string)" check,
// because under per-wrap nonces any such check against static configuration
// would be spoofable and give false confidence.
//
// Wrap/Unwrap is a lossless round trip: content is never mutated or
// neutralized, even when it contains fence-like text.
//
// # Determinism and concurrency
//
// Randomness is injected (randHex func(nBytes int) string) so tests are fully
// deterministic; production defaults to crypto/rand (CryptoRandHex). A
// Wrapper is immutable after construction and safe for concurrent use by
// multiple goroutines provided the injected randHex is concurrency-safe (the
// default is). The package holds no global mutable state.
//
// CUE contracts live in schema.cue; this file and wrapper.go are the Go
// counterpart and must stay in sync. CUE is authoritative for shape; Go is
// authoritative for behavior.
package untrustedcontentwrapper

import (
	"fmt"
	"strings"
)

// forbiddenTagChars are characters that would make the fence grammar
// ambiguous if they appeared inside a tag.
const forbiddenTagChars = "<>:\n\r"

// WrapConfig controls how untrusted content is fenced. The zero value is not
// valid; start from DefaultConfig and override as needed.
type WrapConfig struct {
	// OpenTag names the opening fence, rendered as <<<OpenTag:nonce>>>.
	// Must be non-empty and must not contain '<', '>', ':', or line breaks.
	OpenTag string

	// CloseTag names the closing fence, rendered as <<<CloseTag:nonce>>>.
	// Same character restrictions as OpenTag.
	CloseTag string

	// Label is an optional tag describing the kind of untrusted source
	// (e.g. "USER_DOCUMENT"), rendered as [Label] on the line after the
	// opening fence. Must not contain line breaks. May be empty.
	Label string

	// NonceBytes is the number of random bytes drawn per wrap; the nonce is
	// hex-encoded to 2*NonceBytes characters. 0 selects the default
	// (16 bytes, i.e. 128 bits); otherwise it must be in [8, 64].
	NonceBytes int
}

const (
	defaultOpenTag    = "UNTRUSTED_CONTENT_BEGIN"
	defaultCloseTag   = "UNTRUSTED_CONTENT_END"
	defaultNonceBytes = 16

	minNonceBytes = 8
	maxNonceBytes = 64
)

// DefaultConfig returns the default WrapConfig: standard tags, no label,
// 16-byte (128-bit) nonces.
func DefaultConfig() WrapConfig {
	return WrapConfig{
		OpenTag:    defaultOpenTag,
		CloseTag:   defaultCloseTag,
		Label:      "",
		NonceBytes: defaultNonceBytes,
	}
}

// Validate returns an error if the config violates the fence grammar:
// empty tags, forbidden characters in tags, line breaks in the label, or an
// out-of-range NonceBytes.
func (c WrapConfig) Validate() error {
	if c.OpenTag == "" {
		return fmt.Errorf("untrustedcontentwrapper: OpenTag must not be empty")
	}
	if strings.ContainsAny(c.OpenTag, forbiddenTagChars) {
		return fmt.Errorf("untrustedcontentwrapper: OpenTag %q must not contain '<', '>', ':', or line breaks", c.OpenTag)
	}
	if c.CloseTag == "" {
		return fmt.Errorf("untrustedcontentwrapper: CloseTag must not be empty")
	}
	if strings.ContainsAny(c.CloseTag, forbiddenTagChars) {
		return fmt.Errorf("untrustedcontentwrapper: CloseTag %q must not contain '<', '>', ':', or line breaks", c.CloseTag)
	}
	if strings.ContainsAny(c.Label, "\n\r") {
		return fmt.Errorf("untrustedcontentwrapper: Label %q must not contain line breaks", c.Label)
	}
	if c.NonceBytes != 0 && (c.NonceBytes < minNonceBytes || c.NonceBytes > maxNonceBytes) {
		return fmt.Errorf("untrustedcontentwrapper: NonceBytes %d must be 0 (default) or in [%d, %d]", c.NonceBytes, minNonceBytes, maxNonceBytes)
	}
	return nil
}

// Wrapped is the artifact produced by Wrap. It carries the exact per-wrap
// fences so unwrapping and verification are exact: a Wrapped artifact is the
// only way to unwrap, which makes stateless misuse (matching text against
// static delimiters) impossible by construction.
//
// CUE mirror: #Wrapped in schema.cue.
type Wrapped struct {
	// Text is the full fenced text to present to the model:
	// OpenFence + "\n" + (optional "[Label]\n") + content + "\n" + CloseFence.
	Text string

	// Nonce is the lowercase hex nonce embedded in both fences of this wrap.
	Nonce string

	// OpenFence is the exact opening fence line, including the nonce.
	OpenFence string

	// CloseFence is the exact closing fence line, including the nonce.
	CloseFence string

	// Label is the label rendered after the opening fence, if any.
	Label string
}

// Unwrap verifies Text against the artifact's exact fences and returns the
// original content. The round trip is lossless: content containing
// fence-like text — even a byte-exact copy of the real fences — is returned
// unmodified, because only the leading open fence and the trailing close
// fence are stripped.
//
// Unwrap returns an error if Text does not have the expected structure, for
// example because it was modified after wrapping or the artifact was
// hand-constructed inconsistently.
func (wr Wrapped) Unwrap() (string, error) {
	if wr.OpenFence == "" || wr.CloseFence == "" {
		return "", fmt.Errorf("untrustedcontentwrapper: Unwrap: artifact has an empty fence")
	}
	rest, ok := strings.CutPrefix(wr.Text, wr.OpenFence+"\n")
	if !ok {
		return "", fmt.Errorf("untrustedcontentwrapper: Unwrap: text does not begin with the artifact's open fence")
	}
	if wr.Label != "" {
		rest, ok = strings.CutPrefix(rest, "["+wr.Label+"]\n")
		if !ok {
			return "", fmt.Errorf("untrustedcontentwrapper: Unwrap: text is missing the label line [%s]", wr.Label)
		}
	}
	content, ok := strings.CutSuffix(rest, "\n"+wr.CloseFence)
	if !ok {
		return "", fmt.Errorf("untrustedcontentwrapper: Unwrap: text does not end with the artifact's close fence")
	}
	return content, nil
}
