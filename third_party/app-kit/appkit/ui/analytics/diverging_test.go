package analytics

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func signedFixture() Dataset {
	d := fixture()
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One observation is missing."
	d.Rows = []Row{
		{ID: "n2", Label: "Negative two", Value: value(-2)},
		{ID: "n1", Label: "Negative one", Value: value(-1)},
		{ID: "zero", Label: "Zero", Value: value(0)},
		{ID: "p1", Label: "Positive one", Value: value(1)},
		{ID: "p2", Label: "Positive two", Value: value(2)},
		{ID: "missing", Label: "Unobserved", MissingReason: "Not reported"},
	}
	return d
}

func signedNodes(t *testing.T, markup, attr string) []*html.Node {
	t.Helper()
	n, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for _, a := range n.Attr {
			if a.Key == attr {
				found = append(found, n)
				break
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

func signedAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func signedNumber(t *testing.T, n *html.Node, key string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(signedAttr(n, key), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		t.Fatalf("invalid %s=%q", key, signedAttr(n, key))
	}
	return v
}

func TestDivergingGeometryAndTruth(t *testing.T) {
	d := signedFixture()
	h, err := RenderDivergingBars(d, viz.BarsOptions{W: 640, LabelW: 180}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	axes, marks, zeroes := signedNodes(t, h, "data-analytics-zero-axis"), signedNodes(t, h, "data-analytics-bar"), signedNodes(t, h, "data-analytics-zero")
	if len(axes) != 1 || len(marks) != 4 || len(zeroes) != 1 {
		t.Fatalf("axes=%d marks=%d zeroes=%d", len(axes), len(marks), len(zeroes))
	}
	x0 := signedNumber(t, axes[0], "x1")
	widths := []float64{2, 1, 1, 2}
	unit := signedNumber(t, marks[1], "width")
	if unit <= 0 {
		t.Fatal("invisible marks")
	}
	for i, m := range marks {
		x, w := signedNumber(t, m, "x"), signedNumber(t, m, "width")
		if math.Abs(w-unit*widths[i]) > .03 {
			t.Fatalf("mark %d magnitude: %g", i, w)
		}
		if i < 2 && math.Abs(x+w-x0) > .03 || i >= 2 && math.Abs(x-x0) > .03 {
			t.Fatalf("mark %d is on wrong side: x=%g width=%g zero=%g", i, x, w, x0)
		}
		if signedAttr(m, "fill") != "var(--ak-color-accent)" {
			t.Fatal("sign must not assign semantic role")
		}
	}
	if math.Abs(signedNumber(t, zeroes[0], "x1")-x0) > .03 {
		t.Fatal("zero not on baseline")
	}
	rows := signedNodes(t, h, "data-analytics-row")
	if len(rows) != 5 {
		t.Fatalf("missing observation became a mark: %d", len(rows))
	}
	for _, want := range []string{"-2 USD", "-1 USD", "0 USD", "1 USD", "2 USD", "Not reported", "Coverage: partial", "Uncertainty:"} {
		if !strings.Contains(h, want) {
			t.Errorf("missing truth %q", want)
		}
	}
	csv, _ := dataCSV(t, d)
	for i, want := range []string{"-2", "-1", "0", "1", "2", ""} {
		if csv[i][2] != want {
			t.Errorf("CSV row %d value %q, want %q", i, csv[i][2], want)
		}
	}
	if _, err := RenderBars(d, viz.BarsOptions{}, testNow); err == nil {
		t.Fatal("legacy negative refusal changed")
	}
}

func TestDivergingAdmissionAndFallback(t *testing.T) {
	for _, max := range []float64{-1, math.NaN(), math.Inf(1), MaxMagnitude + 1, 1.99} {
		h, err := RenderDivergingBars(signedFixture(), viz.BarsOptions{Max: max}, testNow)
		if err == nil || h != "" {
			t.Errorf("accepted invalid/clipping extent %v", max)
		}
	}
	d := signedFixture()
	d.Rows[0].Value = value(-2.004)
	if _, err := RenderDivergingBars(d, viz.BarsOptions{Max: 2}, testNow); err != nil {
		t.Fatal("extent must compare rounded values", err)
	}
	d.Rows[0].Value = value(-2.006)
	if _, err := RenderDivergingBars(d, viz.BarsOptions{Max: 2}, testNow); err == nil {
		t.Fatal("rounded clipping accepted")
	}
	d = signedFixture()
	d.Rows[0].Label = strings.Repeat("Wide界", 30)
	h, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, "Bar chart omitted:") || len(signedNodes(t, h, "data-analytics-diverging")) != 0 || !strings.Contains(h, d.Rows[0].Label) {
		t.Fatal("readable fallback lost", err)
	}
	d = signedFixture()
	d.Contract.Title = `Signed <script>alert(1)</script>`
	d.Rows[0].Label = `<img onerror=x>`
	h, err = RenderDivergingBars(d, viz.BarsOptions{W: 900, LabelW: 250}, testNow)
	if err != nil || strings.Contains(h, "<script>") || strings.Contains(h, "<img onerror") || !strings.Contains(h, "&lt;script&gt;") {
		t.Fatal("unsafe text", err)
	}
}

func TestDivergingZeroMissingAndOneSided(t *testing.T) {
	for _, values := range [][]float64{{0, 0}, {-2, -1}, {1, 2}} {
		d := fixture()
		for i, v := range values {
			d.Rows[i].Value = value(v)
		}
		h, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow)
		if err != nil || len(signedNodes(t, h, "data-analytics-zero-axis")) != 1 {
			t.Fatal("one-sided domain missing", err)
		}
		if values[0] == 0 && len(signedNodes(t, h, "data-analytics-zero")) != 2 {
			t.Fatal("zero marks missing")
		}
	}
	d := signedFixture()
	d.Rows = d.Rows[5:]
	h, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, "No measured values") || len(signedNodes(t, h, "data-analytics-diverging")) != 0 {
		t.Fatal("missing-only domain invented", err)
	}
}

func TestDivergingScaleAndIntrinsicSize(t *testing.T) {
	d := fixture()
	d.Rows = d.Rows[1:]
	d.Contract.Decimals = 0
	h, err := RenderDivergingBars(d, viz.BarsOptions{W: 640, Max: .001}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	svg := signedNodes(t, h, "data-analytics-diverging")
	if len(svg) != 1 || signedAttr(svg[0], "width") != "640" || signedAttr(svg[0], "height") == "" {
		t.Fatal("chart must retain intrinsic size for native scrolling")
	}
	if !strings.Contains(h, ">-0.001</text>") || !strings.Contains(h, ">0.001</text>") {
		t.Fatal("scale rounded away explicit extent")
	}
	h, err = RenderDivergingBars(d, viz.BarsOptions{W: 320, LabelW: 120, Max: MaxMagnitude}, testNow)
	if err != nil || !strings.Contains(h, "Bar chart omitted:") {
		t.Fatal("overlapping domain labels admitted", err)
	}
	d = fixture()
	d.Rows[0].Value, d.Rows[1].Value = value(-.01), value(MaxMagnitude)
	h, err = RenderDivergingBars(d, viz.BarsOptions{W: 2000, LabelW: 180}, testNow)
	if err != nil || !strings.Contains(h, "Bar chart omitted:") {
		t.Fatal("subpixel nonzero mark must not silently disappear", err)
	}
}
