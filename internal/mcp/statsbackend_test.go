// Package mcp_test — coverage for the optional statsBackend capability
// (ADR-021 §2 / TI-6): server_stats and the ingest tool both surface
// process-lifetime unclassified/oversize counters when the backend supports
// them, and omit the fields entirely when it doesn't (extendedFakeBackend,
// used throughout this package, deliberately does NOT implement it — proving
// the capability is additive, not a breaking QueryInterface change).
package mcp_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// statsFakeBackend embeds extendedFakeBackend and additionally implements
// IngestStats + ClassifiedZeroStats, simulating a backend whose
// IngestService call finds 2 new unclassified files and 1 new oversize file
// each time it's invoked.
type statsFakeBackend struct {
	extendedFakeBackend
	unclassified, oversize int64
	classifiedZero         map[string]int64
}

func (s *statsFakeBackend) IngestStats() (int64, int64) {
	return s.unclassified, s.oversize
}

func (s *statsFakeBackend) ClassifiedZeroStats() map[string]int64 {
	return s.classifiedZero
}

func (s *statsFakeBackend) IngestService(_ context.Context, _ string) error {
	s.unclassified += 2
	s.oversize++
	return nil
}

var _ mcp.QueryInterface = (*statsFakeBackend)(nil)

func newStatsSrv() (*mcp.Server, *statsFakeBackend) {
	backend := &statsFakeBackend{
		extendedFakeBackend: extendedFakeBackend{
			nodes:    extFixtureNodes(),
			services: []string{"svc"},
		},
	}
	return mcp.NewServer(backend), backend
}

func TestServerStats_WithStatsBackend_ReportsCumulativeCounters(t *testing.T) {
	srv, backend := newStatsSrv()
	backend.unclassified = 5
	backend.oversize = 2
	backend.classifiedZero = map[string]int64{"compose": 3}

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 900, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if got := p["unclassified_files"]; got != float64(5) {
		t.Errorf("unclassified_files = %v, want 5", got)
	}
	if got := p["oversize_files"]; got != float64(2) {
		t.Errorf("oversize_files = %v, want 2", got)
	}
	cz, ok := p["classified_zero_files"].(map[string]any)
	if !ok {
		t.Fatalf("classified_zero_files missing or wrong shape: %v", p)
	}
	if got := cz["compose"]; got != float64(3) {
		t.Errorf("classified_zero_files[compose] = %v, want 3", got)
	}
}

func TestServerStats_EmptyClassifiedZeroOmitted(t *testing.T) {
	srv, backend := newStatsSrv()
	backend.classifiedZero = map[string]int64{}

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 904, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if _, ok := p["classified_zero_files"]; ok {
		t.Errorf("empty classified_zero_files should be omitted, got %v", p)
	}
}

func TestServerStats_WithoutStatsBackend_OmitsCounters(t *testing.T) {
	srv := newExtSrv() // extendedFakeBackend — no IngestStats
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 901, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if _, ok := p["unclassified_files"]; ok {
		t.Errorf("unclassified_files should be omitted when the backend has no IngestStats, got %v", p)
	}
}

func TestIngestTool_ReportsPerCallDelta(t *testing.T) {
	srv, backend := newStatsSrv()
	backend.unclassified = 10
	backend.oversize = 3

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 902, Method: "tools/call",
		Params: map[string]any{
			"name":      "ingest",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["ok"] != true {
		t.Fatalf("ingest: expected ok=true, got %v", p)
	}
	// statsFakeBackend.IngestService adds +2 unclassified, +1 oversize —
	// the tool must report THIS call's delta, not the new cumulative total.
	if got := p["unclassified_files"]; got != float64(2) {
		t.Errorf("unclassified_files delta = %v, want 2", got)
	}
	if got := p["oversize_files"]; got != float64(1) {
		t.Errorf("oversize_files delta = %v, want 1", got)
	}
	if backend.unclassified != 12 || backend.oversize != 4 {
		t.Errorf("backend cumulative counters = (%d, %d), want (12, 4)", backend.unclassified, backend.oversize)
	}
}

// TestServerStats_StatsForwardThroughAutoRefreshWrapper: same O7 class as
// TestIngestTool_WithPath_ThroughWrapper — interface embedding does not
// forward methods outside QueryInterface's method set, so autoRefreshBackend
// needs an explicit IngestStats passthrough (internal/mcp/autorefresh.go) or
// wrapping a stats-capable backend in auto-refresh would silently hide the
// counters from server_stats/ingest.
func TestServerStats_StatsForwardThroughAutoRefreshWrapper(t *testing.T) {
	backend := &statsFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}},
		unclassified:        7,
		oversize:            1,
		classifiedZero:      map[string]int64{"k8s": 2},
	}
	srv := mcp.NewServer(mcp.NewAutoRefreshForTest(t, backend))

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 903, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if got := p["unclassified_files"]; got != float64(7) {
		t.Errorf("unclassified_files through auto-refresh wrapper = %v, want 7", got)
	}
	if got := p["oversize_files"]; got != float64(1) {
		t.Errorf("oversize_files through auto-refresh wrapper = %v, want 1", got)
	}
	cz, ok := p["classified_zero_files"].(map[string]any)
	if !ok {
		t.Fatalf("classified_zero_files missing through auto-refresh wrapper: %v", p)
	}
	if got := cz["k8s"]; got != float64(2) {
		t.Errorf("classified_zero_files[k8s] through wrapper = %v, want 2", got)
	}
}
