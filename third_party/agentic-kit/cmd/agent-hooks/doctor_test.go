package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// installHooks writes a host's hook configuration the way an adopter would:
// the print-hooks output, pointing at a fake executable.
func installHooks(t *testing.T, ta *testApp, host string, extra map[string]any) string {
	t.Helper()
	bin := filepath.Join(ta.dir, "bin", "agent-hooks")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tmpl, err := hookTemplate(host, bin)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(tmpl), &doc); err != nil {
		t.Fatalf("template is not JSON: %v", err)
	}
	for k, v := range extra {
		doc["hooks"].(map[string]any)[k] = v
	}
	raw, _ := json.Marshal(doc)
	dir, name := ta.claudeHome(), "settings.json"
	if host == hostCodex {
		dir, name = ta.codexHome(), "hooks.json"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDoctorPassesOnAWiredHost(t *testing.T) {
	ta := newTestApp(t)
	installHooks(t, ta, hostClaude, nil)
	ta.hook(t, hostClaude, claudeSpawn("s1", map[string]any{"prompt": "x", "model": "haiku"})) // a live ledger
	ta.out.Reset()
	code := ta.run([]string{"doctor", "--host", hostClaude})
	out := ta.out.String()
	if code != 0 {
		t.Fatalf("doctor exit = %d\n%s", code, out)
	}
	for _, want := range []string{`PASS\s+self-test`, `PASS\s+wiring claude\s+PreToolUse Agent`, `PASS\s+wiring claude\s+PostToolUse Bash`, `PASS\s+ledger`} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("doctor output lacks %q:\n%s", want, out)
		}
	}
}

func TestDoctorFailsWhenUnwired(t *testing.T) {
	ta := newTestApp(t)
	if err := os.MkdirAll(ta.claudeHome(), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(ta.claudeHome(), "settings.json"), []byte(`{"hooks":{}}`), 0o600)
	if code := ta.run([]string{"doctor", "--host", hostClaude}); code != 1 {
		t.Fatalf("doctor exit = %d, want 1 for missing wiring\n%s", code, ta.out.String())
	}
	if !regexp.MustCompile(`FAIL\s+wiring claude`).MatchString(ta.out.String()) {
		t.Fatalf("doctor output:\n%s", ta.out.String())
	}
}

func TestDoctorFailsOnRejectedPolicy(t *testing.T) {
	ta := newTestApp(t)
	_ = os.WriteFile(ta.vars["AGENT_HOOKS_POLICY"], []byte(`{"version":9}`), 0o600)
	if code := ta.run([]string{"doctor", "--host", hostClaude}); code != 1 || !regexp.MustCompile(`FAIL\s+policy`).MatchString(ta.out.String()) {
		t.Fatalf("doctor exit = %d\n%s", code, ta.out.String())
	}
}

func TestDoctorWarnsOnASecondSpawnGate(t *testing.T) {
	ta := newTestApp(t)
	installHooks(t, ta, hostClaude, map[string]any{"PreToolUse": []any{
		map[string]any{"matcher": "Agent|Task", "hooks": []any{map[string]any{"type": "command", "command": filepath.Join(ta.dir, "bin", "agent-hooks") + " hook --host claude"}}},
		map[string]any{"matcher": "Agent", "hooks": []any{map[string]any{"type": "command", "command": "/old/pre-tool-use.sh"}}},
	}})
	ta.run([]string{"doctor", "--host", hostClaude})
	if !strings.Contains(ta.out.String(), "two gates decide each spawn") {
		t.Fatalf("legacy gate not flagged:\n%s", ta.out.String())
	}
}

func TestDoctorChecksCodexTrust(t *testing.T) {
	ta := newTestApp(t)
	path := installHooks(t, ta, hostCodex, nil)
	trusted := fmt.Sprintf("[hooks.state.\"%s:pre_tool_use:0:0\"]\ntrusted_hash = \"sha256:x\"\n", path)
	_ = os.WriteFile(filepath.Join(ta.codexHome(), "config.toml"), []byte(trusted), 0o600)
	ta.run([]string{"doctor", "--host", hostCodex})
	out := ta.out.String()
	if !regexp.MustCompile(`PASS\s+trust codex\s+PreToolUse`).MatchString(out) {
		t.Errorf("trusted PreToolUse not recognized:\n%s", out)
	}
	if !strings.Contains(out, "PostToolUse handler not trusted yet") {
		t.Errorf("untrusted PostToolUse not flagged:\n%s", out)
	}
}

// One Codex PostToolUse handler satisfies both the spawn-tool and Bash
// requirements; its trust line must appear once, not look like two handlers.
func TestDoctorCodexTrustLinePerHandler(t *testing.T) {
	ta := newTestApp(t)
	installHooks(t, ta, hostCodex, nil)
	_ = os.WriteFile(filepath.Join(ta.codexHome(), "config.toml"), []byte(""), 0o600)
	ta.run([]string{"doctor", "--host", hostCodex})
	out := ta.out.String()
	if n := strings.Count(out, "PostToolUse handler not trusted yet"); n != 1 {
		t.Errorf("PostToolUse trust line printed %d times, want 1:\n%s", n, out)
	}
	lines := map[string]int{}
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "trust codex") {
			lines[strings.TrimSpace(l)]++
		}
	}
	for l, n := range lines {
		if n != 1 {
			t.Errorf("duplicate trust line %q x%d", l, n)
		}
	}
}

func TestDoctorReportsAgentDrift(t *testing.T) {
	ta := newTestApp(t)
	for _, ag := range agentAssets() {
		if ag.Host != hostClaude {
			continue
		}
		path := ta.agentInstallPath(ag)
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		content := ag.Content
		if strings.Contains(ag.Name, "builder") {
			content = []byte("edited locally")
		}
		_ = os.WriteFile(path, content, 0o600)
	}
	ta.run([]string{"doctor", "--host", hostClaude})
	out := ta.out.String()
	if !regexp.MustCompile(`PASS\s+agent claude\s+cavecrew-investigator\.md`).MatchString(out) || !strings.Contains(out, "cavecrew-builder.md differs") {
		t.Fatalf("agent drift report:\n%s", out)
	}
}

func TestMatcherCovers(t *testing.T) {
	for _, c := range []struct {
		matcher, tool string
		want          bool
	}{
		{"Agent|Task", "Agent", true},
		{"Agent|Task", "Agents", false},
		{"", "Bash", true},
		{"*", "Bash", true},
		{"spawn_agent|Bash", "Bash", true},
		{"(", "Bash", false},
		{"Bash", "", true},
	} {
		if got := matcherCovers(c.matcher, c.tool); got != c.want {
			t.Errorf("matcherCovers(%q, %q) = %v", c.matcher, c.tool, got)
		}
	}
}

func TestExpandHome(t *testing.T) {
	ta := newTestApp(t)
	for in, want := range map[string]string{
		"~/.local/bin/agent-hooks":       filepath.Join(ta.home, ".local/bin/agent-hooks"),
		"$HOME/.local/bin/agent-hooks":   filepath.Join(ta.home, ".local/bin/agent-hooks"),
		"${HOME}/.local/bin/agent-hooks": filepath.Join(ta.home, ".local/bin/agent-hooks"),
		"/opt/agent-hooks":               "/opt/agent-hooks",
	} {
		if got := ta.expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeEventAndFirstField(t *testing.T) {
	if got := snakeEvent("PreToolUse"); got != "pre_tool_use" {
		t.Errorf("snakeEvent = %q", got)
	}
	if got := snakeEvent("SubagentStop"); got != "subagent_stop" {
		t.Errorf("snakeEvent = %q", got)
	}
	if got := firstField("'/a b/agent-hooks' hook --host codex"); got != "/a b/agent-hooks" {
		t.Errorf("firstField quoted = %q", got)
	}
	if got := firstField("/bin/agent-hooks hook"); got != "/bin/agent-hooks" {
		t.Errorf("firstField = %q", got)
	}
}
