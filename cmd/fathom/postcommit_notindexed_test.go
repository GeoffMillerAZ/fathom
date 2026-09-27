package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/refresh"
)

// TestPostCommitNotIndexedNamesOnboarding is RT-111 (design §12.3): the
// hook queues a refresh of a service the daemon already indexes and never
// onboards one (the ingest tool with a path did, before). A commit in a
// repository the daemon does not index says exactly that and how to
// onboard it once through the daemon — not "when the daemon is back".
func TestPostCommitNotIndexedNamesOnboarding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(refreshRequestResponse{Service: "newrepo", State: string(refresh.StateRefused), Error: "service is not indexed by this daemon"})
	}))
	defer srv.Close()
	_, err := ingestViaDaemon(srv.URL, "newrepo", "/src/newrepo", 5*time.Second)
	if err == nil {
		t.Fatal("an unindexed service's refresh must fail")
	}
	line := postCommitFailureLine("newrepo", "/src/newrepo", err)
	for _, want := range []string{"not indexed", "never onboards", "ingest MCP tool", `"path": "/src/newrepo"`} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "When the daemon is back") {
		t.Errorf("line %q blames an absent daemon for an unindexed service", line)
	}
}
