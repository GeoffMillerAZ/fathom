package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
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
type rpcSpan struct{ start, end time.Time }

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
}

func newHangingDaemon(t *testing.T) *hangingDaemon {
	t.Helper()
	h := &hangingDaemon{release: make(chan struct{})}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		began := time.Now()
		h.requests.Add(1)
		if r.Header.Get(hookguard.DeadlineHeader) == "" {
			h.noHeader.Add(1)
		}
		select {
		case <-r.Context().Done():
		case <-h.release:
		}
		h.mu.Lock()
		h.spans = append(h.spans, rpcSpan{start: began, end: time.Now()})
		h.paths = append(h.paths, r.URL.Path)
		h.mu.Unlock()
	}))
	t.Cleanup(func() {
		close(h.release)
		h.srv.Close()
	})
	return h
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

// rpcBusyTime is how long at least one RPC was in flight: the union of the
// spans, so two concurrent hung RPCs count once (the deadline bounds the
// hook's waiting, not the number of requests it makes at once).
func rpcBusyTime(spans []rpcSpan) time.Duration {
	sorted := append([]rpcSpan(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].start.Before(sorted[j].start) })
	var busy time.Duration
	var curStart, curEnd time.Time
	for i, sp := range sorted {
		if i == 0 || sp.start.After(curEnd) {
			busy += curEnd.Sub(curStart)
			curStart, curEnd = sp.start, sp.end
			continue
		}
		if sp.end.After(curEnd) {
			curEnd = sp.end
		}
	}
	return busy + curEnd.Sub(curStart)
}

// TestNoHookRPCOutlivesItsDescriptorBudget is RT-7 (§11.1): every hook
// binding of every harness descriptor, run against a daemon that never
// answers, finishes every RPC by its deadline — descriptor TimeoutSec
// minus max(1 s, 20%) — and every RPC carries X-Fathom-Deadline. Budgets
// run at a tenth of real time (hookBudgetScale) so ~40 bindings take
// seconds; the formula itself is checked unscaled for each binding.
func TestNoHookRPCOutlivesItsDescriptorBudget(t *testing.T) {
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

			cpu0, cpuOK := processCPUTime()
			start := time.Now()
			code, _, elapsed := runHookVerb(t, verb, payload)
			cpu1, _ := processCPUTime()
			if code != 0 && verb[0] != "doctor" {
				t.Errorf("%s %q: exit %d, want 0 (I9)", d.ID, b.Verb, code)
			}
			n := int(h.requests.Load() - before)
			if n > 0 {
				withRPC++
			}
			spans := h.settledSpans(start, n)

			// The deadline property, in wall-clock time: a hanging daemon is
			// abandoned within the budget, measured from when that RPC
			// started. Nothing else bounds a hung request, and the deadline
			// is absolute, so neither queueing for a core before the RPC
			// nor a slow fork beforehand moves this line.
			for _, sp := range spans {
				if dur := sp.end.Sub(sp.start); dur > budget+hookRPCSlack {
					t.Errorf("%s %q: an RPC hung %v on the daemon; budget %v", d.ID, b.Verb, dur.Round(time.Millisecond), budget)
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
			rpcWait := rpcBusyTime(spans)
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
	if h.noHeader.Load() != 0 {
		t.Errorf("%d hook RPCs carried no %s", h.noHeader.Load(), hookguard.DeadlineHeader)
	}
	if bindings < 20 || withRPC < 5 {
		t.Fatalf("exercised %d bindings, %d reached the hanging daemon — the test did not test", bindings, withRPC)
	}
	t.Logf("%d bindings, %d reached the hanging daemon, %d RPCs total; worst CPU+RPC overrun %v of %v slack (%s)",
		bindings, withRPC, h.requests.Load(), worstOver.Round(time.Millisecond), hookLocalWorkSlack, worstWho)
}

// TestRPCBusyTimeCountsOverlapOnce pins the helper the whole-hook bound
// leans on: concurrent hung RPCs are one wait, a gap between two is not
// waiting, and no spans is no wait.
func TestRPCBusyTimeCountsOverlapOnce(t *testing.T) {
	t0 := time.Unix(0, 0)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	for _, c := range []struct {
		name  string
		spans []rpcSpan
		want  time.Duration
	}{
		{"none", nil, 0},
		{"one", []rpcSpan{{at(10), at(110)}}, 100 * time.Millisecond},
		{"concurrent", []rpcSpan{{at(0), at(100)}, {at(0), at(100)}, {at(20), at(60)}}, 100 * time.Millisecond},
		{"overlapping", []rpcSpan{{at(50), at(150)}, {at(0), at(100)}}, 150 * time.Millisecond},
		{"gap", []rpcSpan{{at(0), at(100)}, {at(300), at(350)}}, 150 * time.Millisecond},
	} {
		if got := rpcBusyTime(c.spans); got != c.want {
			t.Errorf("%s: busy %v, want %v", c.name, got, c.want)
		}
	}
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
