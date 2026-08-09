package main

// realization_test.go — phase Y. The grade must reward OUTCOME, not
// configuration, and the case that matters most is the one the incumbent
// grade gets wrong: a repository wired perfectly that delivers nothing.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func wiredRow(svc string) obsServiceRow {
	r := obsServiceRow{}
	r.Service = svc
	r.Path = "/repo/" + svc
	r.StanzaNote = "present"
	r.HooksNote = "all 6 fathom hook reference(s) wired (project scope)"
	r.Freshness = "fresh"
	return r
}

// TestRealizationGradeDimensions pins the headline behaviour change:
// perfect configuration is not an A.
func TestRealizationGradeDimensions(t *testing.T) {
	t.Run("wired and fresh but delivering nothing is NOT an A", func(t *testing.T) {
		// Exactly the repository the incumbent wiringGrade scores A:
		// stanza present, hooks wired, index fresh, and one call ever.
		got := realizationGrade(wiredRow("dormant"), realizationWindow{
			Calls: 0, Sessions: 4, ObligationServes: 0, VerifyRuns: 0, HasPlans: true,
		})
		if strings.HasPrefix(got.Grade, "A") {
			t.Fatalf("grade = %q — configuration alone must not earn an A", got.Grade)
		}
		if got.FirstGap != "queried" {
			t.Errorf("first gap = %q, want queried — the ladder's first BROKEN rung", got.FirstGap)
		}
		if !strings.Contains(got.Grade, "not queried") {
			t.Errorf("grade = %q — the letter must name the gap; a bare letter is not actionable", got.Grade)
		}
		if got.Remedy == "" {
			t.Error("the first gap must carry its remedy")
		}
	})

	t.Run("everything realized is an A", func(t *testing.T) {
		got := realizationGrade(wiredRow("healthy"), realizationWindow{
			Calls: 900, Sessions: 12, ObligationServes: 40, VerifyRuns: 6, HasPlans: true,
		})
		if got.Grade != "A" {
			t.Errorf("grade = %q, want a bare A with no gap named", got.Grade)
		}
		if got.Realized != 5 || got.Available != 5 {
			t.Errorf("realized/available = %d/%d, want 5/5", got.Realized, got.Available)
		}
	})

	t.Run("a repository with no plans is not punished for not gating", func(t *testing.T) {
		got := realizationGrade(wiredRow("noplans"), realizationWindow{
			Calls: 50, Sessions: 3, ObligationServes: 5, VerifyRuns: 0, HasPlans: false,
		})
		if got.Available != 4 {
			t.Errorf("available = %d, want 4 — a not-applicable dimension leaves the DENOMINATOR", got.Available)
		}
		if got.Grade != "A" {
			t.Errorf("grade = %q, want A — it realized everything available to it", got.Grade)
		}
		for _, d := range got.Dims {
			if d.Name == "gating" {
				if d.Met != nil {
					t.Error("gating must be not-applicable (nil), not false")
				}
				if !strings.Contains(d.Evidence, "no plans") {
					t.Errorf("a not-applicable dimension must say WHY: %q", d.Evidence)
				}
			}
		}
	})

	t.Run("unknown freshness is not a failed dimension", func(t *testing.T) {
		r := wiredRow("unreachable")
		r.Freshness = "unknown: statusz unreachable"
		got := realizationGrade(r, realizationWindow{Calls: 10, Sessions: 2, ObligationServes: 1, VerifyRuns: 1, HasPlans: true})
		for _, d := range got.Dims {
			if d.Name == "fresh" && d.Met != nil {
				t.Errorf("freshness that could not be determined must be not-applicable, not %v — "+
					"otherwise every repo with an unreachable daemon fails", *d.Met)
			}
		}
	})

	t.Run("every dimension always carries evidence", func(t *testing.T) {
		got := realizationGrade(obsServiceRow{}, realizationWindow{})
		if len(got.Dims) != 5 {
			t.Fatalf("want 5 dimensions, got %d", len(got.Dims))
		}
		for _, d := range got.Dims {
			if strings.TrimSpace(d.Evidence) == "" {
				t.Errorf("dimension %q has no evidence (law 3)", d.Name)
			}
		}
	})
}

// TestIdleIsNotFailure pins the cry-wolf guard: a reference corpus nobody
// codes in is working as intended.
func TestIdleIsNotFailure(t *testing.T) {
	r := obsServiceRow{}
	r.Service = "corpus"
	got := realizationGrade(r, realizationWindow{Calls: 0, Sessions: 0})
	if got.Grade != "idle" {
		t.Fatalf("grade = %q, want idle — a repository with no sessions is not failing", got.Grade)
	}
	if got.FirstGap != "" || got.Remedy != "" {
		t.Error("an idle repository must not be given a to-do; there is nothing wrong with it")
	}
}

// TestGradeIsPure pins the determinism contract.
func TestGradeIsPure(t *testing.T) {
	row := wiredRow("svc")
	w := realizationWindow{Calls: 3, Sessions: 2, ObligationServes: 1, VerifyRuns: 0, HasPlans: true}
	a := realizationGrade(row, w)
	b := realizationGrade(row, w)
	if a.Grade != b.Grade || a.Realized != b.Realized || a.FirstGap != b.FirstGap {
		t.Errorf("same inputs produced different results:\n%+v\n%+v", a, b)
	}
	if len(a.Dims) != len(b.Dims) {
		t.Fatal("dimension count differs between identical calls")
	}
	for i := range a.Dims {
		if a.Dims[i].Name != b.Dims[i].Name {
			t.Errorf("dimension order differs at %d: %q vs %q", i, a.Dims[i].Name, b.Dims[i].Name)
		}
	}
}

// TestRankByUnrealizedValue pins the fleet ordering: where is the most
// value on the table, not who is busiest.
func TestRankByUnrealizedValue(t *testing.T) {
	// busy: 2 gaps x 100 sessions = 200
	// small: 3 gaps x  10 sessions =  30
	// perfect: 0 gaps x 500 sessions = 0 (busiest, nothing to do)
	in := []realizationReport{
		{Service: "small", Realized: 2, Available: 5, Sessions: 10},
		{Service: "perfect", Realized: 5, Available: 5, Sessions: 500},
		{Service: "busy", Realized: 3, Available: 5, Sessions: 100},
	}
	got := rankByUnrealized(in)
	want := []string{"busy", "small", "perfect"}
	for i, w := range want {
		if got[i].Service != w {
			t.Fatalf("rank %d = %q, want %q — ranking must answer 'where is the most unrealized value', "+
				"not 'who is busiest' (got order: %v)", i, got[i].Service, w, names(got))
		}
	}

	t.Run("stable and deterministic under equal keys", func(t *testing.T) {
		tied := []realizationReport{
			{Service: "zebra", Realized: 4, Available: 5, Sessions: 10},
			{Service: "alpha", Realized: 4, Available: 5, Sessions: 10},
		}
		first := rankByUnrealized(tied)
		second := rankByUnrealized(tied)
		if first[0].Service != "alpha" {
			t.Errorf("ties must break deterministically on name, got %v", names(first))
		}
		if first[0].Service != second[0].Service {
			t.Error("ranking is not stable across calls")
		}
	})

	t.Run("the input slice is not mutated", func(t *testing.T) {
		orig := []realizationReport{
			{Service: "a", Realized: 0, Available: 5, Sessions: 1},
			{Service: "b", Realized: 5, Available: 5, Sessions: 900},
		}
		_ = rankByUnrealized(orig)
		if orig[0].Service != "a" {
			t.Error("rankByUnrealized must not reorder its caller's slice")
		}
	})
}

func names(rs []realizationReport) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Service
	}
	return out
}

// TestUnrealizedScoreDrivesTheFleetOrder pins RG-4's wiring: the ordering
// key the observatory sorts by is the ladder's, not the traffic count.
// Without this the ladder would be computed and then ignored — a pure
// function nobody's ordering depends on.
func TestUnrealizedScoreDrivesTheFleetOrder(t *testing.T) {
	withLadder := func(svc string, realized, available, sessions int) obsServiceRow {
		r := wiredRow(svc)
		rr := realizationReport{Service: svc, Realized: realized, Available: available, Sessions: sessions}
		r.Realization = &rr
		return r
	}

	// The busiest repository has nothing wrong with it and must NOT lead.
	perfect := withLadder("perfect", 5, 5, 900)
	perfect.CallsTotal = 99999
	gap := withLadder("gap", 3, 5, 50)
	gap.CallsTotal = 10

	if unrealizedScore(perfect) != 0 {
		t.Errorf("a fully-realized repository has nothing on the table: %d", unrealizedScore(perfect))
	}
	if unrealizedScore(gap) != 100 {
		t.Errorf("score = %d, want 100 (2 unmet x 50 sessions)", unrealizedScore(gap))
	}
	if unrealizedScore(gap) <= unrealizedScore(perfect) {
		t.Error("the busiest repository must not outrank the one with unrealized value")
	}

	t.Run("a row with no ladder is ranked last, never dropped", func(t *testing.T) {
		if got := unrealizedScore(obsServiceRow{}); got != 0 {
			t.Errorf("score = %d, want 0 — an unrankable row is still a row", got)
		}
	})
}

// TestRepoHasPlansDistinguishesAbsenceFromEmptiness pins the gating
// dimension's applicability test.
func TestRepoHasPlansDistinguishesAbsenceFromEmptiness(t *testing.T) {
	if repoHasPlans("") {
		t.Error("an unknown root has no plans")
	}
	root := t.TempDir()
	if repoHasPlans(root) {
		t.Error("a repo with no docs/plans has no plans")
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if repoHasPlans(root) {
		t.Error("an EMPTY docs/plans is still no plans")
	}
	// A README alone does not make a plan — otherwise every repo that
	// scaffolded the directory would be told it is failing to gate.
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "README.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if repoHasPlans(root) {
		t.Error("docs/plans/README.md alone is not a plan")
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "plans", "real.md"), []byte("- [ ] x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !repoHasPlans(root) {
		t.Error("a real plan file must count")
	}
}
