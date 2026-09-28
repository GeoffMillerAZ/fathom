package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestArchViolationTrendStores (CX-20): both stores append readings with
// the previous reading's commit and the per-rule change, read them back in
// (seq, rule) order, bound the history, and drop them with the service.
func TestArchViolationTrendStores(t *testing.T) {
	sq, err := Open(filepath.Join(t.TempDir(), "trend.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	for name, st := range map[string]Store{"sqlite": sq, "memory": NewMemoryStore()} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if rs, err := st.ArchViolationTrend(ctx, "svc"); err != nil || rs != nil {
				t.Fatalf("empty trend = %+v, %v", rs, err)
			}
			if _, err := st.AppendArchViolations(ctx, "svc", "c1", 10, []ArchRuleCount{{Rule: "b -> c", Count: 2}, {Rule: "a -> b", Count: 1}}); err != nil {
				t.Fatal(err)
			}
			got, err := st.AppendArchViolations(ctx, "svc", "c2", 20, []ArchRuleCount{{Rule: "a -> b", Count: 3}, {Rule: "b -> c", Count: 1}, {Rule: "x -> y", Count: 4}})
			if err != nil {
				t.Fatal(err)
			}
			want := []ArchViolationReading{
				{Seq: 2, Commit: "c2", PrevCommit: "c1", RecordedAt: 20, Rule: "a -> b", Count: 3, Delta: 2},
				{Seq: 2, Commit: "c2", PrevCommit: "c1", RecordedAt: 20, Rule: "b -> c", Count: 1, Delta: -1},
				{Seq: 2, Commit: "c2", PrevCommit: "c1", RecordedAt: 20, Rule: "x -> y", Count: 4, Delta: 0},
			}
			if len(got) != len(want) {
				t.Fatalf("appended %+v, want %+v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
				}
			}
			all, err := st.ArchViolationTrend(ctx, "svc")
			if err != nil || len(all) != 5 || all[0].Seq != 1 || all[0].Rule != "a -> b" || all[0].PrevCommit != "" || all[4].Rule != "x -> y" {
				t.Fatalf("trend = %+v, %v", all, err)
			}
			for i := 0; i < ArchViolationKeep; i++ {
				if _, err := st.AppendArchViolations(ctx, "svc", "cN", 30, []ArchRuleCount{{Rule: "a -> b", Count: 1}}); err != nil {
					t.Fatal(err)
				}
			}
			all, _ = st.ArchViolationTrend(ctx, "svc")
			if len(all) != ArchViolationKeep || all[0].Seq != 3 {
				t.Fatalf("bounded trend: %d rows from seq %d, want %d from seq 3", len(all), all[0].Seq, ArchViolationKeep)
			}
			if _, err := st.RemoveService(ctx, "svc"); err != nil {
				t.Fatal(err)
			}
			if rs, _ := st.ArchViolationTrend(ctx, "svc"); len(rs) != 0 {
				t.Fatalf("RemoveService left %d readings", len(rs))
			}
		})
	}
}
