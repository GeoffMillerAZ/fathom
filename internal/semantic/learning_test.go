// learning_test.go — GL-13 (docs/plans/governance-learning.md, design doc
// §6 "Rules that learn"): applyLearning's own decisions, pure. The row's
// proof anchor (reachable_from) is cmd/fathom's TestLearningPromotesAndRetires,
// which reaches applyLearning through `fathom claim learn`.
package semantic

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestApplyLearningPromotesAndRetires(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	thresholds := DefaultLearningThresholds() // 3/2/30d promote, 3 refutes retire, 180d never-triggered

	claim := func(id string, lifecycle core.ClaimLifecycle, validFrom time.Time) core.SemanticClaim {
		return core.SemanticClaim{
			ServiceID: "svc", ClaimID: id, Layer: "obligations", Kind: "obligation",
			Subject: id + "#subject", Rule: "rule text", Status: core.ClaimFresh,
			Lifecycle: lifecycle, ValidFrom: validFrom,
		}
	}

	corrob := func(id, session string, at time.Time) RuleOutcomeEvent {
		return RuleOutcomeEvent{RuleID: id, Event: RuleEventCorroborated, Session: session, At: at}
	}
	refute := func(id, session string, at time.Time) RuleOutcomeEvent {
		return RuleOutcomeEvent{RuleID: id, Event: RuleEventRefuted, Session: session, At: at}
	}
	served := func(id string, at time.Time) RuleOutcomeEvent {
		return RuleOutcomeEvent{RuleID: id, Event: RuleEventServed, At: at}
	}

	t.Run("promotes candidate with 3 corroborations from 2 sessions, 0 refutes", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c1", core.LifecycleCandidate, time.Time{})}
		events := []RuleOutcomeEvent{
			corrob("c1", "sess-a", now.Add(-5*24*time.Hour)),
			corrob("c1", "sess-a", now.Add(-4*24*time.Hour)),
			corrob("c1", "sess-b", now.Add(-3*24*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleActive {
			t.Fatalf("lifecycle = %q, want active", out[0].Lifecycle)
		}
		if len(decisions) != 1 || decisions[0].ClaimID != "c1" ||
			decisions[0].From != core.LifecycleCandidate || decisions[0].To != core.LifecycleActive {
			t.Fatalf("decisions = %+v, want one c1 candidate->active", decisions)
		}
		if decisions[0].Corroborations != 3 || decisions[0].DistinctSessions != 2 {
			t.Errorf("decision signals = %+v, want 3 corroborations/2 sessions", decisions[0])
		}
	})

	t.Run("does not promote: 3 corroborations from only 1 session", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c2", core.LifecycleCandidate, time.Time{})}
		events := []RuleOutcomeEvent{
			corrob("c2", "sess-a", now.Add(-5*24*time.Hour)),
			corrob("c2", "sess-a", now.Add(-4*24*time.Hour)),
			corrob("c2", "sess-a", now.Add(-3*24*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleCandidate {
			t.Fatalf("lifecycle = %q, want still candidate (only 1 distinct session)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("does not promote: a refute inside the window blocks it", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c3", core.LifecycleCandidate, time.Time{})}
		events := []RuleOutcomeEvent{
			corrob("c3", "sess-a", now.Add(-5*24*time.Hour)),
			corrob("c3", "sess-b", now.Add(-4*24*time.Hour)),
			corrob("c3", "sess-c", now.Add(-3*24*time.Hour)),
			refute("c3", "sess-d", now.Add(-2*24*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleCandidate {
			t.Fatalf("lifecycle = %q, want still candidate (a refute in-window)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("retires active: refutes >= 3 and greater than corroborations", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c4", core.LifecycleActive, now.Add(-400*24*time.Hour))}
		events := []RuleOutcomeEvent{
			refute("c4", "sess-a", now.Add(-100*24*time.Hour)),
			refute("c4", "sess-b", now.Add(-90*24*time.Hour)),
			refute("c4", "sess-c", now.Add(-80*24*time.Hour)),
			corrob("c4", "sess-d", now.Add(-200*24*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleRetired {
			t.Fatalf("lifecycle = %q, want retired", out[0].Lifecycle)
		}
		if out[0].ValidTo.IsZero() {
			t.Fatal("ValidTo not set on retirement — GL-13's own 'closes the validity window, never deletes'")
		}
		if !out[0].ValidTo.Equal(now) {
			t.Errorf("ValidTo = %v, want %v (the injected clock)", out[0].ValidTo, now)
		}
		if len(decisions) != 1 || decisions[0].To != core.LifecycleRetired || decisions[0].Refutes != 3 || decisions[0].Corroborations != 1 {
			t.Fatalf("decisions = %+v, want one c4 retirement with 3 refutes/1 corroboration", decisions)
		}
	})

	t.Run("does not retire: refutes >= 3 but corroborations keep pace", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c5", core.LifecycleActive, now.Add(-400*24*time.Hour))}
		events := []RuleOutcomeEvent{
			refute("c5", "sess-a", now.Add(-100*24*time.Hour)),
			refute("c5", "sess-b", now.Add(-90*24*time.Hour)),
			refute("c5", "sess-c", now.Add(-80*24*time.Hour)),
			corrob("c5", "sess-d", now.Add(-70*24*time.Hour)),
			corrob("c5", "sess-e", now.Add(-60*24*time.Hour)),
			corrob("c5", "sess-f", now.Add(-50*24*time.Hour)),
			served("c5", now.Add(-10*24*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleActive {
			t.Fatalf("lifecycle = %q, want still active (refutes not > corroborations)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("retires active: never triggered for the threshold window", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c6", core.LifecycleActive, now.Add(-400*24*time.Hour))}
		events := []RuleOutcomeEvent{
			served("c6", now.Add(-300*24*time.Hour)), // outside the 180d window
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleRetired {
			t.Fatalf("lifecycle = %q, want retired (never triggered)", out[0].Lifecycle)
		}
		if !out[0].ValidTo.Equal(now) {
			t.Errorf("ValidTo = %v, want %v", out[0].ValidTo, now)
		}
		if len(decisions) != 1 || decisions[0].To != core.LifecycleRetired {
			t.Fatalf("decisions = %+v, want one c6 retirement", decisions)
		}
	})

	t.Run("does not retire never-triggered: claim too young for the window", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c7", core.LifecycleActive, now.Add(-10*24*time.Hour))}
		out, decisions := applyLearning(claims, nil, thresholds, now)
		if out[0].Lifecycle != core.LifecycleActive {
			t.Fatalf("lifecycle = %q, want still active (too young to judge never-triggered)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("does not retire never-triggered: served recently", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c8", core.LifecycleActive, now.Add(-400*24*time.Hour))}
		events := []RuleOutcomeEvent{served("c8", now.Add(-5*24*time.Hour))}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleActive {
			t.Fatalf("lifecycle = %q, want still active (served recently)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("retired is terminal: no further transition, no ValidTo overwrite", func(t *testing.T) {
		validTo := now.Add(-1 * time.Hour)
		c := claim("c9", core.LifecycleRetired, now.Add(-400*24*time.Hour))
		c.ValidTo = validTo
		events := []RuleOutcomeEvent{
			refute("c9", "sess-a", now.Add(-1*24*time.Hour)),
			refute("c9", "sess-b", now.Add(-2*24*time.Hour)),
			refute("c9", "sess-c", now.Add(-3*24*time.Hour)),
		}
		out, decisions := applyLearning([]core.SemanticClaim{c}, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleRetired {
			t.Fatalf("lifecycle = %q, want still retired", out[0].Lifecycle)
		}
		if !out[0].ValidTo.Equal(validTo) {
			t.Errorf("ValidTo = %v, want unchanged %v (retirement is a one-time event)", out[0].ValidTo, validTo)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none (no re-decision on a terminal claim)", decisions)
		}
	})

	t.Run("a future-dated event is never trusted", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c10", core.LifecycleCandidate, time.Time{})}
		events := []RuleOutcomeEvent{
			corrob("c10", "sess-a", now.Add(24*time.Hour)), // in the future
			corrob("c10", "sess-b", now.Add(48*time.Hour)),
			corrob("c10", "sess-c", now.Add(72*time.Hour)),
		}
		out, decisions := applyLearning(claims, events, thresholds, now)
		if out[0].Lifecycle != core.LifecycleCandidate {
			t.Fatalf("lifecycle = %q, want still candidate (future events ignored)", out[0].Lifecycle)
		}
		if len(decisions) != 0 {
			t.Fatalf("decisions = %+v, want none", decisions)
		}
	})

	t.Run("legacy claim (Lifecycle unset) is treated as active", func(t *testing.T) {
		claims := []core.SemanticClaim{claim("c11", "", now.Add(-400*24*time.Hour))}
		out, decisions := applyLearning(claims, nil, thresholds, now)
		if out[0].Lifecycle != core.LifecycleRetired {
			t.Fatalf("lifecycle = %q, want retired (an unset-lifecycle claim is judged as active, and this one is never triggered)", out[0].Lifecycle)
		}
		if len(decisions) != 1 || decisions[0].From != core.LifecycleActive {
			t.Fatalf("decisions = %+v, want one c11 active->retired (From reads as the effective, not literal, lifecycle)", decisions)
		}
	})

	t.Run("output is deterministic: decisions sorted by ClaimID", func(t *testing.T) {
		claims := []core.SemanticClaim{
			claim("cz", core.LifecycleCandidate, time.Time{}),
			claim("ca", core.LifecycleCandidate, time.Time{}),
		}
		events := []RuleOutcomeEvent{
			corrob("cz", "sess-a", now.Add(-1*24*time.Hour)),
			corrob("cz", "sess-b", now.Add(-1*24*time.Hour)),
			corrob("cz", "sess-c", now.Add(-1*24*time.Hour)),
			corrob("ca", "sess-a", now.Add(-1*24*time.Hour)),
			corrob("ca", "sess-b", now.Add(-1*24*time.Hour)),
			corrob("ca", "sess-c", now.Add(-1*24*time.Hour)),
		}
		_, decisions := applyLearning(claims, events, thresholds, now)
		if len(decisions) != 2 || decisions[0].ClaimID != "ca" || decisions[1].ClaimID != "cz" {
			t.Fatalf("decisions = %+v, want [ca, cz] sorted", decisions)
		}
	})
}
