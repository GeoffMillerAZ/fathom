// Package tokenization provides pure-logic sensitive-data tokenization and
// vaulting. A sensitive value is replaced with an opaque token; only an
// authorized principal can detokenize. Every tokenize and detokenize action
// (allowed and denied) is recorded in an immutable audit log.
//
// Design goals:
//   - Zero external dependencies: stdlib only (crypto/hmac, crypto/sha256,
//     crypto/rand, encoding/hex, sync, time, fmt, errors, sort).
//   - No inline time.Now() or crypto/rand calls in domain logic; both are
//     injected so tests are fully deterministic.
//   - Defensive copies on all slice/map returns.
//   - The Vault interface is a port; encrypted-at-rest storage is an app
//     adapter — not implemented here. MemoryVault is the reference impl.
//
// CUE contracts live in schema.cue; this file and its siblings are the Go
// counterpart. Drift between Go and CUE is a bug.
//
// Concurrency: Tokenizer is safe for concurrent use after construction; MemoryVault is safe for concurrent use via sync.RWMutex.
package tokenization

import (
	"fmt"
)

// TokenClass categorizes the underlying secret. The class drives access-policy
// decisions and is embedded in the token value string.
type TokenClass string

const (
	// ClassPII covers personally identifiable information (SSNs, DOBs, addresses).
	ClassPII TokenClass = "pii"

	// ClassFinancial covers financial account numbers, card numbers, etc.
	ClassFinancial TokenClass = "financial"

	// ClassPrivileged covers attorney-client privileged or legally protected data.
	ClassPrivileged TokenClass = "privileged"

	// ClassSensitive covers sensitive-but-unclassified data not in other classes.
	ClassSensitive TokenClass = "sensitive"

	// ClassOther covers any sensitive data that does not fit another class.
	ClassOther TokenClass = "other"
)

// validClasses is the authoritative set of accepted TokenClass values.
var validClasses = map[TokenClass]bool{
	ClassPII:        true,
	ClassFinancial:  true,
	ClassPrivileged: true,
	ClassSensitive:  true,
	ClassOther:      true,
}

// Validate returns an error if c is not a recognised TokenClass value.
func (c TokenClass) Validate() error {
	if !validClasses[c] {
		return fmt.Errorf("tokenization: unknown TokenClass %q; must be one of pii|financial|privileged|sensitive|other", c)
	}
	return nil
}

// Principal identifies the actor requesting a Vault operation.
type Principal struct {
	// ID is a non-empty unique identifier for the actor (e.g. user UUID or email).
	ID string

	// Roles is the set of roles assigned to this actor. An empty slice means
	// the actor has no roles and will be denied by a RolePolicy.
	Roles []string
}

// AccessPolicy determines whether a principal may detokenize a value of a
// given class for a stated purpose.
type AccessPolicy interface {
	Allow(principal Principal, class TokenClass, purpose string) bool
}

// RandFunc is the signature of the injected randomness source. It returns
// exactly n cryptographically random bytes, or an error.
type RandFunc func(n int) ([]byte, error)
