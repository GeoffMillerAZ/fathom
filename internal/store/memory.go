package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MemoryStore is an in-process Store implementation backed by plain Go maps.
// It is intended for unit tests and session-mode operation where persistence is
// not required. All operations are goroutine-safe.
type MemoryStore struct {
	mu             sync.RWMutex
	observations   []core.Observation
	fingerprints   map[fpKey]string
	knowledgeCards map[kcKey]KnowledgeCard
	docSummaries   map[fpKey]DocSummary
	serviceMeta    map[string]ServiceMeta
	commits        map[string][]Commit        // serviceID → commits (newest-first)
	fileChurn      map[string][]FileChurn     // serviceID → file_churn rows (TQ-1), sorted by path
	centrality     map[string]centralityEntry // serviceID → symbol_centrality rows (TQ-2)
	claims         map[claimKey]core.SemanticClaim
	claimLinks     map[string][]core.ClaimLink // claim_id → links (replace-semantic)
	claimEvents    []core.ClaimEvent
	compileRuns    []core.CompileRun
	typedEdges     map[typedEdgeKey][]core.Edge     // (serviceID, pkgPath) → edges (replace-semantic)
	typedEdgeFPs   map[fpKey]string                 // reuses fpKey{serviceID, pkgPath} shape
	typedEdgeStat  map[string]TypedEdgeStatusRecord // serviceID → last persisted typed-edge status (DG-10)
	reviewRuns     map[reviewRunKey]core.ReviewRun  // (serviceID, idempotencyKey) → review run (FL-15)
	ingestState    map[string]IngestState           // serviceID → ingest progress (RT-109)
	fileImports    map[fileImportsKey]FileImports   // (service, path) → parsed imports (RT-108)

	// Test evidence + declared certifications (vision 33 / ADR-043 stage
	// 0, TC-2/TC-5): see memory_testevidence.go.
	testSuites          map[string][]TestSuite          // serviceID → declared suites (wholesale replace)
	testRuns            map[string][]TestRun            // serviceID → runs, append-only, dedup by (RunDigest, Suite)
	certifyDeclarations map[string][]CertifyDeclaration // serviceID → declared certifications (wholesale replace)

	// externalFindings is the findings-loop's per-claim cursor table (FL-7),
	// reusing claimKey's (serviceID, claimID) shape — one row per claim, same
	// identity PutClaims already keys on.
	externalFindings map[claimKey]core.ExternalFinding

	// summaries and fileStats are the derived facts (RT-76,
	// memory_derived.go); nil until first written.
	summaries map[string]ServiceSummary
	fileStats map[string][]FileStat
	// serviceRefs are RT-89's extracted routes and client calls.
	serviceRefs map[string]*memServiceRefs

	// gate is the per-call suppression filter (scoped.go); initialised by
	// NewMemoryStore and exercised by the constructor audit.
	gate scopeGate
}

// typedEdgeKey scopes stored typed edges to (serviceID, pkgPath) — the
// package-granularity replace unit PutTypedEdges implements.
type typedEdgeKey struct {
	serviceID string
	pkgPath   string
}

type fpKey struct {
	serviceID string
	sourceRef string
}

type kcKey struct {
	serviceID string
	symbol    string
}

type claimKey struct {
	serviceID string
	claimID   string
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		gate:           newScopeGate(),
		fingerprints:   make(map[fpKey]string),
		knowledgeCards: make(map[kcKey]KnowledgeCard),
		docSummaries:   make(map[fpKey]DocSummary),
		serviceMeta:    make(map[string]ServiceMeta),
		commits:        make(map[string][]Commit),
		fileChurn:      make(map[string][]FileChurn),
		centrality:     make(map[string]centralityEntry),
		claims:         make(map[claimKey]core.SemanticClaim),
		claimLinks:     make(map[string][]core.ClaimLink),
		typedEdges:     make(map[typedEdgeKey][]core.Edge),
		typedEdgeFPs:   make(map[fpKey]string),
		typedEdgeStat:  make(map[string]TypedEdgeStatusRecord),
		ingestState:    make(map[string]IngestState),
		fileImports:    make(map[fileImportsKey]FileImports),

		externalFindings: make(map[claimKey]core.ExternalFinding),
		reviewRuns:       make(map[reviewRunKey]core.ReviewRun),

		testSuites:          make(map[string][]TestSuite),
		testRuns:            make(map[string][]TestRun),
		certifyDeclarations: make(map[string][]CertifyDeclaration),
	}
}

// EnsureIndexes is SQLiteStore.EnsureIndexes' MemoryStore counterpart
// (SU-1, docs/plans/substrate.md §A): a plain Go map has no query planner
// to steer, so there is nothing to create — always a no-op, never an
// error, kept only so callers that hold a Store interface value can call
// it uniformly regardless of backend.
func (m *MemoryStore) EnsureIndexes(_ context.Context) error {
	return nil
}

// PutObservations appends the supplied observations. IngestedAt is set to now
// when the caller leaves it as the zero value.
func (m *MemoryStore) PutObservations(_ context.Context, obs []core.Observation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, o := range obs {
		if o.IngestedAt.IsZero() {
			o.IngestedAt = now
		}
		m.observations = append(m.observations, o)
	}
	return nil
}

// QueryObservations returns observations matching serviceID and, when
// concernID is non-empty, also matching concernID.
func (m *MemoryStore) queryObservationsRaw(_ context.Context, serviceID, concernID string) ([]core.Observation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.Observation
	for _, o := range m.observations {
		if o.ServiceID != serviceID {
			continue
		}
		if concernID != "" && o.ConcernID != concernID {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

// sourcePackagesRaw is SourcePackages before the suppression filter.
func (m *MemoryStore) sourcePackagesRaw(_ context.Context, serviceID, sourceRef string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, o := range m.observations {
		if o.ServiceID == serviceID && o.SourceRef == sourceRef && !seen[o.Package] {
			seen[o.Package] = true
			out = append(out, o.Package)
		}
	}
	sort.Strings(out)
	return out, nil
}

// packageObservationsRaw is PackageObservations before the suppression
// filter.
func (m *MemoryStore) packageObservationsRaw(_ context.Context, serviceID string, pkgs []string) ([]core.Observation, error) {
	want := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		want[p] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.Observation
	for _, o := range m.observations {
		if o.ServiceID == serviceID && want[o.Package] {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SourceRef != out[j].SourceRef {
			return out[i].SourceRef < out[j].SourceRef
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out, nil
}

// sourceKindCountsRaw is SourceKindCounts before the suppression filter.
func (m *MemoryStore) sourceKindCountsRaw(_ context.Context, serviceID string) ([]SourceKindCount, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type key struct {
		ref  string
		kind core.SourceKind
	}
	counts := map[key]int{}
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			counts[key{o.SourceRef, o.SourceKind}]++
		}
	}
	if len(counts) == 0 {
		return nil, nil
	}
	out := make([]SourceKindCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, SourceKindCount{SourceRef: k.ref, Kind: k.kind, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceRef != out[j].SourceRef {
			return out[i].SourceRef < out[j].SourceRef
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// Fingerprint returns the stored hash for (serviceID, sourceRef).
func (m *MemoryStore) Fingerprint(_ context.Context, serviceID, sourceRef string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.fingerprints[fpKey{serviceID, sourceRef}]
	return h, ok, nil
}

// SetFingerprint records or replaces the hash for (serviceID, sourceRef).
func (m *MemoryStore) SetFingerprint(_ context.Context, serviceID, sourceRef, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fingerprints[fpKey{serviceID, sourceRef}] = hash
	return nil
}

// PurgeSource removes all observations for (serviceID, sourceRef).
func (m *MemoryStore) PurgeSource(_ context.Context, serviceID, sourceRef string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.observations[:0]
	for _, o := range m.observations {
		if o.ServiceID == serviceID && o.SourceRef == sourceRef {
			continue
		}
		kept = append(kept, o)
	}
	m.observations = kept
	return nil
}

// PurgeService removes all content rows (observations, fingerprints, doc
// summaries, typed edges, typed-edge fingerprints) for serviceID, keeping
// identity rows. Mirrors SQLiteStore for the ADR-017 self-heal.
func (m *MemoryStore) PurgeService(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropDerivedLocked(serviceID)
	kept := m.observations[:0]
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			continue
		}
		kept = append(kept, o)
	}
	m.observations = kept
	for k := range m.fingerprints {
		if k.serviceID == serviceID {
			delete(m.fingerprints, k)
		}
	}
	for k := range m.docSummaries {
		if k.serviceID == serviceID {
			delete(m.docSummaries, k)
		}
	}
	for k := range m.typedEdges {
		if k.serviceID == serviceID {
			delete(m.typedEdges, k)
		}
	}
	for k := range m.typedEdgeFPs {
		if k.serviceID == serviceID {
			delete(m.typedEdgeFPs, k)
		}
	}
	m.deleteServiceFileImports(serviceID)
	return nil
}

// RemoveService deletes every row for serviceID — see the Store interface
// doc comment for the registry-surgery motivation. The service's claim IDs
// are collected FIRST (claim events and links are claim_id-keyed), then
// every table class is cleared. Mirrors SQLiteStore's single-transaction
// delete under this store's one write lock.
func (m *MemoryStore) RemoveService(_ context.Context, serviceID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := false

	// Claim membership first, while the claims are still present.
	claimIDs := make(map[string]struct{})
	for k := range m.claims {
		if k.serviceID == serviceID {
			claimIDs[k.claimID] = struct{}{}
		}
	}
	if len(claimIDs) > 0 {
		keptEvents := m.claimEvents[:0]
		for _, ev := range m.claimEvents {
			if _, ours := claimIDs[ev.ClaimID]; ours {
				removed = true
				continue
			}
			keptEvents = append(keptEvents, ev)
		}
		m.claimEvents = keptEvents
		for id := range claimIDs {
			if _, ok := m.claimLinks[id]; ok {
				removed = true
				delete(m.claimLinks, id)
			}
		}
		for k := range m.claims {
			if k.serviceID == serviceID {
				removed = true
				delete(m.claims, k)
			}
		}
	}

	keptObs := m.observations[:0]
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			removed = true
			continue
		}
		keptObs = append(keptObs, o)
	}
	m.observations = keptObs
	for k := range m.fingerprints {
		if k.serviceID == serviceID {
			removed = true
			delete(m.fingerprints, k)
		}
	}
	for k := range m.knowledgeCards {
		if k.serviceID == serviceID {
			removed = true
			delete(m.knowledgeCards, k)
		}
	}
	for k := range m.docSummaries {
		if k.serviceID == serviceID {
			removed = true
			delete(m.docSummaries, k)
		}
	}
	if _, ok := m.serviceMeta[serviceID]; ok {
		removed = true
		delete(m.serviceMeta, serviceID)
	}
	if _, ok := m.ingestState[serviceID]; ok {
		removed = true
		delete(m.ingestState, serviceID)
	}
	if m.dropDerivedLocked(serviceID) {
		removed = true
	}
	m.deleteServiceFileImports(serviceID)
	if _, ok := m.commits[serviceID]; ok {
		removed = true
		delete(m.commits, serviceID)
	}
	if _, ok := m.fileChurn[serviceID]; ok {
		removed = true
		delete(m.fileChurn, serviceID)
	}
	if _, ok := m.centrality[serviceID]; ok {
		removed = true
		delete(m.centrality, serviceID)
	}
	keptRuns := m.compileRuns[:0]
	for _, r := range m.compileRuns {
		if r.ServiceID == serviceID {
			removed = true
			continue
		}
		keptRuns = append(keptRuns, r)
	}
	m.compileRuns = keptRuns
	for k := range m.typedEdges {
		if k.serviceID == serviceID {
			removed = true
			delete(m.typedEdges, k)
		}
	}
	for k := range m.typedEdgeFPs {
		if k.serviceID == serviceID {
			removed = true
			delete(m.typedEdgeFPs, k)
		}
	}
	return removed, nil
}

// RenameService moves every service_id-scoped row from oldID to newID — see
// the Store interface doc comment for the claim-ID stability contract.
// claimEvents (and the claim_id-keyed claimLinks map) are untouched: claim
// IDs keep their minted values, so both keep resolving after the rename.
func (m *MemoryStore) RenameService(_ context.Context, oldID, newID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.serviceHasRowsLocked(oldID) {
		return &ServiceNotFoundError{ServiceID: oldID}
	}
	if m.serviceHasRowsLocked(newID) {
		return &ServiceExistsError{ServiceID: newID}
	}

	for i := range m.observations {
		if m.observations[i].ServiceID == oldID {
			m.observations[i].ServiceID = newID
		}
	}
	rekeyFP(m.fingerprints, oldID, newID)
	for k, c := range m.knowledgeCards {
		if k.serviceID == oldID {
			c.ServiceID = newID
			delete(m.knowledgeCards, k)
			m.knowledgeCards[kcKey{newID, k.symbol}] = c
		}
	}
	for k, d := range m.docSummaries {
		if k.serviceID == oldID {
			d.ServiceID = newID
			delete(m.docSummaries, k)
			m.docSummaries[fpKey{newID, k.sourceRef}] = d
		}
	}
	if meta, ok := m.serviceMeta[oldID]; ok {
		meta.ServiceID = newID
		delete(m.serviceMeta, oldID)
		m.serviceMeta[newID] = meta
	}
	if is, ok := m.ingestState[oldID]; ok {
		is.ServiceID = newID
		delete(m.ingestState, oldID)
		m.ingestState[newID] = is
	}
	m.renameDerivedLocked(oldID, newID)
	m.renameServiceFileImports(oldID, newID)
	if commits, ok := m.commits[oldID]; ok {
		for i := range commits {
			commits[i].ServiceID = newID
		}
		delete(m.commits, oldID)
		m.commits[newID] = commits
	}
	if fc, ok := m.fileChurn[oldID]; ok {
		delete(m.fileChurn, oldID)
		m.fileChurn[newID] = fc
	}
	if ce, ok := m.centrality[oldID]; ok {
		delete(m.centrality, oldID)
		m.centrality[newID] = ce
	}
	for k, c := range m.claims {
		if k.serviceID == oldID {
			c.ServiceID = newID
			delete(m.claims, k)
			m.claims[claimKey{newID, k.claimID}] = c
		}
	}
	for i := range m.compileRuns {
		if m.compileRuns[i].ServiceID == oldID {
			m.compileRuns[i].ServiceID = newID
		}
	}
	for k, edges := range m.typedEdges {
		if k.serviceID == oldID {
			delete(m.typedEdges, k)
			m.typedEdges[typedEdgeKey{newID, k.pkgPath}] = edges
		}
	}
	rekeyFP(m.typedEdgeFPs, oldID, newID)
	return nil
}

// serviceHasRowsLocked reports whether serviceID holds any row, judged
// against the same three presence tables SQLiteStore.RenameService checks
// (service_meta, observations, semantic_claims). Callers must hold m.mu.
func (m *MemoryStore) serviceHasRowsLocked(serviceID string) bool {
	if _, ok := m.serviceMeta[serviceID]; ok {
		return true
	}
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			return true
		}
	}
	for k := range m.claims {
		if k.serviceID == serviceID {
			return true
		}
	}
	return false
}

// rekeyFP moves every fpKey-shaped entry for oldID to newID in place —
// shared by the fingerprints and typed-edge-fingerprint maps (both reuse
// the fpKey shape).
func rekeyFP(mm map[fpKey]string, oldID, newID string) {
	for k, v := range mm {
		if k.serviceID == oldID {
			delete(mm, k)
			mm[fpKey{newID, k.sourceRef}] = v
		}
	}
}

// CountAbsoluteSourceRefs counts fingerprint rows whose source_ref is an
// absolute path (the pre-ADR-017 anchor style).
func (m *MemoryStore) CountAbsoluteSourceRefs(_ context.Context, serviceID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for k := range m.fingerprints {
		if k.serviceID == serviceID && filepath.IsAbs(k.sourceRef) {
			n++
		}
	}
	return n, nil
}

// ServiceContentDigest digests serviceID's fingerprints in source_ref order
// (see Store.ServiceContentDigest).
func (m *MemoryStore) ServiceContentDigest(_ context.Context, serviceID string) (string, error) {
	m.mu.RLock()
	var refs []string
	for k := range m.fingerprints {
		if k.serviceID == serviceID {
			refs = append(refs, k.sourceRef)
		}
	}
	sort.Strings(refs)
	h := sha256.New()
	for _, ref := range refs {
		writeDigestRow(h, ref, m.fingerprints[fpKey{serviceID, ref}])
	}
	m.mu.RUnlock()
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DeleteFingerprint removes the fingerprint entry for (serviceID, sourceRef).
// Deleting a missing fingerprint is a no-op.
func (m *MemoryStore) DeleteFingerprint(_ context.Context, serviceID, sourceRef string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fingerprints, fpKey{serviceID, sourceRef})
	return nil
}

// ListSourceRefs returns the sorted distinct source_refs for serviceID across
// observations and fingerprints.
func (m *MemoryStore) listSourceRefsRaw(_ context.Context, serviceID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			seen[o.SourceRef] = struct{}{}
		}
	}
	for k := range m.fingerprints {
		if k.serviceID == serviceID {
			seen[k.sourceRef] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	refs := make([]string, 0, len(seen))
	for ref := range seen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs, nil
}

// PutKnowledgeCards upserts knowledge-card rows keyed by (service_id, symbol).
func (m *MemoryStore) PutKnowledgeCards(_ context.Context, cards []KnowledgeCard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range cards {
		// Copy the Tags slice so the stored value is independent of the caller's slice.
		var tags []string
		if c.Tags != nil {
			tags = make([]string, len(c.Tags))
			copy(tags, c.Tags)
		}
		m.knowledgeCards[kcKey{c.ServiceID, c.Symbol}] = KnowledgeCard{
			ServiceID: c.ServiceID,
			Symbol:    c.Symbol,
			File:      c.File,
			Summary:   c.Summary,
			Tags:      tags,
		}
	}
	return nil
}

// KnowledgeCards returns all knowledge cards for serviceID ordered by symbol.
func (m *MemoryStore) knowledgeCardsRaw(_ context.Context, serviceID string) ([]KnowledgeCard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []KnowledgeCard
	for k, c := range m.knowledgeCards {
		if k.serviceID != serviceID {
			continue
		}
		// Return a copy so callers cannot mutate stored state.
		var tags []string
		if c.Tags != nil {
			tags = make([]string, len(c.Tags))
			copy(tags, c.Tags)
		}
		result = append(result, KnowledgeCard{
			ServiceID: c.ServiceID,
			Symbol:    c.Symbol,
			File:      c.File,
			Summary:   c.Summary,
			Tags:      tags,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result, nil
}

// PutDocSummaries upserts doc-summary rows keyed by (service_id, source_ref).
func (m *MemoryStore) PutDocSummaries(_ context.Context, summaries []DocSummary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range summaries {
		m.docSummaries[fpKey{d.ServiceID, d.SourceRef}] = d
	}
	return nil
}

// DocSummaries returns all doc summaries for serviceID ordered by source_ref.
func (m *MemoryStore) docSummariesRaw(_ context.Context, serviceID string) ([]DocSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []DocSummary
	for k, d := range m.docSummaries {
		if k.serviceID != serviceID {
			continue
		}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceRef < result[j].SourceRef })
	return result, nil
}

// ListServiceIDs returns the sorted distinct set of service IDs present in the
// store.
func (m *MemoryStore) ListServiceIDs(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{}, 4)
	for _, o := range m.observations {
		seen[o.ServiceID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// ClaimServiceIDs returns the sorted distinct set of service IDs present in
// m.claims — see the Store interface doc comment.
func (m *MemoryStore) ClaimServiceIDs(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{}, 4)
	for k := range m.claims {
		seen[k.serviceID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// SetServiceMeta upserts the ingest metadata for meta.ServiceID.
func (m *MemoryStore) SetServiceMeta(_ context.Context, meta ServiceMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceMeta[meta.ServiceID] = meta
	return nil
}

// GetServiceMeta returns the ingest metadata for serviceID.
// found is false when no metadata has been recorded.
func (m *MemoryStore) GetServiceMeta(_ context.Context, serviceID string) (ServiceMeta, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.serviceMeta[serviceID]
	return meta, ok, nil
}

// SetCommits replaces all commits for serviceID with a copy sorted newest-first.
func (m *MemoryStore) SetCommits(_ context.Context, serviceID string, commits []Commit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Commit, len(commits))
	copy(cp, commits)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Date > cp[j].Date })
	m.commits[serviceID] = cp
	return nil
}

// ListCommits returns commits for serviceID newest-first.
func (m *MemoryStore) ListCommits(_ context.Context, serviceID string) ([]Commit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.commits[serviceID]
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]Commit, len(src))
	copy(out, src)
	return out, nil
}

// PutClaims upserts claims and replaces the link set for every distinct
// claim_id that appears in links. Scope slices are copied so stored values are
// independent of the caller's slice.
//
// Refutation durability (FT-4, docs/vision/03-findings-and-experts.md §3): a
// claim_id whose stored status is already ClaimRefuted retains it across the
// upsert — a recompile that re-emits an unchanged rule under a refuted
// ClaimID must not silently resurrect it to the compiler's fresh status.
//
// SU-19 (ADR-041 decision 7) mirrors SQLiteStore's own upsert CASE logic:
// ValidFrom is stamped once, at first mint, and never drifts forward on a
// later upsert of the same claim_id; ValidTo survives across the upsert
// exactly when Status does (refuted), and otherwise takes the incoming
// value — reopening the validity window on a superseded→fresh resurrection,
// exactly as the status field itself does.
func (m *MemoryStore) PutClaims(_ context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range claims {
		sc := c
		if c.Scope != nil {
			sc.Scope = make([]string, len(c.Scope))
			copy(sc.Scope, c.Scope)
		}
		key := claimKey{c.ServiceID, c.ClaimID}
		if existing, ok := m.claims[key]; ok {
			if existing.Status == core.ClaimRefuted {
				sc.Status = core.ClaimRefuted
				sc.ValidTo = existing.ValidTo
			}
			if !existing.ValidFrom.IsZero() {
				sc.ValidFrom = existing.ValidFrom
			}
		}
		m.claims[key] = sc
	}
	// Collect distinct claim_ids whose link sets must be replaced.
	distinct := make(map[string]struct{}, len(links))
	for _, l := range links {
		distinct[l.ClaimID] = struct{}{}
	}
	for id := range distinct {
		m.claimLinks[id] = nil
	}
	for _, l := range links {
		m.claimLinks[l.ClaimID] = append(m.claimLinks[l.ClaimID], l)
	}
	return nil
}

// ClaimsForService returns claims for serviceID (optionally filtered by layer)
// ordered deterministically by (layer, subject, claim_id). Scope slices are
// copied so callers cannot mutate stored state.
func (m *MemoryStore) ClaimsForService(_ context.Context, serviceID, layer string) ([]core.SemanticClaim, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.SemanticClaim
	for k, c := range m.claims {
		if k.serviceID != serviceID {
			continue
		}
		if layer != "" && c.Layer != layer {
			continue
		}
		sc := c
		if c.Scope != nil {
			sc.Scope = make([]string, len(c.Scope))
			copy(sc.Scope, c.Scope)
		}
		result = append(result, sc)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Layer != result[j].Layer {
			return result[i].Layer < result[j].Layer
		}
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		return result[i].ClaimID < result[j].ClaimID
	})
	return result, nil
}

// scopeReachesAnywhere mirrors SQLiteStore's scopeReachAnywhereLike check
// (sqlite.go): true when g is unanchored/wildcard-rooted ("*", "**",
// "**/…") or a bare literal with no "/" (gitignore-style, matches
// anywhere) — either way a claim carrying it must survive ClaimsForServe's
// scope-prefix filter regardless of scopePrefixes, per the Store interface
// doc comment.
func scopeReachesAnywhere(g string) bool {
	if g == "*" || g == "**" || strings.HasPrefix(g, "**/") {
		return true
	}
	return !strings.Contains(g, "/")
}

// scopeServeMatch reports whether ANY glob in scope passes ClaimsForServe's
// over-approximate scope-prefix test: reaches anywhere (scopeReachesAnywhere)
// or textually contains one of prefixes as a substring — see the Store
// interface's ClaimsForServe doc comment for why substring containment is
// the deliberately loose (never under-excluding) test used here.
func scopeServeMatch(scope []string, prefixes []string) bool {
	for _, g := range scope {
		if scopeReachesAnywhere(g) {
			return true
		}
		for _, p := range prefixes {
			if strings.Contains(g, p) {
				return true
			}
		}
	}
	return false
}

// ClaimsForServe is ClaimsForService's prefiltered sibling — see the Store
// interface doc comment for the exact (deliberately over-approximate)
// scope-prefix semantics, mirrored here in Go rather than SQL.
func (m *MemoryStore) ClaimsForServe(_ context.Context, serviceID string, layers []string, statuses []core.ClaimStatus, scopePrefixes []string) ([]core.SemanticClaim, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	layerSet := map[string]bool{}
	for _, l := range layers {
		layerSet[l] = true
	}
	statusSet := map[core.ClaimStatus]bool{}
	for _, st := range statuses {
		statusSet[st] = true
	}

	var result []core.SemanticClaim
	for k, c := range m.claims {
		if k.serviceID != serviceID {
			continue
		}
		if len(layerSet) > 0 && !layerSet[c.Layer] {
			continue
		}
		if len(statusSet) > 0 && !statusSet[c.Status] {
			continue
		}
		if len(scopePrefixes) > 0 && !scopeServeMatch(c.Scope, scopePrefixes) {
			continue
		}
		sc := c
		if c.Scope != nil {
			sc.Scope = make([]string, len(c.Scope))
			copy(sc.Scope, c.Scope)
		}
		result = append(result, sc)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Layer != result[j].Layer {
			return result[i].Layer < result[j].Layer
		}
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		return result[i].ClaimID < result[j].ClaimID
	})
	return result, nil
}

// ClaimLinks returns links for the supplied claim IDs ordered by (claim_id,
// rel, target_ref). The serviceID parameter is accepted for interface
// conformance; claim_id is content-addressed and globally unique, so
// service-level filtering is not required in the in-process store.
func (m *MemoryStore) claimLinksRaw(_ context.Context, _ string, claimIDs []string) ([]core.ClaimLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(claimIDs) == 0 {
		return nil, nil
	}
	want := make(map[string]struct{}, len(claimIDs))
	for _, id := range claimIDs {
		want[id] = struct{}{}
	}
	var result []core.ClaimLink
	for id, ls := range m.claimLinks {
		if _, ok := want[id]; !ok {
			continue
		}
		result = append(result, ls...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ClaimID != result[j].ClaimID {
			return result[i].ClaimID < result[j].ClaimID
		}
		if result[i].Rel != result[j].Rel {
			return result[i].Rel < result[j].Rel
		}
		return result[i].TargetRef < result[j].TargetRef
	})
	return result, nil
}

// UpdateClaimLink re-anchors a single link in place — see the Store interface
// doc comment. AnchorText is left untouched (the text itself never changes on
// a re-anchor, only where it points). A no-op when no link matches
// (claim_id, rel, oldTargetRef).
func (m *MemoryStore) UpdateClaimLink(_ context.Context, _, claimID, rel, oldTargetRef, newTargetRef, newTargetHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	links := m.claimLinks[claimID]
	for i, l := range links {
		if l.Rel == rel && l.TargetRef == oldTargetRef {
			links[i].TargetRef = newTargetRef
			links[i].TargetHash = newTargetHash
			return nil
		}
	}
	return nil
}

// AppendClaimLink appends ONE link row without touching claim_id's existing
// link set (SU-20, ADR-041 decision 7) — see the Store interface doc
// comment.
func (m *MemoryStore) AppendClaimLink(_ context.Context, _ string, link core.ClaimLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimLinks[link.ClaimID] = append(m.claimLinks[link.ClaimID], link)
	return nil
}

// SetClaimStatus flips the status of a single claim. No-op when absent.
func (m *MemoryStore) SetClaimStatus(_ context.Context, serviceID, claimID string, status core.ClaimStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := claimKey{serviceID, claimID}
	if c, ok := m.claims[k]; ok {
		c.Status = status
		m.claims[k] = c
	}
	return nil
}

// SetClaimValidTo closes a single claim's bi-temporal validity window at t
// (SU-19, ADR-041 decision 7) — see the Store interface doc comment.
func (m *MemoryStore) SetClaimValidTo(_ context.Context, serviceID, claimID string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := claimKey{serviceID, claimID}
	if c, ok := m.claims[k]; ok {
		c.ValidTo = t
		m.claims[k] = c
	}
	return nil
}

// AppendClaimEvent appends one verdict to the claim's append-only event log.
func (m *MemoryStore) AppendClaimEvent(_ context.Context, ev core.ClaimEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claimEvents = append(m.claimEvents, ev)
	return nil
}

// ClaimEvents returns all events for claimID ordered by (at, verdict).
func (m *MemoryStore) ClaimEvents(_ context.Context, claimID string) ([]core.ClaimEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.ClaimEvent
	for _, ev := range m.claimEvents {
		if ev.ClaimID == claimID {
			result = append(result, ev)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].At.Equal(result[j].At) {
			return result[i].At.Before(result[j].At)
		}
		return result[i].Verdict < result[j].Verdict
	})
	return result, nil
}

// ClaimEventsSince (Q-6) returns every event in [since, until] across every
// claim, ordered by (At, Verdict) — see the Store interface's own doc
// comment for the store-wide-not-service-scoped caveat.
func (m *MemoryStore) ClaimEventsSince(_ context.Context, since, until time.Time) ([]core.ClaimEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.ClaimEvent
	for _, ev := range m.claimEvents {
		if ev.At.Before(since) || ev.At.After(until) {
			continue
		}
		result = append(result, ev)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].At.Equal(result[j].At) {
			return result[i].At.Before(result[j].At)
		}
		return result[i].Verdict < result[j].Verdict
	})
	return result, nil
}

// RecordCompileRun appends a cost/audit ledger row for one compilation pass.
func (m *MemoryStore) RecordCompileRun(_ context.Context, run core.CompileRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compileRuns = append(m.compileRuns, run)
	return nil
}

// CompileRunRefsAfter returns every compile run recorded after the one
// whose Seq is after, in record order (SQLiteStore.CompileRunRefsAfter;
// Seq is the run's 1-based position in the record).
func (m *MemoryStore) CompileRunRefsAfter(_ context.Context, after int64) ([]CompileRunRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []CompileRunRef
	for i, r := range m.compileRuns {
		if seq := int64(i + 1); seq > after {
			out = append(out, CompileRunRef{Seq: seq, RunID: r.RunID, ServiceID: r.ServiceID, Layer: r.Layer, RanAt: r.RanAt.UTC()})
		}
	}
	return out, nil
}

// CompileRuns returns compile runs for serviceID (optionally filtered by
// layer; layer "" returns all), newest-first by RanAt then run_id.
func (m *MemoryStore) CompileRuns(_ context.Context, serviceID, layer string) ([]core.CompileRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.CompileRun
	for _, r := range m.compileRuns {
		if r.ServiceID != serviceID {
			continue
		}
		if layer != "" && r.Layer != layer {
			continue
		}
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].RanAt.Equal(result[j].RanAt) {
			return result[i].RanAt.After(result[j].RanAt)
		}
		return result[i].RunID < result[j].RunID
	})
	return result, nil
}

// PutTypedEdges replaces all typed call edges for (serviceID, pkgPath). A
// copy is stored so the caller's slice can be mutated/reused afterward.
func (m *MemoryStore) PutTypedEdges(_ context.Context, serviceID, pkgPath string, edges []core.Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := typedEdgeKey{serviceID, pkgPath}
	if len(edges) == 0 {
		delete(m.typedEdges, key)
		return nil
	}
	cp := make([]core.Edge, len(edges))
	copy(cp, edges)
	m.typedEdges[key] = cp
	return nil
}

// TypedEdges returns every typed call edge stored for serviceID, ordered by
// (from_symbol, to_symbol, kind).
func (m *MemoryStore) typedEdgesRaw(_ context.Context, serviceID string) ([]core.Edge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.Edge
	for k, edges := range m.typedEdges {
		if k.serviceID != serviceID {
			continue
		}
		result = append(result, edges...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].FromSymbol != result[j].FromSymbol {
			return result[i].FromSymbol < result[j].FromSymbol
		}
		if result[i].ToSymbol != result[j].ToSymbol {
			return result[i].ToSymbol < result[j].ToSymbol
		}
		return result[i].Kind < result[j].Kind
	})
	return result, nil
}

// TypedEdgePackageFingerprint returns the cached content-hash for
// (serviceID, pkgPath). found is false when the package has never been
// loaded.
func (m *MemoryStore) TypedEdgePackageFingerprint(_ context.Context, serviceID, pkgPath string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.typedEdgeFPs[fpKey{serviceID, pkgPath}]
	return h, ok, nil
}

// SetTypedEdgePackageFingerprint records or replaces the content-hash cache
// entry for (serviceID, pkgPath).
func (m *MemoryStore) SetTypedEdgePackageFingerprint(_ context.Context, serviceID, pkgPath, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.typedEdgeFPs[fpKey{serviceID, pkgPath}] = hash
	return nil
}

// ListTypedEdgePackages returns the sorted distinct typed-edge package IDs
// for serviceID (union of stored edges and fingerprints).
func (m *MemoryStore) ListTypedEdgePackages(_ context.Context, serviceID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{})
	for k := range m.typedEdges {
		if k.serviceID == serviceID {
			seen[k.pkgPath] = struct{}{}
		}
	}
	for k := range m.typedEdgeFPs {
		if k.serviceID == serviceID {
			seen[k.sourceRef] = struct{}{} // fpKey reuse: sourceRef holds pkgPath here
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// DeleteTypedEdgePackage removes all typed-edge rows and the fingerprint
// entry for (serviceID, pkgPath). Missing rows are a no-op.
func (m *MemoryStore) DeleteTypedEdgePackage(_ context.Context, serviceID, pkgPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.typedEdges, typedEdgeKey{serviceID, pkgPath})
	delete(m.typedEdgeFPs, fpKey{serviceID, pkgPath})
	return nil
}

// SetTypedEdgeStatus records or replaces the persisted typed-edge status
// for serviceID (DG-10).
func (m *MemoryStore) SetTypedEdgeStatus(_ context.Context, serviceID string, rec TypedEdgeStatusRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec.RecordedAt.IsZero() {
		rec.RecordedAt = time.Now().UTC()
	}
	// Store a defensive copy of the slice: the caller may reuse/mutate its
	// backing array (e.g. TypedEdgeReport.DiscoveryGapFiles across runs).
	if rec.DiscoveryGapFiles != nil {
		rec.DiscoveryGapFiles = append([]string(nil), rec.DiscoveryGapFiles...)
	}
	m.typedEdgeStat[serviceID] = rec
	return nil
}

// TypedEdgeStatus returns the last persisted typed-edge status for
// serviceID. found is false when none has ever been recorded.
func (m *MemoryStore) typedEdgeStatusRaw(_ context.Context, serviceID string) (TypedEdgeStatusRecord, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.typedEdgeStat[serviceID]
	return rec, ok, nil
}

// Close is a no-op for the in-memory store.
func (m *MemoryStore) Close() error { return nil }
