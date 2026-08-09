package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestReconfirmationTierDependsOnWhoConfirmed is the package-level half of
// AM-5: the SAME memory, drifted the same way, re-confirmed by a model lands
// strictly below the same memory re-confirmed by a human — and neither caller
// states a tier.
func TestReconfirmationTierDependsOnWhoConfirmed(t *testing.T) {
	ctx := context.Background()
	at := mustTime(t, "2026-08-08T12:00:00Z")
	later := mustTime(t, "2026-08-09T09:00:00Z")

	confirmBy := func(t *testing.T, by Actor) Memory {
		t.Helper()
		ms := NewMemStore()
		idx := newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		idx.set("symbol:parseConfig", "hash-v2")
		idx.setGeneration("gen-2")

		out, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveStillTrue, by, "commit:bbbb222", "", "")
		if err != nil {
			t.Fatalf("Resolve by %s: %v", by, err)
		}
		return out
	}

	human := confirmBy(t, Actor{Kind: ActorHuman, Name: "alex"})
	model := confirmBy(t, Actor{Kind: ActorModel, Name: "some-model"})
	machine := confirmBy(t, Actor{Kind: ActorMachine, Name: "task validate"})

	if ladderIndex(model.Tier) <= ladderIndex(human.Tier) {
		t.Errorf("model-confirmed tier %q is not below human-confirmed tier %q", model.Tier, human.Tier)
	}
	// Once the anchored content has changed, the only thing supporting the
	// claim is the confirmer's own authority: the original index derivation
	// ran against code that no longer exists.
	if human.Tier != core.TierSMEStated {
		t.Errorf("human-confirmed tier = %q, want %q", human.Tier, core.TierSMEStated)
	}
	if model.Tier != core.TierAssumed {
		t.Errorf("model-confirmed tier = %q, want %q", model.Tier, core.TierAssumed)
	}
	if model.Confidence > 1 {
		t.Errorf("model-confirmed confidence = %d, want <= 1", model.Confidence)
	}
	for _, m := range []Memory{human, model} {
		if !m.Capped || m.CapReason == "" {
			t.Errorf("%s confirmation weakened the tier below %q and must say so", m.ConfirmedBy, core.TierPreScan)
		}
	}
	// …and it is a genuine "weaker of", not a blanket downgrade: a machine
	// re-check outranks the memory's own pre-scan evidence, so the tier is
	// held at pre-scan rather than pushed anywhere.
	if machine.Tier != core.TierPreScan {
		t.Errorf("machine-confirmed tier = %q, want %q (the evidence's own tier, unweakened)", machine.Tier, core.TierPreScan)
	}
	if machine.Capped {
		t.Errorf("machine confirmation reported a cap it did not apply: %s", machine.CapReason)
	}

	// Re-confirmation returns the memory to fresh and updates
	// last_confirmed_at, recording who and against which commit.
	for _, m := range []Memory{human, model, machine} {
		if !m.LastConfirmedAt.Equal(later) {
			t.Errorf("LastConfirmedAt = %s, want %s", m.LastConfirmedAt, later)
		}
		if m.ConfirmedAtCommit != "commit:bbbb222" {
			t.Errorf("ConfirmedAtCommit = %q, want commit:bbbb222", m.ConfirmedAtCommit)
		}
		if len(m.History) != 2 || m.History[1].Kind != ConfirmStillTrue {
			t.Errorf("history = %+v, want a written entry then a still-true entry", m.History)
		}
	}

	// Back to fresh against the content it was confirmed against.
	idx := newFakeIndex("gen-2")
	idx.set("symbol:parseConfig", "hash-v2")
	ev, err := EvaluateState(ctx, idx, NewDriftCache(), fixedClock(later), human)
	if err != nil {
		t.Fatalf("EvaluateState: %v", err)
	}
	if ev.State != StateFresh {
		t.Errorf("state after re-confirmation = %q, want %q", ev.State, StateFresh)
	}
}

// TestReviewQueueHoldsExactlySuspectAndOrphaned: fresh memories must not
// clutter the queue, and retired ones must never come back to it.
func TestReviewQueueHoldsExactlySuspectAndOrphaned(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")

	idx.set("symbol:stayFresh", "fresh-v1")
	idx.set("symbol:willDrift", "drift-v1")
	idx.set("symbol:willVanish", "vanish-v1")
	idx.set("symbol:willRetire", "retire-v1")

	mk := func(sym, claim string) Memory {
		t.Helper()
		m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
			Service:  "svc",
			Claim:    claim,
			Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: sym}},
			Evidence: []Evidence{{Kind: EvidenceIndex, Ref: "svc::pkg." + sym}},
			Author:   Actor{Kind: ActorHuman, Name: "alex"},
		})
		if err != nil {
			t.Fatalf("WriteMemory(%s): %v", sym, err)
		}
		return m
	}
	mk("stayFresh", "this one stays true")
	drifted := mk("willDrift", "this one is about changed code")
	vanished := mk("willVanish", "this one is about deleted code")
	retired := mk("willRetire", "this one was retired")

	later := mustTime(t, "2026-08-09T09:00:00Z")
	if _, err := Resolve(ctx, ms, idx, fixedClock(later), retired.ID, ResolveNoLongerTrue,
		Actor{Kind: ActorHuman, Name: "alex"}, "commit:bbbb222", "", "no longer true"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	idx.set("symbol:willDrift", "drift-v2")
	idx.remove("symbol:willVanish")
	idx.setGeneration("gen-2")

	items, err := ReviewQueue(ctx, ms, idx, NewDriftCache(), fixedClock(later), "svc")
	if err != nil {
		t.Fatalf("ReviewQueue: %v", err)
	}
	got := map[string]State{}
	for _, it := range items {
		got[it.Memory.ID] = it.Evaluation.State
	}
	if len(got) != 2 {
		t.Fatalf("queue holds %d items, want 2: %+v", len(got), got)
	}
	if got[drifted.ID] != StateSuspect {
		t.Errorf("drifted memory state = %q, want %q", got[drifted.ID], StateSuspect)
	}
	if got[vanished.ID] != StateOrphaned {
		t.Errorf("vanished memory state = %q, want %q", got[vanished.ID], StateOrphaned)
	}
	for _, it := range items {
		if it.Reason == "" {
			t.Errorf("queue item %s carries no reason", it.Memory.ID)
		}
	}
}

// TestRewriteKeepsTheIdAndReconfirms: a rewrite is the same memory about the
// same anchors, said better — so the id must not move, or every reference to
// it breaks.
func TestRewriteKeepsTheIdAndReconfirms(t *testing.T) {
	ctx := context.Background()
	ms := NewMemStore()
	idx := newFakeIndex("gen-1")
	at := mustTime(t, "2026-08-08T12:00:00Z")
	m := writeFixture(t, ctx, ms, idx, at)

	idx.set("symbol:parseConfig", "hash-v2")
	idx.setGeneration("gen-2")
	later := mustTime(t, "2026-08-09T09:00:00Z")

	out, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveRewrite,
		Actor{Kind: ActorHuman, Name: "alex"}, "commit:bbbb222",
		"parseConfig validates the schema and normalizes defaults before returning.", "clarified")
	if err != nil {
		t.Fatalf("Resolve(rewrite): %v", err)
	}
	if out.ID != m.ID {
		t.Errorf("rewrite changed the id: %s → %s", m.ID, out.ID)
	}
	if !strings.Contains(out.Claim, "normalizes defaults") {
		t.Errorf("claim was not rewritten: %q", out.Claim)
	}
	if out.History[len(out.History)-1].Kind != ConfirmRewrite {
		t.Errorf("history does not record the rewrite: %+v", out.History)
	}

	ev, err := EvaluateState(ctx, idx, NewDriftCache(), fixedClock(later), out)
	if err != nil {
		t.Fatalf("EvaluateState: %v", err)
	}
	if ev.State != StateFresh {
		t.Errorf("state after rewrite = %q, want %q", ev.State, StateFresh)
	}
}

// TestResolveRefusals: a rewrite with no text, an unknown resolution, an
// unknown actor, a missing id, an already-retired memory, and an orphaned
// anchor all refuse rather than half-apply.
func TestResolveRefusals(t *testing.T) {
	ctx := context.Background()
	at := mustTime(t, "2026-08-08T12:00:00Z")
	later := mustTime(t, "2026-08-09T09:00:00Z")
	human := Actor{Kind: ActorHuman, Name: "alex"}

	t.Run("rewrite with no text", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		_, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveRewrite, human, "", "   ", "")
		if re, ok := Refused(err); !ok || re.Reason != RefusalNoClaim {
			t.Fatalf("err = %v, want a no-claim refusal", err)
		}
	})
	t.Run("unknown resolution", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		if _, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, "maybe", human, "", "", ""); err == nil {
			t.Fatal("Resolve accepted an unknown resolution")
		}
	})
	t.Run("unknown actor", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		if _, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveStillTrue, Actor{Kind: "oracle"}, "", "", ""); err == nil {
			t.Fatal("Resolve accepted an unknown actor kind")
		}
	})
	t.Run("missing id", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		if _, err := Resolve(ctx, ms, idx, fixedClock(later), "nope", ResolveStillTrue, human, "", "", ""); err == nil {
			t.Fatal("Resolve accepted an unknown id")
		}
	})
	t.Run("already retired", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		if _, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveNoLongerTrue, human, "", "", "gone"); err != nil {
			t.Fatalf("first Resolve: %v", err)
		}
		if _, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveStillTrue, human, "", "", ""); err == nil {
			t.Fatal("a retired memory was re-resolved")
		}
	})
	t.Run("orphaned anchor cannot be re-confirmed", func(t *testing.T) {
		ms, idx := NewMemStore(), newFakeIndex("gen-1")
		m := writeFixture(t, ctx, ms, idx, at)
		idx.remove("symbol:parseConfig")
		_, err := Resolve(ctx, ms, idx, fixedClock(later), m.ID, ResolveStillTrue, human, "", "", "")
		if err == nil || !strings.Contains(err.Error(), "no longer exists") {
			t.Fatalf("err = %v, want a refusal naming the vanished anchor", err)
		}
	})
}
