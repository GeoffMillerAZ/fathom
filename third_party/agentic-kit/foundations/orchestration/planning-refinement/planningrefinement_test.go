package planningrefinement

import "testing"

func newPlan() Plan {
	return Plan{ID: "plan-1", Content: "do the thing", Round: 0}
}

func TestRefine_Accept(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 5})
	p, result, err := r.Refine(newPlan(), SignalAccept)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Converged {
		t.Fatal("expected Converged=true on Accept")
	}
	if result.Escalated {
		t.Fatal("expected Escalated=false on Accept")
	}
	if p.Round != 0 {
		t.Fatalf("Round should not change on Accept, got %d", p.Round)
	}
}

func TestRefine_Reject(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 5})
	p, result, err := r.Refine(newPlan(), SignalReject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Converged {
		t.Fatal("expected Converged=true on Reject")
	}
	if result.Escalated {
		t.Fatal("expected Escalated=false on Reject")
	}
	if p.Round != 0 {
		t.Fatalf("Round should not change on Reject, got %d", p.Round)
	}
}

func TestRefine_Improve_NotYetConverged(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 3})
	p := newPlan()
	p, result, err := r.Refine(p, SignalImprove)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Converged {
		t.Fatal("expected Converged=false after first improve")
	}
	if result.Escalated {
		t.Fatal("expected Escalated=false after first improve")
	}
	if p.Round != 1 {
		t.Fatalf("expected Round=1, got %d", p.Round)
	}
}

func TestRefine_Improve_EscalatesAtMaxRounds(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 2})
	p := newPlan()
	var result RefinementResult
	var err error

	// Round 1: not yet at max
	p, result, err = r.Refine(p, SignalImprove)
	if err != nil {
		t.Fatalf("round 1 error: %v", err)
	}
	if result.Converged {
		t.Fatal("should not converge at round 1 with MaxRounds=2")
	}

	// Round 2: hits max
	p, result, err = r.Refine(p, SignalImprove)
	if err != nil {
		t.Fatalf("round 2 error: %v", err)
	}
	if !result.Converged {
		t.Fatal("expected Converged=true at MaxRounds")
	}
	if !result.Escalated {
		t.Fatal("expected Escalated=true at MaxRounds")
	}
	if p.Round != 2 {
		t.Fatalf("expected Round=2, got %d", p.Round)
	}
}

func TestRefine_InvalidSignal(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 3})
	_, _, err := r.Refine(newPlan(), Signal("unknown"))
	if err != ErrInvalidSignal {
		t.Fatalf("expected ErrInvalidSignal, got %v", err)
	}
}

func TestRefine_Determinism(t *testing.T) {
	// Same input → same output, regardless of how many times called.
	r := NewRefiner(Policy{MaxRounds: 5})
	p := newPlan()
	p1, res1, _ := r.Refine(p, SignalImprove)
	p2, res2, _ := r.Refine(p, SignalImprove)
	if p1.Round != p2.Round {
		t.Fatalf("non-deterministic Round: %d vs %d", p1.Round, p2.Round)
	}
	if res1.Converged != res2.Converged || res1.Escalated != res2.Escalated {
		t.Fatal("non-deterministic result")
	}
}

func TestNewRefiner_MaxRoundsClampedToOne(t *testing.T) {
	r := NewRefiner(Policy{MaxRounds: 0})
	p, result, err := r.Refine(newPlan(), SignalImprove)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// MaxRounds was clamped to 1, so first improve hits it immediately.
	if !result.Converged || !result.Escalated {
		t.Fatalf("expected immediate escalation with MaxRounds=0 (clamped to 1), got %+v round=%d", result, p.Round)
	}
}
