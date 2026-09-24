package conserve

// conserve_test.go — CV-18 (ADR-044 §2, I44): the conservation runner —
// planes report findings and missing inputs; Run computes each plane's
// verdict and produces a sorted, schema-stable report. Fake planes only:
// the real planes (integrity, surface, certificates, governance, absence,
// resurrection, generated) are later rows/waves, registered from their own
// init().

import (
	"context"
	"encoding/json"
	"testing"
)

// fakePlane is a minimal Plane for exercising Run's verdict/sort logic
// without any real analysis.
type fakePlane struct {
	name     string
	findings []Finding
	missing  []string
}

func (p fakePlane) Name() string { return p.name }
func (p fakePlane) Check(ctx context.Context, in Input) ([]Finding, []string) {
	return p.findings, p.missing
}

func TestRun_PlanesVerdictsAndMissingInputs(t *testing.T) {
	in := Input{Service: "fathom", Base: "abc123", Head: "worktree"}

	planes := []Plane{
		// Registered out of alphabetical order on purpose — Run must sort
		// the OUTPUT by plane name regardless of call order.
		fakePlane{name: "surface", findings: []Finding{
			{Plane: "surface", Kind: "changed", Key: "api:z.Z", Declared: "none"},
			{Plane: "surface", Kind: "changed", Key: "api:a.A", Declared: "none"},
			{Plane: "surface", Kind: "removed", Key: "api:m.M", Declared: "none"},
		}},
		fakePlane{name: "absence"}, // no findings, no missing -> conserved
		fakePlane{name: "certificates", missing: []string{"no store"}},
		fakePlane{name: "governance", findings: []Finding{
			{Plane: "governance", Kind: "orphaned", Key: "rule:1", Declared: "prior"},
		}},
	}

	rep := Run(context.Background(), in, planes)

	if rep.Schema != "fathom.conserve/v1" {
		t.Errorf("Schema = %q, want fathom.conserve/v1", rep.Schema)
	}
	if rep.Service != "fathom" || rep.Base != "abc123" || rep.Head != "worktree" {
		t.Errorf("Report top-level fields not propagated from Input: %+v", rep)
	}
	if len(rep.Planes) != 4 {
		t.Fatalf("got %d plane reports, want 4", len(rep.Planes))
	}

	// Sorted by plane name.
	wantOrder := []string{"absence", "certificates", "governance", "surface"}
	for i, want := range wantOrder {
		if rep.Planes[i].Plane != want {
			t.Errorf("plane[%d] = %q, want %q (order: %v)", i, rep.Planes[i].Plane, want, planeNames(rep.Planes))
		}
	}

	byName := map[string]PlaneReport{}
	for _, pr := range rep.Planes {
		byName[pr.Plane] = pr
	}

	if v := byName["absence"].Verdict; v != Conserved {
		t.Errorf("absence verdict = %q, want %q (no findings, no missing)", v, Conserved)
	}
	if v := byName["certificates"].Verdict; v != Unproven {
		t.Errorf("certificates verdict = %q, want %q (missing input beats everything)", v, Unproven)
	}
	if got := byName["certificates"].Missing; len(got) != 1 || got[0] != "no store" {
		t.Errorf("certificates missing = %v, want [no store]", got)
	}
	if v := byName["governance"].Verdict; v != Conserved {
		t.Errorf("governance verdict = %q, want %q (its one finding is declared prior)", v, Conserved)
	}
	if v := byName["surface"].Verdict; v != Losses {
		t.Errorf("surface verdict = %q, want %q (undeclared findings)", v, Losses)
	}

	// Findings sorted by (kind, key) within a plane.
	sf := byName["surface"].Findings
	if len(sf) != 3 {
		t.Fatalf("surface findings = %d, want 3", len(sf))
	}
	wantKeys := []string{"api:a.A", "api:z.Z", "api:m.M"} // changed:a, changed:z, removed:m
	for i, want := range wantKeys {
		if sf[i].Key != want {
			t.Errorf("surface finding[%d].Key = %q, want %q", i, sf[i].Key, want)
		}
	}
	if sf[0].Kind != "changed" || sf[2].Kind != "removed" {
		t.Errorf("kind ordering wrong: %+v", sf)
	}

	// JSON is schema-stable: the "schema" field is the wire contract, and
	// marshal must not error (Run's own slices, never a map, so encoding/
	// json's field order is already deterministic — this pins it).
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if round["schema"] != "fathom.conserve/v1" {
		t.Errorf("marshaled schema = %v, want fathom.conserve/v1", round["schema"])
	}
}

func planeNames(prs []PlaneReport) []string {
	out := make([]string, len(prs))
	for i, pr := range prs {
		out[i] = pr.Plane
	}
	return out
}

// TestRun_EmptyPlanesIsEmptyConservedReport pins the degenerate case: no
// planes registered/passed at all still produces a well-formed, empty
// report rather than a nil slice mismatch.
func TestRun_EmptyPlanesIsEmptyConservedReport(t *testing.T) {
	rep := Run(context.Background(), Input{Service: "svc"}, nil)
	if rep.Schema != "fathom.conserve/v1" || len(rep.Planes) != 0 {
		t.Fatalf("empty Run = %+v", rep)
	}
}

// TestRun_AllFindingsDeclaredPriorIsConserved: several findings, ALL
// declared prior — none excuse a fail, so the plane is conserved, not
// merely "not unproven."
func TestRun_AllFindingsDeclaredPriorIsConserved(t *testing.T) {
	planes := []Plane{fakePlane{name: "p", findings: []Finding{
		{Plane: "p", Kind: "k", Key: "1", Declared: "prior"},
		{Plane: "p", Kind: "k", Key: "2", Declared: "prior"},
	}}}
	rep := Run(context.Background(), Input{Service: "svc"}, planes)
	if rep.Planes[0].Verdict != Conserved {
		t.Errorf("verdict = %q, want conserved", rep.Planes[0].Verdict)
	}
}

// TestRun_OneSelfDeclaredFindingAmongPriorOnesIsLosses: a self-declared
// finding excuses nothing (I45) even alongside other, properly prior ones.
func TestRun_OneSelfDeclaredFindingAmongPriorOnesIsLosses(t *testing.T) {
	planes := []Plane{fakePlane{name: "p", findings: []Finding{
		{Plane: "p", Kind: "k", Key: "1", Declared: "prior"},
		{Plane: "p", Kind: "k", Key: "2", Declared: "self"},
	}}}
	rep := Run(context.Background(), Input{Service: "svc"}, planes)
	if rep.Planes[0].Verdict != Losses {
		t.Errorf("verdict = %q, want losses", rep.Planes[0].Verdict)
	}
}

// TestRun_MissingBeatsFindings: a plane with BOTH a missing input and
// undeclared findings still reports unproven — missing always wins (I44).
func TestRun_MissingBeatsFindings(t *testing.T) {
	planes := []Plane{fakePlane{
		name:     "p",
		findings: []Finding{{Plane: "p", Kind: "k", Key: "1", Declared: "none"}},
		missing:  []string{"index not at HEAD"},
	}}
	rep := Run(context.Background(), Input{Service: "svc"}, planes)
	if rep.Planes[0].Verdict != Unproven {
		t.Errorf("verdict = %q, want unproven", rep.Planes[0].Verdict)
	}
}
