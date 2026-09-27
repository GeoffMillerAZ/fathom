package main

// lifecycle_gate.go — what the lifecycle modes refuse on the request path
// (design §2.1, §2.3, §3.4, §8.5; RT-39, RT-40, RT-41, RT-44): safe mode
// refuses tree and fleet; a drain closes tree, fleet and ingest; store
// distress refuses ingest and writes; warm-up gives each heavy class one
// slot. A refusal is HTTP 503 + Retry-After + JSON-RPC -32050 with a named
// reason — never an empty answer that looks complete (I49).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// rpcBusyCode is §3.4's JSON-RPC error code for a refusal before dispatch.
const rpcBusyCode = -32050

// draining is true once SIGTERM started the drain: heavy admission closed.
var draining atomic.Bool

// errDraining cancels in-flight heavy and ingest work at the drain.
var errDraining = errors.New("fathom is draining (SIGTERM): heavy work cancelled")

// lcRefusal is one refusal: the sentence, the class, how long to wait.
type lcRefusal struct {
	Reason     string
	Class      tenancy.Class
	RetryAfter time.Duration
}

// heavyClass reports tree and fleet.
func heavyClass(c tenancy.Class) bool { return c == tenancy.ClassTree || c == tenancy.ClassFleet }

// lifecycleRefusal is the mode check for one tools/call of class c.
func lifecycleRefusal(c tenancy.Class, now time.Time) *lcRefusal {
	if draining.Load() && (heavyClass(c) || c == tenancy.ClassIngest) {
		return &lcRefusal{Reason: fmt.Sprintf("draining: fathom is shutting down (SIGTERM); %s-class calls are closed", c), Class: c, RetryAfter: 30 * time.Second}
	}
	if heavyClass(c) {
		if s, on := tenancy.SafeModeNow(); on {
			return &lcRefusal{Reason: s.Message(), Class: c, RetryAfter: 60 * time.Second}
		}
	}
	if c == tenancy.ClassIngest {
		if why, on := tenancy.StoreDistressNow(now); on {
			return &lcRefusal{Reason: why + " — ingest and writes are refused until it clears; fathom resources", Class: c, RetryAfter: 60 * time.Second}
		}
	}
	return nil
}

// writeBusy writes §3.4's refusal: 503, Retry-After, -32050 {reason,
// class, level, retry_after_ms}.
func writeBusy(w http.ResponseWriter, id json.RawMessage, reason, class string, retry time.Duration) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    rpcBusyCode,
			"message": "fathom busy",
			"data": map[string]any{
				"reason":         reason,
				"class":          class,
				"level":          tenancy.CurrentLevel().String(),
				"retry_after_ms": retry.Milliseconds(),
			},
		},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(int((retry+time.Second-1)/time.Second)))
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write(body)
}

// rpcCallInfo parses the parts of a /rpc message the gate needs: the
// call's class and, for a batch, every sub-call's class (classes[0] is
// the call's own).
func rpcCallInfo(raw []byte) (id json.RawMessage, tool string, classes []tenancy.Class, ok bool) {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(raw, &msg) != nil || msg.Method != "tools/call" || msg.Params.Name == "" {
		return nil, "", nil, false
	}
	classes = []tenancy.Class{rpcToolClass(msg.Params.Name, msg.Params.Arguments)}
	if msg.Params.Name == "batch" {
		classes = append(classes, rpcBatchClasses(msg.Params.Arguments)...)
	}
	return msg.ID, msg.Params.Name, classes, true
}

// lifecycleGate runs before dispatch: the mode refusals, then the warm-up
// slot. It writes the refusal itself and reports false; on true the caller
// dispatches and calls release when done.
func lifecycleGate(ctx context.Context, w http.ResponseWriter, raw []byte, origin string) (release func(), ok bool) {
	id, tool, classes, isCall := rpcCallInfo(raw)
	if !isCall {
		return func() {}, true
	}
	now := time.Now()
	// A batch dispatches its sub-calls in process: each sub-call's class
	// is checked, so a batch never carries a tree, fleet or ingest call
	// past safe mode, a drain or store distress.
	for _, c := range classes {
		if ref := lifecycleRefusal(c, now); ref != nil {
			writeBusy(w, id, ref.Reason, c.String(), ref.RetryAfter)
			return nil, false
		}
	}
	class := classes[0]
	rel, ref := serveWarmupGate.acquire(ctx, class, tool, origin, now)
	if ref != nil {
		writeBusy(w, id, ref.Reason, class.String(), ref.RetryAfter)
		return nil, false
	}
	return rel, true
}

// ─── warm-up (RT-40) ──────────────────────────────────────────────────────

// warmupGate holds one slot per heavy class while tenancy.InWarmup. A
// call that cannot get the slot within wait (a hook: at once) is refused
// (503 + Retry-After, which a hook's breaker reads as busy, not a
// failure). Warm-up is a lifecycle bound, like safe mode's refusals: it
// holds under admission = "observe" (the default until RT-96) as under
// "enforce"; only "off" skips it.
type warmupGate struct {
	mode    func() string
	wait    time.Duration
	mu      sync.Mutex
	slots   map[tenancy.Class]chan struct{}
	holders map[tenancy.Class]string // the slot holder's tool, for the refusal
	logf    func(string)
}

func newWarmupGate(mode func() string, wait time.Duration) *warmupGate {
	return &warmupGate{mode: mode, wait: wait, slots: map[tenancy.Class]chan struct{}{}, holders: map[tenancy.Class]string{}}
}

// serveWarmupGate is serve-http's; its mode is the budget's admission.
var serveWarmupGate = newWarmupGate(func() string { return "observe" }, 5*time.Second)

func (g *warmupGate) slot(c tenancy.Class) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, ok := g.slots[c]
	if !ok {
		ch = make(chan struct{}, 1)
		g.slots[c] = ch
	}
	return ch
}

func (g *warmupGate) acquire(ctx context.Context, c tenancy.Class, tool, origin string, now time.Time) (func(), *lcRefusal) {
	mode := g.mode()
	if mode == "off" || !tenancy.InWarmup(now) || !(heavyClass(c) || c == tenancy.ClassIngest) {
		return func() {}, nil
	}
	start, d := tenancy.Warmup()
	slots := tenancy.WarmupSlots(map[tenancy.Class]int{c: 2}, start, now, d)
	if slots[c] != 1 {
		return func() {}, nil
	}
	ch := g.slot(c)
	var once sync.Once
	release := func() { once.Do(func() { <-ch }) }
	held := func() (func(), *lcRefusal) {
		g.mu.Lock()
		g.holders[c] = tool
		g.mu.Unlock()
		return release, nil
	}
	select {
	case ch <- struct{}{}:
		return held()
	default:
	}
	if origin != "hook" && g.wait > 0 {
		t := time.NewTimer(g.wait)
		defer t.Stop()
		select {
		case ch <- struct{}{}:
			return held()
		case <-ctx.Done():
		case <-t.C:
		}
	}
	g.mu.Lock()
	holder := firstNonEmpty(g.holders[c], "?")
	g.mu.Unlock()
	remain := d - now.Sub(start)
	reason := fmt.Sprintf("warm-up: %s-class holds 1 slot for %s after the start (in use: %s)", c, d, holder)
	if g.logf != nil {
		g.logf("tenancy: warm-up refused " + tool + ": " + reason)
	}
	return nil, &lcRefusal{Reason: reason, Class: c, RetryAfter: max(remain, time.Second)}
}

// ─── safe mode (RT-39) ────────────────────────────────────────────────────

// backgroundAllowed reports whether the console feeders may poll: not at
// shed and above, not in safe mode.
func backgroundAllowed() bool {
	if backgroundPaused.Load() {
		return false
	}
	_, safe := tenancy.SafeModeNow()
	return !safe
}

// applySafeMode enters safe mode (§2.3): tree and fleet are refused with
// the reason (lifecycleRefusal), auto-refresh's inline ingest is off
// (tenancy.InlineIngestAllowed), the feeders stop (backgroundAllowed) and
// the decision port closes (gatedDecider). point and index are served.
func applySafeMode(info *tenancy.SafeModeInfo, logf func(string)) {
	tenancy.SetSafeMode(info)
	if logf != nil && info != nil {
		logf(info.Message() + " — tree and fleet refused; auto-refresh, feeders and the decision port off; point and index served")
	}
}

// leaveSafeMode clears the breaker's safe mode.
func leaveSafeMode(logf func(string)) {
	tenancy.SetSafeMode(nil)
	if logf != nil {
		logf("lifecycle: safe mode ended after 30 min at normal — warm-up rules apply")
	}
}

// safeModeController steps the breaker's safe mode down after 30 min at
// normal (§2.3), writing a mode row and restarting warm-up.
type safeModeController struct {
	mu      sync.Mutex
	tracker tenancy.SafeModeTracker
	ledger  string
	pid     int
	version string
	warmup  time.Duration
	logf    func(string)
}

// newSafeModeController tracks the breaker's safe mode from this start's
// verdict (out.safe). paused.json's safe mode is the operator's: it ends
// with `fathom daemon resume`, never by stepping down, so it is not
// tracked and writes no mode row.
func newSafeModeController(out lifecycleOutcome, ledger string, pid int, version string, warmup time.Duration, logf func(string)) *safeModeController {
	sm := &safeModeController{ledger: ledger, pid: pid, version: version, warmup: warmup, logf: logf}
	sm.tracker.Safe = out.safe != nil
	return sm
}

func (s *safeModeController) step(now time.Time, level tenancy.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.tracker.Step(now, level) {
		return
	}
	leaveSafeMode(s.logf)
	tenancy.SetWarmup(now, s.warmup)
	if s.ledger != "" {
		row := tenancy.NewLifecycleRow(tenancy.RowMode, now, s.pid, s.version)
		row.From, row.To, row.Reason = tenancy.ModeSafe, tenancy.ModeNormal, "30 min at normal"
		if err := appendLifecycle(s.ledger, row); err != nil && s.logf != nil {
			s.logf(fmt.Sprintf("lifecycle: mode row not written: %v", err))
		}
	}
}

// gatedDecider closes the decision port in safe mode and at refuse and
// above (§2.1, §2.3). Its callers fail open on an error (I14).
type gatedDecider struct{ inner decision.Decider }

func (g gatedDecider) Name() string { return g.inner.Name() }

func (g gatedDecider) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if s, on := tenancy.SafeModeNow(); on {
		return decision.Response{}, fmt.Errorf("decision port closed: %s: %w", s.Message(), decision.ErrNoProvider)
	}
	if l := tenancy.CurrentLevel(); l >= tenancy.LevelRefuse {
		return decision.Response{}, fmt.Errorf("decision port closed: governor at %s: %w", l, decision.ErrNoProvider)
	}
	return g.inner.Decide(ctx, req)
}
