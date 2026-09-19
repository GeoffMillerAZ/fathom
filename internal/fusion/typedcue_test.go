package fusion_test

// SU-10 (docs/plans/substrate.md §B, ADR-041 decision 5): CUE typed-edge
// enrichment via cue/load.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cueFixtureDir returns the absolute path to the small, self-contained CUE
// module (its own cue.mod) used to exercise EnrichCUEEdges's resolution
// rules: same-file definition reference, cross-file same-package reference,
// cross-package (imported) reference — both from a definition and from a
// plain field — and one deliberately unresolved cross-package reference.
func cueFixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "cuefixture"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "cue.mod", "module.cue")); err != nil {
		t.Fatalf("cue fixture module missing cue.mod/module.cue at %s: %v", abs, err)
	}
	return abs
}

func cueKey(t *testing.T, service, dir, pkg, name string) string {
	t.Helper()
	// Mirrors cueQualifiedPackage/core.QualifiedPackage: a file under `dir`
	// declaring `package pkg` keys as service::dir/pkg.name (dir != pkg
	// collapses to service::pkg.name only when dir is "." or "").
	qual := pkg
	if dir != "." && dir != "" {
		qual = dir + "/" + pkg
	}
	return core.SymbolKey(service, qual, name)
}

// TestEnrichCUEEdges_Fixture pins the CUE typed-edge set over cuefixture
// (called directly, mirroring TestEnrichTypedEdges_Resolution's shape) AND
// — the SU-10 acceptance contract — ingests the SAME fixture end to end
// through the real pipeline (extraction + the CUE typed pass wired beside
// the Go one) and asserts every RESOLVED edge's From/To key exists as an
// extracted symbol, joining cueextract.go's own SymbolKey convention.
func TestEnrichCUEEdges_Fixture(t *testing.T) {
	dir := cueFixtureDir(t)
	ctx := context.Background()
	const svc = "cuefixture"

	t.Run("direct", func(t *testing.T) {
		st := store.NewMemoryStore()
		report, err := fusion.EnrichCUEEdges(ctx, st, svc, dir, nil)
		if err != nil {
			t.Fatalf("EnrichCUEEdges: %v", err)
		}
		if report.PackagesFailed != 0 {
			t.Errorf("PackagesFailed = %d, want 0 (fixture module must load clean)", report.PackagesFailed)
		}
		if report.PackagesLoaded < 2 {
			t.Fatalf("PackagesLoaded = %d, want at least 2 (pkgone, pkgtwo)", report.PackagesLoaded)
		}

		edges, err := st.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges: %v", err)
		}

		base := cueKey(t, svc, "pkgone", "pkgone", "#Base")
		wantDirect := map[string]string{
			cueKey(t, svc, "pkgone", "pkgone", "#Derived"): base, // same-file
			cueKey(t, svc, "pkgone", "pkgone", "#Extra"):   base, // cross-file, same package
			cueKey(t, svc, "pkgtwo", "pkgtwo", "#Widget"):  base, // cross-package, from a definition
			cueKey(t, svc, "pkgtwo", "pkgtwo", "profile"):  base, // cross-package, from a plain field
		}
		gotDirect := map[string]string{}
		var gotUnresolvedFrom []string
		for _, e := range edges {
			switch e.Kind {
			case core.EdgeDirect:
				gotDirect[e.FromSymbol] = e.ToSymbol
			case core.EdgeUnresolved:
				gotUnresolvedFrom = append(gotUnresolvedFrom, e.FromSymbol)
			default:
				t.Errorf("unexpected edge kind %q on %+v", e.Kind, e)
			}
		}
		if len(gotDirect) != len(wantDirect) {
			t.Errorf("direct edge count = %d, want %d (%+v)", len(gotDirect), len(wantDirect), gotDirect)
		}
		for from, to := range wantDirect {
			if got, ok := gotDirect[from]; !ok || got != to {
				t.Errorf("edge %s -> %s missing or wrong (got %q, ok=%v)", from, to, got, ok)
			}
		}

		wantGhost := cueKey(t, svc, "pkgtwo", "pkgtwo", "#Ghost")
		if len(gotUnresolvedFrom) != 1 || gotUnresolvedFrom[0] != wantGhost {
			t.Errorf("unresolved edges = %v, want exactly one from %s", gotUnresolvedFrom, wantGhost)
		}
	})

	t.Run("end_to_end", func(t *testing.T) {
		c, err := client.NewConstantClient(client.ModeLocal, "")
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		defer c.Close()

		if _, err := c.Ingest(ctx, client.ServiceRef{ID: svc, Path: dir}); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		edges, err := c.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges: %v", err)
		}
		nodes, err := c.ListAllSymbols(ctx, svc)
		if err != nil {
			t.Fatalf("ListAllSymbols: %v", err)
		}
		known := map[string]bool{}
		for _, n := range nodes {
			known[n.Key()] = true
		}

		resolved := 0
		for _, e := range edges {
			if e.Kind != core.EdgeDirect {
				continue
			}
			resolved++
			if !known[e.FromSymbol] {
				t.Errorf("resolved edge From %s: not an extracted symbol", e.FromSymbol)
			}
			if !known[e.ToSymbol] {
				t.Errorf("resolved edge To %s: not an extracted symbol", e.ToSymbol)
			}
		}
		if resolved == 0 {
			t.Fatal("end-to-end ingest produced zero resolved CUE edges")
		}
	})
}

// TestEnrichCUEEdges_SelfRepo runs EnrichCUEEdges over this repository's own
// CUE corpus (.fathom/, schema/, internal/**/*_schema.cue) — the SU-10
// calibration subject named in docs/languages.md. It only asserts a
// non-zero edge count and zero panics: this repo's real .cue files are the
// integration surface, pinned edge tables belong to the fixture. Skipped
// when this checkout has no cue.mod (a source archive without VCS
// metadata, or a build sandbox that trimmed it).
func TestEnrichCUEEdges_SelfRepo(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "cue.mod", "module.cue")); err != nil {
		t.Skip("this checkout has no cue.mod/module.cue — skipping the self-repo CUE calibration run")
	}

	ctx := context.Background()
	st := store.NewMemoryStore()
	report, err := fusion.EnrichCUEEdges(ctx, st, "fathom-self-cue", repoRoot, nil)
	if err != nil {
		t.Fatalf("EnrichCUEEdges over this repo: %v", err)
	}
	if report.PackagesFailed > 0 {
		t.Logf("note: %d/%d CUE package(s) failed to load (non-fatal, recorded)", report.PackagesFailed, report.PackagesTotal)
	}
	edges, err := st.TypedEdges(ctx, "fathom-self-cue")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	var direct int
	for _, e := range edges {
		if e.Kind == core.EdgeDirect {
			direct++
		}
	}
	if direct == 0 {
		t.Fatal("EnrichCUEEdges over this repo produced zero direct edges — schema/semantic's own cross-file #FindingClass/#Producer references should resolve to at least one")
	}
	t.Logf("this repo: %d CUE package(s) processed, %d direct edge(s), %d total edges", report.PackagesLoaded, direct, len(edges))
}
