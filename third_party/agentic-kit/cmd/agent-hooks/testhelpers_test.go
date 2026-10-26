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

// fixedNow is the clock every test app uses.
var fixedNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// testApp is an app wired to in-memory streams, a temp home and state dir,
// a fake environment and a fixed clock.
type testApp struct {
	*app
	out, errOut *bytes.Buffer
	vars        map[string]string
	dir         string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	dir := t.TempDir()
	ta := &testApp{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}, dir: dir, vars: map[string]string{
		"AGENT_HOOKS_STATE":  filepath.Join(dir, "state"),
		"AGENT_HOOKS_POLICY": filepath.Join(dir, "policy.json"), // absent: built-in policy
	}}
	ta.app = &app{
		stdin: strings.NewReader(""), stdout: ta.out, stderr: ta.errOut,
		env: func(k string) string { return ta.vars[k] }, now: func() time.Time { return fixedNow },
		home: filepath.Join(dir, "home"),
	}
	return ta
}

// hook runs one hook invocation with the payload and returns stdout.
func (ta *testApp) hook(t *testing.T, host string, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ta.out.Reset()
	ta.stdin = bytes.NewReader(raw)
	if code := ta.run([]string{"hook", "--host", host}); code != 0 {
		t.Fatalf("hook exit = %d, want 0", code)
	}
	return ta.out.String()
}

// ledger returns every record written so far.
func (ta *testApp) ledger(t *testing.T) []ledgerRecord {
	t.Helper()
	var recs []ledgerRecord
	for _, f := range ta.ledgerFiles() {
		scanLedger(f, func(r ledgerRecord, _ time.Time) { recs = append(recs, r) })
	}
	return recs
}

// writePolicy writes a policy file derived from the built-in one.
func (ta *testApp) writePolicy(t *testing.T, mutate func(m map[string]any)) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(defaultPolicyJSON, &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ta.vars["AGENT_HOOKS_POLICY"], raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// hookOutput is the decoded host envelope.
type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string          `json:"hookEventName"`
		PermissionDecision       string          `json:"permissionDecision"`
		PermissionDecisionReason string          `json:"permissionDecisionReason"`
		AdditionalContext        string          `json:"additionalContext"`
		UpdatedToolOutput        json.RawMessage `json:"updatedToolOutput"`
	} `json:"hookSpecificOutput"`
}

func decodeOutput(t *testing.T, s string) hookOutput {
	t.Helper()
	var o hookOutput
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		t.Fatalf("hook output is not JSON: %v\n%s", err, s)
	}
	return o
}

// claudeSpawn builds a Claude Code PreToolUse Agent payload.
func claudeSpawn(session string, input map[string]any) map[string]any {
	return map[string]any{"hook_event_name": "PreToolUse", "session_id": session, "tool_name": "Agent", "tool_input": input}
}

// codexSpawn builds a Codex PreToolUse spawn_agent payload.
func codexSpawn(session string, input map[string]any) map[string]any {
	return map[string]any{"hook_event_name": "PreToolUse", "session_id": session, "tool_name": "spawn_agent", "model": "parent-model", "tool_input": input}
}

// codexPolicy maps example Codex model names onto the built-in tiers.
func codexPolicy(m map[string]any) {
	tiers := m["tiers"].([]any)
	names := [][]any{{"cx-small"}, {"cx-mid"}, {"cx-large"}, {"cx-max"}}
	for i, t := range tiers {
		t.(map[string]any)["codex"] = names[i]
	}
}
