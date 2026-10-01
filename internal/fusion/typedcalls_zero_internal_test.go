package fusion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestTypedPassZeroPackagesDisclosed (EW-39, O304): go/packages can answer
// err == nil with zero packages (a go list cut short, a pattern that matched
// nothing). Where the connector found loadable Go files that is a disclosed
// failure (TypedFailed with a reason), never a silent "ok"; a repository
// with no loadable Go files stays silent.
func TestTypedPassZeroPackagesDisclosed(t *testing.T) {
	old := typedPackagesLoad
	typedPackagesLoad = func(*packages.Config, ...string) ([]*packages.Package, error) { return nil, nil }
	t.Cleanup(func() { typedPackagesLoad = old })

	abs, err := filepath.Abs(filepath.Join("testdata", "typedfixture"))
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 2})
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "zeropkgs", Path: abs})
	if err != nil {
		t.Fatalf("Ingest must not fail when only the typed pass does: %v", err)
	}
	if !rep.TypedFailed || !strings.Contains(rep.TypedFailReason, "0 packages") {
		t.Fatalf("zero packages with Go files discovered must be a disclosed failure naming it: failed=%v reason=%q", rep.TypedFailed, rep.TypedFailReason)
	}
	// The failed pass records no success, so the next ingest retries it.
	if p.passStateOK(context.Background(), "zeropkgs", goPassStateKey) {
		t.Fatalf("a zero-package load must not record the pass as done")
	}

	// No loadable Go files: testdata, underscore dirs and nested modules are
	// not part of ./..., so a zero load there is silent.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	quiet := map[string]bool{"testdata/a.go": true, "_x/b.go": true, "sub/c.go": true, "vendor/d.go": true}
	if e := zeroPackagesDisclosed(root, quiet, 0); e != nil {
		t.Fatalf("no loadable Go files must stay silent: %v", e)
	}
	if e := zeroPackagesDisclosed(root, map[string]bool{"main.go": true}, 0); e == nil {
		t.Fatalf("a root Go file with zero packages loaded must be disclosed")
	}
	if e := zeroPackagesDisclosed(root, map[string]bool{"main.go": true}, 2); e != nil {
		t.Fatalf("a non-empty load is not a failure: %v", e)
	}
}
