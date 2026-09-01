// Package tui is the cli-kit's bubbletea/v2 bootstrap: house defaults for
// running a tea.Model (context-bound lifetime, alt-screen and mouse OFF
// unless asked for, a color-profile passthrough) plus a teatest/v2 wrapper
// pinned to a deterministic terminal size and profile for goldens.
package tui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// settings collects Run's house defaults before NewProgram is built.
type settings struct {
	mouse        tea.MouseMode
	altScreen    bool
	colorProfile *colorprofile.Profile
	input        io.Reader
	inputSet     bool
	output       io.Writer
}

// Option configures Run.
type Option func(*settings)

// WithMouse turns mouse reporting on (cell-motion). Mouse is OFF by
// default — a cli-kit program that wants raw terminal scrollback/selection
// to keep working (the common case for a small status view) gets that for
// free; a program that wants mouse events opts in explicitly.
func WithMouse() Option {
	return func(s *settings) { s.mouse = tea.MouseModeCellMotion }
}

// WithAltScreen puts the program in the alternate screen buffer.
func WithAltScreen() Option {
	return func(s *settings) { s.altScreen = true }
}

// WithColorProfile pins the program's color profile (e.g.
// colorprofile.Ascii for a deterministic test or golden run) instead of
// bubbletea's own runtime detection.
func WithColorProfile(p colorprofile.Profile) Option {
	return func(s *settings) { s.colorProfile = &p }
}

// WithInput is a thin passthrough to tea.WithInput — most programs never
// need it (input defaults to stdin, falling back to opening /dev/tty when
// stdin isn't a terminal), but a test or a headless environment with no
// controlling terminal at all needs a way to disable input reading
// entirely: WithInput(nil), matching tea.WithInput's own nil convention.
func WithInput(r io.Reader) Option {
	return func(s *settings) { s.input = r; s.inputSet = true }
}

// WithOutput is a thin passthrough to tea.WithOutput (default: stdout).
func WithOutput(w io.Writer) Option {
	return func(s *settings) { s.output = w }
}

// Run runs model to completion with the kit's house defaults:
// tea.WithContext(ctx) ties the program's lifetime to ctx (canceling ctx
// quits the program — TestRunHonorsContextCancel asserts this), and
// nothing else is forced unless an Option asks for it.
//
// bubbletea v2 moved alt-screen and mouse mode from Program options onto
// the per-frame tea.View a Model's View() returns (there is no
// tea.WithAltScreen/tea.WithMouseCellMotion in v2 the way there was in
// v1) — so WithAltScreen/WithMouse work by wrapping model in a decorator
// that forces those two View fields on every frame without touching the
// model's own Init/Update logic. This is Run's one deviation from a pure
// passthrough; everything else is exactly the tea.ProgramOption it names.
func Run(ctx context.Context, model tea.Model, opts ...Option) (tea.Model, error) {
	cfg := &settings{}
	for _, o := range opts {
		o(cfg)
	}

	m := model
	if cfg.altScreen || cfg.mouse != tea.MouseModeNone {
		m = &viewDecorator{Model: model, altScreen: cfg.altScreen, mouse: cfg.mouse}
	}

	progOpts := []tea.ProgramOption{tea.WithContext(ctx)}
	if cfg.colorProfile != nil {
		progOpts = append(progOpts, tea.WithColorProfile(*cfg.colorProfile))
	}
	if cfg.inputSet {
		progOpts = append(progOpts, tea.WithInput(cfg.input))
	}
	if cfg.output != nil {
		progOpts = append(progOpts, tea.WithOutput(cfg.output))
	}

	result, err := tea.NewProgram(m, progOpts...).Run()
	if dec, ok := result.(*viewDecorator); ok {
		return dec.Model, err
	}
	return result, err
}

// viewDecorator forces AltScreen/MouseMode on every rendered View without
// altering the wrapped model's Init/Update/View content.
type viewDecorator struct {
	tea.Model
	altScreen bool
	mouse     tea.MouseMode
}

func (d *viewDecorator) Init() tea.Cmd { return d.Model.Init() }

func (d *viewDecorator) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := d.Model.Update(msg)
	d.Model = inner
	return d, cmd
}

func (d *viewDecorator) View() tea.View {
	v := d.Model.View()
	if d.altScreen {
		v.AltScreen = true
	}
	if d.mouse != tea.MouseModeNone {
		v.MouseMode = d.mouse
	}
	return v
}
