package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// postToolCall posts one tools/call and returns the status, headers and
// decoded body.
func postToolCall(t *testing.T, h http.Handler, tool string, args map[string]any, origin string) (int, http.Header, map[string]any) {
	t.Helper()
	body := rpcBody(t, map[string]any{"jsonrpc": "2.0", "id": 7, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	req := httptest.NewRequest(http.MethodPost, "/rpc", body)
	if origin != "" {
		req.Header.Set("X-Fathom-Origin", origin)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	return rr.Code, rr.Header(), m
}

// busyReason returns the -32050 refusal's reason, or "" when m is not one.
func busyReason(m map[string]any) string {
	e, _ := m["error"].(map[string]any)
	if e == nil || e["code"] != float64(rpcBusyCode) {
		return ""
	}
	d, _ := e["data"].(map[string]any)
	r, _ := d["reason"].(string)
	return r
}

func resetLifecycleModes(t *testing.T) {
	t.Helper()
	tenancy.ResetLifecycleStateForTest()
	draining.Store(false)
	backgroundPaused.Store(false)
	t.Cleanup(func() {
		tenancy.ResetLifecycleStateForTest()
		draining.Store(false)
		backgroundPaused.Store(false)
	})
}

type fakeDecider struct{ calls atomic.Int32 }

func (d *fakeDecider) Name() string { return "fake" }
func (d *fakeDecider) Decide(context.Context, decision.Request) (decision.Response, error) {
	d.calls.Add(1)
	return decision.Response{}, nil
}

// TestSafeModeRefusesHeavyServesIndex (RT-39, §2.3): safe mode refuses
// tree and fleet with the named reason (503, Retry-After, -32050), serves
// point and index, and turns off auto-refresh's inline ingest, the console
// feeders and the decision port; leaving it restores each.
func TestSafeModeRefusesHeavyServesIndex(t *testing.T) {
	resetLifecycleModes(t)
	h := newTestHandler(t)
	since := time.Date(2026, 9, 26, 21, 7, 0, 0, time.Local)
	applySafeMode(&tenancy.SafeModeInfo{Since: since, Reason: "3 counted exits in 30m0s (over_budget ×3)"}, nil)

	for _, tool := range []string{"search", "list_services"} {
		code, hdr, m := postToolCall(t, h, tool, map[string]any{"service": "x", "pattern": "y"}, "")
		reason := busyReason(m)
		if code != http.StatusServiceUnavailable || hdr.Get("Retry-After") == "" || !strings.Contains(reason, "fathom safe mode since 21:07: 3 counted exits") || !strings.Contains(reason, "run fathom resources") {
			t.Fatalf("%s in safe mode: %d %v %q", tool, code, hdr, reason)
		}
	}
	for _, tool := range []string{"find_symbol", "capabilities"} {
		if code, _, m := postToolCall(t, h, tool, map[string]any{"service": "x", "name": "y"}, ""); code != http.StatusOK || busyReason(m) != "" {
			t.Fatalf("%s in safe mode: %d %v", tool, code, m)
		}
	}
	if ok, why := tenancy.InlineIngestAllowed(time.Now()); ok || !strings.Contains(why, "safe mode") {
		t.Fatalf("auto-refresh inline ingest in safe mode: %v %q", ok, why)
	}
	if backgroundAllowed() {
		t.Fatal("console feeders run in safe mode")
	}
	fd := &fakeDecider{}
	gd := gatedDecider{inner: fd}
	if _, err := gd.Decide(context.Background(), decision.Request{}); err == nil || !strings.Contains(err.Error(), "decision port closed") || fd.calls.Load() != 0 {
		t.Fatalf("decision port in safe mode: %v (calls %d)", err, fd.calls.Load())
	}

	leaveSafeMode(nil)
	if code, _, m := postToolCall(t, h, "search", map[string]any{"service": "x", "pattern": "y"}, ""); code != http.StatusOK || busyReason(m) != "" {
		t.Fatalf("search after safe mode: %d %v", code, m)
	}
	if !backgroundAllowed() {
		t.Fatal("feeders still off after safe mode")
	}
	if _, err := gd.Decide(context.Background(), decision.Request{}); err != nil || fd.calls.Load() != 1 {
		t.Fatalf("decision port after safe mode: %v", err)
	}
	// The refuse rung closes the decision port too (§2.1).
	tenancy.PublishLevel(tenancy.LevelRefuse)
	defer tenancy.PublishLevel(tenancy.LevelNormal)
	if _, err := gd.Decide(context.Background(), decision.Request{}); err == nil {
		t.Fatal("decision port open at refuse")
	}
}

// TestSafeModeLeavesAfter30MinNormalAndWritesModeRow (§2.3): the managed
// daemon's safe mode steps down after 30 min at normal, writing a mode row
// and restarting warm-up.
func TestSafeModeLeavesAfter30MinNormalAndWritesModeRow(t *testing.T) {
	resetLifecycleModes(t)
	ledger := filepath.Join(t.TempDir(), "lifecycle.jsonl")
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	sm := &safeModeController{tracker: tenancy.SafeModeTracker{Safe: true}, ledger: ledger, pid: 9, warmup: time.Minute, logf: func(string) {}}
	applySafeMode(&tenancy.SafeModeInfo{Since: t0, Reason: "r"}, nil)
	sm.step(t0, tenancy.LevelNormal)
	sm.step(t0.Add(31*time.Minute), tenancy.LevelNormal)
	if _, on := tenancy.SafeModeNow(); on {
		t.Fatal("still in safe mode after 30 min at normal")
	}
	if !tenancy.InWarmup(t0.Add(31*time.Minute + time.Second)) {
		t.Fatal("leaving safe mode does not restart warm-up")
	}
	rows := readRowsT(t, ledger)
	if len(rows) != 1 || rows[0].Row != tenancy.RowMode || rows[0].From != tenancy.ModeSafe || rows[0].To != tenancy.ModeNormal {
		t.Fatalf("rows = %+v", rows)
	}
}

// TestWarmupGateOneHeavySlot (RT-40, §2.3): during warm-up a second
// concurrent tree call is refused under admission=enforce (a hook at
// once), logged and admitted under observe, and unaffected after warm-up.
func TestWarmupGateOneHeavySlot(t *testing.T) {
	resetLifecycleModes(t)
	now := time.Now()
	tenancy.SetWarmup(now, time.Minute)
	g := newWarmupGate(func() string { return "enforce" }, 50*time.Millisecond)
	rel, ref := g.acquire(context.Background(), tenancy.ClassTree, "search", "agent", now)
	if ref != nil {
		t.Fatalf("first tree call refused: %+v", ref)
	}
	if _, ref := g.acquire(context.Background(), tenancy.ClassTree, "obligations", "hook", now); ref == nil || !strings.Contains(ref.Reason, "warm-up") || !strings.Contains(ref.Reason, "search") {
		t.Fatalf("second tree call during warm-up: %+v", ref)
	}
	if r2, ref := g.acquire(context.Background(), tenancy.ClassIndex, "find_symbol", "hook", now); ref != nil {
		t.Fatalf("index refused during warm-up: %+v", ref)
	} else {
		r2()
	}
	rel()
	if r3, ref := g.acquire(context.Background(), tenancy.ClassTree, "obligations", "hook", now); ref != nil {
		t.Fatalf("tree slot not released: %+v", ref)
	} else {
		r3()
	}
	// Warm-up holds under admission = "observe" too (a lifecycle bound,
	// like safe mode): TestWarmupHoldsUnderObserve.
	obs := newWarmupGate(func() string { return "observe" }, 10*time.Millisecond)
	ra, _ := obs.acquire(context.Background(), tenancy.ClassFleet, "list_services", "agent", now)
	if _, ref := obs.acquire(context.Background(), tenancy.ClassFleet, "list_services", "agent", now); ref == nil {
		t.Fatal("observe admitted a second fleet call during warm-up")
	}
	ra()
	after := now.Add(2 * time.Minute)
	r4, _ := g.acquire(context.Background(), tenancy.ClassTree, "a", "agent", after)
	r5, ref := g.acquire(context.Background(), tenancy.ClassTree, "b", "hook", after)
	if ref != nil {
		t.Fatalf("after warm-up: %+v", ref)
	}
	r4()
	r5()
}

// storeErr mimics modernc.org/sqlite's *Error (Code() int).
type storeErr struct {
	code int
	msg  string
}

func (e storeErr) Error() string { return e.msg }
func (e storeErr) Code() int     { return e.code }

// distressStore is a store whose derived-table reads fail with err, under a
// real client and the daemon's own fleet adapter.
type distressStore struct {
	*store.MemoryStore
	err error
}

func (s distressStore) ListServiceIDs(context.Context) ([]string, error) { return []string{"svc"}, nil }
func (s distressStore) ServiceSummary(context.Context, string) (store.ServiceSummary, bool, error) {
	return store.ServiceSummary{}, false, s.err
}
func (s distressStore) SourceKindCounts(context.Context, string) ([]store.SourceKindCount, error) {
	return nil, s.err
}

// distressAdapter is the daemon's fleet adapter (client.FleetAdapter) over a
// store whose reads fail with err, its log captured in the returned buffer.
func distressAdapter(err error) (*fleetAdapter, *strings.Builder) {
	var buf strings.Builder
	fa := newTestFleetAdapter(newCountingConnector(), distressStore{MemoryStore: store.NewMemoryStore(), err: err})
	fa.SetLog(dlog.New(&buf, dlog.LevelDebug, time.Now))
	return fa, &buf
}

// TestDiskFullIsDistressNeverZero (RT-41, §2.1, I38) pins the product path:
// a SQLITE_FULL or SQLITE_IOERR read through the fleet adapter that
// list_services calls is store distress — named in the error and the daemon
// log, recorded so tenancy.StoreDistressNow refuses ingest and writes — and
// the tool renders that error, never symbol_count 0.
func TestDiskFullIsDistressNeverZero(t *testing.T) {
	resetLifecycleModes(t)
	ctx := context.Background()
	full := storeErr{13, "database or disk is full (13)"}

	// SQLITE_FULL: both fleet reads name distress and record it.
	fa, logged := distressAdapter(full)
	if _, err := fa.StoredSummary(ctx, "svc"); err == nil || !strings.Contains(err.Error(), "store distress") || !strings.Contains(err.Error(), "SQLITE_FULL") {
		t.Fatalf("StoredSummary(FULL) = %v, want a named store-distress error", err)
	}
	if !strings.Contains(logged.String(), "store distress") {
		t.Fatalf("the fleet log does not name the distress: %q", logged.String())
	}
	if why, on := tenancy.StoreDistressNow(time.Now()); !on || !strings.Contains(why, "SQLITE_FULL") {
		t.Fatalf("SQLITE_FULL did not set distress: %v %q", on, why)
	}
	tenancy.ResetLifecycleStateForTest()
	if n, _, err := fa.SymbolAggregatesErr("svc"); err == nil || n != 0 || !strings.Contains(err.Error(), "SQLITE_FULL") {
		t.Fatalf("SymbolAggregatesErr(FULL) = %d, %v, want a named error", n, err)
	}

	// SQLITE_IOERR (a wrapped extended code) is distress too.
	tenancy.ResetLifecycleStateForTest()
	fa, _ = distressAdapter(fmt.Errorf("wrapped: %w", storeErr{266, "disk I/O error (266)"}))
	if _, err := fa.StoredSummary(ctx, "svc"); err == nil || !strings.Contains(err.Error(), "SQLITE_IOERR") {
		t.Fatalf("StoredSummary(IOERR_READ) = %v, want SQLITE_IOERR named", err)
	}
	if why, on := tenancy.StoreDistressNow(time.Now()); !on || !strings.Contains(why, "SQLITE_IOERR") {
		t.Fatalf("SQLITE_IOERR did not set distress: %v %q", on, why)
	}

	// CANTOPEN is a transient lock, not distress.
	tenancy.ResetLifecycleStateForTest()
	fa, _ = distressAdapter(errors.New("SQLITE_CANTOPEN"))
	if _, err := fa.StoredSummary(ctx, "svc"); err == nil || strings.Contains(err.Error(), "distress") {
		t.Fatalf("CANTOPEN is not distress: %v", err)
	}
	if _, on := tenancy.StoreDistressNow(time.Now()); on {
		t.Fatal("a non-distress error set distress")
	}

	// A failed read through list_services is a named error, never a zero count.
	tenancy.ResetLifecycleStateForTest()
	fa, _ = distressAdapter(full)
	srv := mcp.NewServerWithFleetAndLogger(nil, fa, nil)
	resp, _ := srv.HandleRequest(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`))
	body := string(resp)
	if strings.Contains(body, `\"symbol_count\": 0`) || strings.Contains(body, `\"symbol_count\":0`) {
		t.Fatalf("a failed read rendered symbol_count 0: %s", body)
	}
	if !strings.Contains(body, "symbol_count_error") || !strings.Contains(body, "database or disk is full") {
		t.Fatalf("list_services does not name the read failure: %s", body)
	}

	// In distress, ingest (and the refresh-file write) is refused, named;
	// reads are served.
	tenancy.NoteStoreDistress("disk full (SQLITE_FULL)", time.Now())
	h := newTestHandler(t)
	code, _, m := postToolCall(t, h, "ingest", map[string]any{"service": "x", "path": t.TempDir()}, "")
	if code != http.StatusServiceUnavailable || !strings.Contains(busyReason(m), "store distress") {
		t.Fatalf("ingest in distress: %d %v", code, m)
	}
	if code, _, m := postToolCall(t, h, "find_symbol", map[string]any{"service": "x", "name": "y"}, ""); code != http.StatusOK || busyReason(m) != "" {
		t.Fatalf("find_symbol in distress: %d %v", code, m)
	}
	req := httptest.NewRequest(http.MethodPost, "/refresh-file", strings.NewReader(`{"service":"x","file":"a.go"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "store distress") {
		t.Fatalf("/refresh-file in distress: %d %s", rr.Code, rr.Body.String())
	}
	if ok, why := tenancy.InlineIngestAllowed(time.Now()); ok || !strings.Contains(why, "store distress") {
		t.Fatalf("auto-refresh in distress: %v %q", ok, why)
	}
}

// TestStoreWaitLabels (RT-42, §8.4): EPERM on a path that stats is a TCC
// wedge; ENOENT under /Volumes/ is an unmounted volume; anything else is
// unavailable, errno named.
func TestStoreWaitLabels(t *testing.T) {
	stats := func(string) error { return nil }
	gone := func(string) error { return os.ErrNotExist }
	perm := &os.PathError{Op: "open", Path: "/Volumes/m2ext/db/fleet.sqlite", Err: syscall.EPERM}
	if errno, label := classifyStoreWait(perm, "/Volumes/m2ext/db/fleet.sqlite", stats, nil); errno != "EPERM" || label != storeWaitTCC {
		t.Fatalf("EPERM that stats = %s %s", errno, label)
	}
	if _, label := classifyStoreWait(perm, "/Volumes/m2ext/db/fleet.sqlite", gone, nil); label == storeWaitTCC {
		t.Fatal("EPERM on a path that does not stat is not a TCC wedge")
	}
	noent := &os.PathError{Op: "open", Path: "/Volumes/m2ext/db/fleet.sqlite", Err: syscall.ENOENT}
	if errno, label := classifyStoreWait(noent, "/Volumes/m2ext/db/fleet.sqlite", gone, nil); errno != "ENOENT" || label != storeWaitUnmounted {
		t.Fatalf("ENOENT under /Volumes = %s %s", errno, label)
	}
	if _, label := classifyStoreWait(noent, "/Users/x/db/fleet.sqlite", gone, nil); label != storeWaitUnavailable {
		t.Fatalf("ENOENT elsewhere = %s", label)
	}
	if !storeUnavailable(perm) || !storeUnavailable(noent) || !storeUnavailable(&os.PathError{Err: syscall.EACCES}) || storeUnavailable(errors.New("schema mismatch")) {
		t.Fatal("storeUnavailable classifies the wrong errors")
	}
}

// TestTCCWedgeRemedyNamesDaemonBinary (RT-165, §8.4): the tcc-wedge
// remedy names the daemon binary's path, its signing identity and the Full
// Disk Access / Removable Volumes grant it needs — never a terminal's
// bundle id — and names an ad-hoc signature as the likely cause.
func TestTCCWedgeRemedyNamesDaemonBinary(t *testing.T) {
	exe := "/Users/op/.local/libexec/fathom/fathom"
	signed := signingIdentity{Binary: exe, Identifier: "fathom", Authority: "fathom-dev"}
	r := storeWaitRemedy(storeWaitTCC, exe, signed, "/Volumes/m2ext/db/fleet.sqlite")
	for _, want := range []string{exe, "fathom-dev", "Full Disk Access", "Removable Volumes", "/Volumes/m2ext"} {
		if !strings.Contains(r, want) {
			t.Errorf("remedy lacks %q: %s", want, r)
		}
	}
	for _, bad := range []string{"ghostty", "com.mitchellh", "Terminal.app", "iTerm", "tccutil reset SystemPolicyRemovableVolumes com."} {
		if strings.Contains(r, bad) {
			t.Errorf("remedy names a terminal (%q): %s", bad, r)
		}
	}
	adhoc := signingIdentity{Binary: exe, Identifier: "a.out", AdHoc: true}
	if ra := storeWaitRemedy(storeWaitTCC, exe, adhoc, "/Volumes/m2ext/db/fleet.sqlite"); !strings.Contains(ra, "ad-hoc") || !strings.Contains(ra, "task deploy") {
		t.Errorf("ad-hoc remedy: %s", ra)
	}
	if rn := storeWaitRemedy(storeWaitUnmounted, exe, signed, "/Volumes/m2ext/db/fleet.sqlite"); !strings.Contains(rn, "mount") || strings.Contains(rn, "Full Disk Access") {
		t.Errorf("unmounted remedy: %s", rn)
	}
	if ru := storeWaitRemedy(storeWaitTCC, exe, signingIdentity{Binary: exe, Unsigned: true}, "/Volumes/m2ext/db/fleet.sqlite"); !strings.Contains(ru, "unsigned") || !strings.Contains(ru, "task deploy") {
		t.Errorf("unsigned remedy: %s", ru)
	}
}

// checkOpenStoreWithWaitBacksOff (RT-42, §8.4): an unavailable store is
// retried with backoff 5 s → 5 min, each wait disclosed with its label and
// remedy — and a store that recovers is returned. Run as the first subtest
// of TestServeHTTPWaitsForStoreWithoutExiting.
func checkOpenStoreWithWaitBacksOff(t *testing.T) {
	var sleeps []time.Duration
	var states []storeWaitState
	n := 0
	perm := &os.PathError{Op: "open", Path: "/Volumes/m2ext/s.sqlite", Err: syscall.EPERM}
	got, err := openStoreWithWait(context.Background(), storeWaitDeps[int]{
		path: "/Volumes/m2ext/s.sqlite",
		open: func() (int, error) {
			n++
			if n < 9 {
				return 0, perm
			}
			return 42, nil
		},
		stat:     func(string) error { return nil },
		sleep:    func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil },
		onWait:   func(s storeWaitState) { states = append(states, s) },
		now:      time.Now,
		exe:      "/x/fathom",
		codesign: func(string) signingIdentity { return signingIdentity{Binary: "/x/fathom", Authority: "fathom-dev"} },
	})
	if err != nil || got != 42 {
		t.Fatalf("openStoreWithWait = %d, %v", got, err)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second, 160 * time.Second, 5 * time.Minute, 5 * time.Minute}
	if fmt.Sprint(sleeps) != fmt.Sprint(want) {
		t.Fatalf("backoff = %v, want %v", sleeps, want)
	}
	if len(states) != 9 || states[0].Label != storeWaitTCC || !strings.Contains(states[0].Remedy, "/x/fathom") || !states[8].Open {
		t.Fatalf("states = %+v", states)
	}
	// A non-availability error is returned at once (a corrupt store is not
	// waited on).
	_, err = openStoreWithWait(context.Background(), storeWaitDeps[int]{
		path: "/x", open: func() (int, error) { return 0, errors.New("schema mismatch") },
		stat: func(string) error { return nil }, sleep: func(context.Context, time.Duration) error { t.Fatal("slept"); return nil },
		onWait: func(storeWaitState) {}, now: time.Now,
	})
	if err == nil {
		t.Fatal("a non-availability error was waited on")
	}
}

// TestWaitingHandlerAnswersErrno (RT-42, §8.4, §3.4): while waiting,
// /healthz and /statusz answer with the errno and label, and tools/call is
// 503 -32050 "store unavailable: …" with retry_after_ms 30000.
func TestWaitingHandlerAnswersErrno(t *testing.T) {
	var st atomic.Pointer[storeWaitState]
	st.Store(&storeWaitState{Errno: "EPERM", Label: storeWaitTCC, Err: "open /Volumes/m2ext/s.sqlite: operation not permitted", Remedy: "grant it", Since: time.Now()})
	h := waitingHandler(&st)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "waiting-for-store") || !strings.Contains(rr.Body.String(), "EPERM") {
		t.Fatalf("/healthz: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/statusz?format=json", nil))
	if !strings.Contains(rr.Body.String(), `"state":"waiting-for-store"`) || !strings.Contains(rr.Body.String(), "tcc-wedge") {
		t.Fatalf("/statusz: %s", rr.Body.String())
	}
	code, hdr, m := postToolCall(t, h, "find_symbol", map[string]any{"name": "x"}, "")
	e, _ := m["error"].(map[string]any)
	d, _ := e["data"].(map[string]any)
	if code != http.StatusServiceUnavailable || hdr.Get("Retry-After") != "30" || !strings.HasPrefix(fmt.Sprint(d["reason"]), "store unavailable: operation not permitted") || d["retry_after_ms"] != float64(30000) {
		t.Fatalf("tools/call while waiting: %d %v %v", code, hdr, m)
	}
}

// TestPortHeldBySameLabelExitsZero (RT-43, §8.4): a port held by a managed
// fathom of the same label that answers is "already served" (exit 0); any
// other holder exits 75 and is named — pid and command.
func TestPortHeldBySameLabelExitsZero(t *testing.T) {
	rec := &console.DaemonRecord{ID: "com.example.fathom", Managed: true, PID: 500}
	same := portHolderFacts{ListenerPID: 500, ListenerCmd: "/u/.local/libexec/fathom/fathom serve-http --managed", Record: rec, Responds: true}
	if code, reason, detail := resolvePortHolder(true, "com.example.fathom", same); code != 0 || reason != tenancy.ExitPortHeld || !strings.Contains(detail, "already served by pid 500") {
		t.Fatalf("same label: %d %s %q", code, reason, detail)
	}
	for name, f := range map[string]portHolderFacts{
		"other label":        {ListenerPID: 500, ListenerCmd: "fathom serve-http", Record: &console.DaemonRecord{ID: "other", Managed: true, PID: 500}, Responds: true},
		"record pid differs": {ListenerPID: 501, ListenerCmd: "python -m http.server 7575", Record: rec, Responds: true},
		"not answering":      {ListenerPID: 500, ListenerCmd: "fathom serve-http", Record: rec},
		"no record":          {ListenerPID: 77, ListenerCmd: "nc -l 7575"},
	} {
		code, reason, detail := resolvePortHolder(true, "com.example.fathom", f)
		if code != tenancy.ExitRetryLater || reason != tenancy.ExitPortHeld || !strings.Contains(detail, fmt.Sprintf("pid %d", f.ListenerPID)) {
			t.Errorf("%s: %d %s %q", name, code, reason, detail)
		}
		if f.ListenerCmd != "" && !strings.Contains(detail, f.ListenerCmd) {
			t.Errorf("%s: holder command not named: %q", name, detail)
		}
	}
	// An unmanaged daemon is never "already served".
	if code, _, _ := resolvePortHolder(false, "", same); code != tenancy.ExitRetryLater {
		t.Fatalf("unmanaged on a held port: %d", code)
	}
	// An unknown holder is named as unknown, not guessed.
	if _, _, detail := resolvePortHolder(true, "x", portHolderFacts{}); !strings.Contains(detail, "unknown") {
		t.Fatalf("unknown holder: %q", detail)
	}
}

// TestServeHTTPHeldPortExits75AndRecordsHolder (RT-43): a real serve-http
// on a port another process holds exits 75 (after the bind retry) and
// writes exit{port_held} naming the holder.
func TestServeHTTPHeldPortExits75AndRecordsHolder(t *testing.T) {
	home, _ := withIsolatedConfigHome(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	prev := bindRetryMaxWait
	bindRetryMaxWait = 300 * time.Millisecond
	defer func() { bindRetryMaxWait = prev }()
	var out, errb strings.Builder
	code := runServeHTTP([]string{"--store", filepath.Join(t.TempDir(), "s.sqlite"), "--addr", ln.Addr().String(), "--call-log", "", "--lifecycle-id", "held-port-test"}, &out, &errb)
	if code != tenancy.ExitRetryLater {
		t.Fatalf("rc = %d, want 75\n%s\n%s", code, out.String(), errb.String())
	}
	ledger, _ := ledgerPathFor(home, "held-port-test")
	rows := readRowsT(t, ledger)
	last := rows[len(rows)-1]
	if last.Row != tenancy.RowExit || last.Reason != tenancy.ExitPortHeld || last.Code == nil || *last.Code != 75 || !strings.Contains(last.Detail, fmt.Sprintf("pid %d", os.Getpid())) {
		t.Fatalf("exit row = %+v", last)
	}
}

// TestSIGTERMDrainsWithinExitTimeout (RT-44, §8.5): SIGTERM writes the
// exit{drain} row FIRST, closes heavy admission, cancels heavy and ingest
// work (an ingest stops at its next batch boundary), lets point and index
// finish, and exits 0 within the drain budget even when a request never
// finishes.
func TestSIGTERMDrainsWithinExitTimeout(t *testing.T) {
	resetLifecycleModes(t)
	ledger := filepath.Join(t.TempDir(), "lifecycle.jsonl")
	reg := tenancy.NewInFlight(time.Now)
	ingestCtx, endIngest := reg.Begin(context.Background(), tenancy.ClassIngest, "ingest", "svc", "agent")
	defer endIngest()
	treeCtx, endTree := reg.Begin(context.Background(), tenancy.ClassTree, "search", "svc", "agent")
	defer endTree()

	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	exited := make(chan int, 1)
	d := drainer{
		ledger: ledger, pid: 11, version: "v", now: time.Now, inflight: reg, budget: 300 * time.Millisecond,
		logf: func(string) {},
		shutdown: func(ctx context.Context) error {
			rows, _, _ := readLifecycle(ledger)
			if len(rows) == 1 && rows[0].Reason == tenancy.ExitDrain {
				note("row-before-shutdown")
			}
			if draining.Load() {
				note("heavy-closed")
			}
			<-ctx.Done() // a request that never finishes
			return ctx.Err()
		},
		removeRecord: func() { note("record-removed") },
		exit:         func(code int) { exited <- code },
	}
	// The shutdown hook blocks until its ctx ends, so drainAndExit exiting
	// at all is the 300 ms budget firing (a budget ignored would leave the
	// shutdown blocked for good); the budget is the input, not a duration
	// asserted on.
	d.drainAndExit("SIGTERM")
	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("drain exit code %d, want 0", code)
		}
	case <-time.After(waitSafety):
		t.Fatal("drain did not exit")
	}
	if got := strings.Join(order, ","); got != "row-before-shutdown,heavy-closed,record-removed" {
		t.Fatalf("order = %s", got)
	}
	if ingestCtx.Err() == nil || treeCtx.Err() == nil {
		t.Fatal("in-flight ingest/tree work not cancelled at the drain")
	}
	if !errors.Is(context.Cause(ingestCtx), errDraining) {
		t.Fatalf("ingest cancelled with %v", context.Cause(ingestCtx))
	}
	rows := readRowsT(t, ledger)
	if len(rows) != 1 || rows[0].Row != tenancy.RowExit || rows[0].Reason != tenancy.ExitDrain || rows[0].Code == nil || *rows[0].Code != 0 {
		t.Fatalf("rows = %+v", rows)
	}
	// Heavy admission is closed while draining; point and index are not.
	h := newTestHandler(t)
	if code, _, m := postToolCall(t, h, "search", map[string]any{"service": "x", "pattern": "y"}, ""); code != http.StatusServiceUnavailable || !strings.Contains(busyReason(m), "draining") {
		t.Fatalf("search while draining: %d %v", code, m)
	}
	if code, _, _ := postToolCall(t, h, "find_symbol", map[string]any{"service": "x", "name": "y"}, ""); code != http.StatusOK {
		t.Fatalf("find_symbol while draining: %d", code)
	}
	if drainBudget != 12*time.Second {
		t.Fatalf("drain budget %s, want 12 s (under ExitTimeOut 20)", drainBudget)
	}
	// Runtime fatals go to crash.log beside the ledger.
	setCrashOutput(ledger, func(m string) { t.Error(m) })
	defer debug.SetCrashOutput(nil, debug.CrashOptions{})
	if _, err := os.Stat(filepath.Join(filepath.Dir(ledger), "crash.log")); err != nil {
		t.Fatalf("crash.log: %v", err)
	}
	_ = io.Discard
}

// TestLifecycleModesInMetrics (§13.3): safe mode, warm-up and store
// distress are gauges on /metrics and fields of the /statusz tenancy block.
func TestLifecycleModesInMetrics(t *testing.T) {
	resetLifecycleModes(t)
	g := newGovernor(governorConfig{now: time.Now})
	activeGovernor.Store(g)
	defer activeGovernor.Store(nil)
	if m := governorMetricLines(); !strings.Contains(m, "fathom_safe_mode 0") || !strings.Contains(m, "fathom_store_distress 0") {
		t.Fatalf("metrics: %s", m)
	}
	applySafeMode(&tenancy.SafeModeInfo{Since: time.Now(), Reason: "r"}, nil)
	tenancy.SetWarmup(time.Now(), time.Minute)
	tenancy.NoteStoreDistress("disk full (SQLITE_FULL)", time.Now())
	m := governorMetricLines()
	for _, want := range []string{"fathom_safe_mode 1", "fathom_warmup 1", "fathom_store_distress 1"} {
		if !strings.Contains(m, want) {
			t.Errorf("metrics lack %q", want)
		}
	}
	s := g.snapshot()
	if !strings.Contains(s.SafeMode, "fathom safe mode") || s.WarmupUntil == "" || !strings.Contains(s.StoreDistress, "SQLITE_FULL") {
		t.Fatalf("snapshot = %+v", s)
	}
}

// TestServeHTTPWaitsForStoreWithoutExiting (RT-42, §8.4): a real
// serve-http binds before it opens the store and, while the store is
// unavailable, stays up in waiting-for-store — /healthz names the errno,
// tools/call is 503 -32050 "store unavailable" — writes one store_wait row
// (not counted), and serves once the store becomes reachable.
//
// Two real refusals of the store's directory, each in its own daemon. A
// mode-000 directory (EACCES) refuses a non-root daemon only — root's DAC
// override creates the store inside it — so that variant runs when euid
// != 0; a regular file where the store's directory should be (ENOTDIR)
// refuses every user, root included (CS-8: the cloud VMs run the suite as
// root), and always runs.
func TestServeHTTPWaitsForStoreWithoutExiting(t *testing.T) {
	t.Run("backoff", checkOpenStoreWithWaitBacksOff)
	for _, staging := range []struct {
		name, errno string
		needUser    bool
		block, free func(t *testing.T, dir string)
	}{
		{"a mode-000 directory", "EACCES", true,
			func(t *testing.T, dir string) {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(dir, 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			},
			func(t *testing.T, dir string) {
				if err := os.Chmod(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}},
		{"a file where the directory should be", "ENOTDIR", false,
			func(t *testing.T, dir string) {
				if err := os.WriteFile(dir, []byte("not a directory"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			func(t *testing.T, dir string) {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}},
	} {
		t.Run(staging.errno, func(t *testing.T) {
			if staging.needUser && os.Geteuid() == 0 {
				t.Skip("root creates the store inside a mode-000 directory; the ENOTDIR variant covers the wait for root")
			}
			checkServeHTTPWaitsForStore(t, staging.errno, staging.block, staging.free)
		})
	}
}

// checkServeHTTPWaitsForStore runs one real serve-http whose store sits
// under a directory block refused and free makes reachable, and pins the
// wait (named errno, 503s, one uncounted store_wait row) and the recovery.
func checkServeHTTPWaitsForStore(t *testing.T, errno string, block, free func(t *testing.T, dir string)) {
	home, _ := withIsolatedConfigHome(t)
	locked := filepath.Join(t.TempDir(), "locked")
	block(t, locked)
	addr := freeAddr(t)
	logPath := filepath.Join(t.TempDir(), "serve.log")
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), testHelperEnv+"=rt42-serve", "RT42_STORE="+filepath.Join(locked, "db", "s.sqlite"), "RT42_ADDR="+addr, "RT42_LOG="+logPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	logTail := func() string { b, _ := os.ReadFile(logPath); return string(b) }

	get := func(path string) (int, string) {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			return 0, err.Error()
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(waitSafety)
		for !ok() {
			select {
			case err := <-exited:
				t.Fatalf("serve-http exited (%v) while %s:\n%s", err, what, logTail())
			default:
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s:\n%s", what, logTail())
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitFor("waiting-for-store", func() bool {
		code, body := get("/healthz")
		return code == http.StatusServiceUnavailable && strings.Contains(body, "waiting-for-store") && strings.Contains(body, errno)
	})
	resp, err := http.Post("http://"+addr+"/rpc", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"find_symbol","arguments":{"name":"x"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(b), "store unavailable") || !strings.Contains(string(b), "-32050") {
		t.Fatalf("tools/call while waiting: %d %s", resp.StatusCode, b)
	}

	free(t, locked)
	waitFor("the store to open", func() bool {
		code, body := get("/healthz")
		return code == http.StatusOK && body == "ok"
	})
	ledger, _ := ledgerPathFor(home, "rt42")
	rows := readRowsT(t, ledger)
	if got := rowKinds(rows); got != "start,store_wait" {
		t.Fatalf("ledger = %s", got)
	}
	if rows[1].Errno != errno || rows[1].Label != storeWaitUnavailable {
		t.Fatalf("store_wait row = %+v", rows[1])
	}
	if v := tenancy.EvaluateBreaker(rows, time.Now(), tenancy.BreakerSettings{SafeAfter: 1}); v.Counted != 0 {
		t.Fatalf("store_wait counted: %+v", v)
	}

	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(waitSafety):
		t.Fatalf("no exit after SIGTERM:\n%s", logTail())
	}
	rows = readRowsT(t, ledger)
	if last := rows[len(rows)-1]; last.Row != tenancy.RowExit || last.Reason != tenancy.ExitSignal {
		t.Fatalf("exit row = %+v", last)
	}
}

// TestPortHolderWaitingForStoreStillAnswers (RT-43 with RT-42): a same-label
// predecessor that is waiting for its store answers /healthz 503
// "waiting-for-store: …" — it still serves the port, so its successor exits
// 0 instead of a counted port_held. Anything else that is not "ok" is not
// a fathom answering.
func TestPortHolderWaitingForStoreStillAnswers(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   bool
	}{
		{200, "ok", true},
		{503, "waiting-for-store: EPERM (tcc-wedge) — grant …", true},
		{503, "service unavailable", false},
		{200, "<html>", false},
		{404, "not found", false},
	} {
		if got := healthzIsFathom(c.status, []byte(c.body)); got != c.want {
			t.Errorf("healthzIsFathom(%d, %q) = %v, want %v", c.status, c.body, got, c.want)
		}
	}
}

// A holder with a huge command line (a test binary run with a long -run
// regex, as `tests select` emits) must not make the exit row overlong: the
// ledger reader skips a line over 64 KiB, which loses the exit record.
func TestPortHolderDetailClipsALongCommandLine(t *testing.T) {
	f := portHolderFacts{ListenerPID: 42, ListenerCmd: "fathom.test -test.run ^(" + strings.Repeat("TestSomething|", 6000) + ")$"}
	_, _, detail := resolvePortHolder(false, "", f)
	if !strings.Contains(detail, "pid 42 (fathom.test -test.run") {
		t.Fatalf("detail does not name the holder: %.200s", detail)
	}
	if len(detail) > 1024 {
		t.Fatalf("detail is %d bytes; the holder's command line must be clipped", len(detail))
	}
}
