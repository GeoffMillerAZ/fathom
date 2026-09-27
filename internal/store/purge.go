package store

// purge.go — phase V's IRREVERSIBLE half (docs/design/scope-and-purge.md
// §3). Suppression hides; purge deletes, and the design's requirement is
// stated as a claim to be PROVEN:
//
//	After a purge of pattern P in service S, no content originating from a
//	path matching P in S is retrievable through any fathom surface, and no
//	copy remains in any store fathom writes to.
//
// Three findings from the adversarial review of that document shaped this
// file, and undoing any of them makes the guarantee false again:
//
//  1. **Resolution reads EACH location's own path column** (§3.3).
//     ListSourceRefs is observations ∪ fingerprints only; knowledge_cards
//     has no source_ref at all and claim_links anchors through the
//     `path#Lstart-Lend` citation grammar. A purge resolved from one ref
//     list deletes nothing in the very case the feature exists for — a
//     service whose secrets/ was already .fathomignore'd, leaving only an
//     LLM summary of the file behind — and then writes "verified" over it.
//     See resolvePurgeTargets.
//  2. **Verification is re-derived, never inherited** (§3.3). Re-querying
//     the refs the deletion was built from can only confirm the deletion it
//     came from (law 14). Verification re-scans each table's own path
//     column by PATTERN, and additionally re-checks the plan's own keys, so
//     both "the plan missed a row" and "the delete missed a row" are
//     caught.
//  3. **Physical residue is a location** (§3.2's last row). WAL-mode SQLite
//     with no secure_delete keeps the deleted bytes in free pages and in
//     the -wal; reproduced against this repo's own store during review, a
//     purge of a leaked credential reported VERIFIED while `strings
//     fleet.sqlite` still returned the key. The deletion therefore runs on
//     a secure_delete connection, followed by wal_checkpoint(TRUNCATE) and
//     VACUUM, and verification includes a RAW-BYTES probe of the store file
//     — the only check that can see residue SQL structurally cannot.
//
// And the V-round rework added a fourth, V1: **the probe's outcome is part
// of the verdict TYPE.** The previous build printed VERIFIED whenever the
// SQL re-scan was clean, even when the raw-bytes probe never ran (every
// needle skipped as shared with another service; a read error falling
// through; Ran set after a loop that scanned zero files). PurgeVerdict has
// three values and only "SQL zero + probe ran clean" is PurgeVerified;
// "SQL zero, bytes unchecked" is a distinct, named state that the CLI maps
// to its own non-zero exit code; and RawProbe.Ran is true only when at
// least one needle was actually scanned against at least one file with no
// read error. purgeVerdict is the single function that folds these — there
// is no boolean field for a caller to shortcut through.
//
// §3.2's table IS the design: a new table that can hold content must be
// added to PurgeLocations in the same commit that creates it.

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// PurgeLocation names one place a copy of purged content can live (§3.2).
type PurgeLocation string

// The SQL locations. Every one is counted before and after, so a reader can
// tell "checked, empty" from "never looked".
const (
	LocObservations          PurgeLocation = "observations"
	LocFingerprints          PurgeLocation = "fingerprints"
	LocDocSummaries          PurgeLocation = "doc_summaries"
	LocKnowledgeCards        PurgeLocation = "knowledge_cards"
	LocTypedEdges            PurgeLocation = "typed_edges"
	LocTypedEdgeFingerprints PurgeLocation = "typed_edge_fingerprints"
	LocSemanticClaims        PurgeLocation = "semantic_claims"
	LocClaimLinks            PurgeLocation = "claim_links"
	LocClaimEvents           PurgeLocation = "claim_events"
	// LocFileStats is file_stats.path (RT-76): every discovered file,
	// including an oversize or unreadable one that has no fingerprint.
	LocFileStats PurgeLocation = "file_stats"
)

// The non-SQL locations, reported separately because they have no row
// counts — a derived store is either cleared, or absent WITH A REASON.
const (
	LocVectorIndex    PurgeLocation = "vector_index"
	LocResultCache    PurgeLocation = "result_cache"
	LocMemories       PurgeLocation = "memories"
	LocStoreFileBytes PurgeLocation = "store_file_bytes"
)

// PurgeLocations returns the SQL locations, in report order. A test
// iterates this to assert §3.2's table is fully covered.
func PurgeLocations() []PurgeLocation {
	return []PurgeLocation{
		LocObservations, LocFingerprints, LocDocSummaries, LocKnowledgeCards,
		LocTypedEdges, LocTypedEdgeFingerprints, LocSemanticClaims,
		LocClaimLinks, LocClaimEvents, LocFileStats,
	}
}

// PathMatcher decides whether a repo-relative path is inside the purge set.
// *filter.Matcher satisfies it; the interface keeps this package free of a
// dependency on the pattern syntax.
type PathMatcher interface {
	Match(path string) bool
}

// CacheFlusher is the result cache (internal/resultcache). §3.2: it is
// keyed by QUERY, not by ref, so selective removal is not possible and a
// full flush is the only sound option — a purge that leaves a cached answer
// containing the purged content is a purge that failed.
type CacheFlusher interface {
	Invalidate()
}

// VectorPurger is the embedding index (agentic-kit's vectorstore.VectorStore
// satisfies it directly). Ids are the vector entries for the purged
// symbols.
type VectorPurger interface {
	Delete(ids ...string) error
}

// MemoryPurger is the anchored-memory file (<repo>/.fathom/memories.jsonl,
// AM-12): a memory's free-form claim text may QUOTE the purged content, so
// a purge that leaves it has not purged. internal/memory.FileStore's
// PurgeMatching satisfies it through a one-line adapter at the CLI.
type MemoryPurger interface {
	PurgeMemories(m PathMatcher) (int, error)
}

// PurgeOptions carries the two consent flags, the derived stores, and the
// test seam.
type PurgeOptions struct {
	// IncludeHistory also deletes claim_events for the affected claims
	// (§3.4). Refused unless Force is also set.
	IncludeHistory bool
	// Force is the second key for IncludeHistory. Nobody should walk
	// through that door by accident.
	Force bool
	// Cache is the result cache to flush wholly. nil is reported as an
	// absence with its reason, never as a successful zero (law 3).
	Cache CacheFlusher
	// Vectors is the embedding index to delete affected entries from. nil
	// is likewise reported as a named absence.
	Vectors VectorPurger
	// Memories is the repo's anchored-memory file. nil is reported as a
	// named absence (typically: the service's repo root is unknown).
	Memories MemoryPurger
	// Probes are extra raw-byte canaries the operator wants checked —
	// a leaked credential's literal value, typically. They are ALWAYS
	// probed, and a hit fails verification.
	Probes []string
	// BetweenExecuteAndVerify runs after the deletes commit and before the
	// verify pass. It exists so a test can inject an artificially
	// surviving row and prove the run FAILS loudly (§5); nil in
	// production. An options field, deliberately not a package global.
	BetweenExecuteAndVerify func()
}

// LocationCount is one row-count line of the report.
type LocationCount struct {
	Location string `json:"location"`
	Rows     int    `json:"rows"`
}

// Retention is a location the purge deliberately did NOT clear, with the
// count it kept and the reason. §3.4: "a disclosed retention, not a silent
// one."
type Retention struct {
	Location PurgeLocation `json:"location"`
	Rows     int           `json:"rows"`
	Reason   string        `json:"reason"`
}

// DerivedOutcome is one non-SQL store's result. Applied=false always
// carries a Reason; Failed marks a store that WAS wired and errored, which
// fails the verdict (an absent store is disclosed but does not — nothing
// was promised about it).
type DerivedOutcome struct {
	Location PurgeLocation `json:"location"`
	Applied  bool          `json:"applied"`
	Failed   bool          `json:"failed,omitempty"`
	Count    int           `json:"count,omitempty"`
	Reason   string        `json:"reason,omitempty"`
}

// RawProbeHit is one needle found in one file, with its byte offset.
type RawProbeHit struct {
	File   string `json:"file"`
	Needle string `json:"needle"`
	Offset int64  `json:"offset"`
}

// SkippedNeedle is a needle deliberately NOT probed, with the reason —
// typically a path that is still legitimately present under another
// service, where a hit would be a false alarm rather than residue.
type SkippedNeedle struct {
	Needle string `json:"needle"`
	Reason string `json:"reason"`
}

// RawProbe is the raw-bytes scan of the store file and its sidecars.
//
// Ran is true ONLY when at least one needle was scanned against at least
// one on-disk file and every scanned file was read to the end — a probe
// that scanned nothing NEVER reports Ran (V1). When Ran is false, Reason
// names why, and purgeVerdict maps the run to PurgeBytesUnchecked rather
// than PurgeVerified.
type RawProbe struct {
	Ran     bool            `json:"ran"`
	Reason  string          `json:"reason,omitempty"`
	Files   []string        `json:"files,omitempty"`
	Needles []string        `json:"needles,omitempty"`
	Hits    []RawProbeHit   `json:"hits,omitempty"`
	Skipped []SkippedNeedle `json:"skipped,omitempty"`
}

// PhysicalOutcome records §3.2's last row: what was done about the deleted
// rows' raw bytes, and what the probe found.
type PhysicalOutcome struct {
	Applied      bool     `json:"applied"`
	Reason       string   `json:"reason,omitempty"`
	SecureDelete bool     `json:"secure_delete"`
	Checkpointed bool     `json:"checkpointed"`
	Vacuumed     bool     `json:"vacuumed"`
	BytesBefore  int64    `json:"bytes_before"`
	BytesAfter   int64    `json:"bytes_after"`
	Probe        RawProbe `json:"probe"`
}

// PurgeVerdict is the run's outcome, and the probe's outcome is PART of
// the type (V1). There is deliberately no `Verified bool` anywhere in the
// report: a caller that wants to print success must switch on this, and
// the only success value requires the raw-bytes probe to have run clean.
type PurgeVerdict string

const (
	// PurgeVerified: every location's re-derived scan reports zero AND the
	// raw-bytes probe ran and found nothing. The only success value.
	PurgeVerified PurgeVerdict = "verified"
	// PurgeBytesUnchecked: the SQL re-scan reports zero everywhere, but
	// the raw-bytes probe did not run (nothing scannable, every needle
	// skipped as still-live elsewhere, or a read error). NOT success: the
	// deleted rows' bytes were never checked, and the CLI exits non-zero
	// with this state named.
	PurgeBytesUnchecked PurgeVerdict = "sql-zero-bytes-unchecked"
	// PurgeFailed: a survivor row, raw-bytes residue, or a wired derived
	// store that errored. The run names what and where.
	PurgeFailed PurgeVerdict = "failed"
)

// purgeVerdict is the ONE place a verdict is computed: survivors, derived
// failures and the probe outcome fold into the tri-state. Pure, so the
// V1 regressions (probe skipped → VERIFIED; read error → default verdict)
// are pinned by a table test.
func purgeVerdict(survivors []LocationCount, derived []DerivedOutcome, probe RawProbe) (PurgeVerdict, string) {
	if len(survivors) > 0 {
		var names []string
		for _, sv := range survivors {
			names = append(names, fmt.Sprintf("%s (%d row(s))", sv.Location, sv.Rows))
		}
		return PurgeFailed, "FAILED: content survived in " + strings.Join(names, ", ") +
			" — the purge is partial and must not be treated as done"
	}
	for _, d := range derived {
		if d.Failed {
			return PurgeFailed, fmt.Sprintf("FAILED: derived store %s errored: %s — a copy may remain there", d.Location, d.Reason)
		}
	}
	if len(probe.Hits) > 0 {
		var names []string
		for _, h := range probe.Hits {
			names = append(names, fmt.Sprintf("%q in %s at offset %d", h.Needle, h.File, h.Offset))
		}
		return PurgeFailed, "FAILED: the raw-bytes probe found residue the SQL re-scan cannot see: " + strings.Join(names, ", ")
	}
	if !probe.Ran {
		return PurgeBytesUnchecked, fmt.Sprintf(
			"NOT VERIFIED: every location re-scanned from its own path column reports zero, "+
				"but the raw-bytes probe did not run (%s) — the deleted rows' bytes were never checked, "+
				"so this run must not be treated as a verified purge", probe.Reason,
		)
	}
	return PurgeVerified, fmt.Sprintf(
		"every location re-scanned from its own path column reports zero, and a raw-bytes probe of %s found none of %d needle(s)",
		strings.Join(probe.Files, ", "), len(probe.Needles),
	)
}

// PurgeReport is the fourth phase of §3.3: per-location before/after
// counts, the disclosed retentions, the derived stores, the physical pass,
// and the verdict.
type PurgeReport struct {
	ServiceID string           `json:"service_id"`
	Resolved  []string         `json:"resolved"`
	Before    []LocationCount  `json:"before"`
	After     []LocationCount  `json:"after"`
	Retained  []Retention      `json:"retained,omitempty"`
	Derived   []DerivedOutcome `json:"derived,omitempty"`
	Physical  PhysicalOutcome  `json:"physical"`
	Survivors []LocationCount  `json:"survivors,omitempty"`
	// Verdict is purgeVerdict's tri-state; VerdictNote is its sentence —
	// what the verification covered when it passed, the failing location
	// or the unchecked bytes when it did not.
	Verdict     PurgeVerdict `json:"verdict"`
	VerdictNote string       `json:"verdict_note"`
	// HistoryNote is set only under --include-history --force, and states
	// that claim history integrity is now broken for the affected claims.
	HistoryNote string `json:"history_note,omitempty"`
	// Note carries §6's honest limit: erasure is scoped to the live store
	// and its derived indexes, never to snapshots or backups.
	Note string `json:"note"`
}

const purgeScopeNote = "erasure is scoped to fathom's live store and its derived indexes; " +
	"on a copy-on-write filesystem, or with any snapshotting backup in play, older copies of the store file may survive (§6)"

// PurgePlan is §3.3 phase 1 standing alone: what a purge WOULD remove,
// with nothing touched. The confirmation step renders this before anything
// is destroyed, and `fathom scope preview` reads the same resolution so
// preview, filter and purge can never disagree.
type PurgePlan struct {
	ServiceID string          `json:"service_id"`
	Paths     []string        `json:"paths"`
	Counts    []LocationCount `json:"counts"`
}

// ResolvePurge expands m against every location's own path column and
// returns the plan, deleting nothing. It reads raw SQL, deliberately below
// the suppression gate: a plan computed through a read filter would
// under-report exactly the rows the operator is trying to destroy.
func (s *SQLiteStore) ResolvePurge(ctx context.Context, serviceID string, m PathMatcher) (*PurgePlan, error) {
	if serviceID == "" {
		return nil, fmt.Errorf("store.ResolvePurge: a service is required")
	}
	if m == nil {
		return nil, fmt.Errorf("store.ResolvePurge: a path matcher is required")
	}
	plan, err := resolvePurgeTargets(ctx, s.db, serviceID, m, nil)
	if err != nil {
		return nil, err
	}
	return &PurgePlan{ServiceID: serviceID, Paths: plan.paths, Counts: plan.countsList()}, nil
}

// PurgePaths is the four-phase run of §3.3: resolve → execute → verify →
// report. It returns an unverified report rather than an error when
// verification fails: the caller needs the counts and the survivor's name
// to act, and an error would throw them away. The verdict is computed by
// purgeVerdict and the probe's outcome is part of it (V1).
func (s *SQLiteStore) PurgePaths(ctx context.Context, serviceID string, m PathMatcher, opts PurgeOptions) (*PurgeReport, error) {
	if serviceID == "" {
		return nil, fmt.Errorf("store.PurgePaths: a service is required (this refuses to guess at a fleet-wide purge)")
	}
	if m == nil {
		return nil, fmt.Errorf("store.PurgePaths: a path matcher is required")
	}
	if opts.IncludeHistory && !opts.Force {
		return nil, fmt.Errorf("store.PurgePaths: --include-history breaks claim-history integrity for the affected claims and needs --force as well (§3.4)")
	}

	// ---- 1. resolve, from EACH location's own path column ----
	plan, err := resolvePurgeTargets(ctx, s.db, serviceID, m, nil)
	if err != nil {
		return nil, err
	}

	rep := &PurgeReport{
		ServiceID: serviceID,
		Resolved:  plan.paths,
		Before:    plan.countsList(),
		Note:      purgeScopeNote,
	}

	// ---- 2. execute, on a secure_delete connection ----
	phys, err := s.executePurge(ctx, serviceID, plan, opts)
	if err != nil {
		return nil, err
	}
	rep.Physical = phys

	if opts.IncludeHistory {
		rep.HistoryNote = fmt.Sprintf(
			"claim history integrity is now broken for %d claim(s): their claim_events rows were deleted, so the append-only evidence log no longer explains their verdicts",
			len(plan.claimIDs),
		)
	} else {
		rep.Retained = append(rep.Retained, Retention{
			Location: LocClaimEvents,
			Rows:     plan.counts[LocClaimEvents],
			Reason: "append-only evidence log; its rows carry claim ids and verdicts rather than source content, " +
				"and deleting them breaks the integrity of every claim that references them (§3.4). " +
				"Use --include-history --force to delete them anyway.",
		})
	}

	// ---- 2b. derived stores (§3.2) ----
	rep.Derived = purgeDerived(plan, m, opts)

	if opts.BetweenExecuteAndVerify != nil {
		opts.BetweenExecuteAndVerify()
	}

	// ---- 3. verify, re-derived from each table's own path column ----
	after, err := resolvePurgeTargets(ctx, s.db, serviceID, m, plan)
	if err != nil {
		return nil, err
	}
	rep.After = after.countsList()

	retained := map[PurgeLocation]bool{}
	if !opts.IncludeHistory {
		retained[LocClaimEvents] = true
	}
	for _, loc := range PurgeLocations() {
		if retained[loc] {
			continue
		}
		if n := after.counts[loc]; n > 0 {
			rep.Survivors = append(rep.Survivors, LocationCount{Location: string(loc), Rows: n})
		}
	}

	// ---- 3b. raw-bytes probe, a mechanism SQL cannot substitute for ----
	rep.Physical.Probe = s.rawBytesProbe(ctx, plan, opts)

	rep.Verdict, rep.VerdictNote = purgeVerdict(rep.Survivors, rep.Derived, rep.Physical.Probe)
	return rep, nil
}

// ---------------------------------------------------------------- resolve

// purgeTargets is one resolution pass: what matched, where, and the derived
// keys (claim ids, packages, vector entry ids) the delete and the
// verification both need.
type purgeTargets struct {
	paths      []string
	byLocation map[PurgeLocation][]string
	counts     map[PurgeLocation]int
	claimIDs   []string
	packages   []string
	symbolIDs  []string
}

func (p *purgeTargets) countsList() []LocationCount {
	out := make([]LocationCount, 0, len(PurgeLocations()))
	for _, loc := range PurgeLocations() {
		out = append(out, LocationCount{Location: string(loc), Rows: p.counts[loc]})
	}
	return out
}

// resolvePurgeTargets expands m against EACH LOCATION'S OWN path column —
// observations.source_ref, fingerprints.source_ref, doc_summaries.source_ref,
// knowledge_cards.file, typed_edges.from_file, and core.ParseCiteRef over
// claim_links.target_ref (§3.3, SP-11). ListSourceRefs is deliberately NOT
// used: it is observations ∪ fingerprints only, so every row anchored by
// another key would be resolved never, deleted never, and checked never.
//
// Paths are normalized with scope.NormalizeRef — the same function the read
// filter matches through — so suppression, preview and purge can never
// disagree about what a stored locator means.
//
// seed is nil on the planning pass. On the VERIFICATION pass the plan is
// passed back in, so the re-scan additionally re-checks the plan's own keys
// (claim ids, typed-edge packages) — those rows have no path column of
// their own, so a pattern-only rescan could not see a survivor there. The
// path-column rescan remains fully independent of the deletion, which is
// what §3.3 requires; the seed only ever ADDS checks.
func resolvePurgeTargets(ctx context.Context, db *sql.DB, serviceID string, m PathMatcher, seed *purgeTargets) (*purgeTargets, error) {
	t := &purgeTargets{
		byLocation: map[PurgeLocation][]string{},
		counts:     map[PurgeLocation]int{},
	}
	pathSet := map[string]bool{}
	locSets := map[PurgeLocation]map[string]bool{}
	claimSet := map[string]bool{}
	pkgSet := map[string]bool{}
	symbolSet := map[string]bool{}

	hit := func(loc PurgeLocation, raw string) {
		t.counts[loc]++
		pathSet[scope.NormalizeRef(raw)] = true
		if locSets[loc] == nil {
			locSets[loc] = map[string]bool{}
		}
		locSets[loc][raw] = true
	}

	// observations.source_ref (+ the vector entry id for each symbol)
	rows, err := db.QueryContext(ctx, `SELECT source_ref, package, symbol FROM observations WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("store.resolvePurgeTargets observations: %w", err)
	}
	for rows.Next() {
		var ref, pkg, sym string
		if err := rows.Scan(&ref, &pkg, &sym); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store.resolvePurgeTargets observations scan: %w", err)
		}
		if !m.Match(scope.NormalizeRef(ref)) {
			continue
		}
		hit(LocObservations, ref)
		if id := vectorEntryID(pkg, sym); id != "" {
			symbolSet[id] = true
		}
	}
	if err := closeRows(rows, "observations"); err != nil {
		return nil, err
	}

	// The single-column locations, each read from its OWN path column.
	for _, q := range []struct {
		loc   PurgeLocation
		query string
	}{
		{LocFingerprints, `SELECT source_ref FROM fingerprints WHERE service_id = ?`},
		{LocDocSummaries, `SELECT source_ref FROM doc_summaries WHERE service_id = ?`},
		{LocKnowledgeCards, `SELECT file FROM knowledge_cards WHERE service_id = ? AND file <> ''`},
		{LocFileStats, `SELECT path FROM file_stats WHERE service_id = ?`},
	} {
		vals, err := queryStrings(ctx, db, q.query, serviceID)
		if err != nil {
			return nil, fmt.Errorf("store.resolvePurgeTargets %s: %w", q.loc, err)
		}
		for _, v := range vals {
			if m.Match(scope.NormalizeRef(v)) {
				hit(q.loc, v)
			}
		}
	}

	// typed_edges.from_file, carrying the package whose fingerprint must be
	// re-armed.
	rows, err = db.QueryContext(ctx, `SELECT from_file, pkg_path FROM typed_edges WHERE service_id = ? AND from_file <> ''`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("store.resolvePurgeTargets typed_edges: %w", err)
	}
	for rows.Next() {
		var file, pkg string
		if err := rows.Scan(&file, &pkg); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store.resolvePurgeTargets typed_edges scan: %w", err)
		}
		if !m.Match(scope.NormalizeRef(file)) {
			continue
		}
		hit(LocTypedEdges, file)
		if pkg != "" {
			pkgSet[pkg] = true
		}
	}
	if err := closeRows(rows, "typed_edges"); err != nil {
		return nil, err
	}

	// claim_links.target_ref, through the citation grammar it was written
	// with. A rel with no citation (scopes / scope-fp) carries no path.
	rows, err = db.QueryContext(ctx, `SELECT claim_id, target_ref FROM claim_links WHERE service_id = ?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("store.resolvePurgeTargets claim_links: %w", err)
	}
	for rows.Next() {
		var claimID, ref string
		if err := rows.Scan(&claimID, &ref); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store.resolvePurgeTargets claim_links scan: %w", err)
		}
		path, _, _, ok := core.ParseCiteRef(ref)
		if !ok {
			path = ref
		}
		if !m.Match(scope.NormalizeRef(path)) {
			continue
		}
		pathSet[scope.NormalizeRef(path)] = true
		claimSet[claimID] = true
	}
	if err := closeRows(rows, "claim_links"); err != nil {
		return nil, err
	}

	if seed != nil {
		for _, id := range seed.claimIDs {
			claimSet[id] = true
		}
		for _, p := range seed.packages {
			pkgSet[p] = true
		}
	}

	t.claimIDs = sortedSetKeys(claimSet)
	t.packages = sortedSetKeys(pkgSet)
	t.symbolIDs = sortedSetKeys(symbolSet)
	t.paths = sortedSetKeys(pathSet)
	for loc, set := range locSets {
		t.byLocation[loc] = sortedSetKeys(set)
	}

	// The keyed locations: counted from the claim/package sets, because
	// they carry no path column of their own.
	if len(t.claimIDs) > 0 {
		n, err := countIn(ctx, db, `SELECT count(*) FROM claim_links WHERE service_id = ? AND claim_id IN (%s)`, serviceID, t.claimIDs)
		if err != nil {
			return nil, err
		}
		t.counts[LocClaimLinks] = n

		if n, err = countIn(ctx, db, `SELECT count(*) FROM semantic_claims WHERE service_id = ? AND claim_id IN (%s)`, serviceID, t.claimIDs); err != nil {
			return nil, err
		}
		t.counts[LocSemanticClaims] = n

		// claim_events has no service_id column (see ClaimEventsSince), so
		// membership is resolved through the claim ids, exactly as
		// RemoveService does.
		if n, err = countInNoService(ctx, db, `SELECT count(*) FROM claim_events WHERE claim_id IN (%s)`, t.claimIDs); err != nil {
			return nil, err
		}
		t.counts[LocClaimEvents] = n
	}
	if len(t.packages) > 0 {
		n, err := countIn(ctx, db, `SELECT count(*) FROM typed_edge_fingerprints WHERE service_id = ? AND pkg_path IN (%s)`, serviceID, t.packages)
		if err != nil {
			return nil, err
		}
		t.counts[LocTypedEdgeFingerprints] = n
	}
	return t, nil
}

// vectorEntryID mirrors internal/vector's symbolID ("<Package>.<Name>", or
// the bare name when there is no package). Duplicated rather than imported
// so store keeps no dependency on the optional semantic layer; the
// duplication is one line and is named here so a change there is findable.
func vectorEntryID(pkg, symbol string) string {
	if symbol == "" {
		return ""
	}
	if pkg == "" {
		return symbol
	}
	return pkg + "." + symbol
}

// ---------------------------------------------------------------- execute

// executePurge deletes every resolved row on a PINNED connection with
// secure_delete ON, then checkpoints the WAL to truncation and VACUUMs.
// Pinning matters: secure_delete is a per-connection pragma, so the pragma
// and the DELETEs must be the same physical connection or the freed pages
// keep their old bytes (§3.2's last row, reproduced during review).
func (s *SQLiteStore) executePurge(ctx context.Context, serviceID string, plan *purgeTargets, opts PurgeOptions) (PhysicalOutcome, error) {
	out := PhysicalOutcome{}
	if s.path != "" {
		if fi, err := os.Stat(s.path); err == nil {
			out.BytesBefore = fi.Size()
		}
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return out, fmt.Errorf("store.PurgePaths pin connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete=ON`); err != nil {
		return out, fmt.Errorf("store.PurgePaths secure_delete: %w", err)
	}
	var sd int
	if err := conn.QueryRowContext(ctx, `PRAGMA secure_delete`).Scan(&sd); err != nil {
		return out, fmt.Errorf("store.PurgePaths read back secure_delete: %w", err)
	}
	out.SecureDelete = sd >= 1

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("store.PurgePaths begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	del := func(query string, keys []string, withService bool) error {
		if len(keys) == 0 {
			return nil
		}
		return execIn(ctx, tx, query, serviceID, keys, withService)
	}

	if err := del(`DELETE FROM observations WHERE service_id = ? AND source_ref IN (%s)`, plan.byLocation[LocObservations], true); err != nil {
		return out, err
	}
	if err := del(`DELETE FROM fingerprints WHERE service_id = ? AND source_ref IN (%s)`, plan.byLocation[LocFingerprints], true); err != nil {
		return out, err
	}
	// file_stats names the purged paths too (RT-76), resolved by its own
	// path column — an oversize file has a row but no fingerprint; the
	// summary's counts included them and is rebuilt by backfill at first use.
	if err := del(`DELETE FROM file_stats WHERE service_id = ? AND path IN (%s)`, plan.byLocation[LocFileStats], true); err != nil {
		return out, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM service_summary WHERE service_id = ?`, serviceID); err != nil {
		return out, fmt.Errorf("store.PurgePaths summary: %w", err)
	}
	if err := del(`DELETE FROM doc_summaries WHERE service_id = ? AND source_ref IN (%s)`, plan.byLocation[LocDocSummaries], true); err != nil {
		return out, err
	}
	if err := del(`DELETE FROM knowledge_cards WHERE service_id = ? AND file IN (%s)`, plan.byLocation[LocKnowledgeCards], true); err != nil {
		return out, err
	}
	if err := del(`DELETE FROM typed_edges WHERE service_id = ? AND from_file IN (%s)`, plan.byLocation[LocTypedEdges], true); err != nil {
		return out, err
	}
	// The package fingerprint must go too, or the enrichment cache treats
	// the package as already loaded and the purged edges never rebuild.
	if err := del(`DELETE FROM typed_edge_fingerprints WHERE service_id = ? AND pkg_path IN (%s)`, plan.packages, true); err != nil {
		return out, err
	}
	if opts.IncludeHistory {
		// Deleted FIRST, while the claims still resolve membership — the
		// same ordering RemoveService uses.
		if err := del(`DELETE FROM claim_events WHERE claim_id IN (%s)`, plan.claimIDs, false); err != nil {
			return out, err
		}
	}
	if err := del(`DELETE FROM claim_links WHERE service_id = ? AND claim_id IN (%s)`, plan.claimIDs, true); err != nil {
		return out, err
	}
	if err := del(`DELETE FROM semantic_claims WHERE service_id = ? AND claim_id IN (%s)`, plan.claimIDs, true); err != nil {
		return out, err
	}
	if s.path != "" {
		// The VACUUM below may renumber the rowids the call index keys on:
		// every service reads as unindexed from this commit until the
		// rebuild after it commits, never as a complete index over stale
		// rowids (RT-77, I38).
		if err := markCallIndexStale(ctx, tx); err != nil {
			return out, fmt.Errorf("store.PurgePaths call index: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return out, fmt.Errorf("store.PurgePaths commit: %w", err)
	}
	committed = true

	purgePhysicalResidue(ctx, conn, s.path, &out)
	if s.path != "" {
		// VACUUM may renumber observations' rowids, which the call index
		// keys on (RT-77): rebuild it in place for every service. The
		// services were marked unindexed before the VACUUM, so a reader in
		// between is told the index is absent (I38).
		if purgeAfterVacuumHook != nil {
			purgeAfterVacuumHook()
		}
		if err := rebuildCallIndex(ctx, conn); err != nil {
			return out, fmt.Errorf("store.PurgePaths rebuild call index: %w", err)
		}
	}
	return out, nil
}

// purgeAfterVacuumHook, when set, runs between a purge's VACUUM and the
// call index's rebuild (tests: the window a reader must see as unindexed).
var purgeAfterVacuumHook func()

// purgePhysicalResidue is §3.2's last row, and the one the first draft of
// the design was missing entirely: the deleted rows' RAW BYTES.
//
// WAL-mode SQLite with no secure_delete keeps them twice over — in the
// freed pages of the main file, and verbatim in the -wal. Reproduced
// against this repo's own store during review: after a purge the query
// count was zero while the canary bytes were still returned by `strings
// fleet.sqlite`, and the residue would have propagated into every backup
// taken afterwards.
//
// Three things have to be true, and the report states each one rather than
// asserting a blanket success:
//
//   - secure_delete was ON for the DELETEs themselves (the caller's pinned
//     connection — a pragma set afterwards zeroes nothing);
//   - the WAL was checkpointed to TRUNCATE, discarding the pre-delete
//     frames that still hold the original page images;
//   - VACUUM rebuilt the file, so no free page carries old content.
//
// The checkpoint runs both before and after VACUUM: before, to drop the
// original frames; after, so the rebuilt pages land in the main file and
// the -wal is left empty rather than holding the vacuumed image.
//
// An in-memory store reports Applied=false with the REASON — there is no
// file, no -wal and no free pages on disk — never a misleading success
// (law 3).
func purgePhysicalResidue(ctx context.Context, conn *sql.Conn, path string, out *PhysicalOutcome) {
	if path == "" {
		out.Applied = false
		out.Reason = "in-memory store: there is no store file, no -wal and no free pages on disk, so there is nothing physical to zero or probe"
		return
	}

	out.Checkpointed = checkpointTruncate(ctx, conn) == nil
	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		out.Reason = fmt.Sprintf("VACUUM failed: %v (free pages may still hold the deleted rows' bytes)", err)
	} else {
		out.Vacuumed = true
	}
	if err := checkpointTruncate(ctx, conn); err != nil && out.Checkpointed {
		out.Checkpointed = false
	}
	out.Applied = out.SecureDelete && out.Checkpointed && out.Vacuumed
	if !out.Applied && out.Reason == "" {
		out.Reason = fmt.Sprintf("physical pass incomplete (secure_delete=%v checkpoint=%v vacuum=%v)", out.SecureDelete, out.Checkpointed, out.Vacuumed)
	}
	if fi, err := os.Stat(path); err == nil {
		out.BytesAfter = fi.Size()
	}
}

// checkpointTruncate runs PRAGMA wal_checkpoint(TRUNCATE) and reports a
// busy checkpoint as an error — a checkpoint that could not complete leaves
// the deleted rows verbatim in the -wal.
func checkpointTruncate(ctx context.Context, conn *sql.Conn) error {
	var busy, log, checkpointed int
	if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return fmt.Errorf("wal_checkpoint(TRUNCATE) was busy: another connection holds the WAL")
	}
	return nil
}

// Checkpoint runs PRAGMA wal_checkpoint(TRUNCATE) against the store,
// flushing the write-ahead log into the main database file and truncating
// it. Exported so a caller that needs the on-disk file to be current — a
// backup, or a raw-bytes assertion — does not have to close the store
// first. A no-op with no error for an in-memory store.
func (s *SQLiteStore) Checkpoint(ctx context.Context) error {
	if s.path == "" {
		return nil
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store.Checkpoint: %w", err)
	}
	defer conn.Close()
	return checkpointTruncate(ctx, conn)
}

// ---------------------------------------------------------------- derived

// purgeDerived clears the non-SQL copies (§3.2) and reports each one's
// outcome. A store that was not wired reports ABSENT with its reason — law
// 3: never a comforting zero for something nobody looked at. A store that
// WAS wired and errored reports Failed, which fails the verdict.
func purgeDerived(plan *purgeTargets, m PathMatcher, opts PurgeOptions) []DerivedOutcome {
	var out []DerivedOutcome

	if opts.Cache != nil {
		opts.Cache.Invalidate()
		out = append(out, DerivedOutcome{
			Location: LocResultCache, Applied: true,
			Reason: "flushed wholly: the cache is keyed by query, not by ref, so selective removal is unsound",
		})
	} else {
		out = append(out, DerivedOutcome{
			Location: LocResultCache, Applied: false,
			Reason: "no result cache was wired into this purge — if a cache is live in another process, an answer cached before the purge may still be served from it",
		})
	}

	switch {
	case opts.Vectors == nil:
		out = append(out, DerivedOutcome{
			Location: LocVectorIndex, Applied: false,
			Reason: "no vector index was wired into this purge — embeddings of the purged content survive wherever one is live",
		})
	case len(plan.symbolIDs) == 0:
		out = append(out, DerivedOutcome{
			Location: LocVectorIndex, Applied: true,
			Reason: "no symbols were resolved, so no embeddings could exist for the purged paths",
		})
	default:
		if err := opts.Vectors.Delete(plan.symbolIDs...); err != nil {
			out = append(out, DerivedOutcome{
				Location: LocVectorIndex, Applied: false, Failed: true,
				Reason: "vector delete failed: " + err.Error(),
			})
		} else {
			out = append(out, DerivedOutcome{
				Location: LocVectorIndex, Applied: true, Count: len(plan.symbolIDs),
				Reason: "deleted the vector entries for every purged symbol",
			})
		}
	}

	// <repo>/.fathom/memories.jsonl (AM-12): a memory's free-form claim
	// text may QUOTE the purged content; a purge that leaves it has not
	// purged.
	if opts.Memories == nil {
		out = append(out, DerivedOutcome{
			Location: LocMemories, Applied: false,
			Reason: "no memories store was wired into this purge — the service's repo root is unknown here, so <repo>/.fathom/memories.jsonl was NOT checked",
		})
	} else if n, err := opts.Memories.PurgeMemories(m); err != nil {
		out = append(out, DerivedOutcome{
			Location: LocMemories, Applied: false, Failed: true,
			Reason: "memories purge failed: " + err.Error(),
		})
	} else {
		out = append(out, DerivedOutcome{
			Location: LocMemories, Applied: true, Count: n,
			Reason: fmt.Sprintf("deleted %d memorie(s) whose file/path anchors match the pattern", n),
		})
	}
	return out
}

// ------------------------------------------------------------- raw bytes

// rawBytesProbe scans the store file and its sidecars for the purged
// content, byte by byte. §3.3: verification must not be derived from the
// deletion — a re-query can only confirm the delete it came from — so this
// check uses a DIFFERENT MECHANISM entirely, reading the file rather than
// the database, and can therefore see residue in free pages and in the
// -wal that no SQL statement can reach.
//
// A path that is still legitimately present under another service is
// SKIPPED with its reason rather than reported as a hit: a false alarm on a
// shared relative path would train the operator to ignore the probe. What
// that skip must NEVER do is silently upgrade the run to verified — when it
// empties the needle set, Ran stays false and purgeVerdict lands on
// PurgeBytesUnchecked (V1).
func (s *SQLiteStore) rawBytesProbe(ctx context.Context, plan *purgeTargets, opts PurgeOptions) RawProbe {
	probe := RawProbe{}
	if s.path == "" {
		probe.Reason = "in-memory store: there is no file to read"
		return probe
	}

	needleSet := map[string]bool{}
	for _, n := range opts.Probes {
		if n != "" {
			needleSet[n] = true
		}
	}
	for _, p := range plan.paths {
		if p == "" {
			continue
		}
		live, err := s.pathStillLive(ctx, p)
		if err != nil {
			probe.Skipped = append(probe.Skipped, SkippedNeedle{Needle: p, Reason: "could not check whether the path is still live elsewhere: " + err.Error()})
			continue
		}
		if live {
			probe.Skipped = append(probe.Skipped, SkippedNeedle{
				Needle: p,
				Reason: "still present under another service in this store, so a byte hit would be that row, not residue",
			})
			continue
		}
		needleSet[p] = true
	}
	probe.Needles = sortedSetKeys(needleSet)
	sort.Slice(probe.Skipped, func(i, j int) bool { return probe.Skipped[i].Needle < probe.Skipped[j].Needle })

	if len(probe.Needles) == 0 {
		probe.Reason = "nothing to probe: every resolved path is still live under another service and no explicit canary was given (--probe '<literal>' probes the content itself)"
		return probe
	}

	for _, f := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		hits, err := scanFileForNeedles(f, probe.Needles)
		if err != nil {
			// A probe that could not read is a probe that did NOT run —
			// the previous build let this fall through to the default
			// verdict and printed VERIFIED over bytes nobody checked (V1).
			probe.Ran = false
			probe.Reason = fmt.Sprintf("could not read %s: %v", f, err)
			return probe
		}
		probe.Files = append(probe.Files, f)
		probe.Hits = append(probe.Hits, hits...)
	}
	if len(probe.Files) == 0 {
		probe.Reason = "no store file could be opened for scanning"
		return probe
	}
	sort.Slice(probe.Hits, func(i, j int) bool {
		if probe.Hits[i].File != probe.Hits[j].File {
			return probe.Hits[i].File < probe.Hits[j].File
		}
		if probe.Hits[i].Needle != probe.Hits[j].Needle {
			return probe.Hits[i].Needle < probe.Hits[j].Needle
		}
		return probe.Hits[i].Offset < probe.Hits[j].Offset
	})
	// Ran is earned, never assumed: needles exist, at least one file was
	// scanned, and every scan completed (V1: "a probe that scanned nothing
	// NEVER reports Ran").
	probe.Ran = true
	return probe
}

// pathStillLive reports whether path anchors any row anywhere in the store,
// across every service — the check that keeps a shared relative path from
// producing a false residue alarm.
func (s *SQLiteStore) pathStillLive(ctx context.Context, path string) (bool, error) {
	const q = `
SELECT EXISTS(SELECT 1 FROM observations   WHERE source_ref = ?)
    OR EXISTS(SELECT 1 FROM fingerprints   WHERE source_ref = ?)
    OR EXISTS(SELECT 1 FROM doc_summaries  WHERE source_ref = ?)
    OR EXISTS(SELECT 1 FROM knowledge_cards WHERE file = ?)
    OR EXISTS(SELECT 1 FROM typed_edges    WHERE from_file = ?)`
	var live bool
	if err := s.db.QueryRowContext(ctx, q, path, path, path, path, path).Scan(&live); err != nil {
		return false, err
	}
	return live, nil
}

// scanFileForNeedles streams path in overlapping chunks so a needle
// straddling a chunk boundary is still found, and reports the first offset
// of each needle.
func scanFileForNeedles(path string, needles []string) ([]RawProbeHit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	longest := 0
	for _, n := range needles {
		if len(n) > longest {
			longest = len(n)
		}
	}
	const chunk = 1 << 20
	overlap := longest
	buf := make([]byte, chunk+overlap)

	found := map[string]bool{}
	var hits []RawProbeHit
	var base int64
	filled := 0
	for {
		n, err := f.Read(buf[filled:])
		if n > 0 {
			window := buf[:filled+n]
			for _, needle := range needles {
				if found[needle] {
					continue
				}
				if i := bytes.Index(window, []byte(needle)); i >= 0 {
					found[needle] = true
					hits = append(hits, RawProbeHit{File: path, Needle: needle, Offset: base + int64(i)})
				}
			}
			// Carry the tail forward so a needle spanning the boundary is
			// still contiguous in the next window.
			keep := overlap
			if keep > filled+n {
				keep = filled + n
			}
			copy(buf, window[len(window)-keep:])
			base += int64(len(window) - keep)
			filled = keep
		}
		if err != nil {
			break
		}
	}
	return hits, nil
}

// ------------------------------------------------------------- SQL helpers

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// sqlChunk bounds an IN-clause below SQLite's parameter limit.
const sqlChunk = 400

func execIn(ctx context.Context, ex execer, query, serviceID string, keys []string, withService bool) error {
	for start := 0; start < len(keys); start += sqlChunk {
		end := start + sqlChunk
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[start:end]
		args := make([]any, 0, len(batch)+1)
		if withService {
			args = append(args, serviceID)
		}
		for _, k := range batch {
			args = append(args, k)
		}
		if _, err := ex.ExecContext(ctx, fmt.Sprintf(query, placeholders(len(batch))), args...); err != nil {
			return fmt.Errorf("store.PurgePaths delete: %w", err)
		}
	}
	return nil
}

func countIn(ctx context.Context, db *sql.DB, query, serviceID string, keys []string) (int, error) {
	return countInner(ctx, db, query, serviceID, keys, true)
}

func countInNoService(ctx context.Context, db *sql.DB, query string, keys []string) (int, error) {
	return countInner(ctx, db, query, "", keys, false)
}

func countInner(ctx context.Context, db *sql.DB, query, serviceID string, keys []string, withService bool) (int, error) {
	total := 0
	for start := 0; start < len(keys); start += sqlChunk {
		end := start + sqlChunk
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[start:end]
		args := make([]any, 0, len(batch)+1)
		if withService {
			args = append(args, serviceID)
		}
		for _, k := range batch {
			args = append(args, k)
		}
		var n int
		if err := db.QueryRowContext(ctx, fmt.Sprintf(query, placeholders(len(batch))), args...).Scan(&n); err != nil {
			return 0, fmt.Errorf("store.PurgePaths count: %w", err)
		}
		total += n
	}
	return total, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func queryStrings(ctx context.Context, db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func closeRows(rows *sql.Rows, what string) error {
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("store.resolvePurgeTargets %s rows: %w", what, err)
	}
	return rows.Close()
}

// sortedSetKeys is sortedKeys with a nil (not empty-slice) result for an
// empty set, so an absent list serialises as absent rather than as [].
func sortedSetKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	return sortedKeys(set)
}
