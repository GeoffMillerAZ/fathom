// Package mcp_test — TDD tests for `search kind: docs` (KR-5,
// docs/design/knowledge-retrieval.md §5): the MCP surface over KR-4's
// docindex.RankDocs, via search's own optional docsSearchBackend capability
// rather than a new tool (law 4).
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// docsSearchFakeBackend adds docsSearchBackend to fakeBackend for this
// file's tests: a canned RankedDoc slice (or error) SearchDocs returns,
// with the call's own args recorded so a test can assert what search
// passed through.
type docsSearchFakeBackend struct {
	fakeBackend
	ranked []docindex.RankedDoc
	err    error

	gotService string
	gotQuery   string
	gotLimit   int
}

func (d *docsSearchFakeBackend) SearchDocs(_ context.Context, service, query string, _ []string, limit int) ([]docindex.RankedDoc, error) {
	d.gotService, d.gotQuery, d.gotLimit = service, query, limit
	return d.ranked, d.err
}

// TestSearchDocsKind pins KR-5's happy path: `search kind: docs` reaches
// the backend's SearchDocs with the given service/pattern/limit and renders
// RankDocs' output into search's own hit envelope — loc (path:line-range),
// text (snippet), kind (doc unit kind), heading (heading path), and score
// (the fused RRF value) — engine named "docs-fts".
func TestSearchDocsKind(t *testing.T) {
	backend := &docsSearchFakeBackend{
		ranked: []docindex.RankedDoc{
			{
				UnitID: "u1", Path: "ADR.md", LineStart: 18, LineEnd: 23,
				HeadingPath: "Architecture Decision Records > ADR-001 — Three-layer architecture > Decision",
				Kind:        "adr-section", Snippet: "Strictly [separate] three layers", Score: 0.031,
			},
			{
				UnitID: "u2", Path: "docs/plans/archive/old.md", LineStart: 5, LineEnd: 5,
				HeadingPath: "Old plan > KR-9", Kind: "plan-row", Snippet: "an [archived] row",
				StaleByLocation: true, Score: 0.016,
			},
		},
	}
	srv := mcp.NewServer(backend)
	payload := callTool(t, srv, "search", map[string]any{
		"service": "svc", "pattern": "layers", "kind": "docs", "limit": 5,
	})

	if backend.gotService != "svc" || backend.gotQuery != "layers" || backend.gotLimit != 5 {
		t.Fatalf("SearchDocs got (service=%q query=%q limit=%d), want (svc, layers, 5)", backend.gotService, backend.gotQuery, backend.gotLimit)
	}
	if engine, _ := payload["engine"].(string); engine != "docs-fts" {
		t.Fatalf("engine = %v, want docs-fts", payload["engine"])
	}
	hits, _ := payload["hits"].([]any)
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2: %v", len(hits), payload)
	}
	first := hits[0].(map[string]any)
	if got := first["loc"]; got != "ADR.md:18-23" {
		t.Errorf("hits[0].loc = %v, want ADR.md:18-23", got)
	}
	if got := first["text"]; got != "Strictly [separate] three layers" {
		t.Errorf("hits[0].text = %v, want the snippet", got)
	}
	if got := first["kind"]; got != "adr-section" {
		t.Errorf("hits[0].kind = %v, want adr-section", got)
	}
	if got := first["heading"]; got != "Architecture Decision Records > ADR-001 — Three-layer architecture > Decision" {
		t.Errorf("hits[0].heading = %v", got)
	}
	if got, ok := first["score"].(float64); !ok || got != 0.031 {
		t.Errorf("hits[0].score = %v, want 0.031", first["score"])
	}
	if _, has := first["flags"]; has {
		t.Errorf("hits[0].flags = %v, want omitted (not stale_by_location)", first["flags"])
	}
	second := hits[1].(map[string]any)
	if got := second["loc"]; got != "docs/plans/archive/old.md:5" {
		t.Errorf("hits[1].loc = %v, want docs/plans/archive/old.md:5 (single line, no dash)", got)
	}
	if got := second["flags"]; got != "stale_by_location" {
		t.Errorf("hits[1].flags = %v, want stale_by_location", got)
	}
}

// TestSearchDocsKindNoHits pins the empty-result shape: no error, a
// summary that says so, and an empty (not nil-crashing) hits list.
func TestSearchDocsKindNoHits(t *testing.T) {
	srv := mcp.NewServer(&docsSearchFakeBackend{})
	payload := callTool(t, srv, "search", map[string]any{"service": "svc", "pattern": "zzznomatch", "kind": "docs"})
	if summary, _ := payload["summary"].(string); summary != "no doc hits" {
		t.Errorf("summary = %q, want \"no doc hits\"", summary)
	}
	hits, _ := payload["hits"].([]any)
	if len(hits) != 0 {
		t.Errorf("hits = %v, want none", hits)
	}
}

// TestSearchDocsKindRequiresService pins that `kind: docs` without a
// service (or with a root instead) is refused with a clear params error,
// never silently falling back to a filesystem grep over root.
func TestSearchDocsKindRequiresService(t *testing.T) {
	srv := mcp.NewServer(&docsSearchFakeBackend{})
	assertToolError(t, srv, "search", map[string]any{"pattern": "x", "kind": "docs"})
	assertToolError(t, srv, "search", map[string]any{"pattern": "x", "kind": "docs", "root": "/tmp"})
}

// TestSearchDocsKindUnknownKind pins that an unrecognized kind value is
// refused rather than silently treated as grep-shaped search.
func TestSearchDocsKindUnknownKind(t *testing.T) {
	srv := mcp.NewServer(&docsSearchFakeBackend{})
	assertToolError(t, srv, "search", map[string]any{"service": "svc", "pattern": "x", "kind": "bogus"})
}

// TestSearchDocsKindBackendWithoutCapability pins that a backend not
// implementing docsSearchBackend at all refuses `kind: docs` with a clear
// message instead of falling back to grep or panicking.
func TestSearchDocsKindBackendWithoutCapability(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	assertToolError(t, srv, "search", map[string]any{"service": "svc", "pattern": "x", "kind": "docs"})
}

// TestSearchDocsKindUnavailableStore pins that docindex.ErrDocIndexUnavailable
// (a MemoryStore-backed session, no SQLite doc index) surfaces as a clear,
// distinct refusal — never rendered the same as "no hits" (I38).
func TestSearchDocsKindUnavailableStore(t *testing.T) {
	srv := mcp.NewServer(&docsSearchFakeBackend{err: docindex.ErrDocIndexUnavailable})
	assertToolError(t, srv, "search", map[string]any{"service": "svc", "pattern": "x", "kind": "docs"})
}

// assertToolError calls name and fails the test unless the server's
// JSON-RPC envelope carries an error — the shared shape every "must be
// refused" case in this file checks.
func assertToolError(t *testing.T, srv *mcp.Server, name string, args map[string]any) {
	t.Helper()
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": name, "arguments": args}})
	if resp["error"] == nil {
		t.Fatalf("%s(%v): want a JSON-RPC error, got none: %v", name, args, resp)
	}
}
