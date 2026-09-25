package terminal

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/geoffmilleraz/app-kit/appkit/cli/theme"
)

type hit struct {
	x0, x1, y int
	name, id  string
}

func (m *Model) View() tea.View {
	var lines []string
	var hits []hit
	style := theme.Styles(m.isDark)
	// Inherit the terminal's chosen background. Until polarity is known, use
	// weight and text markers only, not a guessed light/dark foreground.
	if !m.polarityKnown {
		plain := lipgloss.NewStyle()
		style.Title, style.Accent, style.Subtle = plain.Bold(true), plain.Bold(true), plain
	}
	usable := m.width >= 40 && m.height >= 10
	if !usable {
		preference := "off"
		if m.mouse {
			preference = "on"
		}
		lines = []string{"app-kit terminal catalog", "Needs 40 columns and 10 rows.", "Mouse released here. F2 preference: " + preference, "Resize to continue; Ctrl+C quits."}
	} else {
		lines = append(lines, style.Title.Render("app-kit / terminal catalog"), "Experimental. Read-only. No commands or sources are executed.")
		buttons := []struct{ label, name string }{{"[/ Find]", "filter"}}
		if m.detail != nil {
			buttons[0] = struct{ label, name string }{"[Esc Back]", "back"}
		}
		mouse := "off"
		if m.mouse {
			mouse = "on"
		}
		buttons = append(buttons, struct{ label, name string }{"[F2 Mouse " + mouse + "]", "mouse"}, struct{ label, name string }{"[Clear]", "clear"}, struct{ label, name string }{"[Quit]", "quit"})
		var controls strings.Builder
		for _, button := range buttons {
			start := controls.Len()
			if start > 0 {
				controls.WriteByte(' ')
				start++
			}
			if start+len(button.label) > m.width {
				continue // Never create a hit region for a clipped control.
			}
			controls.WriteString(button.label)
			hits = append(hits, hit{start, start + len(button.label), 2, button.name, ""})
		}
		lines = append(lines, controls.String())
		if m.detail == nil {
			query := clean(m.query)
			prompt := "Filter: " + ansi.Truncate(query, m.width-8, "...")
			if m.editing {
				// Editing appends at the end. Keep that insertion point visible,
				// even when the bounded UTF-8 query is wider than this terminal.
				available := m.width - 11 // prefix and insertion marker
				if width := ansi.StringWidth(query); width > available {
					// TruncateLeft preserves a wide grapheme intersecting the cut.
					// Advance the cut if necessary instead of clipping the caret.
					for cut := width - available + 3; cut <= width; cut++ {
						tail := ansi.TruncateLeft(query, cut, "...")
						if ansi.StringWidth(tail) <= available {
							query = tail
							break
						}
					}
				}
				prompt = style.Accent.Render("> Filter: " + query + "_")
			}
			lines = append(lines, prompt)
			count := fmt.Sprintf("%d matches | arrows select; Enter or [Open] inspects", m.page.Total)
			if len(m.page.Items) < m.page.Total {
				count = fmt.Sprintf("First %d of %d matches; narrow the filter", len(m.page.Items), m.page.Total)
			}
			lines = append(lines, count)
			for i := 0; i < m.bodyRows(); i++ {
				index := m.offset + i
				if index >= len(m.page.Items) {
					if i == 0 {
						lines = append(lines, "No matches. [Clear] removes the text filter.")
					} else {
						lines = append(lines, "")
					}
					continue
				}
				item := m.page.Items[index]
				prefix := "  "
				if index == m.selected {
					prefix = "> "
				}
				left := ansi.Truncate(prefix+clean(item.Name)+" | "+clean(item.Maturity), m.width-7, "...")
				line := left + strings.Repeat(" ", max(0, m.width-6-ansi.StringWidth(left))) + "[Open]"
				if index == m.selected {
					line = style.Accent.Render(line)
				}
				lines = append(lines, line)
				hits = append(hits, hit{0, m.width - 6, 5 + i, "select", item.ID}, hit{m.width - 6, m.width, 5 + i, "open", item.ID})
			}
			lines = append(lines, "Selected: "+m.selectedID())
		} else {
			lines = append(lines, style.Accent.Render(clean(m.detail.ID)+" | "+clean(m.detail.Maturity)))
			detail := m.detailLines()
			lines = append(lines, fmt.Sprintf("Detail lines %d-%d of %d | arrows, PgUp/PgDn or wheel", m.detailOffset+1, min(len(detail), m.detailOffset+m.bodyRows()), len(detail)))
			for i := 0; i < m.bodyRows(); i++ {
				index := m.detailOffset + i
				line := ""
				if index < len(detail) {
					line = detail[index]
				}
				lines = append(lines, line)
			}
			lines = append(lines, "Esc or [Back] preserves the filter and selection.")
		}
		status := "F2 toggles mouse capture; off leaves selection/copy to the terminal."
		if m.notice != "" {
			status = m.notice
		}
		lines = append(lines, status, ansi.Truncate("Revision: "+m.page.Revision, m.width, "..."))
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], m.width, "")
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	if m.mouse && usable {
		v.MouseMode = tea.MouseModeCellMotion
	}
	// The handler is bound to this frame. A stale click cannot target a row
	// that moved after filtering, scrolling, toggling capture or resizing.
	epoch, width, rows, enabled := m.epoch, m.width, m.bodyRows(), m.mouse && usable
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		if !enabled {
			return nil
		}
		pointer := msg.Mouse()
		if pointer.X < 0 || pointer.X >= width || pointer.Y < 0 {
			return nil
		}
		var action mouseAction
		switch msg.(type) {
		case tea.MouseClickMsg:
			if pointer.Button != tea.MouseLeft {
				return nil
			}
			for _, h := range hits {
				if pointer.Y == h.y && pointer.X >= h.x0 && pointer.X < h.x1 {
					action = mouseAction{name: h.name, id: h.id, epoch: epoch}
					break
				}
			}
		case tea.MouseWheelMsg:
			if pointer.Y < 5 || pointer.Y >= 5+rows || len(hits) == 0 {
				return nil
			}
			delta := 0
			if pointer.Button == tea.MouseWheelUp {
				delta = -3
			} else if pointer.Button == tea.MouseWheelDown {
				delta = 3
			}
			if delta != 0 {
				action = mouseAction{name: "move", delta: delta, epoch: epoch}
			}
		}
		if action.name == "" {
			return nil
		}
		return func() tea.Msg { return action }
	}
	return v
}

func (m *Model) detailLines() []string {
	if m.detail == nil {
		return nil
	}
	e := m.detail
	var lines []string
	add := func(label, value string) {
		if value == "" {
			value = "None declared."
		}
		text := label + ": " + clean(value)
		lines = append(lines, strings.Split(ansi.Wrap(text, max(1, m.width), ""), "\n")...)
		lines = append(lines, "")
	}
	add("Name", e.Name)
	add("Purpose", e.Summary)
	add("Profiles", strings.Join(e.Profiles, ", "))
	add("Layers", strings.Join(e.Layers, ", "))
	add("Dependencies", strings.Join(e.Dependencies, "; "))
	add("Usage (inert text)", e.Usage)
	add("App owns", strings.Join(e.AppOwns, "; "))
	add("States", strings.Join(e.States, ", "))
	add("Evidence gaps", strings.Join(e.Gaps, " "))
	add("Proof entrypoints (not executed)", strings.Join(e.Proof, "; "))
	add("Source", e.Source.Path+" ["+strings.Join(e.Source.Symbols, ", ")+"]")
	add("Provenance", e.Provenance.Source+"; "+e.Provenance.License+"; "+e.Provenance.Audience)
	add("Catalog revision", m.page.Revision)
	return lines
}
