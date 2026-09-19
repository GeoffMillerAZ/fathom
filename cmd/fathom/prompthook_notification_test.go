package main

// prompthook_notification_test.go — SU-7 (docs/plans/substrate.md §A,
// ADR-041 decision 3): the prompt hook must never candidate-extract a
// system notification's own prose. Regression fixture: a notification
// mentioning a bare URL and a plain "word/word" phrase, reconstructed
// from the two fabricated candidates the defect actually produced,
// `publication/observation/__prompt__.go` and `cursor.com/blog/
// secure-codebase-indexing/__prompt__.go` (SU-7's own "the notification
// text that produced" fixture).

import (
	"bytes"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// systemNotificationFixture reconstructs the notification text that
// produced the two fabricated candidates: "publication/observation" (a
// bare lowercase word/word phrase from the article's own description) and
// "cursor.com/blog/secure-codebase-indexing" (a scheme-less URL mention).
const systemNotificationFixture = `[SYSTEM NOTIFICATION] New article available: "Secure Codebase Indexing" at cursor.com/blog/secure-codebase-indexing — covers publication/observation workflows for research teams.`

func TestIsSystemNotification(t *testing.T) {
	cases := map[string]bool{
		systemNotificationFixture:                                                    true,
		"[SYSTEM NOTIFICATION] task queue drained":                                   true,
		"before you continue: <task-notification>build finished</task-notification>": true,
		"please fix internal/store/backend.go":                                       false,
		"":                                                                           false,
		// Contract is "begins with", never "contains" — a user prompt that
		// merely quotes/discusses the tag mid-sentence is still real,
		// user-authored text and must be candidate-extracted normally.
		"a prompt that merely mentions [SYSTEM NOTIFICATION] in passing": false,
	}
	for prompt, want := range cases {
		if got := isSystemNotification(prompt); got != want {
			t.Errorf("isSystemNotification(%q) = %v, want %v", prompt, got, want)
		}
	}
}

// TestExtractPromptCandidates_NotificationFixtureNeverMisclassified proves
// that EVEN IF extraction ran on the notification's own text (belt and
// suspenders alongside the isSystemNotification early return below), the
// hardened looksLikePackagePath no longer admits the scheme-less URL
// segment — the "cursor.com/blog/…" half of the original defect.
func TestExtractPromptCandidates_NotificationFixtureNeverMisclassified(t *testing.T) {
	cands := extractPromptCandidates(systemNotificationFixture)
	for _, c := range cands {
		if c.text == "cursor.com/blog/secure-codebase-indexing" {
			t.Errorf("cursor.com/blog/secure-codebase-indexing was extracted as a candidate (kind=%v) — a bare URL mention must never be classified as a repo package path", c.kind)
		}
	}
}

// TestPromptHook_SystemNotification_SilentNoDaemonCall is the end-to-end
// regression proof: the fixture prompt produces NO output and makes ZERO
// daemon calls — extraction never even runs.
func TestPromptHook_SystemNotification_SilentNoDaemonCall(t *testing.T) {
	dir := t.TempDir()
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "the daemon must never be queried for a system notification prompt", 500)
	})
	defer srv.Close()

	promptHookStdin(t, dir, systemNotificationFixture)

	var out, errBuf bytes.Buffer
	code := runWithStdin([]string{"prompt-hook", "--url=" + srv.URL}, nil, &out, &errBuf)
	if code != 0 {
		t.Fatalf("prompt-hook exit = %d, want 0; stderr=%s", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("expected silence for a system notification prompt, got %q", out.String())
	}
	if n := atomic.LoadInt32(calls); n != 0 {
		t.Errorf("daemon queried %d time(s) for a system notification prompt, want 0", n)
	}
	if strings.Contains(out.String(), "__prompt__.go") {
		t.Errorf("fabricated __prompt__.go candidate leaked into output: %s", out.String())
	}
}
