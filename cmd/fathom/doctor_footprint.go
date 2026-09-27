package main

// doctor_footprint.go — O108 (.fathom/observations.md): the always-on
// daemon's memory grew ~0.9 GB/hour with nothing in `fathom doctor` to
// flag it before it filled swap and killed a gated push's pre-push hook
// twice in one day. `daemon_footprint` reads the daemon's own /metrics
// (servehttp.go, O108's instrumentation half) and WARNs — never FAILs,
// per doctor's own honesty discipline (doctor.go's doc comment): a large
// heap is an operator signal to act on, not a wiring-broken verdict —
// above a stated budget, naming the remedy this observation records until
// the underlying growth is fixed (docs/plans/daemon-footprint.md).

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// daemonFootprintWarnBytes is the heap-in-use threshold above which
// daemon_footprint WARNs (O108's own 12 GB incident against a normal
// freshly-restarted daemon's tens-of-MB baseline — 2 GiB is generous
// headroom above any legitimate fleet-sized working set while still
// firing well before swap is in play).
const daemonFootprintWarnBytes = 2 << 30 // 2 GiB

// checkDaemonFootprint reads the daemon's /metrics and reports its
// go_memstats_heap_inuse_bytes and process_uptime_seconds. WARN (never
// FAIL) above daemonFootprintWarnBytes; SKIP with a reason when the
// daemon is unreachable or /metrics predates these fields (a daemon built
// before O108 shipped).
func checkDaemonFootprint(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
	const name = "daemon_footprint"
	remedy := "restart the daemon (task deploy, or the way your platform restarts the serve-http launchd job); O108 — the fix is not yet closed, so growth will resume until it is"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot read /metrics (see daemon_reachable)"}
	}
	m, err := fetchMetricsText(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not read %s/metrics: %v", baseURL, err)}
	}
	heap, heapOK := m["go_memstats_heap_inuse_bytes"]
	uptime, uptimeOK := m["process_uptime_seconds"]
	if !heapOK || !uptimeOK {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "the daemon's /metrics carries no go_memstats_heap_inuse_bytes/process_uptime_seconds — it was started from a build predating O108's instrumentation",
			Remedy: "task deploy from the fathom checkout to pick up current /metrics",
		}
	}
	reason := fmt.Sprintf("heap in-use %.2f GiB, uptime %s", heap/(1<<30), time.Duration(uptime*float64(time.Second)).Round(time.Second))
	// memory limit (W-N, docs/plans/daemon-footprint.md): named here best-
	// effort — a daemon built before this wave simply has no
	// go_memory_limit_bytes line, and that alone is not a reason to skip
	// the rest of this check the way a missing heap/uptime pair is.
	if limit, ok := m["go_memory_limit_bytes"]; ok {
		reason += fmt.Sprintf(", memory limit %s", memoryLimitDisplay(limit))
	}
	if heap >= daemonFootprintWarnBytes {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("%s — above the %d GiB budget (O108: the daemon's memory has been observed growing ~0.9 GB/hour until swap fills)", reason, daemonFootprintWarnBytes>>30),
			Remedy: remedy,
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// memoryLimitDisplay renders a go_memory_limit_bytes reading (already
// parsed as float64 by fetchMetricsText) for doctor's reason line — "off"
// for the runtime's own math.MaxInt64 sentinel (unlimitedMemoryLimit,
// memorylimit.go), else two-decimal GiB.
func memoryLimitDisplay(bytes float64) string {
	if unlimitedMemoryLimit(bytes) {
		return "off"
	}
	return fmt.Sprintf("%.2f GiB", bytes/(1<<30))
}

// fetchMetricsText GETs baseURL/metrics and parses its "key value\n" text
// exposition format into a float64 map. Unparseable or duplicate lines are
// skipped rather than erroring the whole read — /metrics is intentionally
// append-only across fathom's own history (new gauges land beside old
// ones), so a reader must tolerate lines it doesn't recognize.
func fetchMetricsText(hc *http.Client, baseURL string) (map[string]float64, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s/metrics: HTTP %d", baseURL, resp.StatusCode)
	}
	out := make(map[string]float64)
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		out[fields[0]] = v
	}
	return out, sc.Err()
}

// checkDaemonDiskWrites is RT-162's doctor row (design §12.4): the
// daemon's bytes written in the last 24 h outside explicit ingest, from
// /metrics, against 2 GiB/day. WARN over budget — disclosed, not enforced
// (the writers are the call log, the WAL and ingest, each bounded by its
// own mechanism) — and a named SKIP when the reading is unavailable, never
// a zero.
func checkDaemonDiskWrites(hc *http.Client, baseURL string, daemonUp bool) doctorCheck {
	const name = "disk_writes"
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot read /metrics (see daemon_reachable); `fathom resources` reads the ring once RT-47/RT-48 land"}
	}
	m, err := fetchMetricsText(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not read %s/metrics: %v", baseURL, err)}
	}
	outside, ok := m["fathom_bytes_written_24h_outside_ingest"]
	budget, okB := m["fathom_disk_write_budget_bytes"]
	if !ok || !okB || budget <= 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "the daemon's /metrics carries no bytes-written judgment — its build predates RT-162, or its host cannot read rusage bytes written (disclosed as unknown, never 0)",
			Remedy: "task deploy from the fathom checkout to pick up current /metrics",
		}
	}
	ingest := m["fathom_bytes_written_24h_explicit_ingest"]
	// Review finding 4: the ring is in memory until RT-47 persists it, so
	// after a start it covers minutes, not a day — name the window.
	span := "in the last 24 h"
	if w, ok := m["fathom_bytes_written_window_seconds"]; !ok {
		span = "over a window this daemon does not report"
	} else if cov := time.Duration(w) * time.Second; cov < 24*time.Hour {
		span = fmt.Sprintf("in the %s since the daemon's first reading (the ring is in memory until RT-47 — not a full day)", cov)
	}
	reason := fmt.Sprintf("written %s %s outside explicit ingest (+%s during explicit ingest) against %s/day",
		tenancy.FormatBytes(int64(outside)), span, tenancy.FormatBytes(int64(ingest)), tenancy.FormatBytes(int64(budget)))
	if outside > budget {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: reason + " — over budget; disclosed, not enforced (the call log, the WAL and ingest are the writers)",
			Remedy: "fathom resources; check the call log's size and rotation and whether auto-refresh is re-ingesting in a loop (the refresh lines in fathom-http.log)",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
