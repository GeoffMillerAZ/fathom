package reliabilitystats_test

import (
	"errors"
	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
	"math"
	"reflect"
	"testing"
)

func TestWildClusterBootstrapCI_SeedAndCoverage(t *testing.T) {
	obs := []rs.Observation{{Task: "a", Diff: 0}, {Task: "a", Diff: 2}, {Task: "b", Diff: 3}}
	original := append([]rs.Observation(nil), obs...)
	got, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil {
		t.Fatal(err)
	}
	again, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil || got != again {
		t.Fatalf("same seed: %+v vs %+v (%v)", got, again, err)
	}
	// Cluster means are 1 and 3, mean 2. With G/(G-1) correction,
	// Webb weights +/-sqrt(3/2) put endpoints at 2 +/- sqrt(3).
	// Each extreme has mass 1/36, above either 2.5% tail.
	if got.Point != 2 || got.Tasks != 2 || !near(got.Low, 2-math.Sqrt(3), 1e-12) || !near(got.High, 2+math.Sqrt(3), 1e-12) {
		t.Errorf("Webb interval: %+v", got)
	}
	if !(got.Low < 2 && got.High > 2) {
		t.Errorf("known mean not covered: %+v", got)
	}
	if !reflect.DeepEqual(obs, original) {
		t.Fatal("mutated input")
	}
	reversed := []rs.Observation{obs[2], obs[1], obs[0]}
	shuffled, err := rs.WildClusterBootstrapCI(reversed, 42, 20000, .95)
	if err != nil || shuffled != got {
		t.Errorf("input order changed result: %+v (%v)", shuffled, err)
	}
}

func TestWildClusterBootstrapCI_Constant(t *testing.T) {
	got, err := rs.WildClusterBootstrapCI([]rs.Observation{{Task: "a", Diff: 3}, {Task: "b", Diff: 3}}, 2, 100, .9)
	if err != nil || got.Low != 3 || got.High != 3 || got.Point != 3 {
		t.Errorf("constant: %+v (%v)", got, err)
	}
}

func TestWildClusterBootstrapCI_SeedDraw(t *testing.T) {
	obs := []rs.Observation{{Task: "a", Diff: 0}, {Task: "b", Diff: 2}}
	// With one replicate the two fixed seeds select different Webb draws.
	for _, tc := range []struct {
		seed int64
		want float64
	}{{0, 1}, {1, 1.5 - math.Sqrt(.75)}} {
		got, err := rs.WildClusterBootstrapCI(obs, tc.seed, 1, .8)
		if err != nil || !near(got.Low, tc.want, 1e-12) || got.Low != got.High || got.Point != 1 || got.Tasks != 2 {
			t.Errorf("seed=%d: %+v (%v), want %v", tc.seed, got, err, tc.want)
		}
	}
}

func TestWildClusterBootstrapCI_LargeFiniteInterval(t *testing.T) {
	obs := []rs.Observation{{Task: "a", Diff: -1}, {Task: "b", Diff: 1}}
	want, err := rs.WildClusterBootstrapCI(obs, 27, 2, .8)
	if err != nil {
		t.Fatal(err)
	}
	for i := range obs {
		obs[i].Diff *= 9e307
	}
	got, err := rs.WildClusterBootstrapCI(obs, 27, 2, .8)
	if err != nil || got.Tasks != 2 || !(got.Low < 0 && got.High > 0) || math.IsInf(got.Low, 0) || math.IsInf(got.High, 0) || !near(got.Low/9e307, want.Low, 1e-12) || !near(got.High/9e307, want.High, 1e-12) {
		t.Errorf("scaled interval: %+v (%v), unscaled %+v", got, err, want)
	}
}

func TestWildClusterBootstrapCI_Errors(t *testing.T) {
	ok := []rs.Observation{{Task: "a", Diff: 1}, {Task: "b", Diff: 2}}
	for _, tc := range []struct {
		obs       []rs.Observation
		resamples int
		level     float64
		want      error
	}{
		{nil, 10, .95, rs.ErrEmptySample},
		{[]rs.Observation{{Task: "a", Diff: 1}, {Task: "a", Diff: 2}}, 10, .95, rs.ErrInsufficientClusters},
		{ok, 0, .95, rs.ErrInvalidResamples}, {ok, 10, 0, rs.ErrInvalidLevel}, {ok, 10, 1, rs.ErrInvalidLevel}, {ok, 10, math.NaN(), rs.ErrInvalidLevel},
		{[]rs.Observation{{Task: "", Diff: 1}, {Task: "b", Diff: 2}}, 10, .95, rs.ErrInvalidSample},
		{[]rs.Observation{{Task: "a", Diff: math.NaN()}, {Task: "b", Diff: 2}}, 10, .95, rs.ErrInvalidSample},
		{[]rs.Observation{{Task: "a", Diff: math.Inf(1)}, {Task: "b", Diff: 2}}, 10, .95, rs.ErrInvalidSample},
	} {
		if _, err := rs.WildClusterBootstrapCI(tc.obs, 1, tc.resamples, tc.level); !errors.Is(err, tc.want) {
			t.Errorf("%+v: %v want %v", tc, err, tc.want)
		}
	}
}
