// Package tokenization defines CUE contracts for the sensitive-data tokenization
// and vaulting foundation. Go types in types.go, token.go, audit.go, vault.go,
// and policy.go are the Go counterparts; any drift is a bug.
package tokenization

// #EncryptedRecord mirrors EncryptedRecord v1. Ciphertext is raw nonce+GCM bytes;
// JSON encodes Go []byte as base64 and adapters decode it before CUE validation.
#EncryptedRecord: {
	Version: 1
	Token: #Token & {
		Class: #TokenClass
		Value: =~("^tok_" + Class + "_[0-9a-fA-F]{64}$")
	}
	Ciphertext:            bytes
	_ciphertextLongEnough: true & (len(Ciphertext) >= 28)
}

// #TokenClass enumerates the recognised sensitivity categories for a secret.
// The class is embedded in the opaque token value and drives access-policy
// decisions.
#TokenClass: "pii" | "financial" | "privileged" | "sensitive" | "other"

// #Token is an opaque reference to a vaulted secret. Value is of the form
// "tok_<class>_<hex>"; it reveals nothing about the underlying value.
#Token: {
	// Value is the opaque token string, e.g. "tok_pii_3fa2c...".
	Value: string & !="" & =~"^tok_[a-z]+"

	// Class is the sensitivity category of the underlying secret.
	Class: #TokenClass

	// Namespace scopes the token. The same value in different namespaces
	// produces different deterministic tokens, providing tenant isolation.
	Namespace: string & !=""
}

// #AuditAction discriminates between the two auditable operations.
#AuditAction: "tokenize" | "detokenize"

// #AuditEvent is a single immutable record in the vault audit log. The raw
// secret value must NEVER appear in any field of an AuditEvent.
#AuditEvent: {
	// At is the RFC3339-formatted UTC wall-clock time of the action.
	At: string & !=""

	// Action is either "tokenize" or "detokenize".
	Action: #AuditAction

	// TokenValue is the opaque token (never the raw secret).
	TokenValue: string & !=""

	// Class is the sensitivity category recorded at audit time.
	Class: #TokenClass

	// PrincipalID identifies the actor. For AI authors this is "ai:<ModelID>".
	PrincipalID: string & !=""

	// Purpose is the stated reason for a detokenize request.
	// May be empty for tokenize events.
	Purpose: string

	// Allowed reports whether the operation was permitted.
	Allowed: bool
}
