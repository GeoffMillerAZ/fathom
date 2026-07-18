// Package entitycrosswalk defines the CUE-first contract for cross-source
// entity identity mappings. Go types in types.go and crosswalk.go mirror these
// definitions; any drift is a bug.
//
// Purpose: record confirmed identity mappings between canonical IDs and their
// aliases across source families. Every mapping carries full provenance (author
// + timestamp). The foundation is append-only: existing mappings are never
// overwritten.
package entitycrosswalk

import aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance:aiprovenance"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// _rfc3339 constrains a string to the RFC3339 timestamp format used by
// time.Time marshalling in Go (e.g. "2026-01-01T00:00:00Z"). The regex
// matches the minimal structure: date T time optional-fractional-seconds
// timezone. This mirrors the Go MappingEntry.RecordedAt field.
_rfc3339: string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"

// #MappingEntry is a single append-only record of a confirmed identity mapping.
// Every successful Add call produces one entry appended to the history.
#MappingEntry: {
	// CanonicalID is the authoritative entity identifier. Must be non-empty.
	CanonicalID: _nonEmpty

	// AliasID is the alternative identifier that resolves to CanonicalID.
	// Must be non-empty and must differ from CanonicalID.
	AliasID: _nonEmpty

	// Invariant: CanonicalID and AliasID must not be equal.
	// (Enforced in Go by Add; CUE expresses it as a structural note here.)
	AliasID: !=CanonicalID

	// Author records who or what recorded this mapping. For AI authors the full
	// reproduction key (ModelID, ModelVersion, PromptHash, InputDigest, At) is
	// required — this satisfies provenance invariant #1.
	Author: aiprovenance.#Author

	// RecordedAt is the RFC 3339 timestamp at which the mapping was appended.
	// Constrained to the RFC3339 format; drawn from the injected clock, never
	// time.Now() inline.
	RecordedAt: _rfc3339
}
