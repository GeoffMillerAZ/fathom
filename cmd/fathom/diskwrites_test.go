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

// diskHost is govHost plus a scripted cumulative bytes-written reading.
type diskHost struct {
	govHost
	written atomic.Int64
}

func (h *diskHost) ReadHost(s *tenancy.Sample) {
	h.govHost.ReadHost(s)
	s.DiskWritten = h.written.Load()
}

// TestDiskWritesJudgedInStatuszMetricsAndDoctor pins RT-162's surfaces
// (design §12.4, §13.3): the governor feeds every sample's bytes written
// into the 24 h ring, split by whether an explicit ingest (the `ingest` or
// `onboard_service` tool — not auto-refresh's pipeline ingest) was in
// flight; /statusz's tenancy block and /metrics carry the judgment; and
// doctor's disk_writes row WARNs over 2 GiB/day, disclosed, never enforced.
func TestDiskWritesJudgedInStatuszMetricsAndDoctor(t *testing.T) {
	r := newGovRig(t)
	h := &diskHost{govHost: govHost{footprint: tenancy.GiB}}
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
		fmt.Sprintf("fathom_bytes_written_total %d", 6*tenancy.GiB+1460*tenancy.MiB),
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
}
