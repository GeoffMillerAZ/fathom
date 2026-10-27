package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// lockedBuf is a call-log writer a test reads after the call returned.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// statsSignalFake is refreshFake whose IngestStats (read by the ingest tool
// before it waits for the ingest slot) signals that the call got there.
type statsSignalFake struct {
	refreshFake
	reached chan struct{}
}

func (f *statsSignalFake) IngestStats() (int64, int64) {
	select {
	case f.reached <- struct{}{}:
	default:
	}
	return 0, 0
}

// ingestRefusalRig is a server over backend with an observing gate (the
// default mode: the handler's own refusals are the live path) and a call
// log, called through HandleRequest with the HTTP layer's refusal sink.
type ingestRefusalRig struct {
	srv  *Server
	gate *tenancy.Gate
	reg  *tenancy.InFlight
	log  *lockedBuf
}

func newIngestRefusalRig(t *testing.T, backend QueryInterface, mode tenancy.Mode) *ingestRefusalRig {
	t.Helper()
	r := &ingestRefusalRig{reg: tenancy.NewInFlight(nil), log: &lockedBuf{}}
	b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	r.gate = tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: mode, InFlight: r.reg})
	r.srv = NewServerWithLogger(backend, NewCallLogger(r.log))
	r.srv.SetGate(r.gate)
	r.srv.SetIngestWaitBudget(waitSafety)
	return r
}

// answer is one ingest call's decoded answer and what the refusal sink
// learned.
type answer struct {
	got rpcResp
	ref *tenancy.Refusal
	err error
}

// do runs one ingest tools/call; safe off the test goroutine.
func (r *ingestRefusalRig) do(ctx context.Context, svc string) answer {
	ctx, refusal := WithRefusalSink(ctx)
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ingest", "arguments": map[string]any{"service": svc}}})
	resp, ok := r.srv.HandleRequest(ctx, raw)
	if !ok {
		return answer{err: errors.New("no response")}
	}
	var a answer
	if err := json.Unmarshal(resp, &a.got); err != nil {
		return answer{err: fmt.Errorf("%v: %s", err, resp)}
	}
	a.ref = refusal()
	return a
}

// call is do on the test goroutine.
func (r *ingestRefusalRig) call(t *testing.T, ctx context.Context, svc string) (rpcResp, *tenancy.Refusal) {
	t.Helper()
	a := r.do(ctx, svc)
	if a.err != nil {
		t.Fatal(a.err)
	}
	return a.got, a.ref
}

// lastOutcome is the call-log row's outcome for the last call.
func (r *ingestRefusalRig) lastOutcome(t *testing.T) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(r.log.String()), "\n")
	var row struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatalf("call-log row: %v: %q", err, lines[len(lines)-1])
	}
	return row.Outcome
}

// assertBusy checks one handler refusal against §3.4's wire format: -32050
// with every data key, the sink set so HTTP answers 503 with Retry-After,
// and the scope writeRefusal derives.
func assertBusy(t *testing.T, got rpcResp, ref *tenancy.Refusal, code string, classScoped bool, retry string) {
	t.Helper()
	if got.Error == nil || got.Error.Code != ErrCodeBusy {
		t.Fatalf("want -32050, got %+v", got)
	}
	var keys []string
	for k := range got.Error.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := "class,in_flight,level,outcome,reason,retry_after_ms"; strings.Join(keys, ",") != want {
		t.Fatalf("data keys %v, want %s", keys, want)
	}
	if got.Error.Data["class"] != "ingest" || got.Error.Data["outcome"] != "refused" {
		t.Fatalf("class and outcome: %+v", got.Error.Data)
	}
	if _, ok := got.Error.Data["in_flight"].([]any); !ok {
		t.Fatalf("in_flight must be a list: %+v", got.Error.Data)
	}
	if !strings.Contains(got.Error.Data["reason"].(string), "adm-svc") {
		t.Fatalf("the reason must name the service: %+v", got.Error.Data)
	}
	if ref == nil {
		t.Fatal("the refusal sink learned nothing: HTTP would answer 200")
	}
	if status, ra := RefusalHTTP(ref); status != 503 || ra != retry {
		t.Fatalf("HTTP %d Retry-After %q, want 503 %q", status, ra, retry)
	}
	if ref.Code != code || ref.ClassScoped() != classScoped {
		t.Fatalf("refusal code %q class-scoped %v, want %q %v", ref.Code, ref.ClassScoped(), code, classScoped)
	}
}

// TestIngestToolRefusalsUseTheAdmissionWireFormat (GDEV-08, design §3.4):
// the ingest tool's own refusals (the refresh gate's store distress and
// refuse rung, at request time or when the queue drops the job, and the
// ingest-slot wait) answer -32050 with the admission data keys and reach
// the HTTP sink, scoped as the refusal reaches; the call-log row says
// refused, and no ticket outlives the call. A governor cancel while the
// call waits is an abandonment, never a 503.
func TestIngestToolRefusalsUseTheAdmissionWireFormat(t *testing.T) {
	t.Cleanup(func() {
		tenancy.PublishLevel(tenancy.LevelNormal)
		tenancy.ResetLifecycleStateForTest()
	})
	noLeak := func(t *testing.T, r *ingestRefusalRig) {
		t.Helper()
		if n := r.reg.Len(); n != 0 {
			t.Fatalf("%d in-flight entries left behind: %+v", n, r.reg.Snapshot())
		}
		if lines := r.gate.MetricLines(); strings.Contains(lines, "fathom_inflight{") {
			t.Fatalf("a running count outlived its call:\n%s", lines)
		}
	}

	t.Run("refuse rung", func(t *testing.T) {
		tenancy.PublishLevel(tenancy.LevelRefuse)
		defer tenancy.PublishLevel(tenancy.LevelNormal)
		f := &refreshFake{path: "/repo", indexed: "old"}
		r := newIngestRefusalRig(t, f, tenancy.ModeObserve)
		r.srv.SetRefreshQueue(newTestRefreshQueue(t, f))
		got, ref := r.call(t, context.Background(), "adm-svc")
		assertBusy(t, got, ref, tenancy.RefusalLevel, false, "30")
		if f.ingests != 0 {
			t.Fatalf("a refused ingest ran %d time(s)", f.ingests)
		}
		if o := r.lastOutcome(t); o != "refused" {
			t.Fatalf("call-log outcome %q, want refused", o)
		}
		noLeak(t, r)
	})

	t.Run("store distress", func(t *testing.T) {
		tenancy.NoteStoreVolumeFree(1, 2, time.Time{})
		defer tenancy.ResetLifecycleStateForTest()
		f := &refreshFake{path: "/repo", indexed: "old"}
		r := newIngestRefusalRig(t, f, tenancy.ModeObserve)
		r.srv.SetRefreshQueue(newTestRefreshQueue(t, f))
		got, ref := r.call(t, context.Background(), "adm-svc")
		assertBusy(t, got, ref, "", true, "60")
		if f.ingests != 0 {
			t.Fatalf("a refused ingest ran %d time(s)", f.ingests)
		}
		if o := r.lastOutcome(t); o != "refused" {
			t.Fatalf("call-log outcome %q, want refused", o)
		}
		noLeak(t, r)
	})

	t.Run("store distress after queueing", func(t *testing.T) {
		defer tenancy.ResetLifecycleStateForTest()
		f := &refreshFake{path: "/repo", indexed: "old"}
		r := newIngestRefusalRig(t, f, tenancy.ModeObserve)
		asked := make(chan struct{}, 1)
		q := NewRefreshQueue(f, refresh.Config{StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false },
			Gate: func(s string, explicit bool, now time.Time) refresh.Verdict {
				v := refresh.TenancyGate(s, explicit, now)
				select {
				case asked <- struct{}{}:
				default:
				}
				return v
			}})
		r.srv.SetRefreshQueue(q)
		done := make(chan answer, 1)
		go func() { done <- r.do(context.Background(), "adm-svc") }()
		select {
		case <-asked: // the request passed the gate and is queued
		case <-time.After(waitSafety):
			t.Fatal("the ingest never reached the queue")
		}
		tenancy.NoteStoreVolumeFree(1, 2, time.Time{})
		startTestQueue(t, q) // the pick re-checks the gate and drops the job
		var a answer
		select {
		case a = <-done:
		case <-time.After(waitSafety):
			t.Fatal("the dropped job never answered its waiter")
		}
		if a.err != nil {
			t.Fatal(a.err)
		}
		assertBusy(t, a.got, a.ref, "", true, "60")
		if f.ingests != 0 {
			t.Fatalf("a dropped ingest ran %d time(s)", f.ingests)
		}
		noLeak(t, r)
	})

	t.Run("ingest slot wait", func(t *testing.T) {
		// The wait ends only when the call's context does, and dispatch
		// never delivers that as a refusal: ingest-class work is detached
		// (no gate: the wait never ends), and a gate's cancel is settled
		// as an abandonment (the next case). So this pins only the
		// handler's defensive answer, called directly.
		release, err := tenancy.AcquireIngestSlot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		srv := NewServer(&refreshFake{path: "/repo", indexed: "old"})
		te, ok := srv.findTool("ingest")
		if !ok {
			t.Fatal("no ingest tool")
		}
		ctx, refusal := WithRefusalSink(context.Background())
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, rerr := te.handler(ctx, json.RawMessage(`{"service":"adm-svc"}`))
		raw, _ := json.Marshal(map[string]any{"error": rerr})
		var got rpcResp
		_ = json.Unmarshal(raw, &got)
		assertBusy(t, got, refusal(), tenancy.RefusalSlots, false, "30")
	})

	t.Run("governor cancel while waiting is abandoned", func(t *testing.T) {
		release, err := tenancy.AcquireIngestSlot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		f := &statsSignalFake{refreshFake: refreshFake{path: "/repo", indexed: "old"}, reached: make(chan struct{}, 1)}
		r := newIngestRefusalRig(t, f, tenancy.ModeEnforce)
		done := make(chan answer, 1)
		go func() { done <- r.do(context.Background(), "adm-svc") }()
		select {
		case <-f.reached:
		case <-time.After(waitSafety):
			t.Fatal("the ingest never reached the slot wait")
		}
		r.reg.Cancel(tenancy.AbandonedBy(tenancy.Cause{}), tenancy.ClassIngest)
		var a answer
		select {
		case a = <-done:
		case <-time.After(waitSafety):
			t.Fatal("the cancelled ingest never answered")
		}
		if a.err != nil {
			t.Fatal(a.err)
		}
		if a.got.Error == nil || a.got.Error.Data["outcome"] != outcomeAbandonedByGovernor {
			t.Fatalf("want the governor's abandonment, got %+v", a.got)
		}
		if a.ref != nil {
			t.Fatalf("an abandonment must not reach the 503 sink: %+v", a.ref)
		}
		noLeak(t, r)
	})
}
