// Package agenticpracticesevaluator defines CUE contracts for scoring an agent
// run against a configurable best-practice checklist. Practices are data —
// loaded from CUE, not hard-coded in Go.
package agenticpracticesevaluator

// #Practice describes a single best-practice checklist item.
#Practice: {
	// Name is a unique, human-readable identifier for the practice
	// (e.g. "uses-evidence-citations").
	Name: string & !=""

	// Description is a human-readable explanation of what the practice checks.
	Description: string & !=""
}

// #Checklist is a named, versioned collection of practices.
#Checklist: {
	// Name identifies the checklist (e.g. "investigation-v1").
	Name: string & !=""

	// Version is a semver-style string (e.g. "1.0.0").
	Version: string & !=""

	// Practices is the list of practices. At least one is required.
	Practices: [#Practice, ...#Practice]
}
