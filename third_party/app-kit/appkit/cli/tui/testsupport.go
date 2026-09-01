package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
)

// NewTest wraps teatest.NewTestModel with the kit's deterministic
// defaults — an 80x24 terminal and the Ascii color profile — so a
// consumer's model test (and any golden it records) never depends on the
// runner's actual terminal size or color capability. Extra options apply
// after the defaults, so a caller can override either.
func NewTest(tb testing.TB, model tea.Model, opts ...teatest.TestOption) *teatest.TestModel {
	tb.Helper()
	base := []teatest.TestOption{
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	}
	return teatest.NewTestModel(tb, model, append(base, opts...)...)
}
