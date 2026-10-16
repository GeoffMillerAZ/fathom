package receipts

import (
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

const (
	maxAmountCents       = int64(1_000_000_000)
	maxSourceAge         = 24 * time.Hour
	projectionSourceText = "apps/receipts/fixture.cue; first-party synthetic net receipts equal receipts minus refunds in integer cents."
)

type fixtureData struct {
	Now             string     `json:"now"`
	StaleObservedAt string     `json:"stale_observed_at"`
	Products        []product  `json:"products"`
	Snapshots       []snapshot `json:"snapshots"`
}

type product struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Channel string `json:"channel"`
}

type snapshot struct {
	Period      string `json:"period"`
	CoverageEnd string `json:"coverage_end"`
	ObservedAt  string `json:"observed_at"`
	Facts       []fact `json:"facts"`
}

type fact struct {
	Product       string `json:"product"`
	ReceiptsCents *int64 `json:"receipts_cents"`
	RefundsCents  *int64 `json:"refunds_cents"`
	MissingReason string `json:"missing_reason"`
}

type selection struct {
	Period   string
	Channel  string
	Compare  string
	Stale    bool
	DetailID string
}

type projectedFact struct {
	Product       product
	ReceiptsCents *int64
	RefundsCents  *int64
	NetCents      *int64
	MissingReason string
}

type projectionSummary struct {
	SelectedCount           int
	KnownCount              int
	KnownSubtotalCents      int64
	ExactTotalCents         *int64
	PreviousExactTotalCents *int64
	DeltaCents              *int64
	LargestDeclineIDs       []string
}

type detailProjection struct {
	Selection        selection
	Product          product
	Current          projectedFact
	Previous         *projectedFact
	CurrentContract  analytics.Contract
	PreviousContract *analytics.Contract
}

type projection struct {
	Selection     selection
	Now           time.Time
	Current       analytics.Dataset
	Previous      *analytics.Dataset
	Facts         []projectedFact
	PreviousFacts []projectedFact
	Summary       projectionSummary
	Detail        *detailProjection
}

type preparedSnapshot struct {
	period                         string
	start, coverageEnd, nominalEnd time.Time
	observedAt                     time.Time
	facts                          map[string]fact
}

func project(input fixtureData, selected selection) (projection, error) {
	now, staleObservedAt, products, snapshots, err := validateFixture(input)
	if err != nil {
		return projection{}, err
	}
	if selected.Period != "2024-09" && selected.Period != "2024-10" && selected.Period != "2024-11" {
		return projection{}, fmt.Errorf("matdash analytics: unsupported selected period %q", selected.Period)
	}
	if selected.Channel != "all" && selected.Channel != "app" && selected.Channel != "mobile" {
		return projection{}, fmt.Errorf("matdash analytics: unsupported channel %q", selected.Channel)
	}
	if selected.Compare != "none" && selected.Compare != "previous" {
		return projection{}, fmt.Errorf("matdash analytics: unsupported comparison %q", selected.Compare)
	}

	chosen := make([]product, 0, len(products))
	for _, item := range products {
		if selected.Channel == "all" || item.Channel == selected.Channel {
			chosen = append(chosen, item)
		}
	}
	if len(chosen) == 0 {
		return projection{}, fmt.Errorf("matdash analytics: channel %q selects no products", selected.Channel)
	}
	if selected.DetailID != "" {
		found := false
		for _, item := range chosen {
			if item.ID == selected.DetailID {
				chosen = []product{item}
				found = true
				break
			}
		}
		if !found {
			return projection{}, fmt.Errorf("matdash analytics: detail product %q is unknown or outside channel %q", selected.DetailID, selected.Channel)
		}
	}

	observedAt := snapshots[selected.Period].observedAt
	if selected.Stale {
		observedAt = staleObservedAt
	}
	currentFacts := selectFacts(chosen, snapshots[selected.Period])
	currentDataset := buildDataset(selected, "current", currentFacts, snapshots[selected.Period], observedAt)
	if err := currentDataset.Validate(now); err != nil {
		return projection{}, fmt.Errorf("matdash analytics: current dataset: %w", err)
	}

	out := projection{
		Selection: selected,
		Now:       now,
		Current:   currentDataset,
		Facts:     currentFacts,
		Summary:   summarizeCurrent(currentFacts, snapshotComplete(snapshots[selected.Period])),
	}
	if selected.Compare == "previous" {
		previousPeriod := map[string]string{"2024-09": "2024-08", "2024-10": "2024-09", "2024-11": "2024-10"}[selected.Period]
		previousSnapshot := snapshots[previousPeriod]
		previousObservedAt := previousSnapshot.observedAt
		if selected.Stale {
			previousObservedAt = staleObservedAt
		}
		previousFacts := selectFacts(chosen, previousSnapshot)
		previousDataset := buildDataset(selected, "previous", previousFacts, previousSnapshot, previousObservedAt)
		if err := previousDataset.Validate(now); err != nil {
			return projection{}, fmt.Errorf("matdash analytics: previous dataset: %w", err)
		}
		out.Previous = &previousDataset
		out.PreviousFacts = previousFacts
		completePair := snapshotComplete(snapshots[selected.Period]) && snapshotComplete(previousSnapshot)
		previousSummary := summarizeCurrent(previousFacts, snapshotComplete(previousSnapshot))
		out.Summary.PreviousExactTotalCents = previousSummary.ExactTotalCents
		if completePair && out.Summary.ExactTotalCents != nil && previousSummary.ExactTotalCents != nil {
			delta := *out.Summary.ExactTotalCents - *previousSummary.ExactTotalCents
			out.Summary.DeltaCents = int64Value(delta)
			out.Summary.LargestDeclineIDs = largestDeclines(currentFacts, previousFacts)
		}
	}

	if selected.DetailID != "" {
		current := factByID(currentFacts, selected.DetailID)
		detail := detailProjection{
			Selection:       selected,
			Product:         current.Product,
			Current:         current,
			CurrentContract: currentDataset.Contract,
		}
		if out.Previous != nil {
			previous := factByID(out.PreviousFacts, selected.DetailID)
			detail.Previous = &previous
			contract := out.Previous.Contract
			detail.PreviousContract = &contract
		}
		out.Detail = &detail
	}
	return out, nil
}

func validateFixture(input fixtureData) (time.Time, time.Time, []product, map[string]preparedSnapshot, error) {
	now, err := parseUTC("now", input.Now)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, err
	}
	staleObservedAt, err := parseUTC("stale source observation", input.StaleObservedAt)
	if err != nil || staleObservedAt.After(now) {
		if err == nil {
			err = fmt.Errorf("matdash analytics: stale source observation is after now")
		}
		return time.Time{}, time.Time{}, nil, nil, err
	}
	if len(input.Products) != 3 {
		return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: expected exactly three products")
	}
	productChannels := map[string]string{"atlas": "app", "beacon": "mobile", "cedar": "app"}
	seenProducts := map[string]bool{}
	for _, item := range input.Products {
		expectedChannel, allowed := productChannels[item.ID]
		if !allowed {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: unknown product id %q", item.ID)
		}
		if seenProducts[item.ID] {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: duplicate product id %q", item.ID)
		}
		seenProducts[item.ID] = true
		if strings.TrimSpace(item.Label) == "" || item.Channel != expectedChannel {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: product %q has invalid label or channel", item.ID)
		}
	}

	if len(input.Snapshots) != 4 {
		return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: expected exactly four snapshots")
	}
	allowedPeriods := map[string]bool{"2024-08": true, "2024-09": true, "2024-10": true, "2024-11": true}
	prepared := make(map[string]preparedSnapshot, len(input.Snapshots))
	for _, item := range input.Snapshots {
		if !allowedPeriods[item.Period] {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: unknown snapshot period %q", item.Period)
		}
		if _, exists := prepared[item.Period]; exists {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: duplicate snapshot period %q", item.Period)
		}
		start, nominalEnd := periodWindow(item.Period)
		coverageEnd, coverageErr := parseUTC("coverage end for "+item.Period, item.CoverageEnd)
		observedAt, observedErr := parseUTC("source observation for "+item.Period, item.ObservedAt)
		if coverageErr != nil {
			return time.Time{}, time.Time{}, nil, nil, coverageErr
		}
		if observedErr != nil {
			return time.Time{}, time.Time{}, nil, nil, observedErr
		}
		if !coverageEnd.After(start) || coverageEnd.After(nominalEnd) {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: coverage end for %s must be within its calendar month", item.Period)
		}
		if observedAt.After(now) {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: source observation for %s is after now", item.Period)
		}
		facts := make(map[string]fact, len(item.Facts))
		for _, sourceFact := range item.Facts {
			if !seenProducts[sourceFact.Product] {
				return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: snapshot %s has unknown fact product %q", item.Period, sourceFact.Product)
			}
			if _, exists := facts[sourceFact.Product]; exists {
				return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: snapshot %s has duplicate fact product %q", item.Period, sourceFact.Product)
			}
			if err := validateFact(item.Period, sourceFact); err != nil {
				return time.Time{}, time.Time{}, nil, nil, err
			}
			facts[sourceFact.Product] = sourceFact
		}
		if len(facts) != len(input.Products) {
			return time.Time{}, time.Time{}, nil, nil, fmt.Errorf("matdash analytics: snapshot %s must represent every product exactly once", item.Period)
		}
		prepared[item.Period] = preparedSnapshot{period: item.Period, start: start, coverageEnd: coverageEnd, nominalEnd: nominalEnd, observedAt: observedAt, facts: facts}
	}
	return now, staleObservedAt, append([]product(nil), input.Products...), prepared, nil
}

func validateFact(period string, item fact) error {
	for _, candidate := range []struct {
		label  string
		amount *int64
	}{{"receipts", item.ReceiptsCents}, {"refunds", item.RefundsCents}} {
		label, amount := candidate.label, candidate.amount
		if amount != nil && (*amount < 0 || *amount > maxAmountCents) {
			return fmt.Errorf("matdash analytics: %s %s for %q is outside 0..%d cents", period, label, item.Product, maxAmountCents)
		}
	}
	missing := item.ReceiptsCents == nil || item.RefundsCents == nil
	if missing != (strings.TrimSpace(item.MissingReason) != "") {
		return fmt.Errorf("matdash analytics: %s fact %q must pair missing inputs with one reason", period, item.Product)
	}
	return nil
}

func parseUTC(field, value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("matdash analytics: %s is required", field)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.Location() != time.UTC {
		return time.Time{}, fmt.Errorf("matdash analytics: %s must be an RFC3339 UTC instant", field)
	}
	return parsed, nil
}

func periodWindow(period string) (time.Time, time.Time) {
	month := map[string]time.Month{"2024-08": time.August, "2024-09": time.September, "2024-10": time.October, "2024-11": time.November}[period]
	start := time.Date(2024, month, 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func snapshotComplete(item preparedSnapshot) bool { return item.coverageEnd.Equal(item.nominalEnd) }

func selectFacts(products []product, item preparedSnapshot) []projectedFact {
	out := make([]projectedFact, 0, len(products))
	for _, selectedProduct := range products {
		source := item.facts[selectedProduct.ID]
		projected := projectedFact{
			Product:       selectedProduct,
			ReceiptsCents: cloneInt64(source.ReceiptsCents),
			RefundsCents:  cloneInt64(source.RefundsCents),
			MissingReason: source.MissingReason,
		}
		if source.ReceiptsCents != nil && source.RefundsCents != nil {
			projected.NetCents = int64Value(*source.ReceiptsCents - *source.RefundsCents)
		}
		out = append(out, projected)
	}
	return out
}

func buildDataset(selected selection, side string, facts []projectedFact, item preparedSnapshot, observedAt time.Time) analytics.Dataset {
	complete := snapshotComplete(item)
	known := 0
	rows := make([]analytics.Row, 0, len(facts))
	for _, item := range facts {
		row := analytics.Row{ID: item.Product.ID, Label: item.Product.Label, MissingReason: item.MissingReason}
		if item.NetCents != nil {
			known++
			row.Value = float64Value(float64(*item.NetCents) / 100)
		}
		rows = append(rows, row)
	}
	completeness := analytics.Complete
	coverageNote := ""
	if !complete || known != len(facts) {
		completeness = analytics.Partial
		parts := make([]string, 0, 2)
		if !complete {
			parts = append(parts, fmt.Sprintf("Measured through %s (end exclusive); the calendar month is incomplete and values are not extrapolated.", item.coverageEnd.Format(time.RFC3339)))
		}
		if known != len(facts) {
			parts = append(parts, fmt.Sprintf("%d of %d selected product net values are known.", known, len(facts)))
		}
		coverageNote = strings.Join(parts, " ")
	}
	return analytics.Dataset{
		Contract: analytics.Contract{
			ID:          fmt.Sprintf("matdash-net-receipts-%s-%s-%s", strings.ReplaceAll(item.period, "-", ""), selected.Channel, side),
			Title:       fmt.Sprintf("Net receipts by product, %s", item.period),
			Unit:        "USD",
			Grain:       "product in the selected channel",
			Aggregation: "receipts minus refunds in integer cents; selected products summed before USD presentation",
			Decimals:    2,
			Window: analytics.Window{
				Kind: analytics.RangeWindow, Start: item.start, End: item.coverageEnd, TimeZone: "UTC",
			},
			Source:       vizframe.Derived(projectionSourceText),
			Freshness:    analytics.Freshness{ObservedAt: observedAt, MaxAge: maxSourceAge},
			Completeness: completeness,
			CoverageNote: coverageNote,
			Uncertainty:  "Synthetic cash-flow inputs; no accounting revenue, forecast or causality claim.",
		},
		Rows: rows,
	}
}

func summarizeCurrent(facts []projectedFact, completeCoverage bool) projectionSummary {
	summary := projectionSummary{SelectedCount: len(facts)}
	for _, item := range facts {
		if item.NetCents != nil {
			summary.KnownCount++
			summary.KnownSubtotalCents += *item.NetCents
		}
	}
	if completeCoverage && summary.KnownCount == summary.SelectedCount {
		summary.ExactTotalCents = int64Value(summary.KnownSubtotalCents)
	}
	return summary
}

func largestDeclines(current, previous []projectedFact) []string {
	previousByID := make(map[string]projectedFact, len(previous))
	for _, item := range previous {
		previousByID[item.Product.ID] = item
	}
	var largest int64
	var ids []string
	for _, item := range current {
		prior := previousByID[item.Product.ID]
		contribution := *item.NetCents - *prior.NetCents
		if contribution >= 0 {
			continue
		}
		if ids == nil || contribution < largest {
			largest, ids = contribution, []string{item.Product.ID}
		} else if contribution == largest {
			ids = append(ids, item.Product.ID)
		}
	}
	return ids
}

func factByID(facts []projectedFact, id string) projectedFact {
	for _, item := range facts {
		if item.Product.ID == id {
			return item
		}
	}
	panic("validated detail id disappeared")
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	return int64Value(*value)
}

func int64Value(value int64) *int64       { return &value }
func float64Value(value float64) *float64 { return &value }
