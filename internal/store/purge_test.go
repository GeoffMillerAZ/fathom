package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	purgeSvc     = "svc"
	secretFile   = "secrets/token.go"
	secretCanary = "AKIA-NOT-A-REAL-KEY-0123456789"
	keptFile     = "internal/keep.go"
)

func secretsMatcher() *filter.Matcher { return filter.New([]string{"secrets/**"}) }

// seedPurgeFixture writes one row into EVERY location §3.2 enumerates, for
// a purged path and for a path that must survive. The canary string is
// embedded in the row bodies so a raw-bytes probe of the store file has
// something to find if the delete leaves residue.
func seedPurgeFixture(t *testing.T, s store.Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	if err := s.PutObservations(ctx, []core.Observation{
		{
			ServiceID: purgeSvc, SourceRef: secretFile, SourceKind: core.KindGoConst, EvidenceTier: core.TierVerified,
			Symbol: "Token", Package: "secrets", Claim: "const Token = \"" + secretCanary + "\"", IngestedAt: now,
		},
		{
			ServiceID: purgeSvc, SourceRef: keptFile, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
			Symbol: "Keep", Package: "internal", Claim: "func Keep()", IngestedAt: now,
		},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	for _, ref := range []string{secretFile, keptFile} {
		if err := s.SetFingerprint(ctx, purgeSvc, ref, "hash-of-"+ref); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	if err := s.PutDocSummaries(ctx, []store.DocSummary{
		{ServiceID: purgeSvc, SourceRef: secretFile, Title: "token", Summary: "documents " + secretCanary},
		{ServiceID: purgeSvc, SourceRef: keptFile, Title: "keep", Summary: "harmless"},
	}); err != nil {
		t.Fatalf("PutDocSummaries: %v", err)
	}
	if err := s.PutKnowledgeCards(ctx, []store.KnowledgeCard{
		{ServiceID: purgeSvc, Symbol: "Token", File: secretFile, Summary: "an LLM summary quoting " + secretCanary},
		{ServiceID: purgeSvc, Symbol: "Keep", File: keptFile, Summary: "harmless"},
	}); err != nil {
		t.Fatalf("PutKnowledgeCards: %v", err)
	}
	if err := s.PutTypedEdges(ctx, purgeSvc, "secrets", []core.Edge{
		{FromService: purgeSvc, FromFile: secretFile, FromSymbol: "secrets.Token", ToSymbol: "fmt.Sprintf", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges secrets: %v", err)
	}
	if err := s.SetTypedEdgePackageFingerprint(ctx, purgeSvc, "secrets", "pkg-hash"); err != nil {
		t.Fatalf("SetTypedEdgePackageFingerprint: %v", err)
	}
	if err := s.PutTypedEdges(ctx, purgeSvc, "internal", []core.Edge{
		{FromService: purgeSvc, FromFile: keptFile, FromSymbol: "internal.Keep", ToSymbol: "fmt.Sprintf", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges internal: %v", err)
	}
	if err := s.SetTypedEdgePackageFingerprint(ctx, purgeSvc, "internal", "pkg-hash-2"); err != nil {
		t.Fatalf("SetTypedEdgePackageFingerprint: %v", err)
	}
	if err := s.PutClaims(ctx,
		[]core.SemanticClaim{
			{ServiceID: purgeSvc, ClaimID: "c-secret", Layer: "l", Subject: "secrets", Rule: "the token is " + secretCanary},
			{ServiceID: purgeSvc, ClaimID: "c-kept", Layer: "l", Subject: "internal", Rule: "harmless"},
		},
		[]core.ClaimLink{
			{ClaimID: "c-secret", Rel: core.RelCites, TargetRef: core.CiteRef(secretFile, 1, 2), AnchorText: secretCanary},
			{ClaimID: "c-kept", Rel: core.RelCites, TargetRef: core.CiteRef(keptFile, 1, 2), AnchorText: "harmless"},
		}); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	for _, ev := range []core.ClaimEvent{
		{ClaimID: "c-secret", Verdict: "corroborated", Author: "someone", At: now},
		{ClaimID: "c-kept", Verdict: "corroborated", Author: "someone", At: now},
	} {
		if err := s.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
	}
}

func openFileStore(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fleet.sqlite")
	s, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func locationCount(t *testing.T, counts []store.LocationCount, loc string) int {
	t.Helper()
	for _, c := range counts {
		if c.Location == loc {
			return c.Rows
		}
	}
	t.Fatalf("location %q missing from the report (%+v) — §3.2's table is the design; every location must be reported", loc, counts)
	return -1
}

// TestPurgePathsClearsEveryTable is SP-5: the purge removes rows from every
// location docs/design/scope-and-purge.md §3.2 enumerates, proven per
// table, and leaves the untouched path completely intact.
func TestPurgePathsClearsEveryTable(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)

	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{secretCanary}})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}

	// Every location in the design's table must appear in the report, both
	// before and after — including the ones that held nothing, so a reader
	// can tell "checked, empty" from "never looked".
	for _, loc := range store.PurgeLocations() {
		_ = locationCount(t, rep.Before, string(loc))
		_ = locationCount(t, rep.After, string(loc))
	}

	// Rows really were there before…
	for _, loc := range []store.PurgeLocation{
		store.LocObservations, store.LocFingerprints, store.LocDocSummaries,
		store.LocKnowledgeCards, store.LocTypedEdges, store.LocTypedEdgeFingerprints,
		store.LocSemanticClaims, store.LocClaimLinks,
	} {
		if n := locationCount(t, rep.Before, string(loc)); n == 0 {
			t.Fatalf("fixture did not populate %s — the test would prove nothing", loc)
		}
		if n := locationCount(t, rep.After, string(loc)); n != 0 {
			t.Errorf("%s still holds %d purged row(s) after the purge", loc, n)
		}
	}

	if rep.Verdict != store.PurgeVerified {
		t.Fatalf("purge did not verify: %s = %s (survivors: %+v)", rep.Verdict, rep.VerdictNote, rep.Survivors)
	}

	// …and the unrelated path survived, every location intact.
	obs, err := s.QueryObservations(ctx, purgeSvc, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].SourceRef != keptFile {
		t.Fatalf("observations after purge = %+v, want only %s", obs, keptFile)
	}
	cards, err := s.KnowledgeCards(ctx, purgeSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].File != keptFile {
		t.Fatalf("knowledge_cards after purge = %+v", cards)
	}
	docs, err := s.DocSummaries(ctx, purgeSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].SourceRef != keptFile {
		t.Fatalf("doc_summaries after purge = %+v", docs)
	}
	edges, err := s.TypedEdges(ctx, purgeSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 || edges[0].FromFile != keptFile {
		t.Fatalf("typed_edges after purge = %+v", edges)
	}
	if _, found, err := s.TypedEdgePackageFingerprint(ctx, purgeSvc, "secrets"); err != nil || found {
		t.Fatalf("typed_edge_fingerprints kept the purged package (found=%v err=%v)", found, err)
	}
	if _, found, err := s.TypedEdgePackageFingerprint(ctx, purgeSvc, "internal"); err != nil || !found {
		t.Fatalf("typed_edge_fingerprints lost the UNpurged package (found=%v err=%v)", found, err)
	}
	claims, err := s.ClaimsForService(ctx, purgeSvc, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 || claims[0].ClaimID != "c-kept" {
		t.Fatalf("semantic_claims after purge = %+v", claims)
	}
	links, err := s.ClaimLinks(ctx, purgeSvc, []string{"c-secret", "c-kept"})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].ClaimID != "c-kept" {
		t.Fatalf("claim_links after purge = %+v", links)
	}
	// The hash gate must be re-armed: leaving the fingerprint would make a
	// re-ingest silently skip the file (§3.2's fingerprints row).
	if _, found, err := s.Fingerprint(ctx, purgeSvc, secretFile); err != nil || found {
		t.Fatalf("fingerprint survived the purge (found=%v err=%v) — re-ingest would silently skip the file", found, err)
	}
}

// TestClaimEventsRetentionDisclosedStore is SP-9's store half (§3.4): claim
// history is RETAINED by default and the retention is disclosed with its
// count — never silently kept, never silently dropped — and deleted only
// when the caller explicitly asks and forces.
func TestClaimEventsRetentionDisclosedStore(t *testing.T) {
	ctx := context.Background()

	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)
	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	var found bool
	for _, r := range rep.Retained {
		if r.Location == store.LocClaimEvents {
			found = true
			if r.Rows != 1 {
				t.Errorf("retained claim_events count = %d, want 1", r.Rows)
			}
			if strings.TrimSpace(r.Reason) == "" {
				t.Error("claim_events retention has no reason — a silent retention is exactly what §3.4 forbids")
			}
		}
	}
	if !found {
		t.Fatalf("claim_events retention was not disclosed at all: %+v", rep.Retained)
	}
	evs, err := s.ClaimEvents(ctx, "c-secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("claim_events for the purged claim = %d, want 1 (retained by default)", len(evs))
	}

	// --include-history without --force is refused, and nothing is deleted.
	s2, _ := openFileStore(t)
	seedPurgeFixture(t, s2)
	if _, err := s2.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{IncludeHistory: true}); err == nil {
		t.Fatal("IncludeHistory without Force was accepted")
	}
	if evs, err := s2.ClaimEvents(ctx, "c-secret"); err != nil || len(evs) != 1 {
		t.Fatalf("a refused purge still mutated the store (evs=%d err=%v)", len(evs), err)
	}

	// With both, the events go and the report says integrity is broken.
	rep2, err := s2.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{IncludeHistory: true, Force: true})
	if err != nil {
		t.Fatalf("PurgePaths --include-history --force: %v", err)
	}
	if evs, err := s2.ClaimEvents(ctx, "c-secret"); err != nil || len(evs) != 0 {
		t.Fatalf("claim_events survived --include-history --force (evs=%d err=%v)", len(evs), err)
	}
	if n := locationCount(t, rep2.Before, string(store.LocClaimEvents)); n != 1 {
		t.Errorf("claim_events before-count = %d, want 1", n)
	}
	if !strings.Contains(strings.ToLower(rep2.HistoryNote), "integrity") {
		t.Errorf("HistoryNote = %q, want it to state that claim history integrity is now broken", rep2.HistoryNote)
	}
	if evs, err := s2.ClaimEvents(ctx, "c-kept"); err != nil || len(evs) != 1 {
		t.Fatalf("--include-history took an UNrelated claim's events (evs=%d err=%v)", len(evs), err)
	}
}

// TestPurgeResolvesRowsWithNoSourceRef is SP-11 — the review finding that
// resolution must read EACH location's own path column. The fixture is
// §3.3's own worked example: a service whose secrets/ was previously
// .fathomignore'd, so nothing anchors the path except a knowledge card
// (knowledge_cards has no source_ref column at all) and a claim citation.
// ListSourceRefs returns nothing for it; a purge resolved from that list
// would delete nothing and write "verified" over an LLM summary of the file.
func TestPurgeResolvesRowsWithNoSourceRef(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)

	if err := s.PutKnowledgeCards(ctx, []store.KnowledgeCard{
		{ServiceID: purgeSvc, Symbol: "Token", File: secretFile, Summary: "an LLM summary quoting " + secretCanary},
	}); err != nil {
		t.Fatalf("PutKnowledgeCards: %v", err)
	}
	if err := s.PutTypedEdges(ctx, purgeSvc, "secrets", []core.Edge{
		{FromService: purgeSvc, FromFile: secretFile, FromSymbol: "secrets.Token", ToSymbol: "fmt.Sprintf", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}
	if err := s.PutClaims(
		ctx,
		[]core.SemanticClaim{{ServiceID: purgeSvc, ClaimID: "c-secret", Layer: "l", Subject: "secrets", Rule: "r"}},
		[]core.ClaimLink{{ClaimID: "c-secret", Rel: core.RelCites, TargetRef: core.CiteRef(secretFile, 4, 9), AnchorText: secretCanary}},
	); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	// The premise: the single-ref-list resolution really is blind here.
	refs, err := s.ListSourceRefs(ctx, purgeSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("fixture premise broken: ListSourceRefs returned %v, want empty", refs)
	}

	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{secretCanary}})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	if len(rep.Resolved) == 0 {
		t.Fatal("resolution found NOTHING — it is reading ListSourceRefs, not each location's own path column")
	}
	if rep.Resolved[0] != secretFile {
		t.Fatalf("Resolved = %v, want [%s]", rep.Resolved, secretFile)
	}
	for _, loc := range []store.PurgeLocation{store.LocKnowledgeCards, store.LocTypedEdges, store.LocClaimLinks, store.LocSemanticClaims} {
		if n := locationCount(t, rep.Before, string(loc)); n == 0 {
			t.Errorf("%s was not resolved (before-count 0) — its own path column was never read", loc)
		}
		if n := locationCount(t, rep.After, string(loc)); n != 0 {
			t.Errorf("%s still holds %d row(s)", loc, n)
		}
	}
	cards, err := s.KnowledgeCards(ctx, purgeSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Fatalf("the knowledge card holding the summary survived: %+v", cards)
	}
	if rep.Verdict != store.PurgeVerified {
		t.Fatalf("purge reported %s: %s", rep.Verdict, rep.VerdictNote)
	}
}

// fakeCache, fakeVectors and fakeMemories stand in for the derived stores
// §3.2 names.
type fakeCache struct{ flushed int }

func (c *fakeCache) Invalidate() { c.flushed++ }

type fakeVectors struct{ deleted []string }

func (v *fakeVectors) Delete(ids ...string) error {
	v.deleted = append(v.deleted, ids...)
	return nil
}

type fakeMemories struct{ purged int }

func (f *fakeMemories) PurgeMemories(m store.PathMatcher) (int, error) {
	if m.Match(secretFile) {
		f.purged++
	}
	return f.purged, nil
}

// TestPurgeFlushesDerivedStores is SP-6 (plus AM-12's memories row): the
// result cache is flushed WHOLLY (it is query-keyed, so selective removal
// is unsound), the affected vectors are deleted, and the repo's
// memories.jsonl is purged of matching anchors. When a derived store is
// not wired, the report names its absence rather than reporting a
// comforting zero (law 3).
func TestPurgeFlushesDerivedStores(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)

	cache := &fakeCache{}
	vecs := &fakeVectors{}
	mems := &fakeMemories{}
	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Cache: cache, Vectors: vecs, Memories: mems})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	if cache.flushed != 1 {
		t.Errorf("result cache flushed %d times, want exactly 1 (wholly, once)", cache.flushed)
	}
	sort.Strings(vecs.deleted)
	if len(vecs.deleted) != 1 || vecs.deleted[0] != "secrets.Token" {
		t.Errorf("vector ids deleted = %v, want [secrets.Token]", vecs.deleted)
	}
	if mems.purged != 1 {
		t.Errorf("memories purge was not invoked with the pattern matcher (purged=%d)", mems.purged)
	}
	for _, loc := range []store.PurgeLocation{store.LocResultCache, store.LocVectorIndex, store.LocMemories} {
		var seen bool
		for _, d := range rep.Derived {
			if d.Location != loc {
				continue
			}
			seen = true
			if !d.Applied {
				t.Errorf("%s reported not applied: %s", loc, d.Reason)
			}
		}
		if !seen {
			t.Errorf("%s missing from the derived-store report %+v", loc, rep.Derived)
		}
	}

	// Not wired: absence is NAMED, never rendered as a successful zero.
	s2, _ := openFileStore(t)
	seedPurgeFixture(t, s2)
	rep2, err := s2.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	for _, loc := range []store.PurgeLocation{store.LocResultCache, store.LocVectorIndex, store.LocMemories} {
		var seen bool
		for _, d := range rep2.Derived {
			if d.Location != loc {
				continue
			}
			seen = true
			if d.Applied {
				t.Errorf("%s reported applied with nothing wired", loc)
			}
			if strings.TrimSpace(d.Reason) == "" {
				t.Errorf("%s is absent with no reason — law 3", loc)
			}
		}
		if !seen {
			t.Errorf("%s missing from %+v", loc, rep2.Derived)
		}
	}

	// A wired derived store that ERRORS fails the verdict — a copy may
	// remain, so this must never read as success.
	s3, _ := openFileStore(t)
	seedPurgeFixture(t, s3)
	rep3, err := s3.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{
		Vectors: failingVectors{}, Probes: []string{secretCanary},
	})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	if rep3.Verdict != store.PurgeFailed {
		t.Fatalf("a failed wired vector delete produced verdict %s, want failed", rep3.Verdict)
	}
}

type failingVectors struct{}

func (failingVectors) Delete(ids ...string) error { return errors.New("index unavailable") }

// TestPurgeLeavesNoBytesInStoreFile is SP-12, the review's third finding:
// a re-query can only confirm the delete it came from. The deletion runs on
// a secure_delete connection, is followed by wal_checkpoint(TRUNCATE) and
// VACUUM, and verification includes a RAW-BYTES probe of the store file
// (and its -wal) — the only check that can see residue SQL structurally
// cannot.
func TestPurgeLeavesNoBytesInStoreFile(t *testing.T) {
	ctx := context.Background()
	s, path := openFileStore(t)
	seedPurgeFixture(t, s)

	// Force the canary onto disk so the probe has something real to fail on
	// if the physical pass is skipped.
	if err := s.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if !fileContains(t, path, secretCanary) && !fileContains(t, path+"-wal", secretCanary) {
		t.Fatal("fixture premise broken: the canary is in neither the store file nor its -wal before the purge")
	}

	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{secretCanary}})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}

	if !rep.Physical.Applied {
		t.Fatalf("the physical pass did not run: %s", rep.Physical.Reason)
	}
	if !rep.Physical.SecureDelete {
		t.Error("the deleting connection did not have secure_delete on — freed pages keep the old bytes")
	}
	if !rep.Physical.Checkpointed {
		t.Error("wal_checkpoint(TRUNCATE) did not run — the -wal keeps the deleted rows verbatim")
	}
	if !rep.Physical.Vacuumed {
		t.Error("VACUUM did not run — free pages keep the deleted rows verbatim")
	}
	if !rep.Physical.Probe.Ran {
		t.Fatalf("the raw-bytes probe did not run: %s", rep.Physical.Probe.Reason)
	}
	if len(rep.Physical.Probe.Hits) != 0 {
		t.Fatalf("raw-bytes residue found: %+v", rep.Physical.Probe.Hits)
	}
	if rep.Verdict != store.PurgeVerified {
		t.Fatalf("purge reported %s: %s", rep.Verdict, rep.VerdictNote)
	}

	// The probe must actually have looked for the canary…
	var probed bool
	for _, n := range rep.Physical.Probe.Needles {
		if n == secretCanary {
			probed = true
		}
	}
	if !probed {
		t.Fatalf("the caller's canary was never probed: %v", rep.Physical.Probe.Needles)
	}

	// …and the ground truth: the bytes are gone from every on-disk file.
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		if fileContains(t, f, secretCanary) {
			t.Errorf("%s still contains the canary after the purge", filepath.Base(f))
		}
	}
}

// TestPurgeProbeCatchesInjectedResidue proves the probe is not decorative:
// a byte pattern that survives IS reported, the run reports failed, and it
// names where. Without this, a probe that always returned "clean" would
// pass every other test in this file.
func TestPurgeProbeCatchesInjectedResidue(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)

	// A canary that is still legitimately present in the store after the
	// purge (it is in the KEPT row) stands in for undeleted residue: the
	// probe's job is to report bytes it finds, not to trust the deleter.
	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{"harmless"}})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	if len(rep.Physical.Probe.Hits) == 0 {
		t.Fatal("the probe found nothing even though the needle is demonstrably in the file — it is not reading raw bytes")
	}
	if rep.Verdict == store.PurgeVerified {
		t.Fatal("the run reported VERIFIED with raw-bytes residue present")
	}
	if !strings.Contains(rep.VerdictNote, "raw") && !strings.Contains(rep.VerdictNote, "byte") {
		t.Errorf("VerdictNote %q does not name the raw-bytes probe as the failing check", rep.VerdictNote)
	}
}

// TestPurgeFailsLoudlyOnStoreSurvivor is SP-7's store half: verification is
// re-derived from each table's own path column, so a row a buggy delete
// left behind makes the run FAIL and names the table — never partial
// success. The survivor is injected through the options seam that exists
// for exactly this test (no package-global hook: V6).
func TestPurgeFailsLoudlyOnStoreSurvivor(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)

	opts := store.PurgeOptions{
		BetweenExecuteAndVerify: func() {
			if err := s.PutKnowledgeCards(ctx, []store.KnowledgeCard{
				{ServiceID: purgeSvc, Symbol: "Resurrected", File: secretFile, Summary: "survived"},
			}); err != nil {
				t.Errorf("inject survivor: %v", err)
			}
		},
	}
	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), opts)
	if err != nil {
		t.Fatalf("PurgePaths returned an error rather than an unverified report: %v", err)
	}
	if rep.Verdict != store.PurgeFailed {
		t.Fatalf("a surviving row produced verdict %s, want failed", rep.Verdict)
	}
	var named bool
	for _, sv := range rep.Survivors {
		if sv.Location == string(store.LocKnowledgeCards) && sv.Rows == 1 {
			named = true
		}
	}
	if !named {
		t.Fatalf("the survivor's table was not named: %+v", rep.Survivors)
	}
	if !strings.Contains(rep.VerdictNote, string(store.LocKnowledgeCards)) {
		t.Errorf("VerdictNote %q does not name knowledge_cards", rep.VerdictNote)
	}
}

// TestPurgeBytesUncheckedIsNotVerified is the V1 rework's core case: every
// resolved path is still live under ANOTHER service (two repos sharing the
// same relative layout — the shared-prefix fleet case), so every needle is
// skipped, the probe cannot run, and the previous build printed VERIFIED
// over bytes nobody checked. The verdict type makes that impossible: SQL
// zero + probe skipped is a DISTINCT named state, never success.
func TestPurgeBytesUncheckedIsNotVerified(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedPurgeFixture(t, s)

	// The same relative path lives under a SECOND service.
	if err := s.PutObservations(ctx, []core.Observation{{
		ServiceID: "other", SourceRef: secretFile, SourceKind: core.KindGoConst,
		EvidenceTier: core.TierVerified, Symbol: "Token", Package: "secrets",
		Claim: "const Token — the other repo's copy", IngestedAt: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatal(err)
	}

	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	if rep.Physical.Probe.Ran {
		t.Fatalf("fixture premise broken: the probe ran (needles=%v)", rep.Physical.Probe.Needles)
	}
	if rep.Verdict == store.PurgeVerified {
		t.Fatal("VERIFIED was reported while the raw-bytes probe never ran — the V1 defect is back")
	}
	if rep.Verdict != store.PurgeBytesUnchecked {
		t.Fatalf("verdict = %s, want the distinct bytes-unchecked state", rep.Verdict)
	}
	if !strings.Contains(strings.ToLower(rep.VerdictNote), "not verified") {
		t.Errorf("VerdictNote %q does not state the run is not verified", rep.VerdictNote)
	}
	if !strings.Contains(rep.VerdictNote, "probe") {
		t.Errorf("VerdictNote %q does not name the probe as what is missing", rep.VerdictNote)
	}
	// The skip itself is disclosed with its reason.
	if len(rep.Physical.Probe.Skipped) == 0 {
		t.Error("the skipped needles are not disclosed")
	}
	// And the other service's row is untouched.
	obs, err := s.QueryObservations(ctx, "other", "")
	if err != nil || len(obs) != 1 {
		t.Fatalf("the purge crossed the service boundary (obs=%d err=%v)", len(obs), err)
	}
}

// TestPurgeReportIsDeterministic: two identical purges over identical
// fixtures produce identical reports (modulo the store path), because
// every list is sorted. Determinism is a contract.
func TestPurgeReportIsDeterministic(t *testing.T) {
	ctx := context.Background()
	run := func() string {
		s, _ := openFileStore(t)
		seedPurgeFixture(t, s)
		rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{secretCanary}})
		if err != nil {
			t.Fatalf("PurgePaths: %v", err)
		}
		var b strings.Builder
		b.WriteString(string(rep.Verdict))
		b.WriteString(strings.Join(rep.Resolved, ","))
		for _, c := range rep.Before {
			b.WriteString("|" + c.Location)
		}
		for _, c := range rep.After {
			b.WriteString("/" + c.Location)
		}
		return b.String()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("purge report is not deterministic:\n a=%s\n b=%s", a, b)
	}
}

func fileContains(t *testing.T, path, needle string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.Contains(data, []byte(needle))
}
