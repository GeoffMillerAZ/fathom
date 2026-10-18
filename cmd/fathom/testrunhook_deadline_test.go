package main

// testrunhook_deadline_test.go — EW-76 (G11, docs/bench/readings/
// efficiency-wave/hook-path-deadlines.md): the test-run hook asks the daemon
// no localize (its first candidate restates the failing test, and on an
// index this size the call needs a call-graph build no hook budget holds),
// its remaining calls run to the hook's own deadline rather than a fixed 2 s
// client cap under it, and a run that missed the deadline names why it said
// nothing (GL-24's vocabulary). No wall-clock wait: the client and the
// request deadline are observed, and the hanging daemon runs at a hundredth
// of the real budget.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type testRunRoundTrip func(*http.Request) (*http.Response, error)

func (f testRunRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testImpactAnswer(w http.ResponseWriter) {
	text, _ := json.Marshal(`{"tests":["shop::money/money.TestRound"]}`)
	fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, text)
}

func TestTestRunCallsRunToTheHookDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { testImpactAnswer(w) }))
	defer srv.Close()
	gotTimeout := time.Duration(-1)
	var gotDeadline time.Time
	orig := testRunHTTPClient
	testRunHTTPClient = func() *http.Client {
		c := orig()
		gotTimeout = c.Timeout
		c.Transport = testRunRoundTrip(func(r *http.Request) (*http.Response, error) {
			gotDeadline, _ = r.Context().Deadline()
			return http.DefaultTransport.RoundTrip(r)
		})
		return c
	}
	defer func() { testRunHTTPClient = orig }()

	deadline := time.Now().Add(3500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	idx := daemonTestRunIndex{ctx: ctx, baseURL: srv.URL, service: "shop"}
	tests, err := idx.TestImpact("money/money.Round")
	if err != nil || len(tests) != 1 {
		t.Fatalf("test_impact: %v %v", tests, err)
	}
	if gotTimeout != 0 {
		t.Errorf("tier 1's client carries its own %v timeout, want none: the hook's deadline bounds the call", gotTimeout)
	}
	if !gotDeadline.Equal(deadline) {
		t.Errorf("the request's deadline is %v, want the hook's %v (a fixed cap under it abandoned 385 of 437 fleet calls)", gotDeadline, deadline)
	}
}

// testRunIdentityDaemon resolves repo to its service over GET /identity,
// counts the localize calls it is asked into *localized, and answers every
// tool call with an empty payload.
func testRunIdentityDaemon(t *testing.T, repo string, localized *atomic.Int64) *httptest.Server {
	t.Helper()
	prevDir := identCacheDir
	identCacheDir = t.TempDir()
	t.Cleanup(func() { identCacheDir = prevDir })
	name := strings.ToLower(filepath.Base(repo))
	h := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
		return map[string]svcInfo{name: {symbols: 42, langs: "go", path: repo}}, nil
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/identity":
			h.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/rpc"):
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), `"localize"`) {
				localized.Add(1)
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{}"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestTestRunHookAsksNoLocalize: a failing run reached by no changed symbol
// gets its line from the run alone; the daemon is never asked localize, whose
// first candidate for a failing_test seed is the failing test itself (the
// controlled run's fixed seed: "named TestDaemonTestRunIndexQualifiesService",
// score 8, over every graph hop) and which at 99aa205a was abandoned in 385
// of the fleet's 437 test-run calls.
func TestTestRunHookAsksNoLocalize(t *testing.T) {
	var localized atomic.Int64
	repo := testRunRepo(t, true)
	srv := testRunIdentityDaemon(t, repo, &localized)
	code, out := runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))
	if code != 0 || !strings.Contains(out, "TestRound") {
		t.Fatalf("exit %d output %q: want the line naming the failing test", code, out)
	}
	if n := localized.Load(); n != 0 {
		t.Errorf("the hook asked localize %d times, want none on the hook path", n)
	}
	if strings.Contains(out, "localize:") {
		t.Errorf("line %q carries a localize clause", out)
	}
	rows := testRunBeats(t)
	if len(rows) != 1 || rows[0]["emitted"] != "true" {
		t.Errorf("rows = %v, want one emitted row", rows)
	}
}

func TestTestRunMissedDeadlineNamesItsSilence(t *testing.T) {
	orig := hookBudgetScale
	hookBudgetScale = 0.01
	defer func() { hookBudgetScale = orig }()
	h := newHangingDaemon(t)
	repo := testRunRepo(t, true)
	code, out := runTestRunHook(t, "failed-test-run", repo, h.srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))
	if code != 0 || out != "" {
		t.Fatalf("past the deadline: exit %d output %q, want 0 and silence", code, out)
	}
	rows := testRunBeats(t)
	if len(rows) != 1 || rows[0]["deadline"] != "missed" || rows[0]["emitted"] != "false" || rows[0]["reason"] != string(silenceDaemonError) {
		t.Errorf("rows = %v, want one row marked deadline missed, nothing emitted, reason %s", rows, silenceDaemonError)
	}
}

// flipDeadlineCtx is a context whose deadline passes while the work runs
// but whose own timer has not fired: Err() stays nil. A second timer set to
// the same instant (the hook guard's transport) opens exactly this window,
// and on loaded CI runners the late line was injected (runs 38914891136,
// 38978841572, 39105586242).
type flipDeadlineCtx struct {
	context.Context
	passed *atomic.Bool
}

func (c flipDeadlineCtx) Deadline() (time.Time, bool) {
	if c.passed.Load() {
		return time.Now().Add(-time.Millisecond), true
	}
	return time.Now().Add(time.Hour), true
}

func TestRunTestRunWithDeadline_LateLineBeforeItsTimerIsDropped(t *testing.T) {
	var passed atomic.Bool
	ctx := flipDeadlineCtx{Context: context.Background(), passed: &passed}
	line, missed := runTestRunWithDeadline(ctx, func(context.Context) string {
		passed.Store(true)
		return "late"
	})
	if line != "" || !missed {
		t.Fatalf("work finished past the deadline: got %q missed=%v, want silence and missed", line, missed)
	}
}
