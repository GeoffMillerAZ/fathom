package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestMachineMintedThreadsCarryConditions pins IT-9: both machine-minted
// note kinds carry a close condition at mint, so evaluateThreadConditions
// can retire them instead of leaving them to rot in the open set forever.
//
//   - session-end notes: repo_clean (already an existing checkable form —
//     pinned here as a regression net) — closes once the named tree is
//     clean again.
//   - pre-compact notes: session_ended, keyed on session_id — closes once a
//     session-end note for that SAME session exists in the log.
func TestMachineMintedThreadsCarryConditions(t *testing.T) {
	threadEnv(t)
	t.Setenv("FATHOM_CHANNELS", "")

	gitRepo := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "x"}} {
			cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
			if err := cmd.Run(); err != nil {
				t.Skipf("git unavailable: %v", err)
			}
		}
		return root
	}

	t.Run("session-end notes carry repo_clean and self-close on a clean tree", func(t *testing.T) {
		threadEnv(t)
		root := gitRepo(t)
		if err := os.WriteFile(filepath.Join(root, "wip.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		in := map[string]any{"session_id": "sess-se-1", "cwd": root, "reason": "clear"}
		raw, _ := json.Marshal(in)
		defer pipeStdin(t, string(raw))()
		var out, errb bytes.Buffer
		if code := cmdSessionEndHook(nil, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		items := loadThreadItems(threadsPath())
		if len(items) != 1 {
			t.Fatalf("want exactly one deposit, got %d", len(items))
		}
		it := items[0]
		if it.CompletesWhen == nil {
			t.Fatal("session-end note must carry a close condition at mint")
		}
		if it.CompletesWhen.Kind != "repo_clean" {
			t.Errorf("session-end condition kind = %q, want repo_clean", it.CompletesWhen.Kind)
		}

		// Dirty tree: stays open.
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
		if items[0].ClosedAt != "" {
			t.Fatal("the tree is still dirty; the note must stay open")
		}

		// Commit everything: tree is clean, note self-closes.
		cmd := exec.Command("git", "-C", root, "-c", "user.email=t@t", "-c", "user.name=t", "add", "-A")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "-C", root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "wip")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
		if items[0].ClosedAt == "" {
			t.Fatal("the tree is clean; the session-end note must self-close")
		}
		if items[0].ClosedBy != "condition" {
			t.Errorf("closed_by = %q, want condition", items[0].ClosedBy)
		}
	})

	t.Run("pre-compact notes carry session_ended and close once a same-session session-end note exists", func(t *testing.T) {
		threadEnv(t)
		root := gitRepo(t)
		in := map[string]any{"session_id": "sess-pc-1", "cwd": root, "trigger": "auto"}
		raw, _ := json.Marshal(in)
		defer pipeStdin(t, string(raw))()
		var out, errb bytes.Buffer
		if code := cmdPreCompactHook(nil, &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		items := loadThreadItems(threadsPath())
		if len(items) != 1 {
			t.Fatalf("want exactly one deposit, got %d", len(items))
		}
		it := items[0]
		if it.CompletesWhen == nil {
			t.Fatal("pre-compact note must carry a close condition at mint")
		}
		if it.CompletesWhen.Kind != "session_ended" {
			t.Errorf("pre-compact condition kind = %q, want session_ended", it.CompletesWhen.Kind)
		}
		if it.CompletesWhen.SessionID != "sess-pc-1" {
			t.Errorf("pre-compact condition session_id = %q, want sess-pc-1", it.CompletesWhen.SessionID)
		}

		// No session-end note yet for this session: stays open.
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
		if items[0].ClosedAt != "" {
			t.Fatal("no session-end note for this session exists yet; the pre-compact note must stay open")
		}

		// A session-end note for a DIFFERENT session must not close it.
		seedItem(t, "se-other", "session-end", "session ended", func(it *threadItem) {
			it.Pointers = map[string]string{"session_id": "sess-different"}
		})
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
		for _, x := range items {
			if x.ID == it.ID && x.ClosedAt != "" {
				t.Fatal("a session-end note for a DIFFERENT session must not close this pre-compact note")
			}
		}

		// A session-end note for the SAME session: closes it.
		seedItem(t, "se-"+it.ID, "session-end", "session ended", func(it2 *threadItem) {
			it2.Pointers = map[string]string{"session_id": "sess-pc-1"}
		})
		items = evaluateThreadConditions(loadThreadItems(threadsPath()), root, time.Now())
		var found bool
		for _, x := range items {
			if x.ID == it.ID {
				found = true
				if x.ClosedAt == "" {
					t.Fatal("a session-end note for the SAME session exists; the pre-compact note must close")
				}
				if x.ClosedBy != "condition" {
					t.Errorf("closed_by = %q, want condition", x.ClosedBy)
				}
			}
		}
		if !found {
			t.Fatal("pre-compact item vanished from the loaded set")
		}
	})
}
