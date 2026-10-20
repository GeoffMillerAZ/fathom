package mcp_test

// O1452: the root call (obligations with no file) serves the uncompiled root
// guide's own sections ahead of compiled rules from non-guide documents, and
// its summary names the cited document's kind.

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type rootRawFakeBackend struct {
	semanticFakeBackend
	rows  []semantic.Obligation
	calls int
	max   int
}

func (r *rootRawFakeBackend) RootRawFallbackObligations(_ context.Context, _ string, max int) ([]semantic.Obligation, string, bool, error) {
	r.calls++
	r.max = max
	return r.rows, "compiled layer incomplete: root guide uncompiled; its current sections served raw", len(r.rows) > 0, nil
}

func TestRootObligationsFallBackToTheUncompiledGuide(t *testing.T) {
	design := semantic.Obligation{Rule: "Drawer adapters own focus.", Src: "docs/design/profile-drawer-adapter.md#L3-L5", Tier: "doc-claimed", Layer: "obligations"}
	why := "compiled layer incomplete: root guide uncompiled; its current sections served raw"
	fake := &rootRawFakeBackend{
		semanticFakeBackend: semanticFakeBackend{obligations: semantic.Obligations{Service: "svc", Rules: []semantic.Obligation{design}}},
		rows:                []semantic.Obligation{{Rule: "App-kit agent guidance", Src: "AGENTS.md#L1-L9", Tier: "raw", Layer: "obligations", Why: why}},
	}
	call := func(b mcp.QueryInterface) map[string]any {
		resp := roundtrip(t, mcp.NewServer(b), request{JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc"}}})
		if resp["error"] != nil {
			t.Fatalf("error: %v", resp["error"])
		}
		return extractPayload(t, resp)
	}

	p := call(fake)
	rules, _ := p["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("rules = %v, want raw root-guide row then the compiled design rule", p["rules"])
	}
	first, _ := rules[0].(map[string]any)
	if first["tier"] != "raw" || !strings.HasPrefix(first["src"].(string), "AGENTS.md#") {
		t.Errorf("rules[0] = %v, want the raw AGENTS.md row ranked first", first)
	}
	if sum, _ := p["summary"].(string); !strings.Contains(sum, "root-scope governing rule") || !strings.Contains(sum, "AGENTS.md") {
		t.Errorf("summary = %q, want root guidance cited from AGENTS.md", sum)
	}
	if fake.calls != 1 || fake.max != semantic.RawFallbackReserve {
		t.Errorf("root fallback calls=%d max=%d, want 1 call within the reserve", fake.calls, fake.max)
	}

	// Guide fully compiled: no raw rows, and a design-document rule is no
	// longer called root-scope guidance.
	fake.rows = nil
	p = call(fake)
	sum, _ := p["summary"].(string)
	if strings.Contains(sum, "root-scope governing rule") || !strings.Contains(sum, "design document rule") {
		t.Errorf("summary = %q, want the design document named as such", sum)
	}
}
