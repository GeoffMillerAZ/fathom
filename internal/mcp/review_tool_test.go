package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── reviewFakeBackend ───────────────────────────────────────────────────────
//
// reviewFakeBackend extends extendedFakeBackend with obligationsBackend and
// a minimal in-memory claims store that mirrors the real store's
// refutation-durability upsert (internal/store's PutClaims "status = CASE
// WHEN ... = 'refuted' THEN ... ELSE excluded.status END") — enough to
// prove review_diff's WIRING (attach → read-back → label) without a real
// store.Store; the SQL-level durability guarantee itself is covered by the
// store's own contract tests (FT-4).

type reviewFakeBackend struct {
	extendedFakeBackend
	obligations map[string]semantic.Obligations // keyed by file
	claims      map[string]core.SemanticClaim   // keyed by claim ID
	putErr      error
}

func (r *reviewFakeBackend) Obligations(_ context.Context, _, file string, _ int) (semantic.Obligations, error) {
	return r.obligations[file], nil
}

func (r *reviewFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool { return false }

func (r *reviewFakeBackend) PutClaims(_ context.Context, claims []core.SemanticClaim, _ []core.ClaimLink) error {
	if r.putErr != nil {
		return r.putErr
	}
	if r.claims == nil {
		r.claims = map[string]core.SemanticClaim{}
	}
	for _, c := range claims {
		if existing, ok := r.claims[c.ClaimID]; ok && existing.Status == core.ClaimRefuted {
			c.Status = core.ClaimRefuted // mirror the store's refutation-durability upsert
		}
		r.claims[c.ClaimID] = c
	}
	return nil
}

func (r *reviewFakeBackend) ClaimsForService(_ context.Context, _, layer string) ([]core.SemanticClaim, error) {
	var out []core.SemanticClaim
	for _, c := range r.claims {
		if layer == "" || c.Layer == layer {
			out = append(out, c)
		}
	}
	return out, nil
}

var _ mcp.QueryInterface = (*reviewFakeBackend)(nil)

// ─── tests ────────────────────────────────────────────────────────────────

func TestReviewDiff_RequiredArgs(t *testing.T) {
	srv := mcp.NewServer(&reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}})

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"changed_files": []string{"pkg/a.go"}}},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error when service is omitted")
	}

	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc"}},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error when changed_files is omitted")
	}
}

func TestReviewDiff_ComposesDimensionsHonestly(t *testing.T) {
	// No svcPath (no ServiceMeta) → arch_conformance falls back to
	// checked:false/none; no obligations map wired → obligations dimension
	// checked:true with zero findings (the fake DOES implement
	// obligationsBackend, just with nothing configured for this file).
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	if p["service"] != "svc" {
		t.Errorf("service = %v", p["service"])
	}
	dims, ok := p["dimensions"].([]any)
	if !ok || len(dims) != 4 {
		t.Fatalf("expected 4 dimensions, got %v", p["dimensions"])
	}
	byName := map[string]map[string]any{}
	for _, raw := range dims {
		d := raw.(map[string]any)
		byName[d["name"].(string)] = d
	}
	for _, want := range []string{"impact", "obligations", "arch_conformance", "spec_coverage"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing dimension %q", want)
		}
	}
	if byName["impact"]["checked"] != true {
		t.Errorf("impact should be checked: %v", byName["impact"])
	}
	if byName["arch_conformance"]["checked"] != false {
		t.Errorf("arch_conformance should be unchecked (no rules configured): %v", byName["arch_conformance"])
	}
	if byName["arch_conformance"]["reason"] == "" || byName["arch_conformance"]["reason"] == nil {
		t.Error("arch_conformance should carry a reason when unchecked")
	}
	if p["summary"] == "" || p["summary"] == nil {
		t.Error("expected a non-empty summary")
	}
}

func TestReviewDiff_ObligationsScoped(t *testing.T) {
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		obligations: map[string]semantic.Obligations{
			"pkg/a.go": {
				Service: "svc", File: "pkg/a.go",
				Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed"}},
			},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var obligDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "obligations" {
			obligDim = d
		}
	}
	if obligDim == nil {
		t.Fatal("missing obligations dimension")
	}
	findings := obligDim["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected 1 obligation finding, got %d: %v", len(findings), findings)
	}
}

func TestReviewDiff_ArchViolationViaProjectRules(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "pkg", to_pkg: "pkg", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var archDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "arch_conformance" {
			archDim = d
		}
	}
	if archDim == nil {
		t.Fatal("missing arch_conformance dimension")
	}
	if archDim["checked"] != true {
		t.Fatalf("expected checked:true with a loaded project rule pack, got %v (reason=%v)", archDim["checked"], archDim["reason"])
	}
	count, _ := archDim["count"].(float64)
	if count == 0 {
		t.Fatalf("expected at least one violation, got count=%v findings=%v", archDim["count"], archDim["findings"])
	}
}

// TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview is the C2-1
// "prove it with a test" scenario at the MCP-wiring layer: emit finding →
// simulate refute (as fathom claim refute / ClaimFeedback would persist it)
// → re-run review_diff → the finding carries status "refuted" and is
// excluded from its dimension's active count, but remains listed.
func TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "api", to_pkg: "store", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	// A dedicated, minimal fixture with EXACTLY one arch-violating edge
	// (api.Handler → store.Query) so the round trip below can assert on a
	// single finding without incidental violations from other edges.
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)

	call := func(id int) map[string]any {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: id, Method: "tools/call",
			Params: map[string]any{
				"name":      "review_diff",
				"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
			},
		})
		return extToolPayload(t, resp)
	}
	archDimOf := func(p map[string]any) map[string]any {
		for _, raw := range p["dimensions"].([]any) {
			d := raw.(map[string]any)
			if d["name"] == "arch_conformance" {
				return d
			}
		}
		t.Fatal("missing arch_conformance dimension")
		return nil
	}

	// Run 1: emit + persist the finding.
	p1 := call(10)
	if p1["findings_persisted"] != true {
		t.Fatalf("expected findings_persisted:true, got %v", p1["findings_persisted"])
	}
	d1 := archDimOf(p1)
	f1 := d1["findings"].([]any)[0].(map[string]any)
	claimID, _ := f1["claim_id"].(string)
	if claimID == "" {
		t.Fatal("expected a non-empty claim_id after persistence")
	}
	if _, hasStatus := f1["status"]; hasStatus {
		t.Errorf("a fresh finding should carry no status field, got %v", f1["status"])
	}
	if d1["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", d1["count"])
	}

	// Simulate an external refute (fathom claim refute / ClaimFeedback
	// persisting a refuted status for this exact ClaimID).
	c := fake.claims[claimID]
	c.Status = core.ClaimRefuted
	fake.claims[claimID] = c

	// Run 2: the identical finding must re-derive the SAME ClaimID and now
	// carry status "refuted", excluded from count but still listed.
	p2 := call(11)
	d2 := archDimOf(p2)
	findings2 := d2["findings"].([]any)
	if len(findings2) != 1 {
		t.Fatalf("refuted finding should remain listed, got %d findings", len(findings2))
	}
	f2 := findings2[0].(map[string]any)
	if f2["claim_id"] != claimID {
		t.Fatalf("ClaimID changed across re-review: %v vs %v", claimID, f2["claim_id"])
	}
	if f2["status"] != "refuted" {
		t.Errorf("status = %v, want %q", f2["status"], "refuted")
	}
	if d2["count"].(float64) != 0 {
		t.Errorf("count = %v, want 0 (the only finding is refuted)", d2["count"])
	}
}
