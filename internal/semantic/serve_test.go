package semantic

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func seedServed(t *testing.T, st store.Store, id, rule string, scope []string, status core.ClaimStatus) {
	t.Helper()
	seedServedLayer(t, st, id, LayerObligations, rule, scope, status)
}

// seedServedLayer is seedServed with an explicit layer — T5-2/O14's eviction
// test needs claims blended from multiple layers (obligations, adr-rules, …)
// to prove each served rule carries the layer it actually came from.
func seedServedLayer(t *testing.T, st store.Store, id, layer, rule string, scope []string, status core.ClaimStatus) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: layer, LayerVersion: "v1", Kind: KindObligation,
		Subject: "docs/g.md#s-" + id, Rule: rule, Scope: scope,
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: "docs/g.md#L1-L2", TargetHash: "h"}}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestObligationsFor_ScopedOrderedFlagged(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	seedServed(t, st, "c-broad", "Sort all outputs.", []string{"**"}, core.ClaimFresh)
	seedServed(t, st, "c-narrow", "Inject clocks in query logic.", []string{"internal/query/**"}, core.ClaimFresh)
	seedServed(t, st, "c-stale", "Old rule that drifted.", []string{"internal/query/**"}, core.ClaimStale)
	seedServed(t, st, "c-other", "Rule for the mcp package.", []string{"internal/mcp/**"}, core.ClaimFresh)
	seedServed(t, st, "c-dead", "Refuted rule.", []string{"**"}, core.ClaimRefuted)

	got, err := ObligationsFor(ctx, st, "svc", "internal/query/engine.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 3 {
		t.Fatalf("got %d rules, want 3 (narrow, stale-narrow, broad; refuted+mcp excluded): %+v", len(got.Rules), got.Rules)
	}
	// BP-17 (ADR-032 §4 D5): FRESH first, then specificity. The stale
	// narrow rule therefore ranks BELOW the fresh broad one — before this
	// change it ranked above it, and both T02 payloads in the stopped bench
	// run led with a stale rule as a result.
	if got.Rules[0].Rule != "Inject clocks in query logic." || got.Rules[1].Rule != "Sort all outputs." {
		t.Errorf("order wrong: %+v", got.Rules)
	}
	if got.Rules[2].Rule != "Old rule that drifted." {
		t.Errorf("stale rule not last: %+v", got.Rules)
	}
	// Drifted rule is served FLAGGED, never dropped.
	if got.Rules[2].Status != string(core.ClaimStale) {
		t.Errorf("stale rule status = %q, want flagged stale", got.Rules[2].Status)
	}
	if got.Rules[0].Status != "" {
		t.Errorf("fresh rule must omit status, got %q", got.Rules[0].Status)
	}
	if got.Rules[0].Src == "" || got.Rules[0].Tier != string(core.TierDocClaimed) {
		t.Errorf("citation/tier missing: %+v", got.Rules[0])
	}
	if got.Watermark != "manifest:aaa" {
		t.Errorf("watermark = %q", got.Watermark)
	}
}

func TestObligationsFor_ZeroOutputOnClean(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedServed(t, st, "c-mcp", "Rule for the mcp package.", []string{"internal/mcp/**"}, core.ClaimFresh)

	got, err := ObligationsFor(ctx, st, "svc", "cmd/fathom/main.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 0 {
		t.Fatalf("clean file got %d rules", len(got.Rules))
	}
	// The JSON a hook would emit must carry no rules key at all.
	b, _ := json.Marshal(got)
	if string(b) != `{"service":"svc","file":"cmd/fathom/main.go"}` {
		t.Errorf("clean payload = %s", b)
	}
}

func TestObligationsFor_CapsAtMax(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	rules := []string{"A.", "B.", "C.", "D.", "E.", "F.", "G.", "H."}
	for i, r := range rules {
		seedServed(t, st, "c-"+r, r, []string{"**"}, core.ClaimFresh)
		_ = i
	}
	got, err := ObligationsFor(ctx, st, "svc", "any.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != MaxObligations {
		t.Errorf("got %d rules, want cap %d", len(got.Rules), MaxObligations)
	}
	// T5-2/O14: 8 eligible, cap 6 → 2 displaced.
	if got.Displaced != 2 {
		t.Errorf("Displaced = %d, want 2 (8 eligible - cap 6)", got.Displaced)
	}
}

// TestObligationsFor_DisplacedCountAndLayerLabels is T5-2/O14's eviction
// test: a recompile that blends a new layer (adr-rules) into the served set
// can silently push an older obligations-layer rule out past the cap — the
// eviction itself (dropping the LOWEST-ranked matched claims) is unchanged
// behavior; what T5-2 adds is making it VISIBLE — displaced:N plus a layer
// label on every surviving rule, so an agent (or a human debugging "where
// did my rule go") can tell which layer served what, and how many
// eligible-but-unserved claims the cap silently cut this call.
func TestObligationsFor_DisplacedCountAndLayerLabels(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// 7 served-eligible claims, same broad scope, blended from two layers —
	// exactly O14's reproduction shape (adr-rules joining obligations).
	seedServedLayer(t, st, "c-o1", LayerObligations, "Obligation rule 1.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-o2", LayerObligations, "Obligation rule 2.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-o3", LayerObligations, "Obligation rule 3.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-a1", "adr-rules", "ADR rule 1.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-a2", "adr-rules", "ADR rule 2.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-a3", "adr-rules", "ADR rule 3.", []string{"**"}, core.ClaimFresh)
	seedServedLayer(t, st, "c-a4", "adr-rules", "ADR rule 4.", []string{"**"}, core.ClaimFresh)

	got, err := ObligationsFor(ctx, st, "svc", "any.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != MaxObligations {
		t.Fatalf("got %d rules, want cap %d", len(got.Rules), MaxObligations)
	}
	if got.Displaced != 1 {
		t.Fatalf("Displaced = %d, want 1 (7 eligible - cap 6)", got.Displaced)
	}
	for _, r := range got.Rules {
		if r.Layer == "" {
			t.Errorf("rule %q has no layer label", r.Rule)
		}
		if r.Layer != LayerObligations && r.Layer != "adr-rules" {
			t.Errorf("rule %q layer = %q, want obligations or adr-rules", r.Rule, r.Layer)
		}
	}
	// The JSON payload must actually carry displaced and per-rule layer, not
	// just the Go struct — this is what crosses the MCP wire to the agent.
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"displaced":1`) {
		t.Errorf("JSON payload missing displaced:1: %s", b)
	}
	if !strings.Contains(string(b), `"layer":`) {
		t.Errorf("JSON payload missing per-rule layer label: %s", b)
	}
}

// ─── RootObligationsFor: selection by scope breadth (review finding, MAJOR:
// the old literal-probe approach matched a claim's scope glob against the
// bare string "CLAUDE.md", so a genuinely repo-wide "**/*.go" rule from the
// root CLAUDE.md never matched and was silently dropped) ────────────────────

// seedServedSubject is seedServedLayer with an explicit Subject (governing-
// source citation) — the root-scope tests need to control it directly
// (seedServedLayer always writes "docs/g.md#s-"+id).
func seedServedSubject(t *testing.T, st store.Store, id, layer, subject, rule string, scope []string, status core.ClaimStatus) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: layer, LayerVersion: "v1", Kind: KindObligation,
		Subject: subject, Rule: rule, Scope: scope,
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:root", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: subject, TargetHash: "h"}}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// TestRootObligationsFor_SourceOrGlobBreadth proves the OR the fix requires:
// a claim counts as root-scope when EITHER its source is a root-level
// governing doc OR its scope glob is unanchored/wildcard-rooted — and a
// package-scoped rule from a NESTED CLAUDE.md (floored to its own subtree
// at compile time, BP-32) counts as neither.
func TestRootObligationsFor_SourceOrGlobBreadth(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// The MAJOR finding's exact reproduction: a repo-wide code rule from
	// the root CLAUDE.md, scoped "**/*.go" — the literal probe path
	// "CLAUDE.md" never matches that glob.
	seedServedSubject(t, st, "c-root-go", LayerObligations, "CLAUDE.md#go-gate", "Run gofmt before commit.", []string{"**/*.go"}, core.ClaimFresh)
	// A nested CLAUDE.md's package-scoped rule — must be EXCLUDED: neither
	// a root-level source (the subject names a nested doc) nor a
	// wildcard-rooted glob (floored to internal/mcp/).
	seedServedSubject(t, st, "c-pkg-go", LayerObligations, "internal/mcp/CLAUDE.md#pkg-rule", "Package-scoped rule.", []string{"internal/mcp/**/*.go"}, core.ClaimFresh)
	// Root-scope by GLOB BREADTH alone — a non-root-doc source, but an
	// unanchored glob.
	seedServedSubject(t, st, "c-wild", "adr-rules", "ADR.md#wild", "Sort every output.", []string{"**"}, core.ClaimFresh)
	// Root-scope by SOURCE alone — a root CLAUDE.md rule whose proposed
	// scope is a narrow, non-wildcard glob.
	seedServedSubject(t, st, "c-root-narrow", LayerObligations, "CLAUDE.md#narrow", "Narrow root rule.", []string{"CLAUDE.md"}, core.ClaimFresh)

	got, err := RootObligationsFor(ctx, st, "svc", 0)
	if err != nil {
		t.Fatalf("RootObligationsFor: %v", err)
	}
	if got.File != "" {
		t.Errorf("File = %q, want empty — root-scope answers for no single file", got.File)
	}
	var rules []string
	for _, r := range got.Rules {
		rules = append(rules, r.Rule)
	}
	sort.Strings(rules)
	want := []string{"Narrow root rule.", "Run gofmt before commit.", "Sort every output."}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules %v, want %v (the nested package-scoped rule must be excluded)", len(rules), rules, want)
	}
	for i, w := range want {
		if rules[i] != w {
			t.Errorf("rules = %v, want %v", rules, want)
		}
	}
}

func TestGlobReachesRoot(t *testing.T) {
	cases := map[string]bool{
		"*": true, "**": true, "**/*.go": true, "**/x": true,
		"*.go": false, "internal/**/*.go": false, "internal/**": false, "": false,
	}
	for g, want := range cases {
		if got := globReachesRoot(g); got != want {
			t.Errorf("globReachesRoot(%q) = %v, want %v", g, got, want)
		}
	}
}

func TestIsRootLevelSource(t *testing.T) {
	cases := map[string]bool{
		"CLAUDE.md#gate":               true,
		"CLAUDE.md":                    true,
		"AGENTS.md#x":                  true,
		"docs/intent/README.md#router": true,
		"docs/intent/scope.md":         true,
		"internal/mcp/CLAUDE.md#pkg":   false,
		"docs/vision/28-receipts.md#x": false,
		"ADR.md#decision":              false,
	}
	for subject, want := range cases {
		if got := isRootLevelSource(subject); got != want {
			t.Errorf("isRootLevelSource(%q) = %v, want %v", subject, got, want)
		}
	}
}
