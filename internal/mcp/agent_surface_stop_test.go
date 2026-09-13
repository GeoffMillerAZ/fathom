// Package mcp_test — SB-4 (docs/plans/strengths-bench.md; O31/O32): every
// agent-surface tool whose result can be definitive says so, and what a
// follow-up call would add instead.
package mcp_test

import (
	"strings"
	"testing"
)

// definitiveAgentSurfaceTools is the SB-4 allowlist: the five agent-surface
// tools whose result, in the common case, answers the question completely
// (H1's own findings — an exact find_symbol hit, a verify_plan verdict, the
// obligations governing a file right now, find_callers' resolved edges,
// symbol_context's neighbourhood). Each must say so in its description.
var definitiveAgentSurfaceTools = []string{
	"find_symbol",
	"verify_plan",
	"obligations",
	"find_callers",
	"symbol_context",
}

func TestAgentSurfaceDescriptionsStateWhenToStop(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3000, Method: "tools/list", Params: map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	descs := map[string]string{}
	for _, raw := range resp["result"].(map[string]any)["tools"].([]any) {
		e := raw.(map[string]any)
		descs[e["name"].(string)], _ = e["description"].(string)
	}

	for _, name := range definitiveAgentSurfaceTools {
		desc, ok := descs[name]
		if !ok {
			t.Errorf("%s: not found in tools/list", name)
			continue
		}
		hasStopSentence := strings.Contains(desc, "This result is the answer") ||
			strings.Contains(desc, "This result IS the answer") ||
			strings.Contains(desc, "The result is the answer") ||
			strings.Contains(desc, "The result IS the answer")
		if !hasStopSentence {
			t.Errorf("%s: description does not say the result IS the answer: %q", name, desc)
		}
		if !strings.Contains(desc, "follow-up") {
			t.Errorf("%s: description does not say what a follow-up call would add: %q", name, desc)
		}
	}
}
