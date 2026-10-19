package main

// stophook_reentry_test.go — O1180: a Stop or SubagentStop hook's
// additionalContext re-opens the turn (RO-V1), so every stop-time path that
// can print context must be silent when the stop already continued once
// (stop_hook_active true). The paths: Stop's plan-claim verify (advisory and
// --block) and its claim check, and SubagentStop's claim check.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subagentStopPayloadAt is subagentStopPayload with stop_hook_active set.
func subagentStopPayloadAt(repo, session, agent, msg string, reEntrant bool) string {
	b, _ := json.Marshal(map[string]any{"session_id": session, "cwd": repo, "hook_event_name": "SubagentStop", "stop_hook_active": reEntrant,
		"agent_id": agent, "agent_type": "general-purpose", "last_assistant_message": msg})
	return string(b)
}

func TestStopHookNeverEmitsContextWhenReEntrant(t *testing.T) {
	repo, _ := claimCheckRepo(t, true)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "session-ledger.jsonl"))
	// A plan task open at HEAD and claimed in the tree: Stop's verify flags it.
	writeRepoFile(t, repo, "docs/plans/p.md", "- [ ] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n")
	commitAll(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "docs", "plans", "p.md"), []byte("- [x] One {id: T-1, symbol: Ghost}\n- [x] Two {id: T-2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := fakeStopDaemon(t, strings.ToLower(filepath.Base(repo)))
	defer srv.Close()
	const session = "reentry-session"
	const msg = "Fixed it. All tests pass now."
	markSessionEdit(session)
	runTestRunHook(t, "failed-test-run", repo, srv.URL, testRunFailurePayload(repo, "go test ./...", testRunFailingOutput))

	cases := []struct {
		name    string
		payload func(reEntrant bool) string
		extra   []string
		want    string // what the first stop of the chain prints
	}{
		{"stop: plan verify and claim check", func(r bool) string { return stopPayload(repo, session, msg, r) }, nil, "T-1"},
		{"stop --block: plan verify", func(r bool) string { return stopPayload(repo, session, msg, r) }, []string{"--block"}, `"decision":"block"`},
		{"subagent-stop: claim check", func(r bool) string { return subagentStopPayloadAt(repo, session, "agent-1", msg, r) }, []string{"--event", "subagent-stop"}, "no test run is recorded by this subagent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if out := runStopVerb(t, srv.URL, c.payload(false), c.extra...); !strings.Contains(out, c.want) {
				t.Fatalf("first stop printed %q, want it to carry %q (the path under test must emit)", out, c.want)
			}
			if out := runStopVerb(t, srv.URL, c.payload(true), c.extra...); out != "" {
				t.Errorf("re-entrant stop printed %q, want silence (a stop's context re-opens the turn)", out)
			}
		})
	}
}
