package fusion_test

// typedcalls_goworkfallback_test.go — wave P3 defect 2 / O82
// (.fathom/observations.md): a Go workspace (go.work) naming a sibling
// checkout that does not exist beside THIS repo's own checkout — the exact
// 2026-09-16 incident (a push clone under /private/tmp, ingested by a
// daemon process whose ambient GOWORK pointed at the operator's own
// workspace, which names siblings that exist only beside the volume
// checkout) — must never break typed-edge enrichment. TypedEdgeEnv already
// forces GOWORK=off unconditionally (BP-35, typedenv.go) so packages.Load
// never even attempts to resolve such a workspace; this test pins that
// contract directly against a go.work naming a missing path, and pins the
// disclosure (TypedEdgeReport.Fallback) that names when an ambient
// workspace was overridden.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeGoWorkFixture builds:
//
//	<tmp>/go.work            use ./thismod and ../does-not-exist-sibling
//	<tmp>/thismod/go.mod     module example.com/thismod
//	<tmp>/thismod/main.go    a trivial two-function package
//
// and returns thismod's absolute path — the repoPath EnrichTypedEdges is
// called with, exactly as the pipeline calls it with a service's own root,
// never the workspace root.
func writeGoWorkFixture(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mod := filepath.Join(tmp, "thismod")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	goWork := "go 1.21\n\nuse ./thismod\nuse ../does-not-exist-sibling\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.work"), []byte(goWork), 0o644); err != nil {
		t.Fatalf("write go.work: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/thismod\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	src := "package thismod\n\nfunc Caller() int {\n\treturn helper()\n}\n\nfunc helper() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(mod, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return mod
}

// TestEnrichTypedEdges_GoWorkMissingSiblingSurvives is the defect-2
// regression: a go.work at repoPath's PARENT naming a sibling that does not
// exist must never fail (or degrade) typed-edge enrichment — GOWORK=off
// (unconditional, TypedEdgeEnv) means packages.Load never even attempts to
// resolve the broken workspace.
func TestEnrichTypedEdges_GoWorkMissingSiblingSurvives(t *testing.T) {
	// Neutralize any GOWORK this process itself inherited (this repo's own
	// gate commands export GOWORK=off around every go invocation) so the
	// fixture's go.work is reachable by auto-discovery exactly as an
	// unconfigured daemon process would see it — an explicitly-set
	// GOWORK=off here would make this fixture indistinguishable from "no
	// workspace at all".
	t.Setenv("GOWORK", "")
	mod := writeGoWorkFixture(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "goworkfixture", mod, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges with a go.work naming a missing sibling: %v", err)
	}
	if report.PackagesFailed != 0 {
		t.Errorf("PackagesFailed = %d, want 0 — GOWORK=off must keep the broken workspace out of the load entirely", report.PackagesFailed)
	}
	if report.PackagesLoaded == 0 {
		t.Fatal("PackagesLoaded = 0 — the broken go.work broke the load despite GOWORK=off")
	}
	edges, err := st.TypedEdges(ctx, "goworkfixture")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("no typed edges stored — the pass degraded to syntax-only despite GOWORK=off")
	}
}

// TestEnrichTypedEdges_GoWorkFallbackDisclosed pins the disclosure half:
// when an ambient workspace WOULD have applied (a go.work discoverable from
// repoPath) GOWORK=off suppresses it, report.Fallback must name that so a
// caller — ingest's typed_edges.fallback field — can see the override
// happened, not just that edges came out fine.
func TestEnrichTypedEdges_GoWorkFallbackDisclosed(t *testing.T) {
	t.Setenv("GOWORK", "") // see TestEnrichTypedEdges_GoWorkMissingSiblingSurvives
	mod := writeGoWorkFixture(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "goworkfixture", mod, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	if report.Fallback != "GOWORK=off" {
		t.Errorf("report.Fallback = %q, want %q — a discoverable go.work was overridden and must be disclosed", report.Fallback, "GOWORK=off")
	}
}

// TestEnrichTypedEdges_NoGoWorkNoFallback: the common case (no go.work
// anywhere above repoPath) must report an EMPTY Fallback — never claim an
// override happened when nothing was there to override.
func TestEnrichTypedEdges_NoGoWorkNoFallback(t *testing.T) {
	t.Setenv("GOWORK", "") // see TestEnrichTypedEdges_GoWorkMissingSiblingSurvives
	dir := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	if report.Fallback != "" {
		t.Errorf("report.Fallback = %q, want empty — no go.work applies to this fixture", report.Fallback)
	}
}
