package main

// ingest_typed_disclosure_test.go — wave P3 defect 1 / O82
// (.fathom/observations.md): the CLI TEXT path (`fathom ingest`, no --json)
// must disclose the typed-edge pass's outcome as plainly as --json does —
// a Go tree whose typed pass produced nothing to trust must print it and
// exit non-zero, never a silent "symbols: N" with no typed-edges line at
// all (the pre-O82 "non-zero packages/failures only" print condition).

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// goProxyTripwire (ADR-083, A5) points GOPROXY at a loopback listener that
// answers every request with an error and returns a check that fails the
// test when any request reached it: the runtime tripwire for a go command
// the binary runs on its own that would otherwise download a module. It
// also inherits GOFLAGS=-mod=mod, which the binary must drop.
func goProxyTripwire(t *testing.T) func() {
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
	t.Setenv("GOFLAGS", "-mod=mod")
	return func() {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(hits) > 0 {
			t.Fatalf("an ingest's go command reached the module proxy (ADR-083): %v", hits)
		}
	}
}

func TestCmdIngest_TypedEdgesDisclosedInText(t *testing.T) {
	tripped := goProxyTripwire(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/typedtextprobe\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Hello() { helper() }\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "typedtextprobe", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "typed-edges:") {
		t.Errorf("a Go-source ingest must print a typed-edges line, got:\n%s", stdout.String())
	}
	tripped()
}

// TestCmdIngest_ColdModuleCacheNamesTheRemedy (ADR-083): `fathom ingest` of
// a module whose dependency the module cache lacks never downloads it (the
// tripwire), never rewrites go.mod or go.sum under an inherited -mod=mod,
// and fails the typed pass naming the module and `go mod download`.
func TestCmdIngest_ColdModuleCacheNamesTheRemedy(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	tripped := goProxyTripwire(t)
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "mod"))
	root := t.TempDir()
	gomod := "module example.com/coldcli\n\ngo 1.22\n\nrequire github.com/google/uuid v1.6.0\n"
	gosum := "github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=\n" +
		"github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=\n"
	for name, body := range map[string]string{
		"go.mod":  gomod,
		"go.sum":  gosum,
		"main.go": "package main\n\nimport \"github.com/google/uuid\"\n\nfunc main() { println(uuid.NewString()) }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "coldcli", "--path", root, "--store", store}, &stdout, &stderr)
	tripped()
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (the typed pass failed): stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String() + stderr.String()
	for _, want := range []string{"github.com/google/uuid@v1.6.0", "run go mod download in "} {
		if !strings.Contains(out, want) {
			t.Errorf("the ingest output does not name %q:\n%s", want, out)
		}
	}
	for name, want := range map[string]string{"go.mod": gomod, "go.sum": gosum} {
		if got, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(got) != want {
			t.Errorf("%s was rewritten (an inherited -mod=mod survived):\n%s", name, got)
		}
	}
}

// TestCmdIngest_TypedEdgesFailureExitsNonZero: a Go tree whose typed pass
// fails must exit 1 in text mode too — the ok:false-grade disclosure is not
// a --json-only behavior.
func TestCmdIngest_TypedEdgesFailureExitsNonZero(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/typedtextbroken\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package broken\n\nfunc Broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "typedtextbroken", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "typed-edge pass failed") {
		t.Errorf("stderr must name the typed-edge failure plainly, got: %s", stderr.String())
	}
}
