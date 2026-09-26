package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestFetchServices_InProcessForOwnDaemon: inside serve-http, a registry
// lookup aimed at the daemon's own URL never makes an HTTP round trip (it
// timed out under load and rendered project pages as "store unreadable",
// and wrote a call-log row for a page render); any other URL still goes
// over HTTP.
func TestFetchServices_InProcessForOwnDaemon(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"services":[{"service":"other","symbol_count":3,"languages":["go"]}]}}`))
	}))
	defer ts.Close()

	prevF, prevURL := inProcessServices, inProcessSelfURL
	t.Cleanup(func() { inProcessServices, inProcessSelfURL = prevF, prevURL })
	inProcessSelfURL = ts.URL + "/"
	inProcessServices = func(ctx context.Context) (map[string]svcInfo, error) {
		return svcInfoFromSummaries([]mcp.ServiceSummary{
			{Service: "Fathom", SymbolCount: 7, Languages: []string{"cue", "go"}, Path: "/repo"},
			{Service: "../escape", SymbolCount: 1},
		}), nil
	}

	got, err := fetchServices(&http.Client{}, ts.URL)
	if err != nil {
		t.Fatalf("fetchServices(own daemon): %v", err)
	}
	if hits != 0 {
		t.Fatalf("own-daemon lookup made %d HTTP call(s); want 0", hits)
	}
	info, ok := got["fathom"]
	if !ok || info.symbols != 7 || info.langs != "cue/go" || info.path != "/repo" {
		t.Fatalf("own-daemon registry = %+v, want fathom{7 cue/go /repo}", got)
	}
	if _, bad := got["../escape"]; bad {
		t.Fatalf("a malformed service id passed through: %+v", got)
	}

	other := httptest.NewServer(ts.Config.Handler)
	defer other.Close()
	got2, err := fetchServices(&http.Client{}, other.URL)
	if err != nil {
		t.Fatalf("fetchServices(other daemon): %v", err)
	}
	if hits != 1 || got2["other"].symbols != 3 {
		t.Fatalf("another daemon's URL must go over HTTP: hits=%d got=%+v", hits, got2)
	}
}
