// Package sensitivityclassifier defines CUE contracts for configurable
// sensitivity classification rules. No org-specific literals appear here —
// sensitivity catalogs are pure data loaded at runtime.
package sensitivityclassifier

import dst "github.com/geoffmilleraz/agentic-kit/foundations/security/data-sensitivity-types:datasensitivitytypes"

// #ClassRule is a single configurable classification rule.
// Pattern must be a valid Go regexp; Level must be a recognised #SensitivityLevel.
#ClassRule: {
	// ID is a unique, stable identifier for the rule (e.g. "CR-001").
	ID: string & !=""

	// Pattern is a Go regexp expression matched case-insensitively against text.
	Pattern: string & !=""

	// Level is the sensitivity level assigned when this rule matches.
	Level: dst.#SensitivityLevel

	// Description is a human-readable explanation of what this rule detects.
	Description: string & !=""
}

// #Catalog is a named, versioned collection of ClassRules.
#Catalog: {
	// Name identifies the catalog (e.g. "generic-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Rules is the list of classification rules. At least one rule is required.
	Rules: [#ClassRule, ...#ClassRule]
}
