package honestygate_test

import (
	"testing"

	hg "github.com/geoffmilleraz/agentic-kit/foundations/quality/honesty-gate"
)

func defaultGate() hg.Gate { return hg.Gate{} }

func TestGate_Block_ZeroEvidence(t *testing.T) {
	d := defaultGate().Evaluate(hg.Claim{
		Text:          "Something happened.",
		EvidenceCount: 0,
		Confidence:    0.9,
	})
	if d.Verdict != hg.VerdictBlock {
		t.Errorf("expected Block, got %q (%s)", d.Verdict, d.Reason)
	}
}

func TestGate_Allowed_GoodEvidenceHighConfidence(t *testing.T) {
	d := defaultGate().Evaluate(hg.Claim{
		Text:          "The system uses mTLS.",
		EvidenceCount: 3,
		Confidence:    0.95,
	})
	if d.Verdict != hg.VerdictAllowed {
		t.Errorf("expected Allowed, got %q (%s)", d.Verdict, d.Reason)
	}
}

func TestGate_Hedge_SufficientEvidenceLowConfidence(t *testing.T) {
	d := defaultGate().Evaluate(hg.Claim{
		Text:          "The system uses mTLS.",
		EvidenceCount: 2,
		Confidence:    0.5, // below default 0.7
	})
	if d.Verdict != hg.VerdictHedge {
		t.Errorf("expected Hedge, got %q (%s)", d.Verdict, d.Reason)
	}
}

func TestGate_Hedge_OneEvidence(t *testing.T) {
	d := defaultGate().Evaluate(hg.Claim{
		Text:          "Caching is enabled.",
		EvidenceCount: 1,
		Confidence:    0.8,
	})
	if d.Verdict != hg.VerdictHedge {
		t.Errorf("expected Hedge (1 evidence < 2 required), got %q (%s)", d.Verdict, d.Reason)
	}
}

func TestGate_CustomThresholds(t *testing.T) {
	g := hg.Gate{
		MinEvidenceAllowed:   3,
		MinEvidenceHedge:     2,
		MinConfidenceAllowed: 0.8,
	}

	// 1 evidence → Block (below minHedge=2)
	d := g.Evaluate(hg.Claim{Text: "X", EvidenceCount: 1, Confidence: 0.9})
	if d.Verdict != hg.VerdictBlock {
		t.Errorf("expected Block, got %q", d.Verdict)
	}

	// 2 evidence, high confidence → Hedge (below minAllowed=3)
	d = g.Evaluate(hg.Claim{Text: "X", EvidenceCount: 2, Confidence: 0.9})
	if d.Verdict != hg.VerdictHedge {
		t.Errorf("expected Hedge, got %q", d.Verdict)
	}

	// 3 evidence, confidence 0.9 → Allowed
	d = g.Evaluate(hg.Claim{Text: "X", EvidenceCount: 3, Confidence: 0.9})
	if d.Verdict != hg.VerdictAllowed {
		t.Errorf("expected Allowed, got %q", d.Verdict)
	}

	// 3 evidence, confidence 0.75 → Hedge (below minConf=0.8)
	d = g.Evaluate(hg.Claim{Text: "X", EvidenceCount: 3, Confidence: 0.75})
	if d.Verdict != hg.VerdictHedge {
		t.Errorf("expected Hedge (low confidence), got %q", d.Verdict)
	}
}

func TestGate_Deterministic(t *testing.T) {
	g := defaultGate()
	c := hg.Claim{Text: "statement", EvidenceCount: 2, Confidence: 0.8}
	d1 := g.Evaluate(c)
	d2 := g.Evaluate(c)
	if d1.Verdict != d2.Verdict {
		t.Errorf("non-deterministic: %q vs %q", d1.Verdict, d2.Verdict)
	}
}

func TestGate_ReasonNotEmpty(t *testing.T) {
	cases := []hg.Claim{
		{Text: "a", EvidenceCount: 0, Confidence: 0},
		{Text: "b", EvidenceCount: 1, Confidence: 0.5},
		{Text: "c", EvidenceCount: 2, Confidence: 0.9},
	}
	g := defaultGate()
	for _, c := range cases {
		d := g.Evaluate(c)
		if d.Reason == "" {
			t.Errorf("Reason must not be empty for claim %q (verdict=%q)", c.Text, d.Verdict)
		}
	}
}
