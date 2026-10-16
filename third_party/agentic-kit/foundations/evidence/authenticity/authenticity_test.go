package authenticity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/authenticity"
	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var fixedTime = time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

// humanAuthor returns a valid human author for test signals.
func humanAuthor() aiprovenance.Author {
	return aiprovenance.NewHuman("analyst@example.com")
}

// aiAuthor returns a valid AI author for test signals.
func aiAuthor() aiprovenance.Author {
	return aiprovenance.NewAI(
		"claude-sonnet-4-6",
		"20260608",
		"sha256-promptAAA",
		"sha256-paramsAAA",
		"sha256-inputAAA",
		fixedTime,
	)
}

// makeSignal builds a minimal valid Signal with a human author.
func makeSignal(id, recordID string, method authenticity.Method, verdict authenticity.Verdict, score float64) authenticity.Signal {
	return authenticity.Signal{
		ID:       id,
		RecordID: recordID,
		Method:   method,
		Verdict:  verdict,
		Score:    score,
		Detail:   "test detail",
		Author:   humanAuthor(),
		At:       fixedTime,
	}
}

// ---------------------------------------------------------------------------
// Method validation
// ---------------------------------------------------------------------------

func TestMethodValidate(t *testing.T) {
	valid := []authenticity.Method{
		authenticity.MethodHashMatch,
		authenticity.MethodC2PA,
		authenticity.MethodEXIFForensics,
		authenticity.MethodAITextDetector,
		authenticity.MethodDeepfakeDetector,
		authenticity.MethodMetadataForensics,
		authenticity.MethodManualReview,
		authenticity.MethodDKIM,
		authenticity.MethodAuthResultsHeader,
	}
	for _, m := range valid {
		t.Run(string(m), func(t *testing.T) {
			if err := m.Validate(); err != nil {
				t.Errorf("expected no error for %q, got %v", m, err)
			}
		})
	}
	if err := authenticity.Method("unknown_method").Validate(); err == nil {
		t.Error("expected error for unknown Method")
	}
}

// ---------------------------------------------------------------------------
// Verdict validation + ordering
// ---------------------------------------------------------------------------

func TestVerdictValidate(t *testing.T) {
	valid := []authenticity.Verdict{
		authenticity.VerdictAuthentic,
		authenticity.VerdictLikelyAuthentic,
		authenticity.VerdictInconclusive,
		authenticity.VerdictLikelySynthetic,
		authenticity.VerdictSynthetic,
		authenticity.VerdictTampered,
	}
	for _, v := range valid {
		t.Run(string(v), func(t *testing.T) {
			if err := v.Validate(); err != nil {
				t.Errorf("expected no error for %q, got %v", v, err)
			}
		})
	}
	if err := authenticity.Verdict("made_up").Validate(); err == nil {
		t.Error("expected error for unknown Verdict")
	}
}

func TestVerdictWorseOrdering(t *testing.T) {
	// VerdictSynthetic and VerdictTampered must be "worse" (higher adversity rank)
	// than VerdictAuthentic.
	cases := []struct {
		a, b      authenticity.Verdict
		wantWorse bool // a.IsWorseOrEqual(b) should be true
	}{
		{authenticity.VerdictSynthetic, authenticity.VerdictAuthentic, true},
		{authenticity.VerdictTampered, authenticity.VerdictAuthentic, true},
		{authenticity.VerdictLikelySynthetic, authenticity.VerdictAuthentic, true},
		{authenticity.VerdictAuthentic, authenticity.VerdictSynthetic, false},
		{authenticity.VerdictAuthentic, authenticity.VerdictAuthentic, true}, // equal is "worse or equal"
		{authenticity.VerdictInconclusive, authenticity.VerdictLikelyAuthentic, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.a)+"_vs_"+string(tc.b), func(t *testing.T) {
			got := tc.a.IsWorseOrEqual(tc.b)
			if got != tc.wantWorse {
				t.Errorf("%q.IsWorseOrEqual(%q) = %v, want %v", tc.a, tc.b, got, tc.wantWorse)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Signal.Validate and ValidateStrict
// ---------------------------------------------------------------------------

func TestSignalValidate(t *testing.T) {
	cases := []struct {
		name    string
		sig     authenticity.Signal
		wantErr bool
	}{
		{
			name:    "valid_with_author",
			sig:     makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.99),
			wantErr: false,
		},
		{
			name: "valid_no_author",
			sig: authenticity.Signal{
				ID: "s2", RecordID: "rec-1",
				Method: authenticity.MethodC2PA, Verdict: authenticity.VerdictInconclusive,
				Score: 0.5, At: fixedTime,
				// Author zero-value: Validate (non-strict) allows this
			},
			wantErr: false,
		},
		{
			name: "empty_id",
			sig: authenticity.Signal{
				ID: "", RecordID: "rec-1",
				Method: authenticity.MethodHashMatch, Verdict: authenticity.VerdictAuthentic,
				Score: 0.9, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "empty_record_id",
			sig: authenticity.Signal{
				ID: "s3", RecordID: "",
				Method: authenticity.MethodHashMatch, Verdict: authenticity.VerdictAuthentic,
				Score: 0.9, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "invalid_method",
			sig: authenticity.Signal{
				ID: "s4", RecordID: "rec-1",
				Method: "bogus", Verdict: authenticity.VerdictAuthentic,
				Score: 0.9, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "invalid_verdict",
			sig: authenticity.Signal{
				ID: "s5", RecordID: "rec-1",
				Method: authenticity.MethodHashMatch, Verdict: "made_up",
				Score: 0.9, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "score_negative",
			sig: authenticity.Signal{
				ID: "s6", RecordID: "rec-1",
				Method: authenticity.MethodHashMatch, Verdict: authenticity.VerdictAuthentic,
				Score: -0.1, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "score_above_one",
			sig: authenticity.Signal{
				ID: "s7", RecordID: "rec-1",
				Method: authenticity.MethodHashMatch, Verdict: authenticity.VerdictAuthentic,
				Score: 1.1, Author: humanAuthor(), At: fixedTime,
			},
			wantErr: true,
		},
		{
			name: "zero_time",
			sig: authenticity.Signal{
				ID: "s8", RecordID: "rec-1",
				Method: authenticity.MethodHashMatch, Verdict: authenticity.VerdictAuthentic,
				Score: 0.9, Author: humanAuthor(),
				// At is zero value
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sig.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSignalValidateStrict_RequiresAuthor(t *testing.T) {
	// ValidateStrict must reject a signal with a zero-value Author.
	sig := authenticity.Signal{
		ID: "s-strict", RecordID: "rec-1",
		Method:  authenticity.MethodAITextDetector,
		Verdict: authenticity.VerdictLikelySynthetic,
		Score:   0.85,
		At:      fixedTime,
		// Author intentionally omitted
	}
	err := sig.ValidateStrict()
	if err == nil {
		t.Fatal("ValidateStrict must return error when Author is zero-value")
	}
	if !strings.Contains(err.Error(), "Author") {
		t.Errorf("error %q should mention Author", err.Error())
	}
}

func TestSignalValidateStrict_AcceptsValidAuthor(t *testing.T) {
	sig := makeSignal("s-strict-ok", "rec-1", authenticity.MethodManualReview, authenticity.VerdictAuthentic, 1.0)
	if err := sig.ValidateStrict(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Assess: empty → error
// ---------------------------------------------------------------------------

func TestAssessEmpty(t *testing.T) {
	_, err := authenticity.Assess(nil)
	if err == nil {
		t.Fatal("expected error for empty signals")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Errorf("error %q should mention 'at least one'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Assess: mismatched RecordIDs → error
// ---------------------------------------------------------------------------

func TestAssessMixedRecordIDs(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-A", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.99),
		makeSignal("s2", "rec-B", authenticity.MethodC2PA, authenticity.VerdictAuthentic, 0.90),
	}
	_, err := authenticity.Assess(signals)
	if err == nil {
		t.Fatal("expected error when signals reference different RecordIDs")
	}
	if !strings.Contains(err.Error(), "RecordID") {
		t.Errorf("error %q should mention RecordID", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Assess: one credible 'tampered' dominates pile of 'authentic'
// ---------------------------------------------------------------------------

func TestAssessTamperedDominates(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.99),
		makeSignal("s2", "rec-1", authenticity.MethodC2PA, authenticity.VerdictAuthentic, 0.95),
		makeSignal("s3", "rec-1", authenticity.MethodManualReview, authenticity.VerdictAuthentic, 0.88),
		// One high-confidence tampered signal must dominate
		makeSignal("s4", "rec-1", authenticity.MethodEXIFForensics, authenticity.VerdictTampered, 0.80),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictTampered {
		t.Errorf("Overall = %q, want tampered; tampered must dominate authentic signals", a.Overall)
	}
	if !a.Conservative {
		t.Error("Conservative should be true when an adverse signal dominates")
	}
}

// ---------------------------------------------------------------------------
// Assess: one credible 'synthetic' dominates pile of 'authentic'
// ---------------------------------------------------------------------------

func TestAssessSyntheticDominates(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.95),
		makeSignal("s2", "rec-1", authenticity.MethodC2PA, authenticity.VerdictAuthentic, 0.92),
		makeSignal("s3", "rec-1", authenticity.MethodAITextDetector, authenticity.VerdictSynthetic, 0.75),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictSynthetic {
		t.Errorf("Overall = %q, want synthetic", a.Overall)
	}
	if !a.Conservative {
		t.Error("Conservative should be true when an adverse signal dominates")
	}
}

// ---------------------------------------------------------------------------
// Assess: multiple independent 'authentic' signals raise confidence
// ---------------------------------------------------------------------------

func TestAssessMultipleAuthenticRaiseConfidence(t *testing.T) {
	// Single authentic signal: baseline.
	single := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.70),
	}
	aSingle, err := authenticity.Assess(single)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple authentic signals with independent methods: confidence should be higher.
	multi := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.70),
		makeSignal("s2", "rec-1", authenticity.MethodC2PA, authenticity.VerdictAuthentic, 0.80),
		makeSignal("s3", "rec-1", authenticity.MethodManualReview, authenticity.VerdictAuthentic, 0.90),
	}
	aMulti, err := authenticity.Assess(multi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if aMulti.Confidence <= aSingle.Confidence {
		t.Errorf("multiple authentic signals should raise confidence: single=%g multi=%g",
			aSingle.Confidence, aMulti.Confidence)
	}
	if aMulti.Overall != authenticity.VerdictAuthentic {
		t.Errorf("Overall = %q, want authentic", aMulti.Overall)
	}
}

// ---------------------------------------------------------------------------
// Assess: inconclusive-only → inconclusive
// ---------------------------------------------------------------------------

func TestAssessInconclusiveOnly(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodDeepfakeDetector, authenticity.VerdictInconclusive, 0.5),
		makeSignal("s2", "rec-1", authenticity.MethodMetadataForensics, authenticity.VerdictInconclusive, 0.6),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictInconclusive {
		t.Errorf("Overall = %q, want inconclusive", a.Overall)
	}
}

// ---------------------------------------------------------------------------
// Assess: single low-confidence adverse signal is flagged (not hard-dominant)
// but still conservative when Score matters
// ---------------------------------------------------------------------------

func TestAssessLowConfidenceAdverse(t *testing.T) {
	// Many strong authentic + one very-low-confidence adverse.
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.99),
		makeSignal("s2", "rec-1", authenticity.MethodC2PA, authenticity.VerdictAuthentic, 0.97),
		makeSignal("s3", "rec-1", authenticity.MethodManualReview, authenticity.VerdictAuthentic, 0.95),
		// Very low confidence adverse — should not fully dominate due to near-zero score.
		makeSignal("s4", "rec-1", authenticity.MethodAITextDetector, authenticity.VerdictLikelySynthetic, 0.05),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The assessment must surface the adverse signal in Signals (defensive copy check).
	found := false
	for _, s := range a.Signals {
		if s.Verdict == authenticity.VerdictLikelySynthetic {
			found = true
			break
		}
	}
	if !found {
		t.Error("adverse signal must appear in Assessment.Signals")
	}
}

// ---------------------------------------------------------------------------
// Assess: signal with invalid fields → error
// ---------------------------------------------------------------------------

func TestAssessInvalidSignal(t *testing.T) {
	signals := []authenticity.Signal{
		{
			ID: "", RecordID: "rec-1", Method: authenticity.MethodHashMatch,
			Verdict: authenticity.VerdictAuthentic, Score: 0.9, At: fixedTime,
		},
	}
	_, err := authenticity.Assess(signals)
	if err == nil {
		t.Fatal("expected error for signal with empty ID")
	}
}

// ---------------------------------------------------------------------------
// Assess: determinism — same inputs produce same output
// ---------------------------------------------------------------------------

func TestAssessDeterminism(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.90),
		makeSignal("s2", "rec-1", authenticity.MethodC2PA, authenticity.VerdictLikelyAuthentic, 0.75),
		makeSignal("s3", "rec-1", authenticity.MethodAITextDetector, authenticity.VerdictInconclusive, 0.55),
	}
	a1, err1 := authenticity.Assess(signals)
	a2, err2 := authenticity.Assess(signals)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v / %v", err1, err2)
	}
	if a1.Overall != a2.Overall {
		t.Errorf("non-deterministic: a1.Overall=%q a2.Overall=%q", a1.Overall, a2.Overall)
	}
	if a1.Confidence != a2.Confidence {
		t.Errorf("non-deterministic: a1.Confidence=%g a2.Confidence=%g", a1.Confidence, a2.Confidence)
	}
	if len(a1.Signals) != len(a2.Signals) {
		t.Errorf("non-deterministic: len(a1.Signals)=%d len(a2.Signals)=%d", len(a1.Signals), len(a2.Signals))
	}
}

// ---------------------------------------------------------------------------
// Assess: returned Signals slice is a defensive copy
// ---------------------------------------------------------------------------

func TestAssessSignalsDefensiveCopy(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.9),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mutate the returned slice — must not affect a re-assessment.
	original := a.Signals[0].Detail
	a.Signals[0].Detail = "MUTATED"

	a2, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a2.Signals[0].Detail != original {
		t.Error("Assessment.Signals is not a defensive copy — mutation leaked back")
	}
}

// ---------------------------------------------------------------------------
// MaxEvidenceTier: verdict → tier mapping
// ---------------------------------------------------------------------------

func TestMaxEvidenceTier(t *testing.T) {
	cases := []struct {
		verdict  authenticity.Verdict
		wantTier evidencegrade.Tier
	}{
		{authenticity.VerdictAuthentic, evidencegrade.TierDirect},
		{authenticity.VerdictLikelyAuthentic, evidencegrade.TierDocumentary},
		{authenticity.VerdictInconclusive, evidencegrade.TierCircumstantial},
		{authenticity.VerdictLikelySynthetic, evidencegrade.TierTestimonial},
		{authenticity.VerdictSynthetic, evidencegrade.TierInferred},
		{authenticity.VerdictTampered, evidencegrade.TierInferred},
	}
	for _, tc := range cases {
		t.Run(string(tc.verdict), func(t *testing.T) {
			got := authenticity.MaxEvidenceTier(tc.verdict)
			if got != tc.wantTier {
				t.Errorf("MaxEvidenceTier(%q) = %q, want %q", tc.verdict, got, tc.wantTier)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MaxEvidenceTier: tampered and synthetic never exceed inferred
// ---------------------------------------------------------------------------

func TestMaxEvidenceTierAdverseCannotExceedInferred(t *testing.T) {
	adverse := []authenticity.Verdict{authenticity.VerdictSynthetic, authenticity.VerdictTampered}
	for _, v := range adverse {
		tier := authenticity.MaxEvidenceTier(v)
		if tier != evidencegrade.TierInferred {
			t.Errorf("MaxEvidenceTier(%q) = %q, want inferred (hard cap for adversarial verdicts)", v, tier)
		}
	}
}

// ---------------------------------------------------------------------------
// Assess: RecordID propagated correctly
// ---------------------------------------------------------------------------

func TestAssessRecordIDPropagated(t *testing.T) {
	signals := []authenticity.Signal{
		makeSignal("s1", "record-XYZ", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.9),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.RecordID != "record-XYZ" {
		t.Errorf("RecordID = %q, want record-XYZ", a.RecordID)
	}
}

// ---------------------------------------------------------------------------
// Assess: AI author signal is accepted by ValidateStrict (via strict path)
// ---------------------------------------------------------------------------

func TestAssessAIAuthoredSignal(t *testing.T) {
	sig := authenticity.Signal{
		ID:       "ai-sig-1",
		RecordID: "rec-ai",
		Method:   authenticity.MethodAITextDetector,
		Verdict:  authenticity.VerdictLikelySynthetic,
		Score:    0.82,
		Detail:   "GPT-4o text pattern detected",
		Author:   aiAuthor(),
		At:       fixedTime,
	}
	if err := sig.ValidateStrict(); err != nil {
		t.Fatalf("ValidateStrict rejected valid AI-authored signal: %v", err)
	}
	signals := []authenticity.Signal{sig}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictLikelySynthetic {
		t.Errorf("Overall = %q, want likely_synthetic", a.Overall)
	}
}

// ---------------------------------------------------------------------------
// DEFECT 1: all-Score==0 signals must produce inconclusive, not likely_authentic.
//
// The "Exact determination" section in assess.go states:
//   "Else (inconclusive-only or all Score == 0): Overall = inconclusive"
// But the current authSignals branch does not gate on Score > 0, so auth signals
// with Score==0 produce "likely_authentic" with Confidence==0 — a zero-confidence
// positive verdict that gives a false sense of authenticity.
// ---------------------------------------------------------------------------

func TestAssessAllScoreZeroYieldsInconclusive(t *testing.T) {
	// Single authentic signal, Score==0 — no confidence at all.
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.0),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictInconclusive {
		t.Errorf("Overall = %q, want inconclusive: a single Score=0 authentic signal has no evidential weight", a.Overall)
	}
	if a.Conservative {
		t.Error("Conservative should be false for an inconclusive-all-zero result")
	}
}

func TestAssessAllScoreZeroMixedVerdictsYieldsInconclusive(t *testing.T) {
	// Mix of auth and adverse signals, ALL with Score==0.
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.0),
		makeSignal("s2", "rec-1", authenticity.MethodEXIFForensics, authenticity.VerdictTampered, 0.0),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Overall != authenticity.VerdictInconclusive {
		t.Errorf("Overall = %q, want inconclusive: all-zero Score mix should not resolve to authentic or tampered", a.Overall)
	}
}

// ---------------------------------------------------------------------------
// DEFECT 2: dead allAdverse variable — zero-score adverse with auth signals
// should not silently inflate Conservative to false without comment.
// This test confirms the surfacing behaviour: zero-score adverse signals
// appear in Assessment.Signals even when they do not dominate.
// ---------------------------------------------------------------------------

func TestAssessZeroScoreAdverseSignalSurfacedInSignals(t *testing.T) {
	// A tampered signal with Score==0 must NOT dominate (Conservative=false, no adverse Overall)
	// but it must still appear in Assessment.Signals (no silent truncation).
	signals := []authenticity.Signal{
		makeSignal("s1", "rec-1", authenticity.MethodHashMatch, authenticity.VerdictAuthentic, 0.80),
		makeSignal("s2", "rec-1", authenticity.MethodEXIFForensics, authenticity.VerdictTampered, 0.0),
	}
	a, err := authenticity.Assess(signals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Adverse does NOT dominate when Score==0.
	if a.Overall == authenticity.VerdictTampered {
		t.Error("a zero-Score tampered signal should not dominate the verdict")
	}
	if a.Conservative {
		t.Error("Conservative should be false when the only adverse signal has Score==0")
	}
	// But the signal must still appear in the defensive copy.
	found := false
	for _, s := range a.Signals {
		if s.Verdict == authenticity.VerdictTampered && s.Score == 0.0 {
			found = true
		}
	}
	if !found {
		t.Error("zero-Score tampered signal must appear in Assessment.Signals (no silent truncation)")
	}
}

// ---------------------------------------------------------------------------
// TEST GAP: ValidateStrict must reject a non-zero but semantically invalid Author
// (e.g. Kind=KindAI but AI pointer is nil). The existing strict test only covers
// the zero-value Author; this covers the "set Kind but wrong pointer" case.
// ---------------------------------------------------------------------------

func TestSignalValidateStrict_InvalidNonZeroAuthor(t *testing.T) {
	// KindAI but AI pointer is nil — valid Kind discriminator, invalid shape.
	badAuthor := aiprovenance.Author{
		Kind: aiprovenance.KindAI,
		AI:   nil, // missing required pointer
	}
	sig := authenticity.Signal{
		ID:       "s-strict-bad",
		RecordID: "rec-1",
		Method:   authenticity.MethodAITextDetector,
		Verdict:  authenticity.VerdictLikelySynthetic,
		Score:    0.85,
		Author:   badAuthor,
		At:       fixedTime,
	}
	err := sig.ValidateStrict()
	if err == nil {
		t.Fatal("ValidateStrict must return error for KindAI Author with nil AI pointer")
	}
	if !strings.Contains(err.Error(), "Author") {
		t.Errorf("error %q should mention Author", err.Error())
	}
}

func TestSignalValidateStrict_InvalidHumanAuthorEmptyID(t *testing.T) {
	// KindHuman but empty ID — valid Kind, invalid Human.ID.
	badAuthor := aiprovenance.Author{
		Kind:  aiprovenance.KindHuman,
		Human: &aiprovenance.HumanAuthor{ID: ""}, // empty ID is invalid
	}
	sig := authenticity.Signal{
		ID:       "s-strict-human-empty",
		RecordID: "rec-1",
		Method:   authenticity.MethodManualReview,
		Verdict:  authenticity.VerdictAuthentic,
		Score:    0.9,
		Author:   badAuthor,
		At:       fixedTime,
	}
	err := sig.ValidateStrict()
	if err == nil {
		t.Fatal("ValidateStrict must return error for KindHuman Author with empty Human.ID")
	}
	if !strings.Contains(err.Error(), "Author") {
		t.Errorf("error %q should mention Author", err.Error())
	}
}
