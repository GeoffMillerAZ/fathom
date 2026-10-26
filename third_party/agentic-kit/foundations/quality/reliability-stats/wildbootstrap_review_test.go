package reliabilitystats_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

func TestWildClusterBootstrapCI_TaskWeightedRepositoryClusters(t *testing.T) {
	// Four tasks, seven trials, two repositories. Task mean = .75,
	// trial mean = 3/7, repository mean = 1.5. Only the first is the estimand.
	var obs []rs.Observation
	if err := json.Unmarshal([]byte(`[
  {"task":"a","cluster":"left","diff":0},
  {"task":"a","cluster":"left","diff":0},
  {"task":"a","cluster":"left","diff":0},
  {"task":"a","cluster":"left","diff":0},
  {"task":"b","cluster":"left","diff":0},
  {"task":"c","cluster":"left","diff":0},
  {"task":"d","cluster":"right","diff":3}
 ]`), &obs); err != nil {
		t.Fatal(err)
	}
	original := append([]rs.Observation(nil), obs...)
	got, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil {
		t.Fatal(err)
	}
	// Cluster residuals: (-.75-.75-.75)/4, (3-.75)/4.
	// Multiplying each by sqrt(2) and by one Webb draw per cluster
	// gives extrema .75 +/- 1.125*sqrt(3), each with probability 1/36.
	half := 1.125 * math.Sqrt(3)
	if got.Point != .75 || got.Tasks != 4 || !near(got.Low, .75-half, 1e-12) || !near(got.High, .75+half, 1e-12) {
		t.Fatalf("task-weighted interval: %+v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var counts struct {
		Clusters int `json:"clusters"`
	}
	if err := json.Unmarshal(encoded, &counts); err != nil {
		t.Fatal(err)
	}
	if counts.Clusters != 2 {
		t.Fatalf("want two clusters in %s", encoded)
	}
	again, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil || again != got {
		t.Fatalf("seed changed result: %+v (%v)", again, err)
	}
	if !reflect.DeepEqual(obs, original) {
		t.Fatal("mutated input")
	}
	for i, j := 0, len(obs)-1; i < j; i, j = i+1, j-1 {
		obs[i], obs[j] = obs[j], obs[i]
	}
	shuffled, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil || shuffled != got {
		t.Fatalf("order changed result: %+v (%v)", shuffled, err)
	}
	// Tripling all trials of just one task leaves its task mean and the
	// entire seeded repository interval bit-identical.
	obs = append(obs, original[:4]...)
	obs = append(obs, original[:4]...)
	repeated, err := rs.WildClusterBootstrapCI(obs, 42, 20000, .95)
	if err != nil || repeated != got {
		t.Fatalf("trial replication changed interval: %+v (%v)", repeated, err)
	}
}

func TestWildClusterBootstrapCI_ClusterContractRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        error
	}{
		{"single repository", `[{"task":"a","cluster":"only","diff":1},{"task":"b","cluster":"only","diff":2}]`, rs.ErrInsufficientClusters},
		{"task in two repositories", `[{"task":"a","cluster":"left","diff":1},{"task":"a","cluster":"right","diff":2},{"task":"b","cluster":"right","diff":0}]`, rs.ErrInconsistentCluster},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var obs []rs.Observation
			if err := json.Unmarshal([]byte(tc.input), &obs); err != nil {
				t.Fatal(err)
			}
			if _, err := rs.WildClusterBootstrapCI(obs, 1, 10, .95); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
