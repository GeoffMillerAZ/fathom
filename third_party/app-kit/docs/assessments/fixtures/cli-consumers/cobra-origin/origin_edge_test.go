//go:build cobraoriginprobe

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"
	"testing"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type goOnlyValue struct {
	calls int
	err   error
}

func (v *goOnlyValue) String() string   { return "" }
func (v *goOnlyValue) Set(string) error { v.calls++; return v.err }

type wrappedValue struct {
	pflag.Value
	set func(string) error
}

func (v *wrappedValue) Set(s string) error { return v.set(s) }

type markedComposition struct {
	kind     string
	children []error
}

func (e markedComposition) Error() string               { return "synthetic composition" }
func (e markedComposition) InvocationErrorKind() string { return e.kind }
func (e markedComposition) Unwrap() []error             { return e.children }

type unknownOrigin struct{}

func (unknownOrigin) Error() string               { return "unknown" }
func (unknownOrigin) InvocationErrorKind() string { return "future-unknown-kind" }

func TestCandidateRejectsUnrecognizedAndMixedMarkers(t *testing.T) {
	for _, err := range []error{unknownOrigin{}, markedComposition{kind: "flags"}, markedComposition{kind: "flags", children: []error{errors.New("operational")}}} {
		if candidateCode(err) != 1 {
			t.Fatalf("unearned invocation accepted: %#v", err)
		}
	}
}

func TestGoBridgeAndSynchronousValueMutationStayOperational(t *testing.T) {
	for _, mode := range []string{"go-bridge", "embedded-builtin", "mutate-during-set", "direct-add-flag"} {
		t.Run(mode, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			failure := errors.New("conversion-like application failure")
			builtin := pflag.NewFlagSet("values", pflag.ContinueOnError)
			builtin.Int("integer", 0, "")
			calls := 0
			if mode == "go-bridge" {
				value := &goOnlyValue{err: failure}
				goFlags := flag.NewFlagSet("go", flag.ContinueOnError)
				goFlags.Var(value, "value", "")
				root.Flags().AddGoFlag(goFlags.Lookup("value"))
				defer func() {
					if value.calls != 1 {
						t.Errorf("Go bridge called %d times", value.calls)
					}
				}()
			} else {
				value := &wrappedValue{Value: builtin.Lookup("integer").Value}
				value.set = func(string) error {
					calls++
					if mode == "mutate-during-set" {
						root.Flags().Lookup("value").Value = builtin.Lookup("integer").Value
					}
					return failure
				}
				if mode == "direct-add-flag" {
					root.Flags().AddFlag(&pflag.Flag{Name: "value", Value: value})
				} else {
					root.Flags().Var(value, "value", "")
				}
			}
			got := execute(t, root, []string{"--value=bad"}, &order, nil)
			if !errors.Is(got.err, failure) || candidateCode(got.err) != 1 || got.renders != 1 || got.flagErrors != 1 {
				t.Fatalf("operational refusal changed: %+v", got)
			}
			if mode != "go-bridge" && calls != 1 {
				t.Fatalf("Set called %d times", calls)
			}
			var valueErr *pflag.InvalidValueError
			if !errors.As(got.err, &valueErr) {
				t.Fatalf("missing pflag error: %v", got.err)
			}
			origin, ok := any(valueErr).(interface{ IsBuiltinConversion() bool })
			if !ok || origin.IsBuiltinConversion() {
				t.Fatal("arbitrary callback treated as built-in conversion")
			}
		})
	}
}

func TestRawParseFlagsRetainsConcreteError(t *testing.T) {
	var order []string
	root := newRoot(&order)
	root.Flags().Int("value", 0, "")
	err := root.ParseFlags([]string{"--value=bad"})
	if _, ok := err.(*pflag.InvalidValueError); !ok {
		t.Fatalf("raw ParseFlags changed type: %T", err)
	}
	if _, ok := invocationKind(err); ok {
		t.Fatal("raw pflag API acquired command invocation provenance")
	}
}

func TestBuiltinArgumentCompositionsAndHiddenCompletion(t *testing.T) {
	for _, mode := range []string{"match-all", "exact-valid", "hidden-complete"} {
		t.Run(mode, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			args := []string{"bad"}
			root.ValidArgs = []string{"good"}
			switch mode {
			case "match-all":
				root.Args = cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)
			case "exact-valid":
				root.Args = cobra.ExactValidArgs(1)
			case "hidden-complete":
				args = []string{"__complete"}
			}
			got := execute(t, root, args, &order, nil)
			if kind, ok := invocationKind(got.err); !ok || kind != "arguments" || candidateCode(got.err) != 2 {
				t.Fatalf("missing invocation argument refusal: %v", got.err)
			}
			if got.renders != 1 || got.flagErrors != 0 || !reflect.DeepEqual(order, []string{"render"}) {
				t.Fatalf("unexpected callbacks: %+v", got)
			}
		})
	}
}

func TestPersistentHooksAndRequiredFlagPrecedence(t *testing.T) {
	for _, phase := range []string{"persistent-pre", "persistent-post", "pre-before-required", "pre-before-group"} {
		t.Run(phase, func(t *testing.T) {
			var order []string
			root := newRoot(&order)
			failure := errors.New(`unknown command "missing" for "probe"`)
			fail := func(*cobra.Command, []string) error { order = append(order, "refuse"); return failure }
			want := []string{"refuse", "render"}
			switch phase {
			case "persistent-pre":
				root.PersistentPreRunE = fail
			case "persistent-post":
				root.PersistentPostRunE = fail
				want = []string{"pre", "run", "refuse", "render"}
			case "pre-before-required":
				root.Flags().String("needed", "", "")
				_ = root.MarkFlagRequired("needed")
				root.PreRunE = fail
			case "pre-before-group":
				root.Flags().String("a", "", "")
				root.Flags().String("b", "", "")
				root.MarkFlagsOneRequired("a", "b")
				root.PreRunE = fail
			}
			got := execute(t, root, nil, &order, nil)
			if got.err != failure || candidateCode(got.err) != 1 || !reflect.DeepEqual(order, want) {
				t.Fatalf("consumer precedence changed: %+v", got)
			}
		})
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCompletionWriterFailureRemainsOperational(t *testing.T) {
	var order []string
	root := newRoot(&order)
	failure := errors.New("synthetic output unavailable")
	root.SetOut(failingWriter{failure})
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"completion", "bash"})
	renders := 0
	err := fang.Execute(context.Background(), root, fang.WithoutManpage(), fang.WithErrorHandler(func(w io.Writer, _ fang.Styles, err error) { renders++; _, _ = fmt.Fprintln(w, err) }))
	if !errors.Is(err, failure) || candidateCode(err) != 1 || renders != 1 {
		t.Fatalf("writer failure misclassified: %v, renders=%d", err, renders)
	}
}
