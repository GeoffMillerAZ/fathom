package semantic

// decisionrank_test.go — DP-7/DP-8 (docs/plans/decision-port.md, ADR-042
// decision 2): the obligations re-rank pinned against the tier law it must
// never break.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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

// buildMatchedRuleSet builds n scored claims, all in the SAME exact-file
// governing-scope tier, with distinct ClaimIDs/Rule text
// ("rule000".."rule<n-1>") so a test can seed a MockDecider entry per chunk
// and inspect exactly which claims moved.
func buildMatchedRuleSet(n int) []scored {
	out := make([]scored, n)
	for i := range out {
		id := fmt.Sprintf("rule%03d", i)
		out[i] = scored{
			cl:        core.SemanticClaim{ClaimID: id, Rule: id, Status: core.ClaimFresh},
			governing: governingScope{tier: 0},
		}
	}
	return out
}

// TestRerankWithinTiers_ChunksByDeciderName is DP-22's own acceptance test
// (O109): 45 rules split into 3 chunks (20/20/5) of noul questions under a
// Decider named "llm" (decisionBatchSize's case 2), and go out as ONE call
// under a Decider decisionBatchSize does not recognize (case 3, unbounded).
func TestRerankWithinTiers_ChunksByDeciderName(t *testing.T) {
	ctx := context.Background()
	matched := buildMatchedRuleSet(45)

	t.Run("llm-named mock: 3 chunks", func(t *testing.T) {
		chunkSizes := []int{20, 20, 5}
		entries := make([]decision.MockEntry, len(chunkSizes))
		idx := 0
		for ci, n := range chunkSizes {
			answers := make(map[string]decision.Answer, n)
			for j := 0; j < n; j++ {
				answers[matched[idx].cl.ClaimID] = decision.Answer{ID: matched[idx].cl.ClaimID, Kind: decision.KindNoul, Noul: 0.9}
				idx++
			}
			entries[ci] = decision.MockEntry{Response: decision.Response{Provider: "llm", Answers: answers}}
		}
		md := decision.NewMockDecider("llm", entries...)

		oc := rerankWithinTiers(ctx, md, rerankTestFile, "", matched)
		if oc.ChunksTotal != 3 {
			t.Errorf("ChunksTotal = %d, want 3", oc.ChunksTotal)
		}
		if oc.ChunksFailed != 0 {
			t.Errorf("ChunksFailed = %d, want 0", oc.ChunksFailed)
		}
		if !oc.Ranked {
			t.Error("Ranked = false, want true")
		}
		calls := md.Calls()
		if len(calls) != 3 {
			t.Fatalf("Decide called %d times, want 3", len(calls))
		}
		gotSizes := make([]int, len(calls))
		for i, c := range calls {
			gotSizes[i] = len(c.Questions)
		}
		wantSizes := map[int]bool{20: true, 5: true}
		total := 0
		for _, s := range gotSizes {
			if !wantSizes[s] {
				t.Errorf("chunk size %d not among {20,20,5}", s)
			}
			total += s
		}
		if total != 45 {
			t.Errorf("total questions across chunks = %d, want 45", total)
		}
	})

	t.Run("unrecognized-name mock: 1 call", func(t *testing.T) {
		answers := make(map[string]decision.Answer, len(matched))
		for _, m := range matched {
			answers[m.cl.ClaimID] = decision.Answer{ID: m.cl.ClaimID, Kind: decision.KindNoul, Noul: 0.9}
		}
		md := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{Provider: "mock", Answers: answers}})

		oc := rerankWithinTiers(ctx, md, rerankTestFile, "", matched)
		if oc.ChunksTotal != 1 {
			t.Errorf("ChunksTotal = %d, want 1", oc.ChunksTotal)
		}
		if calls := md.Calls(); len(calls) != 1 {
			t.Fatalf("Decide called %d times, want 1", len(calls))
		} else if len(calls[0].Questions) != 45 {
			t.Errorf("single call carried %d questions, want 45", len(calls[0].Questions))
		}
	})

	t.Run("MaxQuestions()==0 opts in but stays unbounded: 1 call", func(t *testing.T) {
		md := &fixedBatchDecider{name: "llm-variant", max: 0}
		oc := rerankWithinTiers(ctx, md, rerankTestFile, "", matched)
		if oc.ChunksTotal != 1 {
			t.Errorf("ChunksTotal = %d, want 1", oc.ChunksTotal)
		}
		if md.calls != 1 {
			t.Errorf("Decide called %d times, want 1", md.calls)
		}
	})
}

// fixedBatchDecider is a minimal decision.Decider (and maxQuestionser) test
// double: every Decide call succeeds trivially (every question answered
// noul 0.9), and MaxQuestions() reports max — used to exercise
// decisionBatchSize's case 1 (an explicit opt-in) independent of
// decision.MockDecider, which has no way to implement an extra method.
type fixedBatchDecider struct {
	mu    sync.Mutex
	name  string
	max   int
	calls int
}

func (f *fixedBatchDecider) Name() string      { return f.name }
func (f *fixedBatchDecider) MaxQuestions() int { return f.max }
func (f *fixedBatchDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	answers := make(map[string]decision.Answer, len(req.Questions))
	for _, q := range req.Questions {
		answers[q.ID] = decision.Answer{ID: q.ID, Kind: decision.KindNoul, Noul: 0.9}
	}
	return decision.Response{Provider: f.name, Answers: answers}, nil
}

// TestRerankWithinTiers_PartialChunkFailureKeepsOthersRankedAndNotesCount
// (DP-22, O109): 45 rules in 3 chunks (20/20/5) under an "llm"-named mock;
// the MIDDLE chunk (rules 20..39) hard-fails. The other two chunks' rules
// are still genuinely reranked (their ClaimIDs carry a Relevance answer);
// the failed chunk's 20 rules are left at their EXACT original index
// positions, unmoved and unanswered; Ranked is true (some chunks
// succeeded); Failures names exactly the one failing chunk (20 questions);
// and rerankFailureNote's "N of M chunks failed" text names 1 of 3.
func TestRerankWithinTiers_PartialChunkFailureKeepsOthersRankedAndNotesCount(t *testing.T) {
	ctx := context.Background()
	matched := buildMatchedRuleSet(45)
	boom := errors.New("boom: chunk 2 exploded")

	mkAnswers := func(lo, hi int) map[string]decision.Answer {
		answers := make(map[string]decision.Answer, hi-lo)
		for i := lo; i < hi; i++ {
			answers[matched[i].cl.ClaimID] = decision.Answer{ID: matched[i].cl.ClaimID, Kind: decision.KindNoul, Noul: 0.9}
		}
		return answers
	}
	md := decision.NewMockDecider("llm",
		decision.MockEntry{Response: decision.Response{Provider: "llm", Answers: mkAnswers(0, 20)}},
		decision.MockEntry{Err: boom},
		decision.MockEntry{Response: decision.Response{Provider: "llm", Answers: mkAnswers(40, 45)}},
	)

	oc := rerankWithinTiers(ctx, md, rerankTestFile, "", matched)

	if oc.ChunksTotal != 3 {
		t.Fatalf("ChunksTotal = %d, want 3", oc.ChunksTotal)
	}
	if oc.ChunksFailed != 1 {
		t.Fatalf("ChunksFailed = %d, want 1", oc.ChunksFailed)
	}
	if !oc.Ranked {
		t.Error("Ranked = false, want true (2 of 3 chunks succeeded)")
	}
	if len(oc.Failures) != 1 {
		t.Fatalf("Failures = %d, want 1", len(oc.Failures))
	}
	if oc.Failures[0].Questions != 20 {
		t.Errorf("Failures[0].Questions = %d, want 20", oc.Failures[0].Questions)
	}
	if oc.Failures[0].Site != "obligations" {
		t.Errorf("Failures[0].Site = %q, want %q", oc.Failures[0].Site, "obligations")
	}
	if oc.Failures[0].Provider != "llm" {
		t.Errorf("Failures[0].Provider = %q, want %q", oc.Failures[0].Provider, "llm")
	}
	if !errors.Is(oc.Failures[0].Err, boom) {
		t.Errorf("Failures[0].Err = %v, want %v", oc.Failures[0].Err, boom)
	}

	// The failed chunk's 20 rules are untouched at their original indices.
	for i := 20; i < 40; i++ {
		if oc.Matched[i].cl.ClaimID != matched[i].cl.ClaimID {
			t.Errorf("Matched[%d].ClaimID = %q, want %q (unanswered rule must not move)", i, oc.Matched[i].cl.ClaimID, matched[i].cl.ClaimID)
		}
		if _, ok := oc.Relevance[matched[i].cl.ClaimID]; ok {
			t.Errorf("Relevance carries %q, want absent (its chunk never answered)", matched[i].cl.ClaimID)
		}
	}
	// Every rule from the two succeeding chunks carries a Relevance entry.
	for i := 0; i < 20; i++ {
		if _, ok := oc.Relevance[matched[i].cl.ClaimID]; !ok {
			t.Errorf("Relevance missing %q (its chunk succeeded)", matched[i].cl.ClaimID)
		}
	}
	for i := 40; i < 45; i++ {
		if _, ok := oc.Relevance[matched[i].cl.ClaimID]; !ok {
			t.Errorf("Relevance missing %q (its chunk succeeded)", matched[i].cl.ClaimID)
		}
	}

	note := rerankFailureNote(oc.Failures, oc.ChunksTotal)
	if !strings.Contains(note, "1 of 3 chunks failed") {
		t.Errorf("rerankFailureNote = %q, want it to contain %q", note, "1 of 3 chunks failed")
	}
	if !strings.Contains(note, boom.Error()) {
		t.Errorf("rerankFailureNote = %q, want it to contain %v", note, boom)
	}
}

// TestRerankWithinTiers_AllChunksFailStaysUnranked (DP-22): when every
// chunk fails, Ranked is false (byte-identical to the pre-DP-22 single-
// error contract — the caller must leave RankedBy at "none"), Matched is
// content-identical to the input, and Relevance carries nothing.
func TestRerankWithinTiers_AllChunksFailStaysUnranked(t *testing.T) {
	ctx := context.Background()
	matched := buildMatchedRuleSet(3)
	md := decision.NewMockDecider("llm", decision.MockEntry{Err: decision.ErrNoProvider})

	oc := rerankWithinTiers(ctx, md, rerankTestFile, "", matched)
	if oc.Ranked {
		t.Error("Ranked = true, want false (every chunk failed)")
	}
	if oc.ChunksTotal != 1 || oc.ChunksFailed != 1 {
		t.Errorf("ChunksTotal/Failed = %d/%d, want 1/1", oc.ChunksTotal, oc.ChunksFailed)
	}
	if len(oc.Relevance) != 0 {
		t.Errorf("Relevance = %v, want empty", oc.Relevance)
	}
	for i := range matched {
		if oc.Matched[i].cl.ClaimID != matched[i].cl.ClaimID {
			t.Errorf("Matched[%d] = %q, want %q (untouched)", i, oc.Matched[i].cl.ClaimID, matched[i].cl.ClaimID)
		}
	}
	note := rerankFailureNote(oc.Failures, oc.ChunksTotal)
	if !strings.Contains(note, decision.ErrNoProvider.Error()) {
		t.Errorf("rerankFailureNote = %q, want it to contain %v (bare error text, total==1)", note, decision.ErrNoProvider)
	}
}

// concurrencyProbeDecider is a decision.Decider + maxQuestionser test
// double used only to prove rerankWithinTiers' worker pool genuinely runs
// chunks in PARALLEL (not accidentally serial) and stays bounded at
// rerankConcurrency: each Decide call tracks how many calls are
// simultaneously in flight.
type concurrencyProbeDecider struct {
	mu       sync.Mutex
	inFlight int
	maxSeen  int
	calls    int
	max      int
}

func (p *concurrencyProbeDecider) Name() string      { return "llm" }
func (p *concurrencyProbeDecider) MaxQuestions() int { return p.max }
func (p *concurrencyProbeDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	p.mu.Lock()
	p.inFlight++
	p.calls++
	if p.inFlight > p.maxSeen {
		p.maxSeen = p.inFlight
	}
	p.mu.Unlock()

	time.Sleep(15 * time.Millisecond)

	p.mu.Lock()
	p.inFlight--
	p.mu.Unlock()

	answers := make(map[string]decision.Answer, len(req.Questions))
	for _, q := range req.Questions {
		answers[q.ID] = decision.Answer{ID: q.ID, Kind: decision.KindNoul, Noul: 0.9}
	}
	return decision.Response{Provider: "llm", Answers: answers}, nil
}

// TestRerankWithinTiers_ConcurrencyIsBounded (DP-22): 12 rules chunked to 1
// question per Decide call (12 chunks) must overlap in flight (proving the
// chunks actually run concurrently, not one-at-a-time) while never
// exceeding rerankConcurrency simultaneous calls. Run under -race.
func TestRerankWithinTiers_ConcurrencyIsBounded(t *testing.T) {
	ctx := context.Background()
	matched := buildMatchedRuleSet(12)
	probe := &concurrencyProbeDecider{max: 1}

	oc := rerankWithinTiers(ctx, probe, rerankTestFile, "", matched)
	if !oc.Ranked {
		t.Fatal("Ranked = false, want true")
	}
	if probe.calls != 12 {
		t.Errorf("Decide called %d times, want 12 (1 question per chunk)", probe.calls)
	}
	if probe.maxSeen <= 1 {
		t.Errorf("max concurrent in-flight calls = %d, want > 1 (chunks must run concurrently)", probe.maxSeen)
	}
	if probe.maxSeen > rerankConcurrency {
		t.Errorf("max concurrent in-flight calls = %d, want <= rerankConcurrency (%d)", probe.maxSeen, rerankConcurrency)
	}
}
