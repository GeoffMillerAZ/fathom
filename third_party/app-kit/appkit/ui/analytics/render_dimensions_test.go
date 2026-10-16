package analytics

import (
	"math"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func TestRenderBarsRejectsLabelWidthOutsideLayoutBudget(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		labelW int
		data   func() Dataset
	}{
		{name: "maximum integer", width: 560, labelW: math.MaxInt, data: fixture},
		{name: "near maximum integer", width: 560, labelW: math.MaxInt - 1, data: fixture},
		{name: "label wider than chart", width: 560, labelW: 561, data: fixture},
		{name: "empty dataset still validates dimensions", width: 560, labelW: math.MaxInt, data: func() Dataset {
			d := fixture()
			d.Rows = nil
			return d
		}},
		{name: "missing dataset still validates dimensions", width: 560, labelW: math.MaxInt, data: func() Dataset {
			d := fixture()
			d.Contract.Completeness = Partial
			d.Contract.CoverageNote = "One value was not observed"
			d.Rows = []Row{{ID: "sept", Label: "September 2026", MissingReason: "Not observed"}}
			return d
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderBars(tt.data(), viz.BarsOptions{W: tt.width, LabelW: tt.labelW}, testNow)
			if err == nil || !strings.Contains(err.Error(), "dimensions") {
				t.Fatalf("RenderBars() = %q, %v; want dimension error", got, err)
			}
			if got != "" {
				t.Fatalf("RenderBars() returned partial output %q", got)
			}
		})
	}
}

func TestRenderBarsAcceptsLabelWidthBoundaryAndPreservesDefaults(t *testing.T) {
	if got, err := RenderBars(fixture(), viz.BarsOptions{RowH: 1}, testNow); err != nil || got == "" {
		t.Fatalf("positive sub-default row height must remain accepted: %q, %v", got, err)
	}
	const width = 560
	boundary, err := RenderBars(fixture(), viz.BarsOptions{W: width, LabelW: width - 120}, testNow)
	if err != nil {
		t.Fatalf("RenderBars() rejected legitimate boundary: %v", err)
	}
	if boundary == "" {
		t.Fatal("RenderBars() returned empty boundary output")
	}

	defaults, err := RenderBars(fixture(), viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatalf("RenderBars() with defaults: %v", err)
	}
	explicit, err := RenderBars(fixture(), viz.BarsOptions{W: 560, LabelW: 180, RowH: 28}, testNow)
	if err != nil {
		t.Fatalf("RenderBars() with explicit defaults: %v", err)
	}
	if defaults != explicit {
		t.Fatal("RenderBars() defaults changed ordinary output")
	}
}
