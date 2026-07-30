package metrics

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func spendWindow() (time.Time, time.Time) {
	return time.Unix(0, 0).UTC(), time.Unix(1<<32, 0).UTC()
}

func approxEqual(a, b float64) bool {
	diff := a - b
	return diff > -0.0001 && diff < 0.0001
}

func TestAggregateSpend_GroupByLayer_CostPerClaimAndBudget(t *testing.T) {
	since, until := spendWindow()
	runs := []core.CompileRun{
		{Layer: "obligations", CostUSD: 0.10, TokensIn: 100, TokensOut: 50, ClaimsEmitted: 5, RanAt: time.Unix(100, 0).UTC()},
		{Layer: "obligations", CostUSD: 0.20, TokensIn: 200, TokensOut: 100, ClaimsEmitted: 5, RanAt: time.Unix(200, 0).UTC()},
	}
	lookup := func(layer string) (float64, bool) {
		if layer == "obligations" {
			return 1.00, true
		}
		return 0, false
	}
	report := AggregateSpend(runs, lookup, SpendOptions{Since: since, Until: until, GroupBy: SpendGroupByLayer})
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %+v, want 1", report.Rows)
	}
	row := report.Rows[0]
	if row.Group != "obligations" || row.Runs != 2 || row.ClaimsEmitted != 10 {
		t.Fatalf("row = %+v, want group=obligations runs=2 claims=10", row)
	}
	if row.CostPerClaimUSD == nil || !approxEqual(*row.CostPerClaimUSD, 0.03) {
		t.Fatalf("CostPerClaimUSD = %v, want ~0.3/10=0.03", row.CostPerClaimUSD)
	}
	if row.BudgetMaxUSD == nil || !approxEqual(*row.BudgetMaxUSD, 1.00) {
		t.Fatalf("BudgetMaxUSD = %v, want 1.00", row.BudgetMaxUSD)
	}
	if row.BudgetHeadroomUSD == nil || !approxEqual(*row.BudgetHeadroomUSD, 0.80) {
		t.Fatalf("BudgetHeadroomUSD = %v, want ~1.00-0.20=0.80 (largest single run)", row.BudgetHeadroomUSD)
	}
	if report.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2", report.TotalRuns)
	}
	if diff := report.TotalCostUSD - 0.30; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("TotalCostUSD = %f, want ~0.30", report.TotalCostUSD)
	}
}

func TestAggregateSpend_GroupByDay(t *testing.T) {
	since, until := spendWindow()
	runs := []core.CompileRun{
		{Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)},
		{Layer: "findings", CostUSD: 0.20, ClaimsEmitted: 1, RanAt: time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)},
		{Layer: "obligations", CostUSD: 0.05, ClaimsEmitted: 1, RanAt: time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)},
	}
	report := AggregateSpend(runs, nil, SpendOptions{Since: since, Until: until, GroupBy: SpendGroupByDay})
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %+v, want 2 days", report.Rows)
	}
	if report.Rows[0].Group != "2026-07-30" || report.Rows[0].Runs != 2 {
		t.Errorf("row0 = %+v", report.Rows[0])
	}
	if report.Rows[1].Group != "2026-07-31" || report.Rows[1].Runs != 1 {
		t.Errorf("row1 = %+v", report.Rows[1])
	}
}

func TestAggregateSpend_GroupByModel_BudgetNotApplicable(t *testing.T) {
	since, until := spendWindow()
	runs := []core.CompileRun{
		{Layer: "obligations", Model: "haiku", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Unix(1, 0).UTC()},
	}
	report := AggregateSpend(runs, nil, SpendOptions{Since: since, Until: until, GroupBy: SpendGroupByModel})
	if len(report.Rows) != 1 || report.Rows[0].Group != "haiku" {
		t.Fatalf("rows = %+v", report.Rows)
	}
	if report.Rows[0].BudgetMaxUSD != nil {
		t.Errorf("BudgetMaxUSD = %v, want nil under --by model", report.Rows[0].BudgetMaxUSD)
	}
	if report.Rows[0].BudgetNote == "" {
		t.Error("BudgetNote is empty, want an explicit not-applicable note")
	}
}

func TestAggregateSpend_ZeroClaimsEmitted_CostPerClaimHonestNote(t *testing.T) {
	since, until := spendWindow()
	runs := []core.CompileRun{
		{Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 0, RanAt: time.Unix(1, 0).UTC()},
	}
	report := AggregateSpend(runs, nil, SpendOptions{Since: since, Until: until})
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %+v", report.Rows)
	}
	row := report.Rows[0]
	if row.CostPerClaimUSD != nil {
		t.Fatalf("CostPerClaimUSD = %v, want nil (zero claims)", *row.CostPerClaimUSD)
	}
	if row.CostPerClaimNote == "" {
		t.Error("CostPerClaimNote is empty, want an honest zero-claims note")
	}
}

func TestAggregateSpend_EscapeHatchRunsFlagged(t *testing.T) {
	since, until := spendWindow()
	runs := []core.CompileRun{
		{Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Unix(1, 0).UTC(), Escape: "force: idempotent recompile"},
		{Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Unix(2, 0).UTC()},
	}
	report := AggregateSpend(runs, nil, SpendOptions{Since: since, Until: until})
	if len(report.Rows) != 1 || report.Rows[0].EscapeHatchRuns != 1 {
		t.Fatalf("rows = %+v, want 1 escape-hatch run", report.Rows)
	}
}

func TestAggregateSpend_EmptyInput_EmptyReport(t *testing.T) {
	since, until := spendWindow()
	report := AggregateSpend(nil, nil, SpendOptions{Since: since, Until: until})
	if len(report.Rows) != 0 || report.TotalRuns != 0 {
		t.Fatalf("report = %+v, want empty", report)
	}
	if report.Note == "" {
		t.Error("Note is empty, want an honest empty-window note")
	}
}

// TestAggregateSpend_WindowFiltersOutOfRangeRuns.
func TestAggregateSpend_WindowFiltersOutOfRangeRuns(t *testing.T) {
	since := time.Unix(100, 0).UTC()
	until := time.Unix(200, 0).UTC()
	runs := []core.CompileRun{
		{Layer: "obligations", CostUSD: 0.10, ClaimsEmitted: 1, RanAt: time.Unix(50, 0).UTC()},  // before window
		{Layer: "obligations", CostUSD: 0.20, ClaimsEmitted: 1, RanAt: time.Unix(150, 0).UTC()}, // in window
		{Layer: "obligations", CostUSD: 0.30, ClaimsEmitted: 1, RanAt: time.Unix(250, 0).UTC()}, // after window
	}
	report := AggregateSpend(runs, nil, SpendOptions{Since: since, Until: until})
	if report.TotalRuns != 1 || report.TotalCostUSD != 0.20 {
		t.Fatalf("report = %+v, want only the in-window run", report)
	}
}
