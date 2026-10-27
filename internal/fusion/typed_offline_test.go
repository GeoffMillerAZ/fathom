package fusion_test

// ADR-083: the typed pass never reaches the module network. A cloned
// repository whose dependencies are not in the module cache fails the pass
// with a named reason (the missing module and the remedy) instead of a
// download, the inherited -mod=mod never rewrites its go.mod or go.sum, and
// the next ingest retries the pass, so the remedy works without touching a
// Go file. A loopback GOPROXY fails the test on any request: the tripwire
// for an automatic go command that would otherwise have fetched.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// coldModuleGoMod requires one public module whose go.sum lines the fixture
// carries, so only the module cache is missing.
const (
	coldModuleGoMod = "module example.test/coldcache\n\ngo 1.22\n\nrequire github.com/google/uuid v1.6.0\n"
	coldModuleGoSum = "github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=\n" +
		"github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=\n"
	coldModuleSrc = "package coldcache\n\nimport \"github.com/google/uuid\"\n\nfunc ID() string { return uuid.NewString() }\n\nfunc Call() string { return ID() }\n"
)

// proxyTripwire points GOPROXY at a loopback listener and returns a check
// that fails the test when any request reached it.
func proxyTripwire(t *testing.T) func(label string) {
	t.Helper()
	var mu sync.Mutex
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits = append(hits, r.Method+" "+r.URL.Path)
		mu.Unlock()
		http.Error(w, "fathom test tripwire: no module download", http.StatusGone)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("GOPROXY", srv.URL)
	return func(label string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(hits) > 0 {
			t.Fatalf("%s: an automatic go command reached the module proxy (ADR-083): %v", label, hits)
		}
	}
}

func TestTypedPassColdModuleCacheNamesTheModuleAndNeverFetches(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	tripped := proxyTripwire(t)
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "mod")) // empty: the dependency is not cached
	t.Setenv("GOFLAGS", "-mod=mod")                           // an inherited -mod=mod must not rewrite go.mod

	dir := t.TempDir()
	for name, body := range map[string]string{"go.mod": coldModuleGoMod, "go.sum": coldModuleGoSum, "cold.go": coldModuleSrc} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "coldcache", Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	tripped("first ingest")
	status, known := p.TypedEdgeStatus("coldcache")
	if !known || !status.Failed || !rep.TypedFailed {
		t.Fatalf("a cold module cache read as a healthy typed pass (status %+v, report failed=%v) — I38", status, rep.TypedFailed)
	}
	if rep.TypedPackagesFailed == 0 {
		t.Fatalf("the failed package is not counted: %+v", rep)
	}
	for _, want := range []string{"github.com/google/uuid@v1.6.0", "run go mod download in " + dir} {
		if !strings.Contains(status.Reason, want) || !strings.Contains(rep.TypedFailReason, want) {
			t.Fatalf("the named reason lacks %q:\n status: %s\n report: %s", want, status.Reason, rep.TypedFailReason)
		}
	}
	for name, want := range map[string]string{"go.mod": coldModuleGoMod, "go.sum": coldModuleGoSum} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s was rewritten by an automatic go command (an inherited -mod=mod survived):\n%s", name, got)
		}
	}

	// Nothing Go changed, yet the next ingest runs the pass again: a
	// missing module is fixed outside the tree (go mod download), so the
	// partial state that holds the pass back is not recorded for it.
	rep2, err := p.Ingest(ctx, core.ServiceRef{ID: "coldcache", Path: dir})
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	tripped("second ingest")
	if rep2.TypedSkipped != "" {
		t.Fatalf("the second ingest skipped the typed pass (%q): the remedy would never be picked up", rep2.TypedSkipped)
	}
	if !rep2.TypedFailed {
		t.Fatalf("second ingest: the module is still missing, the pass must still fail: %+v", rep2)
	}
}
