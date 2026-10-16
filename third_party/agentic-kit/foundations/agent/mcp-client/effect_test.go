package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
)

func TestEffectFromAnnotations(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want *toolregistry.Effect
	}{
		{name: "absent → undeclared", raw: "", want: nil},
		{name: "null → undeclared", raw: "null", want: nil},
		{name: "malformed → undeclared, never permissive", raw: `{"readOnlyHint": "yes"`, want: nil},
		{name: "wrong type → undeclared, never permissive", raw: `{"readOnlyHint": "yes"}`, want: nil},
		{
			// MCP defaults for absent hints are the strict reading.
			name: "empty object takes the MCP defaults: destructive, open world",
			raw:  `{}`,
			want: &toolregistry.Effect{ReadOnlyHint: false, DestructiveHint: true, IdempotentHint: false, OpenWorldHint: true},
		},
		{
			name: "read-only, other hints defaulted",
			raw:  `{"readOnlyHint": true}`,
			want: &toolregistry.Effect{ReadOnlyHint: true, DestructiveHint: true, IdempotentHint: false, OpenWorldHint: true},
		},
		{
			name: "explicit non-destructive additive",
			raw:  `{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}`,
			want: &toolregistry.Effect{ReadOnlyHint: false, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false},
		},
		{
			name: "title and unknown keys are ignored",
			raw:  `{"title": "Delete", "destructiveHint": true, "futureHint": 1}`,
			want: &toolregistry.Effect{ReadOnlyHint: false, DestructiveHint: true, IdempotentHint: false, OpenWorldHint: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := effectFromAnnotations(json.RawMessage(tc.raw))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("effectFromAnnotations(%s) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// annotatedTools is a listing where one tool declares a read-only effect and
// one declares nothing.
var annotatedTools = []toolInfoJSON{
	{
		Name:        "read",
		Description: "reads",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Annotations: json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false}`),
	},
	{
		Name:        "mystery",
		Description: "declares nothing",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	},
}

func TestListToolsMarksAnnotationsUntrustedByDefault(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", annotatedTools)
	defer ps.close()

	go ps.respond(toolsListResp(3, annotatedTools))
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	byName := map[string]ToolInfo{}
	for _, ti := range tools {
		byName[ti.Name] = ti
	}
	read := byName["read"]
	if read.Effect == nil || !read.Effect.ReadOnlyHint || read.Effect.DestructiveHint {
		t.Errorf("read.Effect = %+v, want read-only non-destructive", read.Effect)
	}
	if read.EffectTrusted {
		t.Error("read.EffectTrusted = true without Options.TrustedServer; annotations must default to untrusted")
	}
	if mystery := byName["mystery"]; mystery.Effect != nil || mystery.EffectTrusted {
		t.Errorf("mystery = %+v, want no effect and untrusted", mystery)
	}
}

func TestListToolsTrustedServerMarksAnnotationsTrusted(t *testing.T) {
	ps, r, w := newPipeServer()
	defer ps.close()
	go func() {
		ps.respond(initResp(1, "2025-11-25"))
		ps.skip()
		ps.respond(toolsListResp(2, annotatedTools))
	}()
	c, err := New(context.Background(), NewIOTransport(r, w, 0), Options{TrustedServer: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	go ps.respond(toolsListResp(3, annotatedTools))
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, ti := range tools {
		if !ti.EffectTrusted {
			t.Errorf("%s.EffectTrusted = false, want true under Options.TrustedServer", ti.Name)
		}
	}
}

// TestAnnotationDriftIsPinned proves the pin covers annotations: a server
// that keeps a tool's description and schema but flips its effect (say,
// destructiveHint true → false, to slip past a strict harness) trips
// ToolChangedError exactly like a description edit does.
func TestAnnotationDriftIsPinned(t *testing.T) {
	c, ps := newTestClient(t, "2025-11-25", annotatedTools)
	defer ps.close()

	drifted := []toolInfoJSON{
		{
			Name:        "read",
			Description: "reads",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Annotations: json.RawMessage(`{"readOnlyHint":false,"destructiveHint":true}`),
		},
		annotatedTools[1],
	}
	go ps.respond(toolsListResp(3, drifted))
	_, err := c.ListTools(context.Background())
	var changed *ToolChangedError
	if !errors.As(err, &changed) {
		t.Fatalf("ListTools error = %v, want *ToolChangedError on annotation drift", err)
	}
	if changed.Name != "read" {
		t.Errorf("ToolChangedError.Name = %q, want %q", changed.Name, "read")
	}
}

func TestToolHashAnnotationsExtendNotBreak(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	// No annotations: the hash is the pre-annotation hash — existing pins
	// and cassettes stay valid.
	if toolHash("echo", "echo text", schema, nil) != toolHash("echo", "echo text", schema, json.RawMessage("")) {
		t.Error("nil and empty annotations must hash identically")
	}
	// Annotations change the hash.
	if toolHash("echo", "echo text", schema, nil) == toolHash("echo", "echo text", schema, json.RawMessage(`{"readOnlyHint":true}`)) {
		t.Error("annotations must be part of the pinned hash")
	}
}

func TestBridgeAttachesEffectOnlyWhenTrusted(t *testing.T) {
	t.Run("untrusted server registers undeclared", func(t *testing.T) {
		c, ps := newTestClient(t, "2025-11-25", annotatedTools)
		defer ps.close()
		go ps.respond(toolsListResp(3, annotatedTools))
		reg := toolregistry.New(toolregistry.Options{})
		if err := RegisterAll(reg, c, BridgeOptions{}); err != nil {
			t.Fatalf("RegisterAll: %v", err)
		}
		if eff, ok := reg.EffectOf("read"); ok {
			t.Errorf("untrusted server's annotation reached the registry: %+v", eff)
		}
	})

	t.Run("trusted server registers the declared effect", func(t *testing.T) {
		ps, r, w := newPipeServer()
		defer ps.close()
		go func() {
			ps.respond(initResp(1, "2025-11-25"))
			ps.skip()
			ps.respond(toolsListResp(2, annotatedTools))
			ps.respond(toolsListResp(3, annotatedTools))
		}()
		c, err := New(context.Background(), NewIOTransport(r, w, 0), Options{TrustedServer: true})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		reg := toolregistry.New(toolregistry.Options{})
		if err := RegisterAll(reg, c, BridgeOptions{}); err != nil {
			t.Fatalf("RegisterAll: %v", err)
		}
		eff, ok := reg.EffectOf("read")
		if !ok || !eff.ReadOnlyHint || eff.DestructiveHint {
			t.Errorf("read effect = %+v (ok=%v), want trusted read-only declaration", eff, ok)
		}
		if _, ok := reg.EffectOf("mystery"); ok {
			t.Error("a tool the server did not annotate must stay undeclared even when trusted")
		}
	})

	t.Run("EffectMap is the operator's word and wins over the server", func(t *testing.T) {
		c, ps := newTestClient(t, "2025-11-25", annotatedTools)
		defer ps.close()
		go ps.respond(toolsListResp(3, annotatedTools))
		reg := toolregistry.New(toolregistry.Options{})
		err := RegisterAll(reg, c, BridgeOptions{EffectMap: map[string]toolregistry.Effect{
			"mystery": {ReadOnlyHint: true},
			"read":    {DestructiveHint: true},
		}})
		if err != nil {
			t.Fatalf("RegisterAll: %v", err)
		}
		if eff, ok := reg.EffectOf("mystery"); !ok || !eff.ReadOnlyHint {
			t.Errorf("mystery effect = %+v (ok=%v), want the operator's read-only declaration", eff, ok)
		}
		if eff, ok := reg.EffectOf("read"); !ok || !eff.DestructiveHint {
			t.Errorf("read effect = %+v (ok=%v), want the operator's destructive override", eff, ok)
		}
	})
}
