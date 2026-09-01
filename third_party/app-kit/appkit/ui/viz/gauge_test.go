package viz

import (
	"strings"
	"testing"
)

func TestGauge_Golden(t *testing.T) {
	got := Gauge(72, 100, GaugeOptions{Label: "case strength", Graded: true})
	assertGolden(t, got, "gauge_default")
}

func TestGauge_ZeroMax(t *testing.T) {
	got := Gauge(0, 0, GaugeOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("zero-max gauge has NaN/Inf: %s", got)
	}
	// Track present, no fill path beyond it.
	if strings.Count(got, "<path") != 1 {
		t.Errorf("zero-max gauge should render only the track path")
	}
}

func TestGauge_ClampsOverAndUnder(t *testing.T) {
	for _, v := range []float64{-50, 250} {
		got := Gauge(v, 100, GaugeOptions{})
		if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
			t.Errorf("gauge(%v) has NaN/Inf", v)
		}
	}
}

func TestGradedRole(t *testing.T) {
	cases := map[float64]string{0.1: "danger", 0.5: "caution", 0.9: "positive"}
	for f, want := range cases {
		if got := GradedRole(f); got != want {
			t.Errorf("GradedRole(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestGauge_GradedColors(t *testing.T) {
	strong := Gauge(90, 100, GaugeOptions{Graded: true})
	if !strings.Contains(strong, "ak-color-positive") {
		t.Errorf("strong graded gauge should use positive role")
	}
	weak := Gauge(10, 100, GaugeOptions{Graded: true})
	if !strings.Contains(weak, "ak-color-danger") {
		t.Errorf("weak graded gauge should use danger role")
	}
}

func TestGauge_CenterTextDefaultPercent(t *testing.T) {
	got := Gauge(50, 100, GaugeOptions{})
	if !strings.Contains(got, ">50%<") {
		t.Errorf("default center should be rounded percent, got: %s", got)
	}
}

func TestGauge_Escaping(t *testing.T) {
	got := Gauge(1, 2, GaugeOptions{Label: `<script>`, CenterText: `<b>`, AriaLabel: `<i>`})
	for _, bad := range []string{"<script>", "<b>", "<i>"} {
		if strings.Contains(got, bad) {
			t.Errorf("gauge did not escape %q", bad)
		}
	}
}

func TestGauge_Determinism(t *testing.T) {
	a := Gauge(33, 100, GaugeOptions{Label: "x", Graded: true})
	b := Gauge(33, 100, GaugeOptions{Label: "x", Graded: true})
	if a != b {
		t.Errorf("gauge non-deterministic")
	}
}

func TestGauge_Accessibility(t *testing.T) {
	got := Gauge(1, 2, GaugeOptions{AriaLabel: "likelihood"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("gauge missing accessibility attributes")
	}
}
