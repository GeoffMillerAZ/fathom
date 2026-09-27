package main

// The wave-B live reading's lifecycle findings
// (docs/bench/readings/rt-wave-b-seatbelt-2026-09-27.md §5): F3, a hook
// origin is classified like the admission gate classifies it; F4, every
// lifecycle refusal names its reason in the message and says how far it
// reaches (X-Fathom-Busy-Scope, design §3.4).

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// F3: hooks send edit-hook, session-hook or prime — never the bare "hook" —
// and each is refused at once during warm-up, never queued for the wait.
func TestWarmupRefusesHookOriginsAtOnce(t *testing.T) {
	resetLifecycleModes(t)
	now := time.Now()
	tenancy.SetWarmup(now, time.Minute)
	g := newWarmupGate(func() string { return "observe" }, 2*time.Second)
	rel, ref := g.acquire(context.Background(), tenancy.ClassTree, "symbol_context", "", now)
	if ref != nil {
		t.Fatalf("first tree call refused: %+v", ref)
	}
	defer rel()
	for _, origin := range []string{"edit-hook", "session-hook", "prime", "post-commit-hook"} {
		start := time.Now()
		_, ref := g.acquire(context.Background(), tenancy.ClassTree, "obligations", origin, now)
		if ref == nil {
			t.Fatalf("%s: second tree call during warm-up admitted", origin)
		}
		if waited := time.Since(start); waited > 500*time.Millisecond {
			t.Fatalf("%s: waited %v for the warm-up slot; a hook is refused at once", origin, waited)
		}
	}
}

func lifecycleGateResponse(t *testing.T, tool string, args string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	raw := []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + args + `}}`)
	w := httptest.NewRecorder()
	if _, ok := lifecycleGate(context.Background(), w, raw, ""); ok {
		t.Fatalf("%s admitted, want a lifecycle refusal", tool)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("refusal body: %v", err)
	}
	return w, body
}

// F4: safe mode, warm-up, drain and store distress each send the scope
// §3.4 gives them and "fathom busy: <reason>".
func TestLifecycleRefusalsCarryScopeAndReason(t *testing.T) {
	cases := []struct {
		name, tool, args, scope, reason string
		setup                           func(t *testing.T)
	}{
		{"safe mode", "symbol_context", `{"service":"s","symbol":"x"}`, hookguard.BusyScopeDaemon, "safe mode", func(t *testing.T) {
			tenancy.SetSafeMode(&tenancy.SafeModeInfo{Since: time.Now(), Reason: "3 counted exits in 30m0s"})
		}},
		{"drain", "symbol_context", `{"service":"s","symbol":"x"}`, hookguard.BusyScopeDaemon, "draining", func(t *testing.T) {
			draining.Store(true)
		}},
		{"store distress", "ingest", `{"service":"s"}`, hookguard.BusyScopeClass, "ingest and writes are refused", func(t *testing.T) {
			tenancy.NoteStoreDistress("disk full", time.Now())
		}},
		{"warm-up", "symbol_context", `{"service":"s","symbol":"x"}`, hookguard.BusyScopeClass, "warm-up", func(t *testing.T) {
			tenancy.SetWarmup(time.Now(), time.Minute)
			rel, ref := serveWarmupGate.acquire(context.Background(), tenancy.ClassTree, "search", "", time.Now())
			if ref != nil {
				t.Fatalf("first tree call refused: %+v", ref)
			}
			t.Cleanup(rel)
			old := serveWarmupGate.wait
			serveWarmupGate.wait = 0
			t.Cleanup(func() { serveWarmupGate.wait = old })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetLifecycleModes(t)
			tc.setup(t)
			w, body := lifecycleGateResponse(t, tc.tool, tc.args)
			if w.Code != 503 || w.Header().Get("Retry-After") == "" {
				t.Fatalf("status %d Retry-After %q, want 503 with Retry-After", w.Code, w.Header().Get("Retry-After"))
			}
			if got := w.Header().Get(hookguard.BusyScopeHeader); got != tc.scope {
				t.Fatalf("%s = %q, want %q", hookguard.BusyScopeHeader, got, tc.scope)
			}
			e, _ := body["error"].(map[string]any)
			msg, _ := e["message"].(string)
			if !strings.HasPrefix(msg, "fathom busy: ") || !strings.Contains(msg, tc.reason) {
				t.Fatalf("message %q, want \"fathom busy: <reason>\" naming %q", msg, tc.reason)
			}
		})
	}
}

// F4: the store-wait handler's refusal is daemon-scoped.
func TestStoreWaitRefusalIsDaemonScoped(t *testing.T) {
	var st atomic.Pointer[storeWaitState]
	st.Store(&storeWaitState{Errno: "EPERM", Label: storeWaitTCC})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/rpc", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"find_symbol"}}`))
	waitingHandler(&st).ServeHTTP(w, r)
	if w.Code != 503 || w.Header().Get(hookguard.BusyScopeHeader) != hookguard.BusyScopeDaemon {
		t.Fatalf("store-wait refusal: status %d scope %q, want 503 daemon", w.Code, w.Header().Get(hookguard.BusyScopeHeader))
	}
}

// F6: a --lifecycle-id daemon's record lives under addr-<port>-<pid>/ but
// its ledger, ring and diagnostics under <lifecycle-id>/. resources (and
// doctor, which reads the same projection) resolve the ledger through the
// record, by URL or by --id, and label the daemon unmanaged with its pid.
func TestResourcesFindsTheLifecycleIDLedger(t *testing.T) {
	home := t.TempDir()
	dir := daemonRecordsDir(home)
	rec := console.DaemonRecord{SchemaVersion: console.DaemonRecordSchemaVersion, Addr: "127.0.0.1:7691", PID: 4321,
		ID: "addr-7691-4321", LifecycleID: "rtb", StartedAt: "2026-09-27T10:00:00Z"}
	recPath := filepath.Join(dir, rec.ID, "record.json")
	if err := console.WriteDaemonRecord(recPath, rec); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(dir, "rtb", "lifecycle.jsonl")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	row := tenancy.NewLifecycleRow(tenancy.RowStart, time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC), 4321, "test")
	if err := appendLifecycle(ledger, row); err != nil {
		t.Fatal(err)
	}
	env := resourcesEnv{home: home, now: func() time.Time { return time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC) }}
	for _, by := range []struct{ id, url string }{{"", "http://127.0.0.1:7691"}, {"rtb", ""}} {
		rep := buildResources(env, by.id, by.url, 24*time.Hour)
		if rep.Lifecycle.Ledger != ledger {
			t.Fatalf("by %+v: ledger %q, want the lifecycle-id ledger %q", by, rep.Lifecycle.Ledger, ledger)
		}
		if rep.Lifecycle.NotRecorded != "" {
			t.Fatalf("by %+v: %q — the ledger has a start row", by, rep.Lifecycle.NotRecorded)
		}
		if rep.Daemon.Managed || rep.Daemon.PID != 4321 {
			t.Fatalf("by %+v: managed=%v pid=%d, want an unmanaged daemon with pid 4321", by, rep.Daemon.Managed, rep.Daemon.PID)
		}
	}
}

// F7: the exit row's top_in_flight said 1m15s for an ingest the reading
// timed at 10 minutes. The age is now the wall-clock difference (the
// ledger's and the ring's own clock: a monotonic difference stops while a
// Mac sleeps) and each entry names the instant it started, so the row can
// be checked against the ledger.
func TestTopInFlightNamesWallAgeAndStart(t *testing.T) {
	r := newGovRig(t)
	started := r.now
	_, done := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "rtb-probe", "agent")
	defer done()
	r.now = r.now.Add(10 * time.Minute)
	r.g.sample.At = r.now
	top := r.g.topInFlight(8)
	if len(top) != 1 || !strings.Contains(top[0], "10m0s") || !strings.Contains(top[0], started.UTC().Format(time.RFC3339)) {
		t.Fatalf("top_in_flight %q, want the 10m0s wall age and the start %s", top, started.UTC().Format(time.RFC3339))
	}
}
