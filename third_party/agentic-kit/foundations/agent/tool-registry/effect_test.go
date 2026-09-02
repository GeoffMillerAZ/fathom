package toolregistry

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

func nopHandler(context.Context, json.RawMessage) (string, error) { return "", nil }

func TestEffectOf(t *testing.T) {
	reg := New(Options{})
	reg.MustRegister(Tool{Def: llmclient.ToolDef{Name: "undeclared"}, Handler: nopHandler})
	reg.MustRegister(Tool{
		Def:     llmclient.ToolDef{Name: "reader"},
		Handler: nopHandler,
		Effect:  &Effect{ReadOnlyHint: true},
	})
	reg.MustRegister(Tool{
		Def:     llmclient.ToolDef{Name: "deleter"},
		Handler: nopHandler,
		Effect:  &Effect{DestructiveHint: true, OpenWorldHint: true},
	})

	tests := []struct {
		name   string
		tool   string
		want   *Effect
		wantOK bool
	}{
		{name: "unknown tool", tool: "nope", want: nil, wantOK: false},
		{name: "registered without a declaration", tool: "undeclared", want: nil, wantOK: false},
		{name: "read-only declaration", tool: "reader", want: &Effect{ReadOnlyHint: true}, wantOK: true},
		{name: "destructive declaration", tool: "deleter", want: &Effect{DestructiveHint: true, OpenWorldHint: true}, wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := reg.EffectOf(tc.tool)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("effect = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestEffectOfReturnsACopy(t *testing.T) {
	declared := &Effect{ReadOnlyHint: true}
	reg := New(Options{})
	reg.MustRegister(Tool{Def: llmclient.ToolDef{Name: "reader"}, Handler: nopHandler, Effect: declared})

	got, ok := reg.EffectOf("reader")
	if !ok {
		t.Fatal("EffectOf: ok = false, want true")
	}
	got.ReadOnlyHint = false
	got.DestructiveHint = true

	again, _ := reg.EffectOf("reader")
	if !again.ReadOnlyHint || again.DestructiveHint {
		t.Errorf("mutating the returned copy changed the registered declaration: %+v", again)
	}
	if !declared.ReadOnlyHint || declared.DestructiveHint {
		t.Errorf("mutating the returned copy changed the caller's original: %+v", declared)
	}
}

func TestEffectDestructive(t *testing.T) {
	tests := []struct {
		name string
		eff  Effect
		want bool
	}{
		{name: "zero value: additive, not destructive", eff: Effect{}, want: false},
		{name: "read-only", eff: Effect{ReadOnlyHint: true}, want: false},
		{name: "destructive", eff: Effect{DestructiveHint: true}, want: true},
		{name: "destructive and idempotent", eff: Effect{DestructiveHint: true, IdempotentHint: true}, want: true},
		{name: "contradictory read-only + destructive reads fail-closed", eff: Effect{ReadOnlyHint: true, DestructiveHint: true}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.eff.Destructive(); got != tc.want {
				t.Errorf("Destructive() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEffectDoesNotChangeDefs pins the decision-record guarantee: a declared
// Effect changes dispatch governance only — the model-facing Defs (and so an
// MCP tools/list built from them) carry exactly what they carried before.
func TestEffectDoesNotChangeDefs(t *testing.T) {
	def := llmclient.ToolDef{
		Name:        "rm",
		Description: "removes a thing",
		InputSchema: ObjectSchema(Prop{Name: "id", Type: "string", Required: true}),
	}
	plain := New(Options{})
	plain.MustRegister(Tool{Def: def, Handler: nopHandler})
	declared := New(Options{})
	declared.MustRegister(Tool{Def: def, Handler: nopHandler, Effect: &Effect{DestructiveHint: true}})

	if got, want := declared.Defs(), plain.Defs(); !reflect.DeepEqual(got, want) {
		t.Errorf("Defs with an Effect = %+v, want identical to Defs without: %+v", got, want)
	}
	res := declared.Dispatch(context.Background(), llmclient.ToolUseBlock("tu-1", "rm", json.RawMessage(`{"id":"x"}`)))
	if res.IsError {
		t.Errorf("Dispatch under a permissive registry must not gate on Effect; got is_error: %s", res.Content)
	}
}
