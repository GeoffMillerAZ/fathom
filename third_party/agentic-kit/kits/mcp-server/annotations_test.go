package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestAnnotationsFromEffect(t *testing.T) {
	tests := []struct {
		name string
		eff  *toolregistry.Effect
		want *toolAnnotations
	}{
		{name: "undeclared emits no annotations", eff: nil, want: nil},
		{
			name: "read-only",
			eff:  &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
			want: &toolAnnotations{ReadOnlyHint: true, DestructiveHint: false, IdempotentHint: true, OpenWorldHint: false},
		},
		{
			name: "destructive open-world",
			eff:  &toolregistry.Effect{DestructiveHint: true, OpenWorldHint: true},
			want: &toolAnnotations{ReadOnlyHint: false, DestructiveHint: true, IdempotentHint: false, OpenWorldHint: true},
		},
		{
			// The explicit all-false declaration must reach the wire as four
			// explicit falses: an omitted destructiveHint would DEFAULT to
			// true on the client per the MCP schema, silently inverting the
			// declaration.
			name: "additive all-false is emitted explicitly",
			eff:  &toolregistry.Effect{},
			want: &toolAnnotations{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := annotationsFromEffect(tc.eff); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("annotationsFromEffect = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestToolsListAnnotationsGolden pins the wire shape: a tool with a declared
// Effect carries all four MCP annotation fields verbatim; a tool without one
// carries no annotations key at all (the pre-Effect descriptor, byte for
// byte — Defs() is unchanged and so is the list for undeclared tools).
func TestToolsListAnnotationsGolden(t *testing.T) {
	reg := toolregistry.New(toolregistry.Options{})
	reg.MustRegister(toolregistry.Tool{
		Def:     llmclient.ToolDef{Name: "delete_item", Description: "deletes an item", InputSchema: toolregistry.ObjectSchema()},
		Handler: func(context.Context, json.RawMessage) (string, error) { return "deleted", nil },
		Effect:  &toolregistry.Effect{DestructiveHint: true},
	})
	reg.MustRegister(toolregistry.Tool{
		Def:     llmclient.ToolDef{Name: "read_item", Description: "reads an item", InputSchema: toolregistry.ObjectSchema()},
		Handler: func(context.Context, json.RawMessage) (string, error) { return "item", nil },
		Effect:  &toolregistry.Effect{ReadOnlyHint: true, IdempotentHint: true},
	})
	reg.MustRegister(toolregistry.Tool{
		Def:     llmclient.ToolDef{Name: "undeclared", Description: "declares nothing", InputSchema: toolregistry.ObjectSchema()},
		Handler: func(context.Context, json.RawMessage) (string, error) { return "", nil },
	})
	s := New(Config{Name: "demo-tools", Version: "1.2.3"}, reg, fixedClock(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test-client","version":"0.0.1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}, "\n") + "\n"

	want := []string{
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"demo-tools","version":"1.2.3"}}}`,
		`{"jsonrpc":"2.0","id":2,"result":{"tools":[` +
			`{"name":"delete_item","description":"deletes an item","inputSchema":` + failSchemaJSON + `,"annotations":{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":false}},` +
			`{"name":"read_item","description":"reads an item","inputSchema":` + failSchemaJSON + `,"annotations":{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}},` +
			`{"name":"undeclared","description":"declares nothing","inputSchema":` + failSchemaJSON + `}]}}`,
	}
	assertLines(t, runServe(t, s, input), want)
}
