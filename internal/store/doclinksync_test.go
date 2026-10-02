package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

// TestDocLinkSyncIsSyncDocLinksForUnit (EW-50): a pass through DocLinkSync
// writes exactly the doc_links rows the per-unit SyncDocLinksForUnit
// writes — an open link left untouched, one closed, one opened, a removed
// unit's links closed, a unit synced twice in one pass — and reads the
// service's open links once per pass, not once per unit.
func TestDocLinkSyncIsSyncDocLinksForUnit(t *testing.T) {
	ctx := context.Background()
	open := func(name string) *SQLiteStore {
		s, err := NewSQLiteStore(filepath.Join(t.TempDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	}
	perUnit, perPass := open("unit.sqlite"), open("pass.sqlite")
	link := func(kind, target string, line int, hash string) DocLink {
		return DocLink{TargetKind: kind, Target: target, Line: line, LinkTier: "exact", TargetHashAtLink: hash}
	}
	type step struct {
		unit  string
		fresh []DocLink
	}
	passes := []struct {
		now   int64
		steps []step
	}{
		{100, []step{
			{"u1", []DocLink{link("symbol", "A", 1, "h1"), link("path", "x.go", 2, "hx")}},
			{"u2", []DocLink{link("symbol", "B", 3, "hb")}},
			{"u3", nil},
			{"other-service-shape", []DocLink{link("symbol", "A", 9, "h1")}},
		}},
		{200, []step{
			{"u1", []DocLink{link("symbol", "A", 1, "h1-moved"), link("symbol", "C", 4, "hc")}},
			{"u2", nil}, // removed
			{"u4", []DocLink{link("symbol", "B", 3, "hb")}},
			{"u1", []DocLink{link("symbol", "C", 4, "hc")}}, // synced again in the same pass
		}},
	}
	for _, p := range passes {
		sync := perPass.NewDocLinkSync("svc")
		for _, st := range p.steps {
			if err := perUnit.SyncDocLinksForUnit(ctx, "svc", st.unit, st.fresh, p.now); err != nil {
				t.Fatal(err)
			}
			if err := sync.SyncUnit(ctx, st.unit, st.fresh, p.now); err != nil {
				t.Fatal(err)
			}
		}
		if sync.OpenReads() != 1 {
			t.Fatalf("pass at %d read the open links %d times, want 1", p.now, sync.OpenReads())
		}
	}
	rows := func(s *SQLiteStore) [][]any {
		r, err := s.db.QueryContext(ctx, `SELECT service_id, unit_id, target_kind, target, line, link_tier, target_hash_at_link, valid_from, valid_to FROM doc_links ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var out [][]any
		for r.Next() {
			var a, b, c, d, f, g string
			var e, h, i int64
			if err := r.Scan(&a, &b, &c, &d, &e, &f, &g, &h, &i); err != nil {
				t.Fatal(err)
			}
			out = append(out, []any{a, b, c, d, e, f, g, h, i})
		}
		return out
	}
	want, got := rows(perUnit), rows(perPass)
	if len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("doc_links through DocLinkSync =\n%v\nthrough SyncDocLinksForUnit =\n%v", got, want)
	}
}
