package confidencescoring_test

import (
	"testing"

	cs "github.com/geoffmilleraz/agentic-kit/foundations/review/confidence-scoring"
)

func TestValidate_ValidRange(t *testing.T) {
	for score := 1; score <= 5; score++ {
		if err := cs.ConfidenceScore(score).Validate(); err != nil {
			t.Errorf("score %d should be valid, got error: %v", score, err)
		}
	}
}

func TestValidate_InvalidRange(t *testing.T) {
	for _, bad := range []int{0, 6, -1, 100} {
		if err := cs.ConfidenceScore(bad).Validate(); err == nil {
			t.Errorf("score %d should be invalid, got nil error", bad)
		}
	}
}

func TestDerive_ZeroCount(t *testing.T) {
	score, err := cs.Derive(cs.EvidenceInput{Count: 0, AgreementRatio: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1 {
		t.Errorf("0 items → expected 1, got %d", score)
	}
}

func TestDerive_MajorityDisagrees(t *testing.T) {
	score, err := cs.Derive(cs.EvidenceInput{Count: 10, AgreementRatio: 0.4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1 {
		t.Errorf("agreement 0.4 → expected 1, got %d", score)
	}
}

func TestDerive_OneItem(t *testing.T) {
	score, err := cs.Derive(cs.EvidenceInput{Count: 1, AgreementRatio: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 2 {
		t.Errorf("1 item, agreement 0.5 → expected 2, got %d", score)
	}
}

func TestDerive_TwoThreeItems(t *testing.T) {
	for _, n := range []int{2, 3} {
		score, err := cs.Derive(cs.EvidenceInput{Count: n, AgreementRatio: 0.6})
		if err != nil {
			t.Fatalf("n=%d: unexpected error: %v", n, err)
		}
		if score != 3 {
			t.Errorf("n=%d, agreement 0.6 → expected 3, got %d", n, score)
		}
	}
}

func TestDerive_FourToSixItems(t *testing.T) {
	for _, n := range []int{4, 5, 6} {
		score, err := cs.Derive(cs.EvidenceInput{Count: n, AgreementRatio: 0.7})
		if err != nil {
			t.Fatalf("n=%d: unexpected error: %v", n, err)
		}
		if score != 4 {
			t.Errorf("n=%d, agreement 0.7 → expected 4, got %d", n, score)
		}
	}
}

func TestDerive_SevenPlusItems(t *testing.T) {
	score, err := cs.Derive(cs.EvidenceInput{Count: 7, AgreementRatio: 0.6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 5 {
		t.Errorf("7 items, agreement 0.6 → expected 5, got %d", score)
	}
}

func TestDerive_HighAgreementBonus(t *testing.T) {
	// 1 item with 90%+ agreement: base 2 + bonus = 3
	score, err := cs.Derive(cs.EvidenceInput{Count: 1, AgreementRatio: 0.9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 3 {
		t.Errorf("1 item, agreement 0.9 → expected 3 (2+bonus), got %d", score)
	}
}

func TestDerive_HighAgreementBonusCappedAt5(t *testing.T) {
	// 7+ items already at 5, bonus must not push above 5
	score, err := cs.Derive(cs.EvidenceInput{Count: 10, AgreementRatio: 0.95})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 5 {
		t.Errorf("10 items, agreement 0.95 → expected 5 (capped), got %d", score)
	}
	if err := score.Validate(); err != nil {
		t.Errorf("derived score failed Validate: %v", err)
	}
}

func TestDerive_Deterministic(t *testing.T) {
	input := cs.EvidenceInput{Count: 4, AgreementRatio: 0.75}
	s1, err1 := cs.Derive(input)
	s2, err2 := cs.Derive(input)
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v, %v", err1, err2)
	}
	if s1 != s2 {
		t.Errorf("Derive is not deterministic: %d vs %d", s1, s2)
	}
}

func TestDerive_InvalidAgreementRatio(t *testing.T) {
	for _, ratio := range []float64{-0.1, 1.01, 2.0} {
		_, err := cs.Derive(cs.EvidenceInput{Count: 3, AgreementRatio: ratio})
		if err == nil {
			t.Errorf("expected error for AgreementRatio %v, got nil", ratio)
		}
	}
}

func TestDerive_NegativeCountRejected(t *testing.T) {
	_, err := cs.Derive(cs.EvidenceInput{Count: -1, AgreementRatio: 0.5})
	if err == nil {
		t.Error("expected error for negative Count, got nil")
	}
}
