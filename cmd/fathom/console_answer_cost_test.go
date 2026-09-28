package main

// console_answer_cost_test.go — PE-21 (docs/plans/progress-and-economics.md,
// docs/design/progress-and-economics.md §5): the console shows the cost of
// each answer on the call-log pages. renderAnswerCost renders PE-20's
// per-receipt token estimate on the project workspace's Recent-calls table
// (console_project_activity.go).

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestConsoleAnswerCost pins renderAnswerCost's two states — an accounted
// answer renders its estimate, an unaccounted one (a call logged before
// PE-20's estimate existed, or an absent field) renders an explicit dash,
// never a fabricated zero-cost answer — and that activityRecentRow, the
// Recent-calls row builder, actually wires the estimate through rather than
// re-deriving it.
func TestConsoleAnswerCost(t *testing.T) {
	cases := []struct {
		tokens int
		want   string
	}{
		{0, "—"},
		{-1, "—"},
		{1, "~1 tok"},
		{1234, "~1,234 tok"},
	}
	for _, c := range cases {
		if got := renderAnswerCost(c.tokens); got != c.want {
			t.Errorf("renderAnswerCost(%d) = %q, want %q", c.tokens, got, c.want)
		}
	}

	accounted := console.ActivityCall{At: "2026-09-27T10:00:00Z", Tool: "search", Surface: "agent", LatencyMS: 12, ResponseTokens: 42}
	row := activityRecentRow(0, accounted)
	if len(row.Cells) < 5 || row.Cells[4] != "~42 tok" {
		t.Fatalf("activityRecentRow cost cell = %+v, want \"~42 tok\" carried from ResponseTokens", row.Cells)
	}
	if !strings.Contains(row.Cells[1], "search") {
		t.Fatalf("activityRecentRow must still carry the tool cell: %+v", row.Cells)
	}

	unaccounted := console.ActivityCall{At: "2026-09-27T09:00:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 3}
	row2 := activityRecentRow(1, unaccounted)
	if len(row2.Cells) < 5 || row2.Cells[4] != "—" {
		t.Fatalf("activityRecentRow cost cell for a row logged before PE-20 = %+v, want an explicit dash, never 0 tok", row2.Cells)
	}
}
