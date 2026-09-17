package mcp_test

// ingest_typed_disclosure_test.go — wave P3 defect 1 / O82
// (.fathom/observations.md): the MCP `ingest` tool result must carry the
// same typed-edge disclosure the ingest CLI/--json now do — a caller must
// never need a SEPARATE staleness_report/doctor call to learn a Go tree's
// typed pass produced nothing to trust. This is the exact gap the
// 2026-09-16 incident hit: the daemon answered {"ok":true} for a push clone
// whose typed-edge enrichment produced ZERO typed edges, and nothing about
// that ingest response itself said so.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// typedIngestBackend is a minimal QueryInterface whose IngestService
// succeeds and whose TypedEdgeStatus answers a fixed, per-test status —
// simulating what the daemon's Pipeline already records after a real
// enrichTypedEdges pass (BP-35).
type typedIngestBackend struct {
	extendedFakeBackend
	status fusion.TypedEdgeStatus
	known  bool
}

func (b *typedIngestBackend) TypedEdgeStatus(string) (fusion.TypedEdgeStatus, bool) {
	return b.status, b.known
}

var _ mcp.QueryInterface = (*typedIngestBackend)(nil)

func TestIngestTool_TypedEdgesDisclosedOnSuccess(t *testing.T) {
	backend := &typedIngestBackend{
		status: fusion.TypedEdgeStatus{PackagesSeen: 4, Packages: 4, EdgesTotal: 9},
		known:  true,
	}
	srv := mcp.NewServer(backend)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1350, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["ok"] != true {
		t.Fatalf("ok = %v, want true for a clean typed pass", p["ok"])
	}
	te, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("no typed_edges block in ingest result: %v", p)
	}
	if got, _ := te["packages_seen"].(float64); int(got) != 4 {
		t.Errorf("typed_edges.packages_seen = %v, want 4", te["packages_seen"])
	}
	if got, _ := te["edges"].(float64); int(got) != 9 {
		t.Errorf("typed_edges.edges = %v, want 9", te["edges"])
	}
	if te["failed"] != false {
		t.Errorf("typed_edges.failed = %v, want false", te["failed"])
	}
}

// TestIngestTool_TypedEdgesFailureMarksOkFalse pins the ok:false-grade
// disclosure (wave P3 / O82): a Go tree with a failed/zero-processed typed
// pass must flip the top-level ok to false — the syntax index is kept
// (IngestService still ran and wrote), but a caller checking only `ok`
// (the common case) must not read this as a clean ingest.
func TestIngestTool_TypedEdgesFailureMarksOkFalse(t *testing.T) {
	backend := &typedIngestBackend{
		status: fusion.TypedEdgeStatus{
			PackagesSeen: 3, Failed: true,
			Reason: "3 package(s) seen but 0 processed — every package excluded by the connector scope filter",
		},
		known: true,
	}
	srv := mcp.NewServer(backend)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1351, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	te, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("no typed_edges block: %v", p)
	}
	if te["failed"] != true {
		t.Fatalf("typed_edges.failed = %v, want true", te["failed"])
	}
	if te["error"] == "" || te["error"] == nil {
		t.Error("typed_edges.error is empty (I24: a named reason)")
	}
	if p["ok"] != false {
		t.Errorf("ok = %v, want false (ok:false-grade disclosure) — this is exactly the {\"ok\":true} the 2026-09-16 incident rode", p["ok"])
	}
}

// TestIngestTool_TypedEdgesOmittedWhenUnknown: a backend with no typed-edge
// capability (or a service this daemon has never typed-pass'd, e.g. a
// non-Go tree) must omit typed_edges entirely — never a fabricated clean
// pass.
func TestIngestTool_TypedEdgesOmittedWhenUnknown(t *testing.T) {
	backend := &typedIngestBackend{known: false}
	srv := mcp.NewServer(backend)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1352, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["ok"] != true {
		t.Fatalf("ok = %v, want true when the typed-edge outcome is simply unknown (non-Go tree)", p["ok"])
	}
	if _, ok := p["typed_edges"]; ok {
		t.Errorf("typed_edges present with known=false: %v", p["typed_edges"])
	}
}
