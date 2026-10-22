package langbench

import "testing"

func TestLanguageNearMissSelection(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		margin float64
		want   bool
	}{
		{"below just outside", .739, .01, false}, {"boundary", .74, .01, true}, {"below near", .745, .01, true}, {"at bar", .75, .01, false}, {"above", .76, .01, false}, {"wider", .72, .04, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gapFixture(t)
			r.Metrics[MRangeAgreement] = new(tt.value)
			got := NearMisses(r, tt.margin)
			if (len(got) > 0) != tt.want {
				t.Fatalf("got=%+v", got)
			}
			if tt.want && (got[0].Metric != MRangeAgreement || got[0].Bar != .75 || got[0].GrammarCommit != r.Grammar.Commit) {
				t.Fatalf("got=%+v", got)
			}
		})
	}
	r := gapFixture(t)
	r.Tenancy.FilesWithError = new(6)
	if got := NearMisses(r, .01); len(got) != 1 || got[0].Metric != "parse_error_rate" || got[0].Bar != .05 {
		t.Fatalf("parse: %+v", got)
	}
	r.Tenancy.FilesWithError = new(5)
	if got := NearMisses(r, .01); len(got) != 0 {
		t.Fatalf("at parse bar: %+v", got)
	}
	r.TargetLevel = "L3"
	r.Metrics[MCallRecall] = new(.1)
	r.Metrics[MImportResolution] = new(.84)
	if got := NearMisses(r, .01); len(got) != 0 {
		t.Fatalf("later level cannot hide first blocker: %+v", got)
	}
}
