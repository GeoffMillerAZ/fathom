package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// TestStopHookLandsRetroArtifact pins IT-13: the Stop hook mechanically
// lands a retro artifact for the session it just ended (`fathom retro
// --write`, session omitted in the CLI but resolved explicitly here from
// the hook's own stdin — LLM-free, zero tokens) so the firings→outcomes
// join becomes a standing corpus instead of a run-on-request.
func TestStopHookLandsRetroArtifact(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "ledger.jsonl"))
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	t.Setenv("FATHOM_CHANNELS", "")
	resetObsCache()

	const sessionID = "retro-session-1"
	now := time.Now().UTC()
	beatsPath := writeBeatsFixture(t, []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "answered", At: now, Meta: map[string]string{
			"session": sessionID, "project_dir": repo, "emitted": "true", "phase": "post", "file": "main.go",
		}},
	})
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)

	hook := map[string]any{
		"stop_hook_active": false,
		"cwd":              repo,
		"session_id":       sessionID,
		"transcript_path":  filepath.Join(t.TempDir(), "nope.jsonl"),
	}
	raw, _ := json.Marshal(hook)
	defer pipeStdin(t, string(raw))()

	var out, errb bytes.Buffer
	if code := cmdStopHook(nil, &out, &errb); code != 0 {
		t.Fatalf("a stop hook must never block session end: exit %d, %s", code, errb.String())
	}

	name := sessionID
	if len(name) > 8 {
		name = name[:8]
	}
	artifact := filepath.Join(repo, "workspace", "artifacts", "retros", name+".json")
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("expected a retro artifact at %s: %v", artifact, err)
	}
	var m retroModel
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("bad retro artifact: %v\n%s", err, data)
	}
	if m.Session != sessionID {
		t.Errorf("retro artifact session = %q, want %q", m.Session, sessionID)
	}
	if m.Injections != 1 {
		t.Errorf("retro artifact Injections = %d, want 1 (the seeded edit-hook beat)", m.Injections)
	}
}

// TestStopHookRetroFailureNeverBreaksHook pins the non-fatal contract: a
// retro that cannot even resolve a session (no beats for this project)
// must not change the hook's own exit code or block session end.
func TestStopHookRetroFailureNeverBreaksHook(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("FATHOM_SESSION_LEDGER", filepath.Join(t.TempDir(), "ledger.jsonl"))
	t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "arm.json"))
	t.Setenv("FATHOM_NARRATE_STATE", filepath.Join(t.TempDir(), "narrate.json"))
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "missing-hooks.jsonl"))
	t.Setenv("FATHOM_CHANNELS", "")
	resetObsCache()

	hook := map[string]any{
		"stop_hook_active": false,
		"cwd":              repo,
		"session_id":       "no-beats-session",
		"transcript_path":  filepath.Join(t.TempDir(), "nope.jsonl"),
	}
	raw, _ := json.Marshal(hook)
	defer pipeStdin(t, string(raw))()

	var out, errb bytes.Buffer
	if code := cmdStopHook(nil, &out, &errb); code != 0 {
		t.Fatalf("a hook with nothing to retro must still exit 0: %d, %s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "workspace", "artifacts", "retros")); err == nil {
		t.Error("no beats existed for this project — no retro artifact should have been written")
	}
}
