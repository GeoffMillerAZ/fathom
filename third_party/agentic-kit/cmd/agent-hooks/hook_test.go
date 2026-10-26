package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHookDeniesUnsetModelWithHostEnvelope(t *testing.T) {
	ta := newTestApp(t)
	out := decodeOutput(t, ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{
		"description": "find loader", "prompt": "find the config loader", "subagent_type": "general-purpose",
	})))
	h := out.HookSpecificOutput
	if h.HookEventName != "PreToolUse" || h.PermissionDecision != "deny" || !strings.Contains(h.PermissionDecisionReason, "no model set") {
		t.Fatalf("envelope = %+v", h)
	}
	recs := ta.ledger(t)
	if len(recs) != 1 || recs[0].Kind != kindSpawn || recs[0].Decision != "deny" || recs[0].Codes[0] != codeNoModel {
		t.Fatalf("ledger = %+v", recs)
	}
}

func TestHookGatesCodexNamespacedSpawn(t *testing.T) {
	ta := newTestApp(t)
	payload := codexSpawn("s1", map[string]any{"message": "pong", "task_name": "t", "fork_turns": "none"})
	payload["tool_name"] = "collaborationspawn_agent" // what Codex 0.163 actually sends
	out := decodeOutput(t, ta.hook(t, hostCodex, payload))
	if out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("namespaced Codex spawn was not gated: %+v", out.HookSpecificOutput)
	}
}

func TestHookAllowsSilently(t *testing.T) {
	ta := newTestApp(t)
	if out := ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{"prompt": "find it", "model": "haiku"})); out != "" {
		t.Fatalf("allowed spawn printed %q, want silence", out)
	}
	if recs := ta.ledger(t); recs[0].Decision != "allow" || recs[0].Tier != "fast" {
		t.Fatalf("ledger = %+v", recs)
	}
}

func TestHookAdviceIsOncePerSession(t *testing.T) {
	ta := newTestApp(t)
	brief := map[string]any{"prompt": "[critic] /caveman verify the invariants hold", "model": "opus"}
	first := decodeOutput(t, ta.hook(t, hostClaude, claudeSpawn("s1", brief)))
	if !strings.Contains(first.HookSpecificOutput.AdditionalContext, "critic's prose") || first.HookSpecificOutput.PermissionDecision != "" {
		t.Fatalf("first advice envelope = %+v (advice must not auto-approve)", first.HookSpecificOutput)
	}
	if second := ta.hook(t, hostClaude, claudeSpawn("s1", brief)); second != "" {
		t.Fatalf("second call in the same session printed %q, want silence", second)
	}
	if other := ta.hook(t, hostClaude, claudeSpawn("s2", brief)); other == "" {
		t.Fatal("a new session must get the advice again")
	}
}

func TestHookPolicyFallbackIsAnnouncedOnce(t *testing.T) {
	ta := newTestApp(t)
	if err := os.WriteFile(ta.vars["AGENT_HOOKS_POLICY"], []byte(`{broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := decodeOutput(t, ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{"prompt": "x", "model": "haiku"})))
	if !strings.Contains(out.HookSpecificOutput.AdditionalContext, "policy file was rejected") {
		t.Fatalf("fallback not announced: %+v", out.HookSpecificOutput)
	}
	if again := ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{"prompt": "x", "model": "haiku"})); again != "" {
		t.Fatalf("fallback announced twice: %q", again)
	}
}

func TestHookFailsOpen(t *testing.T) {
	cases := map[string][]byte{
		"not json":       []byte("not json"),
		"json array":     []byte(`[1,2]`),
		"spawn bad type": []byte(`{"hook_event_name":"PreToolUse","tool_name":"Agent","tool_input":"oops"}`),
		"oversized":      bytes.Repeat([]byte("x"), maxHookInput+1),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			ta.stdin = bytes.NewReader(payload)
			if code := ta.run([]string{"hook", "--host", hostClaude}); code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			if ta.out.Len() != 0 {
				t.Fatalf("stdout = %q, want nothing", ta.out.String())
			}
			recs := ta.ledger(t)
			if len(recs) != 1 || recs[0].Kind != kindFault {
				t.Fatalf("want one fault record, got %+v", recs)
			}
		})
	}
}

func TestHookMisconfiguredHostNeverBlocks(t *testing.T) {
	ta := newTestApp(t)
	ta.stdin = strings.NewReader(`{}`)
	if code := ta.run([]string{"hook", "--host", "cursor"}); code != 0 {
		t.Fatalf("exit = %d, want 0 for a misconfigured hook command", code)
	}
}

func TestHookIgnoresUnrelatedTools(t *testing.T) {
	ta := newTestApp(t)
	out := ta.hook(t, hostClaude, map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Edit", "tool_input": map[string]any{}})
	if out != "" || len(ta.ledger(t)) != 0 {
		t.Fatalf("an Edit PreToolUse must be ignored entirely, got %q / %+v", out, ta.ledger(t))
	}
}

func TestHookRecordsClaudeSpawnResult(t *testing.T) {
	ta := newTestApp(t)
	ta.hook(t, hostClaude, map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Agent",
		"tool_input": map[string]any{"prompt": "x", "model": "haiku", "subagent_type": "cavecrew-investigator"},
		"tool_response": map[string]any{
			"status": "completed", "resolvedModel": "claude-haiku-5-5", "totalTokens": 1234,
			"totalDurationMs": 900, "totalToolUseCount": 3, "content": []any{map[string]any{"type": "text", "text": "src/a.go:10"}},
		},
	})
	r := ta.ledger(t)[0]
	if r.Kind != kindSpawnResult || r.Resolved != "claude-haiku-5-5" || r.TotalTokens != 1234 || r.ReportChars != len("src/a.go:10")+len("text") {
		t.Fatalf("spawn result = %+v", r)
	}
}

func TestHookRecordsCodexSubagentStop(t *testing.T) {
	ta := newTestApp(t)
	ta.hook(t, hostCodex, map[string]any{
		"hook_event_name": "SubagentStop", "session_id": "s1", "agent_id": "a1", "agent_type": "explorer",
		"last_assistant_message": "found it", "model": "parent-model",
	})
	r := ta.ledger(t)[0]
	if r.Kind != kindSubagentStop || r.ReportChars != 8 || r.AgentType != "explorer" {
		t.Fatalf("subagent stop = %+v", r)
	}
}

func bashPayload(agentID string, stdout, stderr string) map[string]any {
	p := map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "s1", "tool_name": "Bash", "tool_use_id": "toolu_1",
		"tool_input":    map[string]any{"command": "FOO=1 go test ./..."},
		"tool_response": map[string]any{"stdout": stdout, "stderr": stderr, "interrupted": false, "isImage": false},
	}
	if agentID != "" {
		p["agent_id"] = agentID
	}
	return p
}

func TestHookContainsLargeMainThreadBash(t *testing.T) {
	ta := newTestApp(t)
	big := "HEAD" + strings.Repeat("x", 20000) + "TAIL"
	out := decodeOutput(t, ta.hook(t, hostClaude, bashPayload("", big, "warning: one")))
	var resp map[string]any
	if err := json.Unmarshal(out.HookSpecificOutput.UpdatedToolOutput, &resp); err != nil {
		t.Fatalf("updatedToolOutput: %v", err)
	}
	stdout := resp["stdout"].(string)
	if !strings.HasPrefix(stdout, "HEAD") || !strings.HasSuffix(stdout, "TAIL") || !strings.Contains(stdout, "main-thread output contained") {
		t.Fatalf("contained stdout lost its head, tail or notice (%d chars)", len(stdout))
	}
	if resp["interrupted"] != false || resp["isImage"] != false || resp["stderr"] != "warning: one" {
		t.Fatalf("other fields must pass through unchanged: %+v", resp)
	}
	path := stdout[strings.Index(stdout, "Full output: ")+len("Full output: "):]
	path = path[:strings.Index(path, " .")]
	saved, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(saved), big) || !strings.Contains(string(saved), "warning: one") {
		t.Fatalf("saved full output missing or incomplete at %q: %v", path, err)
	}
	r := ta.ledger(t)[0]
	if !r.Contained || r.Command != "go" || r.OutputChars != len(big)+len("warning: one") {
		t.Fatalf("ledger = %+v", r)
	}
}

func TestHookNeverContainsSubagentOrSmallOrCodexOutput(t *testing.T) {
	big := strings.Repeat("x", 20000)
	for name, run := range map[string]func(ta *testApp) string{
		"subagent": func(ta *testApp) string { return ta.hook(t, hostClaude, bashPayload("agent-7", big, "")) },
		"small":    func(ta *testApp) string { return ta.hook(t, hostClaude, bashPayload("", "ok", "")) },
		"codex": func(ta *testApp) string {
			p := bashPayload("", big, "")
			p["tool_name"] = "shell"
			return ta.hook(t, hostCodex, p)
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			if out := run(ta); out != "" {
				t.Fatalf("printed %q, want pass-through", out)
			}
			if r := ta.ledger(t)[0]; r.Contained || r.Kind != kindToolOutput {
				t.Fatalf("ledger = %+v", r)
			}
		})
	}
}

func TestHookLeavesNativelyPersistedOutputAlone(t *testing.T) {
	ta := newTestApp(t)
	p := bashPayload("", strings.Repeat("x", 30000), "")
	resp := p["tool_response"].(map[string]any)
	resp["persistedOutputPath"], resp["persistedOutputSize"] = "/tmp/tool-results/x.txt", 168894
	if out := ta.hook(t, hostClaude, p); out != "" {
		t.Fatalf("replaced output Claude Code already persisted: %q", out)
	}
	if r := ta.ledger(t)[0]; r.Contained || r.OutputChars != 168894 {
		t.Fatalf("ledger = %+v, want the true size and no containment", r)
	}
}

func TestHookLedgerCanBeDisabled(t *testing.T) {
	ta := newTestApp(t)
	ta.writePolicy(t, func(m map[string]any) { m["ledger"].(map[string]any)["enabled"] = false })
	ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{"prompt": "x"}))
	if _, err := os.Stat(ta.ledgerPath()); !os.IsNotExist(err) {
		t.Fatalf("ledger written while disabled: %v", err)
	}
}
