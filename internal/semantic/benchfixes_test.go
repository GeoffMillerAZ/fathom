package semantic

// benchfixes_test.go — BP-17/18/19 (docs/plans/bench-pyramid.md, ADR-032
// §4): the three serve-time fixes the stopped replicated bench run found,
// each pinned against O23's actual shape.
//
// O23, in one sentence: asked what governs internal/extractor/goextract.go,
// the obligations tool served six plan-row spec-bindings about C++, CUE and
// tree-sitter, displaced 65 more, and never served the internal/CLAUDE.md
// extractor-wiring rule the file is actually governed by. The fixture below
// reproduces exactly that — a governs-role rule scoped to the file, plus
// claims-role plan rows whose PROSE names the file's directory while their
// CONTRACT names another package entirely.

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func bpFixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "bp19fixture")
}

// seedRoleClaim seeds one served claim on a named layer with an explicit
// citation ANCHOR TEXT — the cited plan row, which is what ContractScope
// parses at serve time.
func seedRoleClaim(t *testing.T, st store.Store, id, layer, rule string, scope []string, status core.ClaimStatus, cite, anchor string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: layer, LayerVersion: "v1", Kind: KindObligation,
		Subject: cite, Rule: rule, Scope: scope,
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: cite, TargetHash: "h", AnchorText: anchor}}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func bpStore(t *testing.T) store.Store {
	t.Helper()
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: bpFixtureRoot(t)}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	return st
}

// ─── BP-18: layer roles ─────────────────────────────────────────────────────

// TestLayerRole is BP-18's anchor: the role is manifest DATA, read through
// the same embedded ∪ project-local union layerIsDriftTracked resolves
// through, and it defaults to "governs" so a pack that predates the field
// (or a layer name that no longer resolves at all) can never silently lose
// serving priority.
func TestLayerRole(t *testing.T) {
	root := bpFixtureRoot(t)
	for _, tc := range []struct {
		layer string
		want  string
	}{
		{LayerObligations, RoleGoverns},
		{"adr-rules", RoleGoverns},
		{"spec-bindings", RoleClaims},
		{"findings", RoleClaims},
		{"bp-governs", RoleGoverns},    // project-local pack, explicit
		{"bp-claims", RoleClaims},      // project-local pack, explicit
		{"no-such-layer", RoleGoverns}, // unresolvable: fail-open, never a demotion
	} {
		if got := LayerRole(root, tc.layer); got != tc.want {
			t.Errorf("LayerRole(%q) = %q, want %q", tc.layer, got, tc.want)
		}
	}
	// With no project root the embedded packs still resolve.
	if got := LayerRole("", "spec-bindings"); got != RoleClaims {
		t.Errorf("LayerRole(\"\", spec-bindings) = %q, want %q", got, RoleClaims)
	}
}

// TestObligationsGovernsFillsBudgetFirst is BP-18's serving half: claims
// rules take only the slots governs rules leave, and Displaced still counts
// BOTH roles so the eviction is never silent.
func TestObligationsGovernsFillsBudgetFirst(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)

	// Two governing rules, both BROAD. Eight plan rows, every one of them
	// narrowly and correctly scoped to the edited package — the shape that
	// makes specificity alone serve zero governing rules.
	seedRoleClaim(t, st, "g-1", "bp-governs", "Wire every extractor into fusion.New.", []string{"internal/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")
	seedRoleClaim(t, st, "g-2", "bp-governs", "Every extractor needs a golden test.", []string{"internal/query/**"}, core.ClaimFresh, "CLAUDE.md#L5-L5", "")
	for _, id := range []string{"p-1", "p-2", "p-3", "p-4", "p-5", "p-6", "p-7", "p-8"} {
		seedRoleClaim(t, st, id, "bp-claims", "Plan row "+id+" governs the extractor package.",
			[]string{"internal/extractor/**"}, core.ClaimFresh,
			"docs/plans/x.md#L9-L9",
			"- [ ] task "+id+" {id: "+id+", package: internal/extractor}")
	}

	got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != MaxObligations {
		t.Fatalf("got %d rules, want %d", len(got.Rules), MaxObligations)
	}
	if got.Rules[0].Layer != "bp-governs" {
		t.Errorf("first rule came from %q, want the governs-role layer: %+v", got.Rules[0].Layer, got.Rules)
	}
	governs := 0
	for _, r := range got.Rules {
		if r.Layer == "bp-governs" {
			governs++
		}
	}
	if governs != 1 {
		t.Errorf("governs rules served = %d, want 1 (the only one that matched) — claims rules must not evict it", governs)
	}
	// 9 matched (1 governs + 8 claims), 6 served.
	if got.Displaced != 3 {
		t.Errorf("Displaced = %d, want 3 — the count spans BOTH roles", got.Displaced)
	}
}

// ─── BP-17: fresh first ─────────────────────────────────────────────────────

// TestObligationsFreshFirst is BP-17's anchor (ADR-032 §4 D5): both T02
// payloads in the stopped run led with a stale rule. Served claims are
// ordered fresh before non-fresh, then governs before claims, then
// specificity, then rule text — so the agent, which reads top-down, reads
// the freshest binding rule first.
func TestObligationsFreshFirst(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)

	// Deliberately adversarial to every OTHER key: the stale rules are the
	// narrowest AND the governing ones, so only freshness can put the fresh
	// rules on top.
	seedRoleClaim(t, st, "s-gov", "bp-governs", "Stale governing rule.", []string{"internal/query/engine.go"}, core.ClaimStale, "CLAUDE.md#L1-L1", "")
	seedRoleClaim(t, st, "c-gov", "bp-governs", "Contradicted governing rule.", []string{"internal/query/engine.go"}, core.ClaimContradicted, "CLAUDE.md#L2-L2", "")
	seedRoleClaim(t, st, "f-gov", "bp-governs", "Fresh governing rule.", []string{"internal/**"}, core.ClaimFresh, "CLAUDE.md#L3-L3", "")
	seedRoleClaim(t, st, "f-claim", "bp-claims", "Fresh plan row.", []string{"**"}, core.ClaimFresh,
		"docs/plans/x.md#L9-L9", "- [ ] task {id: BP-X, package: internal/query}")

	got, err := ObligationsFor(ctx, st, "svc", "internal/query/engine.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 4 {
		t.Fatalf("got %d rules, want 4: %+v", len(got.Rules), got.Rules)
	}
	want := []string{
		"Fresh governing rule.", // fresh + governs
		"Fresh plan row.",       // fresh + claims
		"Contradicted governing rule.",
		"Stale governing rule.",
	}
	for i, w := range want {
		if got.Rules[i].Rule != w {
			t.Fatalf("rule[%d] = %q, want %q (full order: %+v)", i, got.Rules[i].Rule, w, got.Rules)
		}
	}
	if got.Rules[0].Status != "" {
		t.Errorf("fresh rule must omit status, got %q", got.Rules[0].Status)
	}
	if got.Rules[2].Status != string(core.ClaimContradicted) || got.Rules[3].Status != string(core.ClaimStale) {
		t.Errorf("non-fresh rules must stay served AND flagged: %+v", got.Rules)
	}
}

// ─── BP-19: contract scoping ────────────────────────────────────────────────

func TestContractScope(t *testing.T) {
	lookup := SymbolFile(func(pkg, sym string) (string, bool) {
		if sym == "ObligationsFor" {
			return "internal/semantic/serve.go", true
		}
		return "", false
	})
	for _, tc := range []struct {
		name  string
		text  string
		want  []string
		wantK bool
	}{
		{"package contract", "- [ ] do the thing {id: BP-1, symbol: Foo, package: internal/extractor}", []string{"internal/extractor/**"}, true},
		{"package already globbed", "- [ ] x {id: BP-2, package: internal/mcp/**}", []string{"internal/mcp/**"}, true},
		{"symbol only", "- [ ] x {id: BP-3, symbol: ObligationsFor}", []string{"internal/semantic/serve.go"}, true},
		{"symbol unknown to the index", "- [ ] x {id: BP-4, symbol: NoSuchSymbol}", nil, true},
		{"row with no contract binds to nothing", "- [ ] x {id: BP-5, skip_test: true}", nil, true},
		{"not a plan row at all", "Finding F1: the retry loop has no ceiling.", nil, false},
		{"empty", "", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ContractScope(tc.text, lookup)
			if ok != tc.wantK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantK)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("scope = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("scope = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestObligationsContractScoping_O23Shape is BP-19's proof on O23's actual
// shape: plan rows whose PROSE names the edited file's directory but whose
// CONTRACT names another package must not be served at all, and the
// governing rule must be first.
func TestObligationsContractScoping_O23Shape(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)

	seedRoleClaim(t, st, "gov", "bp-governs", "Register every extractor in fusion.New and add a golden test.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L7-L7", "")

	// The six O23 rows: the compile-time scope is the LLM's prose-derived
	// guess (it bound them to the extractor package because the word
	// "extractor" appears in the row), while each row's actual contract
	// names a different package.
	misScoped := map[string]string{
		"ct-1": "internal/cpptoolchain",
		"ct-2": "internal/cpptoolchain",
		"ct-3": "schema",
		"ct-4": "internal/treesitter",
		"ct-5": "internal/langtruth",
		"ct-6": "internal/langtruth",
	}
	for id, pkg := range misScoped {
		seedRoleClaim(t, st, id, "bp-claims", "Row "+id+" mentions the extractor and its tree-sitter grammar.",
			[]string{"internal/extractor/**"}, core.ClaimFresh,
			"docs/plans/cpp-toolchain.md#L11-L11",
			"- [ ] work {id: "+id+", package: "+pkg+"}")
	}
	// A row with a contract that DOES name the edited package is served.
	seedRoleClaim(t, st, "ct-ok", "bp-claims", "Row ct-ok binds by contract, not by prose.",
		[]string{"**"}, core.ClaimFresh, "docs/plans/extraction.md#L3-L3",
		"- [ ] work {id: ct-ok, package: internal/extractor}")
	// A row that names NO contract binds to nothing, however suggestive its
	// prose and however broad the LLM's guess was.
	seedRoleClaim(t, st, "ct-none", "bp-claims", "Advance waves only when the extractor gate is green.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "docs/plans/waves.md#L5-L5",
		"- [ ] process {id: ct-none, skip_test: true}")

	got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 2 {
		t.Fatalf("got %d rules, want 2 (the governing rule + the one correctly-contracted row): %+v", len(got.Rules), got.Rules)
	}
	if got.Rules[0].Layer != "bp-governs" {
		t.Errorf("O23 again: the governing rule is not first: %+v", got.Rules)
	}
	if got.Rules[1].Rule != "Row ct-ok binds by contract, not by prose." {
		t.Errorf("rule[1] = %q, want the contract-matched row", got.Rules[1].Rule)
	}
	// Nothing was displaced, because the mis-scoped rows never matched at
	// all — this is the number O23 reported as 65.
	if got.Displaced != 0 {
		t.Errorf("Displaced = %d, want 0 — mis-scoped rows must not even be eligible", got.Displaced)
	}
}

// TestContractScopingLeavesNonPlanCorporaAlone: a claims-role layer whose
// citations are not plan rows (a findings corpus) keeps its compile-time
// scope. The fallback is what keeps BP-19 additive rather than a silent
// de-scoping of every findings claim.
func TestContractScopingLeavesNonPlanCorporaAlone(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "find-1", "bp-claims", "The retry loop has no ceiling.",
		[]string{"internal/extractor/**"}, core.ClaimFresh,
		"findings/f1.md#L3-L3", "The retry loop in the Go extractor has no ceiling.")

	got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("got %d rules, want 1 — a non-plan citation keeps the compiled scope: %+v", len(got.Rules), got.Rules)
	}
}

// TestContractScopeReadsTheCitationWhenTheLinkHasNoAnchorText covers the
// legacy hash-only claim (a cites link written before AnchorText existed):
// the contract is re-read from the cited plan file under the service root,
// so BP-19 cannot silently no-op on the oldest claims in a store.
func TestContractScopeReadsTheCitationWhenTheLinkHasNoAnchorText(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	// testdata/bp19fixture/docs/plans/legacy.md line 3 carries the contract.
	seedRoleClaim(t, st, "legacy", "bp-claims", "Legacy row, no anchor text stored.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "docs/plans/legacy.md#L3-L3", "")

	// The stored scope says extractor; the cited row's contract says query.
	if got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0); err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	} else if len(got.Rules) != 0 {
		t.Errorf("legacy claim served against its prose-derived scope: %+v", got.Rules)
	}
	if got, err := ObligationsFor(ctx, st, "svc", "internal/query/engine.go", 0); err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	} else if len(got.Rules) != 1 {
		t.Errorf("legacy claim not re-scoped to its cited contract: %+v", got.Rules)
	}
}
