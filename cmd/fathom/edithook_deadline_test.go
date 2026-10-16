package main

// edithook_deadline_test.go — EW-76 (G11): a hook-path call is sized to
// what its hook has left, and the post-edit answer that the daemon could not
// give names its silence (GL-24). No wall-clock wait: the sized timeout is
// read straight after the guard starts, against a margin of hundreds of
// milliseconds.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

func TestHookCallTimeoutIsWhatTheHookHasLeft(t *testing.T) {
	if got := hookCallTimeout(postEditReserve, 2*time.Second); got != 2*time.Second {
		t.Errorf("outside a hook: %v, want the 2 s fallback", got)
	}
	// The edit hook's descriptor: TimeoutSec 5, a 4 s deadline.
	restore := activateHookGuard(&hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now(), 5)})
	got := hookCallTimeout(postEditReserve, 2*time.Second)
	restore()
	if got <= 3*time.Second || got > 3500*time.Millisecond {
		t.Errorf("4 s deadline less a 500 ms reserve: %v, want about 3.5 s, not the old fixed 2 s", got)
	}
	restore = activateHookGuard(&hookguard.Guard{Deadline: hookguard.HookDeadline(time.Now().Add(-time.Hour), 5)})
	got = hookCallTimeout(postEditReserve, 2*time.Second)
	restore()
	if got != hookCallFloor {
		t.Errorf("a spent deadline: %v, want the %v floor", got, hookCallFloor)
	}
}

// TestPostEditDaemonFailureNamesItsSilence: an obligations call that fails
// (or runs out of time) leaves the agent nothing, and the answered beat
// says daemon-error beside the error beat that names the cause.
func TestPostEditDaemonFailureNamesItsSilence(t *testing.T) {
	isolateHookRun(t)
	logPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", logPath)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()
	dir := t.TempDir()
	in := fmt.Sprintf(`{"session_id":"s1","hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q}`,
		filepath.Join(dir, "some", "file.go"), dir)
	if code, out, _ := runHookVerb(t, []string{"edit-hook", "--url=" + srv.URL}, in); code != 0 || out != "" {
		t.Fatalf("exit %d output %q, want 0 and silence", code, out)
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	var named bool
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == "answered" && b.Meta["phase"] == "post" && b.Meta["emitted"] == "false" {
			if b.Meta["reason"] != string(silenceDaemonError) {
				t.Errorf("answered beat reason %q, want %s", b.Meta["reason"], silenceDaemonError)
			}
			named = true
		}
	}
	if !named {
		t.Errorf("no answered beat names the silence; beats: %v", beats)
	}
}

// TestPreEditDaemonFailureNamesItsSilence: on a guarded file whose
// obligations call fails (or runs out of the pre-edit budget), the answered
// beat names daemon-error, never a bare error beat alone (GL-24).
func TestPreEditDaemonFailureNamesItsSilence(t *testing.T) {
	isolateHookRun(t)
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })
	dir := t.TempDir()
	writeGuardCue(t, dir, `guards: [{scopes: ["internal/store/**"], note: "Store schema is spec-governed.", max_rules: 4}]`)
	file := filepath.Join(dir, "internal", "store", "db.go")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rpcRequestName(r) == "obligations" {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		payload := map[string]any{"service": strings.ToLower(filepath.Base(dir)), "file": "internal/store/db.go"}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": mustJSON(payload)}}}})
	}))
	defer srv.Close()
	in := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"sess-1"}`, file, dir)
	if code, out, _ := runHookVerb(t, []string{"edit-hook", "--pre", "--url=" + srv.URL}, in); code != 0 || out != "" {
		t.Fatalf("exit %d output %q, want 0 and silence", code, out)
	}
	beats, _ := readBeatLog(os.Getenv("FATHOM_HOOKS_LOG"))
	var named bool
	for _, b := range beats {
		if b.Channel == "edit-hook" && b.Kind == "answered" && b.Meta["phase"] == "pre" && b.Meta["emitted"] == "false" {
			if b.Meta["reason"] != string(silenceDaemonError) {
				t.Errorf("answered beat reason %q, want %s", b.Meta["reason"], silenceDaemonError)
			}
			named = true
		}
	}
	if !named {
		t.Errorf("no answered beat names the pre-edit silence; beats: %v", beats)
	}
}
