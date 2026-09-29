package tokenization

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Token is an opaque reference to a vaulted secret. Its Value field is of the
// form "tok_<class>_<hex>" where <hex> is either random or an HMAC-SHA256
// digest. The token itself reveals nothing about the underlying value.
type Token struct {
	// Value is the opaque token string, e.g. "tok_pii_3fa2c...".
	Value string

	// Class is the sensitivity category of the underlying secret.
	Class TokenClass

	// Namespace scopes the token: the same value in different namespaces
	// produces different deterministic tokens, providing isolation between
	// tenants, cases, or domains.
	Namespace string
}

// Validate returns an error if the Token violates any invariant.
func (t Token) Validate() error {
	if t.Value == "" {
		return fmt.Errorf("tokenization: Token.Value must not be empty")
	}
	if t.Namespace == "" {
		return fmt.Errorf("tokenization: Token.Namespace must not be empty")
	}
	return t.Class.Validate()
}

// String returns the opaque token value string. It is identical to Token.Value
// and is provided so Token satisfies the fmt.Stringer interface.
func (t Token) String() string {
	return t.Value
}

// Parse parses a token value string of the form "tok_<class>_<hex>" and
// returns a Token with the Value and Class populated. Namespace is left empty
// because it is not encoded in the opaque value — callers must supply it from
// context if needed.
//
// Parse returns an error if the string does not begin with "tok_", the class
// segment is not a recognised TokenClass, or the hex segment is missing.
func Parse(s string) (Token, error) {
	if !strings.HasPrefix(s, "tok_") {
		return Token{}, fmt.Errorf("tokenization: Parse: token must start with \"tok_\", got %q", s)
	}
	// Expected format: tok_<class>_<hex>
	rest := s[len("tok_"):]
	idx := strings.Index(rest, "_")
	if idx < 1 {
		return Token{}, fmt.Errorf("tokenization: Parse: malformed token %q (missing class separator)", s)
	}
	classStr := rest[:idx]
	hexPart := rest[idx+1:]
	if hexPart == "" {
		return Token{}, fmt.Errorf("tokenization: Parse: malformed token %q (empty hex segment)", s)
	}
	class := TokenClass(classStr)
	if err := class.Validate(); err != nil {
		return Token{}, fmt.Errorf("tokenization: Parse: %w", err)
	}
	return Token{Value: s, Class: class}, nil
}

// Tokenizer generates Token values. It uses an injected randomness source and
// an optional HMAC key (for DeterministicToken). The clock is injected to keep
// domain logic deterministic in tests.
//
// A nil HMAC key disables HMAC-keying for DeterministicToken; use a non-nil
// key in production to prevent rainbow-table attacks on deterministic tokens.
type Tokenizer struct {
	hmacKey []byte
	clock   func() time.Time
	rand    RandFunc
}

// NewTokenizer constructs a Tokenizer with the supplied HMAC key, clock, and
// random source. hmacKey may be nil (deterministic tokens are then unkeyed
// HMAC-SHA256 over a zero key — adequate for uniqueness but not for
// rainbow-table resistance; supply a non-empty key in production).
// clock and rand must not be nil.
func NewTokenizer(hmacKey []byte, clock func() time.Time, rand RandFunc) *Tokenizer {
	if clock == nil {
		panic("tokenization: NewTokenizer: clock must not be nil")
	}
	if rand == nil {
		panic("tokenization: NewTokenizer: rand must not be nil")
	}
	// Defensive copy of hmacKey.
	var keycp []byte
	if len(hmacKey) > 0 {
		keycp = make([]byte, len(hmacKey))
		copy(keycp, hmacKey)
	}
	return &Tokenizer{hmacKey: keycp, clock: clock, rand: rand}
}

// RandomToken generates a cryptographically random token. The token value is
// "tok_<class>_<32-byte-random-hex>". Each call produces a unique token.
func (t *Tokenizer) RandomToken(class TokenClass, namespace string) (Token, error) {
	if err := class.Validate(); err != nil {
		return Token{}, err
	}
	if namespace == "" {
		return Token{}, fmt.Errorf("tokenization: RandomToken: namespace must not be empty")
	}
	raw, err := t.rand(32)
	if err != nil {
		return Token{}, fmt.Errorf("tokenization: RandomToken: rand: %w", err)
	}
	if len(raw) != 32 {
		return Token{}, fmt.Errorf("tokenization: RandomToken: rand must return 32 bytes")
	}
	value := "tok_" + string(class) + "_" + hex.EncodeToString(raw)
	return Token{Value: value, Class: class, Namespace: namespace}, nil
}

// DeterministicToken derives a token via keyed HMAC-SHA256 over
// (namespace || "\x00" || class || "\x00" || value). The same triple of
// (value, class, namespace) always produces the same token value, making it
// possible to join records without storing the secret. Different namespaces
// always produce different tokens, providing tenant isolation.
//
// WARNING: deterministic tokens leak equality by design. Two records that
// tokenize the same value in the same namespace will have identical token
// values, which an observer can use to infer that the underlying values match.
// This is the intended behavior for join use-cases; do not use deterministic
// tokens where even equality must be hidden.
func (t *Tokenizer) DeterministicToken(value string, class TokenClass, namespace string) Token {
	mac := hmac.New(sha256.New, t.hmacKey)
	// Length-prefix each segment with a NUL separator to make encoding injective:
	// distinct (namespace, class, value) tuples cannot collide even if the fields
	// contain NUL characters.
	fmt.Fprintf(
		mac, "%d\x00%s\x00%d\x00%s\x00%d\x00%s", //nolint:errcheck
		len(namespace), namespace,
		len(class), string(class),
		len(value), value,
	)
	digest := mac.Sum(nil)
	tokenValue := "tok_" + string(class) + "_" + hex.EncodeToString(digest)
	return Token{Value: tokenValue, Class: class, Namespace: namespace}
}

// FormatPreserving returns a display-safe masked form of s that preserves the
// string length and exposes only the last last4 characters (or fewer if the
// input is shorter). All other characters are replaced with mask. Separator
// characters (hyphens and spaces) in the non-exposed prefix are preserved to
// maintain visual format.
//
// This function is display-only. It does NOT store or reference any secret;
// the returned string is safe to display in a UI without revealing the value.
// It must never be stored as a substitute for a real token.
func FormatPreserving(s string, mask byte, last4 int) string {
	runes := []rune(s)
	n := len(runes)
	if last4 >= n {
		// Input shorter than the exposed suffix — return as-is.
		return s
	}
	exposed := n - last4
	out := make([]rune, n)
	for i := 0; i < exposed; i++ {
		r := runes[i]
		// Preserve separator characters so the masked form retains visual format
		// (e.g. "***-**-1234" rather than "*********1234").
		if r == '-' || r == ' ' {
			out[i] = r
		} else {
			out[i] = rune(mask)
		}
	}
	copy(out[exposed:], runes[exposed:])
	return string(out)
}
