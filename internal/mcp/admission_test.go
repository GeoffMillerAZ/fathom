package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// admissionRig is a server with an enforcing (or observing) gate and three
// synthetic tools: a point tool, a tree tool counting its runs, and a tree
// tool that sleeps past its deadline, plus a point tool that charges more
// than its per-call ceiling.
type admissionRig struct {
	srv      *Server
	gate     *tenancy.Gate
	reg      *tenancy.InFlight
	treeRuns atomic.Int64
	log      *bytes.Buffer
	level    tenancy.Level
	safe     bool
}

func newAdmissionRig(t *testing.T, mode tenancy.Mode) *admissionRig {
	t.Helper()
	r := &admissionRig{reg: tenancy.NewInFlight(nil), log: &bytes.Buffer{}}
	b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	r.gate = tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: mode, InFlight: r.reg,
		Level: func() tenancy.Level { return r.level }, SafeMode: func() bool { return r.safe }})
	r.srv = NewServerWithLogger(nil, NewCallLogger(r.log))
	r.srv.SetGate(r.gate)
	obj := map[string]any{"type": "object"}
	r.srv.registerTool(toolDescriptor{Name: "zz_point", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassPoint)},
		func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
			return map[string]any{"ok": true}, nil
		})
	r.srv.registerTool(toolDescriptor{Name: "zz_tree", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassTree)},
		func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
			r.treeRuns.Add(1)
			return map[string]any{"items": []any{}}, nil
		})
	r.srv.registerTool(toolDescriptor{Name: "zz_slow", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassIndex)},
		func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
			<-ctx.Done()
			return map[string]any{"items": []any{}}, nil // empty: looks complete
		})
	r.srv.registerTool(toolDescriptor{Name: "zz_hungry", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassPoint)},
		func(ctx context.Context, _ json.RawMessage) (any, *rpcError) {
			read := 0
			for range 4 {
				if err := tenancy.Charge(ctx, 3*tenancy.MiB); err != nil {
					break
				}
				read++
			}
			return map[string]any{"files": read}, nil
		})
	return r
}

type rpcResp struct {
	Result map[string]any `json:"result"`
	Error  *struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	} `json:"error"`
}

func (r *admissionRig) call(t *testing.T, ctx context.Context, tool string, args map[string]any) rpcResp {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	resp, ok := r.srv.HandleRequest(ctx, raw)
	if !ok {
		t.Fatal("no response")
	}
	var out rpcResp
	if err := json.Unmarshal(resp, &out); err != nil {
		t.Fatalf("%v: %s", err, resp)
	}
	return out
}

// hold occupies every slot of class c and returns the release.
func (r *admissionRig) hold(t *testing.T, c tenancy.Class, tool string) func() {
	t.Helper()
	var tks []*tenancy.Ticket
	for range r.gate.Budget().Slots[c] {
		tk, ref := r.gate.Admit(context.Background(), tenancy.Request{Tool: tool, Class: c, Origin: tenancy.OriginAgent})
		if ref != nil {
			t.Fatalf("hold: %+v", ref)
		}
		tks = append(tks, tk)
	}
	return func() {
		for _, tk := range tks {
			tk.Release(tenancy.OutcomeOK)
		}
	}
}

// TestAdmissionReturnsNamed503WithRetryAfter (RT-56, design §3.3–§3.4):
// dispatch admits before the handler runs; a refusal is JSON-RPC -32050
// naming the reason, class, level, retry and what holds the class, and the
// HTTP layer's sink learns it so it can answer 503 with Retry-After.
func TestAdmissionReturnsNamed503WithRetryAfter(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)
	defer r.hold(t, tenancy.ClassTree, "search")()

	ctx, refusal := WithRefusalSink(WithOrigin(context.Background(), "edit-hook"))
	got := r.call(t, ctx, "zz_tree", nil)
	if r.treeRuns.Load() != 0 {
		t.Fatal("the handler ran for a refused call — a refusal must cost nothing")
	}
	if got.Error == nil || got.Error.Code != ErrCodeBusy {
		t.Fatalf("want -32050, got %+v", got)
	}
	d := got.Error.Data
	if !strings.Contains(fmt.Sprint(d["reason"]), "tree-class slots full (1/1): search") || d["class"] != "tree" || d["level"] != "normal" {
		t.Fatalf("data %+v", d)
	}
	if ms, _ := d["retry_after_ms"].(float64); ms < 1000 {
		t.Fatalf("retry_after_ms %v", d["retry_after_ms"])
	}
	if inf, _ := d["in_flight"].([]any); len(inf) != 1 || inf[0] != "search" {
		t.Fatalf("in_flight %v", d["in_flight"])
	}
	ref := refusal()
	if ref == nil {
		t.Fatal("the HTTP sink did not learn the refusal")
	}
	status, retry := RefusalHTTP(ref)
	if status != 503 || retry != "5" {
		t.Fatalf("HTTP %d Retry-After %q, want 503 and 5", status, retry)
	}

	// An admitted call leaves the sink empty.
	ctx2, refusal2 := WithRefusalSink(context.Background())
	if got := r.call(t, ctx2, "zz_point", nil); got.Error != nil {
		t.Fatalf("point refused: %+v", got.Error)
	}
	if refusal2() != nil {
		t.Fatal("sink set for an admitted call")
	}
}

// TestRefusalIsNamedNeverEmpty (RT-57, §3.4): a refused, an abandoned and
// a partial answer each name themselves; none is an empty result that
// looks complete.
func TestRefusalIsNamedNeverEmpty(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)

	release := r.hold(t, tenancy.ClassTree, "search")
	got := r.call(t, WithOrigin(context.Background(), "edit-hook"), "zz_tree", nil)
	release()
	if got.Result != nil || got.Error == nil || !strings.HasPrefix(got.Error.Message, "fathom busy: ") || got.Error.Data["reason"] == "" {
		t.Fatalf("refusal not named: %+v", got)
	}

	// A handler whose deadline passes returns an empty list; the answer is
	// the named abandonment, not the empty list.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	got = r.call(t, ctx, "zz_slow", nil)
	if got.Result != nil || got.Error == nil || got.Error.Data["outcome"] != "abandoned" || !strings.Contains(got.Error.Message, "deadline") {
		t.Fatalf("abandoned call not named: %+v", got)
	}

	// A producer stopped at the per-call ceiling: the envelope says partial.
	got = r.call(t, context.Background(), "zz_hungry", nil)
	if got.Error != nil {
		t.Fatal(got.Error.Message)
	}
	b, _ := got.Result["budget"].(map[string]any)
	if b["partial"] != true || !strings.Contains(fmt.Sprint(b["reason"]), "point-class per-call ceiling") {
		t.Fatalf("partial answer not named: %+v", got.Result)
	}

	// The refuse rung and safe mode refuse by name too.
	r.level = tenancy.LevelRefuse
	got = r.call(t, context.Background(), "zz_tree", nil)
	if got.Error == nil || got.Error.Code != ErrCodeBusy || got.Error.Data["level"] != "refuse" {
		t.Fatalf("refuse rung: %+v", got)
	}
}

// TestBatchCapsAndAdmitsEachCall (RT-60, §3.3): batch takes at most 16
// calls, admits each in its own class (a refused sub-call is that entry's
// named error, the batch itself answers), and stops when its context ends.
func TestBatchCapsAndAdmitsEachCall(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)
	calls := make([]any, 17)
	for i := range calls {
		calls[i] = map[string]any{"name": "zz_point"}
	}
	got := r.call(t, context.Background(), "batch", map[string]any{"calls": calls})
	if got.Error == nil || got.Error.Code != ErrCodeParams || !strings.Contains(got.Error.Message, "16") {
		t.Fatalf("17 calls: want a named cap error, got %+v", got)
	}

	defer r.hold(t, tenancy.ClassTree, "search")()
	ctx, refusal := WithRefusalSink(WithOrigin(context.Background(), "edit-hook"))
	got = r.call(t, ctx, "batch", map[string]any{"calls": []any{
		map[string]any{"name": "zz_point"}, map[string]any{"name": "zz_tree"},
	}})
	if got.Error != nil {
		t.Fatalf("batch refused whole: %+v", got.Error)
	}
	results := batchResults(t, got)
	if results[0]["error"] != nil || !strings.HasPrefix(fmt.Sprint(results[1]["error"]), "fathom busy: tree-class slots full") {
		t.Fatalf("sub-calls not admitted each in its class: %+v", results)
	}
	if r.treeRuns.Load() != 0 {
		t.Fatal("refused sub-call ran")
	}
	if refusal() != nil {
		t.Fatal("a sub-call refusal must not turn the batch into a 503")
	}

	// The batch stops at ctx.Done(): nothing after the cancellation runs.
	cctx, cancel := context.WithCancel(context.Background())
	r.srv.registerTool(toolDescriptor{Name: "zz_cancel", InputSchema: map[string]any{"type": "object"}, group: GroupShared, cost: declareCost(tenancy.ClassPoint)},
		func(context.Context, json.RawMessage) (any, *rpcError) { cancel(); return map[string]any{}, nil })
	got = r.call(t, cctx, "batch", map[string]any{"calls": []any{
		map[string]any{"name": "zz_cancel"}, map[string]any{"name": "zz_point"}, map[string]any{"name": "zz_point"},
	}})
	if got.Error != nil {
		t.Fatalf("%+v", got.Error)
	}
	results = batchResults(t, got)
	for _, i := range []int{1, 2} {
		if !strings.Contains(fmt.Sprint(results[i]["error"]), "not run: the batch's context ended") {
			t.Fatalf("call %d ran after the batch's context ended: %+v", i, results[i])
		}
	}
}

func batchResults(t *testing.T, got rpcResp) []map[string]any {
	t.Helper()
	sc, _ := got.Result["structuredContent"].(map[string]any)
	if sc == nil {
		content, _ := got.Result["content"].([]any)
		first, _ := content[0].(map[string]any)
		_ = json.Unmarshal([]byte(first["text"].(string)), &sc)
	}
	var out []map[string]any
	for _, x := range sc["results"].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

// TestCallLogRowCarriesClassOutcomeCaller (RT-64, §13.3): every call-log
// row carries class, queued_ms, bytes_read, outcome and the caller's
// identity, under schema_version 3.
func TestCallLogRowCarriesClassOutcomeCaller(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)
	ctx := WithCaller(context.Background(), CallerInfo{PID: 4242, PPID: 1, Session: "sess-1"})
	r.call(t, ctx, "zz_hungry", nil)
	release := r.hold(t, tenancy.ClassTree, "search")
	r.call(t, WithOrigin(ctx, "edit-hook"), "zz_tree", nil)
	release()

	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(r.log.String()), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if len(rows) != 2 {
		t.Fatalf("%d rows", len(rows))
	}
	ok, refused := rows[0], rows[1]
	if ok["schema_version"] != float64(3) {
		t.Fatalf("schema_version %v, want 3", ok["schema_version"])
	}
	if ok["class"] != "point" || ok["outcome"] != "partial" || ok["bytes_read"] != float64(9*tenancy.MiB) {
		t.Fatalf("ok row %+v", ok)
	}
	if _, has := ok["queued_ms"]; !has {
		t.Fatalf("queued_ms missing: %+v", ok)
	}
	caller, _ := ok["caller"].(map[string]any)
	if caller["pid"] != float64(4242) || caller["ppid"] != float64(1) || caller["session"] != "sess-1" || caller["test"] != false {
		t.Fatalf("caller %+v", ok["caller"])
	}
	if refused["class"] != "tree" || refused["outcome"] != "refused" || refused["origin"] != "edit-hook" {
		t.Fatalf("refused row %+v", refused)
	}

	// Without a gate (stdio) the class and outcome are still recorded.
	var buf bytes.Buffer
	plain := NewServerWithLogger(nil, NewCallLogger(&buf))
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "capabilities", "arguments": map[string]any{}}})
	plain.HandleRequest(context.Background(), raw)
	var row map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &row)
	if row["class"] != "point" || row["outcome"] != "ok" {
		t.Fatalf("ungated row %+v", row)
	}
}

// TestDecisionPortClosedUnderRefuse (RT-65, vision 36 §8): a decision-port
// call is admitted as egress; at the refuse rung and in safe mode it is
// skipped with a disclosure, never made.
func TestDecisionPortClosedUnderRefuse(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)
	ctx, release, skip := r.srv.AdmitDecision(context.Background(), "route")
	if skip != "" {
		t.Fatalf("normal: skipped %q", skip)
	}
	if tenancy.TicketFrom(ctx) == nil || tenancy.TicketFrom(ctx).Class() != tenancy.ClassEgress {
		t.Fatal("decision call not admitted as egress")
	}
	release()

	r.level = tenancy.LevelRefuse
	_, release, skip = r.srv.AdmitDecision(context.Background(), "route")
	release()
	if !strings.Contains(skip, "decision port skipped") || !strings.Contains(skip, "refuse") {
		t.Fatalf("refuse: skip %q", skip)
	}
	r.level = tenancy.LevelNormal
	r.safe = true
	_, release, skip = r.srv.AdmitDecision(context.Background(), "obligations-rerank")
	release()
	if !strings.Contains(skip, "safe mode") {
		t.Fatalf("safe mode: skip %q", skip)
	}

	// Observe mode never skips (it counts), and no gate never skips.
	o := newAdmissionRig(t, tenancy.ModeObserve)
	o.level = tenancy.LevelRefuse
	_, release, skip = o.srv.AdmitDecision(context.Background(), "route")
	release()
	if skip != "" {
		t.Fatalf("observe skipped: %q", skip)
	}
	_, release, skip = NewServer(nil).AdmitDecision(context.Background(), "route")
	release()
	if skip != "" {
		t.Fatalf("no gate skipped: %q", skip)
	}
}

// TestPanickingHandlerReleasesItsSlot (review finding, RT-56): a handler
// that panics must not keep its admission slot, its in-flight entry or its
// running count — one panic in a 1-slot class would otherwise refuse that
// class for the life of the process. The panic answers a named error, in a
// top-level call and in a batch sub-call alike.
func TestPanickingHandlerReleasesItsSlot(t *testing.T) {
	r := newAdmissionRig(t, tenancy.ModeEnforce)
	obj := map[string]any{"type": "object"}
	r.srv.registerTool(toolDescriptor{Name: "zz_boom", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassFleet)},
		func(context.Context, json.RawMessage) (any, *rpcError) { panic("boom") })
	r.srv.registerTool(toolDescriptor{Name: "zz_fleet", InputSchema: obj, group: GroupShared, cost: declareCost(tenancy.ClassFleet)},
		func(context.Context, json.RawMessage) (any, *rpcError) { return map[string]any{"ok": true}, nil })

	call := func(tool string, args map[string]any) (got rpcResp, panicked any) {
		defer func() { panicked = recover() }()
		return r.call(t, context.Background(), tool, args), nil
	}
	got, p := call("zz_boom", nil)
	if p != nil {
		t.Fatalf("the panic escaped dispatch: %v", p)
	}
	if got.Error == nil || !strings.Contains(got.Error.Message, "boom") {
		t.Fatalf("a panicking handler must answer a named error: %+v", got)
	}
	if got := r.call(t, WithOrigin(context.Background(), "edit-hook"), "zz_fleet", nil); got.Error != nil {
		t.Fatalf("the panicked call kept its fleet slot: %s", got.Error.Message)
	}
	got, p = call("batch", map[string]any{"calls": []any{map[string]any{"name": "zz_boom"}}})
	if p != nil || got.Error != nil {
		t.Fatalf("batch: panic %v, error %+v", p, got.Error)
	}
	if res := batchResults(t, got); !strings.Contains(fmt.Sprint(res[0]["error"]), "boom") {
		t.Fatalf("batch sub-call panic not named: %+v", res)
	}
	if got := r.call(t, WithOrigin(context.Background(), "edit-hook"), "zz_fleet", nil); got.Error != nil {
		t.Fatalf("the panicked batch sub-call kept its fleet slot: %s", got.Error.Message)
	}
	if n := len(r.reg.Snapshot()); n != 0 {
		t.Fatalf("%d in-flight entries left behind", n)
	}
}
