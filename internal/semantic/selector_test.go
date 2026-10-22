package semantic

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/logtemplate"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestSelectorRanksByScopeEvidence pins RS-1 (ADR-077 d2): each scope
// evidence tier outranks the next (a doc_links row, a contract key or log
// template, a path glob naming the edited path with the most specific first,
// a repo-wide glob), ties keep today's order, a heuristic link only ranks,
// and the output is deterministic.
func TestSelectorRanksByScopeEvidence(t *testing.T) {
	const file = "internal/store/backend.go"
	mk := func(id, rule string, scope ...string) scored {
		return scored{
			cl:          core.SemanticClaim{ClaimID: id, Rule: rule, Status: core.ClaimFresh, Scope: scope},
			specificity: len(scope[0]),
			governing:   rankByGoverningScope(scope, file),
		}
	}
	cands := []scored{
		mk("repo-b", "Rule B, repo-wide.", "**"),
		mk("parent", "A rule for everything under internal.", "internal/**"),
		mk("key", "Log `opened store at %s` once per open.", "**"),
		mk("pkg", "A rule for the store package.", "internal/store/**"),
		mk("repo-a", "Rule A, repo-wide.", "**"),
		mk("exact", "A rule for this one file.", file),
		mk("doc", "A doc-linked rule.", "**"),
	}
	// A repo-wide rule with STRONG lexical relevance to the file no longer
	// leapfrogs a path glob that names the file (tier 4 never outranks tier 3).
	for i := range cands {
		if cands[i].cl.ClaimID == "repo-b" {
			cands[i].relevance, cands[i].strong = 9, true
		}
	}
	facts := scopeFacts{docLinked: map[string]bool{"doc": true}, keys: []string{"opened store at %s"}}
	less := func(a, b scored) bool { l, _ := compareScored(a, b, file, nil); return l }
	text := func(m scored) string { return claimRelevanceText(m.cl) }

	ids := func(ms []scored) []string {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.cl.ClaimID
		}
		return out
	}
	// repo-b's strong relevance still decides INSIDE its tier: it ties
	// repo-a on evidence, and today's order (relevance) puts it first.
	want := []string{"doc", "key", "exact", "pkg", "parent", "repo-b", "repo-a"}
	got := ids(rankByScopeEvidence(append([]scored(nil), cands...), file, facts, text, less))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}

	// Deterministic: every rotation of the input ranks the same.
	for r := 1; r < len(cands); r++ {
		rot := append(append([]scored(nil), cands[r:]...), cands[:r]...)
		if got := ids(rankByScopeEvidence(rot, file, facts, text, less)); !reflect.DeepEqual(got, want) {
			t.Fatalf("rotation %d: order = %v, want %v", r, got, want)
		}
	}

	// Rank only: with no doc link and no key on the file, the same
	// candidates come back, none dropped; "doc" and "key" fall to tier 4.
	bare := ids(rankByScopeEvidence(append([]scored(nil), cands...), file, scopeFacts{}, text, less))
	if len(bare) != len(cands) {
		t.Fatalf("a missing link dropped a candidate: %v", bare)
	}
	sorted := append([]string(nil), bare...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(sorted, []string{"doc", "exact", "key", "parent", "pkg", "repo-a", "repo-b"}) {
		t.Fatalf("candidate set changed: %v", sorted)
	}
	if bare[0] != "exact" || bare[len(bare)-1] == "parent" {
		t.Fatalf("without facts the path globs lead: %v", bare)
	}

	// Freshness still decides before evidence (BP-17): a stale exact-file
	// rule ranks below a fresh repo-wide one.
	stale := mk("stale-exact", "Old exact rule.", file)
	stale.cl.Status = core.ClaimStale
	fresh := mk("fresh-repo", "Fresh repo rule.", "**")
	if got := ids(rankByScopeEvidence([]scored{stale, fresh}, file, scopeFacts{}, text, less)); got[0] != "fresh-repo" {
		t.Fatalf("stale outranked fresh: %v", got)
	}

	// Through serving: a log template the edited file's own observations
	// carry ties a repo-wide rule to the edit, and it wins a slot over six
	// package rules that would otherwise fill the cap.
	ctx := context.Background()
	st := store.NewMemoryStore()
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5", "p6"} {
		seedServed(t, st, id, "Package rule "+id+".", []string{"internal/store/**"}, core.ClaimFresh)
	}
	seedServed(t, st, "tmpl", "Never change the `opened store at %s` log line; dashboards parse it.", []string{"**"}, core.ClaimFresh)
	before, err := ObligationsFor(ctx, st, "svc", file, 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	for _, r := range before.Rules {
		if r.ClaimID == "tmpl" {
			t.Fatalf("without the template fact the repo-wide rule should lose the cap: %+v", before.Rules)
		}
	}
	obs := logtemplate.ToObservation("svc", logtemplate.Fact{File: file, Line: 12, API: "log.Printf", Text: "opened store at %s", Package: "store"}, "h", time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), false)
	if err := st.PutObservations(ctx, []core.Observation{obs}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	after, err := ObligationsFor(ctx, st, "svc", file, 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(after.Rules) == 0 || after.Rules[0].ClaimID != "tmpl" {
		t.Fatalf("the template-tied rule should lead: %+v", after.Rules)
	}
	if after.Displaced != before.Displaced {
		t.Fatalf("ranking changed the eligible count: displaced %d then %d", before.Displaced, after.Displaced)
	}
}
