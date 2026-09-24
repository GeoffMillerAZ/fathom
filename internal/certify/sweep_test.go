package certify

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestSweepCertifications_MovedGoneAndNewRun exercises TC-8's three named
// cases in one pass: a test symbol whose FILE moved (same key) -> stale;
// a test symbol removed from the index entirely -> contradicted; and a
// live certificate whose test gains a NEW attested run (a fresh fail
// after the compile-time pass) -> the sweep's re-derived state reflects
// it (still fresh, but no longer live).
func TestSweepCertifications_MovedGoneAndNewRun(t *testing.T) {
	ctx, st := seedCompileFixture(t)
	if _, err := CompileCertifications(ctx, st, compileSvc, "wm-1", time.Now(), "", noopGitRunner); err != nil {
		t.Fatalf("CompileCertifications: %v", err)
	}

	// 1. TestBroken is removed from the index entirely (deleted test).
	// 2. TestLive's file moves from pkg/x_test.go to pkg/moved_test.go
	//    (same symbol key — QualifiedPackage depends on directory, and
	//    both live under "pkg/", so the key is unchanged; only FilePath
	//    differs).
	// 3. TestUnproven gains a brand-new FAILING attested run.
	// PutObservations (MemoryStore) is an append-only primitive, not a
	// wholesale replace — purge the two source files first so the
	// re-seed below actually reflects "TestBroken deleted" / "TestLive
	// moved" rather than appending duplicates alongside the originals.
	if err := st.PurgeSource(ctx, compileSvc, "pkg/x_test.go"); err != nil {
		t.Fatalf("PurgeSource pkg/x_test.go: %v", err)
	}
	if err := st.PurgeSource(ctx, compileSvc, "pkg/spec.go"); err != nil {
		t.Fatalf("PurgeSource pkg/spec.go: %v", err)
	}

	testNames := []string{"TestLive", "TestUnproven", "TestFlaky", "TestUnreached", "TestBehaviour"}
	var obs []core.Observation
	obs = append(obs, core.Observation{
		ServiceID: compileSvc, SourceRef: "pkg/spec.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Target", Package: "pkg",
		Claim: "func Target() // [US-1]", IngestedAt: time.Now(),
	})
	for _, n := range testNames {
		file := "pkg/x_test.go"
		if n == "TestLive" {
			file = "pkg/moved_test.go"
		}
		obs = append(obs, core.Observation{
			ServiceID: compileSvc, SourceRef: file, SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: n, Package: "pkg",
			Claim: "func " + n + "(t *testing.T)", IngestedAt: time.Now(),
		})
	}
	// PutObservations (MemoryStore) replaces the service's whole
	// observation set (an ingest IS the current truth) — TestBroken's
	// absence here is what makes it "gone".
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("re-seed PutObservations: %v", err)
	}

	if err := st.PutTestRuns(ctx, compileSvc, []store.TestRun{
		{
			ServiceID: compileSvc, RunDigest: "d-unproven-new", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-24T00:00:00Z", Verdict: "fail",
			Cases: []store.TestCase{{CaseID: "1", Name: "TestUnproven", Status: "fail", Attempts: 1, TestSymbolKey: symKey("TestUnproven")}},
		},
	}); err != nil {
		t.Fatalf("PutTestRuns (new run): %v", err)
	}

	report, err := SweepCertifications(ctx, st, compileSvc, "", noopGitRunner)
	if err != nil {
		t.Fatalf("SweepCertifications: %v", err)
	}
	if report.Declared != 6 {
		t.Errorf("Declared = %d, want 6 (every previously-compiled claim reconsidered)", report.Declared)
	}

	claims, err := st.ClaimsForService(ctx, compileSvc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	byTest := make(map[string]core.SemanticClaim, len(claims))
	for _, c := range claims {
		var m Meta
		if len(c.Meta) > 0 {
			_ = json.Unmarshal(c.Meta, &m)
		}
		key := m.Test
		if key == "" {
			key = c.ReproKey // fallback, should not be needed
		}
		byTest[key] = c
	}

	if c := byTest["TestBroken"]; c.Status != core.ClaimContradicted {
		t.Errorf("TestBroken (deleted) status = %s, want contradicted", c.Status)
	}
	if c := byTest["TestLive"]; c.Status != core.ClaimStale {
		t.Errorf("TestLive (moved file) status = %s, want stale", c.Status)
	}
	if c, ok := byTest["TestUnproven"]; !ok || c.Status != core.ClaimFresh {
		t.Errorf("TestUnproven status = %v (ok=%v), want fresh", c.Status, ok)
	} else {
		var m Meta
		_ = json.Unmarshal(c.Meta, &m)
		if State(m.State) != StateBroken {
			t.Errorf("TestUnproven state after new failing run = %s, want broken", m.State)
		}
	}
}
