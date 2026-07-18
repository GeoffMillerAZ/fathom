// valid.cue — concrete values that satisfy the #Span and #Edge constraints
// defined in schema.cue. Running `cue vet ./...` from the package root or the
// repo root validates these against the schema.
package evidencegraph

// A well-formed span: confidence is within the required [0, 1] interval.
_span1: #Span & {
	ID:         "span-001"
	EntityID:   "repo-payments"
	Field:      "language"
	Value:      "Go"
	Confidence: 0.95
	Source:     "code/payments/main.go"
	ExpiresAt:  "2099-01-01T00:00:00Z"
}

// A second span with a different field on the same entity.
_span2: #Span & {
	ID:         "span-002"
	EntityID:   "repo-payments"
	Field:      "owner"
	Value:      "platform-team"
	Confidence: 0.8
	Source:     "docs/CODEOWNERS"
	ExpiresAt:  "2099-06-01T00:00:00Z"
}

// A minimum-confidence span to exercise the lower boundary.
_span3: #Span & {
	ID:         "span-003"
	EntityID:   "svc-alpha"
	Field:      "status"
	Value:      "unknown"
	Confidence: 0.0
	Source:     "sme/interview-2024-01"
	ExpiresAt:  ""
}

// A well-formed edge linking span-001 to span-002.
_edge1: #Edge & {
	From: "span-001"
	To:   "span-002"
	Rel:  "corroborates"
}
