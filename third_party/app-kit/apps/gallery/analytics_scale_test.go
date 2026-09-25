package gallery

import (
	"strings"
	"testing"
)

func TestAnalyticsMissingMeasurementsHaveNoNumericPlotScale(t *testing.T) {
	for _, period := range []string{"2026-08", "2026-09"} {
		for _, compare := range []string{"none", "previous"} {
			for _, category := range []string{"all", "api", "worker"} {
				query := "period=" + period + "&compare=" + compare + "&category=" + category + "&state=missing"
				j := analyticsTestJourney(t, query)
				t.Run(query, func(t *testing.T) {
					assertMissing := func(detail *analyticsDetail) {
						t.Helper()
						content, err := buildAnalyticsContent(j, detail, "dark")
						if err != nil {
							t.Fatal(err)
						}
						body := string(content)
						if strings.Contains(body, "<svg") || strings.Contains(body, "shared scale 0 to") {
							t.Error("missing measurements must have neither a plot nor a numeric plotting scale")
						}
						if !strings.Contains(body, "Missing is not zero") {
							t.Error("missing explanation lost")
						}
					}
					assertMissing(nil)
					for i := range j.Details {
						assertMissing(&j.Details[i])
					}
				})
			}
		}
	}
}

func TestAnalyticsMeasuredZeroKeepsItsPlotAndScale(t *testing.T) {
	j := analyticsTestJourney(t, "period=2026-09&compare=none&category=worker&state=complete")
	if len(j.Details) != 1 || j.Details[0].Current.Value == nil || *j.Details[0].Current.Value != 0 {
		t.Fatal("test requires the known measured-zero worker fixture")
	}
	for _, detail := range []*analyticsDetail{nil, &j.Details[0]} {
		content, err := buildAnalyticsContent(j, detail, "light")
		if err != nil {
			t.Fatal(err)
		}
		body := string(content)
		if !strings.Contains(body, "<svg") || !strings.Contains(body, "shared scale 0 to 1") || strings.Contains(body, "No current measurement") {
			t.Error("measured zero must retain its real value, plot and bounded display scale")
		}
	}
}
