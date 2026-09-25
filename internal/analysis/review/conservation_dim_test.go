package review_test

// conservation_dim_test.go — CV-24 (ADR-044 §1.6/§2): the conservation
// dimension's own contract — unchecked without a base ref (mirrors
// DimIntegrity), unchecked when the caller could not build a conserve
// input at all, and — given a caller-precomputed set of
// ConservationPlaneSummary rows — a finding per plane Finding (severity
// warn unless Declared=="prior") plus an informational finding per plane
// Missing entry, never silently dropped.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

func TestReview_ConservationDimension(t *testing.T) {
	t.Run("no base ref", func(t *testing.T) {
		v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
		d := dimension(t, v, review.DimConservation)
		if d.Checked {
			t.Fatal("conservation should be unchecked without a base ref")
		}
		if d.Reason != "needs a base ref" {
			t.Errorf("Reason = %q, want %q", d.Reason, "needs a base ref")
		}
	})

	t.Run("base given but input could not be built", func(t *testing.T) {
		v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef"})
		d := dimension(t, v, review.DimConservation)
		if d.Checked {
			t.Fatal("conservation should be unchecked when ConservationSupported is false")
		}
		if d.Reason == "" {
			t.Error("expected a non-empty reason")
		}
	})

	t.Run("plane findings and missing inputs both surface", func(t *testing.T) {
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
			ConservationPlanes: []review.ConservationPlaneSummary{
				{
					Plane: "surface", Verdict: "losses",
					Findings: []review.ConservationFinding{
						{Plane: "surface", Kind: "removed", Key: "api:foo.Old", Detail: "func()", Declared: "none"},
						{Plane: "surface", Kind: "removed", Key: "api:foo.Excused", Detail: "func()", Declared: "prior"},
					},
				},
				{Plane: "certificates", Verdict: "unproven", Missing: []string{"no store"}},
			},
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatalf("expected conservation to be checked; reason=%q", d.Reason)
		}
		if d.Count != 3 {
			t.Fatalf("Count = %d, want 3 (two findings + one missing-input note); findings=%+v", d.Count, d.Findings)
		}

		var sawUndeclared, sawExcused, sawMissing bool
		for _, f := range d.Findings {
			switch {
			case f.Message == "api:foo.Old: func()":
				sawUndeclared = true
				if f.Severity != "warn" {
					t.Errorf("undeclared loss severity = %q, want warn", f.Severity)
				}
			case f.Message == "api:foo.Excused: func()":
				sawExcused = true
				if f.Severity != "info" {
					t.Errorf("prior-declared loss severity = %q, want info", f.Severity)
				}
			case f.Kind == "conservation_unproven":
				sawMissing = true
			}
		}
		if !sawUndeclared || !sawExcused || !sawMissing {
			t.Errorf("missing an expected finding: undeclared=%v excused=%v missing=%v (findings=%+v)", sawUndeclared, sawExcused, sawMissing, d.Findings)
		}
	})

	t.Run("supported with no planes is a clean checked pass", func(t *testing.T) {
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatal("expected conservation to be checked")
		}
		if d.Count != 0 {
			t.Errorf("Count = %d, want 0", d.Count)
		}
	})
}

// TestReview_ConservationDimensionCarriesResidue pins CV-65 (design §3.4):
// the conservation dimension carries the reading set's own counts and its
// first five `unreached` items, alongside — never in place of — its
// ordinary plane findings, and never touches Checked/Reason on its own.
func TestReview_ConservationDimensionCarriesResidue(t *testing.T) {
	unreachedItem := func(symbol string, callers int) review.ConservationResidueItem {
		return review.ConservationResidueItem{Symbol: symbol, File: "pkg/a.go", Line: 1, Change: "added", Class: "unreached", Callers: callers}
	}

	t.Run("counts and a capped-to-5 unreached list ride alongside plane findings", func(t *testing.T) {
		items := []review.ConservationResidueItem{
			unreachedItem("A", 6), unreachedItem("B", 5), unreachedItem("C", 4),
			unreachedItem("D", 3), unreachedItem("E", 2), unreachedItem("F", 1),
			{Symbol: "G", File: "pkg/a.go", Line: 2, Change: "modified", Class: "unattested", Tests: []string{"TestG"}},
		}
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
			ConservationPlanes: []review.ConservationPlaneSummary{
				{Plane: "surface", Verdict: "conserved"},
			},
			ConservationResidue: review.ConservationResidueSummary{
				Changed: 7, Attested: 0, Unattested: 1, Unreached: 6,
				Items: items, Truncated: 3, Unanalyzed: []string{"README.md"}, Missing: []string{},
			},
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatalf("expected conservation to be checked; reason=%q", d.Reason)
		}
		if d.Residue.Changed != 7 || d.Residue.Attested != 0 || d.Residue.Unattested != 1 || d.Residue.Unreached != 6 {
			t.Errorf("Residue counts = %+v, want changed=7 attested=0 unattested=1 unreached=6", d.Residue)
		}
		if d.Residue.Truncated != 3 {
			t.Errorf("Residue.Truncated = %d, want 3 (carried through, not recomputed)", d.Residue.Truncated)
		}
		if len(d.Residue.Unanalyzed) != 1 || d.Residue.Unanalyzed[0] != "README.md" {
			t.Errorf("Residue.Unanalyzed = %v, want [README.md]", d.Residue.Unanalyzed)
		}
		if len(d.Residue.Items) != 5 {
			t.Fatalf("Residue.Items = %+v, want exactly 5 (first five unreached, never more)", d.Residue.Items)
		}
		wantSymbols := []string{"A", "B", "C", "D", "E"}
		for i, sym := range wantSymbols {
			if d.Residue.Items[i].Symbol != sym || d.Residue.Items[i].Class != "unreached" {
				t.Errorf("Residue.Items[%d] = %+v, want symbol=%q class=unreached", i, d.Residue.Items[i], sym)
			}
		}
		for _, it := range d.Residue.Items {
			if it.Symbol == "F" || it.Symbol == "G" {
				t.Errorf("Residue.Items must cap at the first five unreached and never include an unattested item: %+v", d.Residue.Items)
			}
		}
		// The residue rides ALONGSIDE the plane findings, never instead
		// of them — an empty-findings plane summary still leaves the
		// dimension checked with a clean plane count.
		if d.Count != 0 {
			t.Errorf("Count = %d, want 0 (the residue itself contributes no Finding)", d.Count)
		}
	})

	t.Run("an unproven residue still surfaces its own Missing, never as a plane finding", func(t *testing.T) {
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
			ConservationResidue: review.ConservationResidueSummary{
				Missing: []string{"the index is not at HEAD"},
			},
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatalf("expected conservation to be checked; reason=%q", d.Reason)
		}
		if len(d.Residue.Items) != 0 {
			t.Errorf("Residue.Items = %+v, want none (nothing was classified)", d.Residue.Items)
		}
		if len(d.Residue.Missing) != 1 || d.Residue.Missing[0] != "the index is not at HEAD" {
			t.Errorf("Residue.Missing = %v, want [the index is not at HEAD]", d.Residue.Missing)
		}
		if d.Count != 0 {
			t.Errorf("Count = %d, want 0 (an unproven residue is never rendered as a plane Finding)", d.Count)
		}
	})
}

// TestReview_ConservationDimensionResidueReachesPayload pins that the
// residue set on Dimension by runConservation actually reaches the wire:
// design §3.4/CHANGELOG/docs/usage.md all promise the review conservation
// dimension carries the reading set's counts and first five unreached
// items on BOTH review_diff (Verdict.Payload) and `fathom review --json`
// (ReviewResponse.Payload) — never only on the in-memory struct
// TestReview_ConservationDimensionCarriesResidue already pins.
func TestReview_ConservationDimensionResidueReachesPayload(t *testing.T) {
	v := review.Run(review.Input{
		Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
		ConservationSupported: true,
		ConservationResidue: review.ConservationResidueSummary{
			Changed: 1, Attested: 0, Unattested: 0, Unreached: 1,
			Items: []review.ConservationResidueItem{
				{Symbol: "ZZUNIQUE", File: "pkg/a.go", Line: 3, Change: "added", Class: "unreached", Tests: []string{}, Callers: 0},
			},
			Truncated: 0, Unanalyzed: []string{}, Missing: []string{},
		},
	})

	payload := v.Payload(false)
	dims, ok := payload["dimensions"].([]any)
	if !ok {
		t.Fatalf("dimensions = %v, want a slice", payload["dimensions"])
	}
	var conservation map[string]any
	for _, d := range dims {
		dm := d.(map[string]any)
		if dm["name"] == review.DimConservation {
			conservation = dm
		}
	}
	if conservation == nil {
		t.Fatalf("no conservation dimension in payload: %+v", dims)
	}
	residue, ok := conservation["residue"].(map[string]any)
	if !ok {
		t.Fatalf("conservation dimension has no \"residue\" object: %+v", conservation)
	}
	if residue["changed"] != 1 || residue["unreached"] != 1 {
		t.Errorf("residue counts = %+v, want changed=1 unreached=1", residue)
	}
	items, ok := residue["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("residue.items = %v, want exactly 1 entry", residue["items"])
	}
	item := items[0].(map[string]any)
	if item["symbol"] != "ZZUNIQUE" || item["class"] != "unreached" {
		t.Errorf("residue.items[0] = %+v, want symbol=ZZUNIQUE class=unreached", item)
	}
}
