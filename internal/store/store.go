// Package store defines the persistence interface for sextant observations and
// the fingerprint (hash-gate) index. Two implementations are provided:
// DuckStore (DuckDB-backed, for production use) and MemoryStore (in-process
// fake, for fast unit tests and session-mode operation).
package store

import (
	"context"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// KnowledgeCard is an enrichment row produced for a single symbol. It mirrors
// the fields from internal/llm.KnowledgeCard but is defined here to avoid an
// import cycle (store must not import llm).
type KnowledgeCard struct {
	ServiceID string
	Symbol    string
	File      string
	Summary   string
	Tags      []string
}

// DocSummary is an enrichment row for a documentation source.
type DocSummary struct {
	ServiceID string
	SourceRef string
	Title     string
	Summary   string
}

// Store is the persistence contract for sextant's evidence layer.
// Implementations must be safe for concurrent reads; writes are serialised by
// the implementation (single writer pattern).
type Store interface {
	// PutObservations upserts a batch of observations. Implementations are
	// free to replace existing rows that share the same (service_id,
	// source_ref, symbol) triple, but at minimum they must not duplicate rows
	// on repeated calls for the same content.
	PutObservations(ctx context.Context, obs []core.Observation) error

	// QueryObservations returns all observations for a service. When concernID
	// is non-empty only observations tagged with that concern are returned;
	// when concernID is "" all observations for the service are returned.
	QueryObservations(ctx context.Context, serviceID, concernID string) ([]core.Observation, error)

	// Fingerprint returns the stored content hash for (serviceID, sourceRef).
	// The boolean is false when no fingerprint has been recorded yet.
	Fingerprint(ctx context.Context, serviceID, sourceRef string) (hash string, found bool, err error)

	// SetFingerprint records (or replaces) the content hash for (serviceID, sourceRef).
	SetFingerprint(ctx context.Context, serviceID, sourceRef, hash string) error

	// PurgeSource removes all observations whose (service_id, source_ref)
	// matches the given pair. It does NOT remove the fingerprint entry; callers
	// that want to force re-ingest should call SetFingerprint afterwards.
	PurgeSource(ctx context.Context, serviceID, sourceRef string) error

	// PutKnowledgeCards upserts a batch of knowledge-card enrichment rows.
	// Existing rows that share (service_id, symbol) are replaced.
	PutKnowledgeCards(ctx context.Context, cards []KnowledgeCard) error

	// KnowledgeCards returns all knowledge cards for serviceID in a
	// deterministic order (by symbol).
	KnowledgeCards(ctx context.Context, serviceID string) ([]KnowledgeCard, error)

	// PutDocSummaries upserts a batch of doc-summary enrichment rows.
	// Existing rows that share (service_id, source_ref) are replaced.
	PutDocSummaries(ctx context.Context, summaries []DocSummary) error

	// DocSummaries returns all doc summaries for serviceID in a deterministic
	// order (by source_ref).
	DocSummaries(ctx context.Context, serviceID string) ([]DocSummary, error)

	// ListServiceIDs returns the sorted, distinct set of service IDs present in
	// the observations table.  An empty store returns a nil slice with no error.
	ListServiceIDs(ctx context.Context) ([]string, error)

	// Close releases all resources held by the store. After Close returns the
	// store must not be used.
	Close() error
}
