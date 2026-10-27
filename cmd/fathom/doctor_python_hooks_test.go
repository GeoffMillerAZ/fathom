package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

const pyPromptHook = "import os\nos.execvp('fathom', ['fathom', 'prompt-hook'])\n"

// pyPromptHooks rewrites the UserPromptSubmit group of a doctorHookFixture
// root to run cmds, in order.
func pyPromptHooks(t *testing.T, root string, cmds ...string) {
	t.Helper()
	p := filepath.Join(root, ".claude/settings.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var hooks []any
	for _, c := range cmds {
		hooks = append(hooks, map[string]any{"type": "command", "command": c})
	}
	doc["hooks"].(map[string]any)["UserPromptSubmit"] = []any{map[string]any{"matcher": "", "hooks": hooks}}
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// S7: indirect Python hooks read as configured (indirect); an unrelated one
// stays a definite FAIL; a dynamic one that mentions fathom is UNKNOWN for
// that hook only.
func TestDoctorPythonHooks(t *testing.T) {
	for _, cmd := range []string{"python3 .claude/hooks/prompt.py", `python3 "$CLAUDE_PROJECT_DIR"/.claude/hooks/prompt.py`, "python3 ${CLAUDE_PROJECT_DIR}/.claude/hooks/prompt.py"} {
		t.Run("indirect "+cmd, func(t *testing.T) {
			root := doctorHookFixture(t, cmd)
			doctorWriteFixture(t, root, ".claude/hooks/prompt.py", pyPromptHook)
			if got := checkHooksInstalled(root, "claude"); got.Status != doctorPass || !strings.Contains(got.Reason, "configured (indirect)") {
				t.Fatalf("got %+v", got)
			}
		})
	}
	t.Run("unrelated Python hook stays definite", func(t *testing.T) {
		root := doctorHookFixture(t, "python3 .claude/hooks/other.py")
		doctorWriteFixture(t, root, ".claude/hooks/other.py", "import json\nimport yaml\nprint(1)\n")
		got := checkHooksInstalled(root, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Remedy, "fathom init") || !strings.Contains(got.Reason, "prompt-hook") {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("unrelated hook beside the real one", func(t *testing.T) {
		root := doctorHookFixture(t, "unused")
		doctorWriteFixture(t, root, ".claude/hooks/other.py", "print(1)\n")
		doctorWriteFixture(t, root, ".claude/hooks/prompt.py", pyPromptHook)
		pyPromptHooks(t, root, "python3 .claude/hooks/other.py", "python3 .claude/hooks/prompt.py")
		if got := checkHooksInstalled(root, "claude"); got.Status != doctorPass {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("dynamic Python hook is unknown for that hook only", func(t *testing.T) {
		root := doctorHookFixture(t, "python3 .claude/hooks/prompt.py")
		doctorWriteFixture(t, root, ".claude/hooks/prompt.py", "import os, sys\nos.execvp('fathom', sys.argv)\n")
		got := checkHooksInstalled(root, "claude")
		if got.Status != doctorUnknown || !strings.Contains(got.Reason, "prompt-hook") || !strings.Contains(got.Reason, ".claude/hooks/prompt.py:2") {
			t.Fatalf("got %+v", got)
		}
		pyPromptHooks(t, root, "python3 .claude/hooks/prompt.py")
		p := filepath.Join(root, ".claude/settings.json")
		data, _ := os.ReadFile(p)
		var doc map[string]any
		_ = json.Unmarshal(data, &doc)
		delete(doc["hooks"].(map[string]any), "Stop")
		data, _ = json.Marshal(doc)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		got = checkHooksInstalled(root, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "stop-hook") || !strings.Contains(got.Reason, "uncertain project wrapper wiring: prompt-hook") {
			t.Fatalf("uncertainty hid a missing hook: %+v", got)
		}
	})
}

func TestDoctorPythonStdioWrapper(t *testing.T) {
	root := t.TempDir()
	doctorWriteFixture(t, root, ".mcp.json", `{"mcpServers":{"fathom":{"command":"python3","args":["scripts/mcp.py"]}}}`)
	doctorWriteFixture(t, root, "scripts/mcp.py", "import os\nos.execvp(\"fathom\", [\"fathom\", \"serve-mcp\"])\n")
	got, ok := checkProjectMCPRegistration(context.Background(), root)
	if !ok || got.Status != doctorPass || !strings.Contains(got.Reason, "configured (indirect)") {
		t.Fatalf("got %+v", got)
	}
	doctorWriteFixture(t, root, "scripts/mcp.py", "import os, sys\nos.execvp(\"fathom\", sys.argv)  # fathom\n")
	got, _ = checkProjectMCPRegistration(context.Background(), root)
	if got.Status != doctorUnknown || !strings.Contains(got.Reason, "scripts/mcp.py:2") {
		t.Fatalf("got %+v", got)
	}
}

// Readiness reads a Python wrapper through the same reader; with the Python
// reader off the wrapper would read as an unregistered hook.
func TestReadinessInstalledPythonWrapper(t *testing.T) {
	root, _ := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	d, _ := harness.Lookup(harness.ClaudeCode)
	hooks := map[string][]any{}
	var script strings.Builder
	script.WriteString("import subprocess\n")
	for _, b := range d.Hooks {
		if b.Capability != harness.CapReadout {
			continue
		}
		hooks[b.Harness] = append(hooks[b.Harness], map[string]any{"matcher": b.Matcher, "hooks": []any{map[string]any{"type": "command", "command": "python3 ./hook-wrapper.py"}}})
		script.WriteString("subprocess.run([\"fathom\", \"" + strings.Join(strings.Fields(b.Verb), "\", \"") + "\"])\n")
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(mustJSON(map[string]any{"hooks": hooks})), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hook-wrapper.py"), []byte(script.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, harness.CapReadout); !got.ready {
		t.Fatalf("registered Python wrapper: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "hook-wrapper.py"), []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readinessInstalled(root, harness.ClaudeCode, harness.CapReadout); got.ready || got.reason != "hook-not-registered" {
		t.Fatalf("unrelated Python wrapper: %+v", got)
	}
}
