package fusion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestTypedEdgeEpoch_FieldReferencesForceReResolution (K17, wave lf2,
// LB-F343): the field references are a new kind the typed pass stores, so a
// package whose rows were written before them — fingerprinted under epoch 4,
// call edges only — must be re-resolved, never served from the cache with
// no field reference at all. The fingerprints are salted with "4", the true
// prior epoch, so the test fails if the bump reverts.
func TestTypedEdgeEpoch_FieldReferencesForceReResolution(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":   "module example.com/epochfields\n\ngo 1.21\n",
		"c/c.go":   "package c\n\ntype Config struct{ Timeout int }\n\nfunc Read(c *Config) int { return c.Timeout }\n",
		"c/c2.go":  "package c\n\nfunc Use() int { return Read(&Config{}) }\n",
		"c/doc.go": "// Package c is the fixture.\npackage c\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: root, Tests: true, Env: TypedEdgeEnv(),
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles}, "./...")
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("load: %v (%d packages)", err, len(pkgs))
	}
	// The pre-K17 cache: every package's rows call edges only, under an
	// epoch-4 fingerprint.
	for _, pkg := range pkgs {
		old, err := packageContentHashAt(root, pkg, "4")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.PutTypedEdges(ctx, "ef", pkg.ID, []core.Edge{{FromService: "ef", FromFile: "c/c2.go",
			FromSymbol: "ef::c/c.Use", ToSymbol: "ef::c/c.Read", Kind: core.EdgeDirect}}); err != nil {
			t.Fatal(err)
		}
		if err := st.SetTypedEdgePackageFingerprint(ctx, "ef", pkg.ID, old); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := EnrichTypedEdges(ctx, st, "ef", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.PackagesCached != 0 {
		t.Fatalf("%d package(s) served from an epoch-4 cache: TypedEdgeEpoch was not bumped for the field references", rep.PackagesCached)
	}
	edges, err := st.TypedEdges(ctx, "ef")
	if err != nil {
		t.Fatal(err)
	}
	read := false
	for _, e := range edges {
		read = read || (e.Kind == core.EdgeFieldRead && e.FromSymbol == "ef::c/c.Read" && e.ToSymbol == "ef::c/Config.Config.Timeout")
	}
	if !read {
		t.Fatalf("no field read of Config.Timeout after re-resolution: %v", edges)
	}
}
