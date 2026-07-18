// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
//
// Exercises:
//   - All three #Knowledge values: first_hand, reported, belief.
//   - Optional Role field (free-text, unconstrained).
//   - AsOf and StatedAt as _rfc3339-constrained strings.
package assertion

// A minimal Proposition (all required fields, no optional).
_prop1: #Proposition & {
	ID:        "prop-001"
	Canonical: "Alice was at The Tap Room on 2024-01-15 at 21:00 local time"
	Subject:   "entity:alice"
}

// A Proposition about an event rather than an entity.
_prop2: #Proposition & {
	ID:        "prop-002"
	Canonical: "The server logs show no access from IP 10.0.0.5 between 20:00 and 22:00"
	Subject:   "event:server-access-20240115"
}

// A minimal Assertion (all required fields, no optional provenance).
_assert1: #Assertion & {
	ID:            "assert-001"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "affirms"
	Content:       "I was at the bar at 9pm"
}

// An Assertion with a denying stance.
_assert2: #Assertion & {
	ID:            "assert-002"
	PropositionID: "prop-001"
	Declarant:     "entity:bob"
	Stance:        "denies"
	Content:       "I never saw her there that evening"
}

// A fully-provenanced Assertion: all optional fields populated.
// This is the shape that would pass ValidateAsEvidence() in Go.
// Note: Declarant ("entity:alice") is distinct from Author (the AI extractor).
_assert3: #Assertion & {
	ID:            "assert-003"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "affirms"
	Content:       "I was definitely at The Tap Room. I remember it clearly."

	AsOf:     "2024-01-15T21:00:00Z"
	StatedAt: "2024-03-10T09:30:00Z"
	StatedIn: "rec-depo-alice-20240310"

	Locator: {
		Kind:  "line"
		Value: "depo-alice.txt:142"
		Start: 142
		End:   145
	}

	Via: "testimonial"

	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "claude-sonnet-4-6"
			ModelVersion:          "20250514"
			PromptHash:            "sha256:promptabc123"
			PromptTemplateVersion: "v1.0.0"
			ParamsHash:            "sha256:paramsxyz789"
			InputDigest:           "sha256:inputdigest000"
			At:                    "2024-03-10T09:31:00Z"
		}
	}

	Grade: {
		Tier:       "testimonial"
		Confidence: 0.80
	}
}

// An Assertion with a human author — paralegal transcribing witness statement.
// Declarant is entity:carol; Author is the paralegal. Distinct roles.
_assert4: #Assertion & {
	ID:            "assert-004"
	PropositionID: "prop-002"
	Declarant:     "entity:carol"
	Stance:        "qualifies"
	Content:       "I think I saw her, but it might have been someone else"

	AsOf:     "2024-01-15T20:45:00Z"
	StatedAt: "2024-03-11T14:00:00Z"
	StatedIn: "rec-interview-carol-20240311"

	Locator: {
		Kind:  "page"
		Value: "3"
		Start: 0
		End:   0
	}

	Via: "testimonial"

	Author: {
		Kind:  "human"
		Human: {ID: "paralegal@lawfirm.example.com"}
	}

	Grade: {
		Tier:       "testimonial"
		Confidence: 0.45
	}
}

// A retraction: witness withdraws prior statement.
_assert5: #Assertion & {
	ID:            "assert-005"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "withdraws"
	Content:       "I wish to withdraw my earlier statement about that evening"
}

// A recantation: witness explicitly states prior statement was false.
_assert6: #Assertion & {
	ID:            "assert-006"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "recants"
	Content:       "My prior statement was incorrect. I was not at the bar."
}

// A does_not_recall stance.
_assert7: #Assertion & {
	ID:            "assert-007"
	PropositionID: "prop-001"
	Declarant:     "entity:dave"
	Stance:        "does_not_recall"
	Content:       "I cannot remember where I was that night"
}

// A declines stance (privilege invocation).
_assert8: #Assertion & {
	ID:            "assert-008"
	PropositionID: "prop-001"
	Declarant:     "entity:eve"
	Stance:        "declines"
	Content:       "I decline to answer on Fifth Amendment grounds"
}

// A first-hand knowledge assertion: declarant directly witnessed the fact.
_assert9: #Assertion & {
	ID:            "assert-009"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "affirms"
	Content:       "I saw it happen with my own eyes"
	Knowledge:     "first_hand"
	Role:          "witness"
}

// A reported (hearsay) assertion: declarant learned of the fact second-hand.
// This exercises the hearsay marker: Knowledge == "reported".
_assert10: #Assertion & {
	ID:            "assert-010"
	PropositionID: "prop-001"
	Declarant:     "entity:bob"
	Stance:        "affirms"
	Content:       "Alice told me she was there"
	Knowledge:     "reported"
	Role:          "bystander"
}

// A belief assertion: declarant holds the claim as inference rather than
// direct observation or reported statement.
_assert11: #Assertion & {
	ID:            "assert-011"
	PropositionID: "prop-001"
	Declarant:     "entity:carol"
	Stance:        "qualifies"
	Content:       "Based on what I know of her habits, I believe she was there"
	Knowledge:     "belief"
}

// A fully-provenanced assertion with Knowledge + Role: verifies that the
// new optional fields do not interfere with the evidence-gate shape.
// AsOf and StatedAt are RFC3339-constrained by _rfc3339 in schema.cue.
_assert12: #Assertion & {
	ID:            "assert-012"
	PropositionID: "prop-001"
	Declarant:     "entity:alice"
	Stance:        "affirms"
	Content:       "I was definitely at The Tap Room. I heard Bob say so too."

	AsOf:     "2024-01-15T21:00:00Z"
	StatedAt: "2024-03-10T09:30:00Z"
	StatedIn: "rec-depo-alice-20240310"

	Locator: {
		Kind:  "line"
		Value: "depo-alice.txt:200"
		Start: 200
		End:   203
	}

	Via: "testimonial"

	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "claude-sonnet-4-6"
			ModelVersion:          "20250514"
			PromptHash:            "sha256:promptabc123"
			PromptTemplateVersion: "v1.0.0"
			ParamsHash:            "sha256:paramsxyz789"
			InputDigest:           "sha256:inputdigest000"
			At:                    "2024-03-10T09:31:00Z"
		}
	}

	Grade: {
		Tier:       "testimonial"
		Confidence: 0.75
	}

	Knowledge: "reported"
	Role:      "witness"
}
