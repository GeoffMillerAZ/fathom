// valid.cue — concrete values that satisfy the tokenization schema constraints.
// Run `cue vet ./...` from the kit root to validate these against schema.cue.
package tokenization

import "github.com/geoffmilleraz/agentic-kit/foundations/security/tokenization"

// A PII token for a Social Security Number.
PIITokenExample: tokenization.#Token & {
	Value:     "tok_pii_3fa2c44298fc1c149afbf4c8996fb924abc12345678901234567890abcdef12"
	Class:     "pii"
	Namespace: "case-001"
}

// A financial token for a credit card number.
FinancialTokenExample: tokenization.#Token & {
	Value:     "tok_financial_9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a0"
	Class:     "financial"
	Namespace: "acme-corp"
}

// A privileged token for attorney-client communication content.
PrivilegedTokenExample: tokenization.#Token & {
	Value:     "tok_privileged_deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef00"
	Class:     "privileged"
	Namespace: "matter-42"
}

// A tokenize audit event (always Allowed=true in normal operation).
TokenizeEventExample: tokenization.#AuditEvent & {
	At:          "2024-01-01T00:00:00Z"
	Action:      "tokenize"
	TokenValue:  "tok_pii_3fa2c44298fc1c149afbf4c8996fb924abc12345678901234567890abcdef12"
	Class:       "pii"
	PrincipalID: "analyst@example.com"
	Purpose:     ""
	Allowed:     true
}

// An allowed detokenize audit event.
DetokenizeAllowedExample: tokenization.#AuditEvent & {
	At:          "2024-01-01T00:01:00Z"
	Action:      "detokenize"
	TokenValue:  "tok_pii_3fa2c44298fc1c149afbf4c8996fb924abc12345678901234567890abcdef12"
	Class:       "pii"
	PrincipalID: "analyst@example.com"
	Purpose:     "legal review"
	Allowed:     true
}

// A denied detokenize audit event (Allowed=false records the refusal).
DetokenizeDeniedExample: tokenization.#AuditEvent & {
	At:          "2024-01-01T00:02:00Z"
	Action:      "detokenize"
	TokenValue:  "tok_pii_3fa2c44298fc1c149afbf4c8996fb924abc12345678901234567890abcdef12"
	Class:       "pii"
	PrincipalID: "intruder@evil.com"
	Purpose:     "unauthorized probe"
	Allowed:     false
}
