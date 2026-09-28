// Package mcp_test — TDD tests for `search kind: docs` scoped to a `pkg:`
// key (KR-12, docs/design/knowledge-retrieval.md §7): the MCP surface over
// docindex.SearchPkgDocs, via search's own optional pkgDocsSearchBackend
// capability — no new tool (law 4), one optional argument.
package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// pkgDocsFakeBackend adds pkgDocsSearchBackend to fakeBackend: a canned
// PkgDocsResult (or error) with the call's own args recorded.
type pkgDocsFakeBackend struct {
	fakeBackend
	res docindex.PkgDocsResult
	err error

	called   bool
	gotKey   string
	gotQuery string
	gotLimit int
}

func (d *pkgDocsFakeBackend) SearchPkgDocs(_ context.Context, key, query string, limit int) (docindex.PkgDocsResult, error) {
	d.called, d.gotKey, d.gotQuery, d.gotLimit = true, key, query, limit
	return d.res, d.err
}

func toolErrorCode(t *testing.T, srv *mcp.Server, args map[string]any) (int, string) {
	t.Helper()
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "search", "arguments": args}})
	e, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("search(%v): want a JSON-RPC error, got %v", args, resp)
	}
	code, _ := e["code"].(float64)
	msg, _ := e["message"].(string)
	return int(code), msg
}

// TestSearchPkgDocsKind pins KR-12's happy path: `search kind: docs pkg: K`
// reaches the backend with the key, pattern and limit, needs no service (a
// dependency's docs are machine-wide, not one repository's), and renders
// the ranked units in search's own hit envelope with `pkg` naming the exact
// version answered from.
func TestSearchPkgDocsKind(t *testing.T) {
	const key = "pkg:golang/example.com/gizmo@v1.0.0"
	backend := &pkgDocsFakeBackend{res: docindex.PkgDocsResult{
		Key: key, State: docindex.PkgDocsAnswered,
		Hits: []docindex.RankedDoc{{
			UnitID: "u1", Path: "gizmo.go", LineStart: 6, LineEnd: 7,
			HeadingPath: "Frob", Kind: docindex.KindDepDoc, Snippet: "Frob returns a [widget]", Score: 0.03,
		}},
	}}
	srv := mcp.NewServer(backend)
	payload := callTool(t, srv, "search", map[string]any{"pattern": "widget", "kind": "docs", "pkg": key, "limit": 7})

	if !backend.called || backend.gotKey != key || backend.gotQuery != "widget" || backend.gotLimit != 7 {
		t.Fatalf("SearchPkgDocs got (%q, %q, %d), want (%s, widget, 7)", backend.gotKey, backend.gotQuery, backend.gotLimit, key)
	}
	if payload["engine"] != "docs-fts" || payload["pkg"] != key {
		t.Fatalf("engine/pkg = %v/%v, want docs-fts/%s", payload["engine"], payload["pkg"], key)
	}
	if _, has := payload["status"]; has {
		t.Fatalf("an answered search carries no status: %v", payload)
	}
	hits, _ := payload["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("hits = %v", payload["hits"])
	}
	h := hits[0].(map[string]any)
	if h["loc"] != "gizmo.go:6-7" || h["kind"] != docindex.KindDepDoc || h["heading"] != "Frob" || h["text"] != "Frob returns a [widget]" {
		t.Fatalf("hit = %v", h)
	}
	if s, _ := payload["summary"].(string); !strings.Contains(s, "1 doc hit") {
		t.Fatalf("summary = %q", s)
	}

	// service is accepted (and ignored: it does not scope a pkg search).
	backend.called = false
	callTool(t, srv, "search", map[string]any{"pattern": "widget", "kind": "docs", "pkg": key, "service": "svc"})
	if !backend.called {
		t.Fatal("a service alongside pkg must not stop the pkg search")
	}
}

// TestSearchPkgDocsNotIndexed pins that a version the index does not hold is
// answered by name — status, the reason, and the versions that are indexed —
// never as an empty hit list that looks like a real no-match (I38), and that
// what indexing cut rides along in partial.
func TestSearchPkgDocsNotIndexed(t *testing.T) {
	const key = "pkg:golang/example.com/gizmo@v3.0.0"
	backend := &pkgDocsFakeBackend{res: docindex.PkgDocsResult{
		Key: key, State: docindex.PkgDocsNotIndexed,
		Reason:          key + " is not indexed: dependency docs are never fetched (I56)",
		IndexedVersions: []string{"v1.0.0", "v2.0.0"},
	}}
	srv := mcp.NewServer(backend)
	payload := callTool(t, srv, "search", map[string]any{"pattern": "widget", "kind": "docs", "pkg": key})
	if payload["status"] != "not_indexed" {
		t.Fatalf("status = %v, want not_indexed: %v", payload["status"], payload)
	}
	if r, _ := payload["reason"].(string); !strings.Contains(r, "never fetched") {
		t.Fatalf("reason = %q", payload["reason"])
	}
	if s, _ := payload["summary"].(string); !strings.Contains(s, "not indexed") || !strings.Contains(s, "v1.0.0, v2.0.0") {
		t.Fatalf("summary = %q, want it to say not indexed and name the indexed versions", s)
	}
	if hits, _ := payload["hits"].([]any); hits == nil || len(hits) != 0 {
		t.Fatalf("hits = %v, want an empty list", payload["hits"])
	}

	backend.res = docindex.PkgDocsResult{Key: key, State: docindex.PkgDocsAnswered, Partial: "3 doc units elided at index time (over the package's byte budget)"}
	payload = callTool(t, srv, "search", map[string]any{"pattern": "widget", "kind": "docs", "pkg": key})
	if p, _ := payload["partial"].(string); !strings.Contains(p, "elided") {
		t.Fatalf("partial = %v, want the index-time cut named", payload["partial"])
	}
}

// TestSearchPkgDocsRefusals pins the argument contract: pkg needs kind docs
// (never a silent grep that ignores it), a root does not combine with it, a
// key the grammar rejects is a params error, a backend without the
// capability refuses by name, and a store with no doc index refuses rather
// than reporting no hits.
func TestSearchPkgDocsRefusals(t *testing.T) {
	const key = "pkg:golang/example.com/gizmo@v1.0.0"
	srv := mcp.NewServer(&pkgDocsFakeBackend{})

	if code, msg := toolErrorCode(t, srv, map[string]any{"pattern": "x", "service": "svc", "pkg": key}); code != -32602 || !strings.Contains(msg, "kind") {
		t.Fatalf("pkg without kind docs: code %d %q, want -32602 naming kind", code, msg)
	}
	if code, _ := toolErrorCode(t, srv, map[string]any{"pattern": "x", "kind": "docs", "root": "/tmp", "pkg": key}); code != -32602 {
		t.Fatalf("pkg with root: code %d, want -32602", code)
	}

	bad := &pkgDocsFakeBackend{err: docindex.ErrBadPkgKey}
	if code, _ := toolErrorCode(t, mcp.NewServer(bad), map[string]any{"pattern": "x", "kind": "docs", "pkg": "not-a-key"}); code != -32602 {
		t.Fatalf("a bad key: code %d, want -32602", code)
	}
	if code, msg := toolErrorCode(t, mcp.NewServer(&fakeBackend{}), map[string]any{"pattern": "x", "kind": "docs", "pkg": key}); code == 0 || !strings.Contains(msg, "not available") {
		t.Fatalf("a backend without the capability: code %d %q, want a refusal by name", code, msg)
	}
	unavailable := &pkgDocsFakeBackend{err: docindex.ErrDocIndexUnavailable}
	if _, msg := toolErrorCode(t, mcp.NewServer(unavailable), map[string]any{"pattern": "x", "kind": "docs", "pkg": key}); !strings.Contains(msg, "SQLite") {
		t.Fatalf("an unavailable store: %q, want it named", msg)
	}
}

// TestSearchPkgArgIsOptionalAndDescribed pins the schema side: `pkg` is an
// optional string on the existing search tool (no new tool), its
// description names the scope and the never-fetched guarantee.
func TestSearchPkgArgIsOptionalAndDescribed(t *testing.T) {
	srv := mcp.NewServer(&pkgDocsFakeBackend{})
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/list"})
	raw, _ := json.Marshal(resp)
	var env struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]struct {
						Type        string `json:"type"`
						Description string `json:"description"`
					} `json:"properties"`
					Required []string `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	for _, tool := range env.Result.Tools {
		if tool.Name != "search" {
			continue
		}
		p, ok := tool.InputSchema.Properties["pkg"]
		if !ok || p.Type != "string" {
			t.Fatalf("search has no string `pkg` property: %+v", tool.InputSchema.Properties)
		}
		for _, r := range tool.InputSchema.Required {
			if r == "pkg" {
				t.Fatal("pkg must be optional")
			}
		}
		if !strings.Contains(p.Description, "pkg:") || !strings.Contains(strings.ToLower(p.Description), "never fetched") {
			t.Fatalf("pkg description = %q, want the key shape and the never-fetched guarantee", p.Description)
		}
		return
	}
	t.Fatal("search tool not listed")
}
