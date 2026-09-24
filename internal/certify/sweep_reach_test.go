package certify

// sweep_reach_test.go — CV-5's own test: TestSweepCertifications_ReachChangedSinceRunGoesStale
// (design doc §1.1, O114): the half of TC-8 O114 found missing — a
// certificate whose test still reaches its target, but whose reach
// (here, the certifying test's own file — always part of Scope) changed
// in a commit AFTER the attested run's own commit, must go stale on the
// next sweep. Uses a REAL temporary git repository (same reasoning as
// git_test.go): reachUnchangedSince's whole job is reading real git
// state, and a canned diff fake would just be re-asserting the fake's own
// behaviour.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestSweepCertifications_ReachChangedSinceRunGoesStale(t *testing.T) {
	const svc = "svc-reach-changed"
	dir := t.TempDir()
	run := ExecGit
	git := func(args ...string) { mustGit(t, run, dir, args...) }
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "test")

	write("docs/intent/README.md", "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	write("docs/intent/invariants.md", "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | Widgets never negative | test (`TestLive`) |\n")
	write("pkg/spec.go", "package pkg\n\nfunc Target() {}\n")
	write("pkg/x_test.go", "package pkg\n\nfunc TestLive() {}\n")
	git("add", "-A")
	git("commit", "-q", "-m", "c0: initial")
	c0 := strings.TrimSpace(string(mustGit(t, run, dir, "rev-parse", "HEAD")))

	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: svc, Path: dir}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	pkgKey := func(file, name string) string {
		return core.SymbolKey(svc, core.QualifiedPackage(file, "pkg"), name)
	}
	testKey := pkgKey("pkg/x_test.go", "TestLive")

	obs := []core.Observation{
		{ServiceID: svc, SourceRef: "pkg/spec.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Target", Package: "pkg", Claim: "func Target() // [US-1]", IngestedAt: time.Now()},
		{ServiceID: svc, SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "TestLive", Package: "pkg", Claim: "func TestLive()", IngestedAt: time.Now()},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}
	if err := st.PutCertifyDeclarations(ctx, svc, []store.CertifyDeclaration{
		{TestSymbolKey: testKey, Kind: "invariant", TargetID: "I1", Strength: "asserts", File: "pkg/x_test.go", Line: 1},
	}); err != nil {
		t.Fatalf("PutCertifyDeclarations: %v", err)
	}
	if err := st.PutTestRuns(ctx, svc, []store.TestRun{
		{
			ServiceID: svc, RunDigest: "d1", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-24T00:00:00Z", Verdict: "pass", Commit: c0, Dirty: false,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestLive", Status: "pass", Attempts: 1, TestSymbolKey: testKey}},
		},
	}); err != nil {
		t.Fatalf("PutTestRuns: %v", err)
	}

	stateOf := func() (State, core.ClaimStatus) {
		t.Helper()
		claims, err := st.ClaimsForService(ctx, svc, Layer)
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		if len(claims) != 1 {
			t.Fatalf("claims = %+v, want exactly 1", claims)
		}
		var m Meta
		if err := json.Unmarshal(claims[0].Meta, &m); err != nil {
			t.Fatalf("unmarshal meta: %v", err)
		}
		return State(m.State), claims[0].Status
	}

	// Compile at c0: the run is bound to c0, and c0 IS HEAD — reach is
	// unchanged (nothing has happened since), so the certificate is live.
	if _, err := CompileCertifications(ctx, st, svc, "wm-1", time.Now(), dir, run); err != nil {
		t.Fatalf("CompileCertifications: %v", err)
	}
	if state, status := stateOf(); state != StateLive || status != core.ClaimFresh {
		t.Fatalf("after compile at c0: state=%s status=%s, want live/fresh", state, status)
	}

	// A LATER commit edits the certifying test's own file — always part
	// of Scope for any certificate kind, invariant included. The attested
	// run is still bound to c0; c0 is no longer HEAD.
	write("pkg/x_test.go", "package pkg\n\nfunc TestLive() { /* edited after the attested run */ }\n")
	git("add", "-A")
	git("commit", "-q", "-m", "c1: edit the certifying test after it was attested")

	report, err := SweepCertifications(ctx, st, svc, dir, run)
	if err != nil {
		t.Fatalf("SweepCertifications: %v", err)
	}
	if report.ClaimsWritten != 1 {
		t.Errorf("ClaimsWritten = %d, want 1", report.ClaimsWritten)
	}

	state, status := stateOf()
	if state != StateUnproven {
		t.Errorf("state = %s, want %s — a changed reach since the attested run must drop state to unproven (O114)", state, StateUnproven)
	}
	if status != core.ClaimStale {
		t.Errorf("status = %s, want stale — I43: a changed reach stales the claim's own status", status)
	}

	claims, err := st.ClaimsForService(ctx, svc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if !strings.Contains(claims[0].Rule, "reach changed since the attested run at "+c0) {
		t.Errorf("Rule = %q, want it to name the reason and the attested commit", claims[0].Rule)
	}
}
