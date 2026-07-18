// valid.cue — a generic DLP rule set that satisfies #DLPRuleSet.
// Used by `cue vet ./...` to confirm the schema accepts well-formed data.
package dlpevaluator

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/dlp-evaluator:dlpevaluator"

// GenericDLPRuleSetV1 is a valid rule set that satisfies #DLPRuleSet.
GenericDLPRuleSetV1: dlpevaluator.#DLPRuleSet & {
	Name:    "generic-v1"
	Version: "1.0.0"
	Rules: [
		{
			ID:          "DLP-001"
			Pattern:     "(?i)\\b[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}\\b"
			RiskBand:    "medium"
			Description: "Email address pattern detected."
		},
		{
			ID:          "DLP-002"
			Pattern:     "(?i)\\b(password|passwd|secret|api[_\\-]?key|token)\\s*[:=]\\s*\\S+"
			RiskBand:    "high"
			Description: "Credential or secret assignment detected."
		},
		{
			ID:          "DLP-003"
			Pattern:     "(?i)\\b\\d{3}[-\\s]?\\d{2}[-\\s]?\\d{4}\\b"
			RiskBand:    "high"
			Description: "US Social Security Number pattern detected."
		},
		{
			ID:          "DLP-004"
			Pattern:     "(?i)\\b\\d{4}[\\s\\-]?\\d{4}[\\s\\-]?\\d{4}[\\s\\-]?\\d{4}\\b"
			RiskBand:    "high"
			Description: "Credit card number pattern detected."
		},
		{
			ID:          "DLP-005"
			Pattern:     "(?i)\\bconfidential\\b"
			RiskBand:    "low"
			Description: "Document marked confidential."
		},
	]
}
