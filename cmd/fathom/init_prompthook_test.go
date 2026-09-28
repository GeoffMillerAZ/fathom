package main

// init_prompthook_test.go — SR-46 (F-28, docs/design/surface-
// reckoning.md §5): `fathom init` wires the UserPromptSubmit prompt hook
// by default, the same as session-hook/edit-hook/stop-hook.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitWiresPromptHookByDefault is SR-46's reachable_from (cmdInit,
// package cmd/fathom): a fresh `fathom init` writes a UserPromptSubmit
// entry invoking `fathom prompt-hook`, with no extra flag needed.
func TestInitWiresPromptHookByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	svc := strings.ToLower(filepath.Base(dir))
	srv := fakeInitDaemon(t, svc, 123)
	defer srv.Close()

	var out, errBuf bytes.Buffer
	code := cmdInit([]string{"--path", dir, "--url", srv.URL, "--bin", testBinPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}

	settingsRaw, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		t.Fatalf("settings.json not valid JSON: %v\n%s", err, settingsRaw)
	}
	settingsStr := string(settingsRaw)
	for _, want := range []string{
		"UserPromptSubmit",
		testBinPath + " prompt-hook",
	} {
		if !strings.Contains(settingsStr, want) {
			t.Errorf("settings.json missing %q (prompt hook not wired by default):\n%s", want, settingsStr)
		}
	}
}
