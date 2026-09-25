package terminal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/appkit/cli/tui"
	"github.com/geoffmilleraz/app-kit/apps/terminal"
)

func terminalCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) < 8 {
		t.Fatal("real catalog fixture missing expected breadth")
	}
	return c
}

func terminalModel(t *testing.T, c *catalog.Catalog, opts terminal.Options) *terminal.Model {
	t.Helper()
	m, err := terminal.New(c, opts)
	if err != nil {
		t.Fatal(err)
	}
	terminalStep(t, m, tea.WindowSizeMsg{Width: 120, Height: 14})
	return m
}

func terminalStep(t *testing.T, m *terminal.Model, msg tea.Msg) tea.Cmd {
	t.Helper()
	next, cmd := m.Update(msg)
	if next != m {
		t.Fatalf("unexpected replacement model %T", next)
	}
	return cmd
}

func terminalKey(t *testing.T, m *terminal.Model, code rune) tea.Cmd {
	t.Helper()
	return terminalStep(t, m, tea.KeyPressMsg{Code: code})
}

func terminalText(m *terminal.Model) string { return ansi.Strip(m.View().Content) }

func TestTerminalRevisionIsCompleteOrExplicitlyClipped(t *testing.T) {
	c := terminalCatalog(t)
	m := terminalModel(t, c, terminal.Options{})
	for _, width := range []int{80, 60, 40} {
		terminalStep(t, m, tea.WindowSizeMsg{Width: width, Height: 12})
		lines := strings.Split(terminalText(m), "\n")
		got := lines[len(lines)-1]
		want := "Revision: " + c.Revision
		if width >= len(want) {
			if got != want {
				t.Fatalf("revision at %d columns = %q, want full digest", width, got)
			}
		} else if !strings.HasPrefix(got, "Revision: ") || !strings.HasSuffix(got, "...") || ansi.StringWidth(got) > width {
			t.Fatalf("revision at %d columns clipped without explicit marker: %q", width, got)
		}
	}
}

func TestTerminalLongFilterKeepsInsertionPointVisible(t *testing.T) {
	c := terminalCatalog(t)
	for width := 40; width <= 80; width++ {
		m := terminalModel(t, c, terminal.Options{Query: strings.Repeat("界", 60) + "tail"})
		terminalStep(t, m, tea.WindowSizeMsg{Width: width, Height: 12})
		terminalKey(t, m, '/')
		line := strings.Split(terminalText(m), "\n")[3]
		if !strings.HasPrefix(line, "> Filter: ...") || !strings.HasSuffix(line, "tail_") || ansi.StringWidth(line) > width {
			t.Fatalf("filter insertion point hidden at %d columns: %q", width, line)
		}
	}
}

func TestTerminalPasteSanitizationIsDisclosed(t *testing.T) {
	for _, input := range []string{"hello\nworld", "hello\tworld", string([]byte{0xff, 'a'})} {
		m := terminalModel(t, terminalCatalog(t), terminal.Options{})
		terminalKey(t, m, '/')
		terminalStep(t, m, tea.PasteMsg{Content: input})
		if !strings.Contains(terminalText(m), "Input sanitized") {
			t.Fatalf("changed paste %q lacked a visible notice", input)
		}
	}
}

func terminalSelected(t *testing.T, m *terminal.Model) string {
	t.Helper()
	for _, line := range strings.Split(terminalText(m), "\n") {
		if strings.HasPrefix(line, "Selected: ") {
			return strings.TrimPrefix(line, "Selected: ")
		}
	}
	t.Fatal("list has no selected identity")
	return ""
}

func terminalFilter(t *testing.T, m *terminal.Model) string {
	t.Helper()
	for _, line := range strings.Split(terminalText(m), "\n") {
		if strings.HasPrefix(line, "> Filter: ") {
			return strings.TrimSuffix(strings.TrimPrefix(line, "> Filter: "), "_")
		}
		if strings.HasPrefix(line, "Filter: ") {
			return strings.TrimPrefix(line, "Filter: ")
		}
	}
	t.Fatal("filter prompt missing")
	return ""
}

func terminalClick(t *testing.T, m *terminal.Model, view tea.View, x, y int) tea.Cmd {
	t.Helper()
	if view.OnMouse == nil {
		t.Fatal("rendered frame has no mouse handler")
	}
	cmd := view.OnMouse(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if cmd == nil {
		return nil
	}
	return terminalStep(t, m, cmd())
}

func terminalControl(t *testing.T, m *terminal.Model, label string) tea.Cmd {
	t.Helper()
	view := m.View()
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	x := strings.Index(lines[2], label)
	if x < 0 {
		t.Fatalf("control %q missing", label)
	}
	return terminalClick(t, m, view, x+1, 2)
}

func TestTerminalCatalogKeyboardJourneyAndIdentity(t *testing.T) {
	c := terminalCatalog(t)
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	m := terminalModel(t, c, terminal.Options{})
	page, err := c.Search(catalog.Query{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := terminalSelected(t, m); got != page.Items[0].ID {
		t.Fatalf("initial selected = %q", got)
	}
	if !strings.Contains(terminalText(m), c.Revision) || !strings.Contains(terminalText(m), fmt.Sprintf("%d matches", page.Total)) {
		t.Fatal("catalog revision/count lost")
	}
	terminalKey(t, m, tea.KeyDown)
	want := page.Items[1].ID
	if got := terminalSelected(t, m); got != want {
		t.Fatalf("down selected %q, want %q", got, want)
	}
	if cmd := terminalKey(t, m, tea.KeyEnter); cmd != nil {
		t.Fatal("opening metadata must not schedule execution")
	}
	entry, _ := c.Get(want)
	if !strings.Contains(terminalText(m), want+" | "+entry.Maturity) {
		t.Fatal("detail identity/maturity differs from catalog")
	}
	terminalStep(t, m, tea.WindowSizeMsg{Width: 300, Height: 120})
	for _, value := range []string{entry.Name, entry.Source.Path, entry.Provenance.Source, c.Revision, "Usage (inert text)", "Proof entrypoints (not executed)"} {
		if !strings.Contains(terminalText(m), value) {
			t.Errorf("detail lacks catalog metadata %q", value)
		}
	}
	terminalKey(t, m, tea.KeyEscape)
	if terminalSelected(t, m) != want {
		t.Fatal("back lost selection")
	}
	terminalKey(t, m, '/')
	terminalStep(t, m, tea.PasteMsg{Content: "forms validation"})
	terminalKey(t, m, tea.KeyEnter)
	if got := terminalSelected(t, m); got != "interaction.forms" {
		t.Fatalf("fixed forms task selected %q", got)
	}
	terminalKey(t, m, tea.KeyEnter)
	terminalKey(t, m, tea.KeyEscape)
	if terminalFilter(t, m) != "forms validation" || terminalSelected(t, m) != "interaction.forms" {
		t.Fatal("detail back lost filter or selected capability")
	}
	after, err := json.Marshal(c)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("browser mutated shared catalog metadata")
	}
}

func TestTerminalMouseAndKeyboardUseTheSameActions(t *testing.T) {
	c := terminalCatalog(t)
	keys := terminalModel(t, c, terminal.Options{Mouse: true})
	mouse := terminalModel(t, c, terminal.Options{Mouse: true})
	for range 3 {
		terminalKey(t, keys, tea.KeyDown)
	}
	view := mouse.View()
	cmd := view.OnMouse(tea.MouseWheelMsg{X: 4, Y: 6, Button: tea.MouseWheelDown})
	if cmd == nil {
		t.Fatal("wheel on list produced no action")
	}
	terminalStep(t, mouse, cmd())
	if terminalText(keys) != terminalText(mouse) {
		t.Fatal("wheel differs from three down arrows")
	}
	terminalKey(t, keys, tea.KeyEnter)
	terminalClick(t, mouse, mouse.View(), 115, 8)
	if terminalText(keys) != terminalText(mouse) {
		t.Fatal("[Open] and Enter detail differ")
	}
	for range 3 {
		terminalKey(t, keys, tea.KeyDown)
	}
	cmd = mouse.View().OnMouse(tea.MouseWheelMsg{X: 4, Y: 6, Button: tea.MouseWheelDown})
	if cmd == nil {
		t.Fatal("wheel on detail produced no action")
	}
	terminalStep(t, mouse, cmd())
	if terminalText(keys) != terminalText(mouse) {
		t.Fatal("detail wheel differs from keyboard scrolling")
	}
	terminalKey(t, keys, tea.KeyEscape)
	terminalControl(t, mouse, "[Esc Back]")
	if terminalText(keys) != terminalText(mouse) {
		t.Fatal("mouse back did not preserve the keyboard state")
	}
	terminalKey(t, keys, tea.KeyHome)
	terminalClick(t, mouse, mouse.View(), 1, 5)
	if terminalText(keys) != terminalText(mouse) {
		t.Fatal("row selection must select without opening")
	}
}

func TestTerminalResizeAndScrollPreserveSelectedIdentity(t *testing.T) {
	m := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
	terminalKey(t, m, tea.KeyEnd)
	selected := terminalSelected(t, m)
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 10}, {Width: 140, Height: 50}, {Width: 75, Height: 12}} {
		terminalStep(t, m, size)
		if terminalSelected(t, m) != selected {
			t.Fatal("resize changed selected ID")
		}
	}
	terminalKey(t, m, tea.KeyEnter)
	terminalKey(t, m, tea.KeyEnd)
	terminalStep(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})
	terminalKey(t, m, tea.KeyEscape)
	if terminalSelected(t, m) != selected {
		t.Fatal("detail scroll/resize/back changed selected ID")
	}
}

func TestTerminalMouseCaptureAndStaleFrames(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(*testing.T, *terminal.Model)
	}{
		{"filter", func(t *testing.T, m *terminal.Model) { terminalKey(t, m, '/') }},
		{"filter text", func(t *testing.T, m *terminal.Model) {
			selected := terminalSelected(t, m)
			terminalKey(t, m, '/')
			terminalStep(t, m, tea.PasteMsg{Content: selected})
		}},
		{"resize", func(t *testing.T, m *terminal.Model) { terminalStep(t, m, tea.WindowSizeMsg{Width: 100, Height: 20}) }},
		{"capture roundtrip", func(t *testing.T, m *terminal.Model) { terminalKey(t, m, tea.KeyF2); terminalKey(t, m, tea.KeyF2) }},
		{"scroll", func(t *testing.T, m *terminal.Model) { terminalKey(t, m, tea.KeyEnd) }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			m := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
			old := m.View()
			mutate.fn(t, m)
			before := terminalText(m)
			terminalClick(t, m, old, 115, 5)
			if terminalText(m) != before {
				t.Fatal("stale frame acted on changed model")
			}
		})
	}
	m := terminalModel(t, terminalCatalog(t), terminal.Options{})
	before := terminalText(m)
	terminalClick(t, m, m.View(), 115, 5)
	terminalStep(t, m, tea.MouseClickMsg{X: 115, Y: 5, Button: tea.MouseLeft})
	if terminalText(m) != before || m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("mouse-off did not preserve native terminal ownership")
	}
	terminalKey(t, m, tea.KeyF2)
	before = terminalText(m)
	terminalStep(t, m, tea.MouseWheelMsg{X: 1, Y: 5, Button: tea.MouseWheelDown})
	if terminalText(m) != before {
		t.Fatal("raw mouse was applied a second time outside frame callback")
	}
	for _, msg := range []tea.MouseMsg{
		tea.MouseClickMsg{X: -1, Y: 5, Button: tea.MouseLeft},
		tea.MouseClickMsg{X: 120, Y: 5, Button: tea.MouseLeft},
		tea.MouseClickMsg{X: 1, Y: 5, Button: tea.MouseRight},
		tea.MouseReleaseMsg{X: 1, Y: 5, Button: tea.MouseLeft},
		tea.MouseWheelMsg{X: 1, Y: 0, Button: tea.MouseWheelDown},
	} {
		if cmd := m.View().OnMouse(msg); cmd != nil {
			t.Fatalf("out-of-scope mouse event accepted: %T", msg)
		}
	}
}

func TestTerminalFilterRecoveryGraphemesAndBounds(t *testing.T) {
	m := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
	terminalStep(t, m, tea.WindowSizeMsg{Width: 300, Height: 24})
	terminalKey(t, m, '/')
	terminalStep(t, m, tea.PasteMsg{Content: "unobtainable-nonexistent-capability-zzzzz"})
	if !strings.Contains(terminalText(m), "No matches.") {
		t.Fatal("empty search did not announce no matches")
	}
	terminalKey(t, m, tea.KeyEnter)
	terminalKey(t, m, tea.KeyEnter)
	if !strings.Contains(terminalText(m), "No matches.") {
		t.Fatal("Enter invented a detail for empty results")
	}
	terminalControl(t, m, "[Clear]")
	if terminalFilter(t, m) != "" || strings.Contains(terminalText(m), "No matches.") {
		t.Fatal("clear did not recover results")
	}
	for _, suffix := range []string{"e\u0301", "👩‍👩‍👧‍👦", "🇺🇸"} {
		terminalKey(t, m, '/')
		terminalStep(t, m, tea.PasteMsg{Content: "Go " + suffix})
		terminalKey(t, m, tea.KeyBackspace)
		if got := terminalFilter(t, m); got != "Go " {
			t.Fatalf("grapheme backspace left %q after %q", got, suffix)
		}
		terminalStep(t, m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	}
	terminalStep(t, m, tea.PasteMsg{Content: "A\x1b]52;c;not-a-command\a\x00\u202e\u2066\nB\tC"})
	q := terminalFilter(t, m)
	for _, r := range q {
		if unicode.IsControl(r) || r == '\u202e' || r == '\u2066' {
			t.Fatalf("unsafe control retained: %U", r)
		}
	}
	terminalStep(t, m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	terminalStep(t, m, tea.PasteMsg{Content: strings.Repeat("界", 1<<18)})
	q = terminalFilter(t, m)
	if !utf8.ValidString(q) || len(q) > 256 || q == "" || !strings.Contains(terminalText(m), "256 bytes") {
		t.Fatalf("huge paste not bounded valid UTF-8: bytes=%d", len(q))
	}
	terminalStep(t, m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	terminalStep(t, m, tea.PasteMsg{Content: string([]byte{0xff, 'a'})})
	if !utf8.ValidString(terminalFilter(t, m)) {
		t.Fatal("invalid pasted bytes reached filter")
	}
	terminalKey(t, m, tea.KeyEscape)
	before := terminalText(m)
	terminalStep(t, m, tea.PasteMsg{Content: "ignored outside editing"})
	if terminalText(m) != before {
		t.Fatal("paste outside editing changed selection/filter")
	}
}

func terminalSyntheticCatalog(t *testing.T, entries []catalog.Entry) *catalog.Catalog {
	t.Helper()
	body, err := json.Marshal(struct {
		SchemaVersion int             `json:"schema_version"`
		Entries       []catalog.Entry `json:"entries"`
	}{1, entries})
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.Load(body)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTerminalBoundsLongLabelsAndInertMetadata(t *testing.T) {
	c := terminalCatalog(t)
	entry := c.Entries[0]
	entry.Name = strings.Repeat("界 e\u0301 👩‍👩‍👧‍👦 ", 30)
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	entry.Usage = "touch " + marker + "; $(printf inert)"
	entry.Proof = []string{"go test ./...; touch " + marker}
	entry.Summary = "Synthetic\x1b[2J unsafe control text"
	m := terminalModel(t, terminalSyntheticCatalog(t, []catalog.Entry{entry}), terminal.Options{})
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 10}, {Width: 1, Height: 1}, {Width: 1000, Height: 1000}, {Width: 0, Height: -1}} {
		terminalStep(t, m, size)
		width, height := max(1, min(size.Width, 300)), max(1, min(size.Height, 120))
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) > height {
			t.Fatalf("view has %d lines at height %d", len(lines), height)
		}
		for _, line := range lines {
			if !utf8.ValidString(line) || ansi.StringWidth(line) > width {
				t.Fatalf("view exceeds width %d or breaks UTF-8", width)
			}
		}
	}
	terminalStep(t, m, tea.WindowSizeMsg{Width: 300, Height: 120})
	if cmd := terminalKey(t, m, tea.KeyEnter); cmd != nil {
		t.Fatal("metadata open scheduled a command")
	}
	if !strings.Contains(terminalText(m), "touch "+marker) || strings.Contains(m.View().Content, "\x1b[2J") {
		t.Fatal("inert usage missing or source control escaped sanitization")
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("metadata caused a filesystem side effect: %v", err)
	}
}

func TestTerminalFirstPageLimitAndProfile(t *testing.T) {
	real := terminalCatalog(t)
	entries := make([]catalog.Entry, 60)
	for i := range entries {
		entries[i] = real.Entries[0]
		entries[i].ID = fmt.Sprintf("fixture.%03d", i)
		entries[i].Name = fmt.Sprintf("Synthetic capability %03d", i)
	}
	m := terminalModel(t, terminalSyntheticCatalog(t, entries), terminal.Options{})
	if !strings.Contains(terminalText(m), "First 50 of 60 matches; narrow the filter") {
		t.Fatal("bounded first page is not explicitly announced")
	}
	terminalKey(t, m, tea.KeyEnd)
	if terminalSelected(t, m) != "fixture.049" {
		t.Fatal("selection escaped bounded first page")
	}
	terminalKey(t, m, '/')
	terminalStep(t, m, tea.PasteMsg{Content: "fixture.059"})
	terminalKey(t, m, tea.KeyEnter)
	if terminalSelected(t, m) != "fixture.059" {
		t.Fatal("filter cannot reach later catalog records")
	}
	profile := terminalModel(t, real, terminal.Options{Profile: "cli"})
	want, err := real.Search(catalog.Query{Profile: "cli", Limit: 50})
	if err != nil || len(want.Items) == 0 {
		t.Fatalf("real CLI profile fixture missing: %v", err)
	}
	if terminalSelected(t, profile) != want.Items[0].ID || !strings.Contains(terminalText(profile), fmt.Sprintf("%d matches", want.Total)) {
		t.Fatal("profile results differ from shared catalog")
	}
}

func TestTerminalConstructorAndQuitContracts(t *testing.T) {
	c := terminalCatalog(t)
	for _, opts := range []terminal.Options{{Query: "bad\nquery"}, {Query: string([]byte{0xff})}, {Query: strings.Repeat("x", 257)}, {Profile: "unknown"}} {
		if _, err := terminal.New(c, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
	if _, err := terminal.New(nil, terminal.Options{}); err == nil {
		t.Fatal("nil catalog accepted")
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'q'}, {Code: 'c', Mod: tea.ModCtrl}} {
		m := terminalModel(t, c, terminal.Options{})
		cmd := terminalStep(t, m, key)
		if cmd == nil {
			t.Fatal("quit key produced no command")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("quit key did not request Bubble Tea termination")
		}
	}
}

func TestTerminalHeadlessProgramJourney(t *testing.T) {
	c := terminalCatalog(t)
	m, err := terminal.New(c, terminal.Options{Mouse: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// The upstream option replaces prior options, so preserve the kit's Ascii pin.
	tm := tui.NewTest(t, m, teatest.WithInitialTermSize(120, 24),
		teatest.WithProgramOptions(tea.WithContext(ctx), tea.WithColorProfile(colorprofile.Ascii)))
	// A failed assertion must still stop and join the real loop before canceling
	// its context; teatest treats cancellation as a fatal program error.
	defer func() {
		_ = tm.Quit()
		tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	}()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return strings.Contains(ansi.Strip(string(b)), "terminal catalog") }, teatest.WithDuration(3*time.Second))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 24})
	tm.Send(tea.KeyPressMsg{Code: '/'})
	tm.Type("forms validation")
	// A real width change forces a complete editing-frame redraw; ANSI stripping
	// alone cannot reconstruct the renderer's incremental insertion patches.
	tm.Send(tea.WindowSizeMsg{Width: 121, Height: 24})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(ansi.Strip(string(b)), "> Filter: forms validation_")
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 24})
	// The selected ID can appear while individual typing events are still being
	// processed. Wait for the non-editing final filter frame before clicking its
	// hit regions; otherwise the model correctly refuses a stale callback.
	teatest.WaitFor(t, tm.Output(), terminalFilterFrameReady, teatest.WithDuration(3*time.Second))
	// The real program invokes View.OnMouse; Update deliberately ignores raw
	// mouse events. This catches a broken callback/event-loop integration.
	tm.Send(tea.MouseClickMsg{X: 115, Y: 5, Button: tea.MouseLeft})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return strings.Contains(ansi.Strip(string(b)), "Detail lines") }, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'q'})
	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	got, ok := final.(*terminal.Model)
	if !ok || terminalSelected(t, got) != "interaction.forms" || terminalFilter(t, got) != "forms validation" {
		t.Fatal("real event loop did not preserve the completed journey")
	}
}

func TestTerminalRunnerCancellation(t *testing.T) {
	m := terminalModel(t, terminalCatalog(t), terminal.Options{Mouse: true})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{}, 1)
	w := &terminalSignalWriter{started: started}
	done := make(chan error, 1)
	go func() {
		_, err := tui.Run(ctx, m, tui.WithInput(nil), tui.WithOutput(w), tui.WithColorProfile(colorprofile.Ascii))
		done <- err
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("runner never began output")
	}
	select {
	case err := <-done:
		if !errors.Is(err, tea.ErrProgramKilled) || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled program error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled terminal runner did not return")
	}
}

type terminalSignalWriter struct{ started chan struct{} }

func (w *terminalSignalWriter) Write(p []byte) (int, error) {
	select {
	case w.started <- struct{}{}:
	default:
	}
	return len(p), nil
}
