package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// diskHost is govHost plus scripted cumulative bytes-written readings for
// the process and its sampled children.
type diskHost struct {
	govHost
	written  atomic.Int64
	children atomic.Int64 // tenancy.Unknown = not read
}

func (h *diskHost) ReadHost(s *tenancy.Sample) {
	h.govHost.ReadHost(s)
	s.DiskWritten = h.written.Load()
	s.ChildDiskWritten = h.children.Load()
}

// TestDiskWritesJudgedInStatuszMetricsAndDoctor pins RT-162's surfaces
// (design §12.4, §13.3): the governor feeds every sample's bytes written
// into the 24 h ring, split by whether an explicit ingest (the `ingest`
// tool — not auto-refresh's pipeline ingest) was in
// flight; /statusz's tenancy block and /metrics carry the judgment; and
// doctor's disk_writes row WARNs over 2 GiB/day, disclosed, never enforced.
func TestDiskWritesJudgedInStatuszMetricsAndDoctor(t *testing.T) {
	r := newGovRig(t)
	h := &diskHost{govHost: govHost{footprint: tenancy.GiB}}
	h.children.Store(tenancy.Unknown)
	r.g.host = h
	h.written.Store(100 * tenancy.MiB)
	r.tick()
	// An auto-refresh ingest (pkg/client's pipeline registration) counts.
	_, endAuto := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "svc", tenancy.OriginPipeline)
	h.written.Store(1100 * tenancy.MiB)
	r.tick()
	endAuto()
	// An explicit ingest (the /rpc ticket) does not.
	_, endExplicit := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "svc", "agent")
	h.written.Store(6 * tenancy.GiB)
	r.tick()
	endExplicit()
	h.written.Store(6*tenancy.GiB + 1460*tenancy.MiB)
	r.tick()

	snap := r.g.snapshot()
	if !snap.DiskWritesOver || !strings.Contains(snap.DiskWrites, "over the 2 GiB/day budget") {
		t.Fatalf("snapshot disk_writes = %q over=%v, want 2.5 GiB outside explicit ingest, over", snap.DiskWrites, snap.DiskWritesOver)
	}
	if !strings.Contains(snap.DiskWrites, "2.5/2 GiB") {
		t.Fatalf("disk_writes = %q, want 2.5/2 GiB outside explicit ingest", snap.DiskWrites)
	}

	activeGovernor.Store(r.g)
	t.Cleanup(func() { activeGovernor.CompareAndSwap(r.g, nil) })
	m := governorMetricLines()
	for _, want := range []string{
		fmt.Sprintf(`fathom_bytes_written_total{subsystem="self"} %d`, 6*tenancy.GiB+1460*tenancy.MiB),
		fmt.Sprintf("fathom_bytes_written_24h_outside_ingest %d", 2560*tenancy.MiB),
		fmt.Sprintf("fathom_disk_write_budget_bytes %d", tenancy.DiskWriteBudgetPerDay),
	} {
		if !strings.Contains(m, want) {
			t.Errorf("/metrics lacks %q:\n%s", want, m)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, m) }))
	defer srv.Close()
	c := checkDaemonDiskWrites(srv.Client(), srv.URL, true)
	if c.Name != "disk_writes" || c.Status != doctorWarn || !strings.Contains(c.Reason, "2.5 GiB") || !strings.Contains(c.Reason, "2 GiB/day") {
		t.Fatalf("doctor row = %+v, want disk_writes WARN naming 2.5 GiB against 2 GiB/day", c)
	}
	if c := checkDaemonDiskWrites(srv.Client(), srv.URL, false); c.Status != doctorSkipped || !strings.Contains(c.Reason, "unreachable") {
		t.Fatalf("doctor row with the daemon down = %+v, want a named SKIP", c)
	}
	// Review finding 4: the ring is in memory and starts empty at every
	// start, so a young daemon's judgment covers minutes, not a day — the
	// window is exported and the doctor row names it instead of "the last
	// 24 h".
	if !strings.Contains(m, "fathom_bytes_written_window_seconds ") {
		t.Errorf("/metrics lacks the judgment's window:\n%s", m)
	}
	if strings.Contains(c.Reason, "last 24 h") || !strings.Contains(c.Reason, "since the daemon's first reading") {
		t.Errorf("doctor row over a %s ring claims a day: %q", "4 min", c.Reason)
	}
}

// TestDiskWritesCountSampledChildren pins review finding 5 on RT-162
// (design §12.4, "self and children"): the sampled children's bytes written
// join the daemon's own in the ring, /metrics carries each by subsystem,
// and a sample whose child reading is unknown holds the children's last
// total instead of reading as a counter reset.
func TestDiskWritesCountSampledChildren(t *testing.T) {
	r := newGovRig(t)
	h := &diskHost{govHost: govHost{footprint: tenancy.GiB}}
	r.g.host = h
	h.written.Store(10 * tenancy.MiB)
	h.children.Store(0)
	r.tick()
	h.children.Store(2 * tenancy.GiB) // `go list -export` filled GOCACHE
	r.tick()
	h.children.Store(tenancy.Unknown) // the tree could not be listed
	r.tick()
	h.children.Store(2*tenancy.GiB + 100*tenancy.MiB)
	r.tick()
	snap := r.g.snapshot()
	if !snap.DiskWritesOver {
		t.Fatalf("2.1 GiB written by children outside explicit ingest is not over budget: %q", snap.DiskWrites)
	}
	activeGovernor.Store(r.g)
	t.Cleanup(func() { activeGovernor.CompareAndSwap(r.g, nil) })
	m := governorMetricLines()
	for _, want := range []string{
		fmt.Sprintf(`fathom_bytes_written_total{subsystem="self"} %d`, 10*tenancy.MiB),
		fmt.Sprintf(`fathom_bytes_written_total{subsystem="children"} %d`, 2*tenancy.GiB+100*tenancy.MiB),
		fmt.Sprintf("fathom_bytes_written_24h_outside_ingest %d", 2*tenancy.GiB+110*tenancy.MiB),
	} {
		if !strings.Contains(m, want) {
			t.Errorf("/metrics lacks %q:\n%s", want, m)
		}
	}
}
