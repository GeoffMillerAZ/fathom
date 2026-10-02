package langbench

import (
	"strings"
	"testing"
)

// TestOracleBlindCap (LB-F203): when calls_oracle_blind exceeds 10% of the
// reading's fathom pairs (found + blind, summed over subjects) the
// exclusion is off: blind pairs count back as fathom-only and a note says so.
func TestOracleBlindCap(t *testing.T) {
	// found pairs in truth / found / blind / truth / truth found
	for _, tc := range []struct {
		name         string
		found, blind int
		fires        bool
	}{
		{"typescript 429 of 11327 keeps the exclusion", 11327 - 429, 429, false},
		{"javascript 422 of 1441 falls back", 1441 - 422, 422, true},
		{"exactly ten percent keeps", 900, 100, false},
		{"just over ten percent falls back", 899, 101, true},
		{"none blind", 500, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReading(Manifest{ID: "javascript"}, "L2")
			inTrue := tc.found / 2
			MergeSubject(&r, Subject{Name: "s", Files: 1, Bytes: 1 << 20,
				Counts: map[string]int{
					CntCallTruth: 100, CntCallTruthFound: 60,
					CntCallFathom: tc.found, CntCallFathomInTrue: inTrue,
					CntCallsOracleBlind: tc.blind,
				},
				Families: map[string]string{FamilyCalls: "A"}})
			wantDen := tc.found
			if tc.fires {
				wantDen = tc.found + tc.blind
			}
			got := r.Metrics[MCallPrecision]
			want := ratio(inTrue, wantDen)
			if got == nil || *got != *want {
				t.Errorf("call_precision = %v, want %v", got, *want)
			}
			if r.Counts[CntCallsOracleBlind] != tc.blind {
				t.Errorf("calls_oracle_blind = %d, still counted, want %d", r.Counts[CntCallsOracleBlind], tc.blind)
			}
			if rec := r.Metrics[MCallRecall]; rec == nil || *rec != 0.6 {
				t.Errorf("call_recall moved: %v", rec)
			}
			has := false
			for _, n := range r.Notes {
				has = has || strings.HasPrefix(n, noteOracleBlindCapPrefix)
			}
			if has != tc.fires {
				t.Errorf("cap note present = %v, want %v", has, tc.fires)
			}
		})
	}
}

// Regrading a fired reading twice leaves one note.
func TestOracleBlindCapIdempotent(t *testing.T) {
	r := NewReading(Manifest{ID: "javascript"}, "L2")
	MergeSubject(&r, Subject{Name: "s", Files: 1, Bytes: 1 << 20,
		Counts:   map[string]int{CntCallFathom: 10, CntCallFathomInTrue: 5, CntCallsOracleBlind: 10, CntCallTruth: 10, CntCallTruthFound: 5},
		Families: map[string]string{FamilyCalls: "A"}})
	Regrade(&r)
	n := 0
	for _, x := range r.Notes {
		if strings.HasPrefix(x, noteOracleBlindCapPrefix) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("cap notes = %d, want 1", n)
	}
}
