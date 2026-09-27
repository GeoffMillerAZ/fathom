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
