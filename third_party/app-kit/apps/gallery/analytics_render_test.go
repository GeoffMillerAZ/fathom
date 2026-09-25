package gallery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestAnalyticsRenderLongLabelsEscapingAndStructure(t *testing.T) {
	j := analyticsTestJourney(t, "")
	label := strings.Repeat("Long international service 名称 ", 12) + "<img src=x onerror=alert(1)>"
	j.Current.Rows[0].Label = label
	j.Previous.Rows[0].Label = label
	j.Details[0].Label = label
	j.Details[0].Current.Label = label
	j.Details[0].Previous.Label = label
	content, err := buildAnalyticsContent(j, nil, "dark")
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	if strings.Contains(body, "<img") || !strings.Contains(body, "&lt;img") || !strings.Contains(body, "[overflow-wrap:anywhere]") {
		t.Fatal("long label lost escaping or wrap affordance")
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "script" || n.Data == "iframe" {
				t.Errorf("unexpected active element %s", n.Data)
			}
			for _, a := range n.Attr {
				if strings.HasPrefix(a.Key, "on") || a.Key == "style" {
					t.Errorf("unexpected inline execution/style: %s", a.Key)
				}
				if a.Key == "id" {
					if ids[a.Val] {
						t.Errorf("duplicate ID %s", a.Val)
					}
					ids[a.Val] = true
				}
				if a.Key == "href" && (strings.HasPrefix(a.Val, "javascript:") || strings.Contains(a.Val, "ZgotmplZ")) {
					t.Errorf("unsafe/broken destination %q", a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
}

func TestAnalyticsReferencePayloadBudgetAndNoBrowserRuntime(t *testing.T) {
	// Preselected budget for this bounded 3-service fixture: <=32 KiB HTML and
	// zero script elements. This is payload evidence, not browser latency proof.
	h := New()
	for _, route := range []string{
		"/analytics", "/analytics?state=missing", "/analytics?state=partial&theme=dark",
		"/analytics/gateway-api?state=stale", "/analytics?compare=none&category=worker",
	} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, route, nil))
		if r.Code != http.StatusOK {
			t.Fatalf("%s returned %d", route, r.Code)
		}
		if r.Body.Len() > 32<<10 {
			t.Errorf("%s exceeded 32 KiB HTML: %d", route, r.Body.Len())
		}
		if strings.Contains(strings.ToLower(r.Body.String()), "<script") {
			t.Errorf("%s acquired a browser runtime", route)
		}
		if strings.Count(r.Body.String(), "<main") != 1 {
			t.Errorf("%s does not have one main landmark", route)
		}
	}
}

func TestAnalyticsComparisonNeverTreatsMissingOrPartialAsZero(t *testing.T) {
	zero, one := float64(0), float64(1)
	if analyticsCount(nil, analyticsStateComplete) != "Missing" || analyticsCount(&zero, analyticsStateComplete) != "0" {
		t.Fatal("missing and zero collapsed")
	}
	if analyticsCount(&zero, analyticsStatePartial) != "At least 0" {
		t.Fatal("partial zero lost lower-bound status")
	}
	for _, state := range []analyticsState{analyticsStateMissing, analyticsStatePartial} {
		if analyticsChange(&zero, &one, state) != "Not comparable" {
			t.Fatal("incomplete source acquired a full-period change")
		}
	}
	if analyticsChange(nil, &one, analyticsStateComplete) != "Not comparable" {
		t.Fatal("missing source acquired a delta")
	}
}

func TestAnalyticsPartialStateDescriptionMatchesSelectedComparison(t *testing.T) {
	const prior = "Previous full-month values remain available in the table and export"
	const none = "No reference month is selected"

	for _, tc := range []struct {
		name     string
		query    string
		detail   bool
		want     string
		dontWant string
	}{
		{"overview without comparison", "state=partial&compare=none", false, none, prior},
		{"detail without comparison", "state=partial&compare=none", true, none, prior},
		{"overview with previous", "state=partial&compare=previous", false, prior, none},
		{"detail with previous", "state=partial&compare=previous", true, prior, none},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := analyticsTestJourney(t, tc.query)
			var detail *analyticsDetail
			if tc.detail {
				detail = &j.Details[0]
			}
			content, err := buildAnalyticsContent(j, detail, "dark")
			if err != nil {
				t.Fatal(err)
			}
			got := string(content)
			if !strings.Contains(got, tc.want) {
				t.Errorf("partial state description missing selected-reference copy %q\n%s", tc.want, got)
			}
			if strings.Contains(got, tc.dontWant) {
				t.Errorf("partial state description claimed unselected reference %q\n%s", tc.dontWant, got)
			}
			if !strings.Contains(got, "no full-month difference is calculated or plotted") {
				t.Error("partial state must retain the no-full-month-delta policy")
			}
		})
	}
}

func TestAnalyticsCapabilityLinkPreservesExplicitTheme(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		content, err := buildAnalyticsContent(analyticsTestJourney(t, ""), nil, theme)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), `href="/catalog/analytics.comparison?theme=`+theme+`"`) {
			t.Fatalf("capability link dropped explicit %s theme", theme)
		}
	}
}
