package mcp

// RT-179 (the wave-B live reading's F2): a find_symbol sent during an
// ingest returned 531 s later while the call log said latency 5,045 ms,
// queued 0 — the wait happened before the log's clock started, and past
// the 30 s index deadline. The call's clock and its class deadline now run
// from the request's arrival: a read that waited past its deadline before
// admission is refused with the named reason (never run for nobody), the
// call log records the true wall and queued time, and a cold caller
// waiting on another call's navigation build leaves at its own deadline.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func lastLogRow(t *testing.T, r *admissionRig) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(r.log.String()), "\n")
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("call log row: %v (%q)", err, r.log.String())
	}
	return row
}

func registerIndexTool(r *admissionRig) {
	r.srv.registerTool(toolDescriptor{Name: "zz_index", InputSchema: map[string]any{"type": "object"}, group: GroupShared, cost: declareCost(tenancy.ClassIndex)},
		func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
			return map[string]any{"symbols": []any{"x"}}, nil
		})
}

func TestIndexReadPastItsDeadlineBeforeAdmissionIsRefusedAndLoggedTrue(t *testing.T) {
	for _, mode := range []tenancy.Mode{tenancy.ModeObserve, tenancy.ModeEnforce} {
		r := newAdmissionRig(t, mode)
		registerIndexTool(r)
		ctx := WithArrival(context.Background(), time.Now().Add(-40*time.Second))
		got := r.call(t, ctx, "zz_index", nil)
		if got.Error == nil || got.Error.Code != ErrCodeBusy || !strings.Contains(got.Error.Message, "deadline") {
			t.Fatalf("%v: an index read that arrived 40 s ago answered %+v, want -32050 naming its passed deadline", mode, got)
		}
		row := lastLogRow(t, r)
		if row["outcome"] != "refused" {
			t.Fatalf("%v: row outcome %v, want refused", mode, row["outcome"])
		}
		if lat, _ := row["latency_ms"].(float64); lat < 40000 {
			t.Fatalf("%v: latency_ms %v, want the true wall (≥ 40000) from the request's arrival", mode, row["latency_ms"])
		}
		if q, _ := row["queued_ms"].(float64); q < 40000 {
			t.Fatalf("%v: queued_ms %v, want the wait before admission (≥ 40000)", mode, row["queued_ms"])
		}
	}
}

func TestIndexReadLogsWaitBeforeAdmission(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeObserve)
	registerIndexTool(r)
	got := r.call(t, WithArrival(context.Background(), time.Now().Add(-2*time.Second)), "zz_index", nil)
	if got.Error != nil {
		t.Fatalf("index read within its deadline: %+v", got.Error)
	}
	row := lastLogRow(t, r)
	if lat, _ := row["latency_ms"].(float64); lat < 2000 {
		t.Fatalf("latency_ms %v, want ≥ 2000 (from arrival)", row["latency_ms"])
	}
	if q, _ := row["queued_ms"].(float64); q < 2000 {
		t.Fatalf("queued_ms %v, want ≥ 2000 (waited before admission)", row["queued_ms"])
	}
}

// A cold caller behind another call's navigation build leaves at its own
// ctx's end instead of waiting for the build.
func TestNavMemoWaiterLeavesAtItsDeadline(t *testing.T) {
	f := &navMemoFake{gate: make(chan struct{})}
	f.mark.Store("w1")
	s := NewServer(f)
	go s.loadNavGraphs(context.Background(), f, "svc") // the leader, held by the gate
	defer close(f.gate)
	for f.loads.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan *navGraphs, 1)
	go func() { done <- s.loadNavGraphs(ctx, f, "svc") }()
	select {
	case g := <-done:
		if g != nil {
			t.Fatalf("a waiter whose ctx ended got graphs %+v, want nil", g)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a nav-memo waiter stayed blocked 3 s past its 100 ms deadline")
	}
}
