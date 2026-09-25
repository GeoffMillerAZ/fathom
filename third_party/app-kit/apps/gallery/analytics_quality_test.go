package gallery

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/vizcheck"
	"golang.org/x/net/html"
)

// This oracle is authored independently of the renderer and model helpers.
// The fixture values are pinned here, not extracted from the generated markup.
type analyticsQualityService struct {
	id, label string
	category  analyticsCategory
	monthly   [3]int // July, August, September.
	partial   [2]int // August, September: the observed first twenty days.
}

func analyticsQualityServices(s analyticsSelection, detailID string) []analyticsQualityService {
	var selected []analyticsQualityService
	for _, row := range []analyticsQualityService{
		{"gateway-api", "Gateway API", analyticsCategoryAPI, [3]int{8, 6, 4}, [2]int{4, 3}},
		{"search-api", "Search API", analyticsCategoryAPI, [3]int{5, 3, 1}, [2]int{2, 1}},
		{"report-worker", "Report worker", analyticsCategoryWorker, [3]int{2, 0, 0}, [2]int{0, 0}},
	} {
		if (s.Category == analyticsCategoryAll || s.Category == row.category) && (detailID == "" || detailID == row.id) {
			selected = append(selected, row)
		}
	}
	return selected
}

func analyticsQualitySpec(s analyticsSelection, detailID string) vizcheck.Spec {
	month := 1
	if s.Period == analyticsPeriodSeptember {
		month = 2
	}
	months := [3]string{"July 2026", "August 2026", "September 2026"}
	shortMonths := [3]string{"Jul 2026", "Aug 2026", "Sep 2026"}
	comparison := s.Compare == analyticsComparePrevious
	caption := "Failed operations by service, " + months[month]
	header := []string{"Service", months[month]}
	if comparison {
		caption += " and " + months[month-1]
		header = append(header, months[month-1], "Count difference")
	}
	header = append(header, "Current state")
	spec := vizcheck.Spec{
		RootID: "gallery-main",
		Charts: []vizcheck.Chart{}, // Explicitly zero for a missing/table-only state.
		Table:  vizcheck.Table{ID: "analytics-value-table", Caption: caption, Rows: [][]string{header}},
		Source: vizcheck.Text{ID: "analytics-source", Value: fmt.Sprintf("Synthetic Gallery operating ledger v1; period=%s; category=%s; state=%s; comparison=%s. Not live telemetry.", s.Period, s.Category, s.State, s.Compare)},
	}
	for _, row := range analyticsQualityServices(s, detailID) {
		current, previous := row.monthly[month], row.monthly[month-1]
		if s.State == analyticsStatePartial {
			current = row.partial[month-1]
		}
		currentText, stateText := strconv.Itoa(current), "Present"
		switch s.State {
		case analyticsStateMissing:
			currentText, stateText = "Missing", "Missing, not zero"
		case analyticsStatePartial:
			currentText, stateText = "At least "+currentText, "Partial, lower bound"
		case analyticsStateStale:
			stateText = "Present, source stale"
		}
		cells := []string{row.label, currentText}
		if comparison {
			change := "Unchanged"
			switch {
			case s.State == analyticsStateMissing || s.State == analyticsStatePartial:
				change = "Not comparable"
			case current < previous:
				change = strconv.Itoa(previous-current) + " fewer"
			case current > previous:
				change = strconv.Itoa(current-previous) + " more"
			}
			cells = append(cells, strconv.Itoa(previous), change)
		}
		spec.Table.Rows = append(spec.Table.Rows, append(cells, stateText))
		if s.State == analyticsStateMissing {
			continue
		}
		period := shortMonths[month]
		if s.State == analyticsStatePartial {
			period = "First 20 days"
		}
		labels := []string{period, strconv.Itoa(current)}
		if comparison && s.State != analyticsStatePartial {
			labels = append(labels, shortMonths[month-1], strconv.Itoa(previous))
		}
		spec.Charts = append(spec.Charts, vizcheck.Chart{
			ID: "analytics-chart-" + row.id, Name: row.label + ": failed operations; direct period labels; shared zero baseline",
			Labels: labels,
		})
	}
	return spec
}

func TestAnalyticsQualityAll48SelectionsAndDetails(t *testing.T) {
	handler := analyticsRouteHandler()
	overviews, details, tableOnly := 0, 0, 0
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, compare := range []analyticsComparison{analyticsCompareNone, analyticsComparePrevious} {
			for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker} {
				for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
					s := analyticsSelection{period, compare, category, state}
					selectedIDs := []string{""}
					for _, service := range analyticsQualityServices(s, "") {
						selectedIDs = append(selectedIDs, service.id)
					}
					for _, detailID := range selectedIDs {
						if detailID == "" {
							overviews++
						} else {
							details++
						}
						if state == analyticsStateMissing {
							tableOnly++
						}
						t.Run(s.Values().Encode()+"/"+detailID, func(t *testing.T) {
							route := "/analytics"
							if detailID != "" {
								route += "/" + detailID
							}
							// Themes vary without claiming CSS visibility/contrast proof.
							theme := "light"
							if period == analyticsPeriodSeptember {
								theme = "dark"
							}
							response := analyticsRequest(handler, http.MethodGet, analyticsURL(route, s, theme))
							if response.Code != http.StatusOK {
								t.Fatalf("selected page returned %d", response.Code)
							}
							spec := analyticsQualitySpec(s, detailID)
							report, err := vizcheck.CheckHTML(response.Body.Bytes(), spec)
							if err != nil || len(report.Findings) != 0 {
								t.Fatalf("static composition mismatch: %v, %+v", err, report.Findings)
							}
							if report.Charts != len(spec.Charts) || report.Tables != 1 || report.Decorations != 0 {
								t.Fatalf("incomplete composition coverage: %+v", report)
							}
							if state == analyticsStateMissing && (spec.Charts == nil || len(spec.Charts) != 0 || report.Charts != 0) {
								t.Fatal("missing must explicitly declare no charts, not plot zeroes")
							}
						})
					}
				}
			}
		}
	}
	if overviews != 48 || details != 96 || tableOnly != 36 {
		t.Fatalf("selection denominator changed: overview=%d detail=%d table-only=%d", overviews, details, tableOnly)
	}
}

func analyticsQualityNode(root *html.Node, match func(*html.Node) bool) *html.Node {
	if match(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := analyticsQualityNode(child, match); found != nil {
			return found
		}
	}
	return nil
}

func analyticsQualityID(root *html.Node, id string) *html.Node {
	return analyticsQualityNode(root, func(n *html.Node) bool {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return true
			}
		}
		return false
	})
}

func TestAnalyticsQualitySourceDisclosureRemainsNative(t *testing.T) {
	s := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAll, analyticsStateComplete}
	for _, detailID := range []string{"", "gateway-api"} {
		route := "/analytics"
		if detailID != "" {
			route += "/" + detailID
		}
		response := analyticsRequest(analyticsRouteHandler(), http.MethodGet, analyticsURL(route, s, ""))
		doc, err := html.Parse(strings.NewReader(response.Body.String()))
		if err != nil {
			t.Fatal(err)
		}
		source := analyticsQualityID(doc, "analytics-source")
		if source == nil {
			t.Fatal("missing source disclosure")
		}
		for source != nil && source.Data != "details" {
			source = source.Parent
		}
		if source == nil {
			t.Fatal("source lost its native details disclosure")
		}
		open := false
		for _, attr := range source.Attr {
			open = open || attr.Key == "open"
		}
		if open != (detailID != "") {
			t.Fatal("audit integration changed the existing disclosure state")
		}
		// Closed details is intentionally accepted: static content presence is
		// not computed visibility, keyboard usability or proof of source truth.
		report, err := vizcheck.CheckHTML(response.Body.Bytes(), analyticsQualitySpec(s, detailID))
		if err != nil || len(report.Findings) != 0 {
			t.Fatalf("native disclosure is not statically checkable: %v, %+v", err, report.Findings)
		}
	}
}

func TestAnalyticsQualityRejectsPlantedCompositionDefects(t *testing.T) {
	s := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAll, analyticsStateComplete}
	response := analyticsRequest(analyticsRouteHandler(), http.MethodGet, analyticsURL("/analytics", s, ""))
	if response.Code != http.StatusOK {
		t.Fatal("valid defect fixture did not render")
	}
	spec := analyticsQualitySpec(s, "")
	for _, defect := range []struct{ name, wantRule string }{
		{"drop chart", "chart-svg-count"}, {"extra chart", "svg-coverage"},
		{"wrong chart value", "chart-labels"}, {"wrong table value", "table-cells"},
		{"wrong source", "source-text"}, {"move source outside selection", "element-scope"},
	} {
		t.Run(defect.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(response.Body.String()))
			if err != nil {
				t.Fatal(err)
			}
			root, chart := analyticsQualityID(doc, spec.RootID), analyticsQualityID(doc, spec.Charts[0].ID)
			source, table := analyticsQualityID(doc, spec.Source.ID), analyticsQualityID(doc, spec.Table.ID)
			if root == nil || chart == nil || source == nil || table == nil {
				t.Fatal("defect fixture lost an expected audit target")
			}
			svg := analyticsQualityNode(chart, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "svg" })
			if svg == nil {
				t.Fatal("defect fixture lacks its chart")
			}
			switch defect.name {
			case "drop chart":
				svg.Parent.RemoveChild(svg)
			case "extra chart":
				root.AppendChild(&html.Node{Type: html.ElementNode, Data: "svg", Namespace: "svg"})
			case "wrong chart value":
				label := analyticsQualityNode(svg, func(n *html.Node) bool {
					return n.Type == html.ElementNode && n.Data == "text" && analyticsNodeText(n) == "4"
				})
				if label == nil || label.FirstChild == nil {
					t.Fatal("numeric chart canary target missing")
				}
				label.FirstChild.Data = "999"
			case "wrong table value":
				cell := analyticsQualityNode(table, func(n *html.Node) bool {
					return n.Type == html.ElementNode && n.Data == "td" && analyticsNodeText(n) == "4"
				})
				if cell == nil || cell.FirstChild == nil {
					t.Fatal("numeric table canary target missing")
				}
				cell.FirstChild.Data = "999"
			case "wrong source":
				source.FirstChild.Data = "Different fixture; not the selected source."
			case "move source outside selection":
				source.Parent.RemoveChild(source)
				root.Parent.AppendChild(source)
			}
			var mutated bytes.Buffer
			if err := html.Render(&mutated, doc); err != nil {
				t.Fatal(err)
			}
			report, err := vizcheck.CheckHTML(mutated.Bytes(), spec)
			if err != nil {
				t.Fatalf("mutation should be a checkable mismatch, not invalid input: %v", err)
			}
			for _, finding := range report.Findings {
				if finding.Rule == defect.wantRule {
					return
				}
			}
			t.Fatalf("planted defect escaped %s: %+v", defect.wantRule, report)
		})
	}
}
