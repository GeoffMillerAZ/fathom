package testevidence

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestRuntimeRunsMatchesKeysAndBareNames pins the attested-run read behind
// runtime_pass (TC-29): a canonical key matches its own row, a qualified
// name the joined tests with that bare name in its package, a bare name
// every joined test with that name (the newest answers), a run's time is
// its envelope's startedAt, and a name with no run is absent.
func TestRuntimeRunsMatchesKeysAndBareNames(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	sha := "0123456789abcdef0123456789abcdef01234567"
	if err := st.PutTestRuns(ctx, "svc", []store.TestRun{
		{RunDigest: "r1", Suite: "unit", StartedAt: "2026-10-09T00:00:00Z", Commit: sha, Cases: []store.TestCase{
			{CaseID: "1", Name: "TestX", Status: "fail", Attempts: 1, TestSymbolKey: "svc::a/a.TestX"},
		}},
		{RunDigest: "r2", Suite: "unit", StartedAt: "2026-10-10T00:00:00Z", Dirty: true, Commit: sha, Cases: []store.TestCase{
			{CaseID: "1", Name: "TestX", Status: "pass", Attempts: 1, TestSymbolKey: "svc::b/b.TestX"},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)
	rr, err := LoadRuntimeRuns(ctx, st, "svc", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if !rr.Now().Equal(now) {
		t.Errorf("Now = %v, want the injected clock", rr.Now())
	}
	got, err := rr.NewestAttestedRuns([]string{"svc::a/a.TestX", "TestX", "a.TestX", "TestNone"})
	if err != nil {
		t.Fatal(err)
	}
	if r := got["svc::a/a.TestX"]; r.Status != "fail" || r.Run != "r1" || r.Commit != sha {
		t.Errorf("by key = %+v, want r1's fail", r)
	}
	if r := got["TestX"]; r.Status != "pass" || r.Run != "r2" || !r.Dirty || !r.At.Equal(time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("by bare name = %+v, want the newest (r2, pass, dirty, startedAt)", r)
	}
	if r := got["a.TestX"]; r.Run != "r1" {
		t.Errorf("a qualified name reads its own package's run: %+v", r)
	}
	if _, ok := got["TestNone"]; ok {
		t.Error("a test with no run must be absent")
	}
}
