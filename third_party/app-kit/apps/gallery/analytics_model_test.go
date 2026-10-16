package gallery

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func analyticsTestJourney(t *testing.T, query string) analyticsJourney {
	t.Helper()
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := parseAnalyticsSelection(values)
	if err != nil {
		t.Fatal(err)
	}
	journey, err := buildAnalyticsJourney(selection)
	if err != nil {
		t.Fatal(err)
	}
	return journey
}

func TestAnalyticsSelectionTypedDefaultsAndRoundTrip(t *testing.T) {
	selection, err := parseAnalyticsSelection(nil)
	want := analyticsSelection{analyticsPeriodSeptember, analyticsComparePrevious, analyticsCategoryAll, analyticsStateComplete}
	if err != nil || selection != want {
		t.Fatalf("defaults = %+v, %v", selection, err)
	}
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, compare := range []analyticsComparison{analyticsCompareNone, analyticsComparePrevious} {
			for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker} {
				for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
					want := analyticsSelection{period, compare, category, state}
					values := want.Values()
					values.Set("theme", "host-owned")
					got, err := parseAnalyticsSelection(values)
					if err != nil || got != want {
						t.Fatalf("selection round-trip = %+v, %v; want %+v", got, err, want)
					}
					values.Set("period", "changed")
					if want.Values().Get("period") != string(period) {
						t.Fatal("Values returned shared mutable state")
					}
				}
			}
		}
	}
}

func TestAnalyticsSelectionRejectsInvalidOwnedValues(t *testing.T) {
	for _, query := range []string{"period=2026-07", "period=2026-9", "period=2027-01", "compare=true", "category=API", "category=finance", "state=zero", "state=loading"} {
		values, _ := url.ParseQuery(query)
		if _, err := parseAnalyticsSelection(values); err == nil {
			t.Errorf("invalid selection accepted: %s", query)
		}
	}
	for _, field := range []string{"period", "compare", "category", "state"} {
		for _, values := range [][]string{nil, {""}, {"complete", "complete"}} {
			if _, err := parseAnalyticsSelection(url.Values{field: values}); err == nil || !strings.Contains(err.Error(), field) {
				t.Errorf("invalid %s values %q: %v", field, values, err)
			}
		}
	}
	for _, invalid := range []analyticsSelection{{}, {"2026-07", "previous", "all", "complete"}, {"2026-09", "previous", "all", "bad"}} {
		if _, err := buildAnalyticsJourney(invalid); err == nil {
			t.Errorf("typed invalid selection reached fixtures: %+v", invalid)
		}
	}
}

func assertAnalyticsValues(t *testing.T, rows []analytics.Row, want []float64) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, row := range rows {
		if row.Value == nil || *row.Value != want[i] || row.MissingReason != "" {
			t.Errorf("row %s did not retain measured value %v", row.ID, want[i])
		}
	}
}

func TestAnalyticsFixtureExactValuesWindowsAndNarrative(t *testing.T) {
	for _, test := range []struct {
		query                     string
		current, previous         []float64
		start, end, previousStart string
		narrative                 []string
	}{
		{"", []float64{4, 1, 0}, []float64{6, 3, 0}, "2026-09-01", "2026-10-01", "2026-08-01", []string{"total 5 failed operations", "totals 9", "4 lower", "not normalized"}},
		{"period=2026-08", []float64{6, 3, 0}, []float64{8, 5, 2}, "2026-08-01", "2026-09-01", "2026-07-01", []string{"total 9 failed operations", "totals 15", "6 lower"}},
		{"category=api", []float64{4, 1}, []float64{6, 3}, "2026-09-01", "2026-10-01", "2026-08-01", []string{"API services (2 services)", "total 5 failed operations"}},
		{"category=worker", []float64{0}, []float64{0}, "2026-09-01", "2026-10-01", "2026-08-01", []string{"worker services (1 service)", "total 0 failed operations", "unchanged"}},
	} {
		t.Run(test.query, func(t *testing.T) {
			j := analyticsTestJourney(t, test.query)
			if j.Previous == nil {
				t.Fatal("comparison omitted")
			}
			assertAnalyticsValues(t, j.Current.Rows, test.current)
			assertAnalyticsValues(t, j.Previous.Rows, test.previous)
			window := j.Current.Contract.Window
			if window.Kind != analytics.RangeWindow || window.TimeZone != "UTC" || window.Start.Format("2006-01-02") != test.start || window.End.Format("2006-01-02") != test.end || j.Previous.Contract.Window.Start.Format("2006-01-02") != test.previousStart || !j.Previous.Contract.Window.End.Equal(window.Start) {
				t.Fatalf("selection windows disagree: current=%+v previous=%+v", window, j.Previous.Contract.Window)
			}
			if j.Current.Contract.Freshness.ObservedAt.Before(window.End) || j.Now.Before(window.End) {
				t.Fatal("a complete period was observed before it ended")
			}
			for _, expected := range test.narrative {
				if !strings.Contains(j.Narrative, expected) {
					t.Errorf("narrative lost %q: %s", expected, j.Narrative)
				}
			}
		})
	}
	none := analyticsTestJourney(t, "compare=none")
	if none.Previous != nil || strings.Contains(none.Narrative, "previous calendar month") {
		t.Fatal("comparison-disabled selection retained a baseline")
	}
	for _, detail := range none.Details {
		if detail.Previous != nil {
			t.Fatal("comparison-disabled detail retained a baseline")
		}
	}
}

func TestAnalyticsTruthStatesDoNotInventCompleteTotals(t *testing.T) {
	missing := analyticsTestJourney(t, "state=missing&category=worker")
	if missing.Current.Contract.Completeness != analytics.Unknown || missing.Current.Contract.Freshness.UnknownReason == "" || missing.Current.Rows[0].Value != nil || missing.Current.Rows[0].MissingReason == "" {
		t.Fatal("missing report became a measured zero or fresh source")
	}
	if !strings.Contains(missing.Narrative, "No current total or change") || !strings.Contains(missing.Narrative, "missing, not zero") {
		t.Fatal("missing narrative invented a total/change")
	}
	for _, test := range []struct {
		query  string
		values []float64
		end    string
	}{
		{"state=partial", []float64{3, 1, 0}, "2026-09-21"}, {"state=partial&period=2026-08", []float64{4, 2, 0}, "2026-08-21"},
	} {
		partial := analyticsTestJourney(t, test.query)
		assertAnalyticsValues(t, partial.Current.Rows, test.values)
		if partial.Current.Contract.Completeness != analytics.Partial || !strings.Contains(partial.Current.Contract.CoverageNote, test.end) || !strings.Contains(partial.Current.Contract.CoverageNote, "lower bound") || !strings.Contains(partial.Narrative, "No complete-period change") {
			t.Fatal("partial selection hid coverage or invented a complete change")
		}
		if partial.Previous.Contract.Completeness != analytics.Complete {
			t.Fatal("current partial state rewrote the known comparison baseline")
		}
	}
	complete := analyticsTestJourney(t, "")
	stale := analyticsTestJourney(t, "state=stale")
	if !reflect.DeepEqual(stale.Current.Rows, complete.Current.Rows) || stale.Current.Contract.Completeness != analytics.Complete || stale.Now.Sub(stale.Current.Contract.Freshness.ObservedAt) <= stale.Current.Contract.Freshness.MaxAge || stale.Current.Contract.Freshness.ObservedAt.Before(stale.Current.Contract.Window.End) || !strings.Contains(stale.Narrative, "stale") {
		t.Fatal("stale fixture changed counts, mislabeled source age, or predated its complete period")
	}
}

func TestAnalyticsDetailsPreserveIdentitySelectionAndDeterminism(t *testing.T) {
	j := analyticsTestJourney(t, "category=api&state=partial")
	for index, id := range []string{"gateway-api", "search-api"} {
		detail, ok := j.Detail(id)
		if !ok || detail.ID != id || detail.Category != analyticsCategoryAPI || !reflect.DeepEqual(detail.Current, j.Current.Rows[index]) || detail.Previous == nil || !reflect.DeepEqual(*detail.Previous, j.Previous.Rows[index]) || !strings.Contains(detail.Explanation, "lower bound") {
			t.Fatalf("detail lost selected observation: %+v", detail)
		}
		link, err := url.Parse(detail.Href)
		if err != nil || link.Path != "/analytics/"+id {
			t.Fatalf("invalid stable detail link: %s", detail.Href)
		}
		selection, err := parseAnalyticsSelection(link.Query())
		if err != nil || selection != j.Selection {
			t.Fatal("detail link lost period/filter/comparison/state")
		}
	}
	for _, id := range []string{"report-worker", "missing", "../gateway-api"} {
		if _, ok := j.Detail(id); ok {
			t.Errorf("detail escaped selected IDs: %s", id)
		}
	}
	again := analyticsTestJourney(t, "category=api&state=partial")
	if !reflect.DeepEqual(j, again) {
		t.Fatal("fixture depends on time, randomness, or unstable ordering")
	}
	*j.Current.Rows[0].Value = 999
	j.Previous.Rows[0].Label = "mutated"
	if !reflect.DeepEqual(analyticsTestJourney(t, "category=api&state=partial"), again) {
		t.Fatal("mutating a result changed a later fixture selection")
	}
	if analyticsTestJourney(t, "period=2026-08&category=api").Details[0].ID != again.Details[0].ID {
		t.Fatal("service identity changed with the period")
	}
}

func TestAnalyticsSelectionsAgreeAcrossChartTableAndCSV(t *testing.T) {
	for _, period := range []string{"2026-08", "2026-09"} {
		for _, category := range []string{"all", "api", "worker"} {
			for _, state := range []string{"complete", "missing", "stale", "partial"} {
				t.Run(period+"/"+category+"/"+state, func(t *testing.T) {
					j := analyticsTestJourney(t, "period="+period+"&category="+category+"&state="+state)
					if len(j.Current.Rows)+len(j.Previous.Rows) > 6 {
						t.Fatal("fixture exceeded six comparison marks")
					}
					if !j.Now.Equal(time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)) {
						t.Fatal("implicit fixture clock")
					}
					for _, dataset := range []analytics.Dataset{j.Current, *j.Previous} {
						assertAnalyticsOutputAgreement(t, dataset, j.Now, category)
					}
				})
			}
		}
	}
}

func assertAnalyticsOutputAgreement(t *testing.T, dataset analytics.Dataset, now time.Time, category string) {
	t.Helper()
	html, err := analytics.RenderBars(dataset, viz.BarsOptions{}, now)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := analytics.WriteCSV(&output, dataset, now); err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(output.Bytes()))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	context, rows := map[string]string{}, [][]string(nil)
	for i, record := range records {
		if slices.Equal(record, []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}) {
			rows = records[i+1:]
			break
		}
		if len(record) == 2 {
			context[record[0]] = record[1]
		}
	}
	if len(rows) != len(dataset.Rows) || context["unit"] != "failed operations" || context["completeness"] != string(dataset.Contract.Completeness) || context["source"] != dataset.Contract.Source.Text || !strings.Contains(context["source"], "category="+category) || !strings.Contains(context["window"], dataset.Contract.Window.Start.Format(time.RFC3339)) || !strings.Contains(context["window"], dataset.Contract.Window.End.Format(time.RFC3339)) {
		t.Fatal("CSV lost selected rows or semantic context")
	}
	for i, row := range dataset.Rows {
		if rows[i][0] != row.ID || rows[i][1] != row.Label || !strings.Contains(html, row.Label) {
			t.Fatal("chart/table/export lost row identity")
		}
		if row.Value == nil {
			if rows[i][2] != "" || rows[i][4] != "missing" || rows[i][5] != row.MissingReason || !strings.Contains(html, ">Missing</td>") || strings.Contains(html, "<svg") {
				t.Fatal("missing source was plotted or exported as zero")
			}
		} else {
			value := fmt.Sprintf("%.0f", *row.Value)
			if rows[i][2] != value || rows[i][4] != "present" || !strings.Contains(html, ">"+value+" failed operations</text>") || !strings.Contains(html, `tabular-nums">`+value+"</td>") {
				t.Fatalf("chart/table/export disagree for %s", row.ID)
			}
		}
	}
}
