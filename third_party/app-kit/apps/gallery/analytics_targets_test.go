package gallery

import (
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestAnalyticsDisclosureSummariesKeepNative44PixelTargets(t *testing.T) {
	selection := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAll, analyticsStateComplete}
	for _, test := range []struct {
		name, path string
		labels     []string
	}{
		{name: "overview", path: "/analytics", labels: []string{"Change comparison all services · Previous calendar month", "Test a data condition", "Source, time window and interpretation"}},
		{name: "detail", path: "/analytics/gateway-api", labels: []string{"Source, time window and interpretation"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := analyticsRequest(analyticsRouteHandler(), http.MethodGet, analyticsURL(test.path, selection, ""))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			doc, err := html.Parse(strings.NewReader(response.Body.String()))
			if err != nil {
				t.Fatal(err)
			}
			main := analyticsQualityID(doc, "gallery-main")
			if main == nil {
				t.Fatal("analytics main landmark is missing")
			}
			// The outer Gallery header has its own native navigation disclosure.
			// This contract concerns every disclosure inside the analytics task.
			summaries := analyticsTargetNodes(main, "summary")
			if len(summaries) != len(test.labels) {
				t.Fatalf("native summary count = %d, want %d", len(summaries), len(test.labels))
			}
			for i, summary := range summaries {
				if summary.Parent == nil || summary.Parent.Type != html.ElementNode || summary.Parent.Data != "details" {
					t.Fatalf("%q is not the native child of details", test.labels[i])
				}
				if got := analyticsNodeText(summary); got != test.labels[i] {
					t.Errorf("summary label = %q, want %q", got, test.labels[i])
				}
				assertAnalyticsTargetClasses(t, summary, "min-h-11", "py-3")
			}
		})
	}
}

func TestAnalyticsOverviewTableDrilldownsOwn44PixelTargets(t *testing.T) {
	selection := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAll, analyticsStateComplete}
	journey, err := buildAnalyticsJourney(selection)
	if err != nil {
		t.Fatal(err)
	}
	response := analyticsRequest(analyticsRouteHandler(), http.MethodGet, analyticsURL("/analytics", selection, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	assertAnalyticsRenderedValues(t, response.Body.String(), journey, journey.Details)

	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	table := analyticsQualityID(doc, "analytics-value-table")
	if table == nil || table.Data != "table" {
		t.Fatal("exact-values table lost its native table element")
	}
	for _, detail := range journey.Details {
		row := analyticsQualityNode(table, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "tr" && analyticsTargetAttr(n, "data-value-id") == detail.ID
		})
		if row == nil {
			t.Fatalf("missing exact-data row %q", detail.ID)
		}
		header := analyticsQualityNode(row, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "th" && analyticsTargetAttr(n, "scope") == "row"
		})
		if header == nil {
			t.Fatalf("%q lost its native row header", detail.ID)
		}
		assertAnalyticsTargetClasses(t, header, "py-0")
		link := analyticsQualityNode(header, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "a"
		})
		if link == nil {
			t.Fatalf("%q row header lost its drill-down anchor", detail.ID)
		}
		if got := analyticsNodeText(link); got != detail.Label {
			t.Errorf("%q link label = %q, want %q", detail.ID, got, detail.Label)
		}
		if got, want := analyticsTargetAttr(link, "href"), analyticsURL("/analytics/"+detail.ID, selection, ""); got != want {
			t.Errorf("%q href = %q, want %q", detail.ID, got, want)
		}
		assertAnalyticsTargetClasses(t, link, "inline-flex", "min-h-11", "items-center")
	}
}

func analyticsTargetNodes(root *html.Node, tag string) []*html.Node {
	var nodes []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == tag {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return nodes
}

func analyticsTargetAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func assertAnalyticsTargetClasses(t *testing.T, node *html.Node, required ...string) {
	t.Helper()
	classes := strings.Fields(analyticsTargetAttr(node, "class"))
	for _, want := range required {
		found := false
		for _, class := range classes {
			found = found || class == want
		}
		if !found {
			t.Errorf("<%s> class %q missing %q", node.Data, strings.Join(classes, " "), want)
		}
	}
}
