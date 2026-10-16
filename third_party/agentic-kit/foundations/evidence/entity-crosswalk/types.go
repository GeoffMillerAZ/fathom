// Package entitycrosswalk maps entity identities across source families.
// A "canonical" ID is the authoritative identifier for a real-world entity;
// "aliases" are the alternative IDs that refer to the same entity in other
// source families (e.g. a repo ID vs. a service ID).
//
// This foundation is goroutine-safe: all exported methods acquire the
// appropriate mutex locks internally. The zero value of Crosswalk is NOT
// ready to use — construct one via New.
//
// Append-only invariant: once an alias is mapped to a canonical, that fact is
// never erased. Remapping an alias to a different canonical returns
// ErrAliasConflict; the old mapping is preserved unchanged. Call History() to
// inspect the full sequence of recorded mappings.
//
// Transitive resolution: Canonical() follows chains until it reaches a root
// (an ID that is not itself an alias). Callers do not need to loop.
//
// CUE contracts live in schema.cue; types here must stay in sync.
//
// Concurrency contract: Crosswalk is safe for concurrent use; all exported
// methods acquire a sync.RWMutex internally.
package entitycrosswalk

import (
	"errors"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Author is the canonical authorship record for a mapping entry.
// It is re-exported from foundations/provenance/ai-provenance so that
// callers only need to import this package.
//
// For AI authors the full reproduction key (ModelID, ModelVersion, PromptHash,
// InputDigest, At) is required — this satisfies invariant #1. Use
// aiprovenance.NewHuman or aiprovenance.NewAI to construct valid values.
//
// CUE mirror: #Author in schema.cue (which imports aiprovenance.#Author).
type Author = aiprovenance.Author

// ErrAliasConflict is returned by Add when the alias is already mapped to a
// different canonical ID. The existing mapping is left intact (append-only).
var ErrAliasConflict = errors.New("entitycrosswalk: alias already mapped to a different canonical")

// MappingEntry is a single append-only record of a confirmed identity mapping.
// Every successful Add call appends one entry to the Crosswalk's history.
//
// CUE mirror: #MappingEntry in schema.cue.
type MappingEntry struct {
	// CanonicalID is the authoritative entity identifier.
	CanonicalID string

	// AliasID is the alternative identifier that resolves to CanonicalID.
	AliasID string

	// Author records who or what recorded this mapping. Must satisfy
	// aiprovenance.Author.Validate() — for AI authors this includes the full
	// reproduction key required by provenance invariant #1.
	Author Author

	// RecordedAt is the wall-clock time at which the mapping was appended,
	// drawn from the Crosswalk's injected clock (never time.Now() directly).
	RecordedAt time.Time
}
