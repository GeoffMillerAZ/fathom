package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func obsWithCalls(svc, ref, sym, pkg string, line int, calls ...string) core.Observation {
	return core.Observation{ServiceID: svc, SourceRef: ref, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
		Symbol: sym, Package: pkg, Line: line, Calls: calls, IngestedAt: time.Unix(1_800_000_000, 0).UTC()}
}

// TestObservationCalls_IndexedReverseLookup is RT-77 (§6, §4.5): an
// ingest's inserts normalize calls into observation_calls, and
// CallersOfKeys answers "who calls X anywhere" by an indexed lookup on
// the callee key or its final name segment — never a scan of every
// service's observations — kept in step by deletes and renames.
func TestObservationCalls_IndexedReverseLookup(t *testing.T) {
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "calls.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.PutObservations(ctx, []core.Observation{
		obsWithCalls("api", "cmd/main.go", "main", "main", 3, "billing::charge.Charge", "fmt.Println"),
		obsWithCalls("web", "pay/pay.go", "Pay", "pay", 9, "charge.Charge"),
		obsWithCalls("web", "pay/none.go", "Idle", "pay", 1),
		obsWithCalls("billing", "charge/charge.go", "Charge", "charge", 5),
	}); err != nil {
		t.Fatal(err)
	}
	for _, svc := range []string{"api", "web", "billing"} {
		if err := s.WriteServiceSummary(ctx, ServiceMeta{ServiceID: svc, IndexedCommit: "c1", IndexedAt: 1}, ServiceSummary{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.CallersOfKeys(ctx, []string{"Charge"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range got {
		names = append(names, r.ServiceID+"/"+r.Symbol+"→"+r.CalleeKey)
	}
	if strings.Join(names, ",") != "api/main→billing::charge.Charge,web/Pay→charge.Charge" {
		t.Fatalf("callers = %v", names)
	}
	if got, _ := s.CallersOfKeys(ctx, []string{"charge.Charge"}); len(got) != 1 || got[0].Symbol != "Pay" {
		t.Fatalf("full-key lookup = %+v", got)
	}

	// The lookup is an index search, never a scan of observations.
	plan := explainPlan(t, s, `SELECT oc.caller_rowid FROM observation_calls oc WHERE oc.callee_key IN (?) OR oc.callee_name IN (?)`, "x", "x")
	joined := strings.Join(plan, " | ")
	if !strings.Contains(joined, "idx_observation_calls_key") || !strings.Contains(joined, "idx_observation_calls_name") || strings.Contains(joined, "SCAN oc") {
		t.Fatalf("reverse lookup plan does not use the callee indexes: %s", joined)
	}

	// Kept in step: a purged file's calls go; a renamed service follows.
	if err := s.PurgeSource(ctx, "web", "pay/pay.go"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameService(ctx, "api", "gateway"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CallersOfKeys(ctx, []string{"Charge"})
	if len(got) != 1 || got[0].ServiceID != "gateway" || got[0].Symbol != "main" {
		t.Fatalf("after purge+rename = %+v", got)
	}
	var orphans int
	if err := s.db.QueryRow(`SELECT count(*) FROM observation_calls oc WHERE NOT EXISTS (SELECT 1 FROM observations o WHERE o.rowid = oc.caller_rowid)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Fatalf("%d observation_calls rows outlived their observation", orphans)
	}

	// The memory backend answers the same question the same way.
	m := NewMemoryStore()
	_ = m.PutObservations(ctx, []core.Observation{
		obsWithCalls("api", "cmd/main.go", "main", "main", 3, "billing::charge.Charge"),
		obsWithCalls("web", "pay/pay.go", "Pay", "pay", 9, "charge.Charge"),
	})
	if got, _ := m.CallersOfKeys(ctx, []string{"Charge"}); len(got) != 2 {
		t.Fatalf("memory callers = %+v", got)
	}
}

// TestSummaryBackfilledWithoutIngest is RT-160 (§6): a store written
// before the derived tables — observations and fingerprints, no summary,
// no call index — gets both by SQL at first use, one service per
// transaction, reading no file; until then CallersOfKeys names nothing for
// the service and DerivedState says why; file_stats waits for an ingest.
func TestSummaryBackfilledWithoutIngest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.sqlite")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// Simulate a pre-RT-76 store: rows written, then the derived tables
	// and triggers dropped, as a store from before them would be.
	if err := s.PutObservations(ctx, []core.Observation{
		obsWithCalls("svc", "a.go", "A", "p", 1, "p.B"),
		obsWithCalls("svc", "b.go", "B", "p", 1),
		{ServiceID: "svc", SourceRef: "c.py", SourceKind: core.SourceKind("PyFunc"), EvidenceTier: core.TierVerified, Symbol: "C", IngestedAt: time.Unix(1, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"a.go", "b.go", "c.py", "Main.scala"} {
		_ = s.SetFingerprint(ctx, "svc", ref, "h")
	}
	_ = s.SetServiceMeta(ctx, ServiceMeta{ServiceID: "svc", Path: "/nonexistent/corpus", IndexedCommit: "abc", IndexedAt: 42})
	for _, q := range []string{`DROP TRIGGER trg_observation_calls_ai`, `DROP TRIGGER trg_observation_calls_ad`, `DROP TRIGGER trg_observation_calls_au`,
		`DROP TABLE observation_calls`, `DROP TABLE observation_calls_state`, `DROP TABLE service_summary`, `DROP TABLE file_stats`, `DROP TABLE derived_meta`} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.Close()

	// Reopen: the migration creates the tables empty (the store held rows,
	// so no calls_from_birth); nothing is derived yet.
	s, err = NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, ok, _ := s.ServiceSummary(ctx, "svc"); ok {
		t.Fatal("summary present before any backfill")
	}
	st, _ := s.DerivedState(ctx, "svc")
	if st.HasSummary || st.CallsIndexed || st.HasFileStats {
		t.Fatalf("state before backfill = %+v", st)
	}
	if got, _ := s.CallersOfKeys(ctx, []string{"B"}); len(got) != 0 {
		t.Fatalf("an unindexed service answered CallersOfKeys: %+v", got)
	}

	classified := 0
	rep, err := s.BackfillDerivedTables(ctx, "svc", BackfillOptions{Summary: true, Calls: true, Classify: func(p string) (string, bool) {
		classified++
		if strings.HasSuffix(p, ".scala") {
			return "Scala", true
		}
		return "", false
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Summary || rep.CallRows != 1 || classified != 4 {
		t.Fatalf("backfill = %+v classified=%d", rep, classified)
	}
	sum, ok, err := s.ServiceSummary(ctx, "svc")
	if err != nil || !ok {
		t.Fatalf("summary after backfill: ok=%v err=%v", ok, err)
	}
	if sum.SymbolCount != 3 || len(sum.SourceKinds) != 2 || sum.FileCount != 4 || sum.UnsupportedLangs["Scala"] != 1 ||
		sum.Source != SummaryFromBackfill || sum.Bytes != -1 || sum.Watermark != "abc@42" || sum.HasFileStats {
		t.Fatalf("summary = %+v", sum)
	}
	if got, _ := s.CallersOfKeys(ctx, []string{"B"}); len(got) != 1 || got[0].Symbol != "A" {
		t.Fatalf("callers after backfill = %+v", got)
	}
	st, _ = s.DerivedState(ctx, "svc")
	if !st.SummaryCurrent || !st.CallsIndexed || st.HasFileStats {
		t.Fatalf("state after backfill = %+v (file_stats must wait for an ingest)", st)
	}
	// A second backfill at the same watermark writes nothing.
	if rep, _ := s.BackfillDerivedTables(ctx, "svc", BackfillOptions{Summary: true, Calls: true}); rep.Summary || rep.CallRows != 0 {
		t.Fatalf("repeat backfill wrote %+v", rep)
	}
	// A new watermark makes the summary stale; the next backfill rewrites it.
	_ = s.SetServiceMeta(ctx, ServiceMeta{ServiceID: "svc", IndexedCommit: "def", IndexedAt: 43})
	if st, _ := s.DerivedState(ctx, "svc"); st.SummaryCurrent {
		t.Fatal("summary current across a watermark move")
	}
	if rep, _ := s.BackfillDerivedTables(ctx, "svc", BackfillOptions{Summary: true}); !rep.Summary {
		t.Fatal("stale summary not rewritten")
	}

	// Memory backend: same contract.
	m := NewMemoryStore()
	_ = m.PutObservations(ctx, []core.Observation{obsWithCalls("svc", "a.go", "A", "p", 1)})
	_ = m.SetFingerprint(ctx, "svc", "a.go", "h")
	_ = m.SetServiceMeta(ctx, ServiceMeta{ServiceID: "svc", IndexedCommit: "abc", IndexedAt: 42})
	if _, ok, _ := m.ServiceSummary(ctx, "svc"); ok {
		t.Fatal("memory: summary before backfill")
	}
	if rep, _ := m.BackfillDerivedTables(ctx, "svc", BackfillOptions{Summary: true}); !rep.Summary {
		t.Fatal("memory: no backfill")
	}
	if sum, ok, _ := m.ServiceSummary(ctx, "svc"); !ok || sum.SymbolCount != 1 || sum.FileCount != 1 {
		t.Fatalf("memory summary = %+v", sum)
	}
}

// TestWriteServiceSummaryOneTransaction pins RT-76's store half: meta,
// file_stats and the summary land together, the summary's watermark is
// the meta's, and a fresh store's ingest is call-indexed without a
// backfill.
func TestWriteServiceSummaryOneTransaction(t *testing.T) {
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "w.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	_ = s.PutObservations(ctx, []core.Observation{obsWithCalls("svc", "a.go", "A", "p", 1, "p.B")})
	files := []FileStat{{Path: "b.rs", Size: 10, Lang: UnsupportedLangPrefix + "Rust"}, {Path: "a.go", Size: 20, MtimeNS: 5, Ino: 7, ContentHash: "h", Lang: string(core.KindGoFunc)}}
	meta := ServiceMeta{ServiceID: "svc", Path: "/p", IndexedCommit: "c", IndexedAt: 9}
	if err := s.WriteServiceSummary(ctx, meta, ServiceSummary{FileCount: 2, Bytes: 30, UnsupportedLangs: map[string]int{"Rust": 1}, ExcludedCounts: map[string]int{"oversize": 0}}, files); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := s.GetServiceMeta(ctx, "svc")
	if !ok || got != meta {
		t.Fatalf("meta = %+v", got)
	}
	sum, ok, _ := s.ServiceSummary(ctx, "svc")
	if !ok || sum.Watermark != "c@9" || sum.SymbolCount != 1 || sum.Bytes != 30 || sum.UnsupportedLangs["Rust"] != 1 || !sum.HasFileStats || sum.Source != SummaryFromIngest {
		t.Fatalf("summary = %+v", sum)
	}
	fs, ok, _ := s.FileStats(ctx, "svc")
	if !ok || len(fs) != 2 || fs[0].Path != "a.go" || fs[0].Ino != 7 {
		t.Fatalf("file_stats = %+v", fs)
	}
	if st, _ := s.DerivedState(ctx, "svc"); !st.CallsIndexed || !st.SummaryCurrent {
		t.Fatalf("fresh store ingest not call-indexed: %+v", st)
	}
	// PurgeService clears the derived rows with the content.
	_ = s.PurgeService(ctx, "svc")
	if _, ok, _ := s.ServiceSummary(ctx, "svc"); ok {
		t.Fatal("summary survived PurgeService")
	}
	var n int
	_ = s.db.QueryRow(`SELECT count(*) FROM observation_calls`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d call rows survived PurgeService", n)
	}
}
