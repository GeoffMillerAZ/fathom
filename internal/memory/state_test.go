package memory

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestDriftAtReadTimeCachedPerGeneration proves AM-3: drift is evaluated at
// READ time from hashes fathom already maintains, with no sweep, and the
// evaluation is cached per index generation — forty surfacings of one memory
// hash its anchors ONCE.
func TestDriftAtReadTimeCachedPerGeneration(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")
	m := writeFixture(t, ctx, ms, idx, at)

	// The write itself probed once. Everything after this is read-time.
	probesAfterWrite := idx.probeCount()
	if probesAfterWrite != 1 {
		t.Fatalf("write probed %d times, want 1", probesAfterWrite)
	}

	cache := NewDriftCache()
	for i := 0; i < 40; i++ {
		ev, err := EvaluateState(ctx, idx, cache, fixedClock(at), m)
		if err != nil {
			t.Fatalf("EvaluateState #%d: %v", i, err)
		}
		if ev.State != StateFresh {
			t.Fatalf("EvaluateState #%d state = %q, want %q", i, ev.State, StateFresh)
		}
	}
	if got := idx.probeCount() - probesAfterWrite; got != 1 {
		t.Errorf("40 surfacings hashed the anchor %d times, want 1", got)
	}

	// A new generation must invalidate the cache: the whole point of
	// keying on the generation is that a re-ingest is seen immediately,
	// with no sweep.
	idx.set("symbol:parseConfig", "hash-v2")
	idx.setGeneration("gen-2")
	ev, err := EvaluateState(ctx, idx, cache, fixedClock(at), m)
	if err != nil {
		t.Fatalf("EvaluateState after generation bump: %v", err)
	}
	if ev.State != StateSuspect {
		t.Errorf("state after content change = %q, want %q — a cached probe outlived its generation", ev.State, StateSuspect)
	}
	if got := idx.probeCount() - probesAfterWrite; got != 2 {
		t.Errorf("probes after generation bump = %d, want 2 (one per generation)", got)
	}

	// And the new generation is cached in turn.
	for i := 0; i < 10; i++ {
		if _, err := EvaluateState(ctx, idx, cache, fixedClock(at), m); err != nil {
			t.Fatalf("EvaluateState (gen-2) #%d: %v", i, err)
		}
	}
	if got := idx.probeCount() - probesAfterWrite; got != 2 {
		t.Errorf("probes after re-surfacing at gen-2 = %d, want 2", got)
	}
}

// TestDriftCacheIsConcurrencySafe runs the same evaluation from several
// goroutines: the -race gate is the point, and the probe count must still
// collapse to one.
func TestDriftCacheIsConcurrencySafe(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")
	m := writeFixture(t, ctx, ms, idx, at)
	base := idx.probeCount()

	cache := NewDriftCache()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 8; j++ {
				if _, err := EvaluateState(ctx, idx, cache, fixedClock(at), m); err != nil {
					t.Errorf("EvaluateState: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	// Concurrent first-callers may race into the same miss, so the bound
	// is "far fewer than 128", not exactly one — asserting exactly one
	// here would be asserting a lock this cache deliberately does not hold
	// across the probe.
	if got := idx.probeCount() - base; got > 16 {
		t.Errorf("concurrent surfacings probed %d times; the cache is not collapsing them", got)
	}
}

// TestMemoryLifecycle proves AM-4 end to end: fresh → suspect on anchor
// content change (labeled with what changed and when, tier dropped one step)
// → orphaned on anchor deletion, and an orphaned memory does not surface.
func TestMemoryLifecycle(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")
	m := writeFixture(t, ctx, ms, idx, at)
	if m.Tier != core.TierPreScan {
		t.Fatalf("fixture tier = %q, want %q", m.Tier, core.TierPreScan)
	}

	// ── fresh ────────────────────────────────────────────────────────────
	ev, err := EvaluateState(ctx, idx, NewDriftCache(), fixedClock(at), m)
	if err != nil {
		t.Fatalf("EvaluateState (fresh): %v", err)
	}
	if ev.State != StateFresh || !ev.Surfaces {
		t.Fatalf("state = %q surfaces = %v, want fresh/true", ev.State, ev.Surfaces)
	}
	if ev.Tier != core.TierPreScan {
		t.Errorf("fresh tier = %q, want %q (no penalty while fresh)", ev.Tier, core.TierPreScan)
	}

	// ── fresh → suspect (anchor content changed) ─────────────────────────
	idx.set("symbol:parseConfig", "hash-v2")
	idx.setGeneration("gen-2")
	ev, err = EvaluateState(ctx, idx, NewDriftCache(), fixedClock(at), m)
	if err != nil {
		t.Fatalf("EvaluateState (suspect): %v", err)
	}
	if ev.State != StateSuspect {
		t.Fatalf("state = %q, want %q", ev.State, StateSuspect)
	}
	if !ev.Surfaces {
		t.Error("a suspect memory must still surface — suppressing it silently loses more than it saves")
	}
	if ev.Tier != core.TierDocClaimed {
		t.Errorf("suspect tier = %q, want %q (pre-scan dropped one step)", ev.Tier, core.TierDocClaimed)
	}
	if ev.BaseTier != core.TierPreScan {
		t.Errorf("BaseTier = %q, want the stored %q", ev.BaseTier, core.TierPreScan)
	}
	if ev.Confidence > 2 {
		t.Errorf("suspect confidence = %d, want <= 2 (the dropped tier's ceiling)", ev.Confidence)
	}
	if len(ev.Drift) != 1 || ev.Drift[0].Status != DriftChanged {
		t.Fatalf("drift = %+v, want one changed anchor", ev.Drift)
	}
	// Labeled with WHAT changed and WHEN.
	detail := ev.Drift[0].Detail
	for _, want := range []string{"symbol:parseConfig", "2026-08-08T12:00:00Z", "gen-2"} {
		if !strings.Contains(detail, want) {
			t.Errorf("drift detail %q does not mention %q", detail, want)
		}
	}
	line := ev.Line()
	if !strings.Contains(line, string(StateSuspect)) || !strings.Contains(line, string(core.TierDocClaimed)) {
		t.Errorf("surfaced line %q must carry both tier and state", line)
	}

	// ── suspect → orphaned (anchor deleted) ──────────────────────────────
	idx.remove("symbol:parseConfig")
	idx.setGeneration("gen-3")
	ev, err = EvaluateState(ctx, idx, NewDriftCache(), fixedClock(at), m)
	if err != nil {
		t.Fatalf("EvaluateState (orphaned): %v", err)
	}
	if ev.State != StateOrphaned {
		t.Fatalf("state = %q, want %q", ev.State, StateOrphaned)
	}
	if ev.Surfaces {
		t.Error("an orphaned memory must NOT surface")
	}
	if len(ev.Drift) != 1 || ev.Drift[0].Status != DriftMissing {
		t.Fatalf("drift = %+v, want one missing anchor", ev.Drift)
	}
	if got := Surfacing([]Evaluation{ev}); len(got) != 0 {
		t.Errorf("Surfacing kept an orphaned memory: %+v", got)
	}
}

// TestRetiredNeverSurfacesButIsRetrievable: §3.2's third key. A retired
// memory is out of every payload forever and still available by id.
func TestRetiredNeverSurfacesButIsRetrievable(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")
	m := writeFixture(t, ctx, ms, idx, at)

	later := mustTime(t, "2026-08-09T09:00:00Z")
	retired, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveNoLongerTrue,
		Actor{Kind: ActorHuman, Name: "alex"}, "commit:bbbb222", "", "we removed the schema check in August")
	if err != nil {
		t.Fatalf("Resolve(no-longer-true): %v", err)
	}
	if !retired.Retired() {
		t.Fatal("Resolve(no-longer-true) did not retire the memory")
	}

	ev, err := EvaluateState(ctx, idx, NewDriftCache(), fixedClock(later), retired)
	if err != nil {
		t.Fatalf("EvaluateState: %v", err)
	}
	if ev.State != StateRetired || ev.Surfaces {
		t.Errorf("state = %q surfaces = %v, want retired/false", ev.State, ev.Surfaces)
	}

	got, found, gerr := ms.Get(ctx, m.ID)
	if gerr != nil {
		t.Fatalf("Get: %v", gerr)
	}
	if !found {
		t.Fatal("a retired memory must stay retrievable by id")
	}
	if got.RetiredReason == "" {
		t.Error("a retired memory must record why")
	}
}

// TestSurfacingOrderIsDeterministic: tier, then specificity, then id — the
// order the injection path will consume.
func TestSurfacingOrderIsDeterministic(t *testing.T) {
	evs := []Evaluation{
		{ID: "ccc", Tier: core.TierPreScan, Specificity: 0},
		{ID: "aaa", Tier: core.TierAssumed, Specificity: 0},
		{ID: "bbb", Tier: core.TierPreScan, Specificity: 4},
		{ID: "ddd", Tier: core.TierVerified, Specificity: 4},
		{ID: "eee", Tier: core.TierPreScan, Specificity: 0},
	}
	SortEvaluations(evs)
	want := []string{"ddd", "ccc", "eee", "bbb", "aaa"}
	for i, id := range want {
		if evs[i].ID != id {
			t.Fatalf("order = %v, want %v", ids(evs), want)
		}
	}

	// Stable under re-sorting: sorting an already-sorted slice is a no-op.
	SortEvaluations(evs)
	for i, id := range want {
		if evs[i].ID != id {
			t.Fatalf("re-sort changed the order: %v", ids(evs))
		}
	}
}

func ids(evs []Evaluation) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.ID)
	}
	return out
}

// TestEveryEvaluationLineCarriesTierAndState: an unlabelled memory is a claim
// fathom would be making on its own authority (§9), so every state's
// rendering is checked, not just the interesting one.
func TestEveryEvaluationLineCarriesTierAndState(t *testing.T) {
	for _, st := range []State{StateFresh, StateSuspect, StateOrphaned, StateRetired} {
		ev := Evaluation{ID: "x", Claim: "a claim", State: st, Tier: core.TierSMEStated}
		line := ev.Line()
		if !strings.Contains(line, string(st)) {
			t.Errorf("line %q for state %q omits the state", line, st)
		}
		if !strings.Contains(line, string(core.TierSMEStated)) {
			t.Errorf("line %q for state %q omits the tier", line, st)
		}
	}
}

// TestEvaluateStateRequiresAnInjectedClock mirrors the write path's rule.
func TestEvaluateStateRequiresAnInjectedClock(t *testing.T) {
	_, err := EvaluateState(context.Background(), newFakeIndex("g"), nil, nil, Memory{})
	if err == nil || !strings.Contains(err.Error(), "clock") {
		t.Fatalf("err = %v, want a complaint about the missing clock", err)
	}
}

// TestUnknownBaselineIsNamedNotAssumedFresh — law 3: an anchor with no
// recorded baseline hash cannot be judged, and saying "unchanged" would be a
// fabricated measurement.
func TestUnknownBaselineIsNamedNotAssumedFresh(t *testing.T) {
	ctx := context.Background()
	idx := newFakeIndex("gen-1")
	idx.set("symbol:parseConfig", "hash-v1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	m := Memory{
		ID: "legacy", Service: "svc", Claim: "a claim",
		Anchors:      []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		Tier:         core.TierPreScan,
		AnchorHashes: map[string]string{}, // no baseline recorded
	}
	ev, err := EvaluateState(ctx, idx, NewDriftCache(), fixedClock(at), m)
	if err != nil {
		t.Fatalf("EvaluateState: %v", err)
	}
	if len(ev.Drift) != 1 || ev.Drift[0].Status != DriftUnknown {
		t.Fatalf("drift = %+v, want status %q", ev.Drift, DriftUnknown)
	}
	if !strings.Contains(ev.Drift[0].Detail, "baseline") {
		t.Errorf("detail %q must name the missing baseline", ev.Drift[0].Detail)
	}
}
