package langbench

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func gapFixture(t *testing.T) Reading {
	t.Helper()
	b, err := os.ReadFile("testdata/gap-reading.json")
	if err != nil {
		t.Fatal(err)
	}
	var r Reading
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLanguageGapClasses(t *testing.T) {
	tests := []struct {
		name  string
		alter func(*Reading)
		class string
		count int
	}{
		{"range", func(r *Reading) { r.Metrics[MRangeAgreement] = new(.74) }, "range mismatch", 3},
		{"import", func(r *Reading) { r.TargetLevel = "L3"; r.Metrics[MImportResolution] = new(.80) }, "unresolved import", 2},
		{"kind", func(r *Reading) { r.RequiredKinds = []string{"class", "method"} }, "missing kind", 1},
		{"tier", func(r *Reading) { r.MetricTiers[MSymRecall] = "D" }, "oracle tier too weak", 1},
		{"parse", func(r *Reading) { r.Tenancy.FilesWithError = new(6) }, "parse error rate", 6},
		{"unmeasured", func(r *Reading) { delete(r.Metrics, MRangeAgreement) }, "metric not measured", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gapFixture(t)
			tt.alter(&r)
			report := ExplainGap(r)
			found := false
			for _, g := range report.Gaps {
				if g.Class == tt.class {
					found = true
					if g.Count != tt.count || g.Next == "" {
						t.Fatalf("gap=%+v", g)
					}
				}
			}
			if !found {
				t.Fatalf("missing %q: %+v", tt.class, report)
			}
			if !reflect.DeepEqual(report, ExplainGap(r)) {
				t.Fatal("not deterministic")
			}
		})
	}
	r := gapFixture(t)
	if got := ExplainGap(r); len(got.Gaps) != 0 {
		t.Fatalf("at target: %+v", got)
	}
}
