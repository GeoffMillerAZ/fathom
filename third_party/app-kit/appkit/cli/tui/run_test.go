package tui_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/geoffmilleraz/app-kit/appkit/cli/tui"
)

// headlessOpts disable real-terminal input (this sandbox and most CI have
// no controlling /dev/tty) and pin a deterministic color profile — the
// same story teatest gives testsupport.go's NewTest, applied by hand here
// since these tests exercise Run directly.
func headlessOpts() []tui.Option {
	return []tui.Option{
		tui.WithColorProfile(colorprofile.Ascii),
		tui.WithInput(nil),
		tui.WithOutput(io.Discard),
	}
}

// quitOnStartModel quits immediately from Init — the trivial model
// TestRunSmokeModel needs to prove Run actually drives a program to
// completion and returns.
type quitOnStartModel struct{ initialized bool }

func (m quitOnStartModel) Init() tea.Cmd { return tea.Quit }
func (m quitOnStartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return quitOnStartModel{initialized: true}, nil
}
func (m quitOnStartModel) View() tea.View { return tea.NewView("") }

// TestRunSmokeModel proves Run itself drives a tea.Model to completion and
// returns its final state — a model that quits from Init, run through
// Run directly (a deterministic profile pins output; a bounded context is
// the test's own safety net against a hang, not something the model
// relies on to quit).
func TestRunSmokeModel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	final, err := tui.Run(ctx, quitOnStartModel{}, headlessOpts()...)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := final.(quitOnStartModel); !ok {
		t.Fatalf("final model type = %T, want quitOnStartModel", final)
	}
}

// foreverModel never quits on its own — TestRunHonorsContextCancel needs
// a model whose only way to stop is the context Run was given.
type foreverModel struct{}

func (foreverModel) Init() tea.Cmd                       { return nil }
func (foreverModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return foreverModel{}, nil }
func (foreverModel) View() tea.View                      { return tea.NewView("") }

// TestRunHonorsContextCancel proves canceling the context passed to Run
// stops the program even though the model itself never asks to quit.
func TestRunHonorsContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := tui.Run(ctx, foreverModel{}, headlessOpts()...)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, tea.ErrProgramKilled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Run returned err = %v, want it to wrap tea.ErrProgramKilled / context.DeadlineExceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of its context expiring — it is not honoring ctx cancellation")
	}
}
