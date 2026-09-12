package mcp_test

// obligationsnippet_test.go — BP-20 (docs/plans/bench-pyramid.md, ADR-032
// §4 D3): the obligations tool inlines each rule's cited lines, so the
// agent stops spending a read_source turn per rule it trusts (5 of the 6
// useful-rule cells in the stopped run did exactly that).

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// snippetFakeBackend is extendedFakeBackend (which serves ServiceMeta's
// corpus root, and so the bounded read behind a snippet) plus the
// obligations capability.
type snippetFakeBackend struct {
	extendedFakeBackend
	obligations semantic.Obligations
}

func (s *snippetFakeBackend) Obligations(_ context.Context, _, _ string, _ int) (semantic.Obligations, error) {
	return s.obligations, nil
}

func (s *snippetFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool { return false }

const snippetGoverningDoc = `# CLAUDE.md

## Rules

- Every extractor is registered in fusion.New and covered by a golden test.
- Determinism is a contract: sorted outputs, injected clocks.
`

func snippetServer(t *testing.T, rules []semantic.Obligation) (*mcp.Server, string) {
	t.Helper()
	root := t.TempDir()
	writeCorpusFile(t, root, "CLAUDE.md", snippetGoverningDoc)
	fake := &snippetFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root},
		obligations:         semantic.Obligations{Service: "svc", File: "internal/extractor/goextract.go", Rules: rules},
	}
	return mcp.NewServer(fake), root
}

func obligationsPayload(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
	t.Helper()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4201, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": args},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	return extractPayload(t, resp)
}

func firstRule(t *testing.T, p map[string]any) map[string]any {
	t.Helper()
	rules, ok := p["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("rules missing: %v", p)
	}
	m, ok := rules[0].(map[string]any)
	if !ok {
		t.Fatalf("rules[0] is not an object: %v", rules[0])
	}
	return m
}

func TestObligationsTool_InlinesCitedLinesByDefault(t *testing.T) {
	srv, _ := snippetServer(t, []semantic.Obligation{{
		Rule: "Register every extractor in fusion.New.", Src: "CLAUDE.md#L5-L5",
		Tier: "doc-claimed", Layer: "obligations",
	}})

	p := obligationsPayload(t, srv, map[string]any{"service": "svc", "file": "internal/extractor/goextract.go"})
	r := firstRule(t, p)
	snip, _ := r["snippet"].(string)
	if !strings.Contains(snip, "registered in fusion.New") {
		t.Fatalf("snippet = %q, want the cited line inlined", snip)
	}
	// Every pre-existing field survives the round-trip (law 4: additive).
	if r["rule"] != "Register every extractor in fusion.New." || r["src"] != "CLAUDE.md#L5-L5" || r["layer"] != "obligations" {
		t.Errorf("existing fields changed shape: %v", r)
	}
}

func TestObligationsTool_IncludeSnippetsFalseOmitsThem(t *testing.T) {
	srv, _ := snippetServer(t, []semantic.Obligation{{
		Rule: "Register every extractor in fusion.New.", Src: "CLAUDE.md#L5-L5",
		Tier: "doc-claimed", Layer: "obligations",
	}})

	p := obligationsPayload(t, srv, map[string]any{
		"service": "svc", "file": "internal/extractor/goextract.go", "include_snippets": false,
	})
	if _, present := firstRule(t, p)["snippet"]; present {
		t.Error("include_snippets:false must omit the field entirely — this is the edit hook's payload budget")
	}
}

func TestObligationsTool_UnreadableCitationOmitsTheSnippet(t *testing.T) {
	srv, _ := snippetServer(t, []semantic.Obligation{
		{Rule: "Rule with an unparseable citation.", Src: "", Tier: "doc-claimed", Layer: "obligations"},
		{Rule: "Rule citing a file that is not there.", Src: "docs/gone.md#L1-L2", Tier: "doc-claimed", Layer: "obligations"},
	})

	p := obligationsPayload(t, srv, map[string]any{"service": "svc", "file": "internal/extractor/goextract.go"})
	rules, _ := p["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("rules = %v, want 2", rules)
	}
	for i, raw := range rules {
		if _, present := raw.(map[string]any)["snippet"]; present {
			t.Errorf("rules[%d] fabricated a snippet for an unreadable citation: %v", i, raw)
		}
	}
}

func TestObligationsTool_SnippetIsCappedWithANamedCut(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("a governing sentence that goes on and on. ", 40)
	writeCorpusFile(t, root, "CLAUDE.md", long+"\n")
	fake := &snippetFakeBackend{
		extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root},
		obligations: semantic.Obligations{Service: "svc", File: "internal/x.go", Rules: []semantic.Obligation{
			{Rule: "A rule with a very long citation.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed", Layer: "obligations"},
		}},
	}
	srv := mcp.NewServer(fake)

	p := obligationsPayload(t, srv, map[string]any{"service": "svc", "file": "internal/x.go"})
	snip, _ := firstRule(t, p)["snippet"].(string)
	if len(snip) > 240 {
		t.Errorf("snippet = %d bytes, want <= 240", len(snip))
	}
	if !strings.Contains(snip, "cut at 240 bytes") {
		t.Errorf("a cut snippet must NAME the cut (law 3), got %q", snip)
	}
}
