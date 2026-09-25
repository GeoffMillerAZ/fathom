package gallery

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func selectionNodeAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func selectionFind(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := selectionFind(child, match); found != nil {
			return found
		}
	}
	return nil
}

func selectionByID(n *html.Node, id string) *html.Node {
	return selectionFind(n, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && selectionNodeAttr(candidate, "id") == id
	})
}

func selectionHasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func selectionAncestor(n, parent *html.Node) bool {
	for node := n.Parent; node != nil; node = node.Parent {
		if node == parent {
			return true
		}
	}
	return false
}

func selectionSourceDisclosure(doc *html.Node) *html.Node {
	summary := selectionFind(doc, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && candidate.Data == "summary" &&
			strings.Contains(analyticsNodeText(candidate), "Source, time window and interpretation")
	})
	if summary == nil || summary.Parent == nil || summary.Parent.Data != "details" {
		return nil
	}
	return summary.Parent
}

func selectionAssertNativeOptions(t *testing.T, form *html.Node, field, selected string, choices []string) {
	t.Helper()
	selectNode := selectionFind(form, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && candidate.Data == "select" &&
			selectionNodeAttr(candidate, "name") == field
	})
	if selectNode == nil {
		t.Fatalf("missing native %s select", field)
	}
	var options, selectedOptions []string
	for option := selectNode.FirstChild; option != nil; option = option.NextSibling {
		if option.Type != html.ElementNode || option.Data != "option" {
			continue
		}
		value := selectionNodeAttr(option, "value")
		options = append(options, value)
		if selectionHasAttr(option, "selected") {
			selectedOptions = append(selectedOptions, value)
		}
	}
	if strings.Join(options, ",") != strings.Join(choices, ",") ||
		len(selectedOptions) != 1 || selectedOptions[0] != selected {
		t.Fatalf("%s options=%q selected=%q, want %q / %q", field, options, selectedOptions, choices, selected)
	}
}

func TestAnalyticsSelectionDisclosureAcrossFortyEightSelections(t *testing.T) {
	handler := analyticsRouteHandler()
	periods := []string{"2026-09", "2026-08"}
	compares := []string{"previous", "none"}
	categories := []string{"all", "api", "worker"}
	// Preserve the original 48 nonempty cases while exposing the separate
	// no-results fixture exercised by TestAnalyticsEmptyCategoryOverviewAndExports.
	categoryOptions := []string{"all", "api", "worker", "empty"}
	states := []string{"complete", "missing", "stale", "partial"}
	stateOptions := []string{"complete", "missing", "stale", "partial", "error"}
	categoryLabel := map[string]string{"all": "all services", "api": "API services", "worker": "worker services"}
	compareLabel := map[string]string{"none": "No comparison", "previous": "Previous calendar month"}
	seen := 0
	for _, period := range periods {
		for _, compare := range compares {
			for _, category := range categories {
				for _, state := range states {
					seen++
					name := fmt.Sprintf("%s/%s/%s/%s", period, compare, category, state)
					t.Run(name, func(t *testing.T) {
						theme := "light"
						if seen%2 == 0 {
							theme = "dark"
						}
						query := url.Values{"period": {period}, "compare": {compare},
							"category": {category}, "state": {state}, "theme": {theme}}
						response := analyticsRequest(handler, http.MethodGet, "/analytics?"+query.Encode())
						if response.Code != http.StatusOK {
							t.Fatalf("overview status %d", response.Code)
						}
						doc, err := html.Parse(strings.NewReader(response.Body.String()))
						if err != nil {
							t.Fatal(err)
						}
						outer := selectionByID(doc, "analytics-selection")
						if outer == nil || outer.Data != "details" || selectionHasAttr(outer, "open") {
							t.Fatal("comparison must start as a closed native details control")
						}
						summary := selectionFind(outer, func(n *html.Node) bool {
							return n.Type == html.ElementNode && n.Data == "summary" && n.Parent == outer
						})
						if summary == nil {
							t.Fatal("comparison has no native summary")
						}
						label := analyticsNodeText(summary)
						for _, want := range []string{"Change comparison", categoryLabel[category], compareLabel[compare]} {
							if !strings.Contains(label, want) {
								t.Errorf("summary %q lacks independently expected %q", label, want)
							}
						}
						form := selectionByID(doc, "analytics-filter")
						if form == nil || form.Data != "form" || !selectionAncestor(form, outer) ||
							selectionNodeAttr(form, "action") != "/analytics" || selectionNodeAttr(form, "method") != "get" {
							t.Fatal("closed comparison lost native GET filter form")
						}
						selectionAssertNativeOptions(t, form, "period", period, periods)
						selectionAssertNativeOptions(t, form, "compare", compare, compares)
						selectionAssertNativeOptions(t, form, "category", category, categoryOptions)
						selectionAssertNativeOptions(t, form, "state", state, stateOptions)
						hiddenTheme := selectionFind(form, func(n *html.Node) bool {
							return n.Type == html.ElementNode && n.Data == "input" &&
								selectionNodeAttr(n, "type") == "hidden" && selectionNodeAttr(n, "name") == "theme"
						})
						if hiddenTheme == nil || selectionNodeAttr(hiddenTheme, "value") != theme {
							t.Fatalf("filter lost selected theme %q", theme)
						}
						inner := selectionFind(form, func(n *html.Node) bool {
							return n.Type == html.ElementNode && n.Data == "details"
						})
						if inner == nil || selectionHasAttr(inner, "open") != (state != "complete") ||
							!strings.Contains(analyticsNodeText(inner), "Test a data condition") {
							t.Fatal("inner truth-state control changed its native disclosure behavior")
						}
						reading := selectionByID(doc, "analytics-reading")
						if reading == nil || reading.Parent == nil || reading.Parent.Data != "section" ||
							selectionAncestor(reading, outer) {
							t.Fatal("report explanation became hidden by comparison disclosure")
						}
						for ancestor := reading.Parent.Parent; ancestor != nil; ancestor = ancestor.Parent {
							if ancestor.Type == html.ElementNode && ancestor.Data == "details" &&
								!selectionHasAttr(ancestor, "open") {
								t.Fatal("report explanation is inside another closed disclosure")
							}
						}
						if state != "complete" && !strings.Contains(analyticsNodeText(reading.Parent),
							map[string]string{"missing": "missing", "stale": "stale", "partial": "lower bound"}[state]) {
							t.Fatal("truth warning disappeared from visible report explanation")
						}
						if source := selectionSourceDisclosure(doc); source == nil || selectionHasAttr(source, "open") {
							t.Fatal("overview source disclosure changed")
						}
					})
				}
			}
		}
	}
	if seen != 48 {
		t.Fatalf("tested %d selections, want 48", seen)
	}
}

func TestAnalyticsDetailHasNoSelectionDisclosure(t *testing.T) {
	handler := analyticsRouteHandler()
	query := url.Values{"period": {"2026-08"}, "compare": {"none"},
		"category": {"api"}, "state": {"partial"}, "theme": {"dark"}}
	response := analyticsRequest(handler, http.MethodGet, "/analytics/gateway-api?"+query.Encode())
	if response.Code != http.StatusOK {
		t.Fatalf("Gateway detail status %d", response.Code)
	}
	doc, err := html.Parse(strings.NewReader(response.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	if selectionByID(doc, "analytics-selection") != nil || selectionByID(doc, "analytics-filter") != nil {
		t.Fatal("Gateway detail gained overview-only filter disclosure")
	}
	if source := selectionSourceDisclosure(doc); source == nil || !selectionHasAttr(source, "open") {
		t.Fatal("detail source disclosure lost its existing open state")
	}
}
