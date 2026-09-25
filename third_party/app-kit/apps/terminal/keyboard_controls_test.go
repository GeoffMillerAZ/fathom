package terminal_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/geoffmilleraz/app-kit/apps/terminal"
)

func TestTerminalSmallViewportDisclosesMouseRelease(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 39, Height: 9}, {Width: 1, Height: 1}} {
		m := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
		terminalStep(t, m, size)
		view := m.View()
		if view.MouseMode != tea.MouseModeNone {
			t.Fatalf("small viewport %dx%d retained mouse capture: %v", size.Width, size.Height, view.MouseMode)
		}
		if view.OnMouse != nil {
			cmd := view.OnMouse(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
			if cmd != nil {
				t.Fatalf("small viewport %dx%d retained an active mouse callback", size.Width, size.Height)
			}
		}
	}

	restored := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
	terminalStep(t, restored, tea.WindowSizeMsg{Width: 39, Height: 9})
	content := strings.ToLower(terminalText(restored))
	for _, disclosure := range []string{"mouse", "released", "f2"} {
		if !strings.Contains(content, disclosure) {
			t.Fatalf("small-viewport fallback does not disclose mouse release and F2 recovery: %q", content)
		}
	}
	terminalStep(t, restored, tea.WindowSizeMsg{Width: 120, Height: 14})
	if got := restored.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("usable resize did not restore mouse preference: %v", got)
	}

	disabled := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
	terminalStep(t, disabled, tea.WindowSizeMsg{Width: 39, Height: 9})
	terminalKey(t, disabled, tea.KeyF2)
	terminalStep(t, disabled, tea.WindowSizeMsg{Width: 120, Height: 14})
	if got := disabled.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("F2 preference change made while small was lost after resize: %v", got)
	}
}

func TestTerminalModifiedQDoesNotQuit(t *testing.T) {
	for _, tc := range []struct {
		name string
		mod  tea.KeyMod
	}{
		{"shift", tea.ModShift},
		{"alt", tea.ModAlt},
		{"ctrl", tea.ModCtrl},
		{"meta", tea.ModMeta},
		{"hyper", tea.ModHyper},
		{"super", tea.ModSuper},
		{"alt with lock", tea.ModAlt | tea.ModNumLock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := terminalModel(t, terminalCatalog(t), terminal.Options{})
			if cmd := terminalStep(t, m, tea.KeyPressMsg{Code: 'q', Mod: tc.mod}); cmd != nil {
				if _, ok := cmd().(tea.QuitMsg); ok {
					t.Fatalf("modified q unexpectedly requested quit: mod=%v", tc.mod)
				}
				t.Fatalf("modified q unexpectedly scheduled a command: mod=%v", tc.mod)
			}
		})
	}

	for _, mod := range []tea.KeyMod{0, tea.ModCapsLock, tea.ModNumLock, tea.ModScrollLock, tea.ModCapsLock | tea.ModNumLock} {
		m := terminalModel(t, terminalCatalog(t), terminal.Options{})
		cmd := terminalStep(t, m, tea.KeyPressMsg{Code: 'q', Mod: mod})
		if cmd == nil {
			t.Fatalf("plain q with lock state %v did not request quit", mod)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("plain q did not return Bubble Tea quit message")
		}
	}
}
