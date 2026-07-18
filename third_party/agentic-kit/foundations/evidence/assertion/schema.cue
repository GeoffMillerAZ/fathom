// Package assertion defines the CUE-first contract for the speech-act
// primitive. Go types in types.go and stance.go mirror these definitions;
// any drift is a bug.
//
// An Assertion separates WHAT is claimed (Proposition) from WHO asserted it,
// how, and when (Assertion). This enables representing "who said what when",
// witness story-changes, and impeachment in a legal/investigation engine.
package assertion

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _rfc3339 constrains a string to the RFC3339 timestamp format used by
// time.Time marshalling in Go (e.g. "2024-06-01T14:02:00Z"). The regex
// matches the minimal structure: date T time optional-fractional-seconds
// timezone. Mirrors the identical alias in foundations/evidence/evidence-span/schema.cue.
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #Stance enumerates the valid speech-act relationships between a declarant
// and a proposition.
#Stance: "affirms" | "denies" | "qualifies" | "withdraws" | "recants" | "does_not_recall" | "declines"

// #Knowledge enumerates how the declarant came to know the asserted fact.
// "reported" is the hearsay marker: the declarant learned of the fact through
// another person's statement rather than direct observation.
#Knowledge: "first_hand" | "reported" | "belief"

// #Proposition is the normalized, subject-anchored statement that many
// declarants can each assert with different stances. It separates the
// semantic content of a claim from its speech-act wrapper.
#Proposition: {
	// ID uniquely identifies this proposition.
	ID: _nonEmpty

	// Canonical is the normalized statement text.
	Canonical: _nonEmpty

	// Subject is the EntityID or EventID this proposition is about.
	Subject: _nonEmpty
}

// #Assertion is the speech-act record: a specific declarant taking a stance
// on a proposition at a particular time, captured from a particular source.
//
// Design invariant: Declarant (the real-world person who spoke) is distinct
// from Author (who or what recorded this assertion into the system). Both
// must be present for full provenance. In Go, ValidateAsEvidence() enforces
// the strict provenance gate; Validate() is the lenient gate.
#Assertion: {
	// ID uniquely identifies this assertion record.
	ID: _nonEmpty

	// PropositionID references the Proposition being addressed.
	PropositionID: _nonEmpty

	// Declarant is the EntityID of the real-world person or party who made
	// the statement. Distinct from Author.
	Declarant: _nonEmpty

	// Stance is the declarant's relationship to the proposition.
	Stance: #Stance

	// Content is the actual asserted value or verbatim text.
	Content: _nonEmpty

	// AsOf is the world-time (RFC3339) when the asserted fact is claimed to
	// be true. Optional for backward compat; required by ValidateAsEvidence.
	AsOf?: _rfc3339

	// StatedAt is the record-time (RFC3339) when the declarant said it.
	// Optional for backward compat; required by ValidateAsEvidence.
	StatedAt?: _rfc3339

	// StatedIn is the RecordID of the source record containing this statement.
	// Optional for backward compat; required by ValidateAsEvidence.
	StatedIn?: string

	// Locator is an exact pointer into the StatedIn record.
	// Optional for backward compat; required by ValidateAsEvidence (non-"none").
	Locator?: {
		// Kind discriminates the locator type.
		Kind: "page" | "line" | "char-range" | "byte-range" | "cell" | "timestamp" | "xpath" | "none"
		// Value is the human- and machine-readable location.
		Value: string
		// Start is the inclusive start offset (0 = unset).
		Start: int & >=0
		// End is the inclusive end offset; must be >= Start when both nonzero.
		End: int & >=0
	}

	// Via is the evidence channel tier for this assertion.
	// Optional for backward compat; required by ValidateAsEvidence.
	Via?: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"

	// Author records who or what recorded this assertion into the system.
	// Optional for backward compat; required by ValidateAsEvidence.
	// Mirrors aiprovenance.Author; inlined here for CUE schema self-containment.
	Author?: {
		// Kind must be "human" or "ai".
		Kind: "human" | "ai"
		// Human is populated when Kind == "human".
		Human?: {ID: _nonEmpty}
		// AI is populated when Kind == "ai".
		AI?: {
			ModelID:               _nonEmpty
			ModelVersion:          _nonEmpty
			PromptHash:            _nonEmpty
			PromptTemplateVersion: string
			ParamsHash:            string
			InputDigest:           _nonEmpty
			At:                    string
		}
	}

	// Grade is the explicit epistemic tier + confidence. Optional (lenient);
	// required by ValidateAsEvidence.
	Grade?: {
		// Tier is one of the five recognised epistemic tiers.
		Tier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"
		// Confidence is the certainty in [0, 1].
		Confidence: >=0 & <=1
	}

	// Knowledge classifies how the declarant came to know the asserted fact.
	// Optional. When present must be one of #Knowledge.
	// "reported" is the hearsay marker; use IsHearsay() in Go to test.
	Knowledge?: #Knowledge

	// Role is a free-text procedural posture populated by the application layer
	// (e.g. the legal pack). The kit does not enumerate legal roles.
	// Optional and unconstrained.
	Role?: string
}
