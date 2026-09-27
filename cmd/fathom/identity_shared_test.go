package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// RT-133 hygiene (§11.3): every hook identity lookup goes through one
// cached resolver backed by GET /identity. SessionStart's stamp and its
// primer share one round trip (the primer no longer lists every service),
// a later process within the TTL makes none, and post-commit's
// registered-root guard reads the same identity instead of list_services.
func TestIdentityLookupsShareOneCachedRoundTrip(t *testing.T) {
	repo, _ := filepath.EvalSymlinks(t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	t.Setenv("FATHOM_IDENT_NO_CACHE", "")
	prevDir := identCacheDir
	identCacheDir = t.TempDir()
	t.Cleanup(func() { identCacheDir = prevDir })
	name := filepath.Base(repo)
	svcs := map[string]svcInfo{strings.ToLower(name): {symbols: 42, langs: "go", path: repo}}
	h := handleIdentity(func(*http.Request) (map[string]svcInfo, error) { return svcs, nil })
	var identityCalls, listCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			_, _ = w.Write([]byte("ok"))
		case r.URL.Path == "/identity":
			identityCalls.Add(1)
			h.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/rpc"):
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"list_services"`) {
				listCalls.Add(1)
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"` + strings.ToLower(name) + `","symbol_count":42,"path":"` + repo + `"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// SessionStart: the stamp's lookup, then the primer.
	if got, _, ok := indexedServiceFor(srv.Client(), srv.URL, repo); !ok || got != strings.ToLower(name) {
		t.Fatalf("stamp lookup: %q %v", got, ok)
	}
	line := buildSessionContext(srv.URL, repo)
	if !strings.Contains(line, "fathom code-intelligence is live") || !strings.Contains(line, strings.ToLower(name)) {
		t.Fatalf("primer: %s", line)
	}
	if identityCalls.Load() != 1 || listCalls.Load() != 0 {
		t.Fatalf("SessionStart made %d /identity and %d list_services round trips, want 1 and 0", identityCalls.Load(), listCalls.Load())
	}
	// A later hook process within the TTL: the on-disk cache answers.
	resetIdentityMemo()
	if _, _, ok := indexedServiceFor(srv.Client(), srv.URL, repo); !ok || identityCalls.Load() != 1 {
		t.Fatalf("a cached identity was fetched again: %d /identity", identityCalls.Load())
	}
	// post-commit's registered-root guard: the same identity, no list.
	resetIdentityMemo()
	svc, path, known := postCommitIdentity(srv.Client(), srv.URL, repo, "")
	if svc != strings.ToLower(name) || !known || path != repo || listCalls.Load() != 0 {
		t.Fatalf("post-commit identity: %q %q %v (list_services %d)", svc, path, known, listCalls.Load())
	}
}
