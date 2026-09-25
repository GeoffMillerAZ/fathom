package terminal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/apps/terminal"
)

func visibleCatalogRows(t *testing.T, m *terminal.Model) int {
	t.Helper()
	rows := 0
	for _, line := range strings.Split(terminalText(m), "\n") {
		if strings.HasSuffix(line, "[Open]") {
			rows++
		}
	}
	if rows < 2 {
		t.Fatalf("rendered catalog has only %d visible rows", rows)
	}
	return rows
}

func detailFrame(t *testing.T, m *terminal.Model) (start, end, total int, rows []string) {
	t.Helper()
	lines := strings.Split(terminalText(m), "\n")
	header, footer := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(line, "Detail lines ") {
			header = i
			if _, err := fmt.Sscanf(line, "Detail lines %d-%d of %d", &start, &end, &total); err != nil {
				t.Fatalf("parse detail window %q: %v", line, err)
			}
		}
		if strings.HasPrefix(line, "Esc or [Back]") {
			footer = i
		}
	}
	if header < 0 || footer <= header {
		t.Fatalf("detail frame boundaries missing: header=%d footer=%d", header, footer)
	}
	return start, end, total, lines[header+1 : footer]
}

func completeDetailRows(t *testing.T, c *catalog.Catalog) []string {
	t.Helper()
	m := terminalModel(t, c, terminal.Options{})
	terminalStep(t, m, tea.WindowSizeMsg{Width: 120, Height: 120})
	terminalKey(t, m, tea.KeyEnter)
	start, end, total, rows := detailFrame(t, m)
	if start != 1 || end != total || total > len(rows) {
		t.Fatalf("tall reference did not expose complete detail: window=%d-%d/%d capacity=%d", start, end, total, len(rows))
	}
	return append([]string(nil), rows[:total]...)
}

func requireDetailSlice(t *testing.T, m *terminal.Model, complete []string, offset, rows int) {
	t.Helper()
	want := make([]string, rows)
	copy(want, complete[offset:min(len(complete), offset+rows)])
	start, end, total, got := detailFrame(t, m)
	if len(got) != rows || !slices.Equal(got, want) {
		t.Fatalf("detail rows at offset %d = %q, want %q", offset, got, want)
	}
	if start != offset+1 || end != min(len(complete), offset+rows) || total != len(complete) {
		t.Fatalf("detail label=%d-%d/%d, want %d-%d/%d", start, end, total, offset+1, min(len(complete), offset+rows), len(complete))
	}
}

func catalogPage(t *testing.T, c *catalog.Catalog) catalog.Page {
	t.Helper()
	page, err := c.Search(catalog.Query{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestTerminalListPageKeysUseRenderedViewportAndClamp(t *testing.T) {
	c := terminalCatalog(t)
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	page := catalogPage(t, c)
	m := terminalModel(t, c, terminal.Options{})
	rows := visibleCatalogRows(t, m)
	if len(page.Items) <= rows {
		t.Fatalf("catalog has %d items; need more than rendered page %d", len(page.Items), rows)
	}

	terminalKey(t, m, tea.KeyPgUp)
	if got := terminalSelected(t, m); got != page.Items[0].ID {
		t.Fatalf("PageUp at top selected %q", got)
	}
	terminalKey(t, m, tea.KeyPgDown)
	if got := terminalSelected(t, m); got != page.Items[rows].ID {
		t.Fatalf("PageDown selected %q, want one rendered page to %q", got, page.Items[rows].ID)
	}
	terminalKey(t, m, tea.KeyEnd)
	last := len(page.Items) - 1
	terminalKey(t, m, tea.KeyPgDown)
	if got := terminalSelected(t, m); got != page.Items[last].ID {
		t.Fatalf("PageDown at bottom selected %q", got)
	}
	terminalKey(t, m, tea.KeyPgUp)
	if got := terminalSelected(t, m); got != page.Items[last-rows].ID {
		t.Fatalf("PageUp selected %q, want one rendered page to %q", got, page.Items[last-rows].ID)
	}

	resized := terminalModel(t, c, terminal.Options{})
	terminalStep(t, resized, tea.WindowSizeMsg{Width: 120, Height: 18})
	resizedRows := visibleCatalogRows(t, resized)
	if resizedRows >= len(page.Items) {
		t.Fatalf("catalog has %d items; resized page has %d rows", len(page.Items), resizedRows)
	}
	terminalKey(t, resized, tea.KeyPgDown)
	if got := terminalSelected(t, resized); got != page.Items[resizedRows].ID {
		t.Fatalf("resized PageDown selected %q, want %q for %d visible rows", got, page.Items[resizedRows].ID, resizedRows)
	}
	after, err := json.Marshal(c)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("paging mutated the shared catalog")
	}
}

func TestTerminalDetailPageKeysUseRenderedViewportAndPreserveSelection(t *testing.T) {
	c := terminalCatalog(t)
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	complete := completeDetailRows(t, c)
	m := terminalModel(t, c, terminal.Options{})
	rows := visibleCatalogRows(t, m)
	if len(complete) <= 2*rows {
		t.Fatalf("detail fixture has %d rows; need more than two %d-row pages", len(complete), rows)
	}
	selected := terminalSelected(t, m)
	terminalKey(t, m, tea.KeyEnter)
	requireDetailSlice(t, m, complete, 0, rows)
	terminalKey(t, m, tea.KeyPgUp)
	requireDetailSlice(t, m, complete, 0, rows)
	terminalKey(t, m, tea.KeyPgDown)
	requireDetailSlice(t, m, complete, rows, rows)
	terminalKey(t, m, tea.KeyPgUp)
	requireDetailSlice(t, m, complete, 0, rows)
	terminalKey(t, m, tea.KeyEnd)
	bottom := len(complete) - rows
	requireDetailSlice(t, m, complete, bottom, rows)
	terminalKey(t, m, tea.KeyPgDown)
	requireDetailSlice(t, m, complete, bottom, rows)
	terminalKey(t, m, tea.KeyPgUp)
	requireDetailSlice(t, m, complete, bottom-rows, rows)
	terminalKey(t, m, tea.KeyEscape)
	if got := terminalSelected(t, m); got != selected {
		t.Fatalf("detail paging/back changed selection %q to %q", selected, got)
	}
	after, err := json.Marshal(c)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("detail paging mutated the shared catalog")
	}
}
