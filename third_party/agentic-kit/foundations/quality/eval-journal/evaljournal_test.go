package evaljournal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// fixedClock returns a function that always returns t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

var epoch = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

func minimalRecord() Record {
	return Record{
		RunID:        "run-1",
		ScenarioID:   "scenario-1",
		Producer:     Producer{Model: "model-a"},
		Judge:        Judge{Model: "judge-b", Family: "family-b"},
		Verdict:      VerdictPass,
		Decisive:     true,
		Rationale:    "looks good",
		EvidenceRefs: []string{},
		Trace:        Trace{},
		OccurredAt:   epoch,
	}
}

// --- Validate ---

func TestValidate_valid(t *testing.T) {
	if err := minimalRecord().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_emptyRunID(t *testing.T) {
	r := minimalRecord()
	r.RunID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty RunID")
	}
}

func TestValidate_emptyScenarioID(t *testing.T) {
	r := minimalRecord()
	r.ScenarioID = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty ScenarioID")
	}
}

func TestValidate_emptyProducerModel(t *testing.T) {
	r := minimalRecord()
	r.Producer.Model = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty Producer.Model")
	}
}

func TestValidate_emptyJudgeModel(t *testing.T) {
	r := minimalRecord()
	r.Judge.Model = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty Judge.Model")
	}
}

func TestValidate_emptyJudgeFamily(t *testing.T) {
	r := minimalRecord()
	r.Judge.Family = ""
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for empty Judge.Family")
	}
}

func TestValidate_unknownVerdict(t *testing.T) {
	r := minimalRecord()
	r.Verdict = "bogus"
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for unknown Verdict")
	}
}

func TestValidate_emptyRationaleWithDecisiveTrue(t *testing.T) {
	r := minimalRecord()
	r.Rationale = ""
	r.Decisive = true
	if err := r.Validate(); err == nil {
		t.Fatal("expected error: empty rationale + decisive=true")
	}
}

func TestValidate_emptyRationaleWithDecisiveFalse(t *testing.T) {
	r := minimalRecord()
	r.Rationale = ""
	r.Decisive = false
	r.Verdict = VerdictAbstain
	if err := r.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_scoreOutOfRange(t *testing.T) {
	r := minimalRecord()
	bad := 1.5
	r.Score = &bad
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for score > 1")
	}
}

func TestValidate_scoreNegative(t *testing.T) {
	r := minimalRecord()
	bad := -0.1
	r.Score = &bad
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for score < 0")
	}
}

func TestValidate_zeroOccurredAt(t *testing.T) {
	r := minimalRecord()
	r.OccurredAt = time.Time{}
	if err := r.Validate(); err == nil {
		t.Fatal("expected error for zero OccurredAt")
	}
}

func TestValidate_allVerdicts(t *testing.T) {
	verdicts := []Verdict{VerdictPass, VerdictFail, VerdictAbsentProven, VerdictAbstain, VerdictError}
	for _, v := range verdicts {
		r := minimalRecord()
		r.Verdict = v
		if v == VerdictAbstain || v == VerdictError {
			r.Rationale = ""
			r.Decisive = false
		}
		if err := r.Validate(); err != nil {
			t.Errorf("verdict %q: unexpected error: %v", v, err)
		}
	}
}

// --- Append / Load round-trip ---

func TestAppendLoad_roundTrip(t *testing.T) {
	rec := minimalRecord()
	score := 0.9
	rec.Score = &score
	rec.EvidenceRefs = []string{"ref-1", "ref-2"}

	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)
	if err := Append(w, rec); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := Load(&buf)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Load returned %d records, want 1", len(got))
	}

	g := got[0]
	if g.RunID != rec.RunID {
		t.Errorf("RunID: got %q, want %q", g.RunID, rec.RunID)
	}
	if g.Verdict != rec.Verdict {
		t.Errorf("Verdict: got %q, want %q", g.Verdict, rec.Verdict)
	}
	if g.Score == nil || *g.Score != *rec.Score {
		t.Errorf("Score: got %v, want %v", g.Score, rec.Score)
	}
	if len(g.EvidenceRefs) != 2 {
		t.Errorf("EvidenceRefs: got %v, want 2 entries", g.EvidenceRefs)
	}
	// OccurredAt must survive round-trip (RFC3339 with second precision)
	if !g.OccurredAt.Equal(rec.OccurredAt) {
		t.Errorf("OccurredAt: got %v, want %v", g.OccurredAt, rec.OccurredAt)
	}
}

func TestAppendLoad_multipleRecords(t *testing.T) {
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)

	for i := 0; i < 5; i++ {
		r := minimalRecord()
		r.ScenarioID = "scenario-" + string(rune('A'+i))
		r.OccurredAt = epoch.Add(time.Duration(i) * time.Second)
		if err := Append(w, r); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	got, err := Load(&buf)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d records, want 5", len(got))
	}
}

func TestAppend_rejectInvalid(t *testing.T) {
	r := minimalRecord()
	r.RunID = ""
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)
	if err := Append(w, r); err == nil {
		t.Fatal("expected error for invalid record")
	}
	if buf.Len() != 0 {
		t.Fatal("nothing should have been written on validation failure")
	}
}

func TestLoad_tornTail(t *testing.T) {
	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)
	if err := Append(w, minimalRecord()); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Append partial bytes simulating a torn write.
	buf.WriteString(`{"run_id":"torn"`)

	got, err := Load(&buf)
	if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		t.Fatalf("expected ErrPartialTrailingLine, got: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 complete record before torn tail, got %d", len(got))
	}
}

// --- Golden / byte-stable marshal ---

func TestMarshal_byteStable(t *testing.T) {
	score := 0.75
	rec := Record{
		RunID:      "run-golden",
		ScenarioID: "scenario-golden",
		Producer: Producer{
			Model:      "producer-x",
			PromptHash: "sha256:abcdef",
		},
		Judge:        Judge{Model: "judge-y", Family: "family-y"},
		Verdict:      VerdictFail,
		Decisive:     true,
		Score:        &score,
		Rationale:    "missing required section",
		EvidenceRefs: []string{"span://a.go#L1", "span://b.go#L5"},
		Trace:        Trace{RunID: "t1", SpanID: "s1"},
		OccurredAt:   epoch,
	}

	b1, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b2, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("marshal not byte-stable:\n%s\n%s", b1, b2)
	}

	// Spot-check key fields appear in output.
	s := string(b1)
	for _, want := range []string{`"run_id":"run-golden"`, `"verdict":"fail"`, `"decisive":true`} {
		if !strings.Contains(s, want) {
			t.Errorf("marshalled output missing %q:\n%s", want, s)
		}
	}
}

// --- NewWithClock ---

func TestNewWithClock(t *testing.T) {
	r := minimalRecord()
	r.OccurredAt = time.Time{} // zero it out
	clock := fixedClock(epoch)
	got := NewWithClock(clock, r)
	if !got.OccurredAt.Equal(epoch) {
		t.Errorf("OccurredAt: got %v, want %v", got.OccurredAt, epoch)
	}
}
