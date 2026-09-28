package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// PE-19 (docs/design/progress-and-economics.md §5): the live session feed
// renders read-only, over the SSE channel's existing "sessions" live
// source (console_live.go, ADR-045) — no fetch of its own, just an
// ordinary page load on the "Updated" notice. renderSessionFeed itself is
// a pure fragment over rows the caller already built; sessionFeedRow
// carries no session id or other per-person figure at all (I58), and the
// row's DOM id is purely positional.
func TestConsoleSessionFeed(t *testing.T) {
	rows := []sessionFeedRow{
		{Harness: "claude-code", Model: "sonnet", Repo: "payment-service",
			Start: "2026-09-28T10:00:00Z", End: "2026-09-28T10:45:00Z", Proven: console.Measured(3, "the proof history joined to the session ledger")},
		{Harness: "codex", Repo: "payment-service",
			Start: "2026-09-28T11:00:00Z", End: "", Proven: console.Absent[int]("no claimed-proven transition joined to this session yet")},
	}
	out := renderSessionFeed(rows)

	for _, want := range []string{"session-feed", "claude-code", "sonnet", "payment-service", "2026-09-28T10:00:00Z", "3", "codex", console.ModelNotRecorded, "in progress", "session-feed-0", "session-feed-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("session feed missing %q:\n%s", want, out)
		}
	}

	if empty := renderSessionFeed(nil); !strings.Contains(empty, "data-session-feed-empty") {
		t.Errorf("an empty feed must say so, never render an empty table: %s", empty)
	}
}
