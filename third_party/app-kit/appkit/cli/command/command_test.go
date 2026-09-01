package command_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
)

// TestExitCodeMapping is table-driven over Code's every branch: nil, a
// plain error (defaults to 1, never guesses 2), and Exit-wrapped errors at
// each of the convention's three codes — including 0, which must still
// round-trip if a caller explicitly wraps a non-nil error at code 0.
func TestExitCodeMapping(t *testing.T) {
	plainErr := errors.New("boom")

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain error defaults to 1", plainErr, 1},
		{"Exit(0, err)", command.Exit(0, plainErr), 0},
		{"Exit(1, err)", command.Exit(1, plainErr), 1},
		{"Exit(2, err)", command.Exit(2, plainErr), 2},
		{"Exit(_, nil) is nil", command.Exit(2, nil), 0},
		{"wrapped Exit still resolves", fmtWrap(command.Exit(2, plainErr)), 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := command.Code(c.err); got != c.want {
				t.Errorf("Code(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func fmtWrap(err error) error {
	return errors.Join(errors.New("context"), err)
}

func TestExitErrorMessageAndUnwrap(t *testing.T) {
	inner := errors.New("disk full")
	wrapped := command.Exit(1, inner)
	if wrapped.Error() != "disk full" {
		t.Errorf("Error() = %q, want %q", wrapped.Error(), "disk full")
	}
	if !errors.Is(wrapped, inner) {
		t.Errorf("errors.Is(wrapped, inner) = false, want true (Unwrap must expose inner)")
	}
}

// TestExecuteVersionFlag runs --version through Execute against a buffer
// and asserts the configured version string is present in the output —
// proof that command.New + Execute actually wire App.Version through to
// fang, not just that fang's own tests pass.
func TestExecuteVersionFlag(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	root := command.New(command.App{
		Name:    "kittool",
		Short:   "a cli-kit smoke test",
		Version: "v9.9.9",
		Commit:  "deadbeef",
	})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--version"})

	code := command.Execute(context.Background(), root)

	if code != 0 {
		t.Fatalf("Execute(--version) exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "v9.9.9") {
		t.Errorf("stdout does not contain version %q:\n%s", "v9.9.9", stdout.String())
	}
}

// TestExecuteUnknownFlagReturnsNonZero proves an actual failure (not just
// the happy path) makes it through Execute as a non-zero exit code.
func TestExecuteUnknownFlagReturnsNonZero(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	root := command.New(command.App{Name: "kittool", Short: "smoke test"})
	root.RunE = func(*cobra.Command, []string) error { return nil }
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--this-flag-does-not-exist"})

	code := command.Execute(context.Background(), root)

	if code == 0 {
		t.Fatalf("Execute(unknown flag) exit code = 0, want non-zero")
	}
}
