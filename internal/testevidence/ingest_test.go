package testevidence

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngestTestEvidence_ManifestAndEnvelope(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	const service = "cart-svc"

	if err := st.PutObservations(ctx, []core.Observation{
		{
			ServiceID: service, SourceRef: "cart_test.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: "TestAdd", Package: "cart",
			Claim: "func TestAdd(t *testing.T)", IngestedAt: time.Now(),
		},
	}); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	manifests := []Manifest{
		{
			File: "tests/cart.cue", Component: "cart",
			Suites: []Suite{{Name: "unit", Class: "unit", Tier: "blocking"}},
		},
	}
	runs := []Run{
		{
			Suite:   SuiteRef{Component: "cart", Name: "unit", Class: "unit"},
			Key:     "k1",
			Verdict: "pass",
			Cases: []Case{
				{ID: "1", Name: "TestAdd", Status: "pass", Duration: "1ms", Attempts: 1},
				{ID: "2", Name: "TestAdd/sub", Status: "pass", Duration: "1ms", Attempts: 1},
				{ID: "3", Name: "TestUnknown", Status: "fail", Duration: "1ms", Attempts: 1},
			},
			Totals:    Totals{Run: 3, Passed: 2, Failed: 1},
			StartedAt: "2026-09-23T00:00:00Z",
			Digest:    "sha256:deadbeef",
		},
	}

	report, err := IngestTestEvidence(ctx, st, service, "wm-1", manifests, runs)
	if err != nil {
		t.Fatalf("IngestTestEvidence: %v", err)
	}
	if report.SuitesWritten != 1 {
		t.Errorf("SuitesWritten = %d, want 1", report.SuitesWritten)
	}
	if report.RunsWritten != 1 {
		t.Errorf("RunsWritten = %d, want 1", report.RunsWritten)
	}
	if report.CasesWritten != 3 {
		t.Errorf("CasesWritten = %d, want 3", report.CasesWritten)
	}
	if report.CasesJoined != 2 {
		t.Errorf("CasesJoined = %d, want 2 (TestAdd + its subtest)", report.CasesJoined)
	}

	statuses, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		t.Fatalf("TestSuiteStatuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("want 1 suite status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Component != "cart" || s.Suite != "unit" || s.Class != "unit" || s.Tier != "blocking" {
		t.Errorf("suite = %+v", s.TestSuite)
	}
	if !s.HasRun {
		t.Fatal("HasRun = false, want true")
	}
	if s.LastRun.Verdict != "pass" {
		t.Errorf("LastRun.Verdict = %q, want pass", s.LastRun.Verdict)
	}
	if s.CasesTotal != 3 || s.CasesJoined != 2 {
		t.Errorf("CasesTotal/CasesJoined = %d/%d, want 3/2", s.CasesTotal, s.CasesJoined)
	}

	// Re-ingesting the same manifest+run must not duplicate suites or
	// cases (wholesale-replace for suites, idempotent-by-digest for
	// runs/cases).
	report2, err := IngestTestEvidence(ctx, st, service, "wm-2", manifests, runs)
	if err != nil {
		t.Fatalf("second IngestTestEvidence: %v", err)
	}
	if report2.SuitesWritten != 1 {
		t.Errorf("second SuitesWritten = %d, want 1", report2.SuitesWritten)
	}
	statuses2, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		t.Fatalf("TestSuiteStatuses after re-ingest: %v", err)
	}
	if len(statuses2) != 1 || statuses2[0].CasesTotal != 3 {
		t.Errorf("re-ingest duplicated rows: %+v", statuses2)
	}
}

func TestIngestRuns_NeverTouchesSuites(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	const service = "cart-svc"

	// A prior `fathom tests ingest --manifest` declared a suite.
	manifests := []Manifest{
		{File: "tests/cart.cue", Component: "cart", Suites: []Suite{{Name: "unit", Class: "unit", Tier: "blocking"}}},
	}
	if _, err := IngestTestEvidence(ctx, st, service, "wm-1", manifests, nil); err != nil {
		t.Fatalf("seed IngestTestEvidence: %v", err)
	}

	// A plain `fathom ingest` with [tests] results_dir configured calls
	// IngestRuns directly (cmd/fathom's cmdIngest) — it must never wipe
	// the suite the manifest call declared.
	runs := []Run{{Suite: SuiteRef{Component: "cart", Name: "unit"}, Verdict: "pass", Digest: "sha256:abc"}}
	if _, err := IngestRuns(ctx, st, service, "wm-2", runs); err != nil {
		t.Fatalf("IngestRuns: %v", err)
	}

	statuses, err := st.TestSuiteStatuses(ctx, service)
	if err != nil {
		t.Fatalf("TestSuiteStatuses: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Suite != "unit" {
		t.Fatalf("IngestRuns wiped the declared suite: %+v", statuses)
	}
	if !statuses[0].HasRun || statuses[0].LastRun.Verdict != "pass" {
		t.Fatalf("IngestRuns did not write the run: %+v", statuses[0])
	}
}

func TestIngestTestEvidence_NoRuns(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	manifests := []Manifest{
		{File: "tests/x.cue", Component: "x", Suites: []Suite{{Name: "unit", Class: "unit", Tier: "blocking"}}},
	}
	report, err := IngestTestEvidence(ctx, st, "svc", "wm", manifests, nil)
	if err != nil {
		t.Fatalf("IngestTestEvidence: %v", err)
	}
	if report.SuitesWritten != 1 || report.RunsWritten != 0 {
		t.Errorf("report = %+v, want SuitesWritten=1 RunsWritten=0", report)
	}
}
