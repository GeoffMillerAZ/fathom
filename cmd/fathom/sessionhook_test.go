package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeDaemon() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case "/rpc":
			// list_services result: one clean service + one malformed (must be hidden).
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[{\"service\":\"fathom\",\"symbol_count\":7093,\"languages\":[\"go\"]},{\"service\":\"../app-kit\",\"symbol_count\":1,\"languages\":[]}]}"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestSessionContext_IndexedService(t *testing.T) {
	srv := fakeDaemon()
	defer srv.Close()
	ctx := buildSessionContext(srv.URL, "fathom")
	for _, want := range []string{`service "fathom"`, "7093", "BEFORE any Grep", "find_symbol"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("indexed context missing %q:\n%s", want, ctx)
		}
	}
}

func TestSessionContext_UnindexedRepoHidesMalformed(t *testing.T) {
	srv := fakeDaemon()
	defer srv.Close()
	ctx := buildSessionContext(srv.URL, "some-random-repo")
	if !strings.Contains(ctx, "not an indexed service") {
		t.Fatalf("want not-indexed message, got:\n%s", ctx)
	}
	if strings.Contains(ctx, "app-kit") {
		t.Errorf("must not advertise the malformed '../app-kit' service:\n%s", ctx)
	}
	if !strings.Contains(ctx, "fathom") {
		t.Errorf("should list the clean 'fathom' service:\n%s", ctx)
	}
}

func TestSessionContext_DaemonDown(t *testing.T) {
	// Port 1 is never listening — the Get fails fast.
	ctx := buildSessionContext("http://127.0.0.1:1", "fathom")
	if !strings.Contains(ctx, "not responding") {
		t.Fatalf("want daemon-down message, got:\n%s", ctx)
	}
}

// TestSessionHookDegraded pins the O2 fix: a healthy daemon whose service
// enumeration FAILS must render a degraded notice — never the confident
// "not an indexed service" instruction that teaches the agent to abandon
// index-first discovery for a repo that IS indexed.
func TestSessionHookDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError) // list_services always fails
	}))
	defer srv.Close()

	got := buildSessionContext(srv.URL, t.TempDir())
	if strings.Contains(got, "not an indexed service") {
		t.Fatalf("enumeration failure rendered the false not-indexed instruction:\n%s", got)
	}
	if !strings.Contains(got, "enumeration") {
		t.Errorf("degraded branch should name the enumeration failure, got:\n%s", got)
	}
}

// TestSessionHookZeroServices pins O2 fix (d): a healthy daemon with a
// genuinely empty index must not render the dangling "Indexed services: ."
// artifact (strings.Join(nil) + trailing period).
func TestSessionHookZeroServices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[]}}`))
	}))
	defer srv.Close()

	got := buildSessionContext(srv.URL, t.TempDir())
	if strings.Contains(got, "Indexed services: .") {
		t.Fatalf("dangling empty-list artifact rendered:\n%s", got)
	}
	if !strings.Contains(got, "none yet") {
		t.Errorf("zero-service state should say so plainly, got:\n%s", got)
	}
}

// TestSessionTripwire pins IO-19 (vision 14 §7 tier-1): a drifted repo's
// session opens by naming the drift and the exact remedy.
func TestSessionTripwire(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{
  "hooks": {"PostToolUse": [{"matcher": "Edit|Write|MultiEdit", "hooks": [{"type": "command", "command": "/stale/bin/fathom edit-hook", "timeout": 5}]}]}
}`), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Write([]byte("ok"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"` + filepath.Base(repo) + `","symbol_count":1,"languages":["go"],"path":"` + repo + `"}]}}`))
	}))
	defer srv.Close()

	got := buildSessionContext(srv.URL, repo)
	if !strings.Contains(got, "wiring drift") {
		t.Fatalf("drifted repo's session line must name the drift:\n%s", got)
	}
	if !strings.Contains(got, "fleet reconcile") {
		t.Errorf("the remedy must be named:\n%s", got)
	}
}
