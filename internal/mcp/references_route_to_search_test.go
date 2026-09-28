// Package mcp_test — TDD test for SR-40 (F-02, design §5): references
// routes a free-text query to search instead of refusing it.
package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// writeReferencesRouteFixture lays out a small tree with a free-text line
// no identifier or dotted-path query would ever match.
func writeReferencesRouteFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	full := filepath.Join(root, "notes.md")
	if err := os.WriteFile(full, []byte("# Notes\n\nTODO fix the flaky login test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestReferencesRoutesFreeTextToSearch pins SR-40: a query that is not one
// of references' three accepted symbol forms (bare identifier, dotted
// path, declared regex) is answered by search instead of refused with an
// error — carrying routed_from so a caller can tell the two tools' answers
// apart.
func TestReferencesRoutesFreeTextToSearch(t *testing.T) {
	root := writeReferencesRouteFixture(t)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	payload := callTool(t, srv, "references", map[string]any{
		"query":    "fix the flaky login test",
		"services": []any{"svc"},
	})

	if got := payload["routed_from"]; got != "references" {
		t.Fatalf("routed_from = %v, want %q (payload: %v)", got, "references", payload)
	}
	hits, _ := payload["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %v", len(hits), payload)
	}
	h := hits[0].(map[string]any)
	if got := h["loc"]; got != "notes.md:3" {
		t.Errorf("hits[0].loc = %v, want notes.md:3", got)
	}
}

// TestReferencesRegexDeclaredSkipsSearchRouting pins that a declared regex
// is unaffected by the SR-40 routing: "fix the" would be refused as
// free-text without regex: true, but declaring it a regex takes the
// ORDINARY references path (definitions/occurrences), never search's
// routed_from envelope.
func TestReferencesRegexDeclaredSkipsSearchRouting(t *testing.T) {
	root := writeReferencesRouteFixture(t)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	payload := callTool(t, srv, "references", map[string]any{
		"query":    "fix the",
		"services": []any{"svc"},
		"regex":    true,
	})
	if _, has := payload["routed_from"]; has {
		t.Fatalf("a declared regex must not be routed to search; payload: %v", payload)
	}
	if _, has := payload["definitions"]; !has {
		t.Fatalf("a declared regex must take the ordinary references path (definitions field missing); payload: %v", payload)
	}
}
