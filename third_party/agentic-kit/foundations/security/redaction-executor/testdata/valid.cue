// Package redactionexecutor — valid test data that must unify with schema.cue.
package redactionexecutor

import "github.com/geoffmilleraz/agentic-kit/foundations/security/redaction-executor:redactionexecutor"

// validPIIRule demonstrates a well-formed rule that redacts email addresses.
validPIIRule: redactionexecutor.#RedactionRule & {
	ID:          "REDACT-PII-001"
	Pattern:     "[a-zA-Z0-9._%+\\-]+@[a-zA-Z0-9.\\-]+\\.[a-zA-Z]{2,}"
	Replacement: "[EMAIL]"
}

// validPhoneRule demonstrates a rule with a non-empty replacement.
validPhoneRule: redactionexecutor.#RedactionRule & {
	ID:          "REDACT-PII-002"
	Pattern:     "\\b\\d{3}[-.\\s]?\\d{3}[-.\\s]?\\d{4}\\b"
	Replacement: "[PHONE]"
}

// validRemovalRule demonstrates a rule with an empty replacement (removes matched text).
validRemovalRule: redactionexecutor.#RedactionRule & {
	ID:          "REDACT-TOKEN-001"
	Pattern:     "Bearer\\s+[A-Za-z0-9._\\-]+"
	Replacement: ""
}

// validPIIPolicy demonstrates a well-formed policy with two rules.
validPIIPolicy: redactionexecutor.#RedactionPolicy & {
	Name:    "pii-v1"
	Version: "1.0.0"
	Rules: [
		{
			ID:          "REDACT-PII-001"
			Pattern:     "[a-zA-Z0-9._%+\\-]+@[a-zA-Z0-9.\\-]+\\.[a-zA-Z]{2,}"
			Replacement: "[EMAIL]"
		},
		{
			ID:          "REDACT-PII-002"
			Pattern:     "\\b\\d{3}[-.\\s]?\\d{3}[-.\\s]?\\d{4}\\b"
			Replacement: "[PHONE]"
		},
	]
}
