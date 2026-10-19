package decide_test

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestDecidedByNamesSiteProviderModelAndCalibration is JG-1 (ADR-073
// decision 6, DMP-3): one renderer turns a decision row into the decided_by
// badge every served surface carries. A judged answer names its site, its
// provider and resolved model, its confidence and the site's calibration
// (uncalibrated 0/100 when no reading is supplied); computed, table and
// override read as themselves, an override with the table's answer beside
// it, and a computed answer the port missed names `not consulted (<class>)`.
// For provider none the badge is absent (I42).
func TestDecidedByNamesSiteProviderModelAndCalibration(t *testing.T) {
	row := decision.Record{Site: "route", Provider: "typesafe", Model: "jev-latest", QuestionID: "verb", Kind: decision.KindChoice, Choice: "localize", Confidence: 0.914}
	for _, tc := range []struct {
		name string
		d    decide.Decision
		want string
	}{
		{"judged, no reading", decide.Decision{Provider: "typesafe", Tier: decide.DecidedJudged, Record: row},
			"judged(route · typesafe/jev-latest · conf 0.91 · calibration uncalibrated 0/100)"},
		{"judged, a reading", decide.Decision{Provider: "typesafe", Tier: decide.DecidedJudged, Record: row, Calibration: decide.Calibration{Status: string(decision.CalibrationInsufficientN), N: 42}},
			"judged(route · typesafe/jev-latest · conf 0.91 · calibration insufficient-n 42/100)"},
		{"judged, the row names no provider", decide.Decision{Provider: "llm", Tier: decide.DecidedJudged, Record: decision.Record{Site: "obligations", Model: "m", Confidence: 0.5}},
			"judged(obligations · llm/m · conf 0.50 · calibration uncalibrated 0/100)"},
		{"computed", decide.Decision{Provider: "typesafe", Tier: decide.DecidedComputed}, "computed"},
		{"computed, a miss", decide.Decision{Provider: "typesafe", Tier: decide.DecidedComputed, Miss: "timeout"}, "computed · not consulted (timeout)"},
		{"table", decide.Decision{Provider: "typesafe", Tier: decide.DecidedTable}, "table"},
		{"override", decide.Decision{Provider: "typesafe", Tier: decide.DecidedOverride}, "override"},
		{"override beside the table", decide.Decision{Provider: "typesafe", Tier: decide.DecidedOverride, Table: "lean"}, "override · table lean"},
		{"provider none", decide.Decision{Provider: "none", Tier: decide.DecidedJudged, Record: row}, ""},
		{"no provider", decide.Decision{Tier: decide.DecidedComputed}, ""},
		{"no tier", decide.Decision{Provider: "typesafe"}, ""},
	} {
		if got := decide.DecidedBy(tc.d); got != tc.want {
			t.Errorf("%s: DecidedBy = %q, want %q", tc.name, got, tc.want)
		}
	}

	// The weakest row is the one a multi-answer surface (the obligations
	// re-rank) names: the lowest confidence, ties by question id.
	recs := []decision.Record{
		{Site: "obligations", Provider: "mock", QuestionID: "b", Confidence: 0.7},
		{Site: "obligations", Provider: "mock", QuestionID: "a", Confidence: 0.6},
		{Site: "obligations", Provider: "mock", QuestionID: "c", Confidence: 0.6},
	}
	if w, ok := decide.WeakestRecord(recs); !ok || w.QuestionID != "a" {
		t.Errorf("WeakestRecord = %+v, %v, want question a", w, ok)
	}
	if _, ok := decide.WeakestRecord(nil); ok {
		t.Error("WeakestRecord(nil) ok, want false")
	}

	// A miss's class comes from the port's own error: a degrade names why,
	// a live kill is not a miss (the answer carries no badge under a kill).
	if got := decide.MissClass(&decide.DegradedError{Provider: "typesafe", Err: context.DeadlineExceeded}); got != "timeout" {
		t.Errorf("MissClass(degraded timeout) = %q, want timeout", got)
	}
	if got := decide.MissClass(&decide.KilledError{Gate: decide.Gate{Closed: true, Source: decide.KillSourceFile}}); got != "" {
		t.Errorf("MissClass(killed) = %q, want empty", got)
	}
	if got := decide.MissClass(nil); got != "" {
		t.Errorf("MissClass(nil) = %q, want empty", got)
	}
	if got := decide.MissClass(errors.New("boom")); got != "error" {
		t.Errorf("MissClass(plain) = %q, want error", got)
	}
	if !decide.IsKilled(&decide.KilledError{}) || decide.IsKilled(errors.New("x")) {
		t.Error("IsKilled reads a *KilledError and nothing else")
	}
}
