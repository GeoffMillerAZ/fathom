package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const existingCodexHooks = `{
  "hooks": {
    "SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "/x/fathom session-hook", "timeout": 10}]}],
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/x/fathom edit-hook --pre", "timeout": 5}]}],
    "SubagentStop": [{"hooks": [{"type": "command", "command": "/x/fathom stop", "timeout": 5}]}]
  },
  "zzz_other": true
}`

func writeHostFile(t *testing.T, ta *testApp, host, content string) string {
	t.Helper()
	path := ta.hostHookFile(host)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWireAppendsWithoutDisturbingExistingHooks(t *testing.T) {
	ta := newTestApp(t)
	path := writeHostFile(t, ta, hostCodex, existingCodexHooks)
	if code := ta.run([]string{"wire", "--host", hostCodex}); code != 0 {
		t.Fatalf("wire exit %d: %s", code, ta.errOut.String())
	}
	raw, _ := os.ReadFile(path)
	var doc struct {
		Hooks map[string][]struct {
			Matcher string                     `json:"matcher"`
			Hooks   []struct{ Command string } `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	pre := doc.Hooks["PreToolUse"]
	if len(pre) != 2 || pre[0].Hooks[0].Command != "/x/fathom edit-hook --pre" || !strings.Contains(pre[1].Hooks[0].Command, "agent-hooks hook --host codex") {
		t.Fatalf("PreToolUse must keep fathom at index 0 and append agent-hooks: %+v", pre)
	}
	if stop := doc.Hooks["SubagentStop"]; len(stop) != 2 || stop[0].Hooks[0].Command != "/x/fathom stop" {
		t.Fatalf("SubagentStop order changed: %+v", stop)
	}
	if len(doc.Hooks["SessionStart"]) != 1 {
		t.Fatal("an event the pack does not use was modified")
	}
	if i, j := bytes.Index(raw, []byte(`"hooks"`)), bytes.Index(raw, []byte(`"zzz_other"`)); i < 0 || j < i {
		t.Fatal("top-level key order was not preserved")
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want the original 0640", info.Mode().Perm())
	}
	backup, err := os.ReadFile(path + ".agent-hooks.bak")
	if err != nil || string(backup) != existingCodexHooks {
		t.Fatalf("backup missing or altered: %v", err)
	}
}

func TestWireIsIdempotent(t *testing.T) {
	ta := newTestApp(t)
	path := writeHostFile(t, ta, hostClaude, `{"permissions":{"defaultMode":"auto"}}`)
	ta.run([]string{"wire", "--host", hostClaude})
	first, _ := os.ReadFile(path)
	ta.out.Reset()
	if code := ta.run([]string{"wire", "--host", hostClaude}); code != 0 || !strings.Contains(ta.out.String(), "unchanged") {
		t.Fatalf("second wire: exit %d, out %q", code, ta.out.String())
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("a second wire changed the file")
	}
	if !strings.Contains(string(second), `"defaultMode": "auto"`) {
		t.Fatal("unrelated settings were lost")
	}
}

func TestWireCreatesAMissingFile(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run([]string{"wire", "--host", hostCodex}); code != 0 {
		t.Fatalf("exit %d: %s", code, ta.errOut.String())
	}
	info, err := os.Stat(ta.hostHookFile(hostCodex))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("new file: %v mode %v", err, info)
	}
	if _, err := os.Stat(ta.hostHookFile(hostCodex) + ".agent-hooks.bak"); !os.IsNotExist(err) {
		t.Fatal("a backup was written for a file that did not exist")
	}
}

func TestWireDryRunAndRefusals(t *testing.T) {
	ta := newTestApp(t)
	path := writeHostFile(t, ta, hostCodex, existingCodexHooks)
	if code := ta.run([]string{"wire", "--host", hostCodex, "--dry-run"}); code != 0 || !strings.Contains(ta.out.String(), "agent-hooks hook --host codex") {
		t.Fatalf("dry run: exit %d", code)
	}
	if raw, _ := os.ReadFile(path); string(raw) != existingCodexHooks {
		t.Fatal("dry run wrote the file")
	}
	for name, content := range map[string]string{"invalid json": `{"hooks":`, "hooks not an object": `{"hooks":[1]}`, "event not an array": `{"hooks":{"PreToolUse":{}}}`} {
		ta := newTestApp(t)
		path := writeHostFile(t, ta, hostCodex, content)
		if code := ta.run([]string{"wire", "--host", hostCodex}); code != 1 {
			t.Errorf("%s: exit %d, want 1", name, code)
		}
		if raw, _ := os.ReadFile(path); string(raw) != content {
			t.Errorf("%s: file modified on error", name)
		}
	}
	if code := newTestApp(t).run([]string{"wire", "--host", hostCodex, "--bin", "rel/path"}); code != 2 {
		t.Errorf("relative --bin: exit %d, want 2", code)
	}
}

func TestWiredFilePassesDoctorWiring(t *testing.T) {
	ta := newTestApp(t)
	bin := filepath.Join(ta.home, ".local", "bin", "agent-hooks")
	_ = os.MkdirAll(filepath.Dir(bin), 0o755)
	_ = os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755)
	writeHostFile(t, ta, hostClaude, `{}`)
	ta.run([]string{"wire", "--host", hostClaude})
	ta.out.Reset()
	ta.run([]string{"doctor", "--host", hostClaude})
	out := ta.out.String()
	// The registration says "$HOME"/...; doctor must resolve it to this home.
	if strings.Contains(out, "FAIL") || !strings.Contains(out, filepath.Join(ta.home, ".local", "bin", "agent-hooks")) {
		t.Fatalf("doctor on a wired file:\n%s", out)
	}
}
