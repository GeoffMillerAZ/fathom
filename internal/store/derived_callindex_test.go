package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type prefixMatcher string

func (p prefixMatcher) Match(path string) bool {
	return len(path) >= len(p) && path[:len(p)] == string(p)
}

// TestCallIndexSurvivesPurgeVacuumAndNewServices (review finding, RT-77,
// RT-160, I38): the call index is never left silently empty. A purge's
// VACUUM (which may renumber rowids) rebuilds every service's index in
// place rather than forgetting it until a restart, and an ingest into a
// store whose index did not exist from birth indexes the new service's
// calls in its final transaction.
func TestCallIndexSurvivesPurgeVacuumAndNewServices(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "idx.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// A store that predates the trigger: no calls_from_birth.
	if _, err := s.db.Exec(`DELETE FROM derived_meta WHERE key = ?`, callsFromBirthKey); err != nil {
		t.Fatal(err)
	}
	if err := s.PutObservations(ctx, []core.Observation{
		obsWithCalls("api", "cmd/main.go", "main", "main", 3, "billing::charge.Charge"),
		obsWithCalls("api", "secrets/x.go", "Leak", "secrets", 1, "fmt.Println"),
		obsWithCalls("web", "pay/pay.go", "Pay", "pay", 9, "charge.Charge"),
	}); err != nil {
		t.Fatal(err)
	}
	for _, svc := range []string{"api", "web"} {
		if err := s.WriteServiceSummary(ctx, ServiceMeta{ServiceID: svc, IndexedCommit: "c1", IndexedAt: 1}, ServiceSummary{}, nil); err != nil {
			t.Fatal(err)
		}
		st, err := s.DerivedState(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		if !st.CallsIndexed {
			t.Fatalf("%s: an ingest into a pre-trigger store left the service call-unindexed until a restart", svc)
		}
	}
	callers := func() int {
		t.Helper()
		got, err := s.CallersOfKeys(ctx, []string{"Charge"})
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	if n := callers(); n != 2 {
		t.Fatalf("callers of Charge = %d, want 2", n)
	}

	rep, err := s.PurgePaths(ctx, "api", prefixMatcher("secrets/"), PurgeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Physical.Vacuumed {
		t.Skip("the purge did not VACUUM; nothing to prove")
	}
	for _, svc := range []string{"api", "web"} {
		st, err := s.DerivedState(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		if !st.CallsIndexed {
			t.Fatalf("%s: the purge's VACUUM left the call index empty", svc)
		}
	}
	if n := callers(); n != 2 {
		t.Fatalf("callers of Charge after the purge = %d, want 2", n)
	}
}

// TestCallIndexReadsUnindexedAcrossTheVacuumWindow (review finding,
// RT-77, I38): between a purge's VACUUM (which may renumber rowids) and
// the call index's rebuild, the index's rows point at renumbered rowids,
// so a lookup in that window could miss a true caller while the service
// still claimed to be indexed. The services read as not indexed from
// before the VACUUM until the rebuild commits, so a reader in the window
// is told the index is absent, never handed a silent partial.
func TestCallIndexReadsUnindexedAcrossTheVacuumWindow(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "win.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutObservations(ctx, []core.Observation{
		obsWithCalls("api", "cmd/main.go", "main", "main", 3, "billing::charge.Charge"),
		obsWithCalls("api", "secrets/x.go", "Leak", "secrets", 1, "fmt.Println"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteServiceSummary(ctx, ServiceMeta{ServiceID: "api", IndexedCommit: "c1", IndexedAt: 1}, ServiceSummary{}, nil); err != nil {
		t.Fatal(err)
	}
	var inWindow *bool
	purgeAfterVacuumHook = func() {
		st, err := s.DerivedState(ctx, "api")
		if err != nil {
			t.Errorf("DerivedState in the window: %v", err)
			return
		}
		v := st.CallsIndexed
		inWindow = &v
	}
	defer func() { purgeAfterVacuumHook = nil }()
	rep, err := s.PurgePaths(ctx, "api", prefixMatcher("secrets/"), PurgeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Physical.Vacuumed {
		t.Skip("the purge did not VACUUM; nothing to prove")
	}
	if inWindow == nil {
		t.Fatal("the hook did not run")
	}
	if *inWindow {
		t.Fatal("between the VACUUM and the rebuild the service still claimed a complete call index")
	}
	st, err := s.DerivedState(ctx, "api")
	if err != nil || !st.CallsIndexed {
		t.Fatalf("after the rebuild: indexed %v, err %v", st.CallsIndexed, err)
	}
}
