package reliabilitystats_test

import (
	"encoding/json"
	"errors"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

func TestWildClusterBootstrapCI_InconsistentCluster(t *testing.T) {
	for _, clusters := range [][2]string{{"left", "right"}, {"", "right"}, {"right", ""}} {
		obs := []rs.Observation{{Task: "a", Cluster: clusters[0], Diff: 1}, {Task: "a", Cluster: clusters[1], Diff: 2}, {Task: "b", Diff: 0}}
		_, err := rs.WildClusterBootstrapCI(obs, 1, 10, .95)
		if !errors.Is(err, rs.ErrInconsistentCluster) || err.Error() != "reliabilitystats: task belongs to multiple clusters" || errors.Is(err, rs.ErrInvalidSample) {
			t.Errorf("clusters %q: got %v, want distinct inconsistent-cluster error", clusters, err)
		}
	}
	// Explicitly naming the default cluster is consistent.
	obs := []rs.Observation{{Task: "a", Diff: 1}, {Task: "a", Cluster: "a", Diff: 2}, {Task: "b", Diff: 0}}
	if _, err := rs.WildClusterBootstrapCI(obs, 1, 10, .95); err != nil {
		t.Fatalf("matching default and explicit cluster: %v", err)
	}
}

func TestEquivalenceTOST_UnexpectedEffect(t *testing.T) {
	for _, tc := range []struct {
		name       string
		x          []float64
		alpha      float64
		unexpected bool
		verdict    rs.Verdict
	}{
		{"positive beyond margin", []float64{1.9, 2, 2.1}, .05, true, rs.VerdictInconclusive},
		{"constant beyond margin", []float64{2, 2}, .05, true, rs.VerdictInconclusive},
		{"at margin", []float64{1, 1}, .05, false, rs.VerdictInconclusive},
		{"positive overlapping margin", []float64{.5, 1.5}, .05, false, rs.VerdictInconclusive},
		{"equivalent", []float64{.25, .25}, .05, false, rs.VerdictEquivalent},
		{"zero", []float64{0, 0}, .05, false, rs.VerdictEquivalent},
		{"harm", []float64{-2, -2}, .05, false, rs.VerdictHarm},
		{"caller alpha", []float64{1.5, 2.5}, .25, true, rs.VerdictInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := rs.EquivalenceTOST(tc.x, make([]float64, len(tc.x)), 1, tc.alpha)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				UnexpectedEffect *bool `json:"unexpected_effect"`
			}
			if err := json.Unmarshal(encoded, &wire); err != nil {
				t.Fatal(err)
			}
			if wire.UnexpectedEffect == nil || *wire.UnexpectedEffect != tc.unexpected || r.Verdict != tc.verdict || r.Equivalent != (tc.verdict == rs.VerdictEquivalent) {
				t.Fatalf("got %s, want unexpected_effect=%v and verdict=%s", encoded, tc.unexpected, tc.verdict)
			}
		})
	}
}
