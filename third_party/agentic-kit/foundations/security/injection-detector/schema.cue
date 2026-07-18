// Package injectiondetector defines CUE contracts for prompt-injection detection.
// Rules and severities are data — they are loaded from CUE, not hard-coded in Go.
package injectiondetector

// #Severity enumerates the four recognised threat levels.
// Values are strictly ordered: low < medium < high < critical.
#Severity: "low" | "medium" | "high" | "critical"

// #Rule describes a single detection rule.
#Rule: {
	// ID is a unique, stable identifier for the rule (e.g. "INJ-001").
	ID: string & !=""

	// Pattern is a Go regexp expression matched against untrusted content.
	Pattern: string & !=""

	// Severity is the threat level assigned when this rule fires.
	Severity: #Severity

	// Description is a human-readable explanation of the threat this rule detects.
	Description: string & !=""
}

// #RuleSet is a named, versioned collection of detection rules.
#RuleSet: {
	// Name identifies the rule set (e.g. "generic-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the list of rules. At least one rule is required.
	Rules: [#Rule, ...#Rule]
}
