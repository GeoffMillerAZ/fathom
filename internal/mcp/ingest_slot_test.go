package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// slotProbeBackend records whether the ingest slot was free while its
// ingest ran.
type slotProbeBackend struct {
	extendedFakeBackend
	calls    int
	slotFree bool
}

func (b *slotProbeBackend) IngestService(context.Context, string) error {
	b.calls++
	if release, ok := tenancy.TryIngestSlot(); ok {
		b.slotFree = true
		release()
	}
	return nil
}

// TestIngestToolHoldsTheIngestSlot (review finding, RT-160): the ingest
// tool (post-commit hooks, onboarding) runs under tenancy's one ingest
// slot, as the inline ingest does, so a derived-table backfill never runs
// beside it and writes a summary with a mixed count; while another holds
// the slot the ingest queues behind it.
func TestIngestToolHoldsTheIngestSlot(t *testing.T) {
	call := func(ctx context.Context, srv *mcp.Server) map[string]any {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc"}}})
		resp, _ := srv.HandleRequest(ctx, raw)
		var out map[string]any
		if err := json.Unmarshal(resp, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	b := &slotProbeBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	seedIdentCache(t)
	call(context.Background(), mcp.NewServer(b))
	if b.calls != 1 || b.slotFree {
		t.Fatalf("ingest ran %d time(s) with the ingest slot free=%v; want once, under the slot", b.calls, b.slotFree)
	}

	// With the slot held (a backfill), the ingest queues behind it —
	// ingest-class work is detached from its requester (§3.5) — and runs
	// once the slot is free, never beside the holder.
	release, err := tenancy.AcquireIngestSlot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan map[string]any, 1)
	go func() { done <- call(context.Background(), mcp.NewServer(b)) }()
	time.Sleep(50 * time.Millisecond)
	select {
	case out := <-done:
		release()
		t.Fatalf("the ingest ran beside the slot's holder: %v", out)
	default:
	}
	release()
	select {
	case out := <-done:
		if out["error"] != nil || b.calls != 2 || b.slotFree {
			t.Fatalf("queued ingest: calls %d, slot free %v, response %v", b.calls, b.slotFree, out)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the queued ingest never ran after the slot was released")
	}
}
