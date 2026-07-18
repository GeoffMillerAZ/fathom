package authenticity_test

import (
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/evidence/authenticity"
)

// ---------------------------------------------------------------------------
// Method ceilings (D35-4)
// ---------------------------------------------------------------------------

// TestScoreCeiling pins the RFC-protocol ceilings: 0.95 for dkim, 0.55 for
// auth_results_header, 1.0 for every other method. These exact values are a
// contract (throughline friction addendum item 4) — changing them is a
// breaking change, not a tweak.
func TestScoreCeiling(t *testing.T) {
	tests := []struct {
		method authenticity.Method
		want   float64
	}{
		{authenticity.MethodDKIM, 0.95},
		{authenticity.MethodAuthResultsHeader, 0.55},
		{authenticity.MethodHashMatch, 1.0},
		{authenticity.MethodC2PA, 1.0},
		{authenticity.MethodEXIFForensics, 1.0},
		{authenticity.MethodAITextDetector, 1.0},
		{authenticity.MethodDeepfakeDetector, 1.0},
		{authenticity.MethodMetadataForensics, 1.0},
		{authenticity.MethodManualReview, 1.0},
	}
	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			if got := tt.method.ScoreCeiling(); got != tt.want {
				t.Errorf("ScoreCeiling(%q) = %g, want %g", tt.method, got, tt.want)
			}
		})
	}
}

func TestVerdictCeiling(t *testing.T) {
	tests := []struct {
		method authenticity.Method
		want   authenticity.Verdict
	}{
		{authenticity.MethodAuthResultsHeader, authenticity.VerdictLikelyAuthentic},
		{authenticity.MethodDKIM, authenticity.VerdictAuthentic},
		{authenticity.MethodHashMatch, authenticity.VerdictAuthentic},
		{authenticity.MethodC2PA, authenticity.VerdictAuthentic},
		{authenticity.MethodEXIFForensics, authenticity.VerdictAuthentic},
		{authenticity.MethodAITextDetector, authenticity.VerdictAuthentic},
		{authenticity.MethodDeepfakeDetector, authenticity.VerdictAuthentic},
		{authenticity.MethodMetadataForensics, authenticity.VerdictAuthentic},
		{authenticity.MethodManualReview, authenticity.VerdictAuthentic},
	}
	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			if got := tt.method.VerdictCeiling(); got != tt.want {
				t.Errorf("VerdictCeiling(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ClampToCeiling
// ---------------------------------------------------------------------------

func TestClampToCeiling(t *testing.T) {
	tests := []struct {
		name        string
		method      authenticity.Method
		verdict     authenticity.Verdict
		score       float64
		wantVerdict authenticity.Verdict
		wantScore   float64
	}{
		{
			name:        "auth_results_header score above ceiling clamped to 0.55",
			method:      authenticity.MethodAuthResultsHeader,
			verdict:     authenticity.VerdictLikelyAuthentic,
			score:       0.9,
			wantVerdict: authenticity.VerdictLikelyAuthentic,
			wantScore:   0.55,
		},
		{
			name:        "auth_results_header authentic demoted to likely_authentic",
			method:      authenticity.MethodAuthResultsHeader,
			verdict:     authenticity.VerdictAuthentic,
			score:       0.5,
			wantVerdict: authenticity.VerdictLikelyAuthentic,
			wantScore:   0.5,
		},
		{
			name:        "auth_results_header adverse verdict never lifted",
			method:      authenticity.MethodAuthResultsHeader,
			verdict:     authenticity.VerdictTampered,
			score:       0.9,
			wantVerdict: authenticity.VerdictTampered,
			wantScore:   0.55,
		},
		{
			name:        "auth_results_header inconclusive untouched",
			method:      authenticity.MethodAuthResultsHeader,
			verdict:     authenticity.VerdictInconclusive,
			score:       0.3,
			wantVerdict: authenticity.VerdictInconclusive,
			wantScore:   0.3,
		},
		{
			name:        "dkim score above ceiling clamped to 0.95",
			method:      authenticity.MethodDKIM,
			verdict:     authenticity.VerdictAuthentic,
			score:       1.0,
			wantVerdict: authenticity.VerdictAuthentic,
			wantScore:   0.95,
		},
		{
			name:        "dkim verdict uncapped",
			method:      authenticity.MethodDKIM,
			verdict:     authenticity.VerdictAuthentic,
			score:       0.9,
			wantVerdict: authenticity.VerdictAuthentic,
			wantScore:   0.9,
		},
		{
			name:        "score at ceiling not reduced",
			method:      authenticity.MethodAuthResultsHeader,
			verdict:     authenticity.VerdictLikelyAuthentic,
			score:       0.55,
			wantVerdict: authenticity.VerdictLikelyAuthentic,
			wantScore:   0.55,
		},
		{
			name:        "uncapped method fully untouched",
			method:      authenticity.MethodHashMatch,
			verdict:     authenticity.VerdictAuthentic,
			score:       1.0,
			wantVerdict: authenticity.VerdictAuthentic,
			wantScore:   1.0,
		},
		{
			name:        "unknown verdict not promoted to ceiling",
			method:      authenticity.MethodDKIM,
			verdict:     authenticity.Verdict("bogus"),
			score:       0.2,
			wantVerdict: authenticity.Verdict("bogus"),
			wantScore:   0.2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := makeSignal("sig-1", "rec-1", tt.method, tt.verdict, tt.score)
			got := authenticity.ClampToCeiling(in)
			if got.Verdict != tt.wantVerdict {
				t.Errorf("Verdict = %q, want %q", got.Verdict, tt.wantVerdict)
			}
			if got.Score != tt.wantScore {
				t.Errorf("Score = %g, want %g", got.Score, tt.wantScore)
			}
			// Untouched fields pass through.
			if got.ID != in.ID || got.RecordID != in.RecordID || got.Method != in.Method {
				t.Errorf("ClampToCeiling mutated identity fields: got %+v", got)
			}
		})
	}
}

// TestClampToCeilingDoesNotMutateInput verifies value semantics: the caller's
// Signal is unchanged.
func TestClampToCeilingDoesNotMutateInput(t *testing.T) {
	in := makeSignal("sig-1", "rec-1", authenticity.MethodAuthResultsHeader, authenticity.VerdictAuthentic, 0.9)
	_ = authenticity.ClampToCeiling(in)
	if in.Score != 0.9 || in.Verdict != authenticity.VerdictAuthentic {
		t.Errorf("input mutated: %+v", in)
	}
}

// TestAssessDoesNotClamp is the regression guard for the opt-in contract:
// Assess must NOT apply the method ceilings. An auth_results_header signal
// claiming authentic at 0.9 flows through Assess unclamped.
func TestAssessDoesNotClamp(t *testing.T) {
	sig := makeSignal("sig-1", "rec-1", authenticity.MethodAuthResultsHeader, authenticity.VerdictAuthentic, 0.9)
	a, err := authenticity.Assess([]authenticity.Signal{sig})
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if a.Overall != authenticity.VerdictAuthentic {
		t.Errorf("Overall = %q, want %q (Assess must not apply VerdictCeiling)", a.Overall, authenticity.VerdictAuthentic)
	}
	if a.Confidence != 0.9 {
		t.Errorf("Confidence = %g, want 0.9 (Assess must not apply ScoreCeiling)", a.Confidence)
	}
	if len(a.Signals) != 1 || a.Signals[0].Score != 0.9 || a.Signals[0].Verdict != authenticity.VerdictAuthentic {
		t.Errorf("stored signal was clamped: %+v", a.Signals)
	}
}
