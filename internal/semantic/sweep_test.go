package semantic

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedClaim writes one claim + its citation link built against docContent.
func seedClaim(t *testing.T, st store.Store, id, subject string, status core.ClaimStatus, docPath string, docContent []byte, start, end int) {
	t.Helper()
	hash, ok := core.HashLines(docContent, start, end)
	if !ok {
		t.Fatalf("seed cite %d-%d does not resolve", start, end)
	}
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: subject, Rule: "rule for " + id, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d-" + id,
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: core.CiteRef(docPath, start, end), TargetHash: hash}}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

// seedClaimAnchored is seedClaim plus the compile-time AnchorText link field
// (T5-1/O13): a real compile stores the literal cited text alongside its
// hash (core.LinesText, same normalization as HashLines) so the sweep can
// re-anchor on drift instead of only detecting it. seedClaim (above) stays
// hash-only deliberately — it models the "legacy" claims compiled before
// AnchorText existed, which drift tests must show can only ever go stale.
func seedClaimAnchored(t *testing.T, st store.Store, id, subject string, status core.ClaimStatus, docPath string, docContent []byte, start, end int) {
	t.Helper()
	hash, ok := core.HashLines(docContent, start, end)
	if !ok {
		t.Fatalf("seed cite %d-%d does not resolve", start, end)
	}
	text, ok := core.LinesText(docContent, start, end)
	if !ok {
		t.Fatalf("seed cite %d-%d text does not resolve", start, end)
	}
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: subject, Rule: "rule for " + id, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d-" + id,
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: core.CiteRef(docPath, start, end), TargetHash: hash, AnchorText: text}}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

// TestSweep_ReanchorsOnUpstreamLineShift is T5-1/O13's exact reproduction
// shape (docs/dogfood-observations.md O13, docs/plans/trust-and-tokenomics.md
// T5-1): obligations(internal/extractor/goextract.go) served a rule cited at
// CLAUDE.md#L89-L90 with NO status flag, but SS-8's prose insertions had
// already shifted the rule's real text down to L94 — the cited lines held
// DIFFERENT text than the served rule, and a pure hash check has no way to
// tell "the rule moved" from "the rule was rewritten." This fixture
// reproduces the shift and proves both branches of the fix: an upstream
// insertion ABOVE the rule heals via re-anchor with an audit event; deleting
// the rule's own text (hash-indistinguishable from the shift case) correctly
// falls through to stale, never guessing. A third case proves the honesty
// half of the design: a legacy hash-only citation (no stored anchor text)
// hits the exact same shift and can only stale, never heal.
func TestSweep_ReanchorsOnUpstreamLineShift(t *testing.T) {
	ctx := context.Background()

	doc := []byte(strings.Join([]string{
		"# CLAUDE.md",
		"",
		"## Conventions",
		"",
		"- Extractor: implement extractor.Extractor, register by extension.",
		"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
	}, "\n") + "\n")
	ruleStart, ruleEnd := 5, 5 // the "- Extractor: ..." bullet

	// Upstream insertion: two prose lines land ABOVE the rule (the O13
	// shape — SS-8's keystone/CUE prose insertions), shifting the rule's own
	// UNCHANGED text down by two lines.
	shifted := []byte(strings.Join([]string{
		"# CLAUDE.md",
		"",
		"## Conventions",
		"",
		"Some new keystone prose inserted above the bullets (SS-8-shaped).",
		"More prose.",
		"- Extractor: implement extractor.Extractor, register by extension.",
		"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
	}, "\n") + "\n")

	t.Run("upstream insertion heals via re-anchor", func(t *testing.T) {
		st := store.NewMemoryStore()
		seedClaimAnchored(t, st, "c-shift", "CLAUDE.md#extractor", core.ClaimFresh, "CLAUDE.md", doc, ruleStart, ruleEnd)

		fixed := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
		rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return shifted, true }, func() time.Time { return fixed })
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if rep.ClaimsChecked != 1 || rep.DriftDetected != 1 || rep.ReAnchored != 1 || rep.Fresh != 1 || rep.Stale != 0 {
			t.Fatalf("report %+v, want checked=1 drift=1 reanchored=1 fresh=1 stale=0", rep)
		}
		if status := statusByID(t, st)["c-shift"]; status != core.ClaimFresh {
			t.Fatalf("c-shift = %s, want fresh (healed)", status)
		}

		links, err := st.ClaimLinks(ctx, "svc", []string{"c-shift"})
		if err != nil {
			t.Fatalf("ClaimLinks: %v", err)
		}
		if len(links) != 1 {
			t.Fatalf("expected 1 cites link, got %d: %+v", len(links), links)
		}
		wantRef := core.CiteRef("CLAUDE.md", 7, 7) // shifted down by 2 lines
		if links[0].TargetRef != wantRef {
			t.Errorf("citation not re-anchored: got %q, want %q", links[0].TargetRef, wantRef)
		}
		wantHash, _ := core.HashLines(shifted, 7, 7)
		if links[0].TargetHash != wantHash {
			t.Errorf("re-anchored hash = %q, want %q (fresh hash at the new location)", links[0].TargetHash, wantHash)
		}
		if links[0].AnchorText == "" {
			t.Errorf("re-anchor must not clear AnchorText")
		}

		events, err := st.ClaimEvents(ctx, "c-shift")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 re-anchored event, got %d: %+v", len(events), events)
		}
		if events[0].Verdict != "re-anchored" || events[0].Author != "sweep" {
			t.Errorf("event = %+v, want verdict=re-anchored author=sweep", events[0])
		}
		if !events[0].At.Equal(fixed) {
			t.Errorf("event.At = %v, want injected clock value %v", events[0].At, fixed)
		}
		if !strings.Contains(events[0].Note, "L5-L5") || !strings.Contains(events[0].Note, "L7-L7") {
			t.Errorf("event note = %q, want it to record the old->new line range", events[0].Note)
		}
	})

	t.Run("rule text deleted falls through to stale, not healed", func(t *testing.T) {
		st := store.NewMemoryStore()
		seedClaimAnchored(t, st, "c-deleted", "CLAUDE.md#extractor", core.ClaimFresh, "CLAUDE.md", doc, ruleStart, ruleEnd)

		// The rule's own text is GONE — rewritten, not shifted. Zero matches
		// anywhere in the file: the sweep must not guess, must go stale.
		rewritten := []byte(strings.Join([]string{
			"# CLAUDE.md",
			"",
			"## Conventions",
			"",
			"- Extractor: totally different guidance now, old bullet is gone.",
			"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
		}, "\n") + "\n")

		rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return rewritten, true }, nil)
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if rep.DriftDetected != 1 || rep.ReAnchored != 0 || rep.Stale != 1 || rep.Fresh != 0 {
			t.Fatalf("report %+v, want drift=1 reanchored=0 stale=1 fresh=0", rep)
		}
		if status := statusByID(t, st)["c-deleted"]; status != core.ClaimStale {
			t.Fatalf("c-deleted = %s, want stale (no re-anchor target exists)", status)
		}
		events, err := st.ClaimEvents(ctx, "c-deleted")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected no event on a plain stale flip (only contradicted/re-anchored transitions append events), got %+v", events)
		}
	})

	t.Run("ambiguous anchor match falls through to stale", func(t *testing.T) {
		st := store.NewMemoryStore()
		seedClaimAnchored(t, st, "c-ambiguous", "CLAUDE.md#extractor", core.ClaimFresh, "CLAUDE.md", doc, ruleStart, ruleEnd)

		// Same upstream shift, but the rule's text now ALSO appears a second
		// time elsewhere in the file (e.g. a bad merge duplicated it): no
		// unambiguous re-anchor target exists, so this must behave exactly
		// like the zero-match case — stale, never a guess.
		duplicated := []byte(strings.Join([]string{
			"# CLAUDE.md",
			"",
			"## Conventions",
			"",
			"Some new keystone prose inserted above the bullets.",
			"- Extractor: implement extractor.Extractor, register by extension.",
			"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
			"- Extractor: implement extractor.Extractor, register by extension.",
		}, "\n") + "\n")

		rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return duplicated, true }, nil)
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if rep.DriftDetected != 1 || rep.ReAnchored != 0 || rep.Stale != 1 {
			t.Fatalf("report %+v, want drift=1 reanchored=0 stale=1 (ambiguous match must not guess)", rep)
		}
	})

	t.Run("legacy hash-only citation cannot heal, only stale", func(t *testing.T) {
		st := store.NewMemoryStore()
		seedClaim(t, st, "c-legacy", "CLAUDE.md#extractor", core.ClaimFresh, "CLAUDE.md", doc, ruleStart, ruleEnd)

		rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return shifted, true }, nil)
		if err != nil {
			t.Fatalf("Sweep: %v", err)
		}
		if rep.DriftDetected != 1 || rep.ReAnchored != 0 || rep.Stale != 1 {
			t.Fatalf("report %+v, want drift=1 reanchored=0 stale=1 (no stored anchor text to heal from)", rep)
		}
		if status := statusByID(t, st)["c-legacy"]; status != core.ClaimStale {
			t.Fatalf("c-legacy = %s, want stale", status)
		}
	})

	// Critic probe 1 (committed as coverage): a heal must be terminal for
	// that drift — re-sweeping the SAME (unchanged) corpus right after a
	// re-anchor must find the updated citation hash-matching at its new
	// range: zero drift, zero additional heal, and — the audit-noise
	// contract — no second re-anchored event.
	t.Run("heal then resweep is idempotent", func(t *testing.T) {
		st := store.NewMemoryStore()
		seedClaimAnchored(t, st, "c-idem", "CLAUDE.md#extractor", core.ClaimFresh, "CLAUDE.md", doc, ruleStart, ruleEnd)

		fetch := func(string) ([]byte, bool) { return shifted, true }
		rep1, err := Sweep(ctx, st, "svc", fetch, nil)
		if err != nil {
			t.Fatalf("first Sweep: %v", err)
		}
		if rep1.ReAnchored != 1 {
			t.Fatalf("first sweep report %+v, want reanchored=1", rep1)
		}

		rep2, err := Sweep(ctx, st, "svc", fetch, nil)
		if err != nil {
			t.Fatalf("second Sweep: %v", err)
		}
		if rep2.DriftDetected != 0 || rep2.ReAnchored != 0 || rep2.Fresh != 1 || rep2.Flipped != 0 {
			t.Fatalf("resweep report %+v, want drift=0 reanchored=0 fresh=1 flipped=0 (heal must be terminal)", rep2)
		}
		events, err := st.ClaimEvents(ctx, "c-idem")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected exactly 1 re-anchored event after heal+resweep, got %d: %+v", len(events), events)
		}
	})

	// Critic probe 2 (committed as coverage): a MULTI-LINE anchor healed
	// once must be healable AGAIN on a later, further shift — i.e. the
	// re-anchor must retain AnchorText and store the fresh hash at the new
	// range (stored hash == recomputed hash of the new location), or the
	// second drift could never re-anchor.
	t.Run("multi-line anchor double-heals across successive shifts", func(t *testing.T) {
		st := store.NewMemoryStore()
		// A two-line citation: the extractor + analyzer bullets together.
		seedClaimAnchored(t, st, "c-multi", "CLAUDE.md#adding", core.ClaimFresh, "CLAUDE.md", doc, 5, 6)

		// First shift: one line inserted above → the pair moves to 6-7.
		shift1 := []byte(strings.Join([]string{
			"# CLAUDE.md",
			"",
			"## Conventions",
			"",
			"Inserted prose, round one.",
			"- Extractor: implement extractor.Extractor, register by extension.",
			"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
		}, "\n") + "\n")
		rep1, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return shift1, true }, nil)
		if err != nil {
			t.Fatalf("first Sweep: %v", err)
		}
		if rep1.ReAnchored != 1 || rep1.Fresh != 1 {
			t.Fatalf("first sweep report %+v, want reanchored=1 fresh=1", rep1)
		}
		links, err := st.ClaimLinks(ctx, "svc", []string{"c-multi"})
		if err != nil || len(links) != 1 {
			t.Fatalf("ClaimLinks after first heal: err=%v links=%+v", err, links)
		}
		if links[0].TargetRef != core.CiteRef("CLAUDE.md", 6, 7) {
			t.Fatalf("first heal ref = %q, want CLAUDE.md#L6-L7", links[0].TargetRef)
		}
		if links[0].AnchorText == "" {
			t.Fatal("first heal cleared AnchorText — the second drift below could never re-anchor")
		}
		wantHash1, _ := core.HashLines(shift1, 6, 7)
		if links[0].TargetHash != wantHash1 {
			t.Fatalf("first heal stored hash %q, want the fresh hash at the new range %q", links[0].TargetHash, wantHash1)
		}

		// Second shift: two MORE lines inserted above → the pair moves to 8-9.
		shift2 := []byte(strings.Join([]string{
			"# CLAUDE.md",
			"",
			"## Conventions",
			"",
			"Inserted prose, round one.",
			"Inserted prose, round two.",
			"And a third line.",
			"- Extractor: implement extractor.Extractor, register by extension.",
			"- Analyzer: a pure internal/analysis/<name> package, deterministic.",
		}, "\n") + "\n")
		rep2, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return shift2, true }, nil)
		if err != nil {
			t.Fatalf("second Sweep: %v", err)
		}
		if rep2.ReAnchored != 1 || rep2.Fresh != 1 || rep2.Stale != 0 {
			t.Fatalf("second sweep report %+v, want reanchored=1 fresh=1 stale=0 (double-heal)", rep2)
		}
		links, err = st.ClaimLinks(ctx, "svc", []string{"c-multi"})
		if err != nil || len(links) != 1 {
			t.Fatalf("ClaimLinks after second heal: err=%v links=%+v", err, links)
		}
		if links[0].TargetRef != core.CiteRef("CLAUDE.md", 8, 9) {
			t.Fatalf("second heal ref = %q, want CLAUDE.md#L8-L9", links[0].TargetRef)
		}
		wantHash2, _ := core.HashLines(shift2, 8, 9)
		if links[0].TargetHash != wantHash2 {
			t.Fatalf("second heal stored hash %q, want the fresh hash at the new range %q", links[0].TargetHash, wantHash2)
		}
		events, err := st.ClaimEvents(ctx, "c-multi")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 re-anchored events (one per heal), got %d: %+v", len(events), events)
		}
		if status := statusByID(t, st)["c-multi"]; status != core.ClaimFresh {
			t.Errorf("c-multi = %s, want fresh through both heals", status)
		}
	})
}

func TestSweep_FlipsAndHeals(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	doc := []byte("line one\nline two\nline three\n")

	seedClaim(t, st, "c-fresh", "d.md#a", core.ClaimFresh, "d.md", doc, 1, 1)
	seedClaim(t, st, "c-drift", "d.md#b", core.ClaimFresh, "d.md", doc, 2, 2)
	seedClaim(t, st, "c-gone", "gone.md#a", core.ClaimFresh, "gone.md", []byte("x\n"), 1, 1)
	seedClaim(t, st, "c-superseded", "d.md#c", core.ClaimSuperseded, "d.md", doc, 3, 3)

	// Current corpus: d.md line 2 edited; gone.md deleted.
	edited := []byte("line one\nline TWO edited\nline three\n")
	fetch := func(path string) ([]byte, bool) {
		if path == "d.md" {
			return edited, true
		}
		return nil, false
	}

	rep, err := Sweep(ctx, st, "svc", fetch, nil)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.ClaimsChecked != 3 {
		t.Errorf("checked %d claims, want 3 (superseded is terminal, never swept)", rep.ClaimsChecked)
	}
	if rep.Fresh != 1 || rep.Stale != 1 || rep.Contradicted != 1 || rep.Flipped != 2 {
		t.Errorf("report %+v, want fresh/stale/contradicted 1/1/1, flipped 2", rep)
	}

	status := statusByID(t, st)
	if status["c-fresh"] != core.ClaimFresh {
		t.Errorf("c-fresh = %s", status["c-fresh"])
	}
	if status["c-drift"] != core.ClaimStale {
		t.Errorf("c-drift = %s, want stale (cited line changed)", status["c-drift"])
	}
	if status["c-gone"] != core.ClaimContradicted {
		t.Errorf("c-gone = %s, want contradicted (file gone)", status["c-gone"])
	}
	if status["c-superseded"] != core.ClaimSuperseded {
		t.Errorf("c-superseded = %s, must not be touched", status["c-superseded"])
	}

	// Healing: restore the original doc → the stale claim goes fresh again.
	rep2, err := Sweep(ctx, st, "svc", func(path string) ([]byte, bool) {
		if path == "d.md" {
			return doc, true
		}
		return nil, false
	}, nil)
	if err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if rep2.Flipped != 1 {
		t.Errorf("second sweep flipped %d, want 1 (stale→fresh heal)", rep2.Flipped)
	}
	if s := statusByID(t, st)["c-drift"]; s != core.ClaimFresh {
		t.Errorf("c-drift after restore = %s, want fresh", s)
	}
}

func TestSweep_OutOfRangeCitationContradicts(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	doc := []byte(strings.Repeat("l\n", 10)) // 10 lines
	seedClaim(t, st, "c-range", "d.md#a", core.ClaimFresh, "d.md", doc, 9, 10)

	// Doc shrinks to 3 lines: the range no longer resolves.
	rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return []byte("a\nb\nc"), true }, nil)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.Contradicted != 1 {
		t.Errorf("report %+v, want 1 contradicted", rep)
	}
	if s := statusByID(t, st)["c-range"]; s != core.ClaimContradicted {
		t.Errorf("c-range = %s, want contradicted", s)
	}
}

// TestSweep_ContradictedRecordsContradictedEvent is D2-2 (docs/plans/
// expert-depth.md) plus P1-9's vocabulary fix (docs/plans/
// pulled-tools-excellence.md): an anchor-dead claim (citation file gone)
// must not just flip status silently — it must ALSO append a
// ClaimEvent{Verdict:"contradicted", Author:"sweep"} so the machine judgment
// has the same append-only audit trail a human `fathom claim refute` would
// leave for ITS verdict. The event's Verdict string is deliberately
// "contradicted", not "refuted": refuted is a human/deliberate terminal
// judgment (core.ClaimRefuted, never served again); contradicted is this
// machine-detected, anchor-dead state, which stays served with the status
// label attached (core.ClaimStatus.Served() includes ClaimContradicted) —
// conflating the two vocabularies would make the event log claim a
// never-served verdict for a claim callers can still see. A second sweep
// over the SAME dead anchor (still contradicted, no transition) must NOT
// append a second event — one event per verdict, not unbounded
// re-affirmation noise.
func TestSweep_ContradictedRecordsContradictedEvent(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedClaim(t, st, "c-dead", "gone.md#a", core.ClaimFresh, "gone.md", []byte("x\n"), 1, 1)

	fixed := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixed }

	rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return nil, false }, clock)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.Contradicted != 1 || rep.Flipped != 1 {
		t.Fatalf("report %+v, want contradicted=1 flipped=1", rep)
	}
	if s := statusByID(t, st)["c-dead"]; s != core.ClaimContradicted {
		t.Fatalf("c-dead = %s, want contradicted", s)
	}

	events, err := st.ClaimEvents(ctx, "c-dead")
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 contradicted event, got %d: %+v", len(events), events)
	}
	if events[0].Verdict != "contradicted" || events[0].Author != "sweep" {
		t.Errorf("event = %+v, want verdict=contradicted author=sweep", events[0])
	}
	if !events[0].At.Equal(fixed) {
		t.Errorf("event.At = %v, want injected clock value %v", events[0].At, fixed)
	}

	// Second sweep: still dead, no status transition — must not append a
	// second event.
	rep2, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return nil, false }, clock)
	if err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if rep2.Flipped != 0 {
		t.Errorf("second sweep flipped %d, want 0 (already contradicted)", rep2.Flipped)
	}
	events2, err := st.ClaimEvents(ctx, "c-dead")
	if err != nil {
		t.Fatalf("ClaimEvents after second sweep: %v", err)
	}
	if len(events2) != 1 {
		t.Errorf("expected still 1 event after a no-op sweep, got %d: %+v", len(events2), events2)
	}
}

func statusByID(t *testing.T, st store.Store) map[string]core.ClaimStatus {
	t.Helper()
	claims, err := st.ClaimsForService(context.Background(), "svc", "")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	out := map[string]core.ClaimStatus{}
	for _, cl := range claims {
		out[cl.ClaimID] = cl.Status
	}
	return out
}

// ─── FT-6 DriftedFindings: bounded fingerprint reads on the serve hot path ───

// countingFPStore wraps MemoryStore, counting Fingerprint reads.
type countingFPStore struct {
	*store.MemoryStore
	fpCalls int
}

func (c *countingFPStore) Fingerprint(ctx context.Context, serviceID, sourceRef string) (string, bool, error) {
	c.fpCalls++
	return c.MemoryStore.Fingerprint(ctx, serviceID, sourceRef)
}

// seedFindingWithScopeFP writes one finding-kind claim whose scope-fp links
// anchor the given files at the given fingerprint values.
func seedFindingWithScopeFP(t *testing.T, st store.Store, id string, anchors map[string]string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerFindings, LayerVersion: "v1", Kind: KindFinding,
		Subject: "findings/" + id + ".md#s", Rule: "finding " + id, Scope: []string{"**/*.go"},
		Tier: core.TierSMEStated, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: findingsPromptVersion(), ReproKey: id, InputDigest: "d-" + id,
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: "findings/" + id + ".md#L1-L1", TargetHash: "h"}}
	for f, fp := range anchors {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopeFP, TargetRef: f, TargetHash: fp})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

// TestDriftedFindings_QueriesEachFileOnce: DriftedFindings runs on EVERY
// ObligationsFor serve (the edit-hook hot path). N claims whose broad-but-
// legal scopes anchor the same files must not multiply store reads — each
// distinct file is fingerprint-queried at most once per call.
func TestDriftedFindings_QueriesEachFileOnce(t *testing.T) {
	ctx := context.Background()
	st := &countingFPStore{MemoryStore: store.NewMemoryStore()}

	shared := map[string]string{"mod/a.go": "fp-a", "mod/b.go": "fp-b"}
	for f, fp := range shared {
		if err := st.SetFingerprint(ctx, "svc", f, fp); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	// Three finding claims all anchoring the SAME two files.
	seedFindingWithScopeFP(t, st, "f1", shared)
	seedFindingWithScopeFP(t, st, "f2", shared)
	seedFindingWithScopeFP(t, st, "f3", shared)

	drifted, err := DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("drifted = %v, want none (fingerprints unchanged)", drifted)
	}
	if st.fpCalls > 2 {
		t.Errorf("Fingerprint queried %d times for 2 distinct files across 3 claims, want ≤2 (per-call cache)", st.fpCalls)
	}

	// Sanity: drift detection still works through the cache — change one file.
	if err := st.SetFingerprint(ctx, "svc", "mod/a.go", "fp-a-changed"); err != nil {
		t.Fatalf("SetFingerprint change: %v", err)
	}
	drifted, err = DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings after change: %v", err)
	}
	if len(drifted) != 3 || !drifted["f1"] || !drifted["f2"] || !drifted["f3"] {
		t.Errorf("drifted = %v, want all three claims (shared anchor changed)", drifted)
	}
}

// TestDriftedFindings_SurvivesBrokenProjectPack is critic finding 1 on the
// LC slice: layerIsDriftTracked resolves through the union loader, and a
// half-authored project pack under the service's .fathom/layers/ used to
// hard-fail the WHOLE union — silently disabling code_drifted for the
// embedded findings layer too. With the lenient union (packs.go), drift
// tracking for embedded findings must keep working while an invalid pack
// dir is present.
func TestDriftedFindings_SurvivesBrokenProjectPack(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// The service's root contains a broken pack — exactly the state a
	// half-finished `mkdir .fathom/layers/wip` leaves a repo in.
	root := t.TempDir()
	brokenDir := root + "/.fathom/layers/wip"
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(brokenDir+"/layer.cue", []byte("not { valid ] cue"), 0o644); err != nil {
		t.Fatalf("write broken layer.cue: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	// A findings claim whose scoped code HAS drifted.
	if err := st.SetFingerprint(ctx, "svc", "mod/a.go", "fp-a-changed"); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	seedFindingWithScopeFP(t, st, "f1", map[string]string{"mod/a.go": "fp-a-original"})

	drifted, err := DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings with a broken project pack present: %v", err)
	}
	if !drifted["f1"] {
		t.Fatalf("drifted = %v, want f1 — a broken, unrelated project pack silently disabled drift tracking for the embedded findings layer", drifted)
	}
}
