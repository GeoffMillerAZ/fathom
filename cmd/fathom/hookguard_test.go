package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// rpcSpan is one hung request as the daemon saw it: when it arrived and
// when the client gave up.
type rpcSpan struct {
	start, end time.Time
	// carried is the remaining budget the request's X-Fathom-Deadline
	// header declared (hasCarried false: absent or unparsable); cancelled
	// is whether the server saw the client cancel (not the test's release).
	carried    time.Duration
	hasCarried bool
	cancelled  bool
}

// hangingDaemon accepts every request and answers none until the client
// gives up; it records when each request arrived and ended and whether it
// carried X-Fathom-Deadline.
type hangingDaemon struct {
	srv      *httptest.Server
	release  chan struct{}
	mu       sync.Mutex
	spans    []rpcSpan
	paths    []string
	noHeader atomic.Int32
	requests atomic.Int32
	// phase names what the test is doing when a request arrives (which
	// binding is running, or which one last returned), and bare holds one
	// line per request that carried no X-Fathom-Deadline: its method, path,
	// phase and the in-process client call sites that sent it.
	phase atomic.Pointer[string]
	bare  []string
}

// setPhase records what the test is doing, for naming a bare request.
func (h *hangingDaemon) setPhase(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	h.phase.Store(&s)
}

// bareRequests is the line per request that carried no deadline header.
func (h *hangingDaemon) bareRequests() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.bare...)
}

func newHangingDaemon(t *testing.T) *hangingDaemon {
	t.Helper()
	h := &hangingDaemon{release: make(chan struct{})}
	h.setPhase("before any binding ran")
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		began := time.Now()
		h.requests.Add(1)
		if r.Header.Get(hookguard.DeadlineHeader) == "" {
			h.noHeader.Add(1)
			line := fmt.Sprintf("%s %s %s; sent by %s", r.Method, r.URL.Path, *h.phase.Load(), inFlightClientCallers())
			h.mu.Lock()
			h.bare = append(h.bare, line)
			h.mu.Unlock()
		}
		var carried time.Duration
		var hasCarried bool
		if ms, err := strconv.ParseInt(r.Header.Get(hookguard.DeadlineHeader), 10, 64); err == nil && ms >= 0 {
			carried, hasCarried = time.Duration(ms)*time.Millisecond, true
		}
		cancelled := false
		select {
		case <-r.Context().Done():
			cancelled = true
		case <-h.release:
		}
		h.mu.Lock()
		h.spans = append(h.spans, rpcSpan{start: began, end: time.Now(), carried: carried, hasCarried: hasCarried, cancelled: cancelled})
		h.paths = append(h.paths, r.URL.Path)
		h.mu.Unlock()
	}))
	t.Cleanup(func() {
		close(h.release)
		h.srv.Close()
	})
	return h
}

// inFlightClientCallers names the fathom call sites of every goroutine in
// this process that is waiting on an HTTP round trip — the hook runs in
// process, so the request the daemon is serving is among them. A round
// trip that did not pass the hook guard (no (*Guard).roundTrip frame) is
// listed first and marked unguarded: that is the request a hook sent
// with no guard active.
func inFlightClientCallers() string {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	var unguarded, guarded []string
	for _, g := range strings.Split(string(buf), "\n\n") {
		if !strings.Contains(g, "net/http.(*Transport).roundTrip") {
			continue
		}
		var frames []string
		lines := strings.Split(g, "\n")
		for i := 1; i+1 < len(lines) && len(frames) < 5; i += 2 {
			fn := lines[i]
			if !strings.HasPrefix(fn, "main.") && !strings.HasPrefix(fn, "github.com/geoffmilleraz/fathom/") {
				continue
			}
			if strings.Contains(fn, "internal/hookguard.") || strings.HasPrefix(fn, "main.inFlightClientCallers") {
				continue
			}
			if p := strings.IndexByte(fn, '('); p > 0 {
				fn = fn[:p]
			}
			loc := strings.TrimSpace(lines[i+1])
			if sp := strings.IndexByte(loc, ' '); sp > 0 {
				loc = loc[:sp]
			}
			frames = append(frames, fn+" "+filepath.Base(loc))
		}
		if len(frames) == 0 {
			continue // net/http's own helpers
		}
		site := strings.Join(frames, " < ")
		if strings.Contains(g, "hookguard.(*Guard).roundTrip") {
			guarded = append(guarded, site)
		} else {
			unguarded = append(unguarded, "UNGUARDED "+site)
		}
	}
	all := append(unguarded, guarded...)
	if len(all) == 0 {
		return "no in-process client (another process sent it)"
	}
	return strings.Join(all, " | ")
}

// settledSpans returns the spans of the n requests a hook run made since
// it started at `since`, once the server has observed the last client
// disconnect (bounded: a span never seen is simply not counted).
func (h *hangingDaemon) settledSpans(since time.Time, n int) []rpcSpan {
	var got []rpcSpan
	for deadline := time.Now().Add(waitSafety); ; time.Sleep(5 * time.Millisecond) {
		h.mu.Lock()
		got = got[:0]
		for _, sp := range h.spans {
			if !sp.start.Before(since) {
				got = append(got, sp)
			}
		}
		h.mu.Unlock()
		if len(got) >= n || time.Now().After(deadline) {
			return got
		}
	}
}

// hookStdinPayload is a Claude-Code-shaped envelope carrying every field
// any hook verb reads; other harnesses' codecs read what they can.
func hookStdinPayload(repo string) string {
	return fmt.Sprintf(`{"session_id":"rt7-session","cwd":%q,"hook_event_name":"x","source":"startup","reason":"exit","trigger":"manual",`+
		`"tool_name":"Edit","tool_input":{"file_path":%q,"command":"echo hi > out.txt","new_string":"x","old_string":"y"},`+
		`"prompt":"where is fetchServices defined and who calls buildSessionContext?","transcript_path":%q}`,
		repo, filepath.Join(repo, "main.go"), filepath.Join(repo, "transcript.jsonl"))
}

// isolateHookRun points every machine-shared hook cache at fresh temp dirs
// so one case's breaker or identity answer cannot short-circuit the next.
func isolateHookRun(t *testing.T) {
	t.Helper()
	origHealth, origIdent := hookcache.HealthDir, identCacheDir
	hookcache.HealthDir = filepath.Join(t.TempDir(), "health")
	identCacheDir = filepath.Join(t.TempDir(), "ident")
	t.Cleanup(func() { hookcache.HealthDir, identCacheDir = origHealth, origIdent })
}

func hookRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitInitTempRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// runHookVerb runs one wired verb in process with the payload on both
// stdin seams (session-hook takes its reader; the others read os.Stdin).
func runHookVerb(t *testing.T, verb []string, payload string) (code int, stdout string, elapsed time.Duration) {
	t.Helper()
	restore := pipeStdin(t, payload)
	defer restore()
	var out, errb bytes.Buffer
	start := time.Now()
	code = runWithStdin(verb, strings.NewReader(payload), &out, &errb)
	return code, out.String(), time.Since(start)
}

// hookLocalWorkSlack is how much local work a hook may spend beyond its
// deadline in TestNoHookRPCOutlivesItsDescriptorBudget: writing its
// records and its answer after the last RPC, which the time scale does not
// shrink. It bounds CPU time (the hook's own plus its reaped git
// children's), never wall time — see the whole-hook check below. Past a
// spent deadline hooks skip their optional local work (hookDeadlineSpent):
// measured under -race on the fixture repo, the median overrun fell from
// 101 ms to 2 ms (doctor --brief) and 91 ms to 15 ms (codex session-hook).
// 250 ms stays well inside the unscaled max(1 s, 20%) margin the deadline
// leaves before the harness kills the hook. Bounding local work before the
// deadline on large repos is §11.3's fan-out rows, not the deadline's.
const hookLocalWorkSlack = 250 * time.Millisecond

// hookRPCSlack is how long past its budget one hung RPC may still be open
// on the daemon's side: the client gives up at the deadline, and the
// server then has to notice the disconnect.
const hookRPCSlack = 150 * time.Millisecond

// TestNoHookRPCOutlivesItsDescriptorBudget is RT-7 (§11.1): every hook
// binding of every harness descriptor, run against a daemon that never
// answers, finishes every RPC by its deadline — descriptor TimeoutSec
// minus max(1 s, 20%) — and every RPC carries X-Fathom-Deadline. Budgets
// run at a tenth of real time (hookBudgetScale) so ~40 bindings take
// seconds; the formula itself is checked unscaled for each binding.
func TestNoHookRPCOutlivesItsDescriptorBudget(t *testing.T) {
	// The hook's CPU is read for the whole process, so goroutines that earlier
	// tests in a shard leave running (a background typed-edge pass) count as
	// the hook's work and fail it under load. Measure in a fresh process that
	// runs only this test; every assertion below is unchanged.
	if os.Getenv("FATHOM_HOOKGUARD_ISOLATED") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNoHookRPCOutlivesItsDescriptorBudget$", "-test.count=1", "-test.v")
		cmd.Env = append(os.Environ(), "FATHOM_HOOKGUARD_ISOLATED=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated run: %v\n%s", err, out)
		}
		return
	}
	const scale = 0.1
	orig := hookBudgetScale
	hookBudgetScale = scale
	defer func() { hookBudgetScale = orig }()

	h := newHangingDaemon(t)
	t.Setenv("FATHOM_HTTP", h.srv.URL)
	repo := hookRepo(t)
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	payload := hookStdinPayload(repo)

	seen := map[string]bool{}
	var bindings, withRPC int
	var worstOver time.Duration // the most any hook's CPU + RPC wait ran past its budget
	var worstWho string
	var worstLeft time.Duration // the most CPU any hook's abandoned work used after it returned
	var worstLeftWho string
	for _, d := range harness.All() {
		for _, b := range d.Hooks {
			if b.TimeoutSec <= 0 || seen[d.ID+"\x00"+b.Verb] {
				continue
			}
			seen[d.ID+"\x00"+b.Verb] = true
			bindings++

			// The unscaled contract: the guard's timeout for this binding is
			// no larger than the descriptor's, and the budget leaves at
			// least max(1 s, 20%) for the hook to exit.
			ts := hookTimeoutSec(d.ID, b.Marker)
			if ts <= 0 || ts > b.TimeoutSec {
				t.Errorf("%s %q: guard timeout %ds, descriptor %ds", d.ID, b.Verb, ts, b.TimeoutSec)
				continue
			}
			full := hookguard.HookDeadline(time.Now(), ts).Budget()
			total := time.Duration(b.TimeoutSec) * time.Second
			if margin := total - full; margin < time.Second || margin < total/5 {
				t.Errorf("%s %q: budget %v leaves %v of %v", d.ID, b.Verb, full, margin, total)
			}

			isolateHookRun(t)
			verb := strings.Fields(b.Verb)
			if d.ID != harness.ClaudeCode && !strings.Contains(b.Verb, "--harness") && verb[0] != "doctor" && verb[0] != "config" {
				verb = append(verb, "--harness", d.ID)
			}
			budget := time.Duration(float64(full) * scale)
			before := h.requests.Load()

			goroutines0 := runtime.NumGoroutine()
			cpu0, cpuOK := processCPUTime()
			start := time.Now()
			h.setPhase("while %s %q ran", d.ID, b.Verb)
			code, _, elapsed := runHookVerb(t, verb, payload)
			h.setPhase("after %s %q returned (%v in)", d.ID, b.Verb, elapsed.Round(time.Millisecond))
			cpu1, _ := processCPUTime()
			// A hook may return at its deadline with work still running:
			// tenancy.RunUntilDone abandons it, and in production os.Exit
			// ends it the moment the hook returns. In process nothing ends
			// it, so it ran inside the NEXT binding's CPU window and was
			// charged there: on v0.18.0's CI, doctor --brief's abandoned
			// probes put 469 ms on claude-code prompt-hook, the binding after
			// it, whose own CPU is 5-10 ms. Wait for
			// what this hook left to finish before the next binding starts,
			// as the process boundary would; that CPU is charged to no hook,
			// because in production it is never spent.
			if left := settleHookGoroutines(goroutines0, total); left > worstLeft {
				worstLeft, worstLeftWho = left, d.ID+" "+b.Verb
			}
			if code != 0 && verb[0] != "doctor" {
				t.Errorf("%s %q: exit %d, want 0 (I9)", d.ID, b.Verb, code)
			}
			n := int(h.requests.Load() - before)
			if n > 0 {
				withRPC++
			}
			spans := h.settledSpans(start, n)

			// The deadline property, asserted on what the RPC carries and
			// what the daemon observes, never on elapsed wall time (a loaded
			// machine delays the server noticing a disconnect, which is
			// neither a late client nor a hung hook): every RPC declares a
			// remaining budget no larger than the hook's, and the daemon saw
			// the client cancel it (the hanging daemon only releases at test
			// end). A request with no deadline carried is never cancelled by
			// the client in time and fails both lines.
			for _, sp := range spans {
				if !sp.hasCarried || sp.carried > budget+hookRPCSlack {
					t.Errorf("%s %q: an RPC carried deadline %v (present %v); budget %v", d.ID, b.Verb, sp.carried, sp.hasCarried, budget)
				}
				if !sp.cancelled {
					t.Errorf("%s %q: an RPC was never cancelled by the client; it outlived its budget %v", d.ID, b.Verb, budget)
				}
			}

			// The whole hook ends by the deadline plus hookLocalWorkSlack: no
			// retry pile-up after it, and only a little local work. "Local
			// work" is CPU time, not wall time. The hook's git forks are
			// slow to START under load (load average ~12 on 10 cores took
			// codex's session-end-hook, which is nothing BUT local work, to
			// 701 ms against a 200 ms budget with no RPC at all), and that
			// wait is neither a hung daemon nor a spinning hook — it costs
			// the hook nothing. So the bound is what the hook consumed (its
			// own CPU plus its reaped children's) plus the wall time it
			// really spent waiting on the daemon: a spin, a retry pile-up or
			// a chain of forks that does real work past the budget still
			// fails it, however idle the machine.
			// The wait on the daemon is what the carried deadlines allow, not
			// the measured wall span: the deadline is absolute, so a retry
			// carries what is left and the union of the waits is bounded by
			// the longest declared budget.
			var rpcWait time.Duration
			for _, sp := range spans {
				if sp.carried > rpcWait {
					rpcWait = sp.carried
				}
			}
			if cpuOK {
				local := cpu1 - cpu0
				if over := local + rpcWait - budget; over > worstOver {
					worstOver, worstWho = over, d.ID+" "+b.Verb
				}
				if local+rpcWait > budget+hookLocalWorkSlack {
					t.Errorf("%s %q: hook used %v CPU + waited %v on RPCs against a %v budget (+%v slack) (%d RPCs; wall %v)",
						d.ID, b.Verb, local.Round(time.Millisecond), rpcWait.Round(time.Millisecond), budget, hookLocalWorkSlack, n, elapsed.Round(time.Millisecond))
				}
			} else if elapsed > budget+hookLocalWorkSlack { // no CPU reading on this platform
				t.Errorf("%s %q: hook took %v against a %v budget (%d RPCs)", d.ID, b.Verb, elapsed.Round(time.Millisecond), budget, n)
			}

			// The backstop, in wall-clock time: a hook that neither spins nor
			// waits on the daemon but simply sits past its deadline (a sleep,
			// a lock, a blocked read) uses no CPU and holds no RPC, so only
			// the clock sees it. Below the harness's own kill time the clock
			// cannot tell that from a machine that was slow to run the hook,
			// so this line is the descriptor's whole TimeoutSec, unscaled:
			// past it the harness would have killed the hook, which is the
			// failure this test exists to prevent, and no load explains it.
			if elapsed > total {
				t.Errorf("%s %q: hook ran %v against a %v budget: past the %v the harness allows before it kills the hook",
					d.ID, b.Verb, elapsed.Round(time.Millisecond), budget, total)
			}
		}
	}
	if n := h.noHeader.Load(); n != 0 {
		t.Errorf("%d hook RPCs carried no %s:\n\t%s", n, hookguard.DeadlineHeader, strings.Join(h.bareRequests(), "\n\t"))
	}
	if bindings < 20 || withRPC < 5 {
		t.Fatalf("exercised %d bindings, %d reached the hanging daemon — the test did not test", bindings, withRPC)
	}
	t.Logf("%d bindings, %d reached the hanging daemon, %d RPCs total; worst CPU+RPC overrun %v of %v slack (%s); most CPU after a hook returned %v (%s)",
		bindings, withRPC, h.requests.Load(), worstOver.Round(time.Millisecond), hookLocalWorkSlack, worstWho, worstLeft.Round(time.Millisecond), worstLeftWho)
}

// TestDoctorBriefSendsNoRPCAfterItReturns pins the CI flake of RT-7 above
// ("1 hook RPCs carried no X-Fathom-Deadline", main at 4f6514927): doctor
// --brief's capabilities row read the console's observatory model, whose
// cold-cache rebuild is a detached flight bound to nothing the hook
// controls. When that flight reached its list_services read after the
// hook had returned — the hook's guard already restored to none — the read
// went out bare: no deadline header, no hook deadline, only its own 5 s
// client cap. In production os.Exit ends abandoned work the moment the hook
// returns, so a hook's work reaches the daemon only while the hook runs.
// The model build is held here until doctor has returned, which is the
// loaded-runner timing made certain.
func TestDoctorBriefSendsNoRPCAfterItReturns(t *testing.T) {
	isolateHookRun(t)
	hermeticObsEnv(t)
	t.Cleanup(resetObsCache)
	t.Setenv("HOME", t.TempDir())
	orig := hookBudgetScale
	hookBudgetScale = 0.1
	defer func() { hookBudgetScale = orig }()
	h := newHangingDaemon(t)
	t.Setenv("FATHOM_HTTP", h.srv.URL)
	repo := hookRepo(t)
	t.Setenv("CLAUDE_PROJECT_DIR", repo)

	entered, gate, built := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	prev := obsModelBuilder
	obsModelBuilder = func(ctx context.Context, baseURL string, now time.Time) observatoryModel {
		entered <- struct{}{}
		<-gate
		defer close(built)
		return buildObservatoryModelCtx(ctx, baseURL, now)
	}
	t.Cleanup(func() { obsModelBuilder = prev })

	h.setPhase("while doctor --brief ran")
	runHookVerb(t, []string{"doctor", "--brief", "--path", repo}, hookStdinPayload(repo))
	h.setPhase("after doctor --brief returned")
	afterReturn := h.requests.Load()
	select {
	case <-entered:
	case <-time.After(waitSafety):
		t.Fatal("doctor --brief never read the observatory model: the test did not test")
	}
	close(gate)
	select {
	case <-built:
	case <-time.After(waitSafety):
		t.Fatal("the held observatory model build never finished")
	}
	if n := h.requests.Load() - afterReturn; n != 0 {
		t.Errorf("%d RPCs reached the daemon after doctor --brief returned", n)
	}
	if n := h.noHeader.Load(); n != 0 {
		t.Errorf("%d RPCs carried no %s:\n\t%s", n, hookguard.DeadlineHeader, strings.Join(h.bareRequests(), "\n\t"))
	}
}

// settleHookGoroutines waits until the goroutines a hook left running when
// it returned have exited (the count is back to baseline, what it was before
// the hook started), bounded by limit, and returns the CPU the process used
// meanwhile. Past limit it stops waiting: the harness would have killed the
// hook process by then.
func settleHookGoroutines(baseline int, limit time.Duration) time.Duration {
	cpu0, _ := processCPUTime()
	for deadline := time.Now().Add(limit); runtime.NumGoroutine() > baseline && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
	}
	cpu1, _ := processCPUTime()
	return cpu1 - cpu0
}

// TestHooksSilentWhenDisabled is RT-10 (§9): with disabled.json (or
// paused.json) naming the managed daemon, every hook verb returns 0 with
// no output and sends no RPC; a switch naming a different daemon silences
// nothing.
func TestHooksSilentWhenDisabled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_HTTP", srv.URL)
	repo := hookRepo(t)
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	payload := hookStdinPayload(repo)
	daemonDir := filepath.Join(home, ".fathom", "daemon")
	if err := os.MkdirAll(daemonDir, 0o755); err != nil {
		t.Fatal(err)
	}

	verbs := [][]string{}
	d, _ := harness.Lookup(harness.ClaudeCode)
	for _, b := range d.Hooks {
		verbs = append(verbs, strings.Fields(b.Verb))
	}
	verbs = append(verbs, []string{"post-commit-hook", "--dir", repo, "--quiet"}, []string{"doctor", "--brief", "--path", repo})

	runAll := func(t *testing.T) {
		t.Helper()
		for _, v := range verbs {
			isolateHookRun(t)
			// "An immediate return" is the silent exit 0 with no RPC sent: the
			// daemon's hit count below is the proof, not a stopwatch.
			code, out, _ := runHookVerb(t, v, payload)
			if code != 0 || out != "" {
				t.Errorf("%v: exit %d stdout %q, want silent 0", v, code, out)
			}
		}
	}
	for _, sw := range []string{"disabled.json", "paused.json"} {
		t.Run(sw, func(t *testing.T) {
			p := filepath.Join(daemonDir, sw)
			if err := os.WriteFile(p, []byte(fmt.Sprintf(`{"by":"test","reason":"rt-10","addr":%q}`, srv.URL)), 0o644); err != nil {
				t.Fatal(err)
			}
			defer os.Remove(p)
			hits.Store(0)
			runAll(t)
			if n := hits.Load(); n != 0 {
				t.Fatalf("%d RPCs reached the daemon while %s named it", n, sw)
			}
		})
	}
	t.Run("empty disabled.json at the default managed URL", func(t *testing.T) {
		p := filepath.Join(daemonDir, "disabled.json")
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(p)
		t.Setenv("FATHOM_HTTP", "http://localhost:"+managedPort)
		hits.Store(0)
		isolateHookRun(t)
		code, out, _ := runHookVerb(t, []string{"session-hook"}, payload)
		if code != 0 || out != "" {
			t.Fatalf("session-hook under an empty disabled.json: exit %d stdout %q", code, out)
		}
	})
	t.Run("a switch naming another daemon silences nothing", func(t *testing.T) {
		p := filepath.Join(daemonDir, "disabled.json")
		if err := os.WriteFile(p, []byte(`{"reason":"managed only","addr":"127.0.0.1:1"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(p)
		hits.Store(0)
		for _, v := range verbs {
			isolateHookRun(t)
			runHookVerb(t, v, payload)
		}
		if hits.Load() == 0 {
			t.Fatal("no hook reached an unmanaged daemon: the switch silenced a URL it does not govern")
		}
	})
}

// A busy daemon (503 + Retry-After) holds the hook breaker, but the
// session primer must not call a busy daemon "not responding".
func TestSessionContextSaysBusyNotDown(t *testing.T) {
	isolateHookRun(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	g := &hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now(), 10)}
	restore := activateHookGuard(g)
	defer restore()
	// The first call meets the 503 and opens the breaker as busy; the
	// second is held off by it.
	_ = buildSessionContext(srv.URL, t.TempDir())
	got := buildSessionContext(srv.URL, t.TempDir())
	if !strings.Contains(got, "is busy right now") || strings.Contains(got, "not responding") {
		t.Fatalf("busy daemon primer = %q", got)
	}
	if g.Skips()[hookguard.Busy] == 0 {
		t.Fatalf("the busy breaker skipped nothing: %v", g.Skips())
	}
}
