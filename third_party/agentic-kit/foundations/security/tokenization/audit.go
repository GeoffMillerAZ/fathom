package tokenization

import (
	"fmt"
	"time"
)

// AuditAction discriminates between the two audited operations.
type AuditAction string

const (
	// ActionTokenize is recorded when a value is tokenized (stored in the vault).
	ActionTokenize AuditAction = "tokenize"

	// ActionDetokenize is recorded when a token lookup is attempted (allowed or denied).
	ActionDetokenize AuditAction = "detokenize"
)

// AuditEvent is a single immutable record in the vault audit log. It records
// who did what, when, and whether they were permitted — but NEVER the raw
// secret value. The raw value must never appear in any AuditEvent field.
type AuditEvent struct {
	// At is the UTC wall-clock time at which the action was recorded.
	At time.Time

	// Action is either ActionTokenize or ActionDetokenize.
	Action AuditAction

	// TokenValue is the opaque token value (e.g. "tok_pii_3fa2..."). The
	// underlying secret is never stored here.
	TokenValue string

	// Class is the sensitivity category of the token.
	Class TokenClass

	// PrincipalID identifies the actor. For tokenize events this is the
	// authoring principal's ID (from the aiprovenance.Author); for detokenize
	// events it is the requesting principal's ID.
	PrincipalID string

	// Purpose is the stated reason for the detokenize request.
	// Empty for tokenize events.
	Purpose string

	// Allowed reports whether the operation was permitted. Always true for
	// tokenize events; may be false for denied detokenize attempts.
	Allowed bool
}

// Validate returns an error if the AuditEvent violates any invariant.
func (e AuditEvent) Validate() error {
	if e.At.IsZero() {
		return fmt.Errorf("tokenization: AuditEvent.At must not be zero")
	}
	if e.TokenValue == "" {
		return fmt.Errorf("tokenization: AuditEvent.TokenValue must not be empty")
	}
	if e.Action != ActionTokenize && e.Action != ActionDetokenize {
		return fmt.Errorf("tokenization: AuditEvent.Action must be %q or %q, got %q",
			ActionTokenize, ActionDetokenize, e.Action)
	}
	return nil
}
