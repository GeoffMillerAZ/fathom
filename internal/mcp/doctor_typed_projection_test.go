package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

type doctorMetadataBackend struct {
	typedIngestBackend
	reads int
}

func (b *doctorMetadataBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	b.reads++
	return nil, nil
}

func TestDoctorTypedProjectionDoesNotReadCorpus(t *testing.T) {
	b := &doctorMetadataBackend{typedIngestBackend: typedIngestBackend{status: fusion.TypedEdgeStatus{Packages: 140}, known: true}}
	srv := mcp.NewServer(b)
	resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "staleness_report", "arguments": map[string]any{"service": "fixture", "section": "typed_edges"}}})
	p := extToolPayload(t, resp)
	if b.reads != 0 {
		t.Fatalf("typed metadata loaded corpus %d time(s)", b.reads)
	}
	te, ok := p["typed_edges"].(map[string]any)
	if !ok || te["known"] != true || te["packages"] != float64(140) {
		t.Fatalf("typed status = %v", p)
	}
}
