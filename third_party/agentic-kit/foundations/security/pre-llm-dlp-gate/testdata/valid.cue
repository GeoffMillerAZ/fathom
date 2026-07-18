// valid.cue — a generic DLP redaction policy that satisfies #RedactPolicy.
package prellmdlpgate

import "github.com/geoffmilleraz/agentic-kit/foundations/security/pre-llm-dlp-gate:prellmdlpgate"

GenericPolicyV1: prellmdlpgate.#RedactPolicy & {
	Name:    "generic-v1"
	Version: "1.0.0"
	Rules: [
		{
			ID:          "DLP-001"
			Pattern:     "(?i)(authorization\\s*:\\s*Bearer\\s+)\\S+"
			Label:       "[REDACTED:BEARER]"
			Description: "Authorization Bearer token redaction."
		},
		{
			ID:          "DLP-002"
			Pattern:     "(?i)(api[_-]?key|apikey)\\s*[=:]\\s*\\S+"
			Label:       "[REDACTED:API_KEY]"
			Description: "API key assignment redaction."
		},
		{
			ID:          "DLP-003"
			Pattern:     "\\b\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}\\b"
			Label:       "[REDACTED:CARD]"
			Description: "16-digit payment card number redaction."
		},
	]
}

// GenericExtendedPolicyV1 mirrors the high-precision secret-shape rules that
// Go's ExtendedPolicy() appends to the default catalog (DLP-101…DLP-106).
// Patterns use raw #"…"# strings so regex backslashes need no escaping.
GenericExtendedPolicyV1: prellmdlpgate.#RedactPolicy & {
	Name:    "generic-extended-v1"
	Version: "1.0.0"
	Rules: [
		{
			ID:          "DLP-101"
			Pattern:     #"(?s)-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY(?: BLOCK)?-----.*?-----END (?:[A-Z0-9]+ )*PRIVATE KEY(?: BLOCK)?-----"#
			Label:       "[REDACTED:PEM_PRIVATE_KEY]"
			Description: "PEM private-key block; both BEGIN and END markers required."
		},
		{
			ID:          "DLP-102"
			Pattern:     #"\bAKIA[A-Z0-9]{16}\b"#
			Label:       "[REDACTED:AWS_ACCESS_KEY_ID]"
			Description: "AWS access key ID: AKIA + exactly 16 uppercase alphanumerics."
		},
		{
			ID:          "DLP-103"
			Pattern:     #"\b(?:gh[opsu]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})\b"#
			Label:       "[REDACTED:GITHUB_TOKEN]"
			Description: "GitHub classic (ghp_/gho_/ghs_/ghu_) or fine-grained (github_pat_) token."
		},
		{
			ID:          "DLP-104"
			Pattern:     #"\bxox[abprs]-[A-Za-z0-9-]{20,}\b"#
			Label:       "[REDACTED:SLACK_TOKEN]"
			Description: "Slack token: xox[baprs]- with a 20+ char body."
		},
		{
			ID:          "DLP-105"
			Pattern:     #"\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b"#
			Label:       "[REDACTED:JWT]"
			Description: "JWT: three base64url segments; header and payload eyJ-prefixed."
		},
		{
			ID:          "DLP-106"
			Pattern:     #"\bAIza[0-9A-Za-z_-]{35}\b"#
			Label:       "[REDACTED:GOOGLE_API_KEY]"
			Description: "Google API key: AIza + exactly 35 base64url characters."
		},
	]
}
