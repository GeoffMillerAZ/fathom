// valid.cue — concrete values that satisfy the #Span constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package evidencespan

// A well-formed span with high confidence.
_span1: #Span & {
	ID:         "span-001"
	EntityID:   "repo-payments"
	Field:      "language"
	Value:      "Go"
	Confidence: 0.95
	SourceID:   "src-code-payments"
}

// A span at minimum confidence boundary.
_span2: #Span & {
	ID:         "span-002"
	EntityID:   "svc-alpha"
	Field:      "status"
	Value:      "unknown"
	Confidence: 0.0
	SourceID:   "src-sme-interview"
}

// A span at maximum confidence boundary.
_span3: #Span & {
	ID:         "span-003"
	EntityID:   "repo-core"
	Field:      "owner"
	Value:      "platform-team"
	Confidence: 1.0
	SourceID:   "src-docs-codeowners"
}

// A span with an empty SourceID (field is not required to be non-empty).
_span4: #Span & {
	ID:         "span-004"
	EntityID:   "repo-core"
	Field:      "license"
	Value:      "MIT"
	Confidence: 0.8
	SourceID:   ""
}

// A fully-provenanced span: all new fields populated (AI author + explicit grade).
// This is the shape that would pass ValidateAsEvidence() in Go.
// Also exercises: OccurredAt and ObservedAt as valid RFC3339 strings (Finding 3),
// and Locator with Start==End==4821 satisfying the End>=Start constraint (Finding 1).
_span5: #Span & {
	ID:         "span-005"
	EntityID:   "payments-service"
	Field:      "error-rate"
	Value:      "elevated-2x"
	Confidence: 0.91
	SourceID:   "src-metrics-feed"

	OccurredAt: "2024-06-01T14:02:00Z"
	ObservedAt: "2024-06-01T14:05:00Z"
	RecordID:   "rec-metrics-feed-20240601"
	RecordHash: "sha256:abc123def456"

	Locator: {
		Kind:  "line"
		Value: "metrics.log:4821"
		Start: 4821
		End:   4821
	}

	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "claude-sonnet-4-6"
			ModelVersion:          "20250514"
			PromptHash:            "sha256:promptabc123"
			PromptTemplateVersion: "v1.0.0"
			ParamsHash:            "sha256:paramsxyz789"
			InputDigest:           "sha256:inputdigest000"
			At:                    "2024-06-01T14:05:01Z"
		}
	}

	Grade: {
		Tier:       "documentary"
		Confidence: 0.91
	}
}
