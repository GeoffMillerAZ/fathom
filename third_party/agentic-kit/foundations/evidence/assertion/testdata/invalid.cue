// invalid.cue — demonstrates #Proposition and #Assertion values that VIOLATE
// the schema. This file is intentionally NOT included in the `cue vet` path
// (the valid.cue sibling is the one vetted in CI). It is kept here as a
// documentation artifact so reviewers understand what the schema rejects.
//
// Additions document what the #Knowledge and _rfc3339 constraints reject.
package assertion

// _badPropositionEmptyID: empty ID violates the non-empty string constraint.
// _badPropositionEmptyID: #Proposition & {
// 	ID:        ""
// 	Canonical: "Alice was at the bar"
// 	Subject:   "entity:alice"
// }

// _badPropositionEmptyCanonical: empty Canonical violates the non-empty constraint.
// _badPropositionEmptyCanonical: #Proposition & {
// 	ID:        "prop-bad"
// 	Canonical: ""
// 	Subject:   "entity:alice"
// }

// _badPropositionEmptySubject: empty Subject violates the non-empty constraint.
// _badPropositionEmptySubject: #Proposition & {
// 	ID:        "prop-bad"
// 	Canonical: "Alice was at the bar"
// 	Subject:   ""
// }

// _badAssertionEmptyID: empty ID violates the non-empty constraint.
// _badAssertionEmptyID: #Assertion & {
// 	ID:            ""
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// }

// _badAssertionUnknownStance: stance "speculates" is not in the allowed set.
// _badAssertionUnknownStance: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "speculates"
// 	Content:       "I think I was there"
// }

// _badAssertionEmptyDeclarant: empty Declarant violates the non-empty constraint.
// _badAssertionEmptyDeclarant: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     ""
// 	Stance:        "affirms"
// 	Content:       "I was there"
// }

// _badAssertionInvalidVia: "hearsay" is not a recognised tier.
// _badAssertionInvalidVia: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	Via:           "hearsay"
// }

// _badAssertionNegativeLocatorStart: Locator.Start < 0 violates int & >=0.
// _badAssertionNegativeLocatorStart: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	Locator: {
// 		Kind:  "line"
// 		Value: "x"
// 		Start: -1
// 		End:   5
// 	}
// }

// _badAssertionBadGradeConfidence: Grade.Confidence > 1 violates >=0 & <=1.
// _badAssertionBadGradeConfidence: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	Grade: {
// 		Tier:       "testimonial"
// 		Confidence: 1.5
// 	}
// }

// _badAssertionInvalidLocatorKind: "paragraph" is not in the allowed set.
// _badAssertionInvalidLocatorKind: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	Locator: {
// 		Kind:  "paragraph"
// 		Value: "intro"
// 		Start: 0
// 		End:   0
// 	}
// }

// _badAssertionUnknownKnowledge: "hearsay" is not in #Knowledge (must be first_hand|reported|belief).
// _badAssertionUnknownKnowledge: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	Knowledge:     "hearsay"
// }

// _badAssertionBadAsOf: "2024-01-15" is a date only — missing time component,
// violates _rfc3339 which requires the full date-time format.
// _badAssertionBadAsOf: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	AsOf:          "2024-01-15"
// }

// _badAssertionBadStatedAt: plain text is not an RFC3339 timestamp.
// _badAssertionBadStatedAt: #Assertion & {
// 	ID:            "assert-bad"
// 	PropositionID: "prop-001"
// 	Declarant:     "entity:alice"
// 	Stance:        "affirms"
// 	Content:       "I was there"
// 	StatedAt:      "yesterday"
// }
