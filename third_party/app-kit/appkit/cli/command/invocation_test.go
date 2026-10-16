package command_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
)

func TestExecuteWithClassifierPreservesExplicitPolicyAndFallbacks(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	first := errors.New("first")
	second := errors.New("second")
	tests := []struct {
		name              string
		err               error
		classifier        func(error) (int, bool)
		want              int
		wantClassifierRun int
	}{
		{name: "nil success", classifier: classifyNine, want: 0},
		{name: "plain default", err: first, want: 1},
		{name: "nil classifier", err: first, classifier: nil, want: 1},
		{name: "classifier declines", err: first, classifier: func(error) (int, bool) { return 9, false }, want: 1, wantClassifierRun: 1},
		{name: "classifier assigns", err: first, classifier: func(error) (int, bool) { return 2, true }, want: 2, wantClassifierRun: 1},
		{name: "invocation classifier assigns", err: invocationError{kind: "command", cause: first}, classifier: command.InvocationOriginCode, want: 2, wantClassifierRun: 1},
		{name: "invocation classifier declines mixed join", err: errors.Join(invocationError{kind: "flags", cause: first}, second), classifier: command.InvocationOriginCode, want: 1, wantClassifierRun: 1},
		{name: "invocation classifier declines unknown", err: invocationError{kind: "future", cause: first}, classifier: command.InvocationOriginCode, want: 1, wantClassifierRun: 1},
		{name: "explicit zero", err: command.Exit(0, first), classifier: classifyNine, want: 0},
		{name: "explicit one", err: command.Exit(1, first), classifier: classifyNine, want: 1},
		{name: "explicit two", err: command.Exit(2, first), classifier: classifyNine, want: 2},
		{name: "explicit other", err: command.Exit(73, first), classifier: classifyNine, want: 73},
		{name: "wrapped explicit", err: fmt.Errorf("context: %w", command.Exit(4, first)), classifier: classifyNine, want: 4},
		{name: "ordered join", err: errors.Join(command.Exit(5, first), command.Exit(6, second)), classifier: classifyNine, want: 5},
		{name: "reversed ordered join", err: errors.Join(command.Exit(6, second), command.Exit(5, first)), classifier: classifyNine, want: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := command.New(command.App{Name: "kittool", Short: "classifier test"})
			root.RunE = func(*cobra.Command, []string) error { return tt.err }
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs(nil)

			calls := 0
			classifier := tt.classifier
			if classifier != nil {
				original := classifier
				classifier = func(err error) (int, bool) {
					calls++
					return original(err)
				}
			}
			got := command.ExecuteWithClassifier(context.Background(), root, classifier,
				fang.WithErrorHandler(func(io.Writer, fang.Styles, error) {}))
			if got != tt.want {
				t.Fatalf("ExecuteWithClassifier() = %d, want %d", got, tt.want)
			}
			if calls != tt.wantClassifierRun {
				t.Fatalf("classifier calls = %d, want %d", calls, tt.wantClassifierRun)
			}
		})
	}
}

func TestExecuteWithClassifierPreservesCallerFangOptionPrecedence(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	failure := errors.New("render through caller handler")
	root := command.New(command.App{Name: "kittool", Short: "option precedence test"})
	root.RunE = func(*cobra.Command, []string) error { return failure }
	var stderr bytes.Buffer
	root.SetOut(io.Discard)
	root.SetErr(&stderr)
	root.SetArgs(nil)

	renders := 0
	classified := 0
	code := command.ExecuteWithClassifier(context.Background(), root, func(err error) (int, bool) {
		classified++
		if !errors.Is(err, failure) {
			t.Fatalf("classifier error = %v, want wrapped failure", err)
		}
		return 2, true
	}, fang.WithErrorHandler(func(w io.Writer, _ fang.Styles, err error) {
		renders++
		_, _ = fmt.Fprintln(w, err)
	}))

	if code != 2 || classified != 1 || renders != 1 {
		t.Fatalf("code/classifier/renderer = %d/%d/%d, want 2/1/1", code, classified, renders)
	}
	if stderr.String() != failure.Error()+"\n" {
		t.Fatalf("stderr = %q, want caller handler output", stderr.String())
	}
}

func TestInvocationOriginCodeRequiresCompleteRecognizedComposition(t *testing.T) {
	plain := errors.New("application failure")
	recognized := []string{"command", "arguments", "required-flags", "flag-group", "flags"}
	for _, kind := range recognized {
		t.Run(kind, func(t *testing.T) {
			err := invocationError{kind: kind, cause: plain}
			if code, ok := command.InvocationOriginCode(err); code != 2 || !ok {
				t.Fatalf("InvocationOriginCode() = (%d, %v), want (2, true)", code, ok)
			}
		})
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil"},
		{name: "plain", err: plain},
		{name: "unknown marker", err: invocationError{kind: "future", cause: plain}},
		{name: "nested recognized", err: fmt.Errorf("outer: %w", invocationError{kind: "command", cause: plain}), want: true},
		{name: "recognized join", err: errors.Join(invocationError{kind: "flags", cause: plain}, fmt.Errorf("nested: %w", invocationError{kind: "arguments", cause: plain})), want: true},
		{name: "mixed join", err: errors.Join(invocationError{kind: "flags", cause: plain}, plain)},
		{name: "unknown join", err: errors.Join(invocationError{kind: "flags", cause: plain}, invocationError{kind: "future", cause: plain})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := command.InvocationOriginCode(tt.err)
			if ok != tt.want {
				t.Fatalf("InvocationOriginCode() = (%d, %v), want classified %v", code, ok, tt.want)
			}
			if ok && code != 2 {
				t.Fatalf("classified code = %d, want 2", code)
			}
			if !ok && code != 0 {
				t.Fatalf("unclassified code = %d, want 0", code)
			}
		})
	}
}

func classifyNine(error) (int, bool) { return 9, true }

type invocationError struct {
	kind  string
	cause error
}

func (e invocationError) Error() string               { return e.kind + " invocation error" }
func (e invocationError) Unwrap() error               { return e.cause }
func (e invocationError) InvocationErrorKind() string { return e.kind }
