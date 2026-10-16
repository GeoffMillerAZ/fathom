package analytics

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func TestAnalyticsTextRejectsMalformedInputBeforeOutput(t *testing.T) {
	fields := []struct {
		name string
		set  func(*Dataset, string)
	}{
		{"title", func(d *Dataset, s string) { d.Contract.Title = s }},
		{"unit", func(d *Dataset, s string) { d.Contract.Unit = s }},
		{"grain", func(d *Dataset, s string) { d.Contract.Grain = s }},
		{"aggregation", func(d *Dataset, s string) { d.Contract.Aggregation = s }},
		{"uncertainty", func(d *Dataset, s string) { d.Contract.Uncertainty = s }},
		{"source text", func(d *Dataset, s string) { d.Contract.Source.Text = s }},
		{"source href", func(d *Dataset, s string) { d.Contract.Source.Href = "https://example.invalid/" + s }},
		{"coverage", func(d *Dataset, s string) { d.Contract.CoverageNote = s }},
		{"freshness reason", func(d *Dataset, s string) { d.Contract.Freshness = Freshness{UnknownReason: s} }},
		{"row ID", func(d *Dataset, s string) { d.Rows[0].ID = s }},
		{"row label", func(d *Dataset, s string) { d.Rows[0].Label = s }},
		{"missing reason", func(d *Dataset, s string) {
			d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One observation unavailable."
			d.Rows[0].Value, d.Rows[0].MissingReason = nil, s
		}},
		{"legend label", func(d *Dataset, s string) { d.Legend = []LegendItem{{Role: "accent", Label: s}} }},
	}
	for _, malformed := range []struct{ name, text string }{
		{"invalid UTF-8", "before\xffafter"},
		{"truncated UTF-8", "before\xe2\x82"},
		{"NUL", "before\x00after"},
	} {
		for _, field := range fields {
			t.Run(malformed.name+"/"+field.name, func(t *testing.T) {
				d := fixture()
				field.set(&d, malformed.text)
				if err := d.Validate(testNow); err == nil {
					t.Error("malformed text accepted by semantic validation")
				}
				if markup, err := RenderBars(d, viz.BarsOptions{}, testNow); err == nil || markup != "" {
					t.Error("malformed text must fail before producing HTML")
				}
				var out bytes.Buffer
				if err := WriteCSV(&out, d, testNow); err == nil || out.Len() != 0 {
					t.Error("malformed text must fail before writing any CSV bytes")
				}
			})
		}
	}
}

func TestAnalyticsTextPreservesValidUnicodeAndByteLimit(t *testing.T) {
	for _, label := range []string{
		"資料 café العربية 👩🏽‍💻",
		"literal replacement character \uFFFD",
		strings.Repeat("é", maxTextBytes/2),
	} {
		d := fixture()
		d.Rows[0].Label = label
		if err := d.Validate(testNow); err != nil {
			t.Fatal(err)
		}
		h, err := RenderBars(d, viz.BarsOptions{}, testNow)
		if err != nil || !strings.Contains(h, label) {
			t.Fatal("valid Unicode must remain available in the rendered table")
		}
		rows, _ := dataCSV(t, d)
		if rows[0][1] != label || rows[0][2] != "1250.5" || rows[1][2] != "0" {
			t.Fatal("text validation changed labels, numeric values or measured zero")
		}
	}
	d := fixture()
	d.Rows[0].Label = strings.Repeat("é", maxTextBytes/2) + "a"
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err == nil || out.Len() != 0 {
		t.Fatal("existing byte ceiling must still refuse atomically")
	}
}
