// Package store defines the persistence interface for fathom observations and
// the fingerprint (hash-gate) index. Three implementations are provided:
// SQLiteStore (pure-Go, WAL-mode SQLite via modernc.org/sqlite — the default,
// ADR-020), DuckStore (DuckDB-backed, retained for migration until SS-8), and
// MemoryStore (in-process fake, for fast unit tests and session-mode
// operation). store.Open (or the lower-level store.BackendForPath) dispatches
// to SQLiteStore or DuckStore by file extension; callers that need a specific
// backend regardless of extension call store.NewSQLiteStore / store.OpenDuckDB
// directly.
package store

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
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

// Store is the persistence contract for fathom's evidence layer.
// Implementations must be safe for concurrent reads; writes are serialised by
// the implementation (single writer pattern).
type Store interface {
	// PutObservations appends a batch of observations with a bare INSERT.
	// This is NOT an upsert: neither backend deduplicates or replaces rows by
	// (service_id, source_ref, symbol) — calling it twice with the same
	// content produces duplicate rows. The purge-then-insert contract that
	// makes re-ingest behave like a replace is enforced by the caller
	// (internal/fusion calls PurgeSource immediately before PutObservations
	// on every changed file); this method alone provides no idempotency.
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

	// PurgeService removes every content row for serviceID — observations,
	// fingerprints, doc summaries, typed edges, and typed-edge package
	// fingerprints. Identity rows (service_meta, commits, knowledge cards)
	// are retained: the service survives, its content is rebuilt by the next
	// ingest. Used by the ADR-017 anchor-style self-heal.
	PurgeService(ctx context.Context, serviceID string) error

	// CountAbsoluteSourceRefs reports how many fingerprint rows for serviceID
	// carry an absolute source_ref — the pre-ADR-017 anchor style. A non-zero
	// count against a connector that now emits repo-relative refs means the
	// service's content must be purged and re-ingested wholesale (self-heal),
	// or every symbol would duplicate under both anchor styles.
	CountAbsoluteSourceRefs(ctx context.Context, serviceID string) (int, error)

	// DeleteFingerprint removes the fingerprint entry for (serviceID,
	// sourceRef). Deleting a missing fingerprint is a no-op, not an error.
	DeleteFingerprint(ctx context.Context, serviceID, sourceRef string) error

	// ListSourceRefs returns the sorted, distinct set of source_refs known to
	// the store for serviceID — the union of refs appearing in observations
	// and fingerprints (fingerprint-only refs exist for files with no
	// registered extractor). A service with no rows returns a nil slice with
	// no error.
	ListSourceRefs(ctx context.Context, serviceID string) ([]string, error)

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

	// PutClaims upserts semantic claims by (service_id, claim_id). For every
	// distinct claim_id that appears in links, the stored link set for that
	// claim is replaced (delete-then-insert). Empty inputs are a no-op.
	PutClaims(ctx context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error

	// ClaimsForService returns claims for serviceID. When layer is non-empty
	// only claims on that layer are returned; layer "" returns all layers.
	// Results are ordered deterministically by (layer, subject, claim_id).
	ClaimsForService(ctx context.Context, serviceID, layer string) ([]core.SemanticClaim, error)

	// ClaimLinks returns links for the given claim IDs within serviceID,
	// ordered by (claim_id, rel, target_ref). An empty claimIDs slice returns
	// nil with no error.
	ClaimLinks(ctx context.Context, serviceID string, claimIDs []string) ([]core.ClaimLink, error)

	// SetClaimStatus flips the status of a single claim. It is a no-op (no
	// error) when (service_id, claim_id) is absent.
	SetClaimStatus(ctx context.Context, serviceID, claimID string, status core.ClaimStatus) error

	// AppendClaimEvent appends one human/machine verdict to the claim's
	// append-only event log.
	AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error

	// ClaimEvents returns all events for claimID ordered by (at, verdict).
	ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error)

	// RecordCompileRun appends a cost/audit ledger row for one compilation
	// pass.
	RecordCompileRun(ctx context.Context, run core.CompileRun) error

	// PutTypedEdges replaces all typed call edges (ADR-021 §3,
	// internal/fusion.EnrichTypedEdges) for (serviceID, pkgPath) —
	// delete-then-insert, keyed at Go PACKAGE granularity so that reloading
	// one changed package replaces exactly its own edges, leaving edges from
	// unchanged (cache-hit) packages untouched. pkgPath is the enrichment
	// pass's stable package identifier (its Go import path). An empty edges
	// slice is a valid call and still clears any previously stored edges for
	// that package (e.g. every call in it was removed).
	PutTypedEdges(ctx context.Context, serviceID, pkgPath string, edges []core.Edge) error

	// TypedEdges returns every typed call edge stored for serviceID, ordered
	// by (from_symbol, to_symbol, kind) for determinism. A service with no
	// typed edges (never enriched, or not a Go service) returns a nil slice
	// with no error — this is the signal test_reaches and the MCP surfacing
	// tools use to fall back to syntax-only edges.
	TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error)

	// TypedEdgePackageFingerprint returns the content-hash fingerprint
	// recorded by the last successful typed-edge load of (serviceID,
	// pkgPath) — the per-package cache EnrichTypedEdges consults to skip
	// reloading unchanged packages. found is false when the package has
	// never been (successfully) loaded.
	TypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath string) (hash string, found bool, err error)

	// SetTypedEdgePackageFingerprint records or replaces the content-hash
	// fingerprint for (serviceID, pkgPath).
	SetTypedEdgePackageFingerprint(ctx context.Context, serviceID, pkgPath, hash string) error

	// CompileRuns returns compile runs for serviceID, optionally filtered by
	// layer (layer "" returns all layers), newest-first by RanAt then run_id.
	CompileRuns(ctx context.Context, serviceID, layer string) ([]core.CompileRun, error)

	// Close releases all resources held by the store. After Close returns the
	// store must not be used.
	Close() error
}
