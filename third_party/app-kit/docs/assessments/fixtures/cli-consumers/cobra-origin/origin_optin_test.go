//go:build cobraoriginprobe

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
)

type invocationOriginSetter interface {
	SetInvocationErrorOrigins(bool)
}

func requireInvocationOriginSetter(t *testing.T, cmd *cobra.Command) invocationOriginSetter {
	t.Helper()
	setter, ok := any(cmd).(invocationOriginSetter)
	if !ok {
		t.Fatal("candidate is missing (*cobra.Command).SetInvocationErrorOrigins")
	}
	return setter
}

func optInKind(err error) (string, bool) {
	var marker interface{ InvocationErrorKind() string }
	if !errors.As(err, &marker) {
		return "", false
	}
	return marker.InvocationErrorKind(), true
}

func executeDirect(t *testing.T, root *cobra.Command) (int, error) {
	t.Helper()
	var rendered error
	code := command.Execute(context.Background(), root, fang.WithErrorHandler(
		command.UnreportedErrorHandler(func(_ io.Writer, _ fang.Styles, errorValue error) {
			rendered = errorValue
		}),
	))
	return code, rendered
}

func TestInvocationOriginsDefaultPreservesExactErrors(t *testing.T) {
	root := command.New(command.App{Name: "probe", Short: "opt-in default"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.Flags().Int("count", 0, "count")
	root.SetArgs([]string{"--count=bad"})
	var raw error
	var calls int
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		calls++
		raw = err
		return err
	})

	code, rendered := executeDirect(t, root)
	if code != 1 || calls != 1 || raw == nil || rendered != raw {
		t.Fatalf("default execution = code %d calls %d raw %T identity %v", code, calls, raw, rendered == raw)
	}
	var valueErr *pflag.InvalidValueError
	if !errors.As(rendered, &valueErr) {
		t.Fatalf("default concrete parser error lost: %T", rendered)
	}
	if kind, ok := optInKind(rendered); ok {
		t.Fatalf("default error unexpectedly marked %q", kind)
	}

	plain := &cobra.Command{Use: "plain"}
	plainErr := cobra.NoArgs(plain, []string{"extra"})
	if plainErr == nil {
		t.Fatal("NoArgs unexpectedly accepted input")
	}
	if kind, ok := optInKind(plainErr); ok {
		t.Fatalf("default NoArgs error unexpectedly marked %q", kind)
	}
	if err := cobra.NoArgs(nil, nil); err != nil {
		t.Fatalf("nil command with no arguments changed behavior: %v", err)
	}
}

func TestNilCommandValidatorsIndependentOfCommandRemainCompatible(t *testing.T) {
	tests := []struct {
		name        string
		validate    cobra.PositionalArgs
		successArgs []string
		failureArgs []string
	}{
		{"exact", cobra.ExactArgs(1), []string{"one"}, nil},
		{"minimum", cobra.MinimumNArgs(1), []string{"one"}, nil},
		{"maximum", cobra.MaximumNArgs(0), nil, []string{"extra"}},
		{"range", cobra.RangeArgs(1, 2), []string{"one"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.validate(nil, test.successArgs); err != nil {
				t.Fatalf("nil-command success changed: %v", err)
			}
			err := test.validate(nil, test.failureArgs)
			if err == nil {
				t.Fatal("nil-command refusal unexpectedly succeeded")
			}
			if kind, marked := optInKind(err); marked {
				t.Fatalf("default nil-command refusal unexpectedly marked %q", kind)
			}
		})
	}
}

func TestInvocationOriginsEnabledWrapsAfterRawFlagCallback(t *testing.T) {
	root := command.New(command.App{Name: "probe", Short: "opt-in flags"})
	requireInvocationOriginSetter(t, root).SetInvocationErrorOrigins(true)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.Flags().Int("count", 0, "count")
	root.SetArgs([]string{"--count=bad"})
	var raw error
	var calls int
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		calls++
		raw = err
		var valueErr *pflag.InvalidValueError
		if !errors.As(err, &valueErr) {
			t.Fatalf("callback did not receive raw pflag error: %T", err)
		}
		return err
	})

	code, rendered := executeDirect(t, root)
	kind, marked := optInKind(rendered)
	if code != 1 || calls != 1 || raw == nil || rendered == raw || !errors.Is(rendered, raw) || !marked || kind != "flags" {
		t.Fatalf("enabled execution = code %d calls %d identity %v errors.Is %v kind %q/%v",
			code, calls, rendered == raw, errors.Is(rendered, raw), kind, marked)
	}
}

func TestInvocationOriginSettingUsesClosestExplicitAncestor(t *testing.T) {
	rootTrue := &cobra.Command{Use: "true-root"}
	requireInvocationOriginSetter(t, rootTrue).SetInvocationErrorOrigins(true)
	branchFalse := &cobra.Command{Use: "false-branch"}
	requireInvocationOriginSetter(t, branchFalse).SetInvocationErrorOrigins(false)
	leafTrue := &cobra.Command{Use: "true-leaf"}
	requireInvocationOriginSetter(t, leafTrue).SetInvocationErrorOrigins(true)
	rootTrue.AddCommand(branchFalse)
	branchFalse.AddCommand(leafTrue)

	for _, test := range []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{"parent true inherited", &cobra.Command{Use: "inherited"}, true},
		{"child false overrides", branchFalse, false},
		{"leaf true overrides", leafTrue, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.cmd.Parent() == nil {
				rootTrue.AddCommand(test.cmd)
			}
			_, marked := optInKind(cobra.NoArgs(test.cmd, []string{"extra"}))
			if marked != test.want {
				t.Fatalf("marked = %v, want %v", marked, test.want)
			}
		})
	}

	reparented := &cobra.Command{Use: "reparented"}
	rootTrue.AddCommand(reparented)
	if _, ok := optInKind(cobra.NoArgs(reparented, []string{"extra"})); !ok {
		t.Fatal("initial parent setting was not inherited")
	}
	rootTrue.RemoveCommand(reparented)
	rootFalse := &cobra.Command{Use: "false-root"}
	requireInvocationOriginSetter(t, rootFalse).SetInvocationErrorOrigins(false)
	rootFalse.AddCommand(reparented)
	if kind, ok := optInKind(cobra.NoArgs(reparented, []string{"extra"})); ok {
		t.Fatalf("reparented command retained stale origin setting %q", kind)
	}
}

func TestExecuteUsesSelectedCommandsClosestInvocationOriginSetting(t *testing.T) {
	setting := func(cmd *cobra.Command, enabled bool) {
		t.Helper()
		requireInvocationOriginSetter(t, cmd).SetInvocationErrorOrigins(enabled)
	}
	newTree := func(rootEnabled, childExplicit *bool) (*cobra.Command, *cobra.Command) {
		t.Helper()
		root := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
		child := &cobra.Command{Use: "child", Args: cobra.NoArgs, Run: func(*cobra.Command, []string) {}}
		setting(root, *rootEnabled)
		if childExplicit != nil {
			setting(child, *childExplicit)
		}
		root.AddCommand(child)
		return root, child
	}
	executeChild := func(root *cobra.Command) error {
		t.Helper()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"child", "extra"})
		return root.Execute()
	}
	trueValue, falseValue := true, false
	for _, test := range []struct {
		name          string
		root          *bool
		childExplicit *bool
		wantMarked    bool
	}{
		{"parent true inherited", &trueValue, nil, true},
		{"child false overrides", &trueValue, &falseValue, false},
		{"child true under false root", &falseValue, &trueValue, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, _ := newTree(test.root, test.childExplicit)
			err := executeChild(root)
			if err == nil {
				t.Fatal("invalid child arguments unexpectedly succeeded")
			}
			kind, marked := optInKind(err)
			if marked != test.wantMarked || marked && kind != "arguments" {
				t.Fatalf("selected command provenance = %q/%v, want arguments/%v", kind, marked, test.wantMarked)
			}
		})
	}

	t.Run("implicit child follows new parent", func(t *testing.T) {
		rootTrue, child := newTree(&trueValue, nil)
		if kind, marked := optInKind(executeChild(rootTrue)); !marked || kind != "arguments" {
			t.Fatalf("initial inherited provenance = %q/%v", kind, marked)
		}
		rootTrue.RemoveCommand(child)
		rootFalse := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
		setting(rootFalse, false)
		rootFalse.AddCommand(child)
		if kind, marked := optInKind(executeChild(rootFalse)); marked {
			t.Fatalf("implicit child retained stale provenance %q", kind)
		}
	})

	t.Run("explicit child survives reparent", func(t *testing.T) {
		rootTrue, child := newTree(&trueValue, &trueValue)
		rootTrue.RemoveCommand(child)
		rootFalse := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
		setting(rootFalse, false)
		rootFalse.AddCommand(child)
		if kind, marked := optInKind(executeChild(rootFalse)); !marked || kind != "arguments" {
			t.Fatalf("explicit child provenance after reparent = %q/%v", kind, marked)
		}
	})
}

func TestDirectFrameworkValidatorsRespectInvocationOriginOptIn(t *testing.T) {
	tests := []struct {
		name, kind string
		prepare    func(*cobra.Command) func() error
	}{
		{"arguments", "arguments", func(cmd *cobra.Command) func() error {
			cmd.Args = cobra.ExactArgs(1)
			return func() error { return cmd.ValidateArgs(nil) }
		}},
		{"required flags", "required-flags", func(cmd *cobra.Command) func() error {
			cmd.Flags().String("needed", "", "needed")
			if err := cmd.MarkFlagRequired("needed"); err != nil {
				t.Fatal(err)
			}
			return cmd.ValidateRequiredFlags
		}},
		{"flag group", "flag-group", func(cmd *cobra.Command) func() error {
			cmd.Flags().String("a", "", "a")
			cmd.Flags().String("b", "", "b")
			cmd.MarkFlagsOneRequired("a", "b")
			return cmd.ValidateFlagGroups
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "probe"}
			validate := test.prepare(cmd)
			if err := validate(); err == nil {
				t.Fatal("default validation unexpectedly succeeded")
			} else if kind, ok := optInKind(err); ok {
				t.Fatalf("default validation unexpectedly marked %q", kind)
			}
			requireInvocationOriginSetter(t, cmd).SetInvocationErrorOrigins(true)
			err := validate()
			if kind, ok := optInKind(err); !ok || kind != test.kind {
				t.Fatalf("enabled validation kind = %q/%v, want %q/true", kind, ok, test.kind)
			}
		})
	}
}

func TestGeneratedCompletionCommandInheritsInvocationOriginOptIn(t *testing.T) {
	root := &cobra.Command{Use: "probe"}
	requireInvocationOriginSetter(t, root).SetInvocationErrorOrigins(true)
	root.AddCommand(&cobra.Command{Use: "work", Run: func(*cobra.Command, []string) {}})
	root.InitDefaultCompletionCmd()
	completion, _, err := root.Find([]string{"completion"})
	if err != nil || completion == root {
		t.Fatal("default completion command was not generated")
	}
	err = completion.ValidateArgs([]string{"extra"})
	if kind, ok := optInKind(err); !ok || kind != "arguments" {
		t.Fatalf("generated completion validation kind = %q/%v", kind, ok)
	}
}
