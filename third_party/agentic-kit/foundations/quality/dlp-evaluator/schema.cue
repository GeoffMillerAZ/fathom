// Package dlpevaluator defines CUE contracts for DLP (Data Loss Prevention)
// rule evaluation. Rules and risk bands are data — loaded from CUE, not
// hard-coded in Go, so the evaluator is fully org-agnostic.
package dlpevaluator

// #RiskBand enumerates the four leakage-risk levels.
// Values are strictly ordered: none < low < medium < high.
#RiskBand: "none" | "low" | "medium" | "high"

// #DLPRule describes a single detection rule for sensitive data patterns.
#DLPRule: {
	// ID is a unique, stable identifier for the rule (e.g. "DLP-001").
	ID: string & !=""

	// Pattern is a Go regexp matched against the evaluated text.
	Pattern: string & !=""

	// RiskBand is the leakage risk level assigned when this rule fires.
	RiskBand: #RiskBand

	// Description is a human-readable explanation of what this rule detects.
	Description: string & !=""
}

// #DLPRuleSet is a named, versioned collection of DLP rules.
#DLPRuleSet: {
	// Name identifies the rule set (e.g. "generic-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the list of rules. At least one rule is required.
	Rules: [#DLPRule, ...#DLPRule]
}
