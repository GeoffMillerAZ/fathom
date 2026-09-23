package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

func TestCheckDaemonFootprint_DaemonDown(t *testing.T) {
	c := checkDaemonFootprint(&http.Client{}, "http://127.0.0.1:1", false)
	if c.Status != doctorSkipped {
		t.Fatalf("status = %s, want skipped", c.Status)
	}
}

func TestCheckDaemonFootprint_BelowBudget_Passes(t *testing.T) {
	ts := metricsServer(t, 64<<20, 3600) // 64 MiB, 1h uptime
	c := checkDaemonFootprint(&http.Client{}, ts.URL, true)
	if c.Status != doctorPass {
		t.Fatalf("status = %s (%s), want pass", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "heap in-use") {
		t.Errorf("reason must name heap in-use: %q", c.Reason)
	}
}

func TestCheckDaemonFootprint_AboveBudget_Warns(t *testing.T) {
	ts := metricsServer(t, 12<<30, 33898) // 12 GiB, O108's own incident numbers
	c := checkDaemonFootprint(&http.Client{}, ts.URL, true)
	if c.Status != doctorWarn {
		t.Fatalf("status = %s (%s), want warn (never fail — doctor's own honesty discipline)", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "O108") {
		t.Errorf("reason must cite O108: %q", c.Reason)
	}
	if c.Remedy == "" {
		t.Error("a warn row must carry a remedy")
	}
}

func TestCheckDaemonFootprint_OldDaemon_MissingFields_Skips(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "fathom_uptime_seconds 10\nfathom_requests_total 1\n")
	}))
	defer ts.Close()
	c := checkDaemonFootprint(&http.Client{}, ts.URL, true)
	if c.Status != doctorSkipped {
		t.Fatalf("status = %s, want skipped", c.Status)
	}
	if !strings.Contains(c.Reason, "predating O108") {
		t.Errorf("reason must name the version gap: %q", c.Reason)
	}
}

func TestCheckDaemonFootprint_Unreachable_Skips(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	c := checkDaemonFootprint(&http.Client{}, ts.URL, true)
	if c.Status != doctorSkipped {
		t.Fatalf("status = %s, want skipped", c.Status)
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
