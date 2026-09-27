package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-124 (design §13.3): /metrics exports every tenancy family — level,
// budget, pressure, swap, volume free, in-flight, oldest in-flight, queue,
// rejected, abandoned, bytes read and written, cache bytes, starts,
// released memory, GC CPU, SQLite memory, children footprint and
// quarantined services.
func TestMetricsTenancyFamilies(t *testing.T) {
	h, gate := gatedTestHandler(t, tenancy.ModeEnforce)
	tk, ref := gate.Admit(context.Background(), tenancy.Request{Tool: "search", Class: tenancy.ClassIndex, Origin: tenancy.OriginHook})
	if ref != nil {
		t.Fatalf("admit: %v", ref)
	}
	defer tk.Release(tenancy.OutcomeOK)
	r := newGovRig(t)
	if err := os.MkdirAll(filepath.Dir(r.ledger), 0o700); err != nil {
		t.Fatal(err)
	}
	ledger := `{"schema_version":1,"row":"start","at":"2026-09-26T20:00:00Z","pid":1}
{"schema_version":1,"row":"exit","at":"2026-09-26T20:10:00Z","pid":1,"code":70,"reason":"over_budget"}
{"schema_version":1,"row":"start","at":"2026-09-26T20:10:10Z","pid":2}
{"schema_version":1,"row":"exit","at":"2026-09-26T20:20:00Z","pid":2,"code":70,"reason":"over_budget"}
{"schema_version":1,"row":"start","at":"2026-09-26T20:20:10Z","pid":3}
`
	if err := os.WriteFile(r.ledger, []byte(ledger), 0o600); err != nil {
		t.Fatal(err)
	}
	r.ticks(2)
	prev := activeGovernor.Swap(r.g)
	t.Cleanup(func() { activeGovernor.Store(prev); tenancy.ResetToolCountersForTest() })
	_, done := r.g.inflight.Begin(context.Background(), tenancy.ClassTree, "references", "svc", "agent")
	defer done()
	r.now = r.now.Add(5 * 1e9)
	tenancy.RecordAbandoned("obligations", tenancy.AbandonGovernor)
	tenancy.RecordBytesRead("search", 4096)
	tenancy.RegisterCacheBytes("graph", func() int64 { return 1234 })

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"fathom_tenancy_level ", "fathom_budget_bytes ", "fathom_host_pressure_level ", "fathom_swap_used_bytes ",
		`fathom_volume_free_bytes{vol="store"} `, `fathom_volume_free_bytes{vol="tmp"} `, `fathom_volume_free_bytes{vol="swap"} `,
		`fathom_inflight{class="index",origin="hook"} 1`, `fathom_oldest_inflight_seconds{class="tree"} 5.000`, `fathom_queue{class=`,
		`fathom_abandoned_total{tool="obligations",cause="governor"} 1`, `fathom_bytes_read_total{tool="search"} 4096`,
		`fathom_cache_bytes{group="graph"} 1234`,
		`fathom_lifecycle_starts_total{reason="first"} 1`, `fathom_lifecycle_starts_total{reason="over_budget"} 2`,
		"fathom_memory_released_bytes ", "fathom_gc_cpu_fraction ", "fathom_sqlite_memory_used_bytes",
		"fathom_children_footprint_bytes ", "fathom_quarantined_services ", "fathom_test_callers_rejected_total ",
		"# HELP go_memstats_heap_sys_bytes virtual",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics lacks %q", want)
		}
	}
	// fathom_inflight{class,origin} and fathom_bytes_written_total{subsystem}
	// render only what was counted: an admitted call and a sampled rusage.
	// The rejected family appears once a refusal is counted; admission's
	// own tests cover its labels. Here: it is wired into the same body.
	if !strings.Contains(body, "fathom_admission_mode{") {
		t.Errorf("/metrics lacks the admission block")
	}
	if t.Failed() {
		t.Logf("body:\n%s", body)
	}
}
