package terminal_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/apps/terminal"
)

// Explicit list keeps the oracle independent of the implementation's property table.
// These are direction controls, not ordinary Arabic/Hebrew letters or join controls.
var bidiControls = []rune{'\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069'}

func TestTerminalBidiControlsAreRejectedOrDisclosed(t *testing.T) {
	c := terminalCatalog(t)
	for _, control := range bidiControls {
		t.Run(fmt.Sprintf("U+%04X", control), func(t *testing.T) {
			input := "forms" + string(control) + " validation"
			t.Run("initial filter", func(t *testing.T) {
				if _, err := terminal.New(c, terminal.Options{Query: input}); err == nil {
					t.Fatalf("direction-control query %q accepted at startup", input)
				}
			})
			for _, method := range []string{"paste", "key text"} {
				t.Run(method, func(t *testing.T) {
					m := terminalModel(t, c, terminal.Options{})
					terminalKey(t, m, '/')
					if method == "paste" {
						terminalStep(t, m, tea.PasteMsg{Content: input})
					} else {
						terminalStep(t, m, tea.KeyPressMsg{Text: input})
					}
					if got := terminalFilter(t, m); got != "forms validation" {
						t.Errorf("filter = %q; want direction controls removed", got)
					}
					if !strings.Contains(terminalText(m), "Input sanitized") {
						t.Error("modified input has no visible sanitization notice")
					}
					terminalKey(t, m, tea.KeyEnter)
					if got := terminalSelected(t, m); got != "interaction.forms" {
						t.Errorf("sanitized search selected %q, want interaction.forms", got)
					}
				})
			}
		})
	}
}

func TestTerminalBidiControlsNeverReachMetadataFrames(t *testing.T) {
	entry := terminalCatalog(t).Entries[0]
	marker := "before" + string(bidiControls) + "after"
	entry.Name, entry.Summary, entry.Usage = marker, marker, marker
	m := terminalModel(t, terminalSyntheticCatalog(t, []catalog.Entry{entry}), terminal.Options{})
	terminalStep(t, m, tea.WindowSizeMsg{Width: 300, Height: 120})
	for _, mode := range []string{"list", "detail"} {
		if mode == "detail" {
			terminalKey(t, m, tea.KeyEnter)
		}
		content := m.View().Content // Inspect emitted bytes, not an ANSI-stripped projection.
		for _, control := range bidiControls {
			if strings.ContainsRune(content, control) {
				t.Errorf("%s frame contains direction control U+%04X", mode, control)
			}
		}
		if !strings.Contains(content, "beforeafter") {
			t.Errorf("%s frame discarded ordinary neighboring text", mode)
		}
	}
}

func TestTerminalOrdinaryMultilingualTextIsPreserved(t *testing.T) {
	for _, input := range []string{"مرحبا بالعالم", "שלום עולם", "e\u0301 👩‍👩‍👧‍👦 🇺🇸", "می\u200cروم"} {
		t.Run(input, func(t *testing.T) {
			c := terminalCatalog(t)
			m := terminalModel(t, c, terminal.Options{Query: input})
			if got := terminalFilter(t, m); got != input {
				t.Fatalf("initial text = %q, want %q", got, input)
			}
			m = terminalModel(t, c, terminal.Options{})
			terminalKey(t, m, '/')
			terminalStep(t, m, tea.PasteMsg{Content: input})
			if got := terminalFilter(t, m); got != input || strings.Contains(terminalText(m), "Input sanitized") {
				t.Fatalf("ordinary multilingual paste modified: %q", got)
			}
		})
	}
}
