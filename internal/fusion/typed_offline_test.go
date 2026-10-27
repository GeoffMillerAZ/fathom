package fusion_test

// ADR-083: the typed pass never reaches the module network. A cloned
// repository whose dependencies are not in the module cache fails the pass
// with a named reason (the missing module and the remedy) instead of a
// download, the inherited -mod=mod never rewrites its go.mod or go.sum, and
// the pass's partial state names the missing modules, so the next ingest
// skips it (RT-104) until one of them appears in the module cache, and the
// remedy works without touching a Go file. A loopback GOPROXY fails the test on any request: the tripwire
// for an automatic go command that would otherwise have fetched.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/goenv/goenvtest"
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
	modCache := filepath.Join(t.TempDir(), "mod")
	t.Setenv("GOMODCACHE", modCache) // empty: the dependency is not cached
	t.Setenv("GOFLAGS", "-mod=mod")  // an inherited -mod=mod must not rewrite go.mod

	dir := writeColdModule(t)
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

	// F5 (RT-104 kept): nothing Go changed and the module is still
	// missing, so the next ingest skips the pass; the partial state names
	// the missing modules.
	rep2, err := p.Ingest(ctx, core.ServiceRef{ID: "coldcache", Path: dir})
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	tripped("second ingest")
	if rep2.TypedSkipped != fusion.SkippedPartialPassReason {
		t.Fatalf("second ingest: TypedSkipped = %q, want the partial-pass skip (RT-104: a missing module must not rerun the pass on every drift)", rep2.TypedSkipped)
	}

	// The module appears in the module cache (what `go mod download`
	// does), with no Go file changed: the next ingest runs the pass.
	at := filepath.Join(modCache, "cache", "download", "github.com", "google", "uuid", "@v")
	if err := os.MkdirAll(at, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(at, "v1.6.0.mod"), []byte("module github.com/google/uuid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep3, err := p.Ingest(ctx, core.ServiceRef{ID: "coldcache", Path: dir})
	if err != nil {
		t.Fatalf("third Ingest: %v", err)
	}
	tripped("third ingest")
	if rep3.TypedSkipped != "" {
		t.Fatalf("third ingest skipped the typed pass (%q) although a missing module appeared in the module cache: the remedy would never be picked up", rep3.TypedSkipped)
	}
}

// writeColdModule writes the cold-cache fixture into a new directory.
func writeColdModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"go.mod": coldModuleGoMod, "go.sum": coldModuleGoSum, "cold.go": coldModuleSrc} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestTypedPassPrivateModuleNeverReachesVCS (ADR-083): GONOPROXY defaults
// to GOPRIVATE, and a module matching it is fetched straight from its
// version control, past GOPROXY=off. With GOPRIVATE naming the fixture's
// dependency and a recording git first on PATH, the typed pass over a cold
// module cache never runs git and never creates the cache's vcs directory.
func TestTypedPassPrivateModuleNeverReachesVCS(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	tripped := proxyTripwire(t)
	modCache := filepath.Join(t.TempDir(), "mod")
	t.Setenv("GOMODCACHE", modCache)
	t.Setenv("GOPRIVATE", "github.com/google/uuid")
	t.Setenv("GONOPROXY", "")
	// The recording git logs what the go command runs to reach a module's
	// version control (git version, then ls-remote, init, fetch) and
	// hands every other call, the ingest's own history reads, to the real
	// git.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git on PATH")
	}
	bin := t.TempDir()
	gitLog := filepath.Join(t.TempDir(), "git.log")
	script := "#!/bin/sh\ncase \" $* \" in *' ls-remote '*|*' fetch '*|*' init '*|*' clone '*|' version ') echo \"$@\" >> '" + gitLog + "'; exit 1 ;; esac\nexec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := writeColdModule(t)
	p := fusion.New(connector.NewLocalConnector(), store.NewMemoryStore(), fusion.Config{Concurrency: 2})
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "private", Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	tripped("ingest")
	if log, err := os.ReadFile(gitLog); err == nil {
		t.Fatalf("the typed pass ran git for a GOPRIVATE module (GONOPROXY not closed, ADR-083):\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(modCache, "cache", "vcs")); err == nil {
		t.Fatal("the typed pass created the module cache's vcs directory: a GOPRIVATE module went to its version control")
	}
	if !rep.TypedFailed || !strings.Contains(rep.TypedFailReason, "github.com/google/uuid@v1.6.0") {
		t.Fatalf("the cold private module is not named: failed=%v reason=%q", rep.TypedFailed, rep.TypedFailReason)
	}
}

// TestTypedPassNeverRewritesGoSum (ADR-083): over a warm module cache and a
// go.sum one line short, the typed pass leaves go.sum as it was with
// -mod=mod in the process GOFLAGS or in the GOENV file, with the toolchain
// caps on (GOFLAGS -p) or lifted (FATHOM_TOOLCHAIN_PROCS=off, where GOFLAGS
// would otherwise be empty and the GOENV file's would apply).
func TestTypedPassNeverRewritesGoSum(t *testing.T) {
	w := goenvtest.WarmModule(t)
	file := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(file, []byte("GOFLAGS=-mod=mod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", file)
	t.Setenv("GOMODCACHE", w.ModCache)
	for i, c := range []struct{ procs, flags string }{{"off", ""}, {"off", "-mod=mod"}, {"", ""}, {"", "-mod=mod"}} {
		t.Setenv("FATHOM_TOOLCHAIN_PROCS", c.procs)
		t.Setenv("GOFLAGS", c.flags)
		w.Reset(t)
		p := fusion.New(connector.NewLocalConnector(), store.NewMemoryStore(), fusion.Config{Concurrency: 2})
		if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "warm" + strconv.Itoa(i), Path: w.Dir}); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		if rewritten, got := w.GoSumRewritten(t); rewritten {
			t.Errorf("FATHOM_TOOLCHAIN_PROCS=%q, process GOFLAGS %q, GOENV file -mod=mod: the typed pass rewrote go.sum:\n%s", c.procs, c.flags, got)
		}
	}
}
