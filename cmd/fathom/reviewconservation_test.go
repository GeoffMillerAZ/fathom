package main

// reviewconservation_test.go — DimConservation's cmd/fathom wiring (ADR-044
// §1.6/§2, wave 2): `fathom review --since <ref>` runs internal/conserve's
// full plane set (conserve.Planes()) and reports a checked "conservation"
// dimension with real findings, end to end against a throwaway git repo —
// the same selfSkipGitRepo/runInDir fixture pattern cmd/fathom/conserve_test.go
// already establishes, since this wiring (like cmdConserve) shells real git
// via conserve.NewInput rather than the injected gitRunner.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

// TestReview_ConservationDimensionWiredEndToEnd pins that `fathom review
// --since` no longer reports checked:false for "conservation" once a base
// resolves: removing an exported function between base and head must
// surface as a checked conservation dimension naming the surface-plane
// loss.
func TestReview_ConservationDimensionWiredEndToEnd(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc Old() {}\n")
	repo.commitAll("base commit")
	base := headSHAOf(t, repo.root)

	// Remove the exported func at head — an undeclared surface loss,
	// exactly TestCmdConserve_JSONSchemaV1's own fixture shape.
	repo.writePlan("widgets/w.go", "package widgets\n\nfunc New() {}\n")

	storePath := filepath.Join(t.TempDir(), "review-conservation.sqlite")
	if code, _, stderr := runInDir(t, repo.root, []string{
		"ingest", "--service", "svc", "--path", repo.root, "--store", storePath,
	}); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, stderr)
	}

	code, stdout, stderr := runInDir(t, repo.root, []string{
		"review", "--service", "svc", "--since", base, "--store", storePath,
		"--no-persist", "--json",
	})
	if code != 0 {
		t.Fatalf("review exit = %d, stderr: %s", code, stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\nstdout=%s", err, stdout)
	}
	dims, _ := payload["dimensions"].([]any)
	var cons map[string]any
	for _, d := range dims {
		dm := d.(map[string]any)
		if dm["name"] == "conservation" {
			cons = dm
		}
	}
	if cons == nil {
		t.Fatalf("no \"conservation\" dimension in payload: %+v", payload)
	}
	if checked, _ := cons["checked"].(bool); !checked {
		t.Fatalf("conservation.checked = %v, want true (a base was resolved and the corpus is readable); dimension=%+v", cons["checked"], cons)
	}
	findings, _ := cons["findings"].([]any)
	var sawSurfaceLoss bool
	for _, f := range findings {
		fm := f.(map[string]any)
		if fm["kind"] == "conservation_removed" {
			sawSurfaceLoss = true
		}
	}
	if !sawSurfaceLoss {
		t.Errorf("expected a conservation_removed finding for the deleted api:widgets.Old; findings=%+v", findings)
	}
}

// TestApplyConservationDimension_NoBaseIsNoOp pins the "no --since"
// degrade at the wiring's own unit level: applyConservationDimension
// never touches ConservationSupported/ConservationPlanes when Base is
// unset — DimConservation's own "needs a base ref" gate (internal/
// analysis/review) is the only thing a --since-less run ever reports.
func TestApplyConservationDimension_NoBaseIsNoOp(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("a.go", "package a\n")
	repo.commitAll("only commit")

	in := review.Input{Root: repo.root} // Base deliberately unset
	applyConservationDimension(context.Background(), &in, nil, repo.root, "svc")
	if in.ConservationSupported {
		t.Errorf("ConservationSupported = true, want false (no --since given)")
	}
	if in.ConservationPlanes != nil {
		t.Errorf("ConservationPlanes = %+v, want nil", in.ConservationPlanes)
	}
}

// TestApplyConservationDimension_UnreadableCorpusFailsOpen pins that an
// unresolvable base (root not a git work tree at all) degrades to
// ConservationSupported:false rather than a panic or a hard error —
// review's OTHER dimensions must still run regardless.
func TestApplyConservationDimension_UnreadableCorpusFailsOpen(t *testing.T) {
	notARepo := t.TempDir()
	in := review.Input{Root: notARepo, Base: "deadbeef"}
	applyConservationDimension(context.Background(), &in, nil, notARepo, "svc")
	if in.ConservationSupported {
		t.Errorf("ConservationSupported = true, want false (root is not a git work tree)")
	}
}
