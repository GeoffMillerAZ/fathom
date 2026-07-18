// Package traceableprose defines the CUE-first contract for annotated, redact-
// able, and regeneration-tracked prose nodes. Go types in types.go mirror
// these definitions; any drift is a bug.
package traceableprose

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #TraceableNode is a prose node carrying full provenance: source evidence
// refs, redaction status, and a regeneration-needed flag.
#TraceableNode: {
	// ID uniquely identifies this node.
	ID: _nonEmpty

	// Text is the current rendered text of this node.
	Text: _nonEmpty

	// SourceRefs holds evidence-span or citation IDs that contributed to this
	// node. May be empty for non-evidentiary nodes.
	SourceRefs: [...string]

	// Redacted indicates whether this node has been redacted.
	Redacted: bool

	// RedactionReason is required when Redacted is true.
	// The CUE constraint below enforces this dependency.
	RedactionReason: string

	// NeedsRegeneration is true when an upstream correction has invalidated
	// this node.
	NeedsRegeneration: bool

	// Constraint: when Redacted is true, RedactionReason must be non-empty.
	if Redacted == true {
		RedactionReason: _nonEmpty
	}
}

// #RegenerationEvent records a single regeneration of a set of prose nodes.
#RegenerationEvent: {
	// TriggeredBy identifies the correction or evidence-update that caused
	// this regeneration.
	TriggeredBy: _nonEmpty

	// AffectedNodeIDs lists the IDs of nodes that were regenerated.
	AffectedNodeIDs: [_nonEmpty, ..._nonEmpty]

	// OccurredAt is an RFC 3339 timestamp string for the regeneration.
	OccurredAt: string & !=""

	// Note is an optional human-readable description.
	Note: string
}
