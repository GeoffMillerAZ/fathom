package console

// regressions_test.go — WI-20/WI-21/WI-22 (docs/plans/work-intelligence.md
// Wave 2; docs/design/work-intelligence.md §3 "U12"): TDD for
// certificateRegressions, conservationRegressions and
// AllProofRegressions.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCertificateRegressionsWithCommit(t *testing.T) {
	t.Run("live to broken, with the reading and commit where it changed", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "spec", TargetID: "spec:foo", Test: "TestFoo", Reached: true,
			// newest first, mirroring store.TestCaseRunsForSymbol's own order.
			Runs: []store.TestCaseRun{
				{RunDigest: "d4", StartedAt: "2026-09-08T00:00:00Z", Commit: "c4", Status: "fail"},
				{RunDigest: "d3", StartedAt: "2026-09-06T00:00:00Z", Commit: "c3", Status: "pass"},
				{RunDigest: "d2", StartedAt: "2026-09-04T00:00:00Z", Commit: "c2", Status: "pass"},
				{RunDigest: "d1", StartedAt: "2026-09-02T00:00:00Z", Commit: "c1", Status: "pass"},
			},
		}}
		regs := certificateRegressions(certs)
		if len(regs) != 1 {
			t.Fatalf("want one regression, got %+v", regs)
		}
		r := regs[0]
		if r.Kind != "certificate" || r.ID != "spec:foo" || r.Test != "TestFoo" {
			t.Fatalf("identity wrong: %+v", r)
		}
		if r.ProvenCommit != "c3" || r.ProvenAt != "2026-09-06T00:00:00Z" {
			t.Fatalf("last live reading wrong: %+v", r)
		}
		if r.ChangedCommit != "c4" || r.ChangedAt != "2026-09-08T00:00:00Z" || r.State != "broken" {
			t.Fatalf("first changed reading wrong: %+v", r)
		}
	})

	t.Run("still live reports no regression", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "spec", TargetID: "spec:foo", Test: "TestFoo", Reached: true,
			Runs: []store.TestCaseRun{
				{RunDigest: "d2", StartedAt: "2026-09-06T00:00:00Z", Commit: "c2", Status: "pass"},
				{RunDigest: "d1", StartedAt: "2026-09-02T00:00:00Z", Commit: "c1", Status: "pass"},
			},
		}}
		if regs := certificateRegressions(certs); len(regs) != 0 {
			t.Fatalf("want no regressions, got %+v", regs)
		}
	})

	t.Run("never live reports no regression (unproven from the start is not a regression)", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "rule", TargetID: "rule:bar", Test: "TestBar", Reached: true,
			Runs: []store.TestCaseRun{
				{RunDigest: "d1", StartedAt: "2026-09-02T00:00:00Z", Commit: "c1", Status: "fail"},
			},
		}}
		if regs := certificateRegressions(certs); len(regs) != 0 {
			t.Fatalf("want no regressions, got %+v", regs)
		}
	})

	t.Run("invariant-kind certificates are conservationRegressions' job, not this one's", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "invariant", TargetID: "I1", Test: "TestI1", Reached: true,
			Runs: []store.TestCaseRun{
				{RunDigest: "d2", StartedAt: "2026-09-06T00:00:00Z", Commit: "c2", Status: "fail"},
				{RunDigest: "d1", StartedAt: "2026-09-02T00:00:00Z", Commit: "c1", Status: "pass"},
			},
		}}
		if regs := certificateRegressions(certs); len(regs) != 0 {
			t.Fatalf("invariant-kind certificate leaked into certificateRegressions: %+v", regs)
		}
	})

	t.Run("newest change first across several certificates", func(t *testing.T) {
		certs := []CertHistory{
			{Service: "fathom", Kind: "spec", TargetID: "spec:a", Test: "TestA", Reached: true, Runs: []store.TestCaseRun{
				{RunDigest: "a2", StartedAt: "2026-09-05T00:00:00Z", Commit: "ca2", Status: "fail"},
				{RunDigest: "a1", StartedAt: "2026-09-01T00:00:00Z", Commit: "ca1", Status: "pass"},
			}},
			{Service: "fathom", Kind: "spec", TargetID: "spec:b", Test: "TestB", Reached: true, Runs: []store.TestCaseRun{
				{RunDigest: "b2", StartedAt: "2026-09-09T00:00:00Z", Commit: "cb2", Status: "fail"},
				{RunDigest: "b1", StartedAt: "2026-09-01T00:00:00Z", Commit: "cb1", Status: "pass"},
			}},
		}
		regs := certificateRegressions(certs)
		if len(regs) != 2 || regs[0].ID != "spec:b" || regs[1].ID != "spec:a" {
			t.Fatalf("want spec:b (newer) before spec:a, got %+v", regs)
		}
	})
}

func TestConservationAndInvariantRegressions(t *testing.T) {
	t.Run("an invariant's enforcing test that stopped reaching or passing", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "invariant", TargetID: "I1", Test: "TestI1", Reached: true,
			Runs: []store.TestCaseRun{
				{RunDigest: "d2", StartedAt: "2026-09-06T00:00:00Z", Commit: "c2", Status: "fail"},
				{RunDigest: "d1", StartedAt: "2026-09-02T00:00:00Z", Commit: "c1", Status: "pass"},
			},
		}}
		regs := conservationRegressions(certs, nil)
		if len(regs) != 1 {
			t.Fatalf("want one regression, got %+v", regs)
		}
		if r := regs[0]; r.Kind != "invariant" || r.ID != "I1" || r.ProvenCommit != "c1" || r.ChangedCommit != "c2" || r.State != "broken" {
			t.Fatalf("invariant regression wrong: %+v", r)
		}
	})

	t.Run("a plane once conserved that no longer reads so", func(t *testing.T) {
		planes := []ConservationReading{
			{Service: "fathom", Plane: "integrity", At: "2026-09-01T00:00:00Z", Commit: "c1", Verdict: "conserved"},
			{Service: "fathom", Plane: "integrity", At: "2026-09-05T00:00:00Z", Commit: "c2", Verdict: "conserved"},
			{Service: "fathom", Plane: "integrity", At: "2026-09-09T00:00:00Z", Commit: "c3", Verdict: "losses"},
		}
		regs := conservationRegressions(nil, planes)
		if len(regs) != 1 {
			t.Fatalf("want one regression, got %+v", regs)
		}
		if r := regs[0]; r.Kind != "conservation" || r.ID != "integrity" || r.ProvenCommit != "c2" || r.ChangedCommit != "c3" || r.State != "losses" {
			t.Fatalf("conservation regression wrong: %+v", r)
		}
	})

	t.Run("still conserved, and no reading at all, both report nothing", func(t *testing.T) {
		planes := []ConservationReading{
			{Service: "fathom", Plane: "surface", At: "2026-09-01T00:00:00Z", Commit: "c1", Verdict: "conserved"},
			{Service: "fathom", Plane: "surface", At: "2026-09-05T00:00:00Z", Commit: "c2", Verdict: "conserved"},
		}
		if regs := conservationRegressions(nil, planes); len(regs) != 0 {
			t.Fatalf("want no regressions, got %+v", regs)
		}
		if regs := conservationRegressions(nil, nil); len(regs) != 0 {
			t.Fatalf("no input reports no regressions, not an error: %+v", regs)
		}
	})

	t.Run("both sources combine, newest change first", func(t *testing.T) {
		certs := []CertHistory{{
			Service: "fathom", Kind: "invariant", TargetID: "I1", Test: "TestI1", Reached: true,
			Runs: []store.TestCaseRun{
				{RunDigest: "d2", StartedAt: "2026-09-05T00:00:00Z", Commit: "c2", Status: "fail"},
				{RunDigest: "d1", StartedAt: "2026-09-01T00:00:00Z", Commit: "c1", Status: "pass"},
			},
		}}
		planes := []ConservationReading{
			{Service: "fathom", Plane: "integrity", At: "2026-09-01T00:00:00Z", Commit: "p1", Verdict: "conserved"},
			{Service: "fathom", Plane: "integrity", At: "2026-09-09T00:00:00Z", Commit: "p2", Verdict: "losses"},
		}
		regs := conservationRegressions(certs, planes)
		if len(regs) != 2 || regs[0].Kind != "conservation" || regs[1].Kind != "invariant" {
			t.Fatalf("want the conservation regression (newer) before the invariant one: %+v", regs)
		}
	})
}

func TestAllProofRegressionsMerged(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false) // A-2 regresses at c3, 2026-09-06T09:00:00Z (PE-8 fixture)
	certs := []CertHistory{{
		Service: "svc", Kind: "spec", TargetID: "spec:foo", Test: "TestFoo", Reached: true,
		Runs: []store.TestCaseRun{
			{RunDigest: "d2", StartedAt: "2026-09-10T00:00:00Z", Commit: "cd2", Status: "fail"},
			{RunDigest: "d1", StartedAt: "2026-09-03T00:00:00Z", Commit: "cd1", Status: "pass"},
		},
	}, {
		Service: "svc", Kind: "invariant", TargetID: "I1", Test: "TestInvariant", Reached: true,
		Runs: []store.TestCaseRun{
			{RunDigest: "i2", StartedAt: "2026-09-08T00:00:00Z", Commit: "ci2", Status: "fail"},
			{RunDigest: "i1", StartedAt: "2026-09-03T00:00:00Z", Commit: "ci1", Status: "pass"},
		},
	}}
	planes := []ConservationReading{
		{Service: "svc", Plane: "integrity", At: "2026-09-03T00:00:00Z", Commit: "p1", Verdict: "conserved"},
		{Service: "svc", Plane: "integrity", At: "2026-09-07T00:00:00Z", Commit: "p2", Verdict: "losses"},
	}
	regs := AllProofRegressions(h, HistoryFilter{}, certs, planes)
	if len(regs) != 4 {
		t.Fatalf("want 4 merged regressions (plan, certificate, conservation, invariant), got %+v", regs)
	}
	// Newest change first, retaining the commits needed to identify each change.
	for i, want := range []struct{ kind, proven, changed string }{
		{"certificate", "cd1", "cd2"},
		{"invariant", "ci1", "ci2"},
		{"conservation", "p1", "p2"},
		{"plan", "c1", "c3"},
	} {
		if got := regs[i]; got.Kind != want.kind || got.ProvenCommit != want.proven || got.ChangedCommit != want.changed {
			t.Errorf("regression %d: want %+v, got %+v", i, want, got)
		}
	}
	if regs[3].ID != "A-2" {
		t.Fatalf("the plan regression should still be PE-8's A-2: %+v", regs[3])
	}
}
