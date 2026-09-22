package semantic

// decisionrank_test.go — DP-7/DP-8 (docs/plans/decision-port.md, ADR-042
// decision 2): the obligations re-rank pinned against the tier law it must
// never break.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const rerankTestFile = "internal/extractor/goextract.go"

// mockNoulResponse builds a decision.Response answering one KindNoul
// question per (claimID, noul) pair — a compact way to script a
// MockDecider entry for these tests.
func mockNoulResponse(provider string, answers map[string]float64) decision.Response {
	out := decision.Response{Provider: provider, Answers: make(map[string]decision.Answer, len(answers))}
	for id, noul := range answers {
		out.Answers[id] = decision.Answer{ID: id, Kind: decision.KindNoul, Noul: noul}
	}
	return out
}

// TestObligations_RerankWithinTierOnly is DP-7's acceptance test: a
// governing-scope-tier law (ADR-035 decision 2) an LLM may never cross,
// with a within-tier swap, an unsure-band rule that must not move, and the
// none-path's served count/Displaced preserved exactly.
func TestObligations_RerankWithinTierOnly(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// Three exact-file-tier rules (scope == the file itself, tier 0) whose
	// RULE TEXT alone (the "1-"/"2-"/"3-" prefixes) fixes their BASELINE
	// (no-rerank) order — deliberately adversarial to noul: 1-exact (an
	// unsure-band 0.3 answer) sits first, 2-exact (0.6) sits second,
	// 3-exact (0.95, the HIGHER noul) sits third. Descending-noul reorder
	// must therefore swap 2-exact and 3-exact while 1-exact never moves.
	// No rule text mentions "internal"/"extractor"/"goextract" — every
	// rule's lexical relevance is 0, so the ordinary O2 relevance
	// tie-break never confounds the baseline order this test controls.
	seedServed(t, st, "1-exact", "1-exact: requires review before any change.", []string{rerankTestFile}, core.ClaimFresh)
	seedServed(t, st, "2-exact", "2-exact: requires review before any change too.", []string{rerankTestFile}, core.ClaimFresh)
	seedServed(t, st, "3-exact", "3-exact: requires review before any change also.", []string{rerankTestFile}, core.ClaimFresh)
	// A root-tier rule (unanchored scope) with the HIGHEST noul of all
	// four — ADR-035's tier law must keep it last regardless.
	seedServed(t, st, "0-root", "0-root: applies broadly across the repository.", []string{"**"}, core.ClaimFresh)

	none, err := ObligationsFor(ctx, st, "svc", rerankTestFile, 0)
	if err != nil {
		t.Fatalf("ObligationsFor (none path): %v", err)
	}
	if len(none.Rules) != 4 || none.Displaced != 0 {
		t.Fatalf("none path = %d rules, displaced %d; want 4 rules, 0 displaced: %+v", len(none.Rules), none.Displaced, none.Rules)
	}

	// The byte-identical assertion: ObligationsForOpts with the zero
	// ObligationsOptions{} must equal ObligationsFor exactly.
	zeroOpts, err := ObligationsForOpts(ctx, st, "svc", rerankTestFile, 0, ObligationsOptions{})
	if err != nil {
		t.Fatalf("ObligationsForOpts (zero opts): %v", err)
	}
	if !reflect.DeepEqual(none, zeroOpts) {
		t.Fatalf("ObligationsForOpts(zero opts) != ObligationsFor:\n none=%+v\n zero=%+v", none, zeroOpts)
	}

	mock := decision.NewMockDecider("mock", decision.MockEntry{
		Response: mockNoulResponse("mock", map[string]float64{
			"1-exact": 0.3, // unsure band (< 0.5): must never move.
			"2-exact": 0.6,
			"3-exact": 0.95,
			"0-root":  0.99, // highest of all four, but root-tier: must stay last.
		}),
	})

	got, err := ObligationsForOpts(ctx, st, "svc", rerankTestFile, 0, ObligationsOptions{Decider: mock, RankWithinTier: true})
	if err != nil {
		t.Fatalf("ObligationsForOpts (reranked): %v", err)
	}
	if got.RankedBy != "mock" {
		t.Errorf("RankedBy = %q, want %q", got.RankedBy, "mock")
	}
	if got.RankNote != "" {
		t.Errorf("RankNote = %q, want empty on success", got.RankNote)
	}
	if len(got.Rules) != len(none.Rules) || got.Displaced != none.Displaced {
		t.Fatalf("reranked = %d rules, displaced %d; want same as none path (%d, %d): %+v",
			len(got.Rules), got.Displaced, len(none.Rules), none.Displaced, got.Rules)
	}

	wantOrder := []string{
		"1-exact: requires review before any change.",      // unsure band: unmoved, stays first
		"3-exact: requires review before any change also.", // 0.95: promoted ahead of 2-exact
		"2-exact: requires review before any change too.",  // 0.6: demoted behind 3-exact
		"0-root: applies broadly across the repository.",   // root tier: never outranks exact-file, despite 0.99
	}
	for i, w := range wantOrder {
		if got.Rules[i].Rule != w {
			t.Errorf("rule[%d] = %q, want %q (full order: %+v)", i, got.Rules[i].Rule, w, got.Rules)
		}
	}

	wantRelevance := map[string]float64{
		"1-exact: requires review before any change.":      0.3,
		"2-exact: requires review before any change too.":  0.6,
		"3-exact: requires review before any change also.": 0.95,
		"0-root: applies broadly across the repository.":   0.99,
	}
	for _, r := range got.Rules {
		if r.Relevance == nil {
			t.Errorf("rule %q: Relevance absent, want the scripted noul", r.Rule)
			continue
		}
		if *r.Relevance != wantRelevance[r.Rule] {
			t.Errorf("rule %q: Relevance = %v, want %v", r.Rule, *r.Relevance, wantRelevance[r.Rule])
		}
	}
}

// TestObligations_RankerErrorIsUntouched is DP-7's fail-open acceptance
// test: a Decider that cannot answer (decision.None's ErrNoProvider is the
// simplest real example) never fails the call and never changes served
// order — the error is swallowed into an untouched, RankedBy:"none"
// envelope with RankNote naming why (ADR-042 decision 3).
func TestObligations_RankerErrorIsUntouched(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedServed(t, st, "1-exact", "1-exact rule.", []string{rerankTestFile}, core.ClaimFresh)
	seedServed(t, st, "2-exact", "2-exact rule.", []string{rerankTestFile}, core.ClaimFresh)
	seedServed(t, st, "0-root", "0-root rule.", []string{"**"}, core.ClaimFresh)

	none, err := ObligationsFor(ctx, st, "svc", rerankTestFile, 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}

	got, err := ObligationsForOpts(ctx, st, "svc", rerankTestFile, 0, ObligationsOptions{Decider: decision.None{}, RankWithinTier: true})
	if err != nil {
		t.Fatalf("ObligationsForOpts: %v", err)
	}
	if got.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q", got.RankedBy, "none")
	}
	if got.RankNote == "" {
		t.Errorf("RankNote = %q, want a note naming the provider error", got.RankNote)
	}
	if !strings.Contains(got.RankNote, decision.ErrNoProvider.Error()) {
		t.Errorf("RankNote = %q, want it to name %v", got.RankNote, decision.ErrNoProvider)
	}
	if len(got.Rules) != len(none.Rules) {
		t.Fatalf("reranked = %d rules, want %d (untouched)", len(got.Rules), len(none.Rules))
	}
	for i := range none.Rules {
		if got.Rules[i].Rule != none.Rules[i].Rule {
			t.Errorf("rule[%d] = %q, want %q (order must be untouched): got=%+v none=%+v", i, got.Rules[i].Rule, none.Rules[i].Rule, got.Rules, none.Rules)
		}
		if got.Rules[i].Relevance != nil {
			t.Errorf("rule[%d] Relevance = %v, want nil under a swallowed provider error", i, *got.Rules[i].Relevance)
		}
	}
}
