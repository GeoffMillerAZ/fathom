package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// TestStageAStatic: every expression is in the denominator; an expression is
// recalled when fathom joins every Service its truth names on its own line;
// a join outside the truth costs precision; the partition says why an
// expression has no truth; the kill fires below .60.
func TestStageAStatic(t *testing.T) {
	dir := t.TempDir()
	truth := `{"tool": "t", "version": "v", "tier": "C", "exprs": [
 {"path": "r.yaml", "from": "PrometheusRule/m/r", "line": 10, "rule": "A", "kind": "alert", "matchers": ["job=\"api\""], "truth": [{"to_path": "s.yaml", "to": "Service/m/api"}], "parsed": true},
 {"path": "r.yaml", "from": "PrometheusRule/m/r", "line": 12, "rule": "B", "kind": "alert", "matchers": ["job=\"api\""], "truth": [{"to_path": "s.yaml", "to": "Service/m/api"}], "parsed": true},
 {"path": "r.yaml", "from": "PrometheusRule/m/r", "line": 14, "rule": "c:rate", "kind": "record", "matchers": ["job=\"apiserver\""], "truth": [], "parsed": true},
 {"path": "r.yaml", "from": "PrometheusRule/m/r", "line": 16, "rule": "D", "kind": "alert", "matchers": [], "truth": [], "parsed": true}
]}`
	tp := filepath.Join(dir, "truth.json")
	if err := os.WriteFile(tp, []byte(truth), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := func(line int, to string) fusion.RouteRef {
		return fusion.RouteRef{FromFile: "r.yaml", From: "PrometheusRule/m/r", ToFile: "s.yaml", To: to, Line: line, Via: fusion.RouteHostStrategy}
	}
	res := fusion.RouteResolution{Refs: []fusion.RouteRef{ref(10, "Service/m/api"), ref(16, "Service/m/other")}}
	out := filepath.Join(dir, "stage-a.json")
	r, err := writeStageA(tp, out, "subj", "2026-10-09", "abc", res)
	if err != nil {
		t.Fatal(err)
	}
	if r.Exprs != 4 || r.Alerts != 3 || r.Records != 1 {
		t.Errorf("counts %d/%d/%d", r.Exprs, r.Alerts, r.Records)
	}
	if r.LabelJoinRecall != 0.25 || r.LabelJoinPrecision != 0.5 || r.RecallJoinable != 0.5 {
		t.Errorf("recall %v precision %v joinable %v, want .25 .5 .5", r.LabelJoinRecall, r.LabelJoinPrecision, r.RecallJoinable)
	}
	if r.Partition.Joinable.Exprs != 2 || r.Partition.Platform.Exprs != 1 || r.Partition.Unlabelled.Exprs != 1 || !r.Kill.Fired {
		t.Errorf("partition %+v kill %+v", r.Partition, r.Kill)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"label_join_recall", "label_join_precision"} {
		if _, ok := m[k]; !ok {
			t.Errorf("the reading lacks %q", k)
		}
	}
}
