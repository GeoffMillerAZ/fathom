package client_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// obligationsCorpus builds a synthetic service of nFiles Go files (~2 KiB
// each, spread over packages) under a fixed set of governing guides, ingests
// it into a SQLite store, and records a budget-stopped compile run so every
// obligations enrichment (live coverage, raw fallback) does its full work.
func obligationsCorpus(t *testing.T, nFiles int) (*client.Client, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	body := strings.Repeat("// filler line for a realistically sized source file\n", 36)
	tree := map[string]string{
		"CLAUDE.md":            "# Rules\n\n## Non-negotiables\n\n- Keep the gate green.\n- workspace/ holds artifacts.\n",
		"ADR.md":               "# ADR\n\n## ADR-001: store purity\n\nThe store is pure Go.\n",
		"internal/CLAUDE.md":   "# internal\n\n- Sort every output.\n",
		"docs/plans/README.md": "# Plans\n\n- Done means proven.\n",
	}
	for i := 0; i < nFiles; i++ {
		pkg := fmt.Sprintf("p%03d", i/20)
		tree[fmt.Sprintf("internal/%s/f%04d.go", pkg, i)] = fmt.Sprintf("package %s\n\n%sfunc F%04d() {}\n", pkg, body, i)
	}
	writeTree(t, dir, tree)
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "fleet.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(ctx, core.CompileRun{RunID: "r1", ServiceID: "svc", Layer: "obligations", RanAt: time.Unix(1, 0).UTC(), SectionsTotal: 4, BudgetStopped: true}); err != nil {
		t.Fatal(err)
	}
	seedAllocClaims(t, st)
	return c, func() { c.Close() }
}

// seedAllocClaims serves the rules path real work (RT-174): a governing
// claim over internal/**, and spec-bindings claims citing plan rows — one
// with a package contract and one symbol-only ({id, symbol}), the shape
// that made serving resolve the symbol's file by loading every observation
// in the service on every call.
func seedAllocClaims(t *testing.T, st store.Store) {
	t.Helper()
	mk := func(id, layer, rule, cite, anchor string, scope []string) (core.SemanticClaim, []core.ClaimLink) {
		cl := core.SemanticClaim{
			ClaimID: id, ServiceID: "svc", Layer: layer, LayerVersion: "v1", Kind: semantic.KindObligation,
			Subject: cite, Rule: rule, Scope: scope, Tier: core.TierDocClaimed, TierProfile: "engineering",
			Status: core.ClaimFresh, ReproKey: id, InputDigest: "d",
			CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		}
		links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: cite, TargetHash: "h", AnchorText: anchor}}
		for _, g := range scope {
			links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopes, TargetRef: g})
		}
		return cl, links
	}
	var claims []core.SemanticClaim
	var links []core.ClaimLink
	for _, c := range []struct{ id, layer, rule, cite, anchor string }{
		{"gov-1", "obligations", "Sort every output.", "internal/CLAUDE.md#L3-L3", "- Sort every output."},
		{"sb-pkg", "spec-bindings", "Row T-1 binds by package.", "docs/plans/README.md#L3-L3", "- [ ] t {id: T-1, symbol: F0007, package: internal/p000}"},
		{"sb-sym", "spec-bindings", "Row T-2 binds by symbol only.", "docs/plans/README.md#L3-L3", "- [ ] t {id: T-2, symbol: F0007}"},
	} {
		cl, ls := mk(c.id, c.layer, c.rule, c.cite, c.anchor, []string{"internal/**"})
		claims = append(claims, cl)
		links = append(links, ls...)
	}
	if err := st.PutClaims(context.Background(), claims, links); err != nil {
		t.Fatal(err)
	}
}

// obligationsCallBytes is the mean bytes allocated by one edit's worth of
// obligations work — exactly the backend calls the obligations tool makes
// for one file: the rules, the index-freshness check, the live layer
// coverage and the raw fallback.
func obligationsCallBytes(t *testing.T, c *client.Client, file string, calls int) uint64 {
	t.Helper()
	ctx := context.Background()
	one := func() {
		obl, err := c.ObligationsLayer(ctx, "svc", file, 6, semantic.ObligationLayerAll)
		if err != nil {
			t.Fatal(err)
		}
		if len(obl.Rules) < 3 {
			t.Fatalf("rules path served %d rules, want the governing claim and both spec-bindings rows (the symbol-only row resolved): %+v", len(obl.Rules), obl.Rules)
		}
		c.ObligationsIndexCheck(ctx, "svc", file)
		if _, found, err := c.ObligationsLayerCoverage(ctx, "svc"); err != nil || !found {
			t.Fatalf("coverage found=%v err=%v", found, err)
		}
		if _, _, ok, err := c.RawFallbackObligations(ctx, "svc", file, semantic.RawFallbackReserve); err != nil || !ok {
			t.Fatalf("raw fallback ok=%v err=%v", ok, err)
		}
	}
	one() // warm the per-watermark memo: the steady state is what an edit pays
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < calls; i++ {
		one()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(calls)
}

// TestObligationsCallAllocationFlatInCorpusSize is the lane's proof for
// RT-81/RT-143/RT-83/RT-26 together (design §4.3; the measured incident:
// 468.6 GB in one profiled hour from whole-tree reads per edit): the bytes
// one obligations call allocates must not grow with the size of the tree.
// Ten times the files may cost at most 1.5x the bytes (noise headroom), and
// a call stays under the design's 2 MiB target.
func TestObligationsCallAllocationFlatInCorpusSize(t *testing.T) {
	if testing.Short() {
		t.Skip("ingests two synthetic corpora")
	}
	const edited = "internal/p000/f0007.go"
	small, closeSmall := obligationsCorpus(t, 60)
	defer closeSmall()
	large, closeLarge := obligationsCorpus(t, 600)
	defer closeLarge()

	const calls = 20
	s := obligationsCallBytes(t, small, edited, calls)
	l := obligationsCallBytes(t, large, edited, calls)
	t.Logf("bytes allocated per obligations call: 60 files %d, 600 files %d (ratio %.2f)", s, l, float64(l)/float64(s))
	if float64(l) > 1.5*float64(s) {
		t.Errorf("per-call allocation grew with the tree: %d B at 60 files, %d B at 600 (x%.2f); want flat", s, l, float64(l)/float64(s))
	}
	if l > 2<<20 {
		t.Errorf("per-call allocation %d B exceeds the 2 MiB target", l)
	}
}
