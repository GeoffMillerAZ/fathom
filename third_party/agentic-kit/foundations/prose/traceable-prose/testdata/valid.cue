// valid.cue — concrete values that satisfy the #TraceableNode and
// #RegenerationEvent constraints defined in schema.cue. Running
// `cue vet ./...` from the package root validates these against the schema.
package traceableprose

import "github.com/geoffmilleraz/agentic-kit/foundations/prose/traceable-prose:traceableprose"

// A non-redacted node with source refs.
_node1: traceableprose.#TraceableNode & {
	ID:                "node-001"
	Text:              "The payments service processes 10 000 requests per second."
	SourceRefs:        ["span-perf-001"]
	Redacted:          false
	RedactionReason:   ""
	NeedsRegeneration: false
}

// A redacted node — RedactionReason is required and non-empty.
_node2: traceableprose.#TraceableNode & {
	ID:                "node-002"
	Text:              "[REDACTED]"
	SourceRefs:        ["span-pii-001"]
	Redacted:          true
	RedactionReason:   "PII detected by DLP gate — prose-policy/restricted."
	NeedsRegeneration: false
}

// A node flagged for regeneration after a correction.
_node3: traceableprose.#TraceableNode & {
	ID:                "node-003"
	Text:              "The owner is the platform team."
	SourceRefs:        ["span-owner-002"]
	Redacted:          false
	RedactionReason:   ""
	NeedsRegeneration: true
}

// A regeneration event affecting two nodes.
_regen1: traceableprose.#RegenerationEvent & {
	TriggeredBy:     "correction-007"
	AffectedNodeIDs: ["node-003", "node-004"]
	OccurredAt:      "2026-06-05T10:00:00Z"
	Note:            "Evidence confidence dropped; nodes regenerated."
}
