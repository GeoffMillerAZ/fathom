package indexbench

// governingrules_test.go — VB-11: GoverningRuleOracle and
// GrepNearestClaudeMD run deterministically. A hermetic, hand-seeded store
// (never the real corpus, never an LLM) proves the ARITHMETIC — same
// inputs, same rates, twice in a row and regardless of seeding order.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedGoverns writes one fresh, RoleGoverns ("obligations" layer) claim —
// the shape GoverningRuleOracle's layer:"rules" call reads.
func seedGoverns(t *testing.T, st store.Store, id, rule string, scope []string, cite string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: semantic.LayerObligations, LayerVersion: "v1", Kind: semantic.KindObligation,
		Subject: cite, Rule: rule, Scope: scope,
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
		ReproKey: id, InputDigest: "d", CompiledAtWatermark: "manifest:test",
		CompiledAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: cite, TargetHash: "h"}}
	for _, g := range scope {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func fixedFixture() GoverningRulesFixture {
	return GoverningRulesFixture{
		Oracle: "governing-rules",
		Entries: []GoverningRuleLabel{
			{Path: "a/x.go", GoverningRule: GoverningRule{Source: "a/CLAUDE.md", LineHint: "L1", Text: "rule a"}},
			{Path: "b/y.go", GoverningRule: GoverningRule{Source: "b/CLAUDE.md", LineHint: "L1", Text: "rule b"}},
			{Path: "c/z.go", GoverningRule: GoverningRule{Source: "nowhere/CLAUDE.md", LineHint: "L1", Text: "unreachable"}},
		},
	}
}

func seededStore(t *testing.T) store.Store {
	t.Helper()
	st := store.NewMemoryStore()
	// a/x.go: the labelled rule is the top (and only) match.
	seedGoverns(t, st, "a-gov", "Rule governing a/.", []string{"a/**"}, "a/CLAUDE.md#L1-L1")
	// b/y.go: a crowding root rule sits ahead in text but must lose to the
	// package-scoped one under tier ranking — exercises that the oracle is
	// scoring the REAL ranker, not a stub.
	seedGoverns(t, st, "b-root", "A root style rule.", []string{"**"}, "CLAUDE.md#L1-L1")
	seedGoverns(t, st, "b-gov", "Rule governing b/.", []string{"b/**"}, "b/CLAUDE.md#L1-L1")
	// c/z.go: nothing seeded governs it — a guaranteed miss.
	return st
}

func TestGoverningRuleOracle_Deterministic(t *testing.T) {
	fx := fixedFixture()
	st := seededStore(t)
	ctx := context.Background()

	first, err := GoverningRuleOracle(ctx, st, "svc", fx)
	if err != nil {
		t.Fatalf("GoverningRuleOracle: %v", err)
	}
	second, err := GoverningRuleOracle(ctx, st, "svc", fx)
	if err != nil {
		t.Fatalf("GoverningRuleOracle (second run): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("GoverningRuleOracle is not deterministic: %+v vs %+v", first, second)
	}
	if first.Labels != 3 {
		t.Fatalf("Labels = %d, want 3", first.Labels)
	}
	if first.Top1Hits != 2 || first.Top3Hits != 2 {
		t.Fatalf("got top1=%d top3=%d, want 2/2 (a/x.go and b/y.go hit, c/z.go always misses)", first.Top1Hits, first.Top3Hits)
	}
	if len(first.Misses) != 1 || first.Misses[0] != "c/z.go" {
		t.Fatalf("Misses = %v, want [c/z.go]", first.Misses)
	}
	wantRate := 2.0 / 3.0
	if first.Top1Rate != wantRate || first.Top3Rate != wantRate {
		t.Fatalf("rates = %.4f/%.4f, want %.4f", first.Top1Rate, first.Top3Rate, wantRate)
	}
}

func TestGrepNearestClaudeMD_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a/CLAUDE.md", "# a")
	writeFile(t, root, "b/CLAUDE.md", "# b")
	writeFile(t, root, "a/x.go", "package a")
	writeFile(t, root, "b/y.go", "package b")
	writeFile(t, root, "c/z.go", "package c")

	fx := fixedFixture()
	first := GrepNearestClaudeMD(root, fx)
	second := GrepNearestClaudeMD(root, fx)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("GrepNearestClaudeMD is not deterministic: %+v vs %+v", first, second)
	}
	if first.Top1Hits != 2 {
		t.Fatalf("got %d hits, want 2 (a/x.go and b/y.go each have their own nearest CLAUDE.md; c/z.go's label names an unreachable doc)", first.Top1Hits)
	}
	if len(first.Misses) != 1 || first.Misses[0] != "c/z.go" {
		t.Fatalf("Misses = %v, want [c/z.go]", first.Misses)
	}
}

func TestLoadGoverningRules_RealFixtureParses(t *testing.T) {
	fx, err := LoadGoverningRules("../../testdata/indexbench/governing-rules.json")
	if err != nil {
		t.Fatalf("LoadGoverningRules: %v", err)
	}
	if len(fx.Entries) != 40 {
		t.Fatalf("got %d entries, want 40", len(fx.Entries))
	}
	for _, e := range fx.Entries {
		if e.Path == "" || e.GoverningRule.Source == "" || e.GoverningRule.Text == "" {
			t.Fatalf("incomplete entry: %+v", e)
		}
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
