package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// TestConfigChangeHook pins PC-24: the ConfigChange envelope runs the
// custodian audit for the repo and writes one custodian/config-change
// beat {source, file_path, result, violated} (+ session); exit 0 always;
// nothing on stdout (a block would surface no message in the harness, so
// none is ever emitted).
func TestConfigChangeHook(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	t.Setenv("HOME", t.TempDir()) // no user-scope concerns
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	root := t.TempDir()
	codec := harness.ClaudeCodec()

	envelope := func(cwd string) *strings.Reader {
		return strings.NewReader(`{"session_id":"s-7","cwd":"` + cwd + `","hook_event_name":"ConfigChange","source":"project_settings","file_path":"` + cwd + `/.claude/settings.json"}`)
	}

	// No concerns registered: the beat says so.
	if code := configChangeHook(codec, envelope(root)); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	beats := loadBeatsAt(t, beatsPath)
	if len(beats) != 1 || beats[0].Channel != "custodian" || beats[0].Kind != configChangeBeatKind {
		t.Fatalf("beats = %+v, want one custodian/config-change beat", beats)
	}
	b := beats[0]
	for k, want := range map[string]string{"source": "project_settings", "file_path": root + "/.claude/settings.json", "result": "no-concerns", "session": "s-7"} {
		if b.Meta[k] != want {
			t.Errorf("meta[%s] = %q, want %q", k, b.Meta[k], want)
		}
	}

	// A registered, violated concern: the count rides the beat, and the
	// next session's attention line (attentionLine) reads the same count.
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	concerns := "concerns: {\n\tno_cp: {\n\t\twant: \"no raw cp\"\n\t\tcheck: {kind: \"forbidden_pattern\", pattern: \"cp bin/fathom\", where: \"agent-config\"}\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(root, ".fathom", "concerns.cue"), []byte(concerns), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("deploy: cp bin/fathom /usr/local/bin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := configChangeHook(codec, envelope(root)); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	beats = loadBeatsAt(t, beatsPath)
	if len(beats) != 2 {
		t.Fatalf("beats = %d, want 2", len(beats))
	}
	if got := beats[1].Meta; got["result"] != "audited" || got["violated"] != "1" {
		t.Errorf("audited beat meta = %v, want result=audited violated=1", got)
	}
	if n := violatedConcernCount(root); n != 1 {
		t.Errorf("violatedConcernCount = %d, want 1 (what the next session line surfaces)", n)
	}

	// A concerns file that will not load: unable-to-check, never a
	// violation, still exit 0.
	if err := os.WriteFile(filepath.Join(root, ".fathom", "concerns.cue"), []byte("concerns: {broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := configChangeHook(codec, envelope(root)); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	beats = loadBeatsAt(t, beatsPath)
	if len(beats) != 3 || beats[2].Meta["result"] != "unable-to-check" {
		t.Errorf("load-error beat = %+v, want result=unable-to-check", beats[len(beats)-1])
	}

	// Malformed stdin: nothing recorded, exit 0.
	if code := configChangeHook(codec, strings.NewReader("{oops")); code != 0 {
		t.Errorf("malformed stdin exit = %d", code)
	}
	if got := len(loadBeatsAt(t, beatsPath)); got != 3 {
		t.Errorf("malformed stdin wrote a beat: %d", got)
	}
}

// TestCmdConfigAudit_HookFlag drives the verb: `config audit --hook` with
// an unknown --harness is I9 fail-open (one stderr line, exit 0), and the
// kill switch silences it.
func TestCmdConfigAudit_HookFlag(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	t.Setenv("FATHOM_CHANNELS", "")
	var stdout, stderr strings.Builder
	if code := cmdConfigAudit([]string{"--hook", "--harness", "nope"}, &stdout, &stderr); code != 0 || !strings.Contains(stderr.String(), "unknown --harness") {
		t.Errorf("unknown harness: exit %d, stderr %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	t.Setenv("FATHOM_CHANNELS", "off")
	stderr.Reset()
	if code := cmdConfigAudit([]string{"--hook"}, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.Len() != 0 {
		t.Errorf("kill switch: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(beatsPath); err == nil {
		t.Error("a beat was written under the kill switch")
	}
}
