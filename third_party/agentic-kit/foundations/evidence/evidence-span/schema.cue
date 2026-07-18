// Package evidencespan defines the CUE-first contract for the atomic evidence
// unit. Go types in types.go mirror these definitions; any drift is a bug.
package evidencespan

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _rfc3339 constrains a string to the RFC3339 timestamp format used by
// time.Time marshalling in Go (e.g. "2024-06-01T14:02:00Z"). The regex
// matches the minimal structure: date T time optional-fractional-seconds
// timezone. This mirrors the Go types.OccurredAt / ObservedAt fields.
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #LocatorKind enumerates the valid discriminator values for #Locator.
#LocatorKind: "page" | "line" | "char-range" | "byte-range" | "cell" | "timestamp" | "xpath" | "none"

// #Locator is a structured pointer into a source document for click-traceability.
// Kind must be one of the recognised values when present.
// Start and End are coordinate offsets in the kind's space; End >= Start when both nonzero.
// When both Start and End are nonzero, End must be >= Start (same rule enforced
// by Go's Locator.Validate; CUE is the source of truth for shape so this
// constraint must live here too — drift is a build failure).
#Locator: {
	// Kind discriminates the locator type.
	Kind: #LocatorKind

	// Value is the human- and machine-readable location (e.g. "42", "xpath://div").
	Value: string

	// Start is the inclusive start offset (0 = unset).
	Start: int & >=0

	// End is the inclusive end offset; must be >= Start when both nonzero.
	// CUE constraint: End >= Start (handles the nonzero case because Start >= 0
	// means End >= Start is always satisfiable when Start == 0).
	End: int & >=Start
}

// #Span is the atomic evidence unit. It records a single claimed value for a
// (EntityID, Field) pair together with provenance and a confidence score.
//
// New fields (OccurredAt, ObservedAt, RecordHash, Locator, Author, Grade) are
// optional for backward compatibility. Use ValidateAsEvidence() in Go for full
// provenance enforcement before persisting to the ledger.
#Span: {
	// ID uniquely identifies this span.
	ID: _nonEmpty

	// EntityID is the identifier of the real-world entity this span describes.
	EntityID: _nonEmpty

	// Field names the attribute being claimed (e.g. "owner", "language").
	Field: _nonEmpty

	// Value is the claimed value for EntityID.Field.
	Value: _nonEmpty

	// Confidence is the certainty of this claim in the closed interval [0, 1].
	// 0 = no confidence, 1 = fully verified.
	Confidence: >=0 & <=1

	// SourceID identifies the originating source registered in source-registry.
	SourceID: string

	// OccurredAt is the world-time (RFC3339) when the event/fact happened. Optional.
	// When present, must be a valid RFC3339 timestamp (mirrors Go time.Time marshalling).
	OccurredAt?: _rfc3339

	// ObservedAt is the record-time (RFC3339) when this span was ingested. Optional.
	// When present, must be a valid RFC3339 timestamp (mirrors Go time.Time marshalling).
	ObservedAt?: _rfc3339

	// RecordID is the identifier of the source record this span was extracted from;
	// pair with RecordHash to detect record changes via record-fingerprint. Optional.
	RecordID?: string

	// RecordHash is the content hash of the source record this span was extracted from. Optional.
	RecordHash?: string

	// Locator is a structured pointer into the source for click-traceability. Optional.
	Locator?: #Locator

	// Author records who or what produced this span (human or AI). Optional.
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

	// Grade is the explicit epistemic tier + confidence. Optional (nil = legacy Confidence).
	Grade?: {
		// Tier is one of the five recognised epistemic tiers.
		Tier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"
		// Confidence is the certainty in [0, 1].
		Confidence: >=0 & <=1
	}
}
