package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReportSummarizesLedger(t *testing.T) {
	ta := newTestApp(t)
	cfg := builtinPolicy().Ledger
	at := func(d time.Duration, r ledgerRecord) {
		now := fixedNow.Add(d)
		ta.now = func() time.Time { return now }
		ta.appendLedger(cfg, r)
	}
	at(-20*24*time.Hour, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Session: "old", Decision: "deny", Codes: []string{codeNoModel}})
	at(-10*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Session: "s", Decision: "deny", Codes: []string{codeNoModel}})
	at(-9*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Session: "s", Decision: "allow", Tier: "fast"})
	at(-8*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Session: "t", Decision: "deny", Codes: []string{codeForkMarker}})
	at(-7*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindSpawn, Session: "t", Decision: "allow", Fork: true, Tier: "inherit"})
	at(-6*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindSpawnResult, Status: "completed", SpawnedAgent: "agent-1", Resolved: "claude-haiku-5-5", TotalTokens: 500, AgentType: "Explore", ReportChars: 300})
	at(-5*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindToolOutput, Command: "go", OutputChars: 20000, Contained: true})
	at(-4*time.Minute, ledgerRecord{Host: hostClaude, Kind: kindToolOutput, AgentID: "a1", Command: "rg", OutputChars: 700})
	at(-3*time.Minute, ledgerRecord{Host: hostCodex, Kind: kindFault, Fault: "boom"})

	ta.now = func() time.Time { return fixedNow }
	if code := ta.run([]string{"report", "--days", "7"}); code != 0 {
		t.Fatalf("report exit = %d", code)
	}
	out := ta.out.String()
	for _, want := range []string{
		"Spawns: 4 decided, 2 denied, 1 allowed forks",
		"recovered within 15m0s: 2 of 2 (1 as forks)",
		"claude-haiku-5-5",
		"main thread 20000 chars, subagents 700 chars",
		"contained in the main thread: 1 outputs",
		"Faults (hook failed open): 1, latest: boom",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestPercentile(t *testing.T) {
	v := []int{5, 1, 4, 2, 3}
	if percentile(v, 50) != 3 || percentile(v, 90) != 5 || percentile(nil, 50) != 0 {
		t.Fatalf("percentile wrong: p50=%d p90=%d", percentile(v, 50), percentile(v, 90))
	}
}

// FuzzHook proves the failure contract: whatever the host sends, the hook
// exits 0 and prints either nothing or one well-formed host envelope.
func FuzzHook(f *testing.F) {
	f.Add([]byte(`{"hook_event_name":"PreToolUse","tool_name":"Agent","tool_input":{"prompt":"x"}}`), false)
	f.Add([]byte(`{"hook_event_name":"PreToolUse","tool_name":"spawn_agent","tool_input":{"message":"x","fork_turns":3}}`), true)
	f.Add([]byte(`{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"`+strings.Repeat("y", 12000)+`"}}`), false)
	f.Add([]byte(`{"hook_event_name":"SubagentStop","last_assistant_message":null}`), true)
	f.Add([]byte(`{"tool_input":{"items":[1,"a",{"text":2}]}}`), true)
	f.Add([]byte("\x00\xff"), false)
	f.Fuzz(func(t *testing.T, payload []byte, codex bool) {
		ta := newTestApp(t)
		host := hostClaude
		if codex {
			host = hostCodex
		}
		ta.stdin = bytes.NewReader(payload)
		if code := ta.run([]string{"hook", "--host", host}); code != 0 {
			t.Fatalf("exit = %d", code)
		}
		if out := bytes.TrimSpace(ta.out.Bytes()); len(out) > 0 {
			var env hookOutput
			if err := json.Unmarshal(out, &env); err != nil || env.HookSpecificOutput.HookEventName == "" {
				t.Fatalf("malformed envelope %q: %v", out, err)
			}
		}
	})
}
