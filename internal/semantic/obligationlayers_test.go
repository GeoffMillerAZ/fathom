package semantic

// obligationlayers_test.go — VB-9 (docs/plans/verbs.md, ADR-035 decision
// 2): `layer: rules|claims|all` pinned against the O23 shape one more
// time, this time as a REQUEST the caller makes rather than a fixed
// blend.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestObligationsForLayer_RulesIsDefaultAndExcludesClaims: "rules" serves
// ONLY RoleGoverns claims — not even a correctly contract-bound plan row.
func TestObligationsForLayer_RulesIsDefaultAndExcludesClaims(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "gov", "bp-governs", "Wire every extractor into fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")
	seedRoleClaim(t, st, "claim-ok", "bp-claims", "Row claim-ok binds by contract.",
		[]string{"**"}, core.ClaimFresh, "docs/plans/x.md#L9-L9",
		"- [ ] work {id: claim-ok, package: internal/extractor}")

	got, err := ObligationsForLayer(ctx, st, "svc", "internal/extractor/goextract.go", 0, ObligationLayerRules)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 || got.Rules[0].Layer != "bp-governs" {
		t.Fatalf("layer:rules served %+v, want exactly the governs-role rule", got.Rules)
	}
}

// TestObligationsForLayer_ClaimsRequiresAContract: "claims" serves ONLY a
// plan row bound by its own {symbol, package} contract — never the
// compile-time prose fallback a findings corpus needs, and never a plan
// row with no contract at all.
func TestObligationsForLayer_ClaimsRequiresAContract(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "gov", "bp-governs", "Wire every extractor into fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")
	seedRoleClaim(t, st, "claim-ok", "bp-claims", "Row claim-ok binds by contract, not by prose.",
		[]string{"**"}, core.ClaimFresh, "docs/plans/x.md#L9-L9",
		"- [ ] work {id: claim-ok, package: internal/extractor}")
	seedRoleClaim(t, st, "claim-none", "bp-claims", "A plan row naming the extractor in prose but no contract.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "docs/plans/waves.md#L5-L5",
		"- [ ] process {id: claim-none, skip_test: true}")
	seedRoleClaim(t, st, "finding-1", "bp-claims", "The retry loop in the Go extractor has no ceiling.",
		[]string{"internal/extractor/**"}, core.ClaimFresh,
		"findings/f1.md#L3-L3", "The retry loop in the Go extractor has no ceiling.")

	got, err := ObligationsForLayer(ctx, st, "svc", "internal/extractor/goextract.go", 0, ObligationLayerClaims)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 || got.Rules[0].Rule != "Row claim-ok binds by contract, not by prose." {
		t.Fatalf("layer:claims served %+v, want exactly the one contract-bound plan row (never the governs rule, the no-contract row, or the findings-style prose fallback)", got.Rules)
	}
}

// TestObligationsForLayer_AllIsThePreVB9Union: "all" is byte-for-byte the
// union ObligationsFor always served — the additive guarantee VB-9 makes.
func TestObligationsForLayer_AllIsThePreVB9Union(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "gov", "bp-governs", "Wire every extractor into fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")
	seedRoleClaim(t, st, "finding-1", "bp-claims", "The retry loop in the Go extractor has no ceiling.",
		[]string{"internal/extractor/**"}, core.ClaimFresh,
		"findings/f1.md#L3-L3", "The retry loop in the Go extractor has no ceiling.")

	viaAll, err := ObligationsForLayer(ctx, st, "svc", "internal/extractor/goextract.go", 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer(all): %v", err)
	}
	viaDefault, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(viaAll.Rules) != len(viaDefault.Rules) || len(viaAll.Rules) != 2 {
		t.Fatalf("layer:all = %+v, ObligationsFor = %+v, want identical 2-rule payloads", viaAll.Rules, viaDefault.Rules)
	}
	for i := range viaAll.Rules {
		if viaAll.Rules[i] != viaDefault.Rules[i] {
			t.Fatalf("rule[%d]: layer:all = %+v, ObligationsFor = %+v", i, viaAll.Rules[i], viaDefault.Rules[i])
		}
	}
}

// TestObligationsForLayer_InvalidValueDegradesToAll: an unrecognized filter
// value is treated exactly like "all" by obligationsCore's switch (fail-open,
// never a silent empty result) — the MCP layer is where an invalid request
// value is refused with an error; the engine itself never refuses to serve.
func TestObligationsForLayer_InvalidValueDegradesToAll(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "gov", "bp-governs", "Wire every extractor into fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")
	seedRoleClaim(t, st, "finding-1", "bp-claims", "The retry loop in the Go extractor has no ceiling.",
		[]string{"internal/extractor/**"}, core.ClaimFresh,
		"findings/f1.md#L3-L3", "The retry loop in the Go extractor has no ceiling.")

	got, err := ObligationsForLayer(ctx, st, "svc", "internal/extractor/goextract.go", 0, ObligationLayerFilter("bogus"))
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 2 {
		t.Fatalf("unrecognized layer value served %+v, want the fail-open union of 2", got.Rules)
	}
}

// TestDisplacedByTier_VB10: the cap's damage is broken out by governing-
// scope tier, and only for a file-scoped call.
func TestDisplacedByTier_VB10(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	// One package-level rule (kept) plus enough root-reaching rules to
	// overflow MaxObligations, all governs-role so role never intervenes.
	seedRoleClaim(t, st, "pkg", "bp-governs", "Package rule.", []string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L1-L1", "")
	for _, id := range []string{"r1", "r2", "r3", "r4", "r5", "r6"} {
		seedRoleClaim(t, st, id, "bp-governs", "Root rule "+id+".", []string{"**"}, core.ClaimFresh, "CLAUDE.md#L2-L2", "")
	}
	got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if got.Displaced == 0 {
		t.Fatalf("expected some displacement, got none: %+v", got)
	}
	if got.DisplacedByTier == nil || got.DisplacedByTier["root"] == 0 {
		t.Fatalf("DisplacedByTier = %+v, want a non-zero \"root\" count", got.DisplacedByTier)
	}
	total := 0
	for _, n := range got.DisplacedByTier {
		total += n
	}
	if total != got.Displaced {
		t.Fatalf("DisplacedByTier sums to %d, Displaced = %d — must agree", total, got.Displaced)
	}
	// RootObligationsFor carries no path to be proximate to — no tier map.
	rootAns, err := RootObligationsFor(ctx, st, "svc", 0)
	if err != nil {
		t.Fatalf("RootObligationsFor: %v", err)
	}
	if rootAns.DisplacedByTier != nil {
		t.Fatalf("root-scope answer must never carry a tier breakdown: %+v", rootAns.DisplacedByTier)
	}
}

// TestObligationsFor_NearestGuideOutranksRootADR is O54 deliverable 4
// (VB-8, once a nested guide's claims actually get compiled): a file
// under internal/extractor/ must find the internal/CLAUDE.md rule that
// governs it in the top three served rules, ahead of root ADR.md rules —
// and the cap's eviction must never take the nearer-scope guide rule in
// favour of a root one.
func TestObligationsFor_NearestGuideOutranksRootADR(t *testing.T) {
	ctx := context.Background()
	st := bpStore(t)
	seedRoleClaim(t, st, "guide", "bp-governs", "Register every extractor in fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "internal/CLAUDE.md#L4-L4", "")
	for _, id := range []string{"adr1", "adr2", "adr3", "adr4", "adr5", "adr6"} {
		seedRoleClaim(t, st, id, "bp-governs", "Root rule "+id+".", []string{"**"}, core.ClaimFresh, "ADR.md#L2-L2", "")
	}

	got, err := ObligationsFor(ctx, st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) == 0 {
		t.Fatal("no rules served")
	}
	pos := -1
	for i, r := range got.Rules {
		if r.Src == "internal/CLAUDE.md#L4-L4" {
			pos = i
			break
		}
	}
	if pos < 0 {
		t.Fatalf("internal/CLAUDE.md rule not served at all: %+v", got.Rules)
	}
	if pos >= 3 {
		t.Fatalf("internal/CLAUDE.md rule at position %d, want top three (rules=%+v)", pos, got.Rules)
	}
	// The 7 eligible claims (1 guide + 6 root) overflow the 6-rule cap —
	// the guide must never be the one displaced in favour of a root rule.
	if got.DisplacedByTier["package"] != 0 {
		t.Fatalf("DisplacedByTier = %+v, the nearer-scope (package) rule must never be displaced in favour of a root one under the cap", got.DisplacedByTier)
	}
	if got.DisplacedByTier["root"] == 0 {
		t.Fatalf("DisplacedByTier = %+v, want a non-zero root count (7 eligible, cap 6)", got.DisplacedByTier)
	}
}
