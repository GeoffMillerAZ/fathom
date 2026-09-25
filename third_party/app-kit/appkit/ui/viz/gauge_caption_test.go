package viz

import (
	"strings"
	"testing"
)

func TestGauge_LongCaptionWrapsAndReservesHeight(t *testing.T) {
	const label = "of this service's recorded calls arrived over the agent surface"
	got := Gauge(7, 10, GaugeOptions{W: 160, Label: label})

	if strings.Contains(got, ">"+label+"<") {
		t.Fatalf("long caption must not be emitted as one clip-prone SVG text run: %s", got)
	}
	for _, want := range []string{
		`<tspan x="80" y="98">of this</tspan>`,
		`<tspan x="80" dy="14">service&#39;s</tspan>`,
		`<tspan x="80" dy="14">calls arrived</tspan>`,
		`<tspan x="80" dy="14">agent surface</tspan>`,
		`<desc>of this service&#39;s recorded calls arrived over the agent surface</desc>`,
		`height="188"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wrapped caption missing %q\n%s", want, got)
		}
	}
}

func TestGauge_CaptionWrapsUnbreakableUnicode(t *testing.T) {
	got := Gauge(1, 2, GaugeOptions{W: 160, Label: "界界界界界界界界界界界界界界界界界界界"})

	for _, want := range []string{"界界界界界界界界界界界界界", "界界界界界界"} {
		if !strings.Contains(got, ">"+want+"<") {
			t.Errorf("caption must retain Unicode characters in a bounded line: missing %q\n%s", want, got)
		}
	}
}

func TestGauge_CaptionWrapsWhitespaceSeparatedWords(t *testing.T) {
	got := Gauge(1, 2, GaugeOptions{W: 160, Label: "  alpha\tbeta\n gamma   delta epsilon  "})

	for _, want := range []string{"alpha beta", "gamma delta", "epsilon"} {
		if !strings.Contains(got, ">"+want+"<") {
			t.Errorf("caption must retain whitespace-separated words: missing %q\n%s", want, got)
		}
	}
	if !strings.Contains(got, `<desc>alpha beta gamma delta epsilon</desc>`) {
		t.Errorf("wrapped caption must retain normalized text as an SVG description\n%s", got)
	}
}

func TestGauge_CaptionUsesWidthAwareConservativeGraphemeBudget(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		label     string
		firstLine string
		lastLine  string
	}{
		{"narrow-wide-ascii", 80, "WWWWWWW", "WWWWWW", "W"},
		{"default-fullwidth", 160, "界界界界界界界界界界界界界界", "界界界界界界界界界界界界界", "界"},
		{"wide-fullwidth", 320, "界界界界界界界界界界界界界界界界界界界界界界界界界界界", "界界界界界界界界界界界界界界界界界界界界界界界界界界", "界"},
		{"emoji", 160, "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂", "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂", "🙂"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Gauge(1, 2, GaugeOptions{W: tc.width, Label: tc.label})
			for _, want := range []string{">" + tc.firstLine + "<", ">" + tc.lastLine + "<", "<desc>" + tc.label + "</desc>"} {
				if !strings.Contains(got, want) {
					t.Errorf("width %d caption missing %q\n%s", tc.width, want, got)
				}
			}
		})
	}
}

func TestGauge_WrappedCaptionEscapesDescriptionAndText(t *testing.T) {
	got := Gauge(1, 2, GaugeOptions{W: 160, Label: "<script> alpha beta gamma delta"})

	if strings.Contains(got, "<script>") {
		t.Fatalf("wrapped caption emitted unescaped markup: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("wrapped caption must escape markup in both visual text and description: %s", got)
	}
}

func TestGauge_CaptionNeverSplitsCombiningOrZWJGraphemes(t *testing.T) {
	cases := []struct {
		name    string
		cluster string
	}{
		{"combining", "e\u0301"},
		{"zwj-emoji", "👩\u200d💻"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label := strings.Repeat(tc.cluster, 14)
			got := Gauge(1, 2, GaugeOptions{W: 160, Label: label})
			for _, want := range []string{
				">" + strings.Repeat(tc.cluster, 13) + "<",
				">" + tc.cluster + "<",
				"<desc>" + label + "</desc>",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("caption split grapheme cluster, missing %q\n%s", want, got)
				}
			}
		})
	}
}
