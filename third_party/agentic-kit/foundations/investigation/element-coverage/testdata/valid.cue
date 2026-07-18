// valid.cue — concrete values that satisfy the #Element and #ElementCoverage
// constraints defined in schema.cue. Running `cue vet ./...` from the package
// root validates these against the schema.
package elementcoverage

import "github.com/geoffmilleraz/agentic-kit/foundations/investigation/element-coverage:elementcoverage"

// --- #Element fixtures ---

// A required element (negligence theory: duty of care).
_elemDuty: elementcoverage.#Element & {
	ID:       "duty"
	Name:     "Duty of care"
	Required: true
}

// An optional/aggravating element.
_elemAggravating: elementcoverage.#Element & {
	ID:       "aggravating"
	Name:     "Aggravating factor"
	Required: false
}

// --- #ElementCoverage fixtures ---

// A fully-met element with documentary evidence.
_covMet: elementcoverage.#ElementCoverage & {
	ElementID:       "duty"
	Status:          "met"
	WeakestTier:     "documentary"
	Confidence:      0.92
	EvidenceSpanIDs: ["span-1", "span-2"]
	Gap:             ""
}

// An under-evidenced element: only inferred evidence available.
_covUnderEvidenced: elementcoverage.#ElementCoverage & {
	ElementID:       "intent"
	Status:          "under_evidenced"
	WeakestTier:     "inferred"
	Confidence:      0.6
	EvidenceSpanIDs: ["span-3"]
	Gap:             "no circumstantial evidence for \"Intent to defraud\" (weakest tier is inferred)"
}

// An unmet element: no evidence submitted; WeakestTier must be empty.
_covUnmet: elementcoverage.#ElementCoverage & {
	ElementID:       "causation"
	Status:          "unmet"
	WeakestTier:     ""
	Confidence:      0
	EvidenceSpanIDs: []
	Gap:             "no evidence for \"Causation\""
}

// Minimum valid ElementCoverage: met with a single span.
_covMinimal: elementcoverage.#ElementCoverage & {
	ElementID:       "damages"
	Status:          "met"
	WeakestTier:     "testimonial"
	Confidence:      0.75
	EvidenceSpanIDs: ["span-4"]
	Gap:             ""
}
