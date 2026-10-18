package main

// claimcheck_test.go — RO-39 (docs/plans/readouts.md): the claim check at
// Stop and SubagentStop reads the session's test-run rows from the hook log
// and the final message the input carries.

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// claimCheckRepo is testRunRepo with the claim check switched on too, one
// committed Go file and CLAUDE_PROJECT_DIR pointing at it.
func claimCheckRepo(t *testing.T, claimCheck bool) (repo, head string) {
	t.Helper()
	repo = testRunRepo(t, true)
	cfg := "[hooks]\ntest_run = true\n"
	if claimCheck {
		cfg += "claim_check = true\n"
	}
	writeRepoFile(t, repo, ".fathom/config.toml", cfg)
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n }\n")
	head = commitAll(t, repo)
	t.Setenv("CLAUDE_PROJECT_DIR", repo)
	return repo, head
}

// markSessionEdit logs the edit hook's post-edit beat for the session, the
// session's last edit the window reads.
func markSessionEdit(session string) {
	emitBeat("edit-hook", "seen", firingMeta(map[string]string{"phase": "post"}, session, "money/money.go"))
}

func stopPayload(repo, session, msg string, reEntrant bool) string {
	b, _ := json.Marshal(map[string]any{"session_id": session, "cwd": repo, "hook_event_name": "Stop", "stop_hook_active": reEntrant, "last_assistant_message": msg})
	return string(b)
}

func subagentStopPayload(repo, session, agent, msg string) string {
	b, _ := json.Marshal(map[string]any{"session_id": session, "cwd": repo, "hook_event_name": "SubagentStop", "stop_hook_active": false,
		"agent_id": agent, "agent_type": "general-purpose", "last_assistant_message": msg})
	return string(b)
}

func runStopVerb(t *testing.T, url, payload string, extra ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	args := append([]string{"stop-hook", "--url", url}, extra...)
	restore := pipeStdin(t, payload) // the stop hook reads os.Stdin by contract
	defer restore()
	if code := runWithStdin(args, strings.NewReader(payload), &out, &errb); code != 0 {
		t.Fatalf("stop-hook exit %d: %s", code, errb.String())
	}
	return out.String()
}

func claimCheckRows(t *testing.T) []map[string]string {
	t.Helper()
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	var out []map[string]string
	for _, b := range beats {
		if b.Channel == claimCheckChannel {
			m := map[string]string{"kind": b.Kind}
			for k, v := range b.Meta {
				m[k] = v
			}
			out = append(out, m)
		}
	}
	return out
}

func agentPostPayload(repo, session, agent, command, output string) string {
	var m map[string]any
	_ = json.Unmarshal([]byte(testRunPostPayload(repo, command, output)), &m)
	m["session_id"], m["agent_id"] = session, agent
	b, _ := json.Marshal(m)
	return string(b)
}

func TestStopClaimCheckNamesAFailedRunAndIsSilentOnABackedClaim(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo, head := claimCheckRepo(t, true)
	const session = "tr-session"
	const msg = "Fixed the rounding. All tests pass now. SECRET-MESSAGE-TEXT"
	markSessionEdit(session)

	// A failing run, then the claim: one line naming that run.
	runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))
	out := runStopVerb(t, srv.URL, stopPayload(repo, session, msg, false))
	if !strings.Contains(out, "fathom claim check") || !strings.Contains(out, "failed at "+head[:7]) || !strings.Contains(out, "TestRound") {
		t.Fatalf("stop output = %q, want one line naming the failed run at %s and its failing test", out, head[:7])
	}
	var env struct {
		HSO struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &env); err != nil || env.HSO.Event != "Stop" || env.Decision != "" {
		t.Fatalf("stop output %q is not one advisory Stop envelope (%v)", out, err)
	}
	if n := strings.Count(env.HSO.Context, "\n") + 1; n != 1 {
		t.Errorf("context has %d lines, want 1: %q", n, env.HSO.Context)
	}

	// The re-entrant stop the line causes prints nothing and logs a repeat.
	if out := runStopVerb(t, srv.URL, stopPayload(repo, session, "Rerunning now; all tests pass.", true)); out != "" {
		t.Errorf("re-entrant stop printed %q, want silence (at most once per stop chain)", out)
	}

	// A passing run at the same head and tree backs the claim: silent.
	runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
	if out := runStopVerb(t, srv.URL, stopPayload(repo, session, msg, false)); out != "" {
		t.Errorf("a backed claim printed %q, want silence", out)
	}

	// An edit after the pass: the claim is no longer backed at this tree.
	writeRepoFile(t, repo, "money/money.go", "package money\n\nfunc Round(n int) int { return n + 1 }\n")
	markSessionEdit(session)
	if out := runStopVerb(t, srv.URL, stopPayload(repo, session, msg, false)); !strings.Contains(out, "none has run at the current tree") {
		t.Errorf("a claim after an edit = %q, want the stale-pass line", out)
	}

	rows := claimCheckRows(t)
	if len(rows) != 4 {
		t.Fatalf("claim-check rows = %d, want one per check (4): %v", len(rows), rows)
	}
	wantKinds := []string{"flagged", "silent", "silent", "flagged"}
	wantVerdicts := []string{"failed", "not checked (repeat)", "backed", "stale"}
	for i, r := range rows {
		if r["kind"] != wantKinds[i] || r["verdicts"] != wantVerdicts[i] || r["claims"] != "all-tests-pass" || r["session"] != session || r["event"] != "stop" {
			t.Errorf("row %d = %v, want kind %s verdict %s", i, r, wantKinds[i], wantVerdicts[i])
		}
		for k, v := range r {
			if strings.Contains(v, "SECRET-MESSAGE-TEXT") || strings.Contains(v, "rounding") {
				t.Errorf("row %d key %s carries message text: %q", i, k, v)
			}
		}
	}
	if rows[0]["rows"] == "-" || rows[2]["rows"] == "-" {
		t.Errorf("a checked claim cites no row: %v", rows)
	}
}

func TestSubagentStopClaimCheckReadsTheAgentsOwnRows(t *testing.T) {
	srv, n := countingDaemon(t)
	repo, _ := claimCheckRepo(t, true)
	const session = "tr-session"
	markSessionEdit(session)
	// The main agent passed; the subagent ran nothing of its own.
	runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
	before := n.Load()
	out := runStopVerb(t, srv.URL, subagentStopPayload(repo, session, "agent-1", "Done: the tests pass."), "--event", "subagent-stop")
	if !strings.Contains(out, "no test run is recorded by this subagent") || !strings.Contains(out, `"hookEventName":"SubagentStop"`) {
		t.Fatalf("subagent stop = %q, want the unbacked line for the subagent's own rows", out)
	}
	// The subagent's own passing run backs it.
	runTestRunHook(t, "test-run", repo, srv.URL, agentPostPayload(repo, session, "agent-1", "go test ./...", testRunPassingOutput))
	if row := lastTestRunRow(t); row["agent_id"] != "agent-1" {
		t.Fatalf("test-run row = %v, want agent_id agent-1", row)
	}
	before = n.Load()
	if out := runStopVerb(t, srv.URL, subagentStopPayload(repo, session, "agent-1", "Done: the tests pass."), "--event", "subagent-stop"); out != "" {
		t.Errorf("a backed subagent claim printed %q", out)
	}
	if got := n.Load() - before; got != 0 {
		t.Errorf("the subagent-stop check made %d daemon calls, want none", got)
	}
	rows := claimCheckRows(t)
	if last := rows[len(rows)-1]; last["event"] != "subagent-stop" || last["agent_id"] != "agent-1" || last["verdicts"] != "backed" {
		t.Errorf("last claim-check row = %v", last)
	}
	// A main-agent row never carries the key.
	runTestRunHook(t, "test-run", repo, srv.URL, testRunPostPayload(repo, "go test ./...", testRunPassingOutput))
	if _, ok := lastTestRunRow(t)["agent_id"]; ok {
		t.Error("a main-agent test-run row carries agent_id")
	}
}

func TestStopClaimCheckWindowOffAndBudget(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo, _ := claimCheckRepo(t, true)
	// Another session's row is in the log, but no edit and no recorded
	// start of this one: no run backs the claim, yet the tail cannot say
	// none happened (I38).
	markSessionEdit("another-session")
	if out := runStopVerb(t, srv.URL, stopPayload(repo, "s-window", "All tests pass.", false)); out != "" {
		t.Errorf("an uncovered claim printed %q, want silence", out)
	}
	rows := claimCheckRows(t)
	if len(rows) != 1 || rows[0]["verdicts"] != "not checked (window)" {
		t.Fatalf("rows = %v, want one not checked (window)", rows)
	}
	// A message with no claim, or a hedge, is no check at all.
	for _, m := range []string{"Edited money.go.", "The tests should pass once CI runs.", "I did not run the tests."} {
		if out := runStopVerb(t, srv.URL, stopPayload(repo, "s-window", m, false)); out != "" {
			t.Errorf("%q printed %q", m, out)
		}
	}
	if got := len(claimCheckRows(t)); got != 1 {
		t.Errorf("rows after non-claims = %d, want still 1", got)
	}
	// A spent budget is silent and logged.
	markSessionEdit("s-budget")
	orig := claimCheckBudget
	claimCheckBudget = 0
	defer func() { claimCheckBudget = orig }()
	if out := runStopVerb(t, srv.URL, stopPayload(repo, "s-budget", "All tests pass.", false)); out != "" {
		t.Errorf("a spent budget printed %q", out)
	}
	if last := claimCheckRows(t); last[len(last)-1]["verdicts"] != "not checked (deadline)" {
		t.Errorf("last row = %v, want not checked (deadline)", last[len(last)-1])
	}
}

func TestStopClaimCheckOffWritesNothing(t *testing.T) {
	srv, _ := countingDaemon(t)
	repo, _ := claimCheckRepo(t, false)
	markSessionEdit("tr-session")
	runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))
	if out := runStopVerb(t, srv.URL, stopPayload(repo, "tr-session", "All tests pass.", false)); out != "" {
		t.Errorf("claim_check off printed %q", out)
	}
	if out := runStopVerb(t, srv.URL, subagentStopPayload(repo, "tr-session", "a1", "All tests pass."), "--event", "subagent-stop"); out != "" {
		t.Errorf("claim_check off printed %q on SubagentStop", out)
	}
	if rows := claimCheckRows(t); len(rows) != 0 {
		t.Errorf("claim_check off logged %v", rows)
	}
}
