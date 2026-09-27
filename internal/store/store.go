// Package store defines the persistence interface for fathom observations and
// the fingerprint (hash-gate) index. Two implementations are provided:
// SQLiteStore (pure-Go, WAL-mode SQLite via modernc.org/sqlite — the only
// on-disk backend, ADR-020) and MemoryStore (in-process fake, for fast unit
// tests and session-mode operation). The DuckDB backend that predated
// SQLiteStore was deleted outright in SS-8 once the fleet finished migrating
// (docs/plans/store-sqlite.md; ADR-025) — CGO is no longer a build
// requirement anywhere in this package. store.Open (or the lower-level
// store.BackendForPath) dispatches to SQLiteStore by file extension
// (".sqlite"/".db"); callers that need the concrete type directly call
// store.NewSQLiteStore.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ServiceNotFoundError is RenameService's refusal when the source serviceID
// has no rows anywhere in the store (judged against service_meta,
// observations and semantic_claims) — a typed error a caller switches on
// with errors.As, never a prose string to substring-match (the same
// named-refusal stance as ConformanceError).
type ServiceNotFoundError struct {
	ServiceID string
}

func (e *ServiceNotFoundError) Error() string {
	return fmt.Sprintf("store: service %q has no rows (nothing to rename)", e.ServiceID)
}

// ServiceExistsError is RenameService's refusal when the target serviceID
// already holds ANY row — renaming onto a live service would silently merge
// two corpora, so the collision is refused loudly instead.
type ServiceExistsError struct {
	ServiceID string
}

func (e *ServiceExistsError) Error() string {
	return fmt.Sprintf("store: service %q already has rows (refusing to rename onto it)", e.ServiceID)
}

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
	Date      int64 // Unix nano
	// Message is the FULL raw commit message — subject, body, and trailers
	// (e.g. "Co-Authored-By:") — as captured by `git log --format=%B`
	// (E2-1: widened from subject-only capture; a legacy row written before
	// this change may still hold just a subject and backfills to the full
	// body on the next ingest, since SetCommits replaces a service's whole
	// commit set). The underlying store column is unbounded TEXT/VARCHAR in
	// every backend, so no schema migration was needed to hold a multi-line
	// value — only the git-log format and parser needed to widen.
	Message string
	Files   []string // files touched
	SpecIDs []string // spec IDs extracted from the message (subject + body)
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

// TestSuite is one declared suite from a service's tests/<component>.cue
// manifest (vision 33 / ADR-043 stage 0, TC-2) — the facts a manifest
// asserts about a suite, independent of any run. These tables are
// ADDITIVE and OPTIONAL: they are not part of store-format-v0's required
// L0/Writer schema (conformance.go's requiredSchemaV0) — a store with no
// test evidence ingested simply has empty tables.
type TestSuite struct {
	ServiceID string
	Component string
	Suite     string
	Class     string
	Tier      string
	Port      string
	// DeclaredCertifiesJSON is the suite's declared #Certifies entries
	// (design doc §1), JSON-encoded — always "[]" this round: test-kit's
	// #Suite.certifies field is a stage-1 ADR-0042 amendment (plan row
	// TC-17) that has not landed upstream yet. The column exists now so
	// stage 1 needs no migration to start reading it.
	DeclaredCertifiesJSON string
	// Watermark is the service's ingest watermark this suite was observed
	// under (every row carries it — design doc §2).
	Watermark string
}

// TestCase is one case within a TestRun (vision 33 stage 0, TC-2/TC-3).
type TestCase struct {
	CaseID     string
	Name       string
	Status     string
	Attempts   int
	DurationMS int64
	// FailureJSON is "" when the case carried no #CaseFailure.
	FailureJSON string
	// TestSymbolKey is "" when JoinCasesToSymbols (internal/testevidence)
	// could not join this case to exactly one Test* symbol — kept, never
	// dropped; the join rate is a reported fact, never a pass/fail (design
	// doc §2, TC-3).
	TestSymbolKey string
}

// TestRun is one ingested #TestResult envelope (vision 33 stage 0, TC-2).
type TestRun struct {
	ServiceID string
	RunDigest string
	Component string
	Suite     string
	// StartedAt is RFC3339, "" when the envelope's timing.startedAt was
	// absent.
	StartedAt  string
	Verdict    string
	TotalsJSON string
	// CoverageJSON is "" when the envelope carried no coverage report.
	CoverageJSON string
	// SignerTier is the DSSE envelope's own tier field ("unsigned" |
	// "local" | "cosign-keyless") when the run arrived wrapped, "" for a
	// bare envelope — fathom records this as a fact and never attempts
	// signature verification itself (design doc §2).
	SignerTier string
	Watermark  string
	Cases      []TestCase

	// Commit, Dirty and Race are CV-2's own facts (design doc §1.1,
	// O114, vision 34 / ADR-044 stage 1): the git commit the run was
	// attested against, whether the working tree carried uncommitted
	// changes at that moment, and whether the race detector was on.
	// Commit is "" (Dirty/Race both false) for a run that never stated
	// them — an envelope from before this field existed, or from an
	// adapter nobody told — never a fabricated commit;
	// internal/certify's certificate ladder (CertificateStateAt) treats
	// an empty Commit as "not bound to a commit."
	Commit string
	Dirty  bool
	Race   bool
}

// TestSuiteStatus is the per-suite read `fathom tests status` (TC-4)
// renders: a declared suite plus its most-recently-ingested run (by
// StartedAt, falling back to insertion order when StartedAt ties or is
// unset) and that run's join stats.
type TestSuiteStatus struct {
	TestSuite
	HasRun      bool
	LastRun     TestRun // zero value when HasRun is false
	CasesTotal  int
	CasesJoined int
}

// CertifyDeclaration is one declared certification fact read from a
// `// certifies:` marker above a Test* function (vision 33 stage 0, TC-5).
// This table is ADDITIVE and OPTIONAL, same conformance note as TestSuite.
type CertifyDeclaration struct {
	ServiceID     string
	TestSymbolKey string
	Kind          string
	TargetID      string
	Strength      string
	File          string
	Line          int
	Watermark     string
}

// TestCaseRun is one historical case-status observation for a Test*
// symbol, across attested runs (vision 33 stage 1, TC-6/TC-8): the read
// internal/certify.CompileCertifications/SweepCertifications use to
// derive a certificate's STATE — the last attested run's status/attempts,
// and a flake within the last three attested runs (design doc §3). It is
// the same test_cases/test_runs join TestSuiteStatuses' own LastRun read
// performs, but keyed by TEST SYMBOL rather than by suite, and carrying
// every historical run rather than only the newest.
type TestCaseRun struct {
	RunDigest  string
	Suite      string
	Component  string
	StartedAt  string
	Status     string
	Attempts   int
	IngestedAt int64

	// Commit, Dirty and Race mirror the owning TestRun's own CV-2 facts
	// — carried onto this per-symbol read because internal/certify's
	// certificate ladder (CertificateStateAt, CV-3) needs them to decide
	// whether a run is bound to a commit, without a second read back to
	// TestRun for every run TestCaseRunsForSymbol already returns.
	Commit string
	Dirty  bool
	Race   bool
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

	// SymbolNamesForSource returns the distinct, sorted symbol names the
	// service's observations declare in file — a source_ref equal to file
	// or file followed by ":<line>". It reads only that file's rows (the
	// (service_id, source_ref) index), never the service's whole
	// observation set: the obligations raw fallback's per-edit relevance
	// signal (RT-81, docs/design/responsible-tenant.md §4.3).
	SymbolNamesForSource(ctx context.Context, serviceID, file string) ([]string, error)

	// SymbolSourcesForNames returns the (package, symbol, source_ref) rows
	// of the service's observations that declare any of names, sorted by
	// (symbol, package, source_ref) — only those rows and columns, never
	// the service's whole observation set: the resolution a symbol-only
	// plan-row contract needs at obligations serve time (RT-174,
	// docs/design/responsible-tenant.md §4.3).
	SymbolSourcesForNames(ctx context.Context, serviceID string, names []string) ([]SymbolSource, error)

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

	// RemoveService deletes EVERY row for serviceID: the PurgeService content
	// set PLUS the identity rows PurgeService deliberately keeps
	// (service_meta, commits, knowledge cards), the semantic layer
	// (semantic_claims, claim_links, compile_runs), and the claim_events
	// rows whose claim_id belongs to the service's semantic_claims —
	// claim_events carries no service_id column (see ClaimEventsSince), so
	// membership is resolved through the claims table before the claims
	// themselves are deleted. removed is false, with no error, when the
	// service had no rows anywhere — the call is idempotent. This is
	// registry surgery for duplicate/malformed service rows — the
	// `../geoffmiller-cloud` twin (USAGE_JOURNAL 2026-06-16) and the O1
	// shadow-service repair (.fathom/observations.md, fix (d) context) both
	// previously required hand-run sqlite against the store file under the
	// daemon lock.
	RemoveService(ctx context.Context, serviceID string) (removed bool, err error)

	// RenameService moves every service_id-scoped row from oldID to newID
	// across all eleven service_id-carrying tables (observations,
	// fingerprints, knowledge_cards, doc_summaries, service_meta, commits,
	// semantic_claims, claim_links, compile_runs, typed_edges,
	// typed_edge_fingerprints) in one atomic step. It refuses with a typed
	// error — *ServiceNotFoundError when oldID has no rows,
	// *ServiceExistsError when newID already holds ANY row — leaving the
	// store untouched; presence for both checks is judged against
	// service_meta, observations and semantic_claims. claim_events rows are
	// UNTOUCHED and claim IDs keep their minted values: a claim ID is
	// historical evidence, content-addressed at compile time under the old
	// name, and rewriting it would orphan the append-only event log it
	// anchors. New compiles simply mint fresh IDs under the new name.
	RenameService(ctx context.Context, oldID, newID string) error

	// CountAbsoluteSourceRefs reports how many fingerprint rows for serviceID
	// carry an absolute source_ref — the pre-ADR-017 anchor style. A non-zero
	// count against a connector that now emits repo-relative refs means the
	// service's content must be purged and re-ingested wholesale (self-heal),
	// or every symbol would duplicate under both anchor styles.
	CountAbsoluteSourceRefs(ctx context.Context, serviceID string) (int, error)

	// ServiceContentDigest is a digest of serviceID's recorded content —
	// every (source_ref, content hash) fingerprint, in source_ref order. It
	// moves when an ingest changed, added or removed a file and stays put
	// across a hash-gated no-op re-ingest (whose service_meta.indexed_at
	// still moves), so a reader keyed on it sees content changes only. A
	// service with no fingerprints digests the empty set.
	ServiceContentDigest(ctx context.Context, serviceID string) (string, error)

	// DeleteFingerprint removes the fingerprint entry for (serviceID,
	// sourceRef). Deleting a missing fingerprint is a no-op, not an error.
	DeleteFingerprint(ctx context.Context, serviceID, sourceRef string) error

	// SourceKindCounts returns serviceID's observation counts grouped by
	// (source_ref, source_kind), sorted by (SourceRef, Kind) — the
	// aggregate list_services answers symbol counts and languages from
	// (RT-144) without loading a single observation row. Filtered by
	// suppression like QueryObservations: a suppressed file's symbols are
	// not counted.
	SourceKindCounts(ctx context.Context, serviceID string) ([]SourceKindCount, error)

	// SourcePackages returns the distinct packages of serviceID's
	// observations anchored at sourceRef, sorted (RT-27: the refresh-file
	// route's package lookup, one file's rows, never the whole table).
	// Filtered by suppression: a suppressed file names no package.
	SourcePackages(ctx context.Context, serviceID, sourceRef string) ([]string, error)

	// PackageObservations returns serviceID's observations whose package is
	// one of pkgs, in (source_ref, line, symbol) order — a package's
	// complete node set without loading any other package's rows (RT-27).
	// Filtered by suppression on source_ref.
	PackageObservations(ctx context.Context, serviceID string, pkgs []string) ([]core.Observation, error)

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

	// ClaimServiceIDs returns the sorted, distinct set of service IDs present
	// in the semantic_claims table — the claims-only counterpart to
	// ListServiceIDs (which scans observations only). A claims-only corpus (a
	// findings-style expert module with claims but no code observations at
	// all — docs/design/expert-modules.md) is otherwise invisible to a caller
	// that discovers services purely via ListServiceIDs (docs/plans/
	// gke-module-host.md's module-host federation layer is exactly such a
	// caller). An empty store returns a nil slice with no error.
	ClaimServiceIDs(ctx context.Context) ([]string, error)

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

	// SetClaimValidTo closes a single claim's bi-temporal validity window at
	// t (SU-19, ADR-041 decision 7, core.SemanticClaim.ValidTo) — called by
	// every path that ends a claim's validity: a native review/compile
	// reconciliation superseding it, an external closure/revocation, or a
	// human `fathom claim refute`. A no-op (no error) when (service_id,
	// claim_id) is absent, mirroring SetClaimStatus.
	SetClaimValidTo(ctx context.Context, serviceID, claimID string, t time.Time) error

	// AppendClaimLink appends ONE link row to claim_id's link set without
	// touching any of its existing links (SU-20, ADR-041 decision 7) —
	// unlike PutClaims, whose link-writing side REPLACES a claim's whole
	// link set wholesale for every claim_id present in the links it is
	// given. Used for a "supersedes" link written once the successor is
	// known, AFTER the successor claim's own mint already wrote its
	// cites/scopes/scope-fp links via PutClaims — a PutClaims wholesale
	// replace at that point would silently drop them.
	AppendClaimLink(ctx context.Context, serviceID string, link core.ClaimLink) error

	// UpdateClaimLink re-anchors a single link in place: it rewrites
	// TargetRef/TargetHash for the row matching (service_id, claim_id, rel,
	// target_ref=oldTargetRef) to (newTargetRef, newTargetHash), leaving
	// AnchorText and every other link untouched. This is the sweep's
	// citation-drift heal (T5-1, O13, docs/plans/trust-and-tokenomics.md):
	// when an upstream edit shifts a cited line range but the claim's stored
	// anchor text still resolves at exactly one new location in the same
	// file, the sweep moves the citation instead of flipping the claim
	// stale. A no-op (no error) when no row matches oldTargetRef — the
	// sweep only calls this immediately after reading the link via
	// ClaimLinks, so a miss here means a concurrent write raced it, not a
	// normal path worth erroring over.
	UpdateClaimLink(ctx context.Context, serviceID, claimID, rel, oldTargetRef, newTargetRef, newTargetHash string) error

	// AppendClaimEvent appends one human/machine verdict to the claim's
	// append-only event log.
	AppendClaimEvent(ctx context.Context, ev core.ClaimEvent) error

	// ClaimEvents returns all events for claimID ordered by (at, verdict).
	ClaimEvents(ctx context.Context, claimID string) ([]core.ClaimEvent, error)

	// ClaimEventsSince (Q-6, docs/plans/expert-suite-q.md) returns every
	// event with At in [since, until], ordered by (At, Verdict) — the same
	// deterministic order ClaimEvents already uses. Unlike ClaimEvents,
	// this is a BULK, time-windowed read across every claim, not one
	// claim's history — added for `fathom metrics interventions` (Q-12),
	// which needs "how many findings were superseded this window" without
	// enumerating every claim ID first. claim_events carries no
	// service_id column (see the CREATE TABLE shape), so this read is
	// honestly store-wide, never service-scoped — a caller needing
	// per-service isolation must filter claimEvents.ClaimID against its
	// own service's claim set separately.
	ClaimEventsSince(ctx context.Context, since, until time.Time) ([]core.ClaimEvent, error)

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

	// ListTypedEdgePackages returns the sorted, distinct set of typed-edge
	// package IDs known to the store for serviceID — the union of pkg_paths
	// appearing in typed_edges and typed_edge_fingerprints (a package can be
	// fingerprint-only when its load emitted zero edges, or edge-only when a
	// fingerprint write failed mid-run). EnrichTypedEdges' post-load
	// reconcile diffs this against the currently loaded package set to purge
	// rows for packages that no longer exist (deleted or moved dirs) —
	// without it, a stale edge row pointing at a still-live SymbolKey would
	// survive re-ingest forever as fabricated reachability evidence. A
	// service with no rows returns a nil slice with no error.
	ListTypedEdgePackages(ctx context.Context, serviceID string) ([]string, error)

	// DeleteTypedEdgePackage removes all typed-edge rows AND the
	// package-fingerprint row for (serviceID, pkgPath). Deleting a missing
	// package is a no-op, not an error.
	DeleteTypedEdgePackage(ctx context.Context, serviceID, pkgPath string) error

	// SetTypedEdgeStatus records or replaces the persisted typed-edge pass
	// status for serviceID (DG-10) — the store-backed counterpart to
	// internal/fusion.Pipeline's in-memory typedEdgeStatus map, so a
	// daemon restart does not make the fleet's typed-edge health
	// unobservable purely from restart timing.
	SetTypedEdgeStatus(ctx context.Context, serviceID string, rec TypedEdgeStatusRecord) error

	// TypedEdgeStatus returns the last persisted typed-edge status for
	// serviceID. found is false when none has ever been recorded.
	TypedEdgeStatus(ctx context.Context, serviceID string) (rec TypedEdgeStatusRecord, found bool, err error)

	// CompileRuns returns compile runs for serviceID, optionally filtered by
	// layer (layer "" returns all layers), newest-first by RanAt then run_id.
	CompileRuns(ctx context.Context, serviceID, layer string) ([]core.CompileRun, error)

	// PutExternalFinding upserts the findings-loop's per-claim cursor row
	// (plan FL-7, vision 30 §3.3 item 4) keyed by (f.ServiceID, f.ClaimID) —
	// the row the external import path (internal/findings/external) reads
	// back on the NEXT import of the same claim to decide stale/duplicate/
	// conflicting-replay, and that obligations serving joins onto a claim
	// whose PromptVersion is "external-import/v1" for source-linked
	// provenance (producer, rule, source id/version/revision, anchor).
	PutExternalFinding(ctx context.Context, f core.ExternalFinding) error

	// ExternalFinding returns the cursor row for (serviceID, claimID). found
	// is false when no such row exists.
	ExternalFinding(ctx context.Context, serviceID, claimID string) (core.ExternalFinding, bool, error)

	// ExternalFindingsForService returns every cursor row for serviceID,
	// sorted by claim_id.
	ExternalFindingsForService(ctx context.Context, serviceID string) ([]core.ExternalFinding, error)

	// RecordReviewRun upserts one `fathom review --idempotency-key` outcome
	// (FL-15, docs/plans/findings-loop.md) keyed by (service_id,
	// idempotency_key). A second call with the same key REPLACES the row —
	// the caller (cmd/fathom's idempotency check) only ever calls this after
	// confirming no prior row exists for that key, or with an identical
	// PayloadDigest, so the replace is never observable as a silent
	// overwrite of a conflicting request.
	RecordReviewRun(ctx context.Context, run core.ReviewRun) error

	// ReviewRun returns the recorded review run for (serviceID,
	// idempotencyKey). found is false when no run was ever recorded under
	// that key.
	ReviewRun(ctx context.Context, serviceID, idempotencyKey string) (core.ReviewRun, bool, error)

	// Close releases all resources held by the store. After Close returns the
	// store must not be used.
	Close() error

	// ClaimsForServe (SU-2, docs/plans/substrate.md §A, ADR-041 decision 3)
	// is ClaimsForService's prefiltered sibling: claims are narrowed by
	// layer, status and scope prefix in the BACKEND (SQL WHERE for
	// SQLiteStore, a Go loop for MemoryStore) instead of every caller
	// loading a service's whole claim set and filtering in Go.
	//
	//   - layers: when non-empty, only claims whose Layer is in this set.
	//     Empty/nil means every layer (ClaimsForService's layer="" behavior).
	//   - statuses: when non-empty, only claims whose Status is in this
	//     set. Empty/nil means every status.
	//   - scopePrefixes: a DELIBERATE OVER-APPROXIMATION, never an exact
	//     scope match — a claim is kept when any of its scope globs (a)
	//     is unanchored/wildcard-rooted ("*", "**", or "**/…" — reaches
	//     anywhere by construction), (b) is a bare literal with no "/"
	//     (gitignore-style, matches anywhere), or (c) textually contains
	//     one of scopePrefixes as a substring (a safe superset of "this
	//     glob is anchored at, or is an ancestor/descendant relative of,
	//     one of these paths" — the real anchor-vs-path relationship is
	//     re-checked by the caller's own scope matcher, e.g.
	//     internal/semantic's fileScopeMatch/rootScopeMatch, exactly as it
	//     already does against ClaimsForService's full result). Empty/nil
	//     means no scope filtering at all (every claim's scope, no matter
	//     what it says, passes this stage). A claim with an EMPTY Scope
	//     slice never matches a non-empty scopePrefixes filter (there is no
	//     glob to test) — same as it never matching a real scope-gated
	//     match() either.
	//
	// Results are ordered exactly like ClaimsForService: (layer, subject,
	// claim_id).
	ClaimsForServe(ctx context.Context, serviceID string, layers []string, statuses []core.ClaimStatus, scopePrefixes []string) ([]core.SemanticClaim, error)

	// PutTestSuites replaces serviceID's entire declared-suite set (vision
	// 33 stage 0, TC-2) with suites — a manifest reload's suites ARE the
	// current truth, the same "wholesale replace" contract PutTypedEdges
	// uses per package. Calling it with an empty slice clears every suite
	// for serviceID (a manifest that declared nothing, or was removed).
	PutTestSuites(ctx context.Context, serviceID string, suites []TestSuite) error

	// PutTestRuns appends runs (each with its embedded Cases) for
	// serviceID — runs are moment-in-time evidence, never replaced,
	// because a suite's run HISTORY is itself a fact stage 1 reads
	// (flakiness from the last three attested runs, design doc §5).
	// Idempotent by (serviceID, RunDigest): re-ingesting the identical
	// envelope a second time must not duplicate its cases.
	PutTestRuns(ctx context.Context, serviceID string, runs []TestRun) error

	// TestSuiteStatuses returns every declared suite for serviceID, each
	// with its most-recently-ingested run (HasRun false when none has
	// ever been ingested) and that run's join stats — the read `fathom
	// tests status` (TC-4) renders directly. Ordered by (component,
	// suite).
	TestSuiteStatuses(ctx context.Context, serviceID string) ([]TestSuiteStatus, error)

	// PutCertifyDeclarations replaces serviceID's entire
	// certify_declarations set (vision 33 stage 0, TC-5) with decls — a
	// fresh marker scan's declarations ARE the current truth, same
	// replace contract as PutTestSuites.
	PutCertifyDeclarations(ctx context.Context, serviceID string, decls []CertifyDeclaration) error

	// CertifyDeclarationsForService returns every declared certification
	// fact for serviceID, ordered by (file, line).
	CertifyDeclarationsForService(ctx context.Context, serviceID string) ([]CertifyDeclaration, error)

	// WriteServiceSummary is an ingest's final transaction (RT-76,
	// docs/design/responsible-tenant.md §6): it records meta (the
	// watermark), replaces serviceID's file_stats with files and writes
	// the service_summary row — symbol count and source kinds counted in
	// the same transaction, watermark WatermarkOf(meta).
	WriteServiceSummary(ctx context.Context, meta ServiceMeta, sum ServiceSummary, files []FileStat) error

	// ServiceSummary returns serviceID's service_summary row; found is
	// false when neither an ingest nor a backfill has written one (a named
	// absence: callers never fall back to a whole-tree or whole-table
	// read). Suppression-filtered: counts are recomputed without a hidden
	// file when a suppression applies (RT-79).
	ServiceSummary(ctx context.Context, serviceID string) (ServiceSummary, bool, error)

	// FileStats returns serviceID's file_stats rows sorted by path; ok is
	// false when no ingest has written them yet. Filtered by path.
	FileStats(ctx context.Context, serviceID string) (files []FileStat, ok bool, err error)

	// CallersOfKeys is the indexed reverse call lookup (RT-77): every
	// observation, in a call-indexed service, whose calls carry a key
	// whose full text or final name segment (CalleeName) is one of keys —
	// a candidate superset callers narrow with their own match. Sorted by
	// (service, symbol, source_ref, line). Filtered by source_ref.
	CallersOfKeys(ctx context.Context, keys []string) ([]CallerRow, error)

	// DerivedState reports which derived facts serviceID has.
	DerivedState(ctx context.Context, serviceID string) (DerivedState, error)

	// BackfillDerivedTables derives serviceID's summary and call index
	// from rows the store holds (RT-160): SQL GROUP BY and json_each, one
	// transaction, no file read. Callers hold the ingest slot.
	BackfillDerivedTables(ctx context.Context, serviceID string, opts BackfillOptions) (BackfillReport, error)

	// TestCaseRunsForSymbol returns testSymbolKey's case observations
	// across every attested run for serviceID, most-recent-run-first (by
	// the owning run's started_at, ties broken by ingested_at — the same
	// ordering TestSuiteStatuses' own last-run read uses), capped at limit
	// (0 = unbounded). vision 33 stage 1 (TC-6/TC-8).
	TestCaseRunsForSymbol(ctx context.Context, serviceID, testSymbolKey string, limit int) ([]TestCaseRun, error)
}

// SourceKindCount is one SourceKindCounts row: how many observations of
// Kind the file SourceRef holds.
type SourceKindCount struct {
	SourceRef string
	Kind      core.SourceKind
	Count     int
}
