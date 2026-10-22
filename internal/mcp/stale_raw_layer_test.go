package mcp_test

// stale_raw_layer_test.go — O1125 (I38, ADR-037, O1105): a rule withheld
// because the section its layer compiled it from changed is replaced by
// that section's current text, whichever layer compiled it. The raw
// fallback takes the layers whose rules were withheld and ranks each one's
// stale sections from its own corpus, within RawFallbackReserve.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// The receipt hands the raw fallback the layer whose rule it withheld as
// stale, and serves what the fallback ranks for that layer.
func TestObligationsTool_WithheldStaleLayerReachesRawFallback(t *testing.T) {
	fake := &perLayerCovFakeBackend{
		rawFallbackFakeBackend: rawFallbackFakeBackend{
			semanticFakeBackend: semanticFakeBackend{
				obligations: semantic.Obligations{Service: "svc", File: "go.mod", Rules: []semantic.Obligation{
					{Rule: "Design rule.", Src: "docs/design/x.md#L4-L6", Tier: "doc-claimed", Layer: "adr-rules", Subject: "docs/design/x.md#sec"},
					{Rule: "Fresh rule.", Src: "AGENTS.md#L5-L6", Tier: "doc-claimed", Layer: "obligations", Subject: "AGENTS.md#gate"},
				}},
			},
			rawByLayer: map[string][]semantic.Obligation{
				"adr-rules": {{Rule: "Sec", Src: "docs/design/x.md#L4-L7", Tier: "raw", Layer: "adr-rules", Why: "stale section withheld"}},
			},
		},
		covs: map[string]semantic.LayerCoverage{
			semantic.LayerObligations: freshObligationsCov(),
			"adr-rules":               {SectionsTotal: 2, SectionsCompiled: 1, UncompiledReasons: map[string]string{"docs/design/x.md#sec": core.UncompiledStale}},
		},
	}
	payload := extractPayload(t, obligationsCall(t, fake))
	if len(fake.gotRawLayers) != 1 || fake.gotRawLayers[0] != "adr-rules" {
		t.Fatalf("raw fallback layers = %v, want [adr-rules] (the layer whose rule was withheld)", fake.gotRawLayers)
	}
	if got := servedRuleTexts(payload); len(got) != 2 || got[0] != "Fresh rule." || got[1] != "Sec" {
		t.Errorf("served %v, want the fresh rule and the stale section's raw row", got)
	}
	if n, _ := staleWithheld(payload).(float64); n != 1 {
		t.Errorf("layer_coverage.stale_withheld = %v, want 1", staleWithheld(payload))
	}
}

// End to end through pkg/client: an adr-rules claim compiled from a
// docs/design section that governs pkg/foo.go is withheld once the section
// changes, and the section's current text is served raw in its place.
func TestObligationsTool_StaleDesignSectionServedRawAfterWithhold(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	design := filepath.Join(dir, "docs", "design", "x.md")
	writeTestFile(t, design, "# Boundary\n\n- pkg/** never imports cmd/**.\n")
	writeTestFile(t, filepath.Join(dir, "CLAUDE.md"), "# Rules\n\n- Run tests before committing.\n")
	writeTestFile(t, filepath.Join(dir, "pkg", "foo.go"), "package pkg\nfunc Foo() {}\n")

	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	c.WithLLM(llmclient.NewMockClient(
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"pkg never imports cmd.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`}},
		llmclient.MockEntry{Response: llmclient.Response{Text: `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/*.go"],"cite_start":3,"cite_end":3}]}`}},
	), "mock-model", 0, 0)
	for _, layer := range []string{"adr-rules", semantic.LayerObligations} {
		if run, err := c.CompileLayer(ctx, "svc", layer, false); err != nil || run.SectionsCompiled == 0 {
			t.Fatalf("compile %s: run=%+v err=%v", layer, run, err)
		}
	}
	call := func() map[string]any {
		return extractPayload(t, roundtrip(t, mcp.NewServer(c), request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": "pkg/foo.go", "include_snippets": false}},
		}))
	}
	if got := servedRuleTexts(call()); strings.Join(got, "|") != "Run tests before committing.|pkg never imports cmd." {
		t.Fatalf("fresh: served %v, want both compiled rules", got)
	}

	writeTestFile(t, design, "# Boundary\n\n- pkg/** must allow imports from cmd/** now.\n")
	payload := call()
	if n, _ := staleWithheld(payload).(float64); n != 1 {
		t.Fatalf("layer_coverage.stale_withheld = %v, want 1 (the adr-rules claim)", staleWithheld(payload))
	}
	rules, _ := payload["rules"].([]any)
	var raw map[string]any
	for _, r := range rules {
		m, _ := r.(map[string]any)
		if m["rule"] == "pkg never imports cmd." {
			t.Errorf("the stale adr-rules claim is still served: %v", m)
		}
		if src, _ := m["src"].(string); strings.HasPrefix(src, "docs/design/x.md") {
			raw = m
		}
	}
	if raw == nil || raw["tier"] != "raw" || raw["layer"] != "adr-rules" {
		t.Fatalf("no raw adr-rules row for the changed docs/design section: %v", rules)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
