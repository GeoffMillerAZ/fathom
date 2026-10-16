package gallery

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

type analyticsPeriod string
type analyticsComparison string
type analyticsCategory string
type analyticsState string

const (
	analyticsPeriodAugust    analyticsPeriod     = "2026-08"
	analyticsPeriodSeptember analyticsPeriod     = "2026-09"
	analyticsCompareNone     analyticsComparison = "none"
	analyticsComparePrevious analyticsComparison = "previous"
	analyticsCategoryAll     analyticsCategory   = "all"
	analyticsCategoryAPI     analyticsCategory   = "api"
	analyticsCategoryWorker  analyticsCategory   = "worker"
	analyticsCategoryEmpty   analyticsCategory   = "empty"
	analyticsStateComplete   analyticsState      = "complete"
	analyticsStateMissing    analyticsState      = "missing"
	analyticsStateStale      analyticsState      = "stale"
	analyticsStatePartial    analyticsState      = "partial"
	analyticsStateError      analyticsState      = "error"
)

type analyticsSelection struct {
	Period   analyticsPeriod
	Compare  analyticsComparison
	Category analyticsCategory
	State    analyticsState
}

// parseAnalyticsSelection owns only these four query fields. Host-owned fields
// such as theme are ignored; duplicate, empty, or unsupported owned values fail.
func parseAnalyticsSelection(q url.Values) (analyticsSelection, error) {
	field := func(name, fallback string, allowed ...string) (string, error) {
		values, exists := q[name]
		if !exists {
			return fallback, nil
		}
		if len(values) != 1 {
			return "", fmt.Errorf("analytics: %s must have exactly one value", name)
		}
		for _, value := range allowed {
			if values[0] == value {
				return value, nil
			}
		}
		return "", fmt.Errorf("analytics: invalid %s %q; want %s", name, values[0], strings.Join(allowed, ", "))
	}
	period, err := field("period", "2026-09", "2026-08", "2026-09")
	if err != nil {
		return analyticsSelection{}, err
	}
	compare, err := field("compare", "previous", "none", "previous")
	if err != nil {
		return analyticsSelection{}, err
	}
	category, err := field("category", "all", "all", "api", "worker", "empty")
	if err != nil {
		return analyticsSelection{}, err
	}
	state, err := field("state", "complete", "complete", "missing", "stale", "partial", "error")
	if err != nil {
		return analyticsSelection{}, err
	}
	return analyticsSelection{analyticsPeriod(period), analyticsComparison(compare), analyticsCategory(category), analyticsState(state)}, nil
}

// Values returns a fresh canonical selection for detail, back, and export URLs.
func (s analyticsSelection) Values() url.Values {
	return url.Values{"period": {string(s.Period)}, "compare": {string(s.Compare)}, "category": {string(s.Category)}, "state": {string(s.State)}}
}

// analyticsJourney is one selection shared by chart/table/CSV consumers. Each
// dataset owns its real window; previous-period counts are never folded into a
// misleading current-period total. Now is a fixed fixture evaluation instant.
type analyticsJourney struct {
	Selection analyticsSelection
	Current   analytics.Dataset
	Previous  *analytics.Dataset
	Now       time.Time
	Narrative string
	Details   []analyticsDetail
}

type analyticsDetail struct {
	ID, Label, Href, Explanation string
	Category                     analyticsCategory
	Current                      analytics.Row
	Previous                     *analytics.Row
}

// Detail only resolves IDs retained by this selection, preserving the filter.
func (j analyticsJourney) Detail(id string) (analyticsDetail, bool) {
	for _, detail := range j.Details {
		if detail.ID == id {
			return detail, true
		}
	}
	return analyticsDetail{}, false
}

type analyticsServiceFixture struct {
	id, label, explanation string
	category               analyticsCategory
	// July is the known comparison baseline for the selectable August period.
	monthly, firstTwentyDays [3]float64
}

// The fixture is local to each build: callers cannot mutate a later selection.
func analyticsServices() []analyticsServiceFixture {
	return []analyticsServiceFixture{
		{"gateway-api", "Gateway API", "Failed incoming request attempts at the gateway; retried attempts count separately.", analyticsCategoryAPI, [3]float64{8, 6, 4}, [3]float64{5, 4, 3}},
		{"search-api", "Search API", "Failed search request attempts; retried attempts count separately.", analyticsCategoryAPI, [3]float64{5, 3, 1}, [3]float64{3, 2, 1}},
		{"report-worker", "Report worker", "Failed report-generation job attempts; retried attempts count separately.", analyticsCategoryWorker, [3]float64{2, 0, 0}, [3]float64{1, 0, 0}},
	}
}

func analyticsCategoryLabel(category analyticsCategory) string {
	switch category {
	case analyticsCategoryAPI:
		return "API services"
	case analyticsCategoryWorker:
		return "worker services"
	case analyticsCategoryEmpty:
		return "empty fixture group"
	default:
		return "all services"
	}
}

func analyticsFixtureDataset(s analyticsSelection, start time.Time, state analyticsState) analytics.Dataset {
	period := start.Format("2006-01")
	category := analyticsCategoryLabel(s.Category)
	d := analytics.Dataset{Contract: analytics.Contract{
		ID:    "service-failures-" + period + "-" + string(s.Category) + "-" + string(state),
		Title: "Failed service operations — " + start.Format("January 2006") + " · " + category,
		Unit:  "failed operations", Grain: "one service in the " + category + " selection",
		Aggregation:  "non-negative integer count of failed API request attempts or worker job attempts in the selected calendar month; retries count separately",
		Window:       analytics.Window{Kind: analytics.RangeWindow, Start: start, End: start.AddDate(0, 1, 0), TimeZone: "UTC"},
		Source:       vizframe.Derived("Synthetic Gallery operating ledger v1; period=" + period + "; category=" + string(s.Category) + "; state=" + string(state) + "; comparison=" + string(s.Compare) + ". Not live telemetry."),
		Freshness:    analytics.Freshness{ObservedAt: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC), MaxAge: 24 * time.Hour},
		Completeness: analytics.Complete,
		Uncertainty:  "Exact synthetic fixture counts, not production measurements. No statistical interval is estimated; counts are not normalized for traffic or month length.",
	}}
	switch state {
	case analyticsStateMissing:
		d.Contract.Completeness = analytics.Unknown
		d.Contract.CoverageNote = "No current-period source report is available for any selected service; missing counts are not zero."
		d.Contract.Freshness = analytics.Freshness{UnknownReason: "The missing fixture has no current-period source observation."}
	case analyticsStateStale:
		// Even this stale observation follows September's exclusive end. Source
		// age is measured at the fixed October fixture clock, never wall time.
		d.Contract.Freshness.ObservedAt = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	case analyticsStatePartial:
		d.Contract.Completeness = analytics.Partial
		d.Contract.CoverageNote = fmt.Sprintf("Only %s to %s (end exclusive; UTC), the first 20 days, is observed. Later days are unavailable; every present count is a lower bound, not a full-month total.", start.Format("2006-01-02"), start.AddDate(0, 0, 20).Format("2006-01-02"))
		d.Contract.Aggregation = "non-negative integer count of failed API request attempts or worker job attempts during the observed first 20 days of the selected month; retries count separately"
		d.Contract.Uncertainty += " Partial counts may increase; the number of failures on unobserved days is unknown."
	}
	for _, service := range analyticsServices() {
		if s.Category != analyticsCategoryAll && s.Category != service.category {
			continue
		}
		row := analytics.Row{ID: service.id, Label: service.label, Role: "accent"}
		if service.category == analyticsCategoryWorker {
			row.Role = "info"
		}
		if state == analyticsStateMissing {
			row.MissingReason = "No source report for " + service.label + " in " + period + "; missing is not zero."
		} else {
			value := service.monthly[int(start.Month())-int(time.July)]
			if state == analyticsStatePartial {
				value = service.firstTwentyDays[int(start.Month())-int(time.July)]
			}
			row.Value = &value
		}
		d.Rows = append(d.Rows, row)
	}
	// These are declared service groups, not inferred reliability ratings. The
	// paired-period view has its own encoding; the public renderer uses this map.
	if len(d.Rows) == 0 {
		return d
	}
	if s.Category != analyticsCategoryWorker {
		d.Legend = append(d.Legend, analytics.LegendItem{Role: "accent", Label: "API services"})
	}
	if s.Category != analyticsCategoryAPI {
		d.Legend = append(d.Legend, analytics.LegendItem{Role: "info", Label: "Worker services"})
	}
	return d
}

func buildAnalyticsJourney(s analyticsSelection) (analyticsJourney, error) {
	if _, err := parseAnalyticsSelection(s.Values()); err != nil {
		return analyticsJourney{}, err
	}
	if s.State == analyticsStateError {
		return analyticsJourney{}, fmt.Errorf("analytics: synthetic data source could not be read")
	}
	start, err := time.Parse("2006-01", string(s.Period))
	if err != nil {
		return analyticsJourney{}, err
	}
	j := analyticsJourney{
		Selection: s, Current: analyticsFixtureDataset(s, start, s.State),
		Now: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
	}
	if s.Compare == analyticsComparePrevious {
		previous := analyticsFixtureDataset(s, start.AddDate(0, -1, 0), analyticsStateComplete)
		j.Previous = &previous
	}
	if err := j.Current.Validate(j.Now); err != nil {
		return analyticsJourney{}, err
	}
	if j.Previous != nil {
		if err := j.Previous.Validate(j.Now); err != nil {
			return analyticsJourney{}, err
		}
	}
	services := analyticsServices()
	for i, row := range j.Current.Rows {
		var service analyticsServiceFixture
		for _, candidate := range services {
			if candidate.id == row.ID {
				service = candidate
				break
			}
		}
		detail := analyticsDetail{
			ID: row.ID, Label: row.Label, Category: service.category, Current: row,
			Href:        "/analytics/" + row.ID + "?" + s.Values().Encode(),
			Explanation: "Synthetic fixture. " + service.explanation + " " + analyticsObservation(row, s.State, start),
		}
		if j.Previous != nil {
			previous := j.Previous.Rows[i]
			detail.Previous = &previous
			detail.Explanation += " " + analyticsObservation(previous, analyticsStateComplete, j.Previous.Contract.Window.Start)
		}
		detail.Explanation += " These counts do not establish a reliability rate or the cause of a change."
		j.Details = append(j.Details, detail)
	}
	j.Narrative = analyticsNarrative(j)
	return j, nil
}

func analyticsObservation(row analytics.Row, state analyticsState, start time.Time) string {
	period := start.Format("January 2006")
	if row.Value == nil {
		return "For " + period + ", the count is missing, not zero."
	}
	if state == analyticsStatePartial {
		return fmt.Sprintf("For %s, the first-20-day failed-operation count is %.0f; this is a lower bound for the full month.", period, *row.Value)
	}
	text := fmt.Sprintf("For %s, the full-month failed-operation count is %.0f.", period, *row.Value)
	if state == analyticsStateStale {
		text += " Its source observation is stale at the fixture evaluation time."
	}
	return text
}

func analyticsNarrative(j analyticsJourney) string {
	period := j.Current.Contract.Window.Start.Format("January 2006")
	serviceWord := "services"
	if len(j.Current.Rows) == 1 {
		serviceWord = "service"
	}
	intro := fmt.Sprintf("Synthetic fixture for %s, %s (%d %s). ", period, analyticsCategoryLabel(j.Selection.Category), len(j.Current.Rows), serviceWord)
	if len(j.Current.Rows) == 0 {
		return intro + "No matching services. This group contains no fixture services, not zero failed operations. Choose another group to inspect counts. Source availability is a separate condition."
	}
	if j.Selection.State == analyticsStateMissing {
		return intro + "All selected counts are missing, not zero. No current total or change can be calculated. Obtain the missing source reports before making a comparison."
	}
	var total float64
	for _, row := range j.Current.Rows {
		total += *row.Value
	}
	if j.Selection.State == analyticsStatePartial {
		return intro + fmt.Sprintf("The first 20 days contain %.0f observed failed operations; this is only a lower bound for the full month. No complete-period change is reported because later days are unavailable.", total)
	}
	text := intro + fmt.Sprintf("The selected full-month counts total %.0f failed operations.", total)
	if j.Previous != nil {
		var previous float64
		for _, row := range j.Previous.Rows {
			previous += *row.Value
		}
		text += fmt.Sprintf(" The previous calendar month, %s, totals %.0f for the same services", j.Previous.Contract.Window.Start.Format("January 2006"), previous)
		switch {
		case total < previous:
			text += fmt.Sprintf("; the selected count is %.0f lower.", previous-total)
		case total > previous:
			text += fmt.Sprintf("; the selected count is %.0f higher.", total-previous)
		default:
			text += "; the selected count is unchanged."
		}
	}
	text += " Counts are not normalized for traffic or month length and do not establish a reliability rate or cause."
	if j.Selection.State == analyticsStateStale {
		text += " Source data is stale at the fixed fixture evaluation time; refresh the source before acting."
	}
	return text
}
