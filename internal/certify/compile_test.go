package certify

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

const compileSvc = "svc"

// fixtureCommit is the sha every bound run in this file's fixtures shares
// — these tests are not exercising CV-5's reach-changed detection (that
// is git_test.go's and state_at_test.go's own job, plus
// TestSweepCertifications_ReachChangedSinceRunGoesStale below), so every
// bound run here is attested against one consistent, made-up commit.
const fixtureCommit = "dddddddddddddddddddddddddddddddddddddddd"

// noopGitRunner is a GitRunner that never actually runs git: every call
// succeeds with an empty diff, i.e. "nothing changed" — the answer every
// test in this file EXCEPT the CV-5 reach-changed test wants, since none
// of their fixtures point at a real git repository. Passed as
// CompileCertifications'/SweepCertifications' `git` argument wherever a
// test is not itself exercising the reach check.
func noopGitRunner(string, ...string) ([]byte, error) { return nil, nil }

// symKey mirrors core.SymbolNode.Key() for a Test* func defined in
// "pkg/x_test.go" under `package pkg` — QualifiedPackage joins the file's
// directory ("pkg") with the declared package name ("pkg"), so the real
// key qualifier is "pkg/pkg", not the bare package name.
func symKey(name string) string {
	return core.SymbolKey(compileSvc, core.QualifiedPackage("pkg/x_test.go", "pkg"), name)
}

// seedCompileFixture seeds a service whose corpus carries: a resolved
// invariant target (I1, docs/intent/invariants.md — reach is vacuous once
// resolved, types.go's own doc comment) and five Test* symbols exercising
// every State the design's ladder names — TestLive/TestBroken/
// TestUnproven/TestFlaky all certify invariant I1 with varying attested
// run histories; TestUnreached certifies a "spec" target it never calls,
// for StateUnreachable; TestBehaviour certifies a "behaviour" target
// (declared only, tier never exceeds doc-claimed regardless of its run).
func seedCompileFixture(t *testing.T) (context.Context, store.Store) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	root := t.TempDir()
	mustWrite(t, root+"/docs/intent/README.md", "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	mustWrite(t, root+"/docs/intent/invariants.md", "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | Widgets never negative | test (`TestWidgetCount`) |\n")
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: compileSvc, Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	testNames := []string{"TestLive", "TestBroken", "TestUnproven", "TestFlaky", "TestUnreached", "TestBehaviour"}
	obs := []core.Observation{
		{
			ServiceID: compileSvc, SourceRef: "pkg/spec.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: "Target", Package: "pkg",
			Claim: "func Target() // [US-1]", IngestedAt: time.Now(),
		},
	}
	for _, n := range testNames {
		obs = append(obs, core.Observation{
			ServiceID: compileSvc, SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: n, Package: "pkg",
			Claim: "func " + n + "(t *testing.T)", IngestedAt: time.Now(),
		})
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	decls := []store.CertifyDeclaration{
		{TestSymbolKey: symKey("TestLive"), Kind: "invariant", TargetID: "I1", Strength: "asserts", File: "pkg/x_test.go", Line: 1},
		{TestSymbolKey: symKey("TestBroken"), Kind: "invariant", TargetID: "I1", Strength: "asserts", File: "pkg/x_test.go", Line: 2},
		{TestSymbolKey: symKey("TestUnproven"), Kind: "invariant", TargetID: "I1", Strength: "asserts", File: "pkg/x_test.go", Line: 3},
		{TestSymbolKey: symKey("TestFlaky"), Kind: "invariant", TargetID: "I1", Strength: "asserts", File: "pkg/x_test.go", Line: 4},
		{TestSymbolKey: symKey("TestUnreached"), Kind: "spec", TargetID: "US-1", Strength: "asserts", File: "pkg/x_test.go", Line: 5},
		{TestSymbolKey: symKey("TestBehaviour"), Kind: "behaviour", TargetID: "checkout-flow", Strength: "smoke", File: "pkg/x_test.go", Line: 6},
	}
	if err := st.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
		t.Fatalf("PutCertifyDeclarations: %v", err)
	}

	runs := []store.TestRun{
		{
			ServiceID: compileSvc, RunDigest: "d-live", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-23T00:00:00Z", Verdict: "pass", Commit: fixtureCommit,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestLive", Status: "pass", Attempts: 1, TestSymbolKey: symKey("TestLive")}},
		},
		{
			ServiceID: compileSvc, RunDigest: "d-broken", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-23T00:00:00Z", Verdict: "fail",
			Cases: []store.TestCase{{CaseID: "1", Name: "TestBroken", Status: "fail", Attempts: 1, TestSymbolKey: symKey("TestBroken")}},
		},
		// TestFlaky: oldest->newest is d-flaky-1 (flake), d-flaky-2 (pass),
		// d-flaky-3 (pass) — the last run passed, but a flake sits within
		// the last three attested runs (design doc §3).
		{
			ServiceID: compileSvc, RunDigest: "d-flaky-1", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-21T00:00:00Z", Verdict: "flake", Commit: fixtureCommit,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestFlaky", Status: "flake", Attempts: 2, TestSymbolKey: symKey("TestFlaky")}},
		},
		{
			ServiceID: compileSvc, RunDigest: "d-flaky-2", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-22T00:00:00Z", Verdict: "pass", Commit: fixtureCommit,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestFlaky", Status: "pass", Attempts: 1, TestSymbolKey: symKey("TestFlaky")}},
		},
		{
			ServiceID: compileSvc, RunDigest: "d-flaky-3", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-23T00:00:00Z", Verdict: "pass", Commit: fixtureCommit,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestFlaky", Status: "pass", Attempts: 1, TestSymbolKey: symKey("TestFlaky")}},
		},
		{
			ServiceID: compileSvc, RunDigest: "d-unreached", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-23T00:00:00Z", Verdict: "pass",
			Cases: []store.TestCase{{CaseID: "1", Name: "TestUnreached", Status: "pass", Attempts: 1, TestSymbolKey: symKey("TestUnreached")}},
		},
	}
	if err := st.PutTestRuns(ctx, compileSvc, runs); err != nil {
		t.Fatalf("PutTestRuns: %v", err)
	}

	return ctx, st
}

// certifies: invariant I43 (asserts)
func TestCompileCertifications_TierAndStateLadders(t *testing.T) {
	ctx, st := seedCompileFixture(t)

	report, err := CompileCertifications(ctx, st, compileSvc, "wm-1", time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), "", noopGitRunner)
	if err != nil {
		t.Fatalf("CompileCertifications: %v", err)
	}
	if report.Declared != 6 {
		t.Errorf("Declared = %d, want 6", report.Declared)
	}
	if report.ClaimsWritten != 6 {
		t.Errorf("ClaimsWritten = %d, want 6", report.ClaimsWritten)
	}

	claims, err := st.ClaimsForService(ctx, compileSvc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	byTest := make(map[string]core.SemanticClaim, len(claims))
	for _, c := range claims {
		var m Meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			t.Fatalf("unmarshal meta for %s: %v", c.ClaimID, err)
		}
		byTest[m.Test] = c
	}

	cases := []struct {
		test      string
		wantState State
		wantTier  core.EvidenceTier
	}{
		{"TestLive", StateLive, core.TierVerified},
		{"TestBroken", StateBroken, core.TierPreScan},
		{"TestUnproven", StateUnproven, core.TierPreScan},
		// TC-21: a flake in the last three attested runs drops the
		// tier one step (never silently Verified) — design doc §5.
		{"TestFlaky", StateFlaky, core.TierPreScan},
		{"TestUnreached", StateUnreachable, core.TierDocClaimed},
		{"TestBehaviour", StateUnproven, core.TierDocClaimed},
	}
	for _, tc := range cases {
		c, ok := byTest[tc.test]
		if !ok {
			t.Errorf("%s: no compiled claim found", tc.test)
			continue
		}
		var m Meta
		_ = json.Unmarshal(c.Meta, &m)
		if State(m.State) != tc.wantState {
			t.Errorf("%s: state = %s, want %s", tc.test, m.State, tc.wantState)
		}
		if c.Tier != tc.wantTier {
			t.Errorf("%s: tier = %s, want %s", tc.test, c.Tier, tc.wantTier)
		}
		if c.Status != core.ClaimFresh {
			t.Errorf("%s: status = %s, want fresh", tc.test, c.Status)
		}
	}

	// Idempotence: recompiling unchanged declarations/runs reproduces the
	// identical claim_ids (design doc §3 / ADR-043 decision 1).
	report2, err := CompileCertifications(ctx, st, compileSvc, "wm-1", time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), "", noopGitRunner)
	if err != nil {
		t.Fatalf("CompileCertifications (2nd): %v", err)
	}
	claims2, err := st.ClaimsForService(ctx, compileSvc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService (2nd): %v", err)
	}
	if len(claims2) != len(claims) {
		t.Errorf("recompile produced %d claims, want %d (idempotent)", len(claims2), len(claims))
	}
	if report2.ClaimsWritten != report.ClaimsWritten {
		t.Errorf("recompile ClaimsWritten = %d, want %d", report2.ClaimsWritten, report.ClaimsWritten)
	}
}

func TestCompileCertifications_ContradictedTargetNeverResolves(t *testing.T) {
	ctx, st := seedCompileFixture(t)
	if err := st.PutCertifyDeclarations(ctx, compileSvc, []store.CertifyDeclaration{
		{TestSymbolKey: symKey("TestLive"), Kind: "invariant", TargetID: "I-GHOST", Strength: "asserts", File: "pkg/x_test.go", Line: 1},
	}); err != nil {
		t.Fatalf("PutCertifyDeclarations: %v", err)
	}
	if _, err := CompileCertifications(ctx, st, compileSvc, "wm-1", time.Now(), "", noopGitRunner); err != nil {
		t.Fatalf("CompileCertifications: %v", err)
	}
	claims, err := st.ClaimsForService(ctx, compileSvc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 1 || claims[0].Status != core.ClaimContradicted {
		t.Fatalf("claims = %+v, want exactly one contradicted claim", claims)
	}
}

// TestCompileCertifications_ScopeNarrowsToReachedSymbols is I43's own
// regression test: a "rule" target whose obligations claim spans TWO
// files (pkg/a.go, pkg/b.go — resolveRule sets target.Symbols/Files from
// the whole governing claim's Scope, never one file at a time) is
// certified by a test that reaches only ONE of them (SymA, via a direct
// Calls edge). Before the fix, evaluateCertificate's Scope was built from
// target.Files WHOLESALE (compile.go, "scope := ... target.Files..."),
// so the compiled certificate covered pkg/b.go too even though the test
// never touches it — letting a certified_by row for pkg/b.go's symbol
// pass on a test that demonstrably never reaches it. Scope must include
// only the test file plus the files of the symbols ACTUALLY reached.
func TestCompileCertifications_ScopeNarrowsToReachedSymbols(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	pkgKey := func(file, name string) string {
		return core.SymbolKey(compileSvc, core.QualifiedPackage(file, "pkg"), name)
	}
	testKey := pkgKey("pkg/x_test.go", "TestRuleReachesA")
	symAKey := pkgKey("pkg/a.go", "SymA")

	obs := []core.Observation{
		{ServiceID: compileSvc, SourceRef: "pkg/a.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "SymA", Package: "pkg", Claim: "func SymA()", IngestedAt: time.Now()},
		{ServiceID: compileSvc, SourceRef: "pkg/b.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "SymB", Package: "pkg", Claim: "func SymB()", IngestedAt: time.Now()},
		{
			ServiceID: compileSvc, SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified,
			Symbol: "TestRuleReachesA", Package: "pkg", Claim: "func TestRuleReachesA(t *testing.T)",
			Calls: []string{symAKey}, IngestedAt: time.Now(),
		},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	claim := core.SemanticClaim{
		ClaimID: "rule-multi-1", ServiceID: compileSvc, Layer: "obligations", Kind: "obligation",
		Subject: "RULE-MULTI", Rule: "a rule spanning two files", Scope: []string{"pkg/a.go", "pkg/b.go"},
		Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("seed PutClaims (obligations): %v", err)
	}

	if err := st.PutCertifyDeclarations(ctx, compileSvc, []store.CertifyDeclaration{
		{TestSymbolKey: testKey, Kind: "rule", TargetID: "RULE-MULTI", Strength: "asserts", File: "pkg/x_test.go", Line: 1},
	}); err != nil {
		t.Fatalf("PutCertifyDeclarations: %v", err)
	}

	if err := st.PutTestRuns(ctx, compileSvc, []store.TestRun{
		{
			ServiceID: compileSvc, RunDigest: "d-rule", Component: "pkg", Suite: "unit",
			StartedAt: "2026-09-23T00:00:00Z", Verdict: "pass", Commit: fixtureCommit,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestRuleReachesA", Status: "pass", Attempts: 1, TestSymbolKey: testKey}},
		},
	}); err != nil {
		t.Fatalf("PutTestRuns: %v", err)
	}

	report, err := CompileCertifications(ctx, st, compileSvc, "wm-1", time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), "", noopGitRunner)
	if err != nil {
		t.Fatalf("CompileCertifications: %v", err)
	}
	if report.ClaimsWritten != 1 {
		t.Fatalf("ClaimsWritten = %d, want 1", report.ClaimsWritten)
	}

	claims, err := st.ClaimsForService(ctx, compileSvc, Layer)
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("claims = %+v, want exactly 1", claims)
	}
	c := claims[0]
	var m Meta
	if err := json.Unmarshal(c.Meta, &m); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if State(m.State) != StateLive || c.Tier != core.TierVerified {
		t.Fatalf("state/tier = %s/%s, want live/verified (the test DOES reach SymA, and its last run passed)", m.State, c.Tier)
	}

	wantScope := []string{"pkg/a.go", "pkg/x_test.go"}
	if !reflect.DeepEqual(c.Scope, wantScope) {
		t.Fatalf("Scope = %v, want %v — I43: a rule target's Scope must never include a file (pkg/b.go) the certifying test does not reach", c.Scope, wantScope)
	}

	// verify-level: a plan row naming a symbol defined in the UNREACHED
	// pkg/b.go, certified_by TestRuleReachesA, must NOT be certified —
	// the exact contract I43 protects (EvaluateCertifiedBy, verify.go).
	byTest := ByTest(claims, []string{"TestRuleReachesA"})
	crit, label := verify.EvaluateCertifiedBy("pkg/b.go", []string{"TestRuleReachesA"}, "", byTest)
	if crit.Status == verify.Pass {
		t.Fatalf("EvaluateCertifiedBy(pkg/b.go) = Pass, want Fail — TestRuleReachesA never reaches anything in pkg/b.go")
	}
	if label != "certificate-absent" {
		t.Errorf("label = %q, want certificate-absent (no certificate's Scope covers pkg/b.go)", label)
	}

	// The reached file itself DOES certify.
	critA, _ := verify.EvaluateCertifiedBy("pkg/a.go", []string{"TestRuleReachesA"}, "", byTest)
	if critA.Status != verify.Pass {
		t.Errorf("EvaluateCertifiedBy(pkg/a.go) = %+v, want Pass", critA)
	}
}
