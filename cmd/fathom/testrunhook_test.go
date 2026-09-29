package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testrun"
)

// testRunFailingOutput is a real `go test` failure (internal/testrun's
// fixture, trimmed) whose assertion message carries a marker no log row
// or line may ever repeat.
const testRunFailingOutput = "--- FAIL: TestRound (0.00s)\n" +
	"    money_test.go:7: Round(15) = 10, want 20 SECRET-OUTPUT-LINE-42\n" +
	"FAIL\n" +
	"FAIL\texample.com/shop/money\t0.140s\n" +
	"FAIL\n"

const testRunPassingOutput = "ok  \texample.com/shop/cart\t0.127s\n?   \texample.com/shop/empty\t[no test files]\nok  \texample.com/shop/money\t0.129s\n"

// testRunRepo is a git repository whose Taskfile declares `task test`
// (running `go test ./...`), with the test-run hook switched on when on.
// HOME and the hooks log are private to the test.
func testRunRepo(t *testing.T, on bool) string {
	t.Helper()
	isolateHookRun(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	repo := t.TempDir()
	gitInitTempRepo(t, repo)
	writeRepoFile(t, repo, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	if on {
		writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
	}
	return repo
}

// countingDaemon answers 404 to everything and counts the requests.
func countingDaemon(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func testRunPostPayload(repo, command, output string) string {
	resp, _ := json.Marshal(map[string]any{"stdout": output, "stderr": "", "interrupted": false, "isImage": false, "noOutputExpected": false})
	return fmt.Sprintf(`{"session_id":"tr-session","cwd":%q,"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":%q},"tool_response":%s,"tool_use_id":"t1","duration_ms":900}`,
		repo, command, resp)
}

func testRunFailurePayload(repo, command, output string) string {
	errText, _ := json.Marshal("Exit code 1\n" + output)
	return fmt.Sprintf(`{"session_id":"tr-session","cwd":%q,"hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_input":{"command":%q},"tool_use_id":"t2","error":%s,"is_interrupt":false,"duration_ms":900}`,
		repo, command, errText)
}

func runTestRunHook(t *testing.T, event, repo, url, payload string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runWithStdin([]string{"session-hook", "--event", event, "--url", url, "--dir", repo}, strings.NewReader(payload), &out, &errb)
	return code, out.String()
}

func testRunBeats(t *testing.T) []map[string]string {
	t.Helper()
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	var out []map[string]string
	for _, b := range beats {
		if b.Channel == testRunChannel {
			m := map[string]string{"kind": b.Kind}
			for k, v := range b.Meta {
				m[k] = v
			}
			out = append(out, m)
		}
	}
	return out
}

// TestTestRunHookSilentForNonMatchingCommand pins TR-2: the hook rides
// PostToolUse(Bash) and PostToolUseFailure(Bash) (`session-hook --event
// test-run` and `--event failed-test-run`), and a Bash call that does not
// run a declared test command — or any call while `[hooks] test_run` is
// off — exits with no output, no log row and no daemon call at all.
func TestTestRunHookSilentForNonMatchingCommand(t *testing.T) {
	srv, n := countingDaemon(t)

	t.Run("not a declared test command", func(t *testing.T) {
		repo := testRunRepo(t, true)
		for _, tc := range []struct{ event, payload string }{
			{"test-run", testRunPostPayload(repo, "ls -la && grep -rn TestRound .", testRunFailingOutput)},
			{"failed-test-run", testRunFailurePayload(repo, "cat missing.txt", testRunFailingOutput)},
			{"test-run", testRunPostPayload(repo, "go vet ./...", testRunFailingOutput)},
			{"test-run", testRunPostPayload(repo, `echo "go test ./..."`, testRunFailingOutput)},
		} {
			before := n.Load()
			code, out := runTestRunHook(t, tc.event, repo, srv.URL, tc.payload)
			if code != 0 || out != "" {
				t.Errorf("%s: exit %d, output %q; want 0 and silence", tc.event, code, out)
			}
			if got := n.Load() - before; got != 0 {
				t.Errorf("%s: %d daemon requests for a command that runs no declared test", tc.event, got)
			}
		}
		if rows := testRunBeats(t); len(rows) != 0 {
			t.Errorf("log rows for non-matching commands: %v", rows)
		}
	})

	t.Run("switched off", func(t *testing.T) {
		repo := testRunRepo(t, false)
		before := n.Load()
		code, out := runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunFailingOutput))
		if code != 0 || out != "" || n.Load() != before || len(testRunBeats(t)) != 0 {
			t.Errorf("hook off: exit %d output %q requests %d rows %d", code, out, n.Load()-before, len(testRunBeats(t)))
		}
	})

	t.Run("control: a declared command with a failing run speaks", func(t *testing.T) {
		repo := testRunRepo(t, true)
		for _, tc := range []struct{ event, payload string }{
			{"test-run", testRunPostPayload(repo, "go test ./... 2>&1 | tail -20", testRunFailingOutput)},
			{"failed-test-run", testRunFailurePayload(repo, "task test", testRunFailingOutput)},
		} {
			code, out := runTestRunHook(t, tc.event, repo, srv.URL, tc.payload)
			if code != 0 || !strings.Contains(out, "TestRound") {
				t.Errorf("%s: exit %d output %q; want the failing test named", tc.event, code, out)
			}
		}
	})
}

// TestRenderTestRunLineSilentOnPassAndNamesSavedPath pins TR-6: nothing on
// a pass or an unknown result; on a failure one line inside the I35 cap
// naming the failing tests, the changed symbols they reach, and the path
// of the full output saved under workspace/tmp (I31) — the path survives
// any cut.
func TestRenderTestRunLineSilentOnPassAndNamesSavedPath(t *testing.T) {
	const saved = "workspace/tmp/test-run/0123456789abcdef.log"
	pass := testrun.Analyze(testRunPassingOutput, testrun.Evidence{})
	if got := renderTestRunLine(pass, testrun.Candidates{}, saved); got != "" {
		t.Errorf("pass rendered %q", got)
	}
	unknown := testrun.Analyze(testRunPassingOutput, testrun.Evidence{Filtered: true})
	if unknown.Verdict != testrun.Unknown || renderTestRunLine(unknown, testrun.Candidates{}, saved) != "" {
		t.Errorf("unknown rendered a line")
	}

	two := "--- FAIL: TestRound (0.00s)\n    money_test.go:7: bad\nFAIL\tex.com/shop/money\t0.1s\n" +
		"--- FAIL: TestTotal (0.00s)\n    cart_test.go:9: bad\nFAIL\tex.com/shop/cart\t0.1s\n"
	res := testrun.Analyze(two, testrun.Evidence{})
	round := testrun.ChangedSymbol{Ref: "money/money.Round", Name: "money.Round", File: "money/money.go", Line: 41}
	cands := testrun.Candidates{Reach: []testrun.TestReach{
		{Test: "ex.com/shop/cart.TestTotal", Changed: []testrun.ChangedSymbol{round}},
		{Test: "ex.com/shop/money.TestRound", Changed: []testrun.ChangedSymbol{round}},
	}}
	line := renderTestRunLine(res, cands, saved)
	for _, want := range []string{"2 failed", "TestRound", "TestTotal", "money.Round", "changed in working tree", "money/money.go:41", saved} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, "\n") || len(line) > testRunLineBudget {
		t.Errorf("line is %d bytes or multi-line: %q", len(line), line)
	}
	if again := renderTestRunLine(res, cands, saved); again != line {
		t.Errorf("not deterministic:\n%q\n%q", line, again)
	}

	// No changed symbol reached: say so, and name localize's candidate.
	loc := testrun.Candidates{
		Reach:     []testrun.TestReach{{Test: "ex.com/shop/cart.TestTotal"}, {Test: "ex.com/shop/money.TestRound"}},
		Localized: []testrun.Located{{Symbol: "shop::money/money.Round", File: "money/money.go"}},
	}
	if l := renderTestRunLine(res, loc, saved); !strings.Contains(l, "no changed symbol") || !strings.Contains(l, "money/money.go") {
		t.Errorf("no-reach line = %q", l)
	}

	// A huge run is cut, the cut is disclosed, and the path survives.
	var big strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&big, "--- FAIL: TestCase%03dWithAQuiteLongDescriptiveName (0.00s)\n    x_test.go:%d: bad\n", i, i+1)
	}
	big.WriteString("FAIL\tex.com/big\t0.1s\n")
	bl := renderTestRunLine(testrun.Analyze(big.String(), testrun.Evidence{}), testrun.Candidates{}, saved)
	if len(bl) > testRunLineBudget || !strings.HasSuffix(bl, saved) || !strings.Contains(bl, "80 failed") || !strings.Contains(bl, "more") {
		t.Errorf("big line (%d bytes) = %q", len(bl), bl)
	}

	// A failure with no parsed test (a carried exit code) still speaks.
	ex := 2
	bare := testrun.Analyze("something broke\n", testrun.Evidence{ExitCode: &ex})
	if l := renderTestRunLine(bare, testrun.Candidates{}, saved); !strings.Contains(l, "exit 2") || !strings.Contains(l, saved) {
		t.Errorf("exit-only line = %q", l)
	}
}

// TestTestRunLogRowCarriesNoOutputText pins TR-7: one row per matched run
// in the existing hook log (runner, three-way result, failing ids and
// counts), never the output's text, and marked as no attested run (I43).
func TestTestRunLogRowCarriesNoOutputText(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo := testRunRepo(t, true)
	code, out := runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, "go test ./money/", testRunFailingOutput))
	if code != 0 || out == "" {
		t.Fatalf("exit %d output %q", code, out)
	}
	rows := testRunBeats(t)
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want exactly one per run", rows)
	}
	r := rows[0]
	for k, want := range map[string]string{
		"kind": "fail", "result": "fail", "runner": "go", "failed": "1",
		"failing": "example.com/shop/money.TestRound", "exit_code": "1",
		"event": "failed-test-run", "invocation": "go test", "attested": "false", "session": "tr-session",
	} {
		if r[k] != want {
			t.Errorf("row[%s] = %q, want %q (row %v)", k, r[k], want, r)
		}
	}
	raw, _ := os.ReadFile(os.Getenv("FATHOM_HOOKS_LOG"))
	for _, leak := range []string{"SECRET-OUTPUT-LINE-42", "Round(15)", "want 20"} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Errorf("the hook log carries output text %q", leak)
		}
	}

	// A pass is logged too (a row per run), silently.
	code, out = runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
	if code != 0 || out != "" {
		t.Errorf("pass: exit %d output %q", code, out)
	}
	if rows := testRunBeats(t); len(rows) != 2 || rows[1]["result"] != "pass" || rows[1]["emitted"] != "false" {
		t.Errorf("rows after a pass = %v", rows)
	}
}

// TestTestRunHookInjectsOnly pins TR-8 (ADR-029 section 5.3, TR-X2): the
// hook's only output is additionalContext — no updatedToolOutput, no
// decision or permission field, no continue:false — it exits 0, and it
// never runs a command: a fake go/task/make/npm on PATH would leave a
// sentinel if anything invoked them.
func TestTestRunHookInjectsOnly(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo := testRunRepo(t, true)
	fake := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"go", "task", "make", "npm"} {
		script := "#!/bin/sh\ntouch " + sentinel + "\n"
		if err := os.WriteFile(filepath.Join(fake, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, tc := range []struct{ event, payload, name string }{
		{"test-run", testRunPostPayload(repo, "task test | tail -30", testRunFailingOutput), "PostToolUse"},
		{"failed-test-run", testRunFailurePayload(repo, "go test ./...", testRunFailingOutput), "PostToolUseFailure"},
	} {
		code, out := runTestRunHook(t, tc.event, repo, srv.URL, tc.payload)
		if code != 0 {
			t.Errorf("%s: exit %d, want 0", tc.event, code)
		}
		var env map[string]any
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("%s: output is not one JSON envelope: %v (%q)", tc.event, err, out)
		}
		if len(env) != 1 || env["hookSpecificOutput"] == nil {
			t.Errorf("%s: top-level keys %v, want only hookSpecificOutput", tc.event, env)
		}
		hso, _ := env["hookSpecificOutput"].(map[string]any)
		if len(hso) != 2 || hso["hookEventName"] != tc.name || hso["additionalContext"] == "" {
			t.Errorf("%s: hookSpecificOutput = %v, want exactly hookEventName and additionalContext", tc.event, hso)
		}
		for _, forbidden := range []string{"updatedToolOutput", "updatedMCPToolOutput", "decision", "permissionDecision", "continue", "stopReason"} {
			if strings.Contains(out, `"`+forbidden+`"`) {
				t.Errorf("%s: output carries %q", tc.event, forbidden)
			}
		}
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("the hook ran a command: a fake runner left its sentinel")
	}
}

// TestTestRunSkipsSilentlyOnMissedDeadline pins TR-10 (I53): the hook's
// tier 1 work runs inside the descriptor's deadline, and a missed deadline
// is silence — never a late line — recorded as such in the run's row.
func TestTestRunSkipsSilentlyOnMissedDeadline(t *testing.T) {
	t.Run("the wrapper", func(t *testing.T) {
		expired, cancel := context.WithCancel(context.Background())
		cancel()
		called := false
		if line, missed := runTestRunWithDeadline(expired, func(context.Context) string { called = true; return "late" }); line != "" || !missed || called {
			t.Errorf("an already-spent deadline: line %q missed %v called %v", line, missed, called)
		}
		ctx, cancelMid := context.WithCancel(context.Background())
		defer cancelMid()
		line, missed := runTestRunWithDeadline(ctx, func(context.Context) string {
			cancelMid() // the deadline passes while the work runs
			return "a line that arrives after the deadline"
		})
		if line != "" || !missed {
			t.Errorf("a deadline passed mid-work: line %q missed %v", line, missed)
		}
		if line, missed := runTestRunWithDeadline(context.Background(), func(context.Context) string { return "in time" }); line != "in time" || missed {
			t.Errorf("in time: %q %v", line, missed)
		}
	})

	t.Run("a hanging daemon", func(t *testing.T) {
		orig := hookBudgetScale
		hookBudgetScale = 0.01
		defer func() { hookBudgetScale = orig }()
		h := newHangingDaemon(t)
		repo := testRunRepo(t, true)
		code, out := runTestRunHook(t, "failed-test-run", repo, h.srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))
		if code != 0 || out != "" {
			t.Errorf("past the deadline: exit %d output %q, want 0 and silence", code, out)
		}
		rows := testRunBeats(t)
		if len(rows) != 1 || rows[0]["deadline"] != "missed" || rows[0]["emitted"] != "false" || rows[0]["result"] != "fail" {
			t.Errorf("rows = %v, want one fail row marked deadline missed, nothing emitted", rows)
		}
	})
}

// TestDaemonTestRunIndexQualifiesService pins the key shape tier 1 sends:
// the call graph resolves "svc::dir/pkg.Name" but not the bare
// "dir/pkg.Name" (read against the live index on 2026-09-29), and
// localize gets its seed kind, value and the 4-hop budget.
func TestDaemonTestRunIndexQualifiesService(t *testing.T) {
	var got []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		req.Params.Arguments["tool"] = req.Params.Name
		got = append(got, req.Params.Arguments)
		payload := `{"tests":["shop::money/money.TestRound"]}`
		if req.Params.Name == "localize" {
			payload = `{"candidates":[{"file":"money/money.go","symbol":"shop::money/money.Round","score":3}]}`
		}
		text, _ := json.Marshal(payload)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":%s}]}}`, text)
	}))
	defer srv.Close()
	idx := daemonTestRunIndex{ctx: context.Background(), baseURL: srv.URL, service: "shop"}
	tests, err := idx.TestImpact("money/money.Round")
	if err != nil || len(tests) != 1 || got[0]["symbol"] != "shop::money/money.Round" || got[0]["service"] != "shop" {
		t.Errorf("test_impact: tests %v err %v args %v", tests, err, got)
	}
	loc, err := idx.Localize(testrun.LocalizeSeed{Kind: "failing_test", Value: "TestRound", Budget: testrun.LocalizeHops})
	if err != nil || len(loc) != 1 || loc[0].Symbol != "shop::money/money.Round" {
		t.Errorf("localize: %v %v", loc, err)
	}
	if a := got[1]; a["failing_test"] != "TestRound" || a["budget"] != float64(4) || a["tool"] != "localize" {
		t.Errorf("localize args = %v", a)
	}
}
