package main

import (
	"net/http"
	"net/http/httptest"
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
