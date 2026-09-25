package receipts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"golang.org/x/net/html"
)

func chartDocument(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader("<!doctype html><html><body>" + body + "</body></html>"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func chartAttr(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func chartHasClass(n *html.Node, class string) bool {
	for _, candidate := range strings.Fields(chartAttr(n, "class")) {
		if candidate == class {
			return true
		}
	}
	return false
}

func chartNodes(root *html.Node, class string) []*html.Node {
	var out []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if chartHasClass(n, class) {
			out = append(out, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return out
}

func chartText(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func chartNodeFor(t *testing.T, root *html.Node, class, product, series string) *html.Node {
	t.Helper()
	for _, node := range chartNodes(root, class) {
		if chartAttr(node, "data-product") == product && chartAttr(node, "data-series") == series {
			return node
		}
	}
	t.Fatalf("missing %s for %s/%s", class, product, series)
	return nil
}

func chartProjection(t *testing.T, input fixtureData, selected selection) projection {
	t.Helper()
	p, err := project(input, selected)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func chartURL(id string) string { return "/receipts/" + id + "?period=2024-09" }

func TestSignedComparisonLiteralGeometryAndKeyboardTargets(t *testing.T) {
	input, expected := loadCUE(t)
	p := chartProjection(t, input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	body, err := renderSignedComparison(p, chartURL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "-$1,400.00 USD") || !strings.Contains(body, "$0.00 USD") || !strings.Contains(body, "+$1,400.00 USD") {
		t.Fatalf("axis lost fixed USD extent labels: %s", body)
	}
	doc := chartDocument(t, body)
	products := chartNodes(doc, "receipts-chart-product")
	if len(products) != 3 || chartAttr(products[0], "data-product") != "atlas" || chartAttr(products[1], "data-product") != "beacon" || chartAttr(products[2], "data-product") != "cedar" {
		t.Fatalf("product order changed: %v", products)
	}
	want := []struct {
		product, series, cents, x, width string
	}{
		{"atlas", "current", "80000", "300", "171.428571"},
		{"atlas", "previous", "140000", "300", "300"},
		{"beacon", "current", "90000", "300", "192.857143"},
		{"beacon", "previous", "90000", "300", "192.857143"},
		{"cedar", "current", "-15000", "267.857143", "32.142857"},
		{"cedar", "previous", "0", "300", "0"},
	}
	if expected.September.Rows[0].Current != 80000 || expected.September.Rows[2].Current != -15000 {
		t.Fatal("literal CUE chart oracle changed")
	}
	marks := chartNodes(doc, "receipts-chart-bar")
	if len(marks) != len(want) {
		t.Fatalf("marks=%d, want %d", len(marks), len(want))
	}
	for i, item := range want {
		mark := marks[i]
		if chartAttr(mark, "data-product") != item.product || chartAttr(mark, "data-series") != item.series || chartAttr(mark, "data-cents") != item.cents ||
			chartAttr(mark, "data-extent") != "140000" || chartAttr(mark, "x") != item.x || chartAttr(mark, "width") != item.width || chartAttr(mark, "height") != "16" {
			t.Fatalf("mark %d geometry changed: attrs=%v", i, mark.Attr)
		}
		if item.series == "current" && (chartAttr(mark, "fill") != "var(--ak-color-accent)" || chartAttr(mark, "stroke") != "none") {
			t.Fatalf("current mark is not solid: %v", mark.Attr)
		}
		if item.series == "previous" && (chartAttr(mark, "fill") != "none" || chartAttr(mark, "stroke") != "var(--ak-color-ink-muted)" || chartAttr(mark, "stroke-width") != "2") {
			t.Fatalf("previous mark is not outlined: %v", mark.Attr)
		}
	}
	series := chartNodes(doc, "receipts-chart-series")
	if len(series) != 6 {
		t.Fatalf("series rows=%d", len(series))
	}
	for _, row := range series {
		svgs := chartNodes(row, "receipts-chart-svg")
		hits := chartNodes(row, "receipts-chart-hit")
		if len(svgs) != 1 || chartAttr(svgs[0], "viewBox") != "0 0 600 44" || chartAttr(svgs[0], "preserveAspectRatio") != "none" || chartAttr(svgs[0], "role") != "group" || chartAttr(svgs[0], "aria-label") == "" ||
			len(hits) != 1 || chartAttr(hits[0], "x") != "0" || chartAttr(hits[0], "y") != "0" || chartAttr(hits[0], "width") != "600" || chartAttr(hits[0], "height") != "44" {
			t.Fatalf("row lost responsive 44-high native target: %s", chartText(row))
		}
		links := make([]*html.Node, 0, 1)
		for child := svgs[0].FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "a" {
				links = append(links, child)
			}
		}
		if len(links) != 1 || chartAttr(links[0], "href") == "" || chartAttr(links[0], "aria-label") == "" {
			t.Fatal("SVG row lacks a native accessible detail link")
		}
	}
}

func TestSignedComparisonMissingZeroOneCentAndExtreme(t *testing.T) {
	input, _ := loadCUE(t)
	october := chartProjection(t, input, selection{Period: "2024-10", Channel: "all", Compare: "none"})
	body, err := renderSignedComparison(october, chartURL)
	if err != nil {
		t.Fatal(err)
	}
	doc := chartDocument(t, body)
	zero := chartNodeFor(t, doc, "receipts-chart-bar", "atlas", "current")
	if chartAttr(zero, "width") != "0" || chartAttr(zero, "x") != "300" || chartAttr(zero, "data-zero") != "true" || len(chartNodes(doc, "receipts-chart-zero")) != 1 {
		t.Fatalf("zero geometry was distorted: %v", zero.Attr)
	}
	for _, id := range []string{"beacon", "cedar"} {
		row := chartNodeFor(t, doc, "receipts-chart-series", id, "current")
		if len(chartNodes(row, "receipts-chart-bar")) != 0 || !strings.Contains(chartText(row), "Missing:") || !strings.Contains(chartText(row), "unavailable") {
			t.Fatalf("missing %s fabricated a mark or lost its reason", id)
		}
	}

	input, _ = loadCUE(t)
	september := snapshotByPeriod(&input, "2024-09")
	*sourceFactByID(september, "atlas").ReceiptsCents, *sourceFactByID(september, "atlas").RefundsCents = 1, 0
	*sourceFactByID(september, "cedar").ReceiptsCents, *sourceFactByID(september, "cedar").RefundsCents = 0, 1
	oneCent := chartProjection(t, input, selection{Period: "2024-09", Channel: "app", Compare: "none"})
	body, err = renderSignedComparison(oneCent, chartURL)
	if err != nil {
		t.Fatal(err)
	}
	doc = chartDocument(t, body)
	if mark := chartNodeFor(t, doc, "receipts-chart-bar", "atlas", "current"); chartAttr(mark, "data-cents") != "1" || chartAttr(mark, "width") != "3" || chartAttr(mark, "x") != "300" {
		t.Fatalf("positive cent geometry changed: %v", mark.Attr)
	}
	if mark := chartNodeFor(t, doc, "receipts-chart-bar", "cedar", "current"); chartAttr(mark, "data-cents") != "-1" || chartAttr(mark, "width") != "3" || chartAttr(mark, "x") != "297" {
		t.Fatalf("negative cent geometry changed: %v", mark.Attr)
	}

	input, _ = loadCUE(t)
	september = snapshotByPeriod(&input, "2024-09")
	*sourceFactByID(september, "atlas").ReceiptsCents, *sourceFactByID(september, "atlas").RefundsCents = maxAmountCents, 0
	extreme := chartProjection(t, input, selection{Period: "2024-09", Channel: "app", Compare: "none"})
	body, err = renderSignedComparison(extreme, chartURL)
	if err != nil || strings.Contains(body, "NaN") || strings.Contains(body, "Inf") {
		t.Fatalf("extreme chart is invalid: %v %s", err, body)
	}
	doc = chartDocument(t, body)
	if mark := chartNodeFor(t, doc, "receipts-chart-bar", "atlas", "current"); chartAttr(mark, "data-extent") != fmt.Sprint(maxAmountCents) || chartAttr(mark, "width") != "300" {
		t.Fatalf("extreme chart was clipped: %v", mark.Attr)
	}
}

func TestSignedComparisonAll72SelectionsDeterministicAndInvariant(t *testing.T) {
	input, _ := loadCUE(t)
	count := 0
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for _, channel := range []string{"all", "app", "mobile"} {
			for _, compare := range []string{"none", "previous"} {
				for _, stale := range []bool{false, true} {
					for _, detail := range []bool{false, true} {
						selected := selection{Period: period, Channel: channel, Compare: compare, Stale: stale}
						if detail {
							selected.DetailID = map[string]string{"all": "atlas", "app": "cedar", "mobile": "beacon"}[channel]
						}
						p := chartProjection(t, input, selected)
						before, _ := json.Marshal(p)
						first, err := renderSignedComparison(p, chartURL)
						if err != nil {
							t.Fatalf("render %+v: %v", selected, err)
						}
						second, err := renderSignedComparison(p, chartURL)
						after, _ := json.Marshal(p)
						if err != nil || first != second || !bytes.Equal(before, after) {
							t.Fatalf("render changed input or output for %+v: %v", selected, err)
						}
						doc := chartDocument(t, first)
						wantProducts := expectedEvidenceProducts(selected)
						products := chartNodes(doc, "receipts-chart-product")
						wantSeries := 1
						if compare == "previous" {
							wantSeries = 2
						}
						if len(products) != len(wantProducts) || len(chartNodes(doc, "receipts-chart-series")) != len(wantProducts)*wantSeries {
							t.Fatalf("selection %+v chart shape changed", selected)
						}
						for i, product := range wantProducts {
							if chartAttr(products[i], "data-product") != product || !strings.Contains(first, p.Current.Contract.Window.Start.Format("January 2006")) {
								t.Fatalf("selection %+v lost order or current period", selected)
							}
						}
						if p.Previous != nil && !strings.Contains(first, p.Previous.Contract.Window.Start.Format("January 2006")) {
							t.Fatalf("selection %+v lost previous period", selected)
						}
						count++
					}
				}
			}
		}
	}
	if count != 72 {
		t.Fatalf("render denominator=%d", count)
	}
}

func TestSignedComparisonEscapesLabelsAndRefusesUnsafeLinks(t *testing.T) {
	input, _ := loadCUE(t)
	input.Products[0].Label = `<script>alert("label")</script> & Atlas`
	p := chartProjection(t, input, selection{Period: "2024-09", Channel: "app", Compare: "none"})
	href := `/receipts/atlas?next=%22%3E%3Cscript%3E`
	body, err := renderSignedComparison(p, func(string) string { return href })
	if err != nil || strings.Contains(body, `<script>`) || !strings.Contains(body, `&lt;script&gt;`) {
		t.Fatalf("untrusted chart text was not escaped: %v %s", err, body)
	}
	doc := chartDocument(t, body)
	if links := chartNodes(doc, "receipts-chart-product-link"); len(links) != 2 || chartAttr(links[0], "href") != href {
		t.Fatalf("long product link was lost: %v", links)
	}
	missingInput, _ := loadCUE(t)
	missing := sourceFactByID(snapshotByPeriod(&missingInput, "2024-10"), "beacon")
	missing.MissingReason = `<script>alert("reason")</script> & unavailable`
	missingProjection := chartProjection(t, missingInput, selection{Period: "2024-10", Channel: "mobile", Compare: "none"})
	missingBody, err := renderSignedComparison(missingProjection, chartURL)
	if err != nil || strings.Contains(missingBody, `<script>`) || !strings.Contains(missingBody, `&lt;script&gt;`) {
		t.Fatalf("untrusted missing reason was not escaped: %v %s", err, missingBody)
	}
	for _, unsafe := range []string{"", "https://example.com/x", "//example.com/x", "/receipts/../x", "/receipts/x#fragment", `/receipts\x`, "/receipts/%2fsecret"} {
		if body, err := renderSignedComparison(p, func(string) string { return unsafe }); err == nil || body != "" {
			t.Fatalf("unsafe link %q returned body=%q err=%v", unsafe, body, err)
		}
	}
}

func TestSignedComparisonRejectsInvalidProjection(t *testing.T) {
	input, _ := loadCUE(t)
	base := chartProjection(t, input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	for name, change := range map[string]func(*projection){
		"contract":  func(p *projection) { p.Current.Contract.Unit = "EUR" },
		"row count": func(p *projection) { p.Current.Rows = p.Current.Rows[:2] },
		"row id":    func(p *projection) { p.Current.Rows[0].ID = "wrong" },
		"value drift": func(p *projection) {
			forged := *p.Current.Rows[0].Value + 0.01
			p.Current.Rows[0].Value = &forged
		},
		"missingness":    func(p *projection) { p.Facts[0].NetCents = nil },
		"missing reason": func(p *projection) { p.Facts[0].MissingReason = "forged reason" },
		"series order":   func(p *projection) { p.PreviousFacts[0], p.PreviousFacts[1] = p.PreviousFacts[1], p.PreviousFacts[0] },
		"paired id": func(p *projection) {
			p.PreviousFacts[0].Product.ID = "forged-atlas"
			p.Previous.Rows[0].ID = "forged-atlas"
		},
		"paired label": func(p *projection) {
			p.PreviousFacts[0].Product.Label = "Forged Atlas"
			p.Previous.Rows[0].Label = "Forged Atlas"
		},
		"previous absent": func(p *projection) { p.Previous = nil },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			changed.Current.Rows = append([]analytics.Row(nil), base.Current.Rows...)
			changed.Facts = append([]projectedFact(nil), base.Facts...)
			changed.PreviousFacts = append([]projectedFact(nil), base.PreviousFacts...)
			previous := *base.Previous
			previous.Rows = append([]analytics.Row(nil), base.Previous.Rows...)
			changed.Previous = &previous
			change(&changed)
			if body, err := renderSignedComparison(changed, chartURL); err == nil || body != "" {
				t.Fatalf("invalid projection returned body=%q err=%v", body, err)
			}
		})
	}
	if body, err := renderSignedComparison(base, nil); err == nil || body != "" {
		t.Fatalf("nil callback returned body=%q err=%v", body, err)
	}
	if reflect.DeepEqual(base, projection{}) {
		t.Fatal("invalid-projection canaries use an empty baseline")
	}
}
