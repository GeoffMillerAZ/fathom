package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// explainPlan returns every EXPLAIN QUERY PLAN detail line for query.
func explainPlan(t *testing.T, s *SQLiteStore, query string, args ...any) []string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		out = append(out, detail)
	}
	return out
}

// TestStorePragmasAndNoTempSort is RT-78 (§6): every pooled connection
// carries cache_size and journal_size_limit, the process carries the
// SQLite soft and hard heap limits, observations are indexed by (service_id,
// symbol) for point lookups, and reading a service's observations sorts
// without a temp B-tree while keeping ingested_at order.
func TestStorePragmasAndNoTempSort(t *testing.T) {
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "p.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	// Two distinct pooled connections both carry the per-connection pragmas.
	c1, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Close()
	c2, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	for name, q := range map[string]int64{"cache_size": tenancyCacheSize, "journal_size_limit": tenancyJournalSizeLimit} {
		for _, conn := range []*sql.Conn{c1, c2} {
			var got int64
			if err := conn.QueryRowContext(ctx, "PRAGMA "+name).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != q {
				t.Fatalf("PRAGMA %s = %d, want %d", name, got, q)
			}
		}
	}
	for name, want := range map[string]int64{"soft_heap_limit": tenancySoftHeapLimit, "hard_heap_limit": tenancyHardHeapLimit} {
		var got int64
		if err := s.db.QueryRowContext(ctx, "PRAGMA "+name).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("PRAGMA %s = %d, want %d", name, got, want)
		}
	}

	// Point lookup uses the (service_id, symbol) index.
	if !explainUsesIndex(t, s, "idx_observations_service_symbol",
		`SELECT symbol FROM observations WHERE service_id = ? AND symbol = ?`, "svc", "Foo") {
		t.Fatal("(service_id, symbol) point lookup does not use idx_observations_service_symbol")
	}

	// The service's observations: no temp B-tree, ingested_at order kept.
	for _, q := range []string{queryObservationsSQL, queryObservationsConcernSQL} {
		for _, d := range explainPlan(t, s, q, "svc", "c") {
			if strings.Contains(d, "TEMP B-TREE") {
				t.Fatalf("%q sorts with a temp B-tree: %s", q, d)
			}
		}
	}
	base := time.Unix(1_800_000_000, 0).UTC()
	obs := []core.Observation{
		{ServiceID: "svc", SourceRef: "b.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Late", IngestedAt: base.Add(2 * time.Second)},
		{ServiceID: "svc", SourceRef: "a.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Early", IngestedAt: base},
		{ServiceID: "svc", SourceRef: "c.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Mid", IngestedAt: base.Add(time.Second)},
	}
	if err := s.PutObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}
	got, err := s.QueryObservations(ctx, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, o := range got {
		names = append(names, o.Symbol)
	}
	if strings.Join(names, ",") != "Early,Mid,Late" {
		t.Fatalf("order = %v, want ingested_at order", names)
	}
}
