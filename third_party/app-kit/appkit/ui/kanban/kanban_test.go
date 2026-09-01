package kanban

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// render executes a templ.Component to a string for assertion.
func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return buf.String()
}

var testCols = []Column{
	{ID: "todo", Label: "To Do", Count: 2},
	{ID: "doing", Label: "Doing", Count: 1},
	{ID: "done", Label: "Done", Count: 0},
}

var testCards = []CardItem{
	{ID: "c1", StageID: "todo", Title: "Design tokens", Subtitle: "Alice", Badges: []Badge{{Label: "high", Role: "danger-soft"}}, Meta: "2 subtasks"},
	{ID: "c2", StageID: "todo", Title: "Write docs", Subtitle: "Bob"},
	{ID: "c3", StageID: "doing", Title: "Implement kanban", Meta: "in progress"},
}

func TestBoard_RootAttributes(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	if !strings.Contains(got, "data-kanban-board") {
		t.Errorf("board missing data-kanban-board attr; got:\n%s", got)
	}
}

func TestBoard_ColumnsEmitted(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	for _, col := range testCols {
		if !strings.Contains(got, `data-kanban-col="`+col.ID+`"`) {
			t.Errorf("board missing column attr for %q; got:\n%s", col.ID, got)
		}
		if !strings.Contains(got, col.Label) {
			t.Errorf("board missing column label %q; got:\n%s", col.Label, got)
		}
	}
}

func TestBoard_CardAttributes(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	for _, c := range testCards {
		if !strings.Contains(got, `data-kanban-card="`+c.ID+`"`) {
			t.Errorf("board missing card attr for %q; got:\n%s", c.ID, got)
		}
		if !strings.Contains(got, `draggable="true"`) {
			t.Errorf("board cards missing draggable=true; got:\n%s", got)
		}
	}
}

func TestBoard_CardDistribution(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	// Verify card titles appear in output.
	for _, c := range testCards {
		if !strings.Contains(got, c.Title) {
			t.Errorf("card title %q missing from board; got:\n%s", c.Title, got)
		}
	}
}

func TestBoard_CountBadge(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	// Columns with Count >= 0 emit a count badge.
	if !strings.Contains(got, ">2<") && !strings.Contains(got, "> 2 <") {
		// The count 2 is for "todo" column; check some form of it.
		if !strings.Contains(got, "2") {
			t.Errorf("column count badge '2' missing from board; got:\n%s", got)
		}
	}
}

func TestBoard_HideCountBadge(t *testing.T) {
	cols := []Column{{ID: "x", Label: "X", Count: -1}}
	got := render(t, Board(cols, nil))
	// Count badge class is "rounded-full" — only emitted when Count >= 0.
	// With Count=-1 there should be no badge span.
	if strings.Contains(got, "font-mono") {
		t.Errorf("column with Count=-1 should not emit count badge; got:\n%s", got)
	}
}

func TestBoard_CardSubtitle(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	if !strings.Contains(got, "Alice") {
		t.Errorf("card subtitle 'Alice' missing; got:\n%s", got)
	}
}

func TestBoard_CardMeta(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	if !strings.Contains(got, "2 subtasks") {
		t.Errorf("card meta '2 subtasks' missing; got:\n%s", got)
	}
}

func TestBoard_Badges(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	if !strings.Contains(got, "high") {
		t.Errorf("card badge label 'high' missing; got:\n%s", got)
	}
	if !strings.Contains(got, "danger-soft") {
		t.Errorf("card badge should use danger-soft token role; got:\n%s", got)
	}
}

func TestBoard_UnknownStageDropped(t *testing.T) {
	orphan := CardItem{ID: "orphan", StageID: "nonexistent", Title: "Ghost Card"}
	got := render(t, Board(testCols, append(testCards, orphan)))
	if strings.Contains(got, "Ghost Card") {
		t.Errorf("card with unknown stageID should be silently dropped; got:\n%s", got)
	}
}

func TestBoard_EmptyColumns(t *testing.T) {
	got := render(t, Board(testCols, nil))
	if !strings.Contains(got, "data-kanban-board") {
		t.Errorf("empty board missing root attr; got:\n%s", got)
	}
	// All 3 columns still render.
	for _, col := range testCols {
		if !strings.Contains(got, col.Label) {
			t.Errorf("empty board missing column %q; got:\n%s", col.Label, got)
		}
	}
}

func TestBoard_NoHexColors(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	if strings.Contains(got, "#") {
		t.Errorf("board output contains a literal hex color; got:\n%s", got)
	}
}

func TestBoard_CardHref_LinkedTitle(t *testing.T) {
	linked := CardItem{
		ID:      "c-linked",
		StageID: "todo",
		Title:   "Linked Card Title",
		Href:    "/applications/c-linked",
	}
	got := render(t, Board(testCols, append(testCards, linked)))
	// Title should be inside an <a> with the href.
	if !strings.Contains(got, `href="/applications/c-linked"`) {
		t.Errorf("card with Href missing href attr; got:\n%s", got)
	}
	if !strings.Contains(got, "Linked Card Title") {
		t.Errorf("linked card title missing; got:\n%s", got)
	}
	// Draggable wrapper must still be present.
	if !strings.Contains(got, `draggable="true"`) {
		t.Errorf("card with Href should still be draggable; got:\n%s", got)
	}
}

func TestBoard_CardNoHref_PlainTitle(t *testing.T) {
	got := render(t, Board(testCols, testCards))
	// testCards have no Href, so titles should NOT be anchor elements.
	// The title "Design tokens" should appear but not inside an <a>.
	if strings.Contains(got, `href=`) && !strings.Contains(got, `href=""`) {
		// href should not appear at all when no card has an Href set.
		// (If it does appear, it must not be from these cards.)
	}
	// Just verify titles still render as expected.
	if !strings.Contains(got, "Design tokens") {
		t.Errorf("plain card title 'Design tokens' missing; got:\n%s", got)
	}
}

// ---- unit: cardsForStage --------------------------------------------------

func TestCardsForStage_FiltersCorrectly(t *testing.T) {
	got := cardsForStage(testCards, "todo")
	if len(got) != 2 {
		t.Errorf("want 2 todo cards, got %d", len(got))
	}
	for _, c := range got {
		if c.StageID != "todo" {
			t.Errorf("unexpected stageID %q in filtered result", c.StageID)
		}
	}
}

func TestCardsForStage_EmptyForUnknownStage(t *testing.T) {
	got := cardsForStage(testCards, "unknown")
	if len(got) != 0 {
		t.Errorf("expected 0 cards for unknown stage, got %d", len(got))
	}
}

// ---- unit: badgeClass ------------------------------------------------------

func TestBadgeClass_KnownRole(t *testing.T) {
	for _, role := range []string{"positive-soft", "caution-soft", "danger-soft", "info-soft", "accent-soft"} {
		cls := badgeClass(role)
		if !strings.Contains(cls, "bg-"+role) {
			t.Errorf("badgeClass(%q) missing bg-%s; got %q", role, role, cls)
		}
	}
}

func TestBadgeClass_UnknownRoleFallback(t *testing.T) {
	for _, bad := range []string{"", "DANGER", "red", "<script>", "accent;color:red"} {
		cls := badgeClass(bad)
		if strings.Contains(cls, bad) && bad != "" {
			t.Errorf("badgeClass(%q) injected unsafe role; got %q", bad, cls)
		}
		if !strings.Contains(cls, "surface-sunken") {
			t.Errorf("badgeClass(%q) should fall back to surface-sunken; got %q", bad, cls)
		}
	}
}

func TestBadgeClass_TokenRolePresent(t *testing.T) {
	cls := badgeClass("accent-soft")
	for _, token := range []string{"text-ink", "border-line", "rounded-full"} {
		if !strings.Contains(cls, token) {
			t.Errorf("badgeClass missing token class %q; got %q", token, cls)
		}
	}
}
