//go:build cobraoriginprobe

package main

import (
	"bytes"
	"context"
	"encoding"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
)

type invocationMarker interface {
	InvocationErrorKind() string
}

type oneUnwrapper interface{ Unwrap() error }
type manyUnwrapper interface{ Unwrap() []error }

// candidateCode is deliberately test-only. It proves the opt-in policy without
// changing app-kit's default Execute or Code contracts.
func candidateCode(err error) int {
	if code, ok := command.ExplicitCode(err); ok {
		return code
	}
	if whollyInvocation(err) {
		return 2
	}
	return command.Code(err)
}

func whollyInvocation(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(manyUnwrapper); ok {
		n := 0
		for _, child := range joined.Unwrap() {
			if child == nil {
				continue
			}
			n++
			if !whollyInvocation(child) {
				return false
			}
		}
		return n > 0
	}
	if marker, ok := err.(invocationMarker); ok {
		switch marker.InvocationErrorKind() {
		case "command", "arguments", "required-flags", "flag-group", "flags":
			return true
		default:
			return false
		}
	}
	if wrapped, ok := err.(oneUnwrapper); ok {
		return whollyInvocation(wrapped.Unwrap())
	}
	return false
}

type execution struct {
	err                 error
	stdout, stderr      string
	renders, flagErrors int
	order               []string
}

func execute(t *testing.T, root *cobra.Command, args []string, events *[]string, onFlagError func(*cobra.Command, error) error) execution {
	t.Helper()
	t.Setenv("__FANG_TEST_WIDTH", "80")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	result := execution{}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		result.flagErrors++
		*events = append(*events, "flag-error")
		if onFlagError != nil {
			return onFlagError(cmd, err)
		}
		return err
	})
	if setter, ok := any(root).(invocationOriginSetter); ok {
		setter.SetInvocationErrorOrigins(true)
	}
	err := fang.Execute(context.Background(), root,
		fang.WithoutManpage(),
		fang.WithErrorHandler(func(w io.Writer, _ fang.Styles, err error) {
			result.renders++
			*events = append(*events, "render")
			_, _ = fmt.Fprintln(w, err)
		}),
	)
	result.err = err
	result.stdout = stdout.String()
	result.stderr = stderr.String()
	result.order = append([]string(nil), (*events)...)
	return result
}

func newRoot(order *[]string) *cobra.Command {
	root := command.New(command.App{Name: "probe", Short: "origin probe"})
	root.PreRunE = func(*cobra.Command, []string) error {
		*order = append(*order, "pre")
		return nil
	}
	root.RunE = func(*cobra.Command, []string) error {
		*order = append(*order, "run")
		return nil
	}
	return root
}

func invocationKind(err error) (string, bool) {
	var marker invocationMarker
	if !errors.As(err, &marker) {
		return "", false
	}
	return marker.InvocationErrorKind(), true
}

func assertFailure(t *testing.T, got execution, kind, text string, wantOrder []string, wantFlagErrors int) {
	t.Helper()
	if got.err == nil {
		t.Fatal("execution unexpectedly succeeded")
	}
	if got.err.Error() != text {
		t.Fatalf("Error() = %q, want %q", got.err.Error(), text)
	}
	if got.stderr != text+"\n" || got.stdout != "" || got.renders != 1 {
		t.Fatalf("output/renders = stdout %q, stderr %q, renders %d; want one stderr diagnostic", got.stdout, got.stderr, got.renders)
	}
	if got.flagErrors != wantFlagErrors || !reflect.DeepEqual(got.order, wantOrder) {
		t.Fatalf("callbacks = flag errors %d, order %v; want %d, %v", got.flagErrors, got.order, wantFlagErrors, wantOrder)
	}
	if markerKind, ok := invocationKind(got.err); !ok || markerKind != kind {
		t.Fatalf("invocation marker = %q, %v; want %q, true (candidate dependency replacements required)", markerKind, ok, kind)
	}
	if candidateCode(got.err) != 2 || command.Code(got.err) != 1 {
		t.Fatalf("codes = candidate %d, default %d; want 2, 1", candidateCode(got.err), command.Code(got.err))
	}
}

func TestFrameworkInvocationOriginsThroughFang(t *testing.T) {
	tests := []struct {
		name, kind, text string
		args             []string
		configure        func(*cobra.Command)
		order            []string
		flagErrors       int
	}{
		{"unknown-command", "command", `unknown command "missing" for "probe"`, []string{"missing"}, func(root *cobra.Command) {
			root.AddCommand(&cobra.Command{Use: "known", Run: func(*cobra.Command, []string) {}})
		}, []string{"render"}, 0},
		{"no-args", "arguments", `unknown command "extra" for "probe"`, []string{"extra"}, func(root *cobra.Command) { root.Args = cobra.NoArgs }, []string{"render"}, 0},
		{"only-valid-args", "arguments", `invalid argument "bad" for "probe"`, []string{"bad"}, func(root *cobra.Command) { root.Args, root.ValidArgs = cobra.OnlyValidArgs, []string{"good"} }, []string{"render"}, 0},
		{"minimum-args", "arguments", "requires at least 1 arg(s), only received 0", nil, func(root *cobra.Command) { root.Args = cobra.MinimumNArgs(1) }, []string{"render"}, 0},
		{"maximum-args", "arguments", "accepts at most 0 arg(s), received 1", []string{"extra"}, func(root *cobra.Command) { root.Args = cobra.MaximumNArgs(0) }, []string{"render"}, 0},
		{"exact-args", "arguments", "accepts 2 arg(s), received 1", []string{"one"}, func(root *cobra.Command) { root.Args = cobra.ExactArgs(2) }, []string{"render"}, 0},
		{"range-args", "arguments", "accepts between 2 and 3 arg(s), received 1", []string{"one"}, func(root *cobra.Command) { root.Args = cobra.RangeArgs(2, 3) }, []string{"render"}, 0},
		{"required-flag", "required-flags", `required flag(s) "needed" not set`, nil, func(root *cobra.Command) { root.Flags().String("needed", "", ""); _ = root.MarkFlagRequired("needed") }, []string{"pre", "render"}, 0},
		{"required-together", "flag-group", "if any flags in the group [a b] are set they must all be set; missing [b]", []string{"--a=x"}, func(root *cobra.Command) {
			root.Flags().String("a", "", "")
			root.Flags().String("b", "", "")
			root.MarkFlagsRequiredTogether("a", "b")
		}, []string{"pre", "render"}, 0},
		{"one-required", "flag-group", "at least one of the flags in the group [a b] is required", nil, func(root *cobra.Command) {
			root.Flags().String("a", "", "")
			root.Flags().String("b", "", "")
			root.MarkFlagsOneRequired("a", "b")
		}, []string{"pre", "render"}, 0},
		{"mutually-exclusive", "flag-group", "if any flags in the group [a b] are set none of the others can be; [a b] were all set", []string{"--a=x", "--b=y"}, func(root *cobra.Command) {
			root.Flags().String("a", "", "")
			root.Flags().String("b", "", "")
			root.MarkFlagsMutuallyExclusive("a", "b")
		}, []string{"pre", "render"}, 0},
		{"unknown-flag", "flags", "unknown flag: --missing", []string{"--missing"}, nil, []string{"flag-error", "render"}, 1},
		{"missing-flag-value", "flags", "flag needs an argument: --name", []string{"--name"}, func(root *cobra.Command) { root.Flags().String("name", "", "") }, []string{"flag-error", "render"}, 1},
		{"invalid-flag-syntax", "flags", "bad flag syntax: ---bad", []string{"---bad"}, nil, []string{"flag-error", "render"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lifecycle []string
			root := newRoot(&lifecycle)
			if tt.configure != nil {
				tt.configure(root)
			}
			got := execute(t, root, tt.args, &lifecycle, nil)
			assertFailure(t, got, tt.kind, tt.text, tt.order, tt.flagErrors)
		})
	}
}

func TestAllBuiltinValueConversionOrigins(t *testing.T) {
	tests := []struct {
		name, invalid string
		register      func(*pflag.FlagSet)
	}{
		{"bool", "bad", func(f *pflag.FlagSet) { f.Bool("value", false, "") }},
		{"bool-slice", "bad", func(f *pflag.FlagSet) { f.BoolSlice("value", nil, "") }},
		{"bytes-hex", "!", func(f *pflag.FlagSet) { f.BytesHex("value", nil, "") }},
		{"bytes-base64", "!", func(f *pflag.FlagSet) { f.BytesBase64("value", nil, "") }},
		{"count", "bad", func(f *pflag.FlagSet) { f.Count("value", "") }},
		{"duration", "bad", func(f *pflag.FlagSet) { f.Duration("value", 0, "") }},
		{"duration-slice", "bad", func(f *pflag.FlagSet) { f.DurationSlice("value", nil, "") }},
		{"float32", "bad", func(f *pflag.FlagSet) { f.Float32("value", 0, "") }},
		{"float64", "bad", func(f *pflag.FlagSet) { f.Float64("value", 0, "") }},
		{"float32-slice", "bad", func(f *pflag.FlagSet) { f.Float32Slice("value", nil, "") }},
		{"float64-slice", "bad", func(f *pflag.FlagSet) { f.Float64Slice("value", nil, "") }},
		{"int", "bad", func(f *pflag.FlagSet) { f.Int("value", 0, "") }},
		{"int8", "bad", func(f *pflag.FlagSet) { f.Int8("value", 0, "") }},
		{"int16", "bad", func(f *pflag.FlagSet) { f.Int16("value", 0, "") }},
		{"int32", "bad", func(f *pflag.FlagSet) { f.Int32("value", 0, "") }},
		{"int64", "bad", func(f *pflag.FlagSet) { f.Int64("value", 0, "") }},
		{"int-slice", "bad", func(f *pflag.FlagSet) { f.IntSlice("value", nil, "") }},
		{"int32-slice", "bad", func(f *pflag.FlagSet) { f.Int32Slice("value", nil, "") }},
		{"int64-slice", "bad", func(f *pflag.FlagSet) { f.Int64Slice("value", nil, "") }},
		{"uint", "bad", func(f *pflag.FlagSet) { f.Uint("value", 0, "") }},
		{"uint8", "bad", func(f *pflag.FlagSet) { f.Uint8("value", 0, "") }},
		{"uint16", "bad", func(f *pflag.FlagSet) { f.Uint16("value", 0, "") }},
		{"uint32", "bad", func(f *pflag.FlagSet) { f.Uint32("value", 0, "") }},
		{"uint64", "bad", func(f *pflag.FlagSet) { f.Uint64("value", 0, "") }},
		{"uint-slice", "bad", func(f *pflag.FlagSet) { f.UintSlice("value", nil, "") }},
		{"ip", "bad", func(f *pflag.FlagSet) { f.IP("value", nil, "") }},
		{"ip-slice", "bad", func(f *pflag.FlagSet) { f.IPSlice("value", nil, "") }},
		{"ip-net", "bad", func(f *pflag.FlagSet) { f.IPNet("value", net.IPNet{}, "") }},
		{"ip-net-slice", "bad", func(f *pflag.FlagSet) { f.IPNetSlice("value", nil, "") }},
		{"ip-mask", "bad", func(f *pflag.FlagSet) { f.IPMask("value", nil, "") }},
		{"string-slice", `"`, func(f *pflag.FlagSet) { f.StringSlice("value", nil, "") }},
		{"string-to-int", "key=bad", func(f *pflag.FlagSet) { f.StringToInt("value", map[string]int{}, "") }},
		{"string-to-int64", "key=bad", func(f *pflag.FlagSet) { f.StringToInt64("value", map[string]int64{}, "") }},
		{"string-to-string", "bad", func(f *pflag.FlagSet) { f.StringToString("value", map[string]string{}, "") }},
		{"time", "bad", func(f *pflag.FlagSet) { f.Time("value", time.Time{}, []string{time.RFC3339}, "") }},
	}
	if len(tests) != 35 {
		t.Fatalf("builtin failure matrix has %d cases, want 35", len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			tt.register(root.Flags())
			got := execute(t, root, []string{"--value=" + tt.invalid}, &order, nil)
			if got.err == nil {
				t.Fatal("invalid built-in value succeeded")
			}
			var invalid *pflag.InvalidValueError
			if !errors.As(got.err, &invalid) {
				t.Fatalf("error type = %T, want *pflag.InvalidValueError", got.err)
			}
			provenance, ok := any(invalid).(interface{ IsBuiltinConversion() bool })
			if !ok || !provenance.IsBuiltinConversion() {
				t.Fatal("built-in provenance absent (candidate replacement required)")
			}
			if kind, ok := invocationKind(got.err); !ok || kind != "flags" {
				t.Fatalf("kind = %q, %v; want flags", kind, ok)
			}
			if got.err.Error() != invalid.Error() || got.stderr != got.err.Error()+"\n" || got.renders != 1 || got.flagErrors != 1 {
				t.Fatalf("transparent render failure: err=%q stderr=%q renders=%d flagErrors=%d", got.err, got.stderr, got.renders, got.flagErrors)
			}
			if candidateCode(got.err) != 2 || command.Code(got.err) != 1 || !reflect.DeepEqual(order, []string{"flag-error", "render"}) {
				t.Fatalf("codes/order = %d, %d, %v", candidateCode(got.err), command.Code(got.err), order)
			}
		})
	}
}

type failingValue struct {
	err      error
	typeName string
	calls    *int
}

func (v *failingValue) String() string   { return "" }
func (v *failingValue) Type() string     { return v.typeName }
func (v *failingValue) Set(string) error { *v.calls++; return v.err }

type failingText struct {
	err   error
	calls *int
}

var _ encoding.TextMarshaler = (*failingText)(nil)
var _ encoding.TextUnmarshaler = (*failingText)(nil)

func (v *failingText) MarshalText() ([]byte, error) { return nil, nil }
func (v *failingText) UnmarshalText([]byte) error   { *v.calls++; return v.err }

func TestCallbackAndCustomValuesRemainApplicationErrors(t *testing.T) {
	tests := []struct {
		name     string
		register func(*pflag.FlagSet, error, *int)
	}{
		{"custom", func(f *pflag.FlagSet, err error, calls *int) {
			f.Var(&failingValue{err: err, typeName: "custom", calls: calls}, "value", "")
		}},
		{"spoofed-custom", func(f *pflag.FlagSet, err error, calls *int) {
			f.Var(&failingValue{err: err, typeName: "int", calls: calls}, "value", "")
		}},
		{"func", func(f *pflag.FlagSet, err error, calls *int) {
			f.Func("value", "", func(string) error { *calls++; return err })
		}},
		{"bool-func", func(f *pflag.FlagSet, err error, calls *int) {
			f.BoolFunc("value", "", func(string) error { *calls++; return err })
		}},
		{"text", func(f *pflag.FlagSet, err error, calls *int) {
			v := &failingText{err: err, calls: calls}
			f.TextVar(v, "value", v, "")
		}},
		{"go-flag", func(f *pflag.FlagSet, err error, calls *int) {
			g := flag.NewFlagSet("go", flag.ContinueOnError)
			g.Var(&failingValue{err: err, typeName: "int", calls: calls}, "value", "")
			f.AddGoFlag(g.Lookup("value"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := errors.New("callback sentinel")
			calls := 0
			var order []string
			root := newRoot(&order)
			tt.register(root.Flags(), failure, &calls)
			got := execute(t, root, []string{"--value=bad"}, &order, nil)
			if got.err == nil || !errors.Is(got.err, failure) || calls != 1 {
				t.Fatalf("error/calls = %v/%d; sentinel or once lost", got.err, calls)
			}
			if _, ok := invocationKind(got.err); ok || candidateCode(got.err) != 1 {
				t.Fatalf("application error reclassified: %T, code %d", got.err, candidateCode(got.err))
			}
			var invalid *pflag.InvalidValueError
			if !errors.As(got.err, &invalid) {
				t.Fatalf("pflag wrapper lost: %T", got.err)
			}
			if provenance, ok := any(invalid).(interface{ IsBuiltinConversion() bool }); !ok || provenance.IsBuiltinConversion() {
				t.Fatalf("custom provenance = %v, %v; want available false", provenance, ok)
			}
			if got.renders != 1 || got.flagErrors != 1 || !reflect.DeepEqual(order, []string{"flag-error", "render"}) || strings.Count(got.stderr, failure.Error()) != 1 {
				t.Fatalf("callbacks/output = render %d flag %d order %v stderr %q", got.renders, got.flagErrors, order, got.stderr)
			}
		})
	}
}

func TestFlagErrorCallbackReplacementAndCapturedMutation(t *testing.T) {
	t.Run("callback-return-is-application-error", func(t *testing.T) {
		callbackFailure := errors.New("flag callback refused")
		var order []string
		root := newRoot(&order)
		root.Flags().Int("value", 0, "")
		got := execute(t, root, []string{"--value=bad"}, &order, func(*cobra.Command, error) error { return callbackFailure })
		if got.err != callbackFailure || candidateCode(got.err) != 1 || got.flagErrors != 1 || got.renders != 1 {
			t.Fatalf("callback failure changed: %#v", got)
		}
	})

	t.Run("identity-callback-sees-raw-error-once", func(t *testing.T) {
		var order []string
		root := newRoot(&order)
		root.Flags().Int("value", 0, "")
		var seen error
		got := execute(t, root, []string{"--value=bad"}, &order, func(_ *cobra.Command, err error) error {
			seen = err
			return err
		})
		if _, ok := seen.(*pflag.InvalidValueError); !ok {
			t.Fatalf("flag callback saw %T, want direct *pflag.InvalidValueError", seen)
		}
		if _, marked := seen.(invocationMarker); marked {
			t.Fatalf("raw callback error was prematurely wrapped: %T", seen)
		}
		if got.err == seen || !errors.Is(got.err, seen) || candidateCode(got.err) != 2 {
			t.Fatalf("post-callback wrapper = %T, identity=%v, errors.Is=%v, code=%d", got.err, got.err == seen, errors.Is(got.err, seen), candidateCode(got.err))
		}
		if !reflect.DeepEqual(order, []string{"flag-error", "render"}) {
			t.Fatalf("callback order = %v", order)
		}
	})

	t.Run("callback-contextual-wrapper-is-application-policy", func(t *testing.T) {
		var order []string
		root := newRoot(&order)
		root.Flags().Int("value", 0, "")
		var seen error
		got := execute(t, root, []string{"--value=bad"}, &order, func(_ *cobra.Command, err error) error {
			seen = err
			return fmt.Errorf("callback context: %w", err)
		})
		if !errors.Is(got.err, seen) || candidateCode(got.err) != 1 {
			t.Fatalf("callback wrapper reclassified: %T, code %d", got.err, candidateCode(got.err))
		}
		if _, marked := invocationKind(got.err); marked {
			t.Fatalf("callback wrapper acquired invocation origin: %T", got.err)
		}
	})

	for _, tt := range []struct {
		name string
		wrap func(error) error
		code int
	}{
		{"callback-reported-wrapper", command.Reported, 1},
		{"callback-explicit-zero", func(err error) error { return command.Exit(0, err) }, 0},
		{"callback-explicit-two", func(err error) error { return command.Exit(2, err) }, 2},
		{"callback-explicit-other", func(err error) error { return command.Exit(9, err) }, 9},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			root.Flags().Int("value", 0, "")
			var seen error
			got := execute(t, root, []string{"--value=bad"}, &order, func(_ *cobra.Command, err error) error {
				seen = err
				return tt.wrap(err)
			})
			if !errors.Is(got.err, seen) || candidateCode(got.err) != tt.code {
				t.Fatalf("callback policy = errors.Is %v, code %d; want true, %d", errors.Is(got.err, seen), candidateCode(got.err), tt.code)
			}
			if _, marked := invocationKind(got.err); marked {
				t.Fatalf("callback-owned wrapper acquired invocation origin: %T", got.err)
			}
			if got.renders != 1 || !reflect.DeepEqual(order, []string{"flag-error", "render"}) {
				t.Fatalf("callback render/order = %d, %v", got.renders, order)
			}
		})
	}

	t.Run("callback-nil-stops-with-success", func(t *testing.T) {
		var order []string
		root := newRoot(&order)
		root.Flags().Int("value", 0, "")
		got := execute(t, root, []string{"--value=bad"}, &order, func(*cobra.Command, error) error { return nil })
		if got.err != nil || candidateCode(got.err) != 0 || got.renders != 0 || got.flagErrors != 1 || !reflect.DeepEqual(order, []string{"flag-error"}) {
			t.Fatalf("nil callback policy changed: %#v order=%v", got, order)
		}
	})

	t.Run("builtin-remains-builtin-after-value-replacement", func(t *testing.T) {
		var order []string
		root := newRoot(&order)
		root.Flags().Int("value", 0, "")
		calls := 0
		custom := &failingValue{err: errors.New("unused"), typeName: "custom", calls: &calls}
		got := execute(t, root, []string{"--value=bad"}, &order, func(cmd *cobra.Command, err error) error { cmd.Flags().Lookup("value").Value = custom; return err })
		if candidateCode(got.err) != 2 || calls != 0 {
			t.Fatalf("stored provenance changed after failure: code %d calls %d", candidateCode(got.err), calls)
		}
	})

	t.Run("custom-remains-custom-after-value-replacement", func(t *testing.T) {
		failure := errors.New("custom failure")
		calls := 0
		var order []string
		root := newRoot(&order)
		root.Flags().Var(&failingValue{err: failure, typeName: "int", calls: &calls}, "value", "")
		builtins := pflag.NewFlagSet("builtins", pflag.ContinueOnError)
		builtins.Int("replacement", 0, "")
		got := execute(t, root, []string{"--value=bad"}, &order, func(cmd *cobra.Command, err error) error {
			cmd.Flags().Lookup("value").Value = builtins.Lookup("replacement").Value
			return err
		})
		if candidateCode(got.err) != 1 || !errors.Is(got.err, failure) || calls != 1 {
			t.Fatalf("custom provenance changed after failure: code %d calls %d err %v", candidateCode(got.err), calls, got.err)
		}
	})
}

func TestTraverseFlagFailureIsTypedWithoutFlagCallback(t *testing.T) {
	var order []string
	root := newRoot(&order)
	root.TraverseChildren = true
	root.PersistentFlags().Int("value", 0, "")
	root.AddCommand(&cobra.Command{Use: "child", Run: func(*cobra.Command, []string) { order = append(order, "child") }})
	got := execute(t, root, []string{"--value=bad", "child"}, &order, nil)
	if got.err == nil || got.flagErrors != 0 || got.renders != 1 || candidateCode(got.err) != 2 {
		t.Fatalf("traverse failure = err %v flags %d renders %d code %d", got.err, got.flagErrors, got.renders, candidateCode(got.err))
	}
	if kind, ok := invocationKind(got.err); !ok || kind != "flags" {
		t.Fatalf("traverse kind = %q, %v", kind, ok)
	}
	if !reflect.DeepEqual(order, []string{"render"}) {
		t.Fatalf("traverse callbacks = %v", order)
	}
}

func TestApplicationHooksAndArgsAreNotInvocationErrors(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*cobra.Command, error, *[]string)
		args      []string
		wantOrder []string
	}{
		{"custom-args", func(root *cobra.Command, failure error, order *[]string) {
			root.Args = func(*cobra.Command, []string) error { *order = append(*order, "args"); return failure }
		}, nil, []string{"args", "render"}},
		{"pre-run", func(root *cobra.Command, failure error, order *[]string) {
			root.PreRunE = func(*cobra.Command, []string) error { *order = append(*order, "pre"); return failure }
		}, nil, []string{"pre", "render"}},
		{"run", func(root *cobra.Command, failure error, order *[]string) {
			root.RunE = func(*cobra.Command, []string) error { *order = append(*order, "run"); return failure }
		}, nil, []string{"pre", "run", "render"}},
		{"post-run", func(root *cobra.Command, failure error, order *[]string) {
			root.PostRunE = func(*cobra.Command, []string) error { *order = append(*order, "post"); return failure }
		}, nil, []string{"pre", "run", "post", "render"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := errors.New("application sentinel")
			var order []string
			root := newRoot(&order)
			tt.configure(root, failure, &order)
			got := execute(t, root, tt.args, &order, nil)
			if !errors.Is(got.err, failure) || candidateCode(got.err) != 1 || got.flagErrors != 0 || got.renders != 1 || !reflect.DeepEqual(order, tt.wantOrder) {
				t.Fatalf("application lifecycle changed: err %v code %d flags %d renders %d order %v", got.err, candidateCode(got.err), got.flagErrors, got.renders, order)
			}
			if _, ok := invocationKind(got.err); ok {
				t.Fatalf("application error acquired invocation marker: %T", got.err)
			}
		})
	}
}

func TestExplicitRunExitCodesSurviveActualFangExecution(t *testing.T) {
	for _, code := range []int{0, 1, 2, 7, -3} {
		t.Run(fmt.Sprintf("code-%d", code), func(t *testing.T) {
			failure := errors.New("explicit run sentinel")
			var order []string
			root := newRoot(&order)
			root.RunE = func(*cobra.Command, []string) error {
				order = append(order, "run")
				return command.Exit(code, failure)
			}
			got := execute(t, root, nil, &order, nil)
			if got.err == nil || !errors.Is(got.err, failure) || candidateCode(got.err) != code || command.Code(got.err) != code {
				t.Fatalf("explicit result = err %v, candidate %d, default %d; want %d", got.err, candidateCode(got.err), command.Code(got.err), code)
			}
			if _, marked := invocationKind(got.err); marked {
				t.Fatalf("explicit application exit acquired invocation origin: %T", got.err)
			}
			if got.renders != 1 || got.stderr != failure.Error()+"\n" || !reflect.DeepEqual(order, []string{"pre", "run", "render"}) {
				t.Fatalf("render/order = %d, %q, %v", got.renders, got.stderr, order)
			}
		})
	}
}

func TestCandidateClassifierCompositionAndExplicitCodes(t *testing.T) {
	var order []string
	root := newRoot(&order)
	root.Args = cobra.ExactArgs(1)
	invocation := execute(t, root, nil, &order, nil).err
	if invocation == nil {
		t.Fatal("failed to obtain actual invocation error")
	}
	ordinary := errors.New("ordinary")
	tests := []struct {
		name string
		err  error
		code int
	}{
		{"nil", nil, 0},
		{"ordinary", ordinary, 1},
		{"wrapped-invocation", fmt.Errorf("context: %w", invocation), 2},
		{"reported-invocation", command.Reported(invocation), 2},
		{"all-invocation-join", errors.Join(invocation, fmt.Errorf("again: %w", invocation)), 2},
		{"mixed-join", errors.Join(invocation, ordinary), 1},
		{"explicit-zero", command.Exit(0, ordinary), 0},
		{"explicit-one", command.Exit(1, invocation), 1},
		{"explicit-two", command.Exit(2, ordinary), 2},
		{"explicit-other", command.Exit(7, ordinary), 7},
		{"explicit-negative", command.Exit(-3, ordinary), -3},
		{"nested-explicit-outer-first", command.Exit(4, command.Exit(2, ordinary)), 4},
		{"joined-explicit-left-first", errors.Join(command.Exit(6, ordinary), command.Exit(8, ordinary)), 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := candidateCode(tt.err); got != tt.code {
				t.Fatalf("code = %d, want %d", got, tt.code)
			}
		})
	}
}

func TestGeneratedHelpAndCompletionSemantics(t *testing.T) {
	t.Run("help-is-success", func(t *testing.T) {
		var order []string
		got := execute(t, newRoot(&order), []string{"--help"}, &order, nil)
		if got.err != nil || candidateCode(got.err) != 0 || got.renders != 0 || got.flagErrors != 0 || got.stdout == "" || got.stderr != "" || len(order) != 0 {
			t.Fatalf("help behavior changed: %#v order=%v", got, order)
		}
	})
	t.Run("generated-completion-argument", func(t *testing.T) {
		var order []string
		got := execute(t, newRoot(&order), []string{"completion", "bash", "extra"}, &order, nil)
		if got.err == nil {
			t.Fatal("invalid generated completion unexpectedly succeeded")
		}
		kind, ok := invocationKind(got.err)
		if !ok || kind != "arguments" || candidateCode(got.err) != 2 || got.renders != 1 || got.flagErrors != 0 {
			t.Fatalf("completion diagnostic = kind %q/%v code %d renders %d flagErrors %d err %v", kind, ok, candidateCode(got.err), got.renders, got.flagErrors, got.err)
		}
	})
}

func TestStringBuiltinsStillSucceed(t *testing.T) {
	for _, tt := range []struct {
		name     string
		register func(*pflag.FlagSet)
	}{
		{"string", func(f *pflag.FlagSet) { f.String("value", "", "") }},
		{"string-array", func(f *pflag.FlagSet) { f.StringArray("value", nil, "") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			tt.register(root.Flags())
			got := execute(t, root, []string{"--value=anything"}, &order, nil)
			if got.err != nil || candidateCode(got.err) != 0 || got.renders != 0 || got.flagErrors != 0 || !reflect.DeepEqual(order, []string{"pre", "run"}) {
				t.Fatalf("success changed: %#v order=%v", got, order)
			}
		})
	}
}
