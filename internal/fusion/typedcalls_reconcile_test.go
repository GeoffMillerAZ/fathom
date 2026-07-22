package fusion_test

// Critic-round regression tests for the typed-edge pass (ADR-021 §3):
//
//   F1 — stale-package reconcile: deleted/moved package dirs must not leave
//        typed_edges/fingerprint rows behind (the fabrication path: a moved
//        package keeps its package NAME, hence its SymbolKeys, so a stale
//        row points at LIVE symbols).
//   F2 — connector scope: packages.Load("./...") knows nothing about
//        .fathomignore; an ignored dir's Go file with colliding
//        package/func names must produce no edges.
//   F3 — cache-hit file marking: a _test.go-only change must not
//        re-resolve the (cache-hit) plain package's files under the
//        test-variant ID (exact-duplicate rows).
//   F4 — Config.DisableTypedEdges skips the pass entirely.
//   F5 — the Ingest report carries the typed-pass counters.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeGoModule materialises files (path → content) under a fresh temp dir
// and returns the dir. Parent directories are created as needed.
func writeGoModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		abs := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return dir
}

// TestEnrichTypedEdges_ReconcileDeletedPackage is F1 regression (a): a
// package directory deleted between runs must lose its typed-edge rows AND
// its cache fingerprint on the next run.
func TestEnrichTypedEdges_ReconcileDeletedPackage(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod": "module example.com/recon\n\ngo 1.21\n",
		"a/a.go": "package a\n\nfunc A() int { return aHelper() }\n\nfunc aHelper() int { return 1 }\n",
		"b/b.go": "package b\n\nfunc B() int { return bHelper() }\n\nfunc bHelper() int { return 2 }\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()

	if _, err := fusion.EnrichTypedEdges(ctx, st, "recon", dir, nil); err != nil {
		t.Fatalf("first EnrichTypedEdges: %v", err)
	}
	edges, err := st.TypedEdges(ctx, "recon")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	var sawA, sawB bool
	for _, e := range edges {
		if e.FromSymbol == "recon::a.A" {
			sawA = true
		}
		if e.FromSymbol == "recon::b.B" {
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Fatalf("first run: expected edges from both packages, got %+v", edges)
	}

	if err := os.RemoveAll(filepath.Join(dir, "b")); err != nil {
		t.Fatalf("remove b: %v", err)
	}
	if _, err := fusion.EnrichTypedEdges(ctx, st, "recon", dir, nil); err != nil {
		t.Fatalf("second EnrichTypedEdges: %v", err)
	}
	edges, err = st.TypedEdges(ctx, "recon")
	if err != nil {
		t.Fatalf("TypedEdges after delete: %v", err)
	}
	for _, e := range edges {
		if strings.HasPrefix(e.FromSymbol, "recon::b.") {
			t.Errorf("deleted package's edge survived re-ingest: %+v", e)
		}
	}
	pkgs, err := st.ListTypedEdgePackages(ctx, "recon")
	if err != nil {
		t.Fatalf("ListTypedEdgePackages: %v", err)
	}
	for _, id := range pkgs {
		if strings.HasPrefix(id, "example.com/recon/b") {
			t.Errorf("deleted package's fingerprint/rows still listed: %q", id)
		}
	}
}

// TestEnrichTypedEdges_ReconcileMovedPackageDropsRemovedCall is F1
// regression (b) — the worst-case fabrication shape: `git mv` a package
// (import path changes, package NAME and therefore SymbolKeys do NOT) and
// simultaneously remove a call. Pre-fix, the old import path's rows
// survived forever, so the REMOVED call kept verifying as reachable.
func TestEnrichTypedEdges_ReconcileMovedPackageDropsRemovedCall(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod":     "module example.com/mv\n\ngo 1.21\n",
		"lib/lib.go": "package lib\n\nfunc F() int { return G() }\n\nfunc G() int { return 1 }\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()

	if _, err := fusion.EnrichTypedEdges(ctx, st, "mv", dir, nil); err != nil {
		t.Fatalf("first EnrichTypedEdges: %v", err)
	}
	edges, err := st.TypedEdges(ctx, "mv")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	var sawCall bool
	for _, e := range edges {
		if e.FromSymbol == "mv::lib.F" && e.ToSymbol == "mv::lib.G" {
			sawCall = true
		}
	}
	if !sawCall {
		t.Fatalf("first run: expected F->G edge, got %+v", edges)
	}

	// Move the package (new import path, same package name → same
	// SymbolKeys) and remove the F->G call.
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.Rename(filepath.Join(dir, "lib"), filepath.Join(dir, "pkg", "lib")); err != nil {
		t.Fatalf("mv lib: %v", err)
	}
	moved := "package lib\n\nfunc F() int { return 1 }\n\nfunc G() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(dir, "pkg", "lib", "lib.go"), []byte(moved), 0o644); err != nil {
		t.Fatalf("rewrite moved lib.go: %v", err)
	}

	if _, err := fusion.EnrichTypedEdges(ctx, st, "mv", dir, nil); err != nil {
		t.Fatalf("second EnrichTypedEdges: %v", err)
	}
	edges, err = st.TypedEdges(ctx, "mv")
	if err != nil {
		t.Fatalf("TypedEdges after move: %v", err)
	}
	for _, e := range edges {
		if e.FromSymbol == "mv::lib.F" && e.ToSymbol == "mv::lib.G" {
			t.Errorf("removed call's edge survived the package move (stale row under the old import path): %+v", e)
		}
	}
}

// TestEnrichTypedEdges_TestOnlyChangeNoDuplicates is F3 regression: a
// _test.go-only change leaves the plain package cache-hit; pre-fix the hit
// skipped processedFiles marking, so the test-variant re-resolved the plain
// files too and stored exact-duplicate rows.
func TestEnrichTypedEdges_TestOnlyChangeNoDuplicates(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod":      "module example.com/dup\n\ngo 1.21\n",
		"lib.go":      "package dup\n\nfunc A() int { return B() }\n\nfunc B() int { return 1 }\n",
		"lib_test.go": "package dup\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tif A() == 0 {\n\t\tt.Fail()\n\t}\n}\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()

	if _, err := fusion.EnrichTypedEdges(ctx, st, "dup", dir, nil); err != nil {
		t.Fatalf("first EnrichTypedEdges: %v", err)
	}
	before, err := st.TypedEdges(ctx, "dup")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("first run produced no edges")
	}

	// Touch ONLY the test file (append a comment — no semantic change).
	testFile := filepath.Join(dir, "lib_test.go")
	orig, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("read lib_test.go: %v", err)
	}
	if err := os.WriteFile(testFile, append(orig, []byte("\n// touched\n")...), 0o644); err != nil {
		t.Fatalf("write lib_test.go: %v", err)
	}

	rep, err := fusion.EnrichTypedEdges(ctx, st, "dup", dir, nil)
	if err != nil {
		t.Fatalf("second EnrichTypedEdges: %v", err)
	}
	if rep.PackagesCached == 0 {
		t.Errorf("second run: PackagesCached = 0, want the untouched plain package cache-hit")
	}
	after, err := st.TypedEdges(ctx, "dup")
	if err != nil {
		t.Fatalf("TypedEdges after touch: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("edge rows changed after a comment-only _test.go touch:\nbefore=%+v\nafter=%+v", before, after)
	}
}

// TestIngest_TypedEdges_RespectsIgnores is F2 regression: a Go file under a
// .fathomignore'd directory — with package AND func names colliding with a
// live indexed package — must contribute no typed edges. Pre-fix,
// packages.Load("./...") swept it in and its edges landed keyed onto the
// LIVE symbols' keys.
func TestIngest_TypedEdges_RespectsIgnores(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod":        "module example.com/collide\n\ngo 1.21\n",
		".fathomignore": "workspace/\n",
		"collide.go":    "package collide\n\nfunc Run() int { return helper() }\n\nfunc helper() int { return 1 }\n",
		// Same package name, same func names, DIFFERENT directory — the
		// collision shape the critic probed under workspace/tmp.
		"workspace/tmp/shadow.go": "package collide\n\nfunc Run() int { return helper() }\n\nfunc helper() int { return 2 }\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})

	report, err := pipe.Ingest(ctx, core.ServiceRef{ID: "collide", Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if report.TypedPackages == 0 {
		t.Fatalf("typed pass did not run: %+v", report)
	}

	edges, err := st.TypedEdges(ctx, "collide")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) == 0 {
		t.Fatal("no typed edges at all — the allowed root file should still produce Run->helper")
	}
	for _, e := range edges {
		if strings.HasPrefix(e.FromFile, "workspace/") {
			t.Errorf("edge from an ignored file survived the connector-scope filter: %+v", e)
		}
	}
	// Exactly one Run->helper edge: the root package's. The ignored twin
	// would have doubled it (or replaced it under another pkg ID).
	n := 0
	for _, e := range edges {
		if e.FromSymbol == "collide::collide.Run" && e.ToSymbol == "collide::collide.helper" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Run->helper edge count = %d, want exactly 1 (the allowed root package's)", n)
	}
}

// TestIngest_TypedEdgeCounters is F5 regression: the production call site
// must surface the typed-pass counters in fusion.Report — a daemon whose
// typed pass always fails must be distinguishable from a non-Go service.
func TestIngest_TypedEdgeCounters(t *testing.T) {
	t.Run("happy_path_counters", func(t *testing.T) {
		dir := writeGoModule(t, map[string]string{
			"go.mod": "module example.com/counters\n\ngo 1.21\n",
			"lib.go": "package counters\n\nfunc A() int { return B() }\n\nfunc B() int { return 1 }\n",
		})
		st := store.NewMemoryStore()
		pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
		report, err := pipe.Ingest(context.Background(), core.ServiceRef{ID: "counters", Path: dir})
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if report.TypedPackages == 0 {
			t.Errorf("Report.TypedPackages = 0, want > 0 for a Go service")
		}
		if report.TypedPackagesFailed != 0 {
			t.Errorf("Report.TypedPackagesFailed = %d, want 0 for a clean module", report.TypedPackagesFailed)
		}
		if report.TypedEdgesDirect == 0 {
			t.Errorf("Report.TypedEdgesDirect = 0, want > 0 (A->B)")
		}
	})

	t.Run("failure_counted", func(t *testing.T) {
		dir := writeGoModule(t, map[string]string{
			"go.mod":    "module example.com/broken\n\ngo 1.21\n",
			"broken.go": "package broken\n\nfunc Broken( {\n",
		})
		st := store.NewMemoryStore()
		pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
		report, err := pipe.Ingest(context.Background(), core.ServiceRef{ID: "broken", Path: dir})
		if err != nil {
			t.Fatalf("Ingest: %v (typed-pass failures must never fail the ingest)", err)
		}
		if report.TypedPackagesFailed == 0 {
			t.Errorf("Report.TypedPackagesFailed = 0, want > 0 for a syntax-broken module")
		}
	})
}

// TestIngest_DisableTypedEdges is the F4 switch: with
// Config.DisableTypedEdges the pass never runs — no typed edges, zero
// counters — the knob that makes a TRUE syntax-only baseline measurable
// (and degraded environments explicit).
func TestIngest_DisableTypedEdges(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod": "module example.com/disabled\n\ngo 1.21\n",
		"lib.go": "package disabled\n\nfunc A() int { return B() }\n\nfunc B() int { return 1 }\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{DisableTypedEdges: true})

	report, err := pipe.Ingest(ctx, core.ServiceRef{ID: "disabled", Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if report.TypedPackages != 0 || report.TypedPackagesFailed != 0 ||
		report.TypedEdgesDirect != 0 || report.TypedEdgesIfaceDispatch != 0 || report.TypedEdgesUnresolved != 0 {
		t.Errorf("typed counters non-zero with DisableTypedEdges: %+v", report)
	}
	edges, err := st.TypedEdges(ctx, "disabled")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("TypedEdges = %d, want 0 with DisableTypedEdges", len(edges))
	}
	// Syntax extraction is unaffected: symbols still indexed.
	if report.Symbols == 0 {
		t.Errorf("Report.Symbols = 0 — DisableTypedEdges must not affect syntax extraction")
	}
}
