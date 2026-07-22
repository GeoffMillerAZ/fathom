package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// seedMigrationSource creates a fresh DuckDB store at <dir>/source.duckdb
// populated with one row in every non-derivable table (plus one derivable
// observations row, to prove MigrateNonDerivable leaves it behind) across
// two services, and returns its path.
func seedMigrationSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.duckdb")
	s, err := OpenDuckDB(path)
	if err != nil {
		t.Fatalf("OpenDuckDB source: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// Derivable table — must NOT be ported.
	if err := s.PutObservations(ctx, []core.Observation{
		{
			ServiceID: "svc-a", SourceRef: "a.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
			Symbol: "F", Claim: "func F()", ContentHash: "h", IngestedAt: time.Now().UTC(),
		},
	}); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	// LLM-derived enrichment — non-derivable in practice (re-ingest does NOT
	// rebuild these; PurgeService deliberately retains them for the same
	// reason), so migrate-store MUST port them (F3).
	if err := s.PutKnowledgeCards(ctx, []KnowledgeCard{
		{ServiceID: "svc-a", Symbol: "F", File: "a.go", Summary: "Does F things.", Tags: []string{"core", "f"}},
	}); err != nil {
		t.Fatalf("seed PutKnowledgeCards: %v", err)
	}
	if err := s.PutDocSummaries(ctx, []DocSummary{
		{ServiceID: "svc-b", SourceRef: "docs/api.md", Title: "API", Summary: "The API doc."},
	}); err != nil {
		t.Fatalf("seed PutDocSummaries: %v", err)
	}

	if err := s.SetServiceMeta(ctx, ServiceMeta{ServiceID: "svc-a", Path: "/code/svc-a", IndexedCommit: "aaa111", IndexedAt: 100}); err != nil {
		t.Fatalf("seed SetServiceMeta svc-a: %v", err)
	}
	if err := s.SetServiceMeta(ctx, ServiceMeta{ServiceID: "svc-b", Path: "/code/svc-b", IndexedCommit: "bbb222", IndexedAt: 200}); err != nil {
		t.Fatalf("seed SetServiceMeta svc-b: %v", err)
	}

	if err := s.SetCommits(ctx, "svc-a", []Commit{
		{ServiceID: "svc-a", Hash: "aaa111", Author: "Alice", Date: 100, Message: "init", Files: []string{"a.go"}, SpecIDs: nil},
	}); err != nil {
		t.Fatalf("seed SetCommits svc-a: %v", err)
	}
	if err := s.SetCommits(ctx, "svc-b", []Commit{
		{ServiceID: "svc-b", Hash: "bbb222", Author: "Bob", Date: 200, Message: "init", Files: []string{"b.go"}, SpecIDs: []string{"M1"}},
	}); err != nil {
		t.Fatalf("seed SetCommits svc-b: %v", err)
	}

	claims := []core.SemanticClaim{
		{
			ClaimID: "claim-a", ServiceID: "svc-a", Layer: "obligations", Kind: "obligation",
			Subject: "S-a", Rule: "R-a", Status: core.ClaimFresh, Tier: core.TierDocClaimed,
			CompiledAt: time.Unix(1000, 0).UTC(),
		},
		{
			ClaimID: "claim-b", ServiceID: "svc-b", Layer: "obligations", Kind: "obligation",
			Subject: "S-b", Rule: "R-b", Status: core.ClaimRefuted, Tier: core.TierDocClaimed,
			CompiledAt: time.Unix(2000, 0).UTC(),
		},
	}
	links := []core.ClaimLink{
		{ClaimID: "claim-a", Rel: core.RelCites, TargetRef: "docs/x.md#L1-L2", TargetHash: "hx"},
		{ClaimID: "claim-b", Rel: core.RelScopes, TargetRef: "svc-b/**", TargetHash: ""},
	}
	if err := s.PutClaims(ctx, claims, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}

	if err := s.AppendClaimEvent(ctx, core.ClaimEvent{ClaimID: "claim-b", Verdict: "refuted", Author: "carol", Note: "wrong", At: time.Unix(2500, 0).UTC()}); err != nil {
		t.Fatalf("seed AppendClaimEvent: %v", err)
	}

	if err := s.RecordCompileRun(ctx, core.CompileRun{
		RunID: "run-1", ServiceID: "svc-a", Layer: "obligations", CorpusWatermark: "wm1",
		Model: "claude-sonnet-4-5", PromptVersion: "obligations/v1", TokensIn: 10, TokensOut: 5,
		CostUSD: 0.001, DurationMS: 50, SectionsTotal: 1, SectionsCompiled: 1, ClaimsEmitted: 1,
		RanAt: time.Unix(3000, 0).UTC(),
	}); err != nil {
		t.Fatalf("seed RecordCompileRun: %v", err)
	}

	return path
}

func TestMigrateNonDerivable_HappyPath(t *testing.T) {
	fromPath := seedMigrationSource(t)
	toPath := filepath.Join(filepath.Dir(fromPath), "target.sqlite")

	report, err := MigrateNonDerivable(context.Background(), fromPath, toPath)
	if err != nil {
		t.Fatalf("MigrateNonDerivable: %v", err)
	}

	wantTables := map[string]int{
		"semantic_claims": 2,
		"claim_links":     2,
		"claim_events":    1,
		"compile_runs":    1,
		"service_meta":    2,
		"commits":         2,
		"knowledge_cards": 1,
		"doc_summaries":   1,
	}
	if len(report.Tables) != len(wantTables) {
		t.Fatalf("Tables len = %d, want %d; got %+v", len(report.Tables), len(wantTables), report.Tables)
	}
	for _, tr := range report.Tables {
		want, ok := wantTables[tr.Table]
		if !ok {
			t.Errorf("unexpected table in report: %q", tr.Table)
			continue
		}
		if tr.RowCount != want {
			t.Errorf("table %q RowCount = %d, want %d", tr.Table, tr.RowCount, want)
		}
		if tr.SpotHash == "" || tr.SpotHash == "hash-error" {
			t.Errorf("table %q SpotHash = %q, want a real hash", tr.Table, tr.SpotHash)
		}
	}

	if len(report.ServiceIDs) != 2 || report.ServiceIDs[0] != "svc-a" || report.ServiceIDs[1] != "svc-b" {
		t.Errorf("ServiceIDs = %v, want [svc-a svc-b]", report.ServiceIDs)
	}
	if report.ServicePaths["svc-a"] != "/code/svc-a" || report.ServicePaths["svc-b"] != "/code/svc-b" {
		t.Errorf("ServicePaths = %v", report.ServicePaths)
	}

	// Verify the target actually holds the ported data (not just counts).
	to, err := NewSQLiteStore(toPath)
	if err != nil {
		t.Fatalf("reopen target: %v", err)
	}
	defer to.Close()
	ctx := context.Background()

	metaA, found, err := to.GetServiceMeta(ctx, "svc-a")
	if err != nil || !found {
		t.Fatalf("GetServiceMeta svc-a: found=%v err=%v", found, err)
	}
	if metaA.IndexedCommit != "aaa111" {
		t.Errorf("svc-a IndexedCommit = %q, want aaa111", metaA.IndexedCommit)
	}

	claimsB, err := to.ClaimsForService(ctx, "svc-b", "")
	if err != nil {
		t.Fatalf("ClaimsForService svc-b: %v", err)
	}
	if len(claimsB) != 1 || claimsB[0].Status != core.ClaimRefuted {
		t.Errorf("svc-b claims = %+v, want 1 refuted claim", claimsB)
	}

	linksA, err := to.ClaimLinks(ctx, "svc-a", []string{"claim-a"})
	if err != nil {
		t.Fatalf("ClaimLinks claim-a: %v", err)
	}
	if len(linksA) != 1 || linksA[0].TargetRef != "docs/x.md#L1-L2" {
		t.Errorf("claim-a links = %+v", linksA)
	}

	events, err := to.ClaimEvents(ctx, "claim-b")
	if err != nil {
		t.Fatalf("ClaimEvents claim-b: %v", err)
	}
	if len(events) != 1 || events[0].Author != "carol" {
		t.Errorf("claim-b events = %+v", events)
	}

	runs, err := to.CompileRuns(ctx, "svc-a", "")
	if err != nil {
		t.Fatalf("CompileRuns svc-a: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-1" {
		t.Errorf("svc-a compile runs = %+v", runs)
	}

	commitsB, err := to.ListCommits(ctx, "svc-b")
	if err != nil {
		t.Fatalf("ListCommits svc-b: %v", err)
	}
	if len(commitsB) != 1 || commitsB[0].Hash != "bbb222" {
		t.Errorf("svc-b commits = %+v", commitsB)
	}

	// LLM-derived enrichment must have been ported (F3: re-ingest does NOT
	// rebuild these — losing them would be silent data loss).
	cards, err := to.KnowledgeCards(ctx, "svc-a")
	if err != nil {
		t.Fatalf("KnowledgeCards svc-a: %v", err)
	}
	if len(cards) != 1 || cards[0].Symbol != "F" || cards[0].Summary != "Does F things." {
		t.Errorf("svc-a knowledge cards = %+v, want the seeded card", cards)
	}
	if len(cards) == 1 && (len(cards[0].Tags) != 2 || cards[0].Tags[0] != "core") {
		t.Errorf("svc-a card tags = %v, want [core f]", cards[0].Tags)
	}
	sums, err := to.DocSummaries(ctx, "svc-b")
	if err != nil {
		t.Fatalf("DocSummaries svc-b: %v", err)
	}
	if len(sums) != 1 || sums[0].Title != "API" {
		t.Errorf("svc-b doc summaries = %+v, want the seeded summary", sums)
	}

	// The derivable observations table must NOT have been ported.
	obs, err := to.QueryObservations(ctx, "svc-a", "")
	if err != nil {
		t.Fatalf("QueryObservations svc-a: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("target observations = %d, want 0 (derivable tables must not be migrated)", len(obs))
	}
}

// TestMigrateNonDerivable_SkipsOrphanLinks is the F4c regression: a
// claim_links row whose claim_id has no claim in semantic_claims (writable
// via PutClaims(nil, links), which cannot derive a service_id for it) must
// be SKIPPED with a count — porting it would produce a service_id=” row no
// ClaimLinks query can ever return.
func TestMigrateNonDerivable_SkipsOrphanLinks(t *testing.T) {
	fromPath := seedMigrationSource(t)
	toPath := filepath.Join(filepath.Dir(fromPath), "target.sqlite")

	// Plant an orphan link in the source: links without claims get
	// service_id '' (the svcOf lookup has nothing to derive from).
	src, err := OpenDuckDB(fromPath)
	if err != nil {
		t.Fatalf("reopen source: %v", err)
	}
	if err := src.PutClaims(context.Background(), nil, []core.ClaimLink{
		{ClaimID: "no-such-claim", Rel: core.RelCites, TargetRef: "ghost.md#L1-L2", TargetHash: "hg"},
	}); err != nil {
		t.Fatalf("seed orphan link: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	report, err := MigrateNonDerivable(context.Background(), fromPath, toPath)
	if err != nil {
		t.Fatalf("MigrateNonDerivable: %v", err)
	}

	if report.OrphanLinksSkipped != 1 {
		t.Errorf("OrphanLinksSkipped = %d, want 1", report.OrphanLinksSkipped)
	}
	for _, tr := range report.Tables {
		if tr.Table == "claim_links" && tr.RowCount != 2 {
			t.Errorf("claim_links RowCount = %d, want 2 (the orphan must not be counted as ported)", tr.RowCount)
		}
	}

	// The orphan must not exist in the target under any service.
	to, err := NewSQLiteStore(toPath)
	if err != nil {
		t.Fatalf("reopen target: %v", err)
	}
	defer to.Close()
	var n int
	if err := to.db.QueryRow(`SELECT count(*) FROM claim_links WHERE claim_id = 'no-such-claim'`).Scan(&n); err != nil {
		t.Fatalf("count orphan in target: %v", err)
	}
	if n != 0 {
		t.Errorf("orphan link rows in target = %d, want 0", n)
	}
}

func TestMigrateNonDerivable_RefusesNonEmptyTarget(t *testing.T) {
	fromPath := seedMigrationSource(t)
	toPath := filepath.Join(filepath.Dir(fromPath), "target.sqlite")

	// Pre-populate the target with one observation via the regular Store
	// path (not through migrate-store) — this should be enough to trip the
	// non-empty-target refusal even though observations isn't one of the
	// eight migrated tables.
	pre, err := NewSQLiteStore(toPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore pre-seed target: %v", err)
	}
	if err := pre.PutObservations(context.Background(), []core.Observation{
		{
			ServiceID: "existing", SourceRef: "x.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
			Symbol: "X", Claim: "func X()", ContentHash: "h", IngestedAt: time.Now().UTC(),
		},
	}); err != nil {
		t.Fatalf("pre-seed PutObservations: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close pre-seeded target: %v", err)
	}

	if _, err := MigrateNonDerivable(context.Background(), fromPath, toPath); err == nil {
		t.Fatal("MigrateNonDerivable into a non-empty target: expected error, got nil")
	}
}

func TestMigrateNonDerivable_AllowsExistingButEmptyTarget(t *testing.T) {
	fromPath := seedMigrationSource(t)
	toPath := filepath.Join(filepath.Dir(fromPath), "target.sqlite")

	// Create the target file (schema only, no rows) before migrating —
	// this must be treated the same as an absent target.
	pre, err := NewSQLiteStore(toPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore pre-create empty target: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close pre-created target: %v", err)
	}

	if _, err := MigrateNonDerivable(context.Background(), fromPath, toPath); err != nil {
		t.Fatalf("MigrateNonDerivable into an existing-but-empty target: %v", err)
	}
}

func TestMigrateNonDerivable_RejectsWrongExtensions(t *testing.T) {
	dir := t.TempDir()
	fromPath := seedMigrationSource(t)

	cases := []struct {
		name string
		from string
		to   string
	}{
		{"from not .duckdb", filepath.Join(dir, "source.sqlite"), filepath.Join(dir, "target.sqlite")},
		{"to not .sqlite", fromPath, filepath.Join(dir, "target.duckdb")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MigrateNonDerivable(context.Background(), tc.from, tc.to); err == nil {
				t.Fatalf("expected an extension-validation error, got nil")
			}
		})
	}
}
