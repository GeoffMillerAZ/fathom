package viz

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- golden file helpers ---------------------------------------------------

// goldenPath returns the path to a golden file in testdata/.
func goldenPath(name string) string {
	return filepath.Join("testdata", name+".svg.golden")
}

// assertGolden compares got against the stored golden file.
// If UPDATE_GOLDEN=1 is set in the environment, it writes/updates the file
// instead of asserting. Run `UPDATE_GOLDEN=1 go test ./...` to regenerate.
func assertGolden(t *testing.T, got, name string) {
	t.Helper()
	path := goldenPath(name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, string(want))
	}
}

// ---- fmtNum ---------------------------------------------------------------

func TestFmtNum(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{1.0, "1"},
		{1.5, "1.5"},
		{1.50, "1.5"},
		{1.23, "1.23"},
		{0.0, "0"},
		{100.0, "100"},
		{3.14159, "3.14"},
		{-1.5, "-1.5"},
	}
	for _, tc := range cases {
		got := fmtNum(tc.in)
		if got != tc.want {
			t.Errorf("fmtNum(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---- cssColor / role validation -------------------------------------------

func TestCSSColor_ValidRole(t *testing.T) {
	got := cssColor("accent")
	want := "var(--ak-color-accent)"
	if got != want {
		t.Errorf("cssColor(accent) = %q, want %q", got, want)
	}
}

func TestCSSColor_EmptyRole_FallsBack(t *testing.T) {
	got := cssColor("")
	if !strings.Contains(got, "ink-muted") {
		t.Errorf("cssColor(\"\") = %q, want fallback to ink-muted", got)
	}
}

func TestCSSColor_InvalidRole_FallsBack(t *testing.T) {
	cases := []string{"UPPER", "has space", "<script>", "accent;color:red"}
	for _, role := range cases {
		got := cssColor(role)
		if strings.Contains(got, role) {
			t.Errorf("cssColor(%q) injected invalid role into output: %q", role, got)
		}
		if !strings.Contains(got, "ink-muted") {
			t.Errorf("cssColor(%q) = %q, want fallback to ink-muted", role, got)
		}
	}
}

// ---- Sparkline ------------------------------------------------------------

func TestSparkline_Golden(t *testing.T) {
	got := Sparkline([]float64{1, 3, 2, 5, 4, 6, 3}, SparklineOptions{})
	assertGolden(t, got, "sparkline_default")
}

func TestSparkline_WithFill_Golden(t *testing.T) {
	got := Sparkline([]float64{10, 20, 15, 25}, SparklineOptions{
		W:          200,
		H:          40,
		StrokeRole: "accent",
		FillRole:   "accent-soft",
		AriaLabel:  "revenue trend",
	})
	assertGolden(t, got, "sparkline_with_fill")
}

func TestSparkline_Empty(t *testing.T) {
	got := Sparkline(nil, SparklineOptions{})
	if !strings.Contains(got, `<svg`) {
		t.Errorf("empty sparkline missing svg tag")
	}
	if strings.Contains(got, `<polyline`) {
		t.Errorf("empty sparkline should not have polyline")
	}
}

func TestSparkline_SingleValue(t *testing.T) {
	got := Sparkline([]float64{42}, SparklineOptions{})
	if !strings.Contains(got, `<polyline`) {
		t.Errorf("single-value sparkline should emit a polyline")
	}
}

func TestSparkline_AllEqual(t *testing.T) {
	// All-equal → flat midline (should not divide by zero).
	got := Sparkline([]float64{5, 5, 5, 5}, SparklineOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("all-equal sparkline has NaN/Inf: %s", got)
	}
}

func TestSparkline_NaNFiltered(t *testing.T) {
	got := Sparkline([]float64{1, math.NaN(), 3}, SparklineOptions{})
	if strings.Contains(got, "NaN") {
		t.Errorf("NaN leaked into sparkline output: %s", got)
	}
}

func TestSparkline_Determinism(t *testing.T) {
	vals := []float64{1, 3, 2, 5, 4}
	a := Sparkline(vals, SparklineOptions{})
	b := Sparkline(vals, SparklineOptions{})
	if a != b {
		t.Errorf("sparkline non-deterministic:\na=%s\nb=%s", a, b)
	}
}

func TestSparkline_LabelEscaping(t *testing.T) {
	got := Sparkline([]float64{1, 2}, SparklineOptions{AriaLabel: `<script>alert(1)</script>`})
	if strings.Contains(got, "<script>") {
		t.Errorf("sparkline did not escape ariaLabel: %s", got)
	}
}

// ---- StackBar -------------------------------------------------------------

func TestStackBar_Golden(t *testing.T) {
	segs := []Segment{
		{Value: 60, Role: "positive", Label: "scanned"},
		{Value: 25, Role: "caution", Label: "pending"},
		{Value: 15, Role: "danger", Label: "failed"},
	}
	got := StackBar(segs, StackBarOptions{})
	assertGolden(t, got, "stackbar_default")
}

func TestStackBar_ZeroTotal(t *testing.T) {
	got := StackBar(nil, StackBarOptions{})
	if !strings.Contains(got, `<rect`) {
		t.Errorf("zero-total stackbar should emit empty-state rect")
	}
	if !strings.Contains(got, "ak-color-line") {
		t.Errorf("zero-total stackbar should use line role color")
	}
}

func TestStackBar_SingleSegment(t *testing.T) {
	got := StackBar([]Segment{{Value: 1, Role: "accent", Label: "only"}}, StackBarOptions{})
	if !strings.Contains(got, `<path`) {
		t.Errorf("single-segment stackbar should emit path")
	}
}

func TestStackBar_LabelEscaping(t *testing.T) {
	segs := []Segment{{Value: 1, Role: "accent", Label: `<script>alert(1)</script>`}}
	got := StackBar(segs, StackBarOptions{})
	if strings.Contains(got, "<script>") {
		t.Errorf("stackbar did not escape label: %s", got)
	}
}

func TestStackBar_InvalidRole_FallsBack(t *testing.T) {
	segs := []Segment{{Value: 1, Role: "INVALID ROLE", Label: "x"}}
	got := StackBar(segs, StackBarOptions{})
	if strings.Contains(got, "INVALID ROLE") {
		t.Errorf("stackbar injected invalid role: %s", got)
	}
}

func TestStackBar_Determinism(t *testing.T) {
	segs := []Segment{
		{Value: 50, Role: "positive", Label: "a"},
		{Value: 30, Role: "caution", Label: "b"},
		{Value: 20, Role: "danger", Label: "c"},
	}
	a := StackBar(segs, StackBarOptions{})
	b := StackBar(segs, StackBarOptions{})
	if a != b {
		t.Errorf("stackbar non-deterministic")
	}
}

// ---- Donut ----------------------------------------------------------------

func TestDonut_Golden(t *testing.T) {
	segs := []Segment{
		{Value: 3, Role: "accent", Label: "tier-1"},
		{Value: 5, Role: "info", Label: "tier-2"},
		{Value: 2, Role: "ink-muted", Label: "tier-3"},
	}
	got := Donut(segs, DonutOptions{CenterText: "10"})
	assertGolden(t, got, "donut_default")
}

func TestDonut_ZeroTotal(t *testing.T) {
	got := Donut(nil, DonutOptions{})
	if !strings.Contains(got, `<circle`) {
		t.Errorf("zero-total donut should emit empty circle")
	}
}

func TestDonut_CenterTextEscaping(t *testing.T) {
	got := Donut([]Segment{{Value: 1, Role: "accent", Label: "x"}}, DonutOptions{
		CenterText: `<script>alert(1)</script>`,
	})
	if strings.Contains(got, "<script>") {
		t.Errorf("donut did not escape center text: %s", got)
	}
}

func TestDonut_LabelEscaping(t *testing.T) {
	segs := []Segment{{Value: 1, Role: "accent", Label: `<b>bold</b>`}}
	got := Donut(segs, DonutOptions{})
	if strings.Contains(got, "<b>") {
		t.Errorf("donut did not escape segment label: %s", got)
	}
}

func TestDonut_Determinism(t *testing.T) {
	segs := []Segment{
		{Value: 3, Role: "accent", Label: "a"},
		{Value: 5, Role: "info", Label: "b"},
	}
	a := Donut(segs, DonutOptions{})
	b := Donut(segs, DonutOptions{})
	if a != b {
		t.Errorf("donut non-deterministic")
	}
}

func TestDonut_SingleSegment(t *testing.T) {
	// Single segment = full ring; should not produce NaN/Inf coordinates.
	got := Donut([]Segment{{Value: 1, Role: "accent", Label: "all"}}, DonutOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("single-segment donut has NaN/Inf: %s", got)
	}
}

// ---- Funnel ---------------------------------------------------------------

func TestFunnel_Golden(t *testing.T) {
	stages := []Stage{
		{Label: "proposed", Value: 100, Role: "info"},
		{Label: "imported", Value: 72, Role: "accent"},
		{Label: "excluded", Value: 18, Role: "caution"},
		{Label: "final", Value: 54, Role: "positive"},
	}
	got := Funnel(stages, FunnelOptions{})
	assertGolden(t, got, "funnel_default")
}

func TestFunnel_Empty(t *testing.T) {
	got := Funnel(nil, FunnelOptions{})
	if !strings.Contains(got, `<svg`) {
		t.Errorf("empty funnel missing svg tag")
	}
}

func TestFunnel_ZeroFirstStage(t *testing.T) {
	stages := []Stage{
		{Label: "a", Value: 0, Role: "accent"},
		{Label: "b", Value: 10, Role: "positive"},
	}
	// Zero first stage; should not divide by zero.
	got := Funnel(stages, FunnelOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("zero-first-stage funnel has NaN/Inf: %s", got)
	}
}

func TestFunnel_LabelEscaping(t *testing.T) {
	stages := []Stage{{Label: `<script>alert(1)</script>`, Value: 5, Role: "accent"}}
	got := Funnel(stages, FunnelOptions{})
	if strings.Contains(got, "<script>") {
		t.Errorf("funnel did not escape label: %s", got)
	}
}

func TestFunnel_Determinism(t *testing.T) {
	stages := []Stage{
		{Label: "a", Value: 100, Role: "info"},
		{Label: "b", Value: 60, Role: "accent"},
	}
	a := Funnel(stages, FunnelOptions{})
	b := Funnel(stages, FunnelOptions{})
	if a != b {
		t.Errorf("funnel non-deterministic")
	}
}

// ---- MeterRow -------------------------------------------------------------

func TestMeterRow_Golden(t *testing.T) {
	got := MeterRow(65, 100, MeterOptions{Label: "coverage", ShowPct: true})
	assertGolden(t, got, "meter_default")
}

func TestMeterRow_ZeroMax(t *testing.T) {
	got := MeterRow(0, 0, MeterOptions{})
	if !strings.Contains(got, `<rect`) {
		t.Errorf("zero-max meter should emit track rect")
	}
	// Should not have a fill rect beyond the track.
	rects := strings.Count(got, `<rect`)
	if rects > 1 {
		t.Errorf("zero-max meter emitted fill rect (rects=%d)", rects)
	}
}

func TestMeterRow_ClampValue(t *testing.T) {
	// value > max → clamped to max; should not produce negative width.
	got := MeterRow(200, 100, MeterOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("clamped meter has NaN/Inf: %s", got)
	}
}

func TestMeterRow_NegativeValue(t *testing.T) {
	got := MeterRow(-10, 100, MeterOptions{})
	// No fill rect for zero value.
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("negative value meter has NaN/Inf: %s", got)
	}
}

func TestMeterRow_LabelEscaping(t *testing.T) {
	got := MeterRow(50, 100, MeterOptions{Label: `<script>alert(1)</script>`})
	if strings.Contains(got, "<script>") {
		t.Errorf("meter did not escape label: %s", got)
	}
}

func TestMeterRow_Determinism(t *testing.T) {
	a := MeterRow(75, 100, MeterOptions{Label: "coverage", ShowPct: true})
	b := MeterRow(75, 100, MeterOptions{Label: "coverage", ShowPct: true})
	if a != b {
		t.Errorf("meter non-deterministic")
	}
}

// ---- Accessibility (shared) -----------------------------------------------

func TestAllComponents_HaveRoleAndAriaLabel(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{"sparkline", Sparkline([]float64{1, 2, 3}, SparklineOptions{AriaLabel: "test"})},
		{"stackbar", StackBar([]Segment{{Value: 1, Role: "accent", Label: "x"}}, StackBarOptions{AriaLabel: "test"})},
		{"donut", Donut([]Segment{{Value: 1, Role: "accent", Label: "x"}}, DonutOptions{AriaLabel: "test"})},
		{"funnel", Funnel([]Stage{{Label: "x", Value: 1, Role: "accent"}}, FunnelOptions{AriaLabel: "test"})},
		{"meter", MeterRow(50, 100, MeterOptions{AriaLabel: "test"})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.got, `role="img"`) {
				t.Errorf("%s missing role=img", tc.name)
			}
			if !strings.Contains(tc.got, `aria-label=`) {
				t.Errorf("%s missing aria-label", tc.name)
			}
			if !strings.Contains(tc.got, `<title>`) {
				t.Errorf("%s missing <title>", tc.name)
			}
		})
	}
}
