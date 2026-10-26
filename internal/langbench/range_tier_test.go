package langbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRangeCompanionTierCapsOnlyRangeAgreement(t *testing.T) {
	for _, tc := range []struct{ l1, ranges, want string }{
		{"A", "C", "C"}, {"B", "A", "B"}, {"A", "D", "D"},
	} {
		m := Manifest{ID: "x", Kind: "language", RequiredKinds: []string{"function"}, Kinds: map[string]KindSpec{"function": {Canonical: "function"}}}
		r := NewReading(m, "L1")
		MergeSubject(&r, Subject{Name: "s", Counts: map[string]int{CntOracleDefs: 1, CntFathomDefs: 1, CntMatched: 1, CntKindEqual: 1, CntRangeEligible: 2, CntRangeOK: 1}, Families: map[string]string{FamilyL1: tc.l1, FamilyRanges: tc.ranges}})
		if got := r.MetricTiers[MRangeAgreement]; got != tc.want {
			t.Errorf("L1 %s ranges %s: range tier %s, want %s", tc.l1, tc.ranges, got, tc.want)
		}
		for _, metric := range []string{MSymRecall, MKindAgreement} {
			if got := r.MetricTiers[metric]; got != tc.l1 {
				t.Errorf("%s tier %s, want L1 tier %s", metric, got, tc.l1)
			}
		}
		b, err := MarshalReading(r)
		if err != nil {
			t.Fatal(err)
		}
		var persisted Reading
		if err := json.Unmarshal(b, &persisted); err != nil {
			t.Fatal(err)
		}
		Regrade(&persisted)
		if got := persisted.MetricTiers[MRangeAgreement]; got != tc.want {
			t.Errorf("regraded range tier %s, want %s", got, tc.want)
		}
	}
}

func TestRangeTierLegacyFallbackAndWeakestContributor(t *testing.T) {
	m := Manifest{ID: "x", Kind: "language", RequiredKinds: []string{"function"}, Kinds: map[string]KindSpec{"function": {Canonical: "function"}}}
	r := NewReading(m, "L1")
	base := map[string]int{CntOracleDefs: 1, CntFathomDefs: 1, CntMatched: 1, CntKindEqual: 1, CntRangeEligible: 2, CntRangeOK: 2}
	MergeSubject(&r, Subject{Name: "legacy", Counts: base, Families: map[string]string{FamilyL1: "B"}})
	if got := r.MetricTiers[MRangeAgreement]; got != "B" {
		t.Fatalf("legacy range tier %s, want B", got)
	}
	MergeSubject(&r, Subject{Name: "a", Counts: map[string]int{CntRangeEligible: 1}, Families: map[string]string{FamilyRanges: "A"}})
	MergeSubject(&r, Subject{Name: "c", Counts: map[string]int{CntRangeEligible: 1}, Families: map[string]string{FamilyRanges: "C"}})
	if got := r.MetricTiers[MRangeAgreement]; got != "C" {
		t.Fatalf("combined range tier %s, want weakest contributor C", got)
	}
}

// Tooling intentionally has no range metric. Regrading the committed corpus
// must not acquire one merely because a subject recorded range counts.
func TestCommittedToolingReadingsKeepMetricsOnRegrade(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "bench", "readings", "lang", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join("..", "..", "docs", "bench", "readings", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, more...)
	if len(paths) == 0 {
		t.Fatal("no committed readings")
	}
	withRanges := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var r Reading
		if err := json.Unmarshal(raw, &r); err != nil || r.Kind != "tooling" {
			continue // Other top-level JSON schemas are not harness readings.
		}
		if r.Counts[CntRangeEligible] > 0 {
			withRanges++
		}
		beforeMetrics, beforeTiers := r.Metrics, r.MetricTiers
		beforeGrade, beforeLevel := r.Grade, r.LevelMet
		Regrade(&r)
		if !reflect.DeepEqual(r.Metrics, beforeMetrics) || !reflect.DeepEqual(r.MetricTiers, beforeTiers) ||
			r.Grade != beforeGrade || r.LevelMet != beforeLevel {
			t.Errorf("%s: tooling grade or metrics changed on regrade", path)
		}
		if _, present := r.Metrics[MRangeAgreement]; present {
			t.Errorf("%s: tooling acquired range_agreement", path)
		}
	}
	if withRanges != 41 {
		t.Errorf("tooling readings with range counts = %d, want 41", withRanges)
	}
}
