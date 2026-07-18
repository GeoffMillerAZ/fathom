// Package sensitivityclassifier classifies text against a configurable catalog
// of ClassRules loaded from CUE. No org-specific literals are hard-coded here —
// the caller supplies the catalog, making the classifier fully org-agnostic.
//
// Concurrency: Classifier is safe for concurrent use after construction.
package sensitivityclassifier

import (
	dst "github.com/geoffmilleraz/agentic-kit/foundations/security/data-sensitivity-types"
)

// ClassRule is a single classification rule. Pattern must be a valid Go regexp.
// Level and Description are supplied by the caller's catalog — never hard-coded
// with org-specific strings inside this package.
type ClassRule struct {
	// ID is a unique, stable identifier for the rule (e.g. "CR-001").
	ID string

	// Pattern is a Go regexp matched case-insensitively against the text.
	Pattern string

	// Level is the sensitivity level assigned when this rule matches.
	Level dst.SensitivityLevel

	// Description is a human-readable explanation of what this rule detects.
	Description string
}

// Catalog is a named, versioned collection of ClassRules. It is the unit loaded
// from a CUE file and passed to Classify.
type Catalog struct {
	// Name identifies the catalog (e.g. "generic-v1").
	Name string

	// Version is a semver-style string (e.g. "1.0.0").
	Version string

	// Rules is the ordered list of classification rules.
	Rules []ClassRule
}
