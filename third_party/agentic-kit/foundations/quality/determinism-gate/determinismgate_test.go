package determinismgate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	qaharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
	scenariobank "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/scenario-bank"
)

// seededJudge returns a MockJudge with a fixed score for deterministic tests.
func seededJudge(score float64) qaharness.Judge {
	return qaharness.MockJudge{FixedScore: score}
}

// variantJudge cycles through scores on each call using a closure.
type cycleJudge struct {
	scores []float64
	idx    int
}

func (c *cycleJudge) Score(_ context.Context, _ qaharness.Item) (float64, error) {
	s := c.scores[c.idx%len(c.scores)]
	c.idx++
	return s, nil
}

// errorJudge always returns an error.
type errorJudge struct{}

func (e errorJudge) Score(_ context.Context, _ qaharness.Item) (float64, error) {
	return 0, errors.New("judge error")
}

func syntheticScenario(id string) scenariobank.Scenario {
	return scenariobank.Scenario{
		ID:               id,
		Version:          1,
		Provenance:       scenariobank.SyntheticAdversarial,
		DecisiveEligible: false,
		Input:            json.RawMessage(`{"prompt":"test"}`),
	}
}

// --- K validation ---

func TestProbe_KLessThanOne(t *testing.T) {
	_, err := Probe(context.Background(), seededJudge(0.9), syntheticScenario("s1"), 0, ProbeOptions{})
	if err == nil {
		t.Fatal("expected error for K=0")
	}
}

// --- Stable outcome (all runs agree) ---

func TestProbe_allAgreement(t *testing.T) {
	j := seededJudge(0.9) // always returns 0.9 → "pass"
	r, err := Probe(context.Background(), j, syntheticScenario("s1"), 5, ProbeOptions{StabilityBand: 0.9})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.AgreementRate != 1.0 {
		t.Errorf("AgreementRate: got %g, want 1.0", r.AgreementRate)
	}
	if r.VerdictFlipCount != 0 {
		t.Errorf("VerdictFlipCount: got %d, want 0", r.VerdictFlipCount)
	}
	if r.Stability != StabilityStable {
		t.Errorf("Stability: got %q, want Stable", r.Stability)
	}
	if r.StabilityBand != 0.9 {
		t.Errorf("StabilityBand: got %g, want 0.9", r.StabilityBand)
	}
	if r.K != 5 {
		t.Errorf("K: got %d, want 5", r.K)
	}
}

// --- Unstable outcome (seeded variance) ---

func TestProbe_highVariance(t *testing.T) {
	// Alternate 0.9 (pass) and 0.1 (fail) → 50% agreement
	j := &cycleJudge{scores: []float64{0.9, 0.1}}
	r, err := Probe(context.Background(), j, syntheticScenario("s2"), 4, ProbeOptions{StabilityBand: 0.9})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	// 2 pass, 2 fail → (0,1), (0,3): agree; (1,2), (1,3): fail; ... let's verify
	// pairs: (0,1)=pass/fail disagree, (0,2)=pass/pass agree, (0,3)=pass/fail disagree,
	//        (1,2)=fail/pass disagree, (1,3)=fail/fail agree, (2,3)=pass/fail disagree
	// agree=2, total=6, rate=0.333
	if r.Stability != StabilityUnstable {
		t.Errorf("expected Unstable, got %q (agreement_rate=%g)", r.Stability, r.AgreementRate)
	}
	if r.VerdictFlipCount == 0 {
		t.Error("expected non-zero VerdictFlipCount")
	}
}

// --- K=1 trivially stable ---

func TestProbe_K1(t *testing.T) {
	j := seededJudge(0.9)
	r, err := Probe(context.Background(), j, syntheticScenario("s3"), 1, ProbeOptions{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.AgreementRate != 1.0 {
		t.Errorf("K=1 AgreementRate: got %g, want 1.0", r.AgreementRate)
	}
	if r.Stability != StabilityStable {
		t.Errorf("K=1: expected Stable, got %q", r.Stability)
	}
}

// --- Default band is 1.0 ---

func TestProbe_defaultBandIsOne(t *testing.T) {
	// score 0.9 always → all agree → rate=1.0 >= 1.0 → stable
	j := seededJudge(0.9)
	r, _ := Probe(context.Background(), j, syntheticScenario("s4"), 3, ProbeOptions{})
	if r.StabilityBand != 1.0 {
		t.Errorf("default StabilityBand: got %g, want 1.0", r.StabilityBand)
	}
}

// --- SchemaValidator integration ---

func TestProbe_schemaValidatorFiltersInvalidOutputs(t *testing.T) {
	// Score always returns 0.9, but our validator always rejects.
	j := seededJudge(0.9)
	opts := ProbeOptions{
		SchemaValidator: func(_ string) error {
			return errors.New("schema invalid")
		},
	}
	r, err := Probe(context.Background(), j, syntheticScenario("s5"), 5, opts)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	// All runs filtered → degenerate result (Unstable because we couldn't measure).
	if r.Stability != StabilityUnstable {
		t.Errorf("expected Unstable when all runs filtered, got %q", r.Stability)
	}
}

func TestProbe_schemaValidatorAllValid(t *testing.T) {
	j := seededJudge(0.9)
	opts := ProbeOptions{
		StabilityBand:   0.9,
		SchemaValidator: func(_ string) error { return nil }, // always valid
	}
	r, err := Probe(context.Background(), j, syntheticScenario("s6"), 5, opts)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.Stability != StabilityStable {
		t.Errorf("all valid → expected Stable, got %q", r.Stability)
	}
}

// --- Judge error ---

func TestProbe_judgeErrors(t *testing.T) {
	j := errorJudge{}
	r, err := Probe(context.Background(), j, syntheticScenario("s7"), 5, ProbeOptions{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	// All runs errored → degenerate Unstable result.
	if r.Stability != StabilityUnstable {
		t.Errorf("all errors → expected Unstable, got %q", r.Stability)
	}
}

// --- Context cancellation ---

func TestProbe_contextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	j := seededJudge(0.9)
	_, err := Probe(ctx, j, syntheticScenario("s8"), 5, ProbeOptions{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// --- PerFieldEntropy ---

func TestProbe_entropyAllSame(t *testing.T) {
	j := seededJudge(0.75)
	r, err := Probe(context.Background(), j, syntheticScenario("s9"), 5, ProbeOptions{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	// All same score → entropy 0 for both fields.
	if r.PerFieldEntropy["verdict"] != 0.0 {
		t.Errorf("verdict entropy: got %g, want 0.0", r.PerFieldEntropy["verdict"])
	}
}

func TestProbe_entropyMixed(t *testing.T) {
	// 50/50 pass/fail → entropy == 1.0 for verdict
	j := &cycleJudge{scores: []float64{0.9, 0.1}}
	r, err := Probe(context.Background(), j, syntheticScenario("s10"), 4, ProbeOptions{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	ve := r.PerFieldEntropy["verdict"]
	if ve == 0.0 {
		t.Errorf("verdict entropy: expected > 0 for 50/50 split, got 0")
	}
}

// --- ScenarioID echoed ---

func TestProbe_scenarioIDEchoed(t *testing.T) {
	j := seededJudge(0.9)
	r, err := Probe(context.Background(), j, syntheticScenario("my-scenario"), 3, ProbeOptions{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r.ScenarioID != "my-scenario" {
		t.Errorf("ScenarioID: got %q, want \"my-scenario\"", r.ScenarioID)
	}
}
