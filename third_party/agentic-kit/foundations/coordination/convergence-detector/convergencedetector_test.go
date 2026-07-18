package convergencedetector

import (
	"testing"
)

func TestNewDetector_InvalidPolicy(t *testing.T) {
	cases := []struct {
		name   string
		policy Policy
	}{
		{"zero window", Policy{Window: 0, Threshold: 0.8, AbandonAfter: 10}},
		{"zero threshold", Policy{Window: 5, Threshold: 0, AbandonAfter: 10}},
		{"threshold too high", Policy{Window: 5, Threshold: 1.5, AbandonAfter: 10}},
		{"zero abandon", Policy{Window: 5, Threshold: 0.8, AbandonAfter: 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDetector(tc.policy)
			if err != ErrInvalidPolicy {
				t.Fatalf("expected ErrInvalidPolicy, got %v", err)
			}
		})
	}
}

func TestNewDetector_ValidPolicy(t *testing.T) {
	p := Policy{Window: 3, Threshold: 0.8, AbandonAfter: 10}
	d, err := NewDetector(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil detector")
	}
}

func TestFeed_ExploringUntilWindowFull(t *testing.T) {
	p := Policy{Window: 3, Threshold: 0.8, AbandonAfter: 20}
	d, _ := NewDetector(p)
	// First two signals: window not full yet → exploring
	for i := 0; i < 2; i++ {
		v := d.Feed(Signal{Value: "answer-A"})
		if v != VerdictExploring {
			t.Fatalf("signal %d: expected exploring, got %v", i+1, v)
		}
	}
}

func TestFeed_Converged(t *testing.T) {
	// Window=3, Threshold=0.8 → need 3 of 3 (100%) or can also use
	// 80% of 5, but with window=3 we need 3/3 ≈ 1.0 or 2/3 ≈ 0.67 < 0.8.
	// Use Window=5, Threshold=0.6 → 3 out of 5.
	p := Policy{Window: 5, Threshold: 0.6, AbandonAfter: 20}
	d, _ := NewDetector(p)

	// Feed 5 signals: 3 same, 2 different
	signals := []string{"A", "B", "A", "C", "A"} // A appears 3/5 = 0.6
	var lastVerdict Verdict
	for _, v := range signals {
		lastVerdict = d.Feed(Signal{Value: v})
	}
	if lastVerdict != VerdictConverged {
		t.Fatalf("expected converged, got %v", lastVerdict)
	}
}

func TestFeed_Abandoned(t *testing.T) {
	// Window=5, Threshold=0.9, AbandonAfter=6 — hard to converge
	p := Policy{Window: 5, Threshold: 0.9, AbandonAfter: 6}
	d, _ := NewDetector(p)

	// Feed 6 diverse signals — convergence won't be reached
	for i := 0; i < 6; i++ {
		v := d.Feed(Signal{Value: string(rune('A' + i))})
		if i == 5 && v != VerdictAbandoned {
			t.Fatalf("signal 6: expected abandoned, got %v", v)
		}
	}
}

func TestFeed_ConvergedBeforeAbandon(t *testing.T) {
	// Convergence should take priority over abandon budget.
	p := Policy{Window: 3, Threshold: 1.0, AbandonAfter: 3}
	d, _ := NewDetector(p)

	signals := []string{"X", "X", "X"} // all same → converged at signal 3
	var verdicts []Verdict
	for _, v := range signals {
		verdicts = append(verdicts, d.Feed(Signal{Value: v}))
	}
	last := verdicts[len(verdicts)-1]
	if last != VerdictConverged {
		t.Fatalf("expected converged (not abandoned), got %v", last)
	}
}

func TestFeed_SlidingWindow(t *testing.T) {
	// Window=3, Threshold=1.0: needs 3 identical in a row.
	// Feed A,B,A then A,A → the last 3 are A,A,A → converged.
	p := Policy{Window: 3, Threshold: 1.0, AbandonAfter: 20}
	d, _ := NewDetector(p)

	mixed := []string{"A", "B", "A"}
	for _, v := range mixed {
		vc := d.Feed(Signal{Value: v})
		if vc != VerdictExploring {
			t.Fatalf("expected exploring during mixed phase, got %v", vc)
		}
	}
	// Now feed two more A's; on the third the window becomes [A,A,A].
	d.Feed(Signal{Value: "A"})
	final := d.Feed(Signal{Value: "A"})
	if final != VerdictConverged {
		t.Fatalf("expected converged after sliding window fills with A, got %v", final)
	}
}

func TestDeterminism_SameInputSameOutput(t *testing.T) {
	p := Policy{Window: 4, Threshold: 0.75, AbandonAfter: 15}
	signals := []string{"A", "B", "A", "A", "A", "C", "A"}

	run := func() []Verdict {
		d, _ := NewDetector(p)
		out := make([]Verdict, 0, len(signals))
		for _, v := range signals {
			out = append(out, d.Feed(Signal{Value: v}))
		}
		return out
	}

	first := run()
	second := run()
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("non-deterministic at index %d: %v != %v", i, first[i], second[i])
		}
	}
}

func TestMostFrequent_TieBreakDeterminism(t *testing.T) {
	// When two values tie, the lexicographically smaller one should win.
	ss := []string{"B", "A"}
	got, _ := mostFrequent(ss)
	if got != "A" {
		t.Fatalf("expected A (lex smaller) on tie, got %v", got)
	}
}

func TestState_ReflectsCurrentVerdict(t *testing.T) {
	p := Policy{Window: 3, Threshold: 1.0, AbandonAfter: 20}
	d, _ := NewDetector(p)
	d.Feed(Signal{Value: "X"})
	d.Feed(Signal{Value: "X"})
	state := d.State()
	if state.SignalCount != 2 {
		t.Fatalf("expected 2 signals, got %d", state.SignalCount)
	}
	if state.Verdict != VerdictExploring {
		t.Fatalf("expected exploring, got %v", state.Verdict)
	}
	d.Feed(Signal{Value: "X"})
	state = d.State()
	if state.Verdict != VerdictConverged {
		t.Fatalf("expected converged, got %v", state.Verdict)
	}
	if state.SignalCount != 3 {
		t.Fatalf("expected 3 signals, got %d", state.SignalCount)
	}
}
