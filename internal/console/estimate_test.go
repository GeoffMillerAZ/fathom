package console

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestEstimateRowWeightFormula(t *testing.T) {
	in := RowWeightInput{ContractKind: ContractReach, Files: []string{"a.go", "b.go", "a.go", " "}, Needs: 2,
		Profile: Profile{Blast: Present(BlastRadius{Callers: 11}, "index"), Similar: Present(SimilarEffort{MedianTokens: 250, FleetMedianTokens: 100}, "history")}}

	if got := EstimateRowWeight(in).EstimatedWeight; got != 13 {
		t.Fatalf("estimate = %d, want 13 (baseline 3 * effort ratio 2.5 + files 2 + needs 2 + blast 2, snapped)", got)
	}
}

func TestEstimateRowWeightNeverOverwritesAuthored(t *testing.T) {
	in := RowWeightInput{ContractKind: ContractReach, Files: []string{"a.go"}, AuthoredWeight: 13}
	before := in
	got := EstimateRowWeight(in)
	if got.AuthoredWeight != 13 || got.EstimatedWeight != 5 || !reflect.DeepEqual(in, before) {
		t.Fatalf("estimate = %+v, input = %+v", got, in)
	}
	in.AuthoredWeight = 1
	if EstimateRowWeight(in).EstimatedWeight != got.EstimatedWeight {
		t.Fatal("authored weight affected estimate")
	}
}

func TestEstimateRowWeightNoIndexBaseline(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want int
	}{{ContractReach, 5}, {ContractSymbol, 5}, {ContractCertified, 5}, {ContractGone, 5}, {ContractLexical, 3}, {ContractLedger, 3}, {ContractDisposition, 3}} {
		in := RowWeightInput{ContractKind: tc.kind, Files: []string{"a.go", "b.go"}}
		got := EstimateRowWeight(in)
		if got.EstimatedWeight != tc.want {
			t.Errorf("%s: %+v, want baseline %d", tc.kind, got, tc.want)
		}
		data, _ := json.Marshal(got)
		if !containsEstimateLabel(data) {
			t.Errorf("missing no-index label: %s", data)
		}
	}
}
func containsEstimateLabel(data []byte) bool {
	var v map[string]any
	_ = json.Unmarshal(data, &v)
	return v["label"] == "estimate (no index evidence)"
}

func TestEstimateRowWeightDeterministic(t *testing.T) {
	in := RowWeightInput{ContractKind: ContractReach, Files: []string{"b.go", "a.go", "a.go"}, Profile: Profile{Blast: Measured(BlastRadius{}, "index")}}
	want := EstimateRowWeight(in)
	for range 20 {
		if got := EstimateRowWeight(in); got != want {
			t.Fatalf("estimate changed: %+v != %+v", got, want)
		}
	}
	reversed := in
	reversed.Files = []string{"a.go", "b.go"}
	if got := EstimateRowWeight(reversed); got != want {
		t.Fatalf("ordering changed estimate: %+v != %+v", got, want)
	}
	if in.Files[0] != "b.go" {
		t.Fatal("estimator mutated input slices")
	}
}

func TestEstimateRowWeightEvidenceChangesEstimate(t *testing.T) {
	base := RowWeightInput{ContractKind: ContractReach, Profile: Profile{Blast: Measured(BlastRadius{}, "index")}}
	want := EstimateRowWeight(base)
	if want.EstimatedWeight != 3 || want.Label != "estimate (no similar-row effort)" {
		t.Fatalf("measured zero: %+v", want)
	}
	for _, tc := range []struct {
		name   string
		change func(*RowWeightInput)
		want   int
	}{
		{"files", func(in *RowWeightInput) { in.Files = []string{"a.go", "a.go", "b.go", " "} }, 5},
		{"needs", func(in *RowWeightInput) { in.Needs = 2 }, 5},
		{"blast", func(in *RowWeightInput) { in.Profile.Blast = Present(BlastRadius{Callers: 20}, "index") }, 5},
		{"effort", func(in *RowWeightInput) {
			in.Profile.Similar = Present(SimilarEffort{MedianTokens: 200, FleetMedianTokens: 100}, "history")
		}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.change(&in)
			if got := EstimateRowWeight(in); got.EstimatedWeight != tc.want {
				t.Fatalf("got %+v, want %d", got, tc.want)
			}
		})
	}
}

func TestEstimateRowWeightNoIndexKeepsPlanFacts(t *testing.T) {
	in := RowWeightInput{ContractKind: ContractSymbol, Files: []string{"a.go", " a.go ", "b.go", " "}, Needs: 1}
	got := EstimateRowWeight(in)
	if got.EstimatedWeight != 5 || got.Label != "estimate (no index evidence)" {
		t.Fatalf("no-index plan facts: %+v, want baseline 2 + files 2 + needs 1", got)
	}
	in.Needs = 1000
	if got := EstimateRowWeight(in); got.EstimatedWeight != 13 {
		t.Fatalf("no-index scope exceeded the cap: %+v", got)
	}
}

func TestEstimateRowWeightRelativeScale(t *testing.T) {
	for _, tc := range []struct {
		ratio float64
		want  int
	}{
		{0.1, 1}, {1, 1}, {1.49, 1}, {1.5, 2}, {2.49, 2}, {2.5, 3},
		{3.99, 3}, {4, 5}, {6.49, 5}, {6.5, 8}, {10.49, 8}, {10.5, 13}, {13, 13}, {100, 13},
	} {
		in := RowWeightInput{ContractKind: ContractLexical, Profile: Profile{
			Blast:   Measured(BlastRadius{}, "index"),
			Similar: Present(SimilarEffort{MedianTokens: 100 * tc.ratio, FleetMedianTokens: 100, MedianTurns: 1000}, "history"),
		}}
		if got := EstimateRowWeight(in); got.EstimatedWeight != tc.want {
			t.Errorf("ratio %g: %+v, want %d", tc.ratio, got, tc.want)
		}
		in.Profile.Similar.V.MedianTokens *= 100
		in.Profile.Similar.V.FleetMedianTokens *= 100
		if got := EstimateRowWeight(in); got.EstimatedWeight != tc.want {
			t.Errorf("scaled effort, ratio %g: %+v, want %d", tc.ratio, got, tc.want)
		}
	}
	for _, n := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		in := RowWeightInput{ContractKind: ContractReach, Profile: Profile{
			Blast:   Measured(BlastRadius{}, "index"),
			Similar: Present(SimilarEffort{MedianTokens: 100, FleetMedianTokens: n}, "history"),
		}}
		if got := EstimateRowWeight(in); got.EstimatedWeight != 3 {
			t.Errorf("invalid service median %g: %+v", n, got)
		}
		in.Profile.Similar.V.MedianTokens, in.Profile.Similar.V.FleetMedianTokens = n, 100
		if got := EstimateRowWeight(in); got.EstimatedWeight != 3 {
			t.Errorf("invalid similar median %g: %+v", n, got)
		}
	}
}

func TestDifficultyEstimateSimilarityExcludesAuthoredWeight(t *testing.T) {
	in := preflightInput()
	in.ExcludeWeight = true
	in.Row.Weight = 1
	first := DifficultyProfile(in)
	in.Row.Weight = 13
	second := DifficultyProfile(in)
	if !reflect.DeepEqual(first.Similar, second.Similar) || second.Row.Weight != 13 {
		t.Fatalf("authored weight changed estimate evidence or was hidden: %+v / %+v", first, second)
	}
	if !strings.Contains(second.Similar.V.Basis, "excluding plan weight") {
		t.Fatalf("similarity basis claims to include weight: %s", second.Similar.V.Basis)
	}
}

func TestRowIndexSimilarityExcludesWeight(t *testing.T) {
	ix := rowFeatureIndex(preflightHistory())
	q := RowQuery{Zones: []string{"internal/console"}, Weight: 1, ExcludeWeight: true}
	first := ix.Similar(q, PreflightK)
	q.Weight = 13
	if second := ix.Similar(q, PreflightK); !reflect.DeepEqual(first, second) {
		t.Fatalf("excluded weight changed neighbours: %+v / %+v", first, second)
	}
	q.ExcludeWeight = false
	if weighted := ix.Similar(q, PreflightK); reflect.DeepEqual(first, weighted) {
		t.Fatal("ordinary difficulty queries lost their weight dimension")
	}
}
