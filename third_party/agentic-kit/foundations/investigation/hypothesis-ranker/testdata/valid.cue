// valid.cue — concrete values that satisfy the #Hypothesis constraint defined
// in schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package hypothesisranker

// A hypothesis with high evidence support and full traceability.
_h1: #Hypothesis & {
	ID:        "h-ownership-gap"
	Statement: "Ownership gap in payments service causes incident escalation delay."
	Support:   0.87
	SpanRefs:  ["span-001", "span-002"]
	Status:    "supported"
	GradeTier: "documentary"
	Author: {
		Kind:       "human"
		Model:      ""
		PromptHash: ""
		InputDigest: ""
	}
}

// A hypothesis with medium support, no span refs yet (draft).
_h2: #Hypothesis & {
	ID:        "h-config-drift"
	Statement: "Config drift between staging and production causes intermittent failures."
	Support:   0.5
	SpanRefs:  []
}

// A hypothesis at minimum support boundary.
_h3: #Hypothesis & {
	ID:        "h-no-evidence"
	Statement: "External network issue is responsible for latency spikes."
	Support:   0.0
	SpanRefs:  []
}

// A hypothesis at maximum support boundary.
_h4: #Hypothesis & {
	ID:        "h-fully-supported"
	Statement: "Missing CODEOWNERS entry confirmed as root cause."
	Support:   1.0
	SpanRefs:  ["span-099"]
	Status:    "supported"
	GradeTier: "direct"
}

// A hypothesis with AI-derived support (Invariant 3: AI labelled).
_h5: #Hypothesis & {
	ID:        "h-ai-inferred"
	Statement: "Database connection pool exhaustion caused timeouts."
	Support:   0.61
	SpanRefs:  ["span-010", "span-011"]
	Status:    "open"
	GradeTier: "inferred"
	Author: {
		Kind:        "ai"
		Model:       "claude-sonnet-4"
		PromptHash:  "sha256:abc123"
		InputDigest: "sha256:def456"
	}
}

// A hypothesis with CreatedAt set (additive audit-trail field, added 2026-06-08).
_h6: #Hypothesis & {
	ID:        "h-with-timestamp"
	Statement: "Memory leak in caching layer caused OOM restarts."
	Support:   0.73
	SpanRefs:  ["span-020"]
	Status:    "open"
	CreatedAt: "2026-06-08T00:00:00Z"
}
