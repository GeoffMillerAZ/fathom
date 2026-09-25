package receipts

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/internal/analyticscsv"
)

type evidenceMeasure string

const (
	evidenceReceipts evidenceMeasure = "receipts"
	evidenceRefunds  evidenceMeasure = "refunds"
	evidenceNet      evidenceMeasure = "net"
)

type evidenceSection struct {
	measure evidenceMeasure
	context []string
	rows    [][]string
}

// buildEvidenceCSV projects first, then composes only validated WriteCSV
// sections. It is an app-owned fixture export, not a general CSV formatter.
func buildEvidenceCSV(input fixtureData, selected selection) ([]byte, error) {
	projected, err := project(input, selected)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	w := csv.NewWriter(&out)
	header := []string{
		"series", "measure", "selection_period", "selection_compare",
		"selection_channel", "selection_stale", "selection_product", "product_channel",
	}
	for _, key := range analyticscsv.ContextKeys() {
		header = append(header, "context_"+key)
	}
	header = append(header, analyticscsv.DataKeys()...)
	if len(header) != 35 {
		return nil, fmt.Errorf("matdash analytics: evidence CSV header width changed")
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	writeSeries := func(series string, facts []projectedFact, net analytics.Dataset) error {
		if series != "current" && series != "previous" {
			return fmt.Errorf("matdash analytics: unsupported evidence series %q", series)
		}
		measures := [...]evidenceMeasure{evidenceReceipts, evidenceRefunds, evidenceNet}
		sections := make([]evidenceSection, 0, len(measures))
		for _, measure := range measures {
			dataset, err := evidenceDataset(measure, net, facts)
			if err != nil {
				return err
			}
			if err := dataset.Validate(projected.Now); err != nil {
				return fmt.Errorf("matdash analytics: %s %s evidence: %w", series, measure, err)
			}
			var shared bytes.Buffer
			if err := analytics.WriteCSV(&shared, dataset, projected.Now); err != nil {
				return err
			}
			context, rows, err := analyticscsv.Section(shared.Bytes(), len(facts))
			if err != nil {
				return err
			}
			for index := range facts {
				if rows[index][0] != facts[index].Product.ID {
					return fmt.Errorf("matdash analytics: %s %s evidence row order changed", series, measure)
				}
			}
			sections = append(sections, evidenceSection{measure: measure, context: context, rows: rows})
		}
		for index, item := range facts {
			for _, section := range sections {
				row := []string{
					series, string(section.measure), selected.Period, selected.Compare,
					selected.Channel, strconv.FormatBool(selected.Stale), selected.DetailID, item.Product.Channel,
				}
				row = append(row, section.context...)
				row = append(row, section.rows[index]...)
				if len(row) != len(header) {
					return fmt.Errorf("matdash analytics: evidence CSV row width changed")
				}
				if err := w.Write(row); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := writeSeries("current", projected.Facts, projected.Current); err != nil {
		return nil, err
	}
	if projected.Previous != nil {
		if err := writeSeries("previous", projected.PreviousFacts, *projected.Previous); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func evidenceDataset(measure evidenceMeasure, net analytics.Dataset, facts []projectedFact) (analytics.Dataset, error) {
	const netPrefix = "matdash-net-receipts-"
	if !strings.HasPrefix(net.Contract.ID, netPrefix) || !strings.HasPrefix(net.Contract.Title, "Net receipts by product, ") {
		return analytics.Dataset{}, fmt.Errorf("matdash analytics: net evidence identity changed")
	}
	if len(net.Rows) != len(facts) {
		return analytics.Dataset{}, fmt.Errorf("matdash analytics: net evidence row count changed")
	}
	for index := range facts {
		if net.Rows[index].ID != facts[index].Product.ID {
			return analytics.Dataset{}, fmt.Errorf("matdash analytics: net evidence row order changed")
		}
	}
	if measure == evidenceNet {
		return net, nil
	}

	dataset := analytics.Dataset{Contract: net.Contract, Rows: slices.Clone(net.Rows)}
	var label, prefix, aggregation string
	switch measure {
	case evidenceReceipts:
		label, prefix = "Receipts", "matdash-receipts-"
		aggregation = "source receipt amounts in integer cents; selected products summed before USD presentation"
	case evidenceRefunds:
		label, prefix = "Refunds", "matdash-refunds-"
		aggregation = "source refund amounts in integer cents; selected products summed before USD presentation"
	default:
		return analytics.Dataset{}, fmt.Errorf("matdash analytics: unsupported evidence measure %q", measure)
	}
	dataset.Contract.ID = prefix + strings.TrimPrefix(net.Contract.ID, netPrefix)
	dataset.Contract.Title = strings.Replace(net.Contract.Title, "Net receipts", label, 1)
	dataset.Contract.Aggregation = aggregation

	known := 0
	for index, item := range facts {
		amount := item.ReceiptsCents
		if measure == evidenceRefunds {
			amount = item.RefundsCents
		}
		row := &dataset.Rows[index]
		row.Value = nil
		row.MissingReason = ""
		if amount == nil {
			row.MissingReason = item.MissingReason
		} else {
			known++
			row.Value = float64Value(float64(*amount) / 100)
		}
	}
	completeCoverage := dataset.Contract.Window.End.Equal(dataset.Contract.Window.Start.AddDate(0, 1, 0))
	dataset.Contract.Completeness = analytics.Complete
	dataset.Contract.CoverageNote = ""
	if !completeCoverage || known != len(facts) {
		dataset.Contract.Completeness = analytics.Partial
		parts := make([]string, 0, 2)
		if !completeCoverage {
			parts = append(parts, fmt.Sprintf("Measured through %s (end exclusive); the calendar month is incomplete and values are not extrapolated.", dataset.Contract.Window.End.Format("2006-01-02T15:04:05Z07:00")))
		}
		if known != len(facts) {
			parts = append(parts, fmt.Sprintf("%d of %d selected product %s values are known.", known, len(facts), measure))
		}
		dataset.Contract.CoverageNote = strings.Join(parts, " ")
	}
	return dataset, nil
}
