// Package investigationjournal defines the CUE-first contracts for the
// investigation journal. Go types in types.go mirror these definitions; any
// drift is a bug.
package investigationjournal

import aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance:aiprovenance"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Entry is a single, immutable step recorded in the investigation journal.
// Entries are append-only — once written they are never modified.
#Entry: {
	// Step is the name or label of the investigation step (e.g. "hypothesis-formed").
	Step: _nonEmpty

	// Note is a human-readable description of what happened at this step.
	Note: _nonEmpty

	// At is the RFC 3339 timestamp of when this entry was recorded.
	At: _nonEmpty

	// Author records who or what produced this entry. Optional — system/internal
	// callers that record deterministic state (e.g. DAD loop phase transitions)
	// may omit it. When set, it must satisfy the #Author discriminated union.
	Author?: aiprovenance.#Author

	// SpanRefs holds IDs of evidence spans that this entry references.
	// Optional; each ID must be non-empty.
	SpanRefs?: [..._nonEmpty]
}
