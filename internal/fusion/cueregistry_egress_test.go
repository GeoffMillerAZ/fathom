package fusion_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// A CUE module that declares an external dependency must ingest without any
// network attempt, and the dependency must read as a counted, named failure,
// whatever CUE_REGISTRY says: unset, empty, an inline empty registry, or a
// registry that names a reachable host (the counting proxy itself).
func TestEnrichCUEEdges_ExternalDependencyNeverFetches(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   bool
		value func(proxyHost string) string
	}{
		{name: "unset"},
		{name: "empty", set: true, value: func(string) string { return "" }},
		{name: "inline-empty", set: true, value: func(string) string { return "inline:{}" }},
		{name: "named-host", set: true, value: func(h string) string { return h + "+insecure" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testExternalDependencyNeverFetches(t, tc.set, tc.value)
		})
	}
}

func testExternalDependencyNeverFetches(t *testing.T, set bool, value func(string) string) {
	var hits atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "egress refused", http.StatusForbidden)
	}))
	defer proxy.Close()
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(k, proxy.URL)
	}
	// Pin the default transport to the counting proxy so a regression cannot
	// go direct through net/http's once-cached ProxyFromEnvironment.
	prev := http.DefaultTransport
	pinned := prev.(*http.Transport).Clone()
	pu, _ := url.Parse(proxy.URL)
	pinned.Proxy = http.ProxyURL(pu)
	http.DefaultTransport = pinned
	defer func() { http.DefaultTransport = prev }()
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	if set {
		t.Setenv("CUE_REGISTRY", value(pu.Host))
	} else {
		t.Setenv("CUE_REGISTRY", "")
		os.Unsetenv("CUE_REGISTRY")
	}
	t.Setenv("CUE_CACHE_DIR", t.TempDir())

	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cue.mod/module.cue", "module: \"example.org/egress@v0\"\nlanguage: version: \"v0.16.1\"\ndeps: \"example.org/remote@v0\": v: \"v0.0.1\"\n")
	write("consumer/main.cue", "package consumer\nimport \"example.org/remote/pkg\"\nout: pkg.#Name\n")

	report, err := fusion.EnrichCUEEdges(context.Background(), store.NewMemoryStore(), "egress", root, nil)
	if err != nil {
		t.Fatalf("EnrichCUEEdges: %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("typed CUE pass attempted %d network request(s)", n)
	}
	if report.PackagesFailed == 0 || len(report.FailedPackages) == 0 {
		t.Fatalf("external import not counted/named as a failure: %+v", report)
	}
	if !strings.Contains(report.FailedPackages[0].Err, "example.org/remote") {
		t.Fatalf("failure does not name the dependency: %q", report.FailedPackages[0].Err)
	}
}
