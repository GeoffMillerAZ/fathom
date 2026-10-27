package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// setupPanicBackend panics in ServiceMeta while panics is positive: the
// citation tools call ServiceMeta in dispatch's setup (newCitationReads),
// after admission and before the handler runs.
type setupPanicBackend struct {
	refreshFake
	panics atomic.Int32
}

func (b *setupPanicBackend) ServiceMeta(ctx context.Context, svc string) (string, string, bool, error) {
	if b.panics.Add(-1) >= 0 {
		panic("service meta boom")
	}
	return b.refreshFake.ServiceMeta(ctx, svc)
}

// TestAdmittedSetupPanicReleasesItsSlot (GDEV-08): a panic between
// admission and settle (the citation reads, the collector, the refresh
// notes) answers the same named error a panicking handler does, and the
// call's ticket is released: no in-flight entry, running count or slot
// outlives it. With no gate (serve-mcp) the panic never kills the loop.
func TestAdmittedSetupPanicReleasesItsSlot(t *testing.T) {
	args := map[string]any{"service": "adm-setup-panic", "name": "Alpha"}
	wantMsg := "internal error: find_symbol panicked: service meta boom"

	t.Run("gated", func(t *testing.T) {
		be := &setupPanicBackend{}
		be.panics.Store(1)
		reg := tenancy.NewInFlight(nil)
		b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
		// A queue wait fires at once: a full class refuses without the wall clock.
		fired := func(time.Duration) (<-chan time.Time, func() bool) {
			ch := make(chan time.Time, 1)
			ch <- time.Time{}
			return ch, func() bool { return true }
		}
		g := tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: tenancy.ModeEnforce, InFlight: reg,
			Level: func() tenancy.Level { return tenancy.LevelNormal }, Timer: fired})
		var log bytes.Buffer
		srv := NewServerWithLogger(be, NewCallLogger(&log))
		srv.SetGate(g)
		r := &admissionRig{srv: srv, gate: g, reg: reg, log: &log}

		if n := b.Slots[tenancy.ClassIndex]; n != 2 {
			t.Fatalf("rig premise: index class has %d slots, want 2", n)
		}
		held, ref := g.Admit(context.Background(), tenancy.Request{Tool: "zz_hold", Class: tenancy.ClassIndex, Origin: tenancy.OriginAgent})
		if ref != nil {
			t.Fatalf("hold: %+v", ref)
		}
		defer held.Release(tenancy.OutcomeOK)

		call := func() (got rpcResp, panicked any) {
			defer func() { panicked = recover() }()
			return r.call(t, context.Background(), "find_symbol", args), nil
		}
		got, p := call()
		if p != nil {
			t.Fatalf("the panic escaped dispatch: %v", p)
		}
		if got.Error == nil || got.Error.Code != ErrCodeInternal || got.Error.Message != wantMsg {
			t.Fatalf("a setup panic must answer the named internal error %q: %+v", wantMsg, got)
		}
		if got.Error.Data["outcome"] != "error" || got.Error.Data["panic"] != "service meta boom" {
			t.Fatalf("data must name the outcome and the panic: %+v", got.Error.Data)
		}
		if snap := reg.Snapshot(); len(snap) != 1 || snap[0].Tool != "zz_hold" {
			t.Fatalf("the panicked call left an in-flight entry: %+v", snap)
		}
		if got := r.call(t, context.Background(), "find_symbol", args); got.Error != nil {
			t.Fatalf("the panicked call kept its index slot: %s", got.Error.Message)
		}
		held.Release(tenancy.OutcomeOK)
		if n := reg.Len(); n != 0 {
			t.Fatalf("%d in-flight entries left behind", n)
		}
		lines := g.MetricLines()
		if strings.Contains(lines, "fathom_inflight{") {
			t.Fatalf("a running count outlived its call:\n%s", lines)
		}
		if !strings.Contains(lines, `fathom_calls_total{class="index",outcome="error"} 1`) {
			t.Fatalf("the panicked call must settle as an error:\n%s", lines)
		}
		if !strings.Contains(log.String(), `"outcome":"error"`) {
			t.Fatalf("the panicked call must write its call-log row: %s", log.String())
		}
	})

	t.Run("no gate", func(t *testing.T) {
		be := &setupPanicBackend{}
		be.panics.Store(1)
		srv := NewServer(be)
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": "find_symbol", "arguments": args}})
		in := strings.NewReader(string(raw) + "\n" + string(raw) + "\n")
		var out bytes.Buffer
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("the panic escaped Serve: %v", p)
				}
			}()
			_ = srv.Serve(context.Background(), in, &out)
		}()
		resps := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(resps) != 2 {
			t.Fatalf("want 2 responses (the loop survives the panic), got %d: %s", len(resps), out.String())
		}
		var first, second rpcResp
		if err := json.Unmarshal([]byte(resps[0]), &first); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(resps[1]), &second); err != nil {
			t.Fatal(err)
		}
		if first.Error == nil || first.Error.Message != wantMsg {
			t.Fatalf("first call: want %q, got %+v", wantMsg, first)
		}
		if second.Error != nil {
			t.Fatalf("second call: %+v", second.Error)
		}
	})
}
