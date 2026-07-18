// invalid.cue — demonstrates a #Span value that VIOLATES the schema.
// Confidence is 1.5, which is outside the allowed [0, 1] range.
// `cue vet` on this file alone would report a constraint violation;
// it is kept here as a documentation artifact only and is NOT included
// in the vet path (the valid.cue sibling is the one vetted in CI).
package evidencegraph

// _badSpan has confidence > 1, violating the >=0 & <=1 constraint.
// This is intentionally invalid — do not add it to a vet target.
_badSpan: {
	ID:         "span-bad"
	EntityID:   "repo-payments"
	Field:      "coverage"
	Value:      "110%"
	Confidence: 1.5
	Source:     "fabricated"
	ExpiresAt:  "2099-01-01T00:00:00Z"
}
