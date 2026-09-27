package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// hangingDaemon accepts every request and answers none until the client
// gives up; it records when each request ended and whether it carried
// X-Fathom-Deadline.
type hangingDaemon struct {
	srv      *httptest.Server
	release  chan struct{}
	mu       sync.Mutex
	ends     []time.Time
	paths    []string
	noHeader atomic.Int32
	requests atomic.Int32
}

func newHangingDaemon(t *testing.T) *hangingDaemon {
	t.Helper()
	h := &hangingDaemon{release: make(chan struct{})}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.requests.Add(1)
		if r.Header.Get(hookguard.DeadlineHeader) == "" {
			h.noHeader.Add(1)
		}
		select {
		case <-r.Context().Done():
		case <-h.release:
		}
		h.mu.Lock()
		h.ends = append(h.ends, time.Now())
		h.paths = append(h.paths, r.URL.Path)
		h.mu.Unlock()
	}))
	t.Cleanup(func() {
		close(h.release)
		h.srv.Close()
	})
	return h
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

// hookLocalWorkSlack is how far past its deadline a hook may run in
// TestNoHookRPCOutlivesItsDescriptorBudget: the local git and file work
// that follows the last RPC, which the time scale does not shrink. Measured
// at most ~120 ms under -race on the fixture repo (codex session-hook);
// 250 ms stays well inside the unscaled max(1 s, 20%) margin the deadline
// leaves before the harness kills the hook. Bounding local work on large
// repos is §11.3's fan-out rows, not the deadline's.
const hookLocalWorkSlack = 250 * time.Millisecond

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
			h.mu.Lock()
			h.ends = h.ends[:0]
			h.mu.Unlock()

			start := time.Now()
			code, _, elapsed := runHookVerb(t, verb, payload)
			if code != 0 && verb[0] != "doctor" {
				t.Errorf("%s %q: exit %d, want 0 (I9)", d.ID, b.Verb, code)
			}
			n := h.requests.Load() - before
			if n > 0 {
				withRPC++
			}
			// Every RPC ended by the deadline (+ scheduling slack).
			limit := start.Add(budget + 150*time.Millisecond)
			time.Sleep(20 * time.Millisecond) // let the server observe the last disconnect
			h.mu.Lock()
			for _, end := range h.ends {
				if end.After(limit) {
					t.Errorf("%s %q: an RPC ended %v after the hook started; budget %v", d.ID, b.Verb, end.Sub(start).Round(time.Millisecond), budget)
				}
			}
			h.mu.Unlock()
			// The whole hook ends by the deadline plus hookLocalWorkSlack:
			// no retry pile-up after it, and only a little local work.
			if elapsed > budget+hookLocalWorkSlack {
				t.Errorf("%s %q: hook took %v against a %v budget (%d RPCs)", d.ID, b.Verb, elapsed.Round(time.Millisecond), budget, n)
			}
		}
	}
	if h.noHeader.Load() != 0 {
		t.Errorf("%d hook RPCs carried no %s", h.noHeader.Load(), hookguard.DeadlineHeader)
	}
	if bindings < 20 || withRPC < 5 {
		t.Fatalf("exercised %d bindings, %d reached the hanging daemon — the test did not test", bindings, withRPC)
	}
	t.Logf("%d bindings, %d reached the hanging daemon, %d RPCs total", bindings, withRPC, h.requests.Load())
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
			code, out, elapsed := runHookVerb(t, v, payload)
			if code != 0 || out != "" {
				t.Errorf("%v: exit %d stdout %q, want silent 0", v, code, out)
			}
			if elapsed > 250*time.Millisecond {
				t.Errorf("%v: took %v while disabled, want an immediate return", v, elapsed)
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
