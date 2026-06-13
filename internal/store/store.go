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

// ServiceMeta records per-service ingest metadata: the repo path and the git
// commit that was HEAD at ingest time. Both may be empty strings when the
// service has no git repo or git is unavailable.
type ServiceMeta struct {
	ServiceID     string
	Path          string // filesystem path used at ingest time
	IndexedCommit string // git HEAD at ingest time ("" if unavailable)
	IndexedAt     int64  // Unix nano
}

// Commit is a single git commit record for a service.
type Commit struct {
	ServiceID string
	Hash      string // full 40-char SHA
	Author    string
	Date      int64    // Unix nano
	Message   string   // subject line
	Files     []string // files touched
	SpecIDs   []string // spec IDs extracted from message
}

// CommitInfo is a presentation-layer view of a git commit for query/MCP results.
type CommitInfo struct {
	Hash              string
	Date              string // RFC3339
	Author            string
	Message           string
	Files             []string // used for filtering; not serialised in output
	FilesTouchedCount int
	SpecIDs           []string
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

	// SetServiceMeta upserts the ingest metadata for serviceID. Implementations
	// must be idempotent: repeated calls for the same serviceID replace the row.
	SetServiceMeta(ctx context.Context, meta ServiceMeta) error

	// GetServiceMeta returns the ingest metadata for serviceID.
	// found is false when no metadata has been recorded for that service.
	GetServiceMeta(ctx context.Context, serviceID string) (meta ServiceMeta, found bool, err error)

	// SetCommits replaces all commits for serviceID with the provided slice.
	// Idempotent: repeated calls replace the stored set.
	SetCommits(ctx context.Context, serviceID string, commits []Commit) error

	// ListCommits returns commits for serviceID newest-first (by Date desc).
	// Returns nil slice with no error when no commits are stored.
	ListCommits(ctx context.Context, serviceID string) ([]Commit, error)

	// Close releases all resources held by the store. After Close returns the
	// store must not be used.
	Close() error
}
