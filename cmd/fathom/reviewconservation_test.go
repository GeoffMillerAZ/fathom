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

// TestReview_ConservationDimensionLoadsCoverage pins defect fixes for
// CV-65/design §3.4/§3.5: `fathom review --since` used to build its
// conserve.Input via conserve.NewInput and NEVER load a coverage profile
// at all, so the conservation dimension's residue always answered from
// static reach even when [tests] results_dir held a usable, bound
// coverage.meta.json — a DIFFERENT answer than `fathom conserve` gave for
// the identical range. F here is never reached by any indexed test at
// all (statically "unreached"), but a usable profile shows its only
// changed block never ran either — with coverage wired in, the item must
// read class "unexecuted" with evidence "executed", not the static
// "unreached"/"static-reach" this dimension always reported before.
// Also pins that the wire JSON now carries the additive
// partial/unexecuted/evidence/unexecuted_lines/coverage_unusable fields
// the CHANGELOG already claimed (defect: payload() silently dropped
// them).
func TestReview_ConservationDimensionLoadsCoverage(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.com/widgets\n\ngo 1.22\n")
	repo.writePlan("pkg/f.go", "package pkg\n\nfunc F() int {\n\treturn 1\n}\n")
	repo.commitAll("base")
	base := headSHAOf(t, repo.root)

	// F's only changed line, never executed by anything.
	repo.writePlan("pkg/f.go", "package pkg\n\nfunc F() int {\n\treturn 2\n}\n")
	repo.commitAll("head edit — F's changed line never runs")
	head := headSHAOf(t, repo.root)

	repo.writePlan(".fathom/config.toml", "[tests]\nresults_dir = \"results\"\n")
	repo.writePlan("results/coverage.out",
		"mode: set\nexample.com/widgets/pkg/f.go:3.15,5.2 1 0\n"+
			// a key from a module this repository does not hold: a
			// partial mapping the payload must count, not drop
			"example.com/elsewhere/x.go:1.1,2.2 1 1\n")
	repo.writePlan("results/coverage.meta.json", `{
  "schema": "fathom.coverage/v1",
  "commit": "`+head+`",
  "dirty": false,
  "race": false,
  "profile": "coverage.out",
  "passed": true
}
`)
	repo.commitAll("coverage fixtures — a clean tree for resolveIndexAtHead's dirty check")

	storePath := filepath.Join(t.TempDir(), "review-coverage.sqlite")
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
	residue, _ := cons["residue"].(map[string]any)
	if residue == nil {
		t.Fatalf("conservation dimension carries no residue: %+v", cons)
	}
	if u, _ := residue["unexecuted"].(float64); u != 1 {
		t.Errorf("residue.unexecuted = %v, want 1 — the review dimension must load the SAME coverage profile `fathom conserve` would (previously always 0: coverage was never loaded here at all); residue=%+v", residue["unexecuted"], residue)
	}
	items, _ := residue["items"].([]any)
	var found bool
	for _, it := range items {
		im := it.(map[string]any)
		if im["symbol"] != "F" {
			continue
		}
		found = true
		if im["class"] != "unexecuted" {
			t.Errorf("F.class = %v, want %q", im["class"], "unexecuted")
		}
		if im["evidence"] != "executed" {
			t.Errorf("F.evidence = %v, want %q — execution evidence must reach this dimension, not just fathom conserve", im["evidence"], "executed")
		}
	}
	if !found {
		t.Fatalf("F not found in residue.items (want class=unexecuted evidence=executed): %+v", items)
	}
	if n, _ := residue["coverage_unmapped"].(float64); n != 1 {
		t.Errorf("residue.coverage_unmapped = %v, want 1 (the unplaced example.com/elsewhere key)", residue["coverage_unmapped"])
	}
	if sample, _ := residue["coverage_unmapped_sample"].([]any); len(sample) != 1 || sample[0] != "example.com/elsewhere/x.go" {
		t.Errorf("residue.coverage_unmapped_sample = %v, want [example.com/elsewhere/x.go]", residue["coverage_unmapped_sample"])
	}
	if _, ok := residue["coverage_unusable"]; ok {
		t.Errorf("residue.coverage_unusable = %v, want absent — a partial mapping is not total failure", residue["coverage_unusable"])
	}
}
