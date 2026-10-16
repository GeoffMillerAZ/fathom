package gallery

import (
	"fmt"
	"html/template"
	"math"
	"strconv"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

type analyticsRowView struct {
	ID, Label, Category, Href        string
	Current, Previous, Change, State string
	Chart                            template.HTML
}

type analyticsView struct {
	Journey                                                    analyticsJourney
	Rows                                                       []analyticsRowView
	Theme, Title, Summary, CurrentLabel, PreviousLabel         string
	Back, Export, ExportLabel, Reset, CompleteFixture, Catalog string
	Recipe                                                     string
	AllGroups, EmptyContextExport                              string
	SelectionLabel                                             string
	StateLabel, StateDescription, Observation, Evaluated       string
	Detail, PlotPrevious, HasPlottedValues                     bool
	Scale                                                      string
}

// buildAnalyticsContent is a Gallery reference, not a permanent workflow API.
// Datasets retain separate windows; missing values are never changed to zero.
func buildAnalyticsContent(j analyticsJourney, detail *analyticsDetail, theme string) (template.HTML, error) {
	if err := j.Current.Validate(j.Now); err != nil {
		return "", err
	}
	if j.Previous != nil {
		if err := j.Previous.Validate(j.Now); err != nil {
			return "", err
		}
	}
	v := analyticsView{
		Journey: j, Theme: theme, Title: "Service failure counts", Summary: j.Narrative,
		CurrentLabel: j.Current.Contract.Window.Start.Format("January 2006"),
		Back:         analyticsURL("/analytics", j.Selection, theme),
		Export:       analyticsURL("/analytics/export.csv", j.Selection, theme), ExportLabel: "Export CSV",
		Reset: previewURL("/analytics", theme), Evaluated: j.Now.Format("2 Jan 2006, 15:04 MST"),
		Catalog:      previewURL("/catalog/analytics.comparison", theme),
		PlotPrevious: len(j.Current.Rows) > 0 && j.Previous != nil && j.Selection.State != analyticsStatePartial && j.Selection.State != analyticsStateMissing,
	}
	if j.Previous != nil {
		v.PreviousLabel = j.Previous.Contract.Window.Start.Format("January 2006")
		v.ExportLabel = "Export comparison CSV"
	}
	v.SelectionLabel = analyticsCategoryLabel(j.Selection.Category) + " · No comparison"
	if j.Selection.Compare == analyticsComparePrevious {
		v.SelectionLabel = analyticsCategoryLabel(j.Selection.Category) + " · Previous calendar month"
	}
	complete := j.Selection
	recipeSelection := j.Selection
	recipeSelection.Compare = analyticsCompareNone
	v.Recipe = analyticsURL("/analytics/recipe", recipeSelection, theme)
	v.EmptyContextExport = analyticsURL(analyticsRecipeExportPath, recipeSelection, theme)
	allGroups := j.Selection
	allGroups.Category = analyticsCategoryAll
	v.AllGroups = analyticsURL("/analytics", allGroups, theme)
	if len(j.Current.Rows) == 0 {
		v.ExportLabel = "Export CSV (headers only)"
	}
	complete.State = analyticsStateComplete
	v.CompleteFixture = analyticsURL("/analytics", complete, theme)
	selected := j.Details
	if detail != nil {
		v.Detail, v.Title, v.Summary = true, detail.Label, detail.Explanation
		v.Export = analyticsURL("/analytics/"+detail.ID+"/export.csv", j.Selection, theme)
		v.ExportLabel = "Export service CSV"
		v.CompleteFixture = analyticsURL("/analytics/"+detail.ID, complete, theme)
		selected = []analyticsDetail{*detail}
	}
	switch j.Selection.State {
	case analyticsStateMissing:
		v.StateLabel = "Source reports missing"
		v.StateDescription = "Current counts are unknown. There is no current total, zero-value substitution or calculated change. Obtain the source reports before comparing."
	case analyticsStateStale:
		v.StateLabel = "Source observation is stale"
		v.StateDescription = "The recorded counts are still visible, but the source is older than its 24-hour freshness budget at the fixed fixture clock. Refresh the source before acting."
	case analyticsStatePartial:
		v.StateLabel = "Partial month: first 20 days only"
		v.StateDescription = "Present counts are lower bounds. Later days are unobserved."
		if j.Previous != nil {
			v.StateDescription += " Previous full-month values remain available in the table and export, but no full-month difference is calculated or plotted."
		} else {
			v.StateDescription += " No reference month is selected, and no full-month difference is calculated or plotted."
		}
	default:
		v.StateLabel = "Complete reporting window"
		v.StateDescription = "The fixture contains counts for the whole selected calendar month. A recorded zero means no failed operations in that reporting window."
	}
	if len(j.Current.Rows) == 0 {
		v.StateDescription = "This source condition is retained independently of the empty group. No selected observations are available to interpret. Showing all service groups keeps this condition."
	}
	if f := j.Current.Contract.Freshness; f.UnknownReason != "" {
		v.Observation = f.UnknownReason
	} else {
		v.Observation = "Source observed " + f.ObservedAt.Format("2 Jan 2006, 15:04 MST") + ". Freshness budget: 24 hours."
	}
	// A shared scale across every selected service also survives drill-down.
	max := float64(1)
	for _, d := range j.Details {
		for i, row := range []*analytics.Row{&d.Current, d.Previous} {
			if row != nil && row.Value != nil {
				if *row.Value < 0 || math.Trunc(*row.Value) != *row.Value {
					return "", fmt.Errorf("gallery analytics: fixture requires nonnegative integer counts")
				}
				if i == 0 || v.PlotPrevious {
					max = math.Max(max, *row.Value)
				}
			}
		}
	}
	v.Scale = strconv.FormatFloat(max, 'f', 0, 64)
	for _, d := range selected {
		row := analyticsRowView{
			ID: d.ID, Label: d.Label, Category: analyticsCategoryLabel(d.Category),
			Href:     analyticsURL("/analytics/"+d.ID, j.Selection, theme),
			Current:  analyticsCount(d.Current.Value, j.Selection.State),
			Previous: "Not selected", Change: "Not compared", State: "Present",
		}
		if d.Current.Value == nil {
			row.State = "Missing, not zero"
		} else if j.Selection.State == analyticsStatePartial {
			row.State = "Partial, lower bound"
		} else if j.Selection.State == analyticsStateStale {
			row.State = "Present, source stale"
		}
		if d.Previous != nil {
			row.Previous = analyticsCount(d.Previous.Value, analyticsStateComplete)
			row.Change = analyticsChange(d.Current.Value, d.Previous.Value, j.Selection.State)
		}
		var bars []viz.Bar
		if d.Current.Value != nil {
			label := j.Current.Contract.Window.Start.Format("Jan 2006")
			if j.Selection.State == analyticsStatePartial {
				label = "First 20 days"
			}
			bars = append(bars, viz.Bar{Label: label, Value: *d.Current.Value, Role: "accent"})
		}
		if v.PlotPrevious && d.Previous != nil && d.Previous.Value != nil {
			bars = append(bars, viz.Bar{Label: j.Previous.Contract.Window.Start.Format("Jan 2006"), Value: *d.Previous.Value, Role: "ink-muted"})
		}
		if len(bars) != 0 {
			// Trusted output comes only from the escaping first-party SVG renderer.
			row.Chart = template.HTML(viz.Bars(bars, viz.BarsOptions{W: 320, LabelW: 92, RowH: 26, Gap: 10,
				Max: max, ShowValue: true, AriaLabel: d.Label + ": failed operations; direct period labels; shared zero baseline"}))
			if row.Chart != "" {
				v.HasPlottedValues = true
			}
		}
		v.Rows = append(v.Rows, row)
	}
	return htmlf("analytics", analyticsHTML, v)
}

func analyticsCount(value *float64, state analyticsState) string {
	if value == nil {
		return "Missing"
	}
	count := strconv.FormatFloat(*value, 'f', 0, 64)
	if state == analyticsStatePartial {
		return "At least " + count
	}
	return count
}

func analyticsChange(current, previous *float64, state analyticsState) string {
	if current == nil || previous == nil || state == analyticsStatePartial || state == analyticsStateMissing {
		return "Not comparable"
	}
	delta := *current - *previous
	if delta == 0 {
		return "Unchanged"
	}
	if delta < 0 {
		return strconv.FormatFloat(-delta, 'f', 0, 64) + " fewer"
	}
	return strconv.FormatFloat(delta, 'f', 0, 64) + " more"
}

func buildAnalyticsError(status int, message, backURL string) (template.HTML, error) {
	return htmlf("analytics-error", analyticsErrorHTML, struct {
		Status        int
		Message, Back string
	}{status, message, backURL})
}
