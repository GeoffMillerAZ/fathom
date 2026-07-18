// valid.cue — example IntegrityPolicy and Report values satisfying the schema.
package outputintegritychecker

import "github.com/geoffmilleraz/agentic-kit/foundations/security/output-integrity-checker:outputintegritychecker"

GenericPolicyV1: outputintegritychecker.#IntegrityPolicy & {
	Name:    "generic-v1"
	Version: "1.0.0"
	RequiredDisclaimers: [
		{
			ID:          "RD-001"
			Substring:   "this is not professional advice"
			Description: "General non-professional-advice disclaimer."
		},
	]
	BannedTokens: [
		{
			ID:          "BT-001"
			Pattern:     "(?i)\\bconfidential\\b"
			Description: "Output must not contain the word 'confidential'."
		},
		{
			ID:          "BT-002"
			Pattern:     "(?i)(ssn|social.security.number)"
			Description: "Output must not contain PII patterns."
		},
	]
}

PassedReport: outputintegritychecker.#Report & {
	Passed:     true
	Violations: []
}

FailedReport: outputintegritychecker.#Report & {
	Passed: false
	Violations: [
		{
			RuleID:  "RD-001"
			Kind:    "missing_disclaimer"
			Detail:  "required disclaimer \"this is not professional advice\" not found"
			Excerpt: ""
		},
	]
}
