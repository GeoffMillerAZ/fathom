package command_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
)

type classifiedError struct {
	kind string
}

func (e *classifiedError) Error() string { return "classified " + e.kind }

type nilChildrenError struct{}

func (nilChildrenError) Error() string   { return "no non-nil children" }
func (nilChildrenError) Unwrap() []error { return []error{nil} }

func TestReportedPreservesErrorIdentityAndExitCode(t *testing.T) {
	target := &classifiedError{kind: "failure"}

	for _, code := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("exit %d", code), func(t *testing.T) {
			original := command.Exit(code, target)
			got := command.Reported(original)

			if got.Error() != original.Error() {
				t.Fatalf("Error() = %q, want %q", got.Error(), original.Error())
			}
			if !errors.Is(got, target) {
				t.Fatal("errors.Is(reported, target) = false, want true")
			}
			var typed *classifiedError
			if !errors.As(got, &typed) || typed != target {
				t.Fatalf("errors.As(reported) = %v, want original target", typed)
			}
			if gotCode := command.Code(got); gotCode != code {
				t.Fatalf("Code(reported) = %d, want %d", gotCode, code)
			}
		})
	}

	if command.Reported(nil) != nil {
		t.Fatal("Reported(nil) is non-nil")
	}
}

func TestUnreportedErrorHandlerReportingBoundary(t *testing.T) {
	a := errors.New("already shown")
	b := errors.New("not shown")

	tests := []struct {
		name      string
		err       error
		wantCalls int
	}{
		{name: "nil", err: nil, wantCalls: 0},
		{name: "plain", err: b, wantCalls: 1},
		{name: "reported", err: command.Reported(a), wantCalls: 0},
		{name: "wrapped reported", err: fmt.Errorf("operation: %w", command.Reported(a)), wantCalls: 0},
		{name: "mixed join", err: errors.Join(command.Reported(a), b), wantCalls: 1},
		{name: "wrapped mixed join", err: fmt.Errorf("operation: %w", errors.Join(command.Reported(a), b)), wantCalls: 1},
		{name: "all reported join", err: errors.Join(command.Reported(a), command.Reported(b)), wantCalls: 0},
		{name: "reported join with nil", err: errors.Join(command.Reported(a), nil), wantCalls: 0},
		{name: "join shape without non-nil child", err: nilChildrenError{}, wantCalls: 1},
		{name: "outer reported join", err: command.Reported(errors.Join(a, b)), wantCalls: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var rendered error
			handler := command.UnreportedErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
				calls++
				rendered = err
			})
			handler(io.Discard, fang.Styles{}, tt.err)

			if calls != tt.wantCalls {
				t.Fatalf("renderer calls = %d, want %d", calls, tt.wantCalls)
			}
			if tt.wantCalls == 1 && rendered != tt.err {
				t.Fatal("renderer did not receive the original error")
			}
		})
	}
}

func TestUnreportedErrorHandlerNilNextUsesFangDefault(t *testing.T) {
	var stderr bytes.Buffer
	command.UnreportedErrorHandler(nil)(&stderr, fang.Styles{}, errors.New("render me"))
	if !strings.Contains(stderr.String(), "render me") {
		t.Fatalf("default renderer output = %q, want error text", stderr.String())
	}
}

func TestExecuteWithUnreportedErrorHandlerMarksOnlyExplicitlyReportedErrors(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	tests := []struct {
		name        string
		err         error
		wantRenders int
	}{
		{name: "unmarked", err: command.Exit(2, errors.New("not rendered yet")), wantRenders: 1},
		{name: "marked", err: command.Reported(command.Exit(2, errors.New("already rendered"))), wantRenders: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := command.New(command.App{Name: "kittool", Short: "reporting smoke test"})
			root.RunE = func(*cobra.Command, []string) error { return tt.err }
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{})

			var renders int
			code := command.Execute(context.Background(), root, fang.WithErrorHandler(command.UnreportedErrorHandler(
				func(w io.Writer, _ fang.Styles, err error) {
					renders++
					_, _ = fmt.Fprintln(w, err)
				},
			)))

			if code != 2 {
				t.Fatalf("Execute exit code = %d, want preserved explicit code 2", code)
			}
			if renders != tt.wantRenders {
				t.Fatalf("renderer calls = %d, want %d", renders, tt.wantRenders)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if (stderr.Len() != 0) != (tt.wantRenders == 1) {
				t.Fatalf("stderr = %q, want nonempty iff rendered", stderr.String())
			}
		})
	}
}

func TestExecuteUsageShapedErrorsRemainExitOneAndCallerControlled(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	// Exit 1 pins the existing compatibility behavior. This reporting helper
	// does not classify these errors or promote them to the usage/code-2 path.
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--this-flag-does-not-exist"}},
		{name: "unknown command", args: []string{"does-not-exist"}},
	}

	for _, tt := range tests {
		t.Run(tt.name+" renders once", func(t *testing.T) {
			root, stdout, stderr := newFailureRoot(tt.args)
			var renders int
			code := command.Execute(context.Background(), root, fang.WithErrorHandler(command.UnreportedErrorHandler(
				func(w io.Writer, _ fang.Styles, err error) {
					renders++
					_, _ = fmt.Fprintln(w, err)
				},
			)))

			if code != 1 {
				t.Fatalf("Execute exit code = %d, want explicit current behavior 1", code)
			}
			if renders != 1 {
				t.Fatalf("renderer calls = %d, want 1", renders)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("stderr is empty, want rendered error")
			}
		})

		t.Run(tt.name+" respects no-op renderer", func(t *testing.T) {
			root, stdout, stderr := newFailureRoot(tt.args)
			code := command.Execute(context.Background(), root, fang.WithErrorHandler(command.UnreportedErrorHandler(
				func(io.Writer, fang.Styles, error) {},
			)))

			if code != 1 {
				t.Fatalf("Execute exit code = %d, want explicit current behavior 1", code)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("output = stdout %q, stderr %q; want caller-selected silence", stdout.String(), stderr.String())
			}
		})
	}
}

func TestExecuteUnreportedDefaultRendererHonorsNoColor(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")
	t.Setenv("NO_COLOR", "1")

	root := command.New(command.App{Name: "kittool", Short: "no-color smoke test"})
	root.RunE = func(*cobra.Command, []string) error { return errors.New("plain failure") }
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{})

	code := command.Execute(context.Background(), root, fang.WithErrorHandler(command.UnreportedErrorHandler(nil)))
	if code != 1 {
		t.Fatalf("Execute exit code = %d, want 1", code)
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "plain failure") {
		t.Fatalf("stderr = %q, want error text", stderr.String())
	}
	if strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("stderr contains ANSI despite NO_COLOR: %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func newFailureRoot(args []string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	root := command.New(command.App{Name: "kittool", Short: "failure smoke test"})
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.AddCommand(&cobra.Command{Use: "known", Run: func(*cobra.Command, []string) {}})
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)
	return root, stdout, stderr
}
