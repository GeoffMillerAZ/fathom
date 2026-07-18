// Package intentclassifier defines CUE contracts for the intent classifier.
// Intents are data — #Intent is an open string type and #Rule carries
// caller-supplied keywords. No org-specific intent labels are hard-coded here.
package intentclassifier

// #Intent is the label assigned to a classified query.
// Any non-empty string is valid; callers define their own intent vocabulary.
#Intent: string & !=""

// #Rule maps a single intent to one or more match keywords.
// All fields are required; Keywords must have at least one entry.
#Rule: {
	// Intent is the label returned when this rule fires.
	Intent: #Intent

	// Keywords is a non-empty list of case-insensitive match terms.
	Keywords: [string & !="", ...string & !=""]
}

// #RuleSet is a named, versioned collection of classifier rules.
#RuleSet: {
	// Name identifies the rule set (e.g. "customer-support-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the ordered list of rules. At least one rule is required.
	Rules: [#Rule, ...#Rule]
}
