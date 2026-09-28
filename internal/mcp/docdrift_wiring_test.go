// Package mcp_test — TestStalenessReportDocLinkDrift pins KR-7's live
// wiring (docs/design/knowledge-retrieval.md §6): a backend implementing
// the optional docDriftBackend capability (pkg/client.Client.DocLinkDrift,
// backed by docindex.StalenessDocDrift) populates staleness_report's
// doc_link_drift key -- additive to, never replacing, SR-28's own
// pre-existing doc_drift key (internal/analysis/docdrift's unrelated
// symbol/path-existence scan), which TestStalenessReportDocDrift
// (new_tools_test.go) already pins on its own shape.
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/docindex"
	mcp "github.com/geoffmilleraz/fathom/internal/mcp"
)

// docDriftFakeBackend adds the optional docLinkDrift capability
// (structural: internal/mcp's docDriftBackend interface is unexported, so
// this satisfies it by having the same method signature, the same way
// pkg/client.Client does for real) on top of the package's existing
// extendedFakeBackend.
type docDriftFakeBackend struct {
	extendedFakeBackend
	drifted []docindex.DriftedDocLink
}

func (d *docDriftFakeBackend) DocLinkDrift(_ context.Context, _ string) ([]docindex.DriftedDocLink, error) {
	return d.drifted, nil
}

var _ mcp.QueryInterface = (*docDriftFakeBackend)(nil)

func TestStalenessReportDocLinkDrift(t *testing.T) {
	srv := mcp.NewServer(&docDriftFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		drifted: []docindex.DriftedDocLink{{
			Path: "docs/design/x.md", HeadingPath: "X", TargetKind: "path", Target: "a/b.go",
			Line: 5, LinkedHash: "h1", CurrentHash: "h2",
			ChangedCommit: "deadbeef", ChangedAt: "2026-09-27T00:00:00Z",
		}},
	})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 900, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	rows, ok := p["doc_link_drift"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("staleness_report: doc_link_drift missing or wrong shape: %v", p)
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["path"] != "docs/design/x.md" || row["target"] != "a/b.go" || row["changed_commit"] != "deadbeef" {
		t.Errorf("doc_link_drift[0] = %v, want path docs/design/x.md target a/b.go changed_commit deadbeef", rows[0])
	}
	// SR-28's own key must still be present -- KR-7's addition is
	// additive, never a replacement.
	if _, ok := p["doc_drift"]; !ok {
		t.Error("staleness_report: doc_drift (SR-28) key missing after KR-7's addition")
	}
}

// TestStalenessReportNoDocLinkDrift pins that a backend without the
// docDriftBackend capability gets no doc_link_drift key at all -- never an
// empty list standing in for "checked, found nothing" (I38).
func TestStalenessReportNoDocLinkDrift(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 901, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if _, ok := p["doc_link_drift"]; ok {
		t.Errorf("staleness_report: doc_link_drift present without a docDriftBackend: %v", p)
	}
}
