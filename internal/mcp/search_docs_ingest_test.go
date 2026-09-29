package mcp

// search_docs_ingest_test.go — KR-8's end to end: `search kind: docs`
// answers from the documents a real ingest indexed, not from a hand-filled
// index. Before the doc ingest had a production caller the same call read
// an empty index and answered "no doc hits" for every question.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestSearchDocsAnswersFromIngestedDocuments(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/docs\n\ngo 1.22\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("ADR.md", "# ADRs\n\n## ADR-001 — Storage\n\n**Decision.** Use one SQLite store with FTS5; no embeddings.\n")
	write("docs/design/tenancy.md", "# Tenancy\n\n## Budget\n\nThe tenancy budget caps every request, hash-gated at ingest.\n")

	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "docs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "docsvc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := NewServer(c)

	for _, tc := range []struct{ query, wantLoc string }{
		{"embeddings", "ADR.md:"},
		{"tenancy budget", "docs/design/tenancy.md:"},
		{"hash-gated", "docs/design/tenancy.md:"}, // punctuation in a plain question is not a syntax error
		{"ADR-001", "ADR.md:"},
	} {
		payload := callTool(t, srv, "search", map[string]any{"service": "docsvc", "pattern": tc.query, "kind": "docs"})
		hits, _ := payload["hits"].([]any)
		if len(hits) == 0 {
			t.Errorf("search kind:docs %q: no hits from the ingested documents: %v", tc.query, payload)
			continue
		}
		if loc, _ := hits[0].(map[string]any)["loc"].(string); !strings.HasPrefix(loc, tc.wantLoc) {
			t.Errorf("search kind:docs %q: first hit at %q, want %q...", tc.query, loc, tc.wantLoc)
		}
	}
}
