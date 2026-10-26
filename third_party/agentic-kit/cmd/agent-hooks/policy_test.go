package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBuiltinPolicyIsValid(t *testing.T) {
	p, err := parsePolicy(defaultPolicyJSON)
	if err != nil {
		t.Fatalf("embedded default policy: %v", err)
	}
	if !p.Spawn.RequireModel || p.Spawn.Fork != forkMarker || !p.requiresEffort(hostCodex) {
		t.Errorf("default policy lost a requirement: %+v", p.Spawn)
	}
	if p.hostMapped(hostCodex) {
		t.Error("the kit's default must not name Codex models (invariant I2): consumers supply them")
	}
}

func TestPolicyValidationRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr string
	}{
		{"wrong version", func(m map[string]any) { m["version"] = 2 }, "version must be 1"},
		{"no tiers", func(m map[string]any) { m["tiers"] = []any{} }, "need 1..8 tiers"},
		{"duplicate tier", func(m map[string]any) {
			tiers := m["tiers"].([]any)
			tiers[1].(map[string]any)["name"] = "fast"
		}, "duplicated"},
		{"model in two tiers", func(m map[string]any) {
			m["tiers"].([]any)[1].(map[string]any)["claude"] = []any{"HAIKU"}
		}, "already belongs to tier"},
		{"inherit as a model", func(m map[string]any) {
			m["tiers"].([]any)[0].(map[string]any)["claude"] = []any{"inherit"}
		}, "not a model"},
		{"model with whitespace", func(m map[string]any) {
			m["tiers"].([]any)[0].(map[string]any)["claude"] = []any{"a b"}
		}, "without whitespace"},
		{"ceiling names no tier", func(m map[string]any) {
			m["classes"].(map[string]any)["worker"] = map[string]any{"max_tier": "huge"}
		}, "is not a tier"},
		{"bad critic pattern", func(m map[string]any) { m["classes"].(map[string]any)["critic_pattern"] = "(" }, "critic_pattern"},
		{"bad fork mode", func(m map[string]any) { m["spawn"].(map[string]any)["fork"] = "sometimes" }, "spawn.fork must be"},
		{"marker mode without marker", func(m map[string]any) { m["spawn"].(map[string]any)["fork_marker"] = " " }, "fork_marker is required"},
		{"unknown effort host", func(m map[string]any) { m["spawn"].(map[string]any)["require_effort"] = []any{"cursor"} }, "unknown host"},
		{"advice without message", func(m map[string]any) {
			m["advice"] = []any{map[string]any{"id": "x", "when": "y", "message": ""}}
		}, "message is required"},
		{"advice duplicate id", func(m map[string]any) {
			a := map[string]any{"id": "x", "when": "y", "message": "z"}
			m["advice"] = []any{a, a}
		}, "unique lowercase identifier"},
		{"advice unknown class", func(m map[string]any) {
			m["advice"] = []any{map[string]any{"id": "x", "when": "y", "message": "z", "classes": []any{"boss"}}}
		}, "unknown class"},
		{"ledger too small", func(m map[string]any) { m["ledger"].(map[string]any)["max_bytes"] = 10 }, "ledger.max_bytes"},
		{"containment below floor", func(m map[string]any) { m["main_thread"].(map[string]any)["bash_max_chars"] = 100 }, "bash_max_chars must be 0"},
		{"containment window too wide", func(m map[string]any) {
			m["main_thread"] = map[string]any{"bash_max_chars": 5000, "head_chars": 3000, "tail_chars": 3000}
		}, "must be below bash_max_chars"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			_ = json.Unmarshal(defaultPolicyJSON, &m)
			tt.mutate(m)
			raw, _ := json.Marshal(m)
			_, err := parsePolicy(raw)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyDecodingIsStrict(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown field": strings.Replace(string(defaultPolicyJSON), `"version": 1,`, `"version": 1, "surprise": true,`, 1),
		"trailing data": string(defaultPolicyJSON) + `{}`,
		"oversize":      strings.Repeat(" ", policyMaxBytes+1),
	} {
		if _, err := parsePolicy([]byte(raw)); err == nil {
			t.Errorf("%s: accepted, want an error", name)
		}
	}
}

func TestContainmentCanBeDisabled(t *testing.T) {
	p := policyWith(t, func(m map[string]any) {
		m["main_thread"] = map[string]any{"bash_max_chars": 0, "head_chars": 0, "tail_chars": 0}
	})
	if p.MainThread.BashMaxChars != 0 {
		t.Fatal("bash_max_chars 0 must disable containment")
	}
}

func TestLoadPolicyFallsBackOnInvalidFile(t *testing.T) {
	ta := newTestApp(t)
	if lp := ta.loadPolicy(); lp.Source != "builtin" || lp.Fallback != nil {
		t.Fatalf("absent file: source=%q fallback=%v, want builtin and no fallback", lp.Source, lp.Fallback)
	}
	if err := os.WriteFile(ta.vars["AGENT_HOOKS_POLICY"], []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	lp := ta.loadPolicy()
	if lp.Source != "builtin" || lp.Fallback == nil {
		t.Fatalf("invalid file: source=%q fallback=%v, want builtin with a fallback error", lp.Source, lp.Fallback)
	}
	ta.writePolicy(t, codexPolicy)
	if lp := ta.loadPolicy(); lp.Source != ta.vars["AGENT_HOOKS_POLICY"] || lp.Fallback != nil {
		t.Fatalf("valid file: source=%q fallback=%v", lp.Source, lp.Fallback)
	}
}

func TestPolicyPathResolution(t *testing.T) {
	ta := newTestApp(t)
	delete(ta.vars, "AGENT_HOOKS_POLICY")
	if got := ta.policyPath(); !strings.HasSuffix(got, "home/.config/agent-hooks/policy.json") {
		t.Errorf("default policy path = %q", got)
	}
	ta.vars["XDG_CONFIG_HOME"] = "/xdg"
	if got := ta.policyPath(); got != "/xdg/agent-hooks/policy.json" {
		t.Errorf("XDG policy path = %q", got)
	}
}
