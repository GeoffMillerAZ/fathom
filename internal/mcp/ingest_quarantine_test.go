package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

type countingIngestBackend struct {
	extendedFakeBackend
	calls int
}

func (b *countingIngestBackend) IngestService(context.Context, string) error {
	b.calls++
	return nil
}

// RT-155 review finding 5: the quarantine is lifted by an explicit ingest
// that completes — not before it runs, not by one that fails, and never by
// a hook's automated ingest (a post-commit refresh is not the operator's
// retry) — and its remedy names what actually lifts it.
func TestQuarantineLiftedOnlyByCompletedExplicitIngest(t *testing.T) {
	t.Cleanup(func() { tenancy.SetQuarantined(nil); tenancy.SetQuarantineClearer(nil) })
	var mu sync.Mutex
	var cleared []string
	tenancy.SetQuarantineClearer(func(service, by string) {
		mu.Lock()
		cleared = append(cleared, service+" by "+by)
		mu.Unlock()
	})
	q := tenancy.Quarantine{Service: "svc", Since: time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC), Reason: tenancy.QuarantineReasonIngest}
	tenancy.SetQuarantined([]tenancy.Quarantine{q})
	if a := q.Attention(); strings.Contains(a, "fathom ingest svc") || !strings.Contains(a, "`ingest` tool") {
		t.Fatalf("the remedy must name what lifts the quarantine (the daemon's ingest tool), got %q", a)
	}

	call := func(srv *mcp.Server, origin string) map[string]any {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc"}}})
		resp, _ := srv.HandleRequest(mcp.WithOrigin(context.Background(), origin), raw)
		var out map[string]any
		if err := json.Unmarshal(resp, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	// A hook's ingest does not run and does not lift it; it names why.
	hookBackend := &countingIngestBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	out := call(mcp.NewServer(hookBackend), "post-commit-hook")
	errObj, _ := out["error"].(map[string]any)
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "quarantined") || hookBackend.calls != 0 {
		t.Fatalf("hook ingest of a quarantined service: calls %d, response %v; want a refusal naming the quarantine", hookBackend.calls, out)
	}
	if !tenancy.IsQuarantined("svc") {
		t.Fatal("a hook ingest lifted the quarantine")
	}

	// An explicit ingest that fails leaves it in place.
	failing := &refusingBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	seedIdentCache(t)
	if out := call(mcp.NewServer(failing), ""); out["error"] == nil || failing.calls != 1 {
		t.Fatalf("failing explicit ingest: calls %d, %v", failing.calls, out)
	}
	if !tenancy.IsQuarantined("svc") {
		t.Fatal("a failed explicit ingest lifted the quarantine")
	}

	// An explicit ingest that completes lifts it, recorded, and says so.
	ok := &countingIngestBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}}}
	out = call(mcp.NewServer(ok), "")
	res, _ := out["result"].(map[string]any)
	sc, _ := res["structuredContent"].(map[string]any)
	if ok.calls != 1 || sc["quarantine_cleared"] != true {
		t.Fatalf("completed explicit ingest: calls %d, result %v; want quarantine_cleared", ok.calls, out)
	}
	if tenancy.IsQuarantined("svc") {
		t.Fatal("a completed explicit ingest did not lift the quarantine")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(cleared) != 1 || cleared[0] != "svc by ingest tool" {
		t.Fatalf("clear rows: %v, want one by the ingest tool", cleared)
	}
}
