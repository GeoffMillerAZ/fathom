package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAD7AgentMemoIsolation(t *testing.T) {
	oldVerdict, oldOutcome := ruleVerdictMemoDir, ruleOutcomeMemoDir
	ruleVerdictMemoDir, ruleOutcomeMemoDir = t.TempDir(), t.TempDir()
	t.Cleanup(func() { ruleVerdictMemoDir, ruleOutcomeMemoDir = oldVerdict, oldOutcome })
	log := filepath.Join(t.TempDir(), "outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", log)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, agent := range []string{"", "agent-a", "agent-b", "agent-a"} {
		text, ok := withRuleVerdictAskAt("session-a", "svc", "pkg/a.go", "Always sort outputs.", "AGENTS.md#L1-L2", "claim-a", "rules", true, now, nil, agent)
		if !ok || text != "rules" {
			t.Errorf("context %q received verdict text: %q", agent, text)
		}
		trackRuleOutcomeAt("session-a", "svc", "pkg/a.go", "claim-a", now, agent)
	}
	for _, agent := range []string{"", "agent-a", "agent-b"} {
		if _, ok := readRuleVerdictMemo("session-a", "svc", "pkg/a.go", now, agent); !ok {
			t.Errorf("missing verdict memo for %q", agent)
		}
		if _, ok := readRuleOutcomeMemo("session-a", "svc", "pkg/a.go", now, agent); !ok {
			t.Errorf("missing outcome memo for %q", agent)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), `"event":"still_served"`); got != 1 {
		t.Errorf("still_served = %d, want one child repeat", got)
	}
	if _, found := readPendingVerdictAsk("session-a", now); found {
		t.Fatal("child wrote a pending user verdict ask")
	}
}

func TestEditHookAD7AgentMemosAndNoChildVerdict(t *testing.T) {
	root := cbHookEnv(t)
	verdictDir, outcomeDir := t.TempDir(), t.TempDir()
	oldVerdict, oldOutcome := ruleVerdictMemoDir, ruleOutcomeMemoDir
	ruleVerdictMemoDir, ruleOutcomeMemoDir = verdictDir, outcomeDir
	t.Cleanup(func() { ruleVerdictMemoDir, ruleOutcomeMemoDir = oldVerdict, oldOutcome })
	log := filepath.Join(t.TempDir(), "outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", log)
	file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
	fake := &specRefsFakeDaemon{t: t, obligationsOut: map[string]any{
		"service": "svc", "file": "pkg/a.go",
		"rules": []any{map[string]any{"rule": "Always sort outputs.", "src": "AGENTS.md#L1-L2", "tier": "doc-claimed", "claim_id": "claim-a"}},
	}}
	srv := fake.start()
	defer srv.Close()
	for _, agent := range []string{"", "agent-a", "agent-b", "agent-a"} {
		raw, _ := json.Marshal(map[string]any{"session_id": "session-a", "agent_id": agent, "cwd": root, "tool_name": "Edit", "tool_input": map[string]any{"file_path": file}})
		hookStdin(t, string(raw))
		var out, errb bytes.Buffer
		if code := cmdEditHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
			t.Fatalf("hook exit %d: %s", code, errb.String())
		}
		if agent != "" && strings.Contains(out.String(), ruleVerdictAskMarker) {
			t.Errorf("child %s received a user verdict ask: %s", agent, out.String())
		}
		if strings.Contains(out.String(), "served rule ignored") {
			t.Errorf("unchecked prose alleged noncompliance: %s", out.String())
		}
	}
	for _, dir := range []string{verdictDir, outcomeDir} {
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil || len(files) != 3 {
			t.Errorf("memo directory has %d entries, want one per parent/child: %v", len(files), err)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), `"event":"still_served"`); got != 1 {
		t.Errorf("still_served count = %d, want only agent-a's repeat", got)
	}
}
