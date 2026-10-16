// Command deadline is an opt-in PTY fixture. It proves the terminal catalog's
// context shutdown path; it is not an interactive catalog command.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/tui"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/apps/terminal"
)

const deadlineMarker = "APPKIT_PTY_DEADLINE_PROVEN"

func main() {
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	c, err := catalog.Default()
	if err != nil {
		fmt.Fprintf(stderr, "appkit PTY deadline probe: load catalog: %v\n", err)
		return 1
	}
	m, err := terminal.New(c, terminal.Options{Mouse: true})
	if err != nil {
		fmt.Fprintf(stderr, "appkit PTY deadline probe: build terminal model: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = tui.Run(ctx, m, tui.WithInput(stdin), tui.WithOutput(stdout), tui.WithColorProfile(colorprofile.Ascii))
	return reportDeadlineResult(err, stdout, stderr)
}

// reportDeadlineResult is writer-injected so unit fixtures can prove that both
// cancellation identities are required without starting a terminal program.
func reportDeadlineResult(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, context.DeadlineExceeded) && errors.Is(err, tea.ErrProgramKilled) {
		if _, writeErr := fmt.Fprintln(stdout, deadlineMarker); writeErr != nil {
			fmt.Fprintf(stderr, "appkit PTY deadline probe failed: write success marker: %v\n", writeErr)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stderr, "appkit PTY deadline probe failed: expected context deadline and tea program-killed errors; got %v\n", err)
	return 1
}
