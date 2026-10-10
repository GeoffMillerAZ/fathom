package main

// langroutes_telemetry.go — the Stage A static reading (TL-8's deploy half,
// wave c1 item mesh): does a PrometheusRule expression's identifying
// matcher, read by fathom's route pass (the kubernetes dialect's promql mode,
// LB-F312, resolved by fusion.ResolveRouteRefs), reach the Service the
// expression's series come from? The truth is the kubernetes oracle's
// -labeljoins-out file (labeljoins.go there: prometheus-operator's
// ServiceMonitor jobLabel semantics, tier C, so the reading caps at good).
// The denominator is every alert and recording expression; each is scored
// by its own line (the route reference's Line is the expr value's).
//
//	label_join_recall: expressions whose every truth Service fathom joins,
//	  over every expression (one with no truth can never count);
//	label_join_precision: fathom's (expression, Service) joins in the truth,
//	  over all of fathom's joins.
//
// The partition says why an expression has no truth: platform (its job or
// service names no Service the subject declares: a control-plane job the
// cluster provides) or unlabelled (no job or service equality matcher).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

type labelJoinTruth struct {
	Tool    string `json:"tool"`
	Version string `json:"version"`
	Tier    string `json:"tier"`
	Exprs   []struct {
		Path     string   `json:"path"`
		From     string   `json:"from"`
		Line     int      `json:"line"`
		Rule     string   `json:"rule"`
		Kind     string   `json:"kind"`
		Matchers []string `json:"matchers"`
		Truth    []struct {
			ToPath string `json:"to_path"`
			To     string `json:"to"`
		} `json:"truth"`
		Parsed bool `json:"parsed"`
	} `json:"exprs"`
}

// stageAPartition counts expressions by why they have (or lack) a truth.
type stageAPartition struct {
	Exprs    int `json:"exprs"`
	Recalled int `json:"recalled"`
}

// stageAReading is docs/bench/readings/telemetry/stage-a-static.json.
type stageAReading struct {
	Reading            string  `json:"reading"`
	Subject            string  `json:"subject"`
	MeasuredAt         string  `json:"measured_at"`
	FathomCommit       string  `json:"fathom_commit"`
	Oracle             string  `json:"oracle"`
	OracleTier         string  `json:"oracle_tier"`
	GradeCap           string  `json:"grade_cap"`
	LabelJoinRecall    float64 `json:"label_join_recall"`
	LabelJoinPrecision float64 `json:"label_join_precision"`
	// RecallJoinable is the recall over the expressions with a truth.
	RecallJoinable float64 `json:"recall_joinable"`
	Exprs          int     `json:"exprs"`
	Alerts         int     `json:"alerts"`
	Records        int     `json:"records"`
	FathomJoins    int     `json:"fathom_joins"`
	JoinsInTruth   int     `json:"joins_in_truth"`
	Partition      struct {
		Joinable   stageAPartition `json:"joinable"`
		Platform   stageAPartition `json:"platform"`
		Unlabelled stageAPartition `json:"unlabelled"`
	} `json:"partition"`
	Kill struct {
		Threshold float64 `json:"threshold"`
		Fired     bool    `json:"fired"`
		Rule      string  `json:"rule"`
	} `json:"kill"`
	Misses []string `json:"misses"`
	Extras []string `json:"extras"`
}

// writeStageA scores fathom's route references against the label-join truth
// and writes the reading.
func writeStageA(truthPath, out, subject, date, commit string, res fusion.RouteResolution) (stageAReading, error) {
	var r stageAReading
	b, err := os.ReadFile(truthPath)
	if err != nil {
		return r, err
	}
	var t labelJoinTruth
	if err := json.Unmarshal(b, &t); err != nil {
		return r, fmt.Errorf("%s: %w", truthPath, err)
	}
	type key struct {
		path, from string
		line       int
	}
	type pair struct{ toPath, to string }
	joins := map[key]map[pair]bool{}
	for _, ref := range res.Refs {
		if ref.Via != fusion.RouteHostStrategy {
			continue
		}
		k := key{ref.FromFile, ref.From, ref.Line}
		if joins[k] == nil {
			joins[k] = map[pair]bool{}
		}
		joins[k][pair{ref.ToFile, ref.To}] = true
	}
	r.Reading, r.Subject, r.MeasuredAt, r.FathomCommit = "stage-a-static", subject, date, commit
	r.Oracle, r.OracleTier, r.GradeCap = t.Tool+" "+t.Version, t.Tier, "good"
	r.Kill.Threshold, r.Kill.Rule = 0.60, "label_join_recall below the threshold: ship rules parsed at L2 with no join, and write the ceiling"
	r.Misses, r.Extras = []string{}, []string{}
	for _, e := range t.Exprs {
		r.Exprs++
		if e.Kind == "alert" {
			r.Alerts++
		} else {
			r.Records++
		}
		found := joins[key{e.Path, e.From, e.Line}]
		truth := map[pair]bool{}
		for _, x := range e.Truth {
			truth[pair{x.ToPath, x.To}] = true
		}
		for p := range found {
			r.FathomJoins++
			if truth[p] {
				r.JoinsInTruth++
			} else {
				r.Extras = append(r.Extras, fmt.Sprintf("%s:%d %s %s -> %s", e.Path, e.Line, e.Rule, joinMatchers(e.Matchers), p.to))
			}
		}
		recalled := len(truth) > 0
		for p := range truth {
			if !found[p] {
				recalled = false
				r.Misses = append(r.Misses, fmt.Sprintf("%s:%d %s %s -> %s", e.Path, e.Line, e.Rule, joinMatchers(e.Matchers), p.to))
			}
		}
		part := &r.Partition.Unlabelled
		switch {
		case len(truth) > 0:
			part = &r.Partition.Joinable
		case len(e.Matchers) > 0:
			part = &r.Partition.Platform
		}
		part.Exprs++
		if recalled {
			part.Recalled++
		}
	}
	sort.Strings(r.Misses)
	sort.Strings(r.Extras)
	if r.Exprs > 0 {
		r.LabelJoinRecall = round4(float64(r.Partition.Joinable.Recalled) / float64(r.Exprs))
	}
	if r.FathomJoins > 0 {
		r.LabelJoinPrecision = round4(float64(r.JoinsInTruth) / float64(r.FathomJoins))
	}
	if r.Partition.Joinable.Exprs > 0 {
		r.RecallJoinable = round4(float64(r.Partition.Joinable.Recalled) / float64(r.Partition.Joinable.Exprs))
	}
	r.Kill.Fired = r.LabelJoinRecall < r.Kill.Threshold
	b, err = json.MarshalIndent(r, "", " ")
	if err != nil {
		return r, err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return r, err
	}
	return r, os.WriteFile(out, append(b, '\n'), 0o644)
}

func joinMatchers(ms []string) string {
	if len(ms) == 0 {
		return "{}"
	}
	out := "{" + ms[0]
	for _, m := range ms[1:] {
		out += "," + m
	}
	return out + "}"
}

func round4(f float64) float64 {
	return float64(int64(f*10000+0.5)) / 10000
}
