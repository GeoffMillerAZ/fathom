package analytics

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func TestValidateDetailHref(t *testing.T) {
	tests := []struct {
		name string
		href string
		ok   bool
	}{
		{"empty", "", true},
		{"origin-root", "/", true},
		{"root", "/receipts/sept?tab=cost#detail", true},
		{"encoded-space", "/receipts/September%202026", true},
		{"encoded-query-space", "/receipts?q=September%202026#row%201", true},
		{"unicode", "/receipts/über", true},
		{"encoded-path-markers", "/receipts/a%3Fb%23c", true},
		{"relative", "receipts/sept", false},
		{"protocol-relative", "//example.test/sept", false},
		{"scheme", "https://example.test/sept", false},
		{"fragment-only", "#sept", false},
		{"space", "/receipts/sept tab", false},
		{"backslash", "/receipts\\sept", false},
		{"bad-escape", "/receipts/%zz", false},
		{"bad-query-escape", "/receipts?q=%zz", false},
		{"bad-fragment-escape", "/receipts#%zz", false},
		{"decoded-control", "/receipts/%00", false},
		{"decoded-backslash", "/receipts/%5csept", false},
		{"decoded-protocol-relative", "/%2f%2fexample.test", false},
		{"oversized", strings.Repeat("/x", 1025), false},
		{"nul", "/x\x00", false},
		{"invalid-utf8", string([]byte{'/', 'x', 0xff}), false},
		{"query-control", "/x?bad=%00", false},
		{"query-backslash", "/x?bad=%5c", false},
		{"fragment-control", "/x#bad%0a", false},
		{"del", "/x%7f", false},
		{"literal-tab", "/x\t", false},
		{"literal-c1", "/x\u0085", false},
		{"encoded-c1", "/x?q=%C2%85", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateDetailHref(tt.href) == nil; got != tt.ok {
				t.Fatalf("validateDetailHref(%q) ok=%v, want %v", tt.href, got, tt.ok)
			}
			if !tt.ok {
				assertDetailHrefRefused(t, tt.href, false)
				assertDetailHrefRefused(t, tt.href, true)
			}
		})
	}
}

func TestDetailHrefPreservesAnalyticsAgreement(t *testing.T) {
	d := fixture()
	d.Rows[0].Label = `<September> & "2026"`
	d.Rows[0].DetailHref = `/receipts/sept?tab=cost&label="month"`
	d.Rows[1].DetailHref = "/receipts/oct"
	if err := d.Validate(testNow); err != nil {
		t.Fatal(err)
	}
	bar, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bar, `href="/receipts/sept?tab=cost&amp;label=&#34;month&#34;"`) || !strings.Contains(bar, "&lt;September&gt; &amp; &#34;2026&#34;") {
		t.Fatalf("accepted detail link or label missing from table: %s", bar)
	}

	unlinked := d
	unlinked.Rows = append([]Row(nil), d.Rows...)
	for i := range unlinked.Rows {
		unlinked.Rows[i].DetailHref = ""
	}
	before, err := detailCSVBytes(unlinked)
	if err != nil {
		t.Fatal(err)
	}
	after, err := detailCSVBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("accepted detail links changed CSV output")
	}
	d.Rows = append(d.Rows, Row{ID: "novalue", Label: "No link", Value: value(3)})
	plain, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(plain, "href=") != 2 || !strings.Contains(plain, "No link") {
		t.Fatalf("expected two links and one plain row: %s", plain)
	}

	d.Rows[0].Value = value(-12.5)
	diverging, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diverging, `href="/receipts/sept?tab=cost&amp;label=&#34;month&#34;"`) {
		t.Fatal("accepted detail link missing from diverging table")
	}

	d.Rows[0].Value = value(12.5)
	d.Rows[1].Value = nil
	d.Rows[1].MissingReason = "not observed"
	d.Contract.Completeness = Partial
	d.Contract.CoverageNote = "October is not observed."
	fallback, err := RenderBars(d, viz.BarsOptions{RowH: 12}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback, `href="/receipts/oct"`) || !strings.Contains(fallback, "not observed") {
		t.Fatal("missing-row detail link or state missing from fallback table")
	}
	d.Rows[0].Value = value(0)
	fallback, err = RenderDivergingBars(d, viz.BarsOptions{RowH: 12}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback, `href="/receipts/sept?tab=cost&amp;label=&#34;month&#34;"`) || !strings.Contains(fallback, `href="/receipts/oct"`) || !strings.Contains(fallback, `tabular-nums">0</td>`) || strings.Contains(fallback, "<svg") {
		t.Fatal("signed table fallback lost linked measured-zero/missing rows or emitted a false chart")
	}
}

func assertDetailHrefRefused(t *testing.T, href string, missing bool) {
	t.Helper()
	d := fixture()
	d.Rows[0].DetailHref = href
	if missing {
		d.Rows[0].Value = nil
		d.Rows[0].MissingReason = "not observed"
		d.Contract.Completeness = Partial
		d.Contract.CoverageNote = "September is not observed."
	}
	if err := d.Validate(testNow); err == nil {
		t.Fatal("Dataset.Validate accepted invalid detail href")
	}
	if body, err := RenderBars(d, viz.BarsOptions{}, testNow); err == nil || body != "" {
		t.Fatalf("RenderBars body=%q err=%v", body, err)
	}
	if body, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow); err == nil || body != "" {
		t.Fatalf("RenderDivergingBars body=%q err=%v", body, err)
	}
	body, err := detailCSVBytes(d)
	if err == nil || len(body) != 0 {
		t.Fatalf("WriteCSV bytes=%d err=%v", len(body), err)
	}
}

func detailCSVBytes(d Dataset) ([]byte, error) {
	var out bytes.Buffer
	err := WriteCSV(&out, d, testNow)
	return out.Bytes(), err
}
