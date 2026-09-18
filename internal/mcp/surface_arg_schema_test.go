// surface_arg_schema_test.go — fl-wave-10 (docs/design/tool-descriptions.md
// grammar, wave-10 brief): pins argument schema STRUCTURE (property names,
// types, enums, items, required lists) for every agent-surface tool against
// a golden captured before the wave-10 description diet. Description TEXT
// is deliberately excluded from the comparison — shortening a description
// or an argument's "description" string to fit the wave-10 byte caps is the
// point of the diet, never a schema change. A real rename, a dropped
// argument, a changed type, or an added/removed enum value still fails this
// test, exactly as it did before the diet.
package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const agentSurfaceArgSchemaGolden = "testdata/agent_surface_arg_schema.golden.json"

// stripDescriptions returns a deep copy of v with every "description" key
// removed from any map along the way — the only thing wave-10 is allowed
// to change.
func stripDescriptions(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if k == "description" {
				continue
			}
			out[k] = stripDescriptions(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = stripDescriptions(val)
		}
		return out
	default:
		return v
	}
}

// canonicalSchemaJSON marshals v with map keys sorted (encoding/json's own
// default for map[string]any) so two structurally-equal schemas produce
// byte-identical output regardless of field order.
func canonicalSchemaJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(stripDescriptions(v), "", "  ")
	if err != nil {
		t.Fatalf("canonicalSchemaJSON: %v", err)
	}
	return string(b)
}

// TestAgentSurfaceArgSchemaUnchanged is the invariant-3 pin (fl-wave-10):
// diffs each agent-surface tool's live InputSchema, descriptions stripped,
// against the golden captured before the description diet.
func TestAgentSurfaceArgSchemaUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(agentSurfaceArgSchemaGolden))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden map[string]any
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("parse golden: %v", err)
	}

	s := NewServer(nil)
	names := AgentSurfaceTools()
	sort.Strings(names)

	goldenNames := make([]string, 0, len(golden))
	for n := range golden {
		goldenNames = append(goldenNames, n)
	}
	sort.Strings(goldenNames)
	if len(goldenNames) != len(names) {
		t.Fatalf("golden carries %d tools, agent surface carries %d — regenerate the golden alongside a real surface membership change (names: golden=%v live=%v)", len(goldenNames), len(names), goldenNames, names)
	}

	for _, name := range names {
		gotEntry, ok := s.findTool(name)
		if !ok {
			t.Errorf("agent surface tool %q not found in the registry", name)
			continue
		}
		wantSchema, ok := golden[name]
		if !ok {
			t.Errorf("golden has no entry for agent-surface tool %q — regenerate the golden", name)
			continue
		}
		want := canonicalSchemaJSON(t, wantSchema)
		got := canonicalSchemaJSON(t, gotEntry.desc.InputSchema)
		if want != got {
			t.Errorf("tool %q: argument schema structure changed (names/types/enums/required must stay identical across the description diet)\n--- golden ---\n%s\n--- live ---\n%s", name, want, got)
		}
	}
}
