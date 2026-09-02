package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ─── RP-9: every agent-surface description names its own required args ──────

// requiredSentence is the per-tool contract RP-9 pins: a description ends
// with (or contains) "Required: a, b." naming exactly the arguments its
// OWN inputSchema marks required, in schema order — "Required: none." when
// the schema requires nothing. Per-tool truth, never a blanket rule
// (ADR-006 as amended: concern is the engine's parameter, not every
// tool's).
var requiredSentence = regexp.MustCompile(`Required: ([a-z_, ]+|none)\.`)

// TestAgentSurfaceDescriptionsNameRequiredArgs iterates the agent-surface
// allowlist (surface.go's companionTools, 17 tools): each served
// description carries a "Reach for it when …" sentence and a "Required:"
// sentence whose names equal the schema's required list; pack_context no
// longer leaks the Go method name QuerySymbols.
func TestAgentSurfaceDescriptionsNameRequiredArgs(t *testing.T) {
	srv := newModuleTestServerInternal()
	for _, name := range AgentSurfaceTools() {
		te, ok := srv.findTool(name)
		if !ok {
			t.Errorf("%s: on the agent allowlist but not registered", name)
			continue
		}
		desc := te.desc.Description
		if !strings.Contains(desc, "Reach for it when") {
			t.Errorf("%s: description lacks a \"Reach for it when …\" sentence", name)
		}
		if strings.Contains(desc, "QuerySymbols") {
			t.Errorf("%s: description leaks the Go method name QuerySymbols", name)
		}
		want, _ := te.desc.InputSchema["required"].([]string)
		m := requiredSentence.FindStringSubmatch(desc)
		if m == nil {
			t.Errorf("%s: description lacks a \"Required: …\" sentence (schema requires %v)", name, want)
			continue
		}
		var got []string
		if m[1] != "none" {
			for _, part := range strings.Split(m[1], ",") {
				got = append(got, strings.TrimSpace(part))
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: description says Required: %v, schema requires %v", name, got, want)
		}
		for _, arg := range want {
			if _, declared := te.desc.InputSchema["properties"].(map[string]any)[arg]; !declared {
				t.Errorf("%s: schema requires %q but does not declare it", name, arg)
			}
		}
	}
}

// newModuleTestServerInternal builds a server with a nil backend — enough
// to read descriptors; handlers are never invoked here.
func newModuleTestServerInternal() *Server { return NewServer(nil) }

// ─── RP-10: annotations on every agent-surface tool ──────────────────────────

func TestToolAnnotationsCoverAgentSurface(t *testing.T) {
	writers := map[string]bool{"ingest": true, "obligations": true}
	for _, name := range AgentSurfaceTools() {
		ann := toolAnnotations(name)
		if ann == nil {
			t.Errorf("%s: no annotations", name)
			continue
		}
		for _, k := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			if _, ok := ann[k].(bool); !ok {
				t.Errorf("%s: annotation %s missing or not boolean: %v", name, k, ann)
			}
		}
		if ann["readOnlyHint"] == writers[name] {
			t.Errorf("%s: readOnlyHint = %v, want %v (writers: ingest re-indexes, obligations carries the claim-feedback write path)", name, ann["readOnlyHint"], !writers[name])
		}
		if ann["destructiveHint"] != false || ann["openWorldHint"] != false {
			t.Errorf("%s: nothing on the agent surface is destructive or open-world: %v", name, ann)
		}
	}
	if toolAnnotations("compile_layer") != nil {
		t.Error("an unclassified long-tail tool must advertise no annotations rather than a guessed set")
	}
	if got := annotatedTools(); len(got) != len(AgentSurfaceTools()) {
		t.Errorf("annotatedTools = %v", got)
	}
}

// ─── RP-12: negotiation + pinned tools/list order ───────────────────────────

func TestNegotiateProtocolVersion(t *testing.T) {
	cases := map[string]string{
		"2025-11-25": "2025-11-25", // echoed
		"2024-11-05": "2024-11-05", // echoed (Claude Code's stdio client sends this)
		"2025-06-18": "2025-06-18",
		"2026-07-28": "2025-11-25", // modern era: not supported → newest supported
		"1.0.0":      "2025-11-25",
		"":           "2025-11-25",
	}
	for req, want := range cases {
		if got := negotiateProtocolVersion(req); got != want {
			t.Errorf("negotiate(%q) = %q, want %q", req, got, want)
		}
	}
	sup := SupportedProtocolVersions()
	if !sort.SliceIsSorted(sup, func(i, j int) bool { return sup[i] > sup[j] }) || sup[0] != "2025-11-25" {
		t.Errorf("supported table must be newest first: %v", sup)
	}
	sup[0] = "mutated"
	if SupportedProtocolVersions()[0] != "2025-11-25" {
		t.Error("SupportedProtocolVersions must return a copy")
	}
}

func TestInitializeEchoesSupportedVersion(t *testing.T) {
	srv := NewServer(nil)
	for req, want := range map[string]string{"2025-06-18": "2025-06-18", "2026-07-28": "2025-11-25"} {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": req, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "0"}}})
		resp, ok := srv.HandleRequest(context.Background(), raw)
		if !ok {
			t.Fatal("no response to initialize")
		}
		var m struct {
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resp, &m); err != nil || m.Result.ProtocolVersion != want {
			t.Errorf("initialize(%s) → %q (err %v), want %q", req, m.Result.ProtocolVersion, err, want)
		}
	}
}

func TestToolsListOrderPinnedAndAnnotated(t *testing.T) {
	srv := NewServer(nil)
	ctx := WithRequestScope(context.Background(), SurfaceAgent, true)
	entries := srv.toolsListEntries(ctx)
	var names []string
	for _, e := range entries {
		m := e.(map[string]any)
		names = append(names, m["name"].(string))
		if m["annotations"] == nil || m["outputSchema"] == nil {
			t.Errorf("%s: agent-surface entry lacks annotations/outputSchema", m["name"])
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("tools/list order is not ascending by name: %v", names)
	}
	if strings.Join(names, ",") != strings.Join(AgentSurfaceTools(), ",") {
		t.Errorf("agent tools/list = %v, want the pinned allowlist %v", names, AgentSurfaceTools())
	}
	// Byte-stable across calls (the per-session schema budget is a number,
	// not a range).
	if a, b := srv.toolsListBytes(ctx), srv.toolsListBytes(ctx); a != b || a == 0 {
		t.Errorf("toolsListBytes not stable: %d vs %d", a, b)
	}
	// The schema budget, honestly: what the surface costs with and without
	// the RP-10/RP-11 additive fields (go test -v prints it).
	stripped := make([]any, len(entries))
	for i, e := range entries {
		m := map[string]any{}
		for k, v := range e.(map[string]any) {
			if k != "annotations" && k != "outputSchema" {
				m[k] = v
			}
		}
		stripped[i] = m
	}
	before, _ := json.Marshal(map[string]any{"tools": stripped})
	t.Logf("agent-surface tools/list: %d bytes without annotations/outputSchema, %d bytes with", len(before), srv.toolsListBytes(ctx))
}

// ─── RP-11: outputSchema is generated and every live result conforms ─────────

func TestOutputSchemaGeneration(t *testing.T) {
	s := outputSchemaFor("find_callers")
	if s["type"] != "object" {
		t.Fatalf("root must be an object: %v", s)
	}
	props := s["properties"].(map[string]any)
	if props["callers"].(map[string]any)["type"] != "array" || props["by_depth"].(map[string]any)["type"] != "array" {
		t.Errorf("find_callers schema properties = %v", props)
	}
	req := s["required"].([]string)
	if strings.Join(req, ",") != "by_depth,callers" {
		t.Errorf("required = %v (omitempty fields must be optional, required sorted)", req)
	}
	items := props["callers"].(map[string]any)["items"].(map[string]any)
	if items["properties"].(map[string]any)["file_path"].(map[string]any)["type"] != "string" {
		t.Errorf("symbol node item schema = %v", items)
	}
	if outputSchemaFor("compile_layer") != nil {
		t.Error("an undeclared long-tail tool must advertise no outputSchema")
	}
	// memory_search: embedded memory.Evaluation fields are inlined.
	ms := outputSchemaFor("memory_search")["properties"].(map[string]any)["memories"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	if _, ok := ms["claim"]; !ok {
		t.Errorf("embedded struct fields not inlined: %v", ms)
	}
	// Validator sanity.
	if msg := validateAgainstSchema("$", s, map[string]any{"callers": []any{}, "by_depth": []any{}}); msg != "" {
		t.Errorf("conforming value rejected: %s", msg)
	}
	if msg := validateAgainstSchema("$", s, map[string]any{"callers": "nope"}); msg == "" {
		t.Error("non-conforming value accepted")
	}
}
