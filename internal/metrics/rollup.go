package metrics

// rollup.go — Q-13 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §4): daily rollups so `fathom metrics series`
// doesn't rescan a growing JSONL over 30/90 days. One flat, named-field
// summary per date at workspace/artifacts/metrics/daily/YYYY-MM-DD.json —
// deterministic, tiny, additive-ACROSS-DATES (never deleted implicitly;
// see cmd/fathom/metrics_rollup.go's cmdMetricsRotate for the SEPARATE,
// explicit raw-log retention verb this does not duplicate).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DailyRollup is one date's flattened aggregate summary — the fields
// `fathom metrics series <metric>` (cmd/fathom) looks up by name.
type DailyRollup struct {
	Date        string `json:"date"` // YYYY-MM-DD, UTC
	GeneratedAt string `json:"generated_at"`

	SpendTotalUSD        float64  `json:"spend_total_usd"`
	SpendCostPerClaimUSD *float64 `json:"spend_cost_per_claim_usd"`

	TrafficTotalCalls int      `json:"traffic_total_calls"`
	TrafficErrorRate  *float64 `json:"traffic_error_rate"`

	ChannelsInjectionCoverageRate *float64 `json:"channels_injection_coverage_rate"`
	ChannelsObligationsServed     int      `json:"channels_obligations_served"`

	InterventionsUnprovenDoneRefusals *int `json:"interventions_unproven_done_refusals"`
	InterventionsReviewSuperseded     *int `json:"interventions_review_superseded"`
}

// BuildDailyRollup flattens one date's four aggregate reports into a
// DailyRollup — pure, no I/O, no clock beyond the injected now.
func BuildDailyRollup(date time.Time, spend SpendReport, traffic TrafficReport, channels ChannelsReport, interventions InterventionsReport, now time.Time) DailyRollup {
	r := DailyRollup{
		Date:                          date.UTC().Format("2006-01-02"),
		GeneratedAt:                   now.UTC().Format(time.RFC3339),
		SpendTotalUSD:                 spend.TotalCostUSD,
		TrafficTotalCalls:             traffic.TotalCalls,
		ChannelsInjectionCoverageRate: channels.InjectionCoverageRate,
		ChannelsObligationsServed:     channels.ObligationsServed,
	}

	// Spend cost-per-claim: aggregate across every row's claims/cost in
	// the report (not any single row) — the rollup's own cross-layer
	// total, computed the same zero-denominator-safe way SpendRow does.
	totalClaims := 0
	for _, row := range spend.Rows {
		totalClaims += row.ClaimsEmitted
	}
	if totalClaims > 0 {
		perClaim := spend.TotalCostUSD / float64(totalClaims)
		r.SpendCostPerClaimUSD = &perClaim
	}

	// Traffic error rate: aggregate across every row.
	totalCalls, totalErrorish := 0, 0.0
	for _, row := range traffic.Rows {
		totalCalls += row.Calls
		if row.ErrorRate != nil {
			totalErrorish += *row.ErrorRate / 100 * float64(row.Calls)
		}
	}
	if totalCalls > 0 {
		rate := totalErrorish / float64(totalCalls) * 100
		r.TrafficErrorRate = &rate
	}

	for _, row := range interventions.Rows {
		switch row.Name {
		case "unproven_done_claims_refused":
			r.InterventionsUnprovenDoneRefusals = row.Count
		case "review_findings_superseded":
			r.InterventionsReviewSuperseded = row.Count
		}
	}

	return r
}

// dailyRollupFileName is the on-disk convention: YYYY-MM-DD.json.
func dailyRollupFileName(date string) string {
	return date + ".json"
}

// WriteDailyRollup writes one date's rollup to dir/YYYY-MM-DD.json,
// overwriting any prior write for the SAME date idempotently (additive
// across dates, never across a single date's own re-computation).
func WriteDailyRollup(dir string, r DailyRollup) error {
	if r.Date == "" {
		return fmt.Errorf("WriteDailyRollup: empty Date")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("WriteDailyRollup: mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("WriteDailyRollup: marshal: %w", err)
	}
	path := filepath.Join(dir, dailyRollupFileName(r.Date))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("WriteDailyRollup: write %s: %w", path, err)
	}
	return nil
}

// ReadDailyRollups reads every rollup file for dates in [from, to]
// (inclusive, by calendar date), skipping any date with no rollup file —
// an honest gap, never a fabricated zero row. Returned in ascending date
// order.
func ReadDailyRollups(dir string, from, to time.Time) ([]DailyRollup, error) {
	from, to = from.UTC(), to.UTC()
	if to.Before(from) {
		return nil, nil
	}
	var dates []string
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	sort.Strings(dates)

	var out []DailyRollup
	for _, date := range dates {
		path := filepath.Join(dir, dailyRollupFileName(date))
		data, err := os.ReadFile(path)
		if err != nil {
			continue // no rollup for this date — an honest gap, not an error
		}
		var r DailyRollup
		if err := json.Unmarshal(data, &r); err != nil {
			continue // a malformed rollup file degrades the same way an absent one does
		}
		out = append(out, r)
	}
	return out, nil
}
