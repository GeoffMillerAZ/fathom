package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// RT-133 (design §11.3): GET /identity resolves a directory to its indexed
// service from the daemon's registry — the hooks' lookup no longer lists
// every service — a daemon without the route falls back to list_services,
// and SubagentStart reuses its parent's SessionStart resolution from the
// session stamp with zero RPCs.
func TestIdentityEndpointAndSubagentReuse(t *testing.T) {
	repo, _ := filepath.EvalSymlinks(t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	sub := filepath.Join(repo, "deploy")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	other, _ := filepath.EvalSymlinks(t.TempDir())
	svcs := map[string]svcInfo{
		"fathom": {symbols: 42, langs: "go", path: repo},
		"deploy": {symbols: 3, path: other}, // a basename collision, recorded elsewhere
	}
	var regErr error
	h := handleIdentity(func(*http.Request) (map[string]svcInfo, error) { return svcs, regErr })
	get := func(q string) (*httptest.ResponseRecorder, identityResult) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/identity?"+q, nil))
		var out identityResult
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}
	// The corpus root wins over a basename that names another service.
	if rec, got := get("path=" + sub + "&path=" + repo); rec.Code != 200 || got.Service != "fathom" || got.MatchedBy != identityByRoot || got.Root != repo || got.Symbols != 42 {
		t.Fatalf("root match: %d %+v", rec.Code, got)
	}
	// Only a basename matches: served, and named as such.
	lone := filepath.Join(t.TempDir(), "deploy")
	if _, got := get("path=" + lone); got.Service != "deploy" || got.MatchedBy != identityByBasename {
		t.Fatalf("basename match: %+v", got)
	}
	if _, got := get("path=" + t.TempDir()); got.Service != "" || got.Schema != 1 {
		t.Fatalf("no match is an empty answer with the schema mark: %+v", got)
	}
	if rec, _ := get("path=relative/dir"); rec.Code != http.StatusBadRequest {
		t.Fatalf("a relative path: %d", rec.Code)
	}
	regErr = errors.New("registry refused")
	if rec, _ := get("path=" + repo); rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("an unreadable registry is a named 503: %d", rec.Code)
	}
	regErr = nil

	// Over HTTP: the cached resolver's miss is one /identity, no list_services.
	var identityCalls, rpcCalls, allCalls atomic.Int64
	mux := http.NewServeMux()
	mux.Handle("/identity", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { identityCalls.Add(1); h.ServeHTTP(w, r) }))
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		rpcCalls.Add(1)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"fathom","symbol_count":42,"path":"` + repo + `"}]}}`))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { allCalls.Add(1); mux.ServeHTTP(w, r) }))
	defer srv.Close()
	cand, ok, err := resolveIndexedCandidateErr(srv.Client(), srv.URL, sub)
	if err != nil || !ok || cand.name != "fathom" || cand.root != repo {
		t.Fatalf("resolve via /identity: %+v %v %v", cand, ok, err)
	}
	if identityCalls.Load() != 1 || rpcCalls.Load() != 0 {
		t.Fatalf("one /identity and no list_services: identity %d rpc %d", identityCalls.Load(), rpcCalls.Load())
	}
	if name, info, ok := indexedServiceFor(srv.Client(), srv.URL, sub); !ok || name != "fathom" || info.symbols != 42 || rpcCalls.Load() != 0 {
		t.Fatalf("indexedServiceFor via /identity: %q %+v %v (rpc %d)", name, info, ok, rpcCalls.Load())
	}

	// A daemon that predates /identity: the full list, as before.
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rpc") {
			rpcCalls.Add(1)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"fathom","symbol_count":42,"path":"` + repo + `"}]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer old.Close()
	before := rpcCalls.Load()
	if cand, ok, err := resolveIndexedCandidateErr(old.Client(), old.URL, sub); err != nil || !ok || cand.name != "fathom" || rpcCalls.Load() != before+1 {
		t.Fatalf("fallback to list_services: %+v %v %v", cand, ok, err)
	}

	// SubagentStart reuses the parent's stamped resolution: zero requests.
	t.Setenv("FATHOM_SESSION_STAMPS", t.TempDir())
	writeSessionStamp(sessionStamp{SchemaVersion: 1, SessionID: "parent", At: "now", Cwd: sub,
		Identity: &identityResult{Schema: 1, Service: "fathom", Symbols: 42, Langs: "go", Path: repo}})
	n := allCalls.Load()
	name, info, ok := subagentIdentity(srv.URL, sub, "parent")
	if !ok || name != "fathom" || info.symbols != 42 || allCalls.Load() != n {
		t.Fatalf("subagent reuse: %q %+v %v, %d request(s)", name, info, ok, allCalls.Load()-n)
	}
	// Another directory, or no stamp: the resolver runs.
	if _, _, ok := subagentIdentity(srv.URL, repo, "parent"); !ok || allCalls.Load() == n {
		t.Fatal("a subagent in another directory resolves for itself")
	}
}
