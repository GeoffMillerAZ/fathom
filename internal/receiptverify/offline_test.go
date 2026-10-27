package receiptverify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/goenv"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// StageUncachedDepModule writes a module requiring a dependency no module
// cache holds (example.invalid/dep v1.0.0) and commits it; it returns the
// repository root and the commit.
func stageUncachedDepModule(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module receiptverify.test/offline\n\ngo 1.21\n\nrequire example.invalid/dep v1.0.0\n",
		"go.sum": "example.invalid/dep v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			"example.invalid/dep v1.0.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n",
		"main.go": "package main\n\nimport \"example.invalid/dep\"\n\nfunc run() { dep.Do() }\n\nfunc main() { run() }\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, initGitRepo(t, root)
}

// TestVerifyOfflineNamesMissingModule is EVX-6 (I56): the toolchain rung
// runs go/packages offline — GOPROXY=off and GOFLAGS=-mod=readonly,
// appended after the operator's own environment — so a module missing from
// the cache is never downloaded: Verify cannot run (cmd/fathom's exit 2)
// and names the module. The operator's GOPROXY here is a recording proxy;
// it must see no request, and the empty module cache must stay without a
// module zip.
func TestVerifyOfflineNamesMissingModule(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(proxy.Close)
	cache := t.TempDir()
	t.Setenv("GOMODCACHE", cache)
	t.Setenv("GOPROXY", proxy.URL)
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "")

	root, commit := stageUncachedDepModule(t)
	r := core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "run", IndexCommit: commit, Callers: []string{"main"}}
	_, err := receiptverify.Verify(context.Background(), root, r)
	if err == nil {
		t.Fatal("Verify ran over a checkout whose dependency is not in the module cache; want cannot-run naming the module")
	}
	if !strings.Contains(err.Error(), "example.invalid/dep@v1.0.0") || !strings.Contains(err.Error(), "never downloads") {
		t.Errorf("error %q does not name the missing module and the offline rule", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the toolchain rung asked the module proxy %d time(s); it must never download (GOPROXY=off)", n)
	}
	zips, _ := filepath.Glob(filepath.Join(cache, "cache", "download", "example.invalid", "dep", "@v", "*.zip"))
	if len(zips) != 0 {
		t.Errorf("a module zip landed in the cache: %v", zips)
	}

	// The environment itself: the operator's settings first, the offline
	// rule appended after them so it wins (exec reads the last entry).
	env := receiptverify.OfflineGoEnv()
	got := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			got[k] = v
		}
	}
	want := map[string]string{
		"GOWORK": "off", "GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": ",",
		"GOFLAGS": "-mod=readonly", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("OfflineGoEnv sets %s=%q, want %q after os.Environ()", k, got[k], v)
		}
	}
	if last := env[len(env)-1]; last != "GOTOOLCHAIN=local" {
		t.Errorf("OfflineGoEnv's last entry is %q, want GOTOOLCHAIN=local", last)
	}
}

// TestVerifyOfflineUnderGOPRIVATE: a GOPRIVATE pattern is GONOPROXY's
// default, and a module it matches goes straight to its version control
// past GOPROXY=off. The offline environment is goenv's (GONOPROXY=","), so
// with GOPRIVATE naming the dependency the go command still refuses with
// its own lookup refusal, never dials the module's host, and Verify names
// the module.
func TestVerifyOfflineUnderGOPRIVATE(t *testing.T) {
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOPRIVATE", "example.invalid")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "")

	root, commit := stageUncachedDepModule(t)
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = root
	cmd.Env = receiptverify.OfflineGoEnv()
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), goenv.LookupRefused) {
		t.Fatalf("go mod download under OfflineGoEnv with GOPRIVATE set: err %v, output %q; want the go command's %q", err, out, goenv.LookupRefused)
	}

	r := core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "run", IndexCommit: commit, Callers: []string{"main"}}
	_, err = receiptverify.Verify(context.Background(), root, r)
	if err == nil || !strings.Contains(err.Error(), "example.invalid/dep@v1.0.0") || !strings.Contains(err.Error(), "never downloads") {
		t.Errorf("Verify with GOPRIVATE set: error %v; want cannot-run naming example.invalid/dep@v1.0.0 and the offline rule", err)
	}
}
