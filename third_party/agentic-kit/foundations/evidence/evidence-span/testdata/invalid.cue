// invalid.cue — demonstrates #Span values that VIOLATE the schema.
// This file is intentionally NOT included in the `cue vet` path (the valid.cue
// sibling is the one vetted in CI). It is kept here as a documentation
// artifact so reviewers understand what the schema rejects.
package evidencespan

// _badConfidenceHigh: Confidence > 1 violates the >=0 & <=1 constraint.
// _badConfidenceHigh: #Span & {
// 	ID:         "span-bad-high"
// 	EntityID:   "repo-payments"
// 	Field:      "coverage"
// 	Value:      "110%"
// 	Confidence: 1.5
// 	SourceID:   "fabricated"
// }

// _badConfidenceLow: Confidence < 0 violates the >=0 & <=1 constraint.
// _badConfidenceLow: #Span & {
// 	ID:         "span-bad-low"
// 	EntityID:   "repo-payments"
// 	Field:      "coverage"
// 	Value:      "-5%"
// 	Confidence: -0.1
// 	SourceID:   "fabricated"
// }

// _badEmptyID: empty ID violates the non-empty string constraint.
// _badEmptyID: #Span & {
// 	ID:         ""
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// }

// _badLocatorKind: Locator.Kind "paragraph" is not in the allowed set
// (page|line|char-range|byte-range|cell|timestamp|xpath|none).
// _badLocatorKind: #Span & {
// 	ID:         "span-bad-locator"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	Locator: {
// 		Kind:  "paragraph"
// 		Value: "intro"
// 		Start: 0
// 		End:   0
// 	}
// }

// _badLocatorNegativeStart: Start < 0 violates int & >=0 constraint.
// Go Locator.Validate() and CUE #Locator both reject negative Start/End.
// Note: 0 is the sentinel for "unset" — negative values are never valid.
// _badLocatorNegativeStart: #Span & {
// 	ID:         "span-bad-neg-start"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	Locator: {
// 		Kind:  "char-range"
// 		Value: "x"
// 		Start: -1
// 		End:   5
// 	}
// }

// _badLocatorNegativeEnd: End < 0 violates int & >=0 constraint.
// _badLocatorNegativeEnd: #Span & {
// 	ID:         "span-bad-neg-end"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	Locator: {
// 		Kind:  "char-range"
// 		Value: "x"
// 		Start: 0
// 		End:   -5
// 	}
// }

// _badLocatorStartGtEnd: Start=20 > End=10 violates the End: int & >=Start
// constraint added in schema.cue (Finding 1: CUE #Locator now enforces
// Start<=End, matching the existing Go Locator.Validate() rule).
// _badLocatorStartGtEnd: #Span & {
// 	ID:         "span-bad-start-gt-end"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	Locator: {
// 		Kind:  "char-range"
// 		Value: "10-20"
// 		Start: 20
// 		End:   10
// 	}
// }

// _badOccurredAtNotRFC3339: OccurredAt "banana" is not a valid RFC3339
// timestamp. The schema now constrains both OccurredAt and ObservedAt to
// _rfc3339 (Finding 3: bare strings accepted non-timestamp values).
// _badOccurredAtNotRFC3339: #Span & {
// 	ID:         "span-bad-occurred-at"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	OccurredAt: "banana"
// }

// _badObservedAtNoTZ: ObservedAt "2024-06-01T14:02:00" lacks a timezone
// offset — not a valid RFC3339 string (RFC3339 requires Z or +/-HH:MM suffix).
// _badObservedAtNoTZ: #Span & {
// 	ID:         "span-bad-observed-at"
// 	EntityID:   "repo-payments"
// 	Field:      "language"
// 	Value:      "Go"
// 	Confidence: 0.9
// 	SourceID:   "src-code"
// 	ObservedAt: "2024-06-01T14:02:00"
// }
