package semantic

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// candidateByRule indexes an explanation's candidates by rule text.
func candidateByRule(t *testing.T, ex ObligationsExplanation) map[string]ExplainCandidate {
	t.Helper()
	out := map[string]ExplainCandidate{}
	for _, c := range ex.Candidates {
		out[c.Rule] = c
	}
	return out
}

// TestExplainObligations_DisplacedReasons pins OC-32: the explanation names
// every scope-matched candidate with its scope tier and rank, the cap and
// the reservation, the served list, and each displaced claim's reason —
// duplicate, rank, role, freshness, tier and reserve — and its served list
// is exactly what serving returns for the same inputs (the default output
// is unchanged by --explain, because the explanation IS the serving run).
func TestExplainObligations_DisplacedReasons(t *testing.T) {
	ctx := context.Background()
	const file = "internal/mcp/foo.go"

	// Scenario 1: dedupe, in-tier rank, role and freshness.
	st := store.NewMemoryStore()
	seedServed(t, st, "p1", "Register every tool in the mcp registry.", []string{"internal/mcp/**"}, core.ClaimFresh)
	seedServed(t, st, "p2", "Keep tool descriptions under the byte cap.", []string{"internal/mcp/**"}, core.ClaimFresh)
	seedServed(t, st, "p3", "Pin argument schemas with the golden.", []string{"internal/mcp/**"}, core.ClaimStale)
	seedServed(t, st, "a1", "Inject clocks; never call time.Now().", []string{"internal/**"}, core.ClaimFresh)
	seedServed(t, st, "a2", "Inject clocks; never call `time.Now()`", []string{"internal/**"}, core.ClaimFresh)
	for _, n := range []string{"1", "2", "3", "4"} {
		seedServed(t, st, "r"+n, "Root rule "+n+".", []string{"**"}, core.ClaimFresh)
	}
	seedServedLayer(t, st, "f1", "findings", "Finding: tool registry drifted.", []string{"internal/mcp/**"}, core.ClaimFresh)

	ex, err := ExplainObligations(ctx, st, "svc", file, 0, ObligationLayerAll)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ObligationsForLayer(ctx, st, "svc", file, 0, ObligationLayerAll)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ex.Served, want) {
		a, _ := json.MarshalIndent(ex.Served, "", " ")
		b, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("the explanation's served list must be serving's own:\nexplain: %s\nserving: %s", a, b)
	}
	if ex.Cap != MaxObligations || ex.ReservedSlots != nearestTierReserveSlots || ex.Eligible != 10 || ex.Deduped != 1 || ex.Displaced != 3 || ex.RankedBy != "none" {
		t.Errorf("summary = cap %d reserved %d eligible %d deduped %d displaced %d ranked_by %q; want 6, 2, 10, 1, 3, none",
			ex.Cap, ex.ReservedSlots, ex.Eligible, ex.Deduped, ex.Displaced, ex.RankedBy)
	}
	if strings.Join(ex.Ranking, ",") != "freshness,role,scope_tier,relevance,specificity,text" {
		t.Errorf("ranking = %v", ex.Ranking)
	}
	byRule := candidateByRule(t, ex)
	served := []string{"Register every tool in the mcp registry.", "Keep tool descriptions under the byte cap.", "Inject clocks; never call `time.Now()`", "Root rule 1.", "Root rule 2.", "Root rule 3."}
	for i, r := range served {
		c := byRule[r]
		if !c.Served || c.Position != i+1 || c.Displaced != nil {
			t.Errorf("%q = %+v, want served at #%d", r, c, i+1)
		}
		if got := want.Rules[i].Rule; got != r {
			t.Errorf("serving's #%d is %q, the explanation says %q", i+1, got, r)
		}
	}
	for _, r := range []string{"Register every tool in the mcp registry.", "Keep tool descriptions under the byte cap."} {
		if c := byRule[r]; !c.Reserved || c.ScopeTier != ScopeTierPackage {
			t.Errorf("%q must hold a reserved package-tier slot: %+v", r, c)
		}
	}
	reasons := map[string]string{
		"Inject clocks; never call time.Now().": DisplacedDuplicate,
		"Root rule 4.":                          DisplacedRank,
		"Finding: tool registry drifted.":       DisplacedRole,
		"Pin argument schemas with the golden.": DisplacedFreshness,
	}
	for rule, reason := range reasons {
		c := byRule[rule]
		if c.Served || c.Displaced == nil || c.Displaced.Reason != reason || c.Displaced.Why == "" {
			t.Errorf("%q displaced = %+v, want reason %s with its why", rule, c.Displaced, reason)
		}
	}
	if d := byRule["Inject clocks; never call time.Now()."].Displaced; d == nil || d.By != "a2" || !strings.Contains(d.Why, "served at #3") {
		t.Errorf("the duplicate must name the served rule it repeats: %+v", d)
	}
	if d := byRule["Root rule 4."].Displaced; d == nil || d.By != "r3" || !strings.Contains(d.Why, "rule text order") {
		t.Errorf("an in-tier loss names the served rule above it and the key: %+v", d)
	}
	if c := byRule["Finding: tool registry drifted."]; c.Role != RoleClaims || c.Layer != "findings" || c.Kind != "" && c.Kind == KindObligation {
		t.Errorf("finding candidate = %+v", c)
	}
	// Ranks are the ranking order, duplicates included; 10 candidates.
	if len(ex.Candidates) != 10 {
		t.Fatalf("candidates = %d, want 10", len(ex.Candidates))
	}
	for i, c := range ex.Candidates {
		if c.Rank != i+1 || c.Src == "" || c.Tier != string(core.TierDocClaimed) || c.Status == "" || len(c.Scope) == 0 {
			t.Errorf("candidate %d = %+v", i, c)
		}
	}
	tiers := map[string]ExplainTier{}
	for _, tr := range ex.Tiers {
		tiers[tr.Tier] = tr
	}
	for tier, w := range map[string]ExplainTier{
		ScopeTierExactFile: {Tier: ScopeTierExactFile},
		ScopeTierPackage:   {Tier: ScopeTierPackage, Candidates: 4, Served: 2, Displaced: 2},
		ScopeTierParentDir: {Tier: ScopeTierParentDir, Candidates: 2, Served: 1, Duplicates: 1},
		ScopeTierRoot:      {Tier: ScopeTierRoot, Candidates: 4, Served: 3, Displaced: 1},
	} {
		if tiers[tier] != w {
			t.Errorf("tier %s = %+v, want %+v", tier, tiers[tier], w)
		}
	}
	if len(ex.Tiers) != 4 || ex.Tiers[0].Tier != ScopeTierExactFile || ex.Tiers[3].Tier != ScopeTierRoot {
		t.Errorf("tiers must be the four columns, nearest first: %+v", ex.Tiers)
	}

	// Scenario 2: a nearer tier fills the cap (tier), and the nearest
	// tier's reserved slots displace higher-ranked rules (reserve).
	st2 := store.NewMemoryStore()
	for _, n := range []string{"1", "2", "3", "4", "5", "6"} {
		seedServed(t, st2, "pk"+n, "Package rule "+n+".", []string{"internal/mcp/**"}, core.ClaimFresh)
	}
	seedServed(t, st2, "root", "Root rule.", []string{"**"}, core.ClaimFresh)
	ex2, err := ExplainObligations(ctx, st2, "svc", file, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if ex2.Layer != string(ObligationLayerRules) {
		t.Errorf("the default layer is the obligations tool's own, rules: %q", ex2.Layer)
	}
	if d := candidateByRule(t, ex2)["Root rule."].Displaced; d == nil || d.Reason != DisplacedTier || !strings.Contains(d.Why, "its package directory") {
		t.Errorf("a farther rule behind a full nearer tier = %+v, want tier", d)
	}
	st3 := store.NewMemoryStore()
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7"} {
		seedServed(t, st3, "r"+n, "Root rule "+n+".", []string{"**"}, core.ClaimFresh)
	}
	seedServed(t, st3, "guide", "Register every tool in the mcp registry.", []string{"internal/mcp/**"}, core.ClaimStale)
	ex3, err := ExplainObligations(ctx, st3, "svc", file, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	by3 := candidateByRule(t, ex3)
	if c := by3["Register every tool in the mcp registry."]; !c.Served || !c.Reserved {
		t.Errorf("the stale nearest rule holds a reserved slot: %+v", c)
	}
	if d := by3["Root rule 6."].Displaced; d == nil || d.Reason != DisplacedReserve {
		t.Errorf("a root rule outranking the reserved one = %+v, want reserve", d)
	}

	// The root-scope question explains RootObligationsForLayer.
	root, err := ExplainObligations(ctx, st3, "svc", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, _ := RootObligationsForLayer(ctx, st3, "svc", 0, ObligationLayerRules)
	if !reflect.DeepEqual(root.Served, wantRoot) || len(root.Tiers) != 1 || root.Tiers[0].Tier != ScopeTierRoot || root.ReservedSlots != 0 {
		t.Errorf("root-scope explanation = %+v", root)
	}
}

// TestObligations_DedupesIdenticalStatements reproduces OC-33's defect on
// the real store's shape: internal/CLAUDE.md#determinism-is-a-contract was
// compiled twice into "Inject clocks (`now func() time.Time`); never call
// `time.Now()` …" and the same sentence without backticks, and serving
// spent two of six slots on it for internal/conserve/conserve.go — pushing
// a distinct package rule past the cap. One statement is now served once,
// and the duplicate is named, not counted as displaced.
func TestObligations_DedupesIdenticalStatements(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	const subject = "internal/CLAUDE.md#determinism-is-a-contract"
	const file = "internal/conserve/conserve.go"
	scope := []string{"internal/**/*.go"}
	withTicks := "Inject clocks (`now func() time.Time`); never call `time.Now()` in query/analysis logic."
	plain := "Inject clocks (now func() time.Time); never call time.Now() in query/analysis logic."
	seedServedSubject(t, st, "clocks-ticks", LayerObligations, subject, withTicks, scope, core.ClaimFresh)
	seedServedSubject(t, st, "clocks-plain", LayerObligations, subject, plain, scope, core.ClaimFresh)
	others := []string{
		"Prefer golden tests against the fauxtown fixture fleet.",
		"Always run go test -race before claiming green.",
		"Implement extractor.Extractor, register by extension, golden-test it.",
		"Never mix fixed fixture dates with real clocks.",
		"Keep internal packages free of UI imports.",
	}
	for i, r := range others {
		seedServedSubject(t, st, "pkg-"+string(rune('a'+i)), LayerObligations, "internal/CLAUDE.md#s", r, scope, core.ClaimFresh)
	}
	for _, n := range []string{"1", "2", "3"} {
		seedServedSubject(t, st, "root-"+n, LayerObligations, "CLAUDE.md#r", "Root rule "+n+".", []string{"**"}, core.ClaimFresh)
	}

	if NormalizedStatement(withTicks) != NormalizedStatement(plain) {
		t.Fatalf("the two statements must normalize alike: %q vs %q", NormalizedStatement(withTicks), NormalizedStatement(plain))
	}
	if NormalizedStatement("Use foo.") == NormalizedStatement("Use bar.") {
		t.Fatal("normalization must never merge different words")
	}

	// Every serving entry point: the tool's default layer and the union.
	for name, serve := range map[string]func() (Obligations, error){
		"ObligationsFor":      func() (Obligations, error) { return ObligationsFor(ctx, st, "svc", file, 0) },
		"ObligationsForLayer": func() (Obligations, error) { return ObligationsForLayer(ctx, st, "svc", file, 0, ObligationLayerRules) },
	} {
		got, err := serve()
		if err != nil {
			t.Fatal(err)
		}
		clocks, served := 0, map[string]bool{}
		for _, r := range got.Rules {
			served[r.Rule] = true
			if strings.HasPrefix(r.Rule, "Inject clocks") {
				clocks++
			}
		}
		if clocks != 1 {
			t.Errorf("%s served the clocks rule %d times, want once: %+v", name, clocks, got.Rules)
		}
		for _, r := range others {
			if !served[r] {
				t.Errorf("%s displaced %q — the duplicate took its slot", name, r)
			}
		}
		// Nine distinct statements, six slots: three displaced; the
		// duplicate is not one of them.
		if got.Displaced != 3 {
			t.Errorf("%s displaced = %d, want 3 (the duplicate is not an eviction)", name, got.Displaced)
		}
	}

	ex, err := ExplainObligations(ctx, st, "svc", file, 0, ObligationLayerRules)
	if err != nil {
		t.Fatal(err)
	}
	var dup *ExplainCandidate
	for i, c := range ex.Candidates {
		if c.Displaced != nil && c.Displaced.Reason == DisplacedDuplicate {
			if dup != nil {
				t.Fatalf("two duplicates reported: %+v and %+v", *dup, c)
			}
			dup = &ex.Candidates[i]
		}
	}
	if dup == nil || !strings.HasPrefix(dup.Rule, "Inject clocks") || ex.Deduped != 1 {
		t.Fatalf("the explanation must name the duplicate: %+v (deduped %d)", dup, ex.Deduped)
	}
	kept := candidateByRule(t, ex)[withTicks]
	if dup.Rule == withTicks {
		kept = candidateByRule(t, ex)[plain]
	}
	if !kept.Served || dup.Displaced.By != kept.ClaimID {
		t.Errorf("the duplicate names the served rule it repeats: dup %+v, kept %+v", dup, kept)
	}
}
