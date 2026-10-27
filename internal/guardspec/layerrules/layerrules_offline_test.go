package layerrules

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestBuildIndexRunsGoOffline (ADR-083): the import graph a generated
// layer-rules test loads runs the go command offline: GOPROXY=off,
// GOSUMDB=off, GOTELEMETRY=off, no inherited -mod=mod, GOTOOLCHAIN=local
// last, GOWORK=off.
func TestBuildIndexRunsGoOffline(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user go_network opt-out
	t.Setenv("GOPROXY", "https://proxy.example.test")
	t.Setenv("GOFLAGS", "-mod=mod")
	env := loadEnv()
	got := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			got[k] = v
		}
	}
	for key, want := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTELEMETRY": "off", "GOTOOLCHAIN": "local", "GOWORK": "off", "GOFLAGS": ""} {
		if got[key] != want {
			t.Errorf("%s = %q, want %q", key, got[key], want)
		}
	}
	if env[len(env)-1] != "GOTOOLCHAIN=local" {
		t.Errorf("env ends %q, want GOTOOLCHAIN=local last", env[len(env)-1])
	}
}

// TestBuildIndexColdModuleCacheIsNamed (ADR-083, I38): a dependency the
// module cache lacks is never fetched (a loopback GOPROXY fails the test on
// any request) and never read as a smaller, clean import graph: BuildIndex
// fails naming the module and the remedy.
func TestBuildIndexColdModuleCacheIsNamed(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "tripwire", http.StatusGone)
	}))
	defer srv.Close()
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Setenv("GOPROXY", srv.URL)
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "mod"))
	t.Setenv("GOFLAGS", "")
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module coldmod\n\ngo 1.22\n\nrequire github.com/google/uuid v1.6.0\n",
		"go.sum": "github.com/google/uuid v1.6.0 h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=\n" +
			"github.com/google/uuid v1.6.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=\n",
		"a/a.go": "package a\n\nimport \"github.com/google/uuid\"\n\nfunc F() string { return uuid.NewString() }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := BuildIndex(dir)
	if n := hits.Load(); n > 0 {
		t.Fatalf("BuildIndex reached the module proxy %d time(s) (ADR-083)", n)
	}
	if err == nil {
		t.Fatal("BuildIndex over a cold module cache returned a clean index: a layer rule would pass over a partial graph (I38)")
	}
	for _, want := range []string{"github.com/google/uuid", "run go mod download in " + dir} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
