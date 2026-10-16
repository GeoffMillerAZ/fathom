package scenariobank

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	evaljournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/eval-journal"
	qaharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/qa-harness"
)

var epoch = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

func syntheticScenario(id string) Scenario {
	return Scenario{
		ID:               id,
		Version:          1,
		Provenance:       SyntheticAdversarial,
		DecisiveEligible: false,
		Input:            json.RawMessage(`{"prompt":"hello"}`),
	}
}

func redactedScenario(id string) Scenario {
	return Scenario{
		ID:               id,
		Version:          1,
		Provenance:       ProductionDerivedRedacted,
		DecisiveEligible: true,
		Input:            json.RawMessage(`{"prompt":"world"}`),
	}
}

// --- Validate ---

func TestValidate_validSynthetic(t *testing.T) {
	if err := syntheticScenario("s1").Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_validRedacted(t *testing.T) {
	if err := redactedScenario("s1").Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_productionCapturedRejected(t *testing.T) {
	s := syntheticScenario("s1")
	s.Provenance = ProductionCaptured
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for production-captured scenario")
	}
}

func TestValidate_syntheticDecisiveRejected(t *testing.T) {
	s := syntheticScenario("s1")
	s.DecisiveEligible = true
	if err := s.Validate(); err == nil {
		t.Fatal("expected error: synthetic-adversarial with decisive_eligible=true")
	}
}

func TestValidate_emptyID(t *testing.T) {
	s := syntheticScenario("")
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestValidate_versionZero(t *testing.T) {
	s := syntheticScenario("s1")
	s.Version = 0
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for version=0")
	}
}

func TestValidate_emptyInput(t *testing.T) {
	s := syntheticScenario("s1")
	s.Input = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestValidate_invalidInputJSON(t *testing.T) {
	s := syntheticScenario("s1")
	s.Input = json.RawMessage(`{not valid}`)
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid JSON input")
	}
}

func TestValidate_badRegexPredicate(t *testing.T) {
	s := syntheticScenario("s1")
	s.Rubric = &Rubric{Criteria: []Criterion{{
		Name:      "check",
		Weight:    1.0,
		Predicate: Predicate{Kind: PredicateRegex, Arg: `[invalid`},
	}}}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for invalid regex in predicate")
	}
}

// --- Load ---

func TestLoad_valid(t *testing.T) {
	data, _ := json.Marshal([]Scenario{
		syntheticScenario("s1"),
		redactedScenario("s2"),
	})
	b, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b.All()) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(b.All()))
	}
}

func TestLoad_rejectsProductionCaptured(t *testing.T) {
	s := syntheticScenario("s1")
	s.Provenance = ProductionCaptured
	data, _ := json.Marshal([]Scenario{s})
	if _, err := Load(data); err == nil {
		t.Fatal("expected Load to reject production-captured scenario")
	}
}

func TestLoad_empty(t *testing.T) {
	b, err := Load([]byte(`[]`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b.All()) != 0 {
		t.Fatalf("expected 0 scenarios")
	}
}

// --- Filter ---

func TestFilter_byProvenance(t *testing.T) {
	data, _ := json.Marshal([]Scenario{
		syntheticScenario("s1"),
		syntheticScenario("s2"),
		redactedScenario("r1"),
	})
	b, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := b.Filter(FilterOptions{Provenance: SyntheticAdversarial})
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

func TestFilter_byIDs(t *testing.T) {
	data, _ := json.Marshal([]Scenario{
		syntheticScenario("s1"),
		syntheticScenario("s2"),
		syntheticScenario("s3"),
	})
	b, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := b.Filter(FilterOptions{IDs: map[string]struct{}{"s1": {}, "s3": {}}})
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

// --- RubricJudgeFrom ---

func TestRubricJudgeFrom_contains(t *testing.T) {
	r := Rubric{Criteria: []Criterion{{
		Name:      "has-hello",
		Weight:    1.0,
		Predicate: Predicate{Kind: PredicateContains, Arg: "hello"},
	}}}
	j, err := RubricJudgeFrom(r)
	if err != nil {
		t.Fatalf("RubricJudgeFrom: %v", err)
	}
	ctx := context.Background()
	score, err := j.Score(ctx, qaharness.Item{ID: "i1", Output: "say hello world"})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score != 1.0 {
		t.Errorf("score: got %g, want 1.0", score)
	}
	score, _ = j.Score(ctx, qaharness.Item{ID: "i2", Output: "goodbye"})
	if score != 0.0 {
		t.Errorf("score for non-match: got %g, want 0.0", score)
	}
}

func TestRubricJudgeFrom_regex(t *testing.T) {
	r := Rubric{Criteria: []Criterion{{
		Name:      "starts-with-capital",
		Weight:    1.0,
		Predicate: Predicate{Kind: PredicateRegex, Arg: `^[A-Z]`},
	}}}
	j, err := RubricJudgeFrom(r)
	if err != nil {
		t.Fatalf("RubricJudgeFrom: %v", err)
	}
	ctx := context.Background()
	score, _ := j.Score(ctx, qaharness.Item{ID: "i1", Output: "Hello world"})
	if score != 1.0 {
		t.Errorf("score: got %g, want 1.0", score)
	}
	score, _ = j.Score(ctx, qaharness.Item{ID: "i2", Output: "hello world"})
	if score != 0.0 {
		t.Errorf("score for non-match: got %g, want 0.0", score)
	}
}

func TestRubricJudgeFrom_nonempty(t *testing.T) {
	r := Rubric{Criteria: []Criterion{{
		Name:      "non-empty",
		Weight:    1.0,
		Predicate: Predicate{Kind: PredicateNonempty, Arg: ""},
	}}}
	j, err := RubricJudgeFrom(r)
	if err != nil {
		t.Fatalf("RubricJudgeFrom: %v", err)
	}
	ctx := context.Background()
	score, _ := j.Score(ctx, qaharness.Item{ID: "i1", Output: "some text"})
	if score != 1.0 {
		t.Errorf("score: got %g, want 1.0", score)
	}
	score, _ = j.Score(ctx, qaharness.Item{ID: "i2", Output: "   "})
	if score != 0.0 {
		t.Errorf("whitespace-only score: got %g, want 0.0", score)
	}
}

func TestRubricJudgeFrom_jsonPathEq(t *testing.T) {
	r := Rubric{Criteria: []Criterion{{
		Name:      "status-ok",
		Weight:    1.0,
		Predicate: Predicate{Kind: PredicateJSONPathEq, Arg: "/status", Value: "ok"},
	}}}
	j, err := RubricJudgeFrom(r)
	if err != nil {
		t.Fatalf("RubricJudgeFrom: %v", err)
	}
	ctx := context.Background()
	score, _ := j.Score(ctx, qaharness.Item{ID: "i1", Output: `{"status":"ok"}`})
	if score != 1.0 {
		t.Errorf("score: got %g, want 1.0", score)
	}
	score, _ = j.Score(ctx, qaharness.Item{ID: "i2", Output: `{"status":"error"}`})
	if score != 0.0 {
		t.Errorf("score for non-match: got %g, want 0.0", score)
	}
}

func TestRubricJudgeFrom_emptyRubric(t *testing.T) {
	j, err := RubricJudgeFrom(Rubric{})
	if err != nil {
		t.Fatalf("RubricJudgeFrom: %v", err)
	}
	score, err := j.Score(context.Background(), qaharness.Item{ID: "i1", Output: "anything"})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if score != 0.0 {
		t.Errorf("empty rubric score: got %g, want 0.0", score)
	}
}

// --- HistoryFor ---

func TestHistoryFor(t *testing.T) {
	makeRec := func(scenID string, at time.Time) evaljournal.Record {
		return evaljournal.Record{
			RunID:        "run-1",
			ScenarioID:   scenID,
			Producer:     evaljournal.Producer{Model: "m"},
			Judge:        evaljournal.Judge{Model: "j", Family: "f"},
			Verdict:      evaljournal.VerdictPass,
			Decisive:     true,
			Rationale:    "ok",
			EvidenceRefs: []string{},
			OccurredAt:   at,
		}
	}
	recs := []evaljournal.Record{
		makeRec("s1", epoch),
		makeRec("s2", epoch.Add(time.Second)),
		makeRec("s1", epoch.Add(2*time.Second)),
		makeRec("s3", epoch.Add(3*time.Second)),
	}
	got := HistoryFor("s1", recs)
	if len(got) != 2 {
		t.Fatalf("HistoryFor: got %d, want 2", len(got))
	}
	if got[0].OccurredAt != epoch {
		t.Errorf("got[0].OccurredAt: got %v, want %v", got[0].OccurredAt, epoch)
	}

	got2 := HistoryFor("missing", recs)
	if got2 != nil {
		t.Fatalf("HistoryFor for missing ID: got %v, want nil", got2)
	}
}
