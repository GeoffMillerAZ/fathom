package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestRewriteBareFathomHooks (O103): only a command whose first token is the
// bare "fathom" is rewritten to the cell binary; absolute and foreign
// commands stay byte-identical, and the rewritten file still satisfies
// assertWiredHooksPresent.
func TestRewriteBareFathomHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	src := `{"hooks":{
	  "SessionStart":[{"hooks":[{"type":"command","command":"/cell/fathom session-hook"}]}],
	  "PreToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"fathom edit-hook --pre"}]}],
	  "PostToolUse":[{"hooks":[{"type":"command","command":"fathom edit-hook"},{"type":"command","command":"/usr/bin/true"},{"type":"command","command":"/usr/bin/other run"},{"type":"command","command":"command -v fathom >/dev/null 2>&1 || exit 0; fathom session-hook --event test-run"}]}]
	}}`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := rewriteBareFathomHooks(path, "/cell/fathom")
	if err != nil || n != 3 {
		t.Fatalf("rewrite: n=%d err=%v, want 3 rewrites", n, err)
	}
	var got map[string]any
	if err := json.Unmarshal(mustRead(t, path), &got); err != nil {
		t.Fatal(err)
	}
	cmds := hookCommands(got)
	want := map[string]bool{"/cell/fathom session-hook": true, "/cell/fathom edit-hook --pre": true, "/cell/fathom edit-hook": true, "/usr/bin/true": true, "/usr/bin/other run": true, "command -v /cell/fathom >/dev/null 2>&1 || exit 0; /cell/fathom session-hook --event test-run": true}
	for _, c := range cmds {
		if !want[c] {
			t.Errorf("unexpected command after rewrite: %q", c)
		}
	}
	if len(cmds) != len(want) {
		t.Fatalf("commands after rewrite = %v", cmds)
	}
	if err := assertWiredHooksPresent(path); err != nil {
		t.Fatalf("rewritten settings still refused: %v", err)
	}
	// Idempotent: nothing left to rewrite, file untouched.
	before := mustRead(t, path)
	if n, err := rewriteBareFathomHooks(path, "/cell/fathom"); err != nil || n != 0 {
		t.Fatalf("second pass: n=%d err=%v", n, err)
	}
	if string(before) != string(mustRead(t, path)) {
		t.Fatal("second pass rewrote the file")
	}
	if n, err := rewriteBareFathomHooks(filepath.Join(dir, "missing.json"), "/cell/fathom"); err != nil || n != 0 {
		t.Fatalf("missing file: n=%d err=%v", n, err)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func hookCommands(settings map[string]any) []string {
	var out []string
	hooksObj, _ := settings["hooks"].(map[string]any)
	for _, groups := range hooksObj {
		arr, _ := groups.([]any)
		for _, g := range arr {
			gm, _ := g.(map[string]any)
			hs, _ := gm["hooks"].([]any)
			for _, h := range hs {
				hm, _ := h.(map[string]any)
				if c, ok := hm["command"].(string); ok {
					out = append(out, c)
				}
			}
		}
	}
	return out
}

// TestAssertWiredHooksGuardedForm: fathom init writes every hook as
// `command -v <bin> >/dev/null 2>&1 || exit 0; <bin> <sub>`; the bare-binary
// check reads the command the guard runs, not the guard's own `command`.
func TestAssertWiredHooksGuardedForm(t *testing.T) {
	write := func(bin string) string {
		p := filepath.Join(t.TempDir(), "settings.json")
		g := "command -v " + bin + " >/dev/null 2>&1 || exit 0; " + bin
		src := `{"hooks":{
	  "SessionStart":[{"hooks":[{"type":"command","command":"` + g + ` session-hook"}]}],
	  "PreToolUse":[{"matcher":"Edit|Write|MultiEdit","hooks":[{"type":"command","command":"` + g + ` edit-hook --pre"}]}]
	}}`
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := assertWiredHooksPresent(write("/cell/fathom")); err != nil {
		t.Errorf("guarded absolute hooks refused: %v", err)
	}
	if err := assertWiredHooksPresent(write("fathom")); err == nil {
		t.Error("guarded bare hooks accepted, want the bare-binary refusal")
	}
}
