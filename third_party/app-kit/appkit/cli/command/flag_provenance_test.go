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
	"github.com/spf13/pflag"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
)

func TestFlagParseErrorsRetainTypedProvenanceThroughExecute(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	tests := []struct {
		name      string
		args      []string
		configure func(*cobra.Command)
		isType    func(error) bool
	}{
		{
			name: "unknown flag",
			args: []string{"--missing"},
			isType: func(err error) bool {
				var target *pflag.NotExistError
				return errors.As(err, &target)
			},
		},
		{
			name: "missing flag value",
			args: []string{"--name"},
			configure: func(root *cobra.Command) {
				root.Flags().String("name", "", "name")
			},
			isType: func(err error) bool {
				var target *pflag.ValueRequiredError
				return errors.As(err, &target)
			},
		},
		{
			name: "invalid flag value",
			args: []string{"--count=not-a-number"},
			configure: func(root *cobra.Command) {
				root.Flags().Int("count", 0, "count")
			},
			isType: func(err error) bool {
				var target *pflag.InvalidValueError
				return errors.As(err, &target)
			},
		},
		{
			name: "invalid flag syntax",
			args: []string{"---bad"},
			isType: func(err error) bool {
				var target *pflag.InvalidSyntaxError
				return errors.As(err, &target)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := command.New(command.App{Name: "kittool", Short: "flag provenance test"})
			if tt.configure != nil {
				tt.configure(root)
			}
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(tt.args)
			var hookCalls, renderCalls, runCalls int
			var hookErr, rendered error
			root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
				hookCalls++
				hookErr = err
				return err
			})
			root.RunE = func(*cobra.Command, []string) error {
				runCalls++
				return nil
			}

			code := command.Execute(context.Background(), root, fang.WithErrorHandler(
				command.UnreportedErrorHandler(func(w io.Writer, _ fang.Styles, err error) {
					renderCalls++
					rendered = err
					_, _ = fmt.Fprintln(w, err)
				}),
			))

			if code != 1 || command.Code(rendered) != 1 {
				t.Fatalf("codes = Execute %d, rendered %d; want unchanged defaults 1, 1", code, command.Code(rendered))
			}
			if hookCalls != 1 || renderCalls != 1 || runCalls != 0 {
				t.Fatalf("calls = hook %d, render %d, RunE %d; want 1, 1, 0", hookCalls, renderCalls, runCalls)
			}
			if hookErr != rendered || !tt.isType(hookErr) || !tt.isType(rendered) {
				t.Fatalf("typed provenance lost: hook=%T rendered=%T", hookErr, rendered)
			}
			if stdout.Len() != 0 || strings.Count(stderr.String(), rendered.Error()) != 1 {
				t.Fatalf("output = stdout %q, stderr %q; want one rendered error", stdout.String(), stderr.String())
			}
		})
	}
}

func TestNonFlagFailuresDoNotAcquireFlagProvenance(t *testing.T) {
	t.Setenv("__FANG_TEST_WIDTH", "80")

	tests := []struct {
		name     string
		args     []string
		prepare  func(*cobra.Command) error
		wantCode int
	}{
		{
			name: "unknown command bypasses flag hook",
			args: []string{"missing-command"},
			prepare: func(root *cobra.Command) error {
				root.AddCommand(&cobra.Command{Use: "known", Run: func(*cobra.Command, []string) {}})
				return nil
			},
			wantCode: 1,
		},
		{
			name: "plain RunE failure",
			prepare: func(root *cobra.Command) error {
				failure := errors.New("deliberate RunE failure")
				root.RunE = func(*cobra.Command, []string) error { return failure }
				return failure
			},
			wantCode: 1,
		},
		{
			name: "explicit RunE exit code survives",
			prepare: func(root *cobra.Command) error {
				failure := errors.New("explicit RunE failure")
				root.RunE = func(*cobra.Command, []string) error { return command.Exit(2, failure) }
				return failure
			},
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := command.New(command.App{Name: "kittool", Short: "negative provenance test"})
			failure := tt.prepare(root)
			root.SetArgs(tt.args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			var hookCalls, renderCalls int
			var rendered error
			root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
				hookCalls++
				return err
			})

			code := command.Execute(context.Background(), root, fang.WithErrorHandler(
				command.UnreportedErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
					renderCalls++
					rendered = err
				}),
			))

			if code != tt.wantCode || command.Code(rendered) != tt.wantCode {
				t.Fatalf("codes = Execute %d, rendered %d; want %d", code, command.Code(rendered), tt.wantCode)
			}
			if hookCalls != 0 || renderCalls != 1 {
				t.Fatalf("calls = hook %d, render %d; want 0, 1", hookCalls, renderCalls)
			}
			if isFlagParseError(rendered) {
				t.Fatalf("non-flag failure acquired flag provenance: %T", rendered)
			}
			if failure != nil && !errors.Is(rendered, failure) {
				t.Fatalf("rendered error %v does not retain deliberate RunE error", rendered)
			}
		})
	}
}

func isFlagParseError(err error) bool {
	var notExist *pflag.NotExistError
	var valueRequired *pflag.ValueRequiredError
	var invalidValue *pflag.InvalidValueError
	var invalidSyntax *pflag.InvalidSyntaxError
	return errors.As(err, &notExist) || errors.As(err, &valueRequired) ||
		errors.As(err, &invalidValue) || errors.As(err, &invalidSyntax)
}
