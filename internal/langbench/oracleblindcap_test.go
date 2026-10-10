package langbench

import (
	"strings"
	"testing"
)

// TestOracleBlindCap (LB-F203): when calls_oracle_blind exceeds 10% of the
// subject's fathom pairs (found + blind; per subject since owner ruling W3) the
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

// TestOracleBlindCapPerSubject (owner ruling W3, 2026-10-10, O482; LB-F203):
// the cap applies per subject. A subject whose own blind pairs exceed 10% of its
// own fathom pairs (found + blind) has them counted back as fathom-only and is
// named in the cap's note, however large the reading's other subjects are; a
// subject under the cap keeps the exclusion; recall never moves; a stale
// summed-cap "oracle-blind share" note is dropped on regrade.
func TestOracleBlindCapPerSubject(t *testing.T) {
	r := NewReading(Manifest{ID: "javascript"}, "L2")
	r.Notes = []string{noteOracleBlindSharePrefix + "untyped 40 of 100 (40.0%): stale, from the summed cap"}
	fam := map[string]string{FamilyCalls: "A"}
	MergeSubject(&r, Subject{Name: "untyped", Files: 1, Bytes: 1 << 20, Families: fam,
		Counts: map[string]int{CntCallFathom: 60, CntCallFathomInTrue: 40, CntCallsOracleBlind: 40, CntCallTruth: 50, CntCallTruthFound: 40}})
	MergeSubject(&r, Subject{Name: "typed", Files: 1, Bytes: 1 << 20, Families: fam,
		Counts: map[string]int{CntCallFathom: 2000, CntCallFathomInTrue: 1900, CntCallsOracleBlind: 20, CntCallTruth: 2000, CntCallTruthFound: 1900}})
	if got := *r.Metrics[MCallPrecision]; got != *ratio(1940, 2100) {
		t.Errorf("call_precision = %v, want 1940/2100: untyped's 40 blind pairs back in, typed's 20 still out", got)
	}
	if got := *r.Metrics[MCallRecall]; got != *ratio(1940, 2050) {
		t.Errorf("call_recall moved: %v", got)
	}
	if r.Counts[CntCallsOracleBlind] != 60 {
		t.Errorf("calls_oracle_blind = %d, still counted, want 60", r.Counts[CntCallsOracleBlind])
	}
	fired := 0
	for _, n := range r.Notes {
		if strings.HasPrefix(n, noteOracleBlindSharePrefix) {
			t.Errorf("summed-cap share note survives: %s", n)
		}
		if strings.HasPrefix(n, noteOracleBlindCapPrefix) {
			fired++
			if !strings.Contains(n, "untyped 40 of 100 (40.0%)") || strings.Contains(n, "typed 20") || !strings.Contains(n, "W3") {
				t.Errorf("cap note %q: want untyped named with its own share, typed not, ruling W3 cited", n)
			}
		}
	}
	if fired != 1 {
		t.Errorf("cap notes = %d, want 1 (%v)", fired, r.Notes)
	}
	Regrade(&r)
	if got := *r.Metrics[MCallPrecision]; got != *ratio(1940, 2100) {
		t.Errorf("regraded call_precision = %v, want 1940/2100", got)
	}
}

// TestFileEdgesOracleBlindCapPerSubject (W3 under LB-F296): the file-edge cap is
// LB-F203's and weighs subjects the same way: a subject over 10% of its own
// file pairs has its blind pairs back in file_edge_precision, the others keep
// the exclusion.
func TestFileEdgesOracleBlindCapPerSubject(t *testing.T) {
	r := NewReading(Manifest{ID: "cpp"}, "L3")
	fam := map[string]string{FamilyFileEdges: "A"}
	MergeSubject(&r, Subject{Name: "small", Files: 1, Bytes: 1 << 20, Families: fam,
		Counts: map[string]int{CntFileEdgesTruth: 500, CntFileEdgesFathom: 249, CntFileEdgesAgree: 241, CntFileEdgesOracleBlind: 36}})
	MergeSubject(&r, Subject{Name: "large", Files: 1, Bytes: 1 << 20, Families: fam,
		Counts: map[string]int{CntFileEdgesTruth: 500, CntFileEdgesFathom: 2000, CntFileEdgesAgree: 1990, CntFileEdgesOracleBlind: 10}})
	if got, want := r.Metrics[MFileEdgePrecision], ratio(2231, 2285); got == nil || *got != *want {
		t.Errorf("file_edge_precision = %v, want %v (small's 36 back in, large's 10 out)", got, *want)
	}
	n := 0
	for _, x := range r.Notes {
		if strings.HasPrefix(x, noteFileEdgesBlindCapPrefix) {
			n++
			if !strings.Contains(x, "small 36 of 285 (12.6%)") || strings.Contains(x, "large") {
				t.Errorf("file-edge cap note %q: want small named with its own share, large not", x)
			}
		}
	}
	if n != 1 {
		t.Errorf("file-edge cap notes = %d, want 1 (%v)", n, r.Notes)
	}
}
