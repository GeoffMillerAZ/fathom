package main

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// metricsServer serves a minimal /metrics text-exposition response with
// the given heap-in-use bytes and uptime seconds — the two fields
// checkDaemonFootprint reads.
func metricsServer(t *testing.T, heapInuseBytes int64, uptimeSeconds float64) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "fathom_uptime_seconds %.3f\n", uptimeSeconds)
		fmt.Fprintf(w, "fathom_requests_total 42\n")
		fmt.Fprintf(w, "process_uptime_seconds %.3f\n", uptimeSeconds)
		fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", heapInuseBytes)
		fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", heapInuseBytes*2)
		fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", heapInuseBytes*3)
		fmt.Fprintf(w, "go_goroutines 12\n")
		fmt.Fprintf(w, "fathom_graph_memo_entries 3\n")
		fmt.Fprintf(w, "fathom_open_stores 1\n")
		fmt.Fprintf(w, "fathom_autorefresh_total 5\n")
		fmt.Fprintf(w, "fathom_typed_pass_total 2\n")
	}))
	t.Cleanup(ts.Close)
	return ts
}

// RT-49 (§13.3): daemon_footprint judges phys_footprint (+ children) from
// the resources projection against the budget; an unreachable daemon is a
// WARN naming the ring's last sample and its age — never a SKIP into
// silence — and no remedy says "restart the daemon".
func TestDoctorDaemonBudgetNeverSkipsWhenDown(t *testing.T) {
	type tc struct {
		name   string
		setup  func(t *testing.T, f *resourcesFixture)
		status doctorStatus
		reason []string
		remedy []string
	}
	for _, c := range []tc{
		{"down with a ring: WARN with the last sample and its age", func(t *testing.T, f *resourcesFixture) {
			f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		}, doctorWarn, []string{"did not answer", "last ring sample 10m0s old", "phys_footprint 1.2 GiB (+100 MiB children)", "budget 4 GiB"}, []string{"fathom resources"}},
		{"down with no ring: WARN, no instrument named", func(t *testing.T, f *resourcesFixture) {}, doctorWarn,
			[]string{"did not answer", "no resource ring", "no footprint instrument"}, []string{"fathom resources"}},
		{"live, within budget: PASS", func(t *testing.T, f *resourcesFixture) {
			f.live = &StatuszReport{Tenancy: &governorSnapshot{Source: "live", Level: "normal", Mode: "normal", Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true,
				Footprint: 900 * tenancy.MiB, Children: 0, Pressure: "normal", SampleAt: f.now.Format(time.RFC3339), InFlightByClass: map[string]int{}}}
		}, doctorPass, []string{"phys_footprint 900 MiB", "budget 4 GiB", "level normal"}, nil},
		{"live, shed: WARN naming the level, class and top tool", func(t *testing.T, f *resourcesFixture) {
			f.live = &StatuszReport{Tenancy: &governorSnapshot{Source: "live", Level: "shed", Mode: "normal", Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true,
				Footprint: 3300 * tenancy.MiB, Children: 0, Pressure: "normal", SampleAt: f.now.Format(time.RFC3339), InFlightByClass: map[string]int{"tree": 1},
				Oldest: &inflightView{Class: "tree", Tool: "obligations", AgeSec: 12}}}
		}, doctorWarn, []string{"level shed", "phys_footprint 3.2 GiB"}, []string{"shed", "obligations (tree)", "fathom resources"}},
		{"live, footprint unknown: WARN, ladder off disclosed", func(t *testing.T, f *resourcesFixture) {
			f.live = &StatuszReport{Tenancy: &governorSnapshot{Source: "live", Level: "normal", LadderOff: true, Budget: 4 * tenancy.GiB, Footprint: -1, Children: -1, Pressure: "unknown", InFlightByClass: map[string]int{}}}
		}, doctorWarn, []string{"footprint unknown", "ladder off"}, []string{"fathom resources"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newResourcesFixture(t)
			c.setup(t, f)
			rep := buildResources(f.env(), "", "http://127.0.0.1:7949", 24*time.Hour)
			got := checkDaemonFootprint(rep, nil)
			if got.Name != "daemon_footprint" {
				t.Fatalf("row name %q", got.Name)
			}
			if got.Status == doctorSkipped {
				t.Fatalf("daemon_footprint SKIPped: %+v", got)
			}
			if got.Status != c.status {
				t.Fatalf("status %s, want %s: %+v", got.Status, c.status, got)
			}
			for _, w := range c.reason {
				if !strings.Contains(got.Reason, w) {
					t.Errorf("reason lacks %q: %s", w, got.Reason)
				}
			}
			for _, w := range c.remedy {
				if !strings.Contains(got.Remedy, w) {
					t.Errorf("remedy lacks %q: %s", w, got.Remedy)
				}
			}
			if strings.Contains(strings.ToLower(got.Remedy+got.Reason), "restart") {
				t.Errorf("the restart remedy is gone (§13.3): %+v", got)
			}
		})
	}
	// The GC hint, when /metrics carries it, is named beside the footprint.
	f := newResourcesFixture(t)
	f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
	got := checkDaemonFootprint(buildResources(f.env(), "", "", 24*time.Hour), map[string]float64{"go_memory_limit_bytes": float64(math.MaxInt64)})
	if !strings.Contains(got.Reason, "GC hint off") {
		t.Errorf("reason lacks the GC hint: %s", got.Reason)
	}
}

func TestFetchMetricsText_ParsesKeyValueLines(t *testing.T) {
	ts := metricsServer(t, 100<<20, 7200)
	m, err := fetchMetricsText(&http.Client{}, ts.URL)
	if err != nil {
		t.Fatalf("fetchMetricsText: %v", err)
	}
	if m["go_memstats_heap_inuse_bytes"] != float64(100<<20) {
		t.Errorf("heap_inuse = %v, want %v", m["go_memstats_heap_inuse_bytes"], float64(100<<20))
	}
	if m["process_uptime_seconds"] != 7200 {
		t.Errorf("uptime = %v, want 7200", m["process_uptime_seconds"])
	}
	if m["fathom_autorefresh_total"] != 5 {
		t.Errorf("autorefresh_total = %v, want 5", m["fathom_autorefresh_total"])
	}
}
