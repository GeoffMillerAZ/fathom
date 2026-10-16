// Package analyticscsv contains the private adapter for projecting the exact
// CSV shape emitted by appkit/ui/analytics into app-owned exports.
package analyticscsv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"slices"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
)

var contextKeys = []string{
	"chart_id", "title", "unit", "grain", "aggregation", "window_kind", "window",
	"source_kind", "source", "source_href", "source_draft", "evaluated_at",
	"freshness", "completeness", "coverage_note", "uncertainty", "decimal_places",
}

var dataKeys = []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}

// ContextKeys returns a copy of the exact context-key inventory emitted by
// analytics.WriteCSV.
func ContextKeys() []string { return slices.Clone(contextKeys) }

// DataKeys returns a copy of the exact data-header inventory emitted by
// analytics.WriteCSV.
func DataKeys() []string { return slices.Clone(dataKeys) }

// Section extracts the context values and data rows from analytics.WriteCSV
// output. It deliberately recognizes only that bounded shared shape.
func Section(body []byte, expectedRows int) ([]string, [][]string, error) {
	if expectedRows < 0 || expectedRows > analytics.MaxRows {
		return nil, nil, fmt.Errorf("analytics: expected row count is out of range")
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1 // analytics.WriteCSV emits two-field context before its table.
	records, err := r.ReadAll()
	if err != nil || len(records) != len(contextKeys)+1+expectedRows {
		return nil, nil, fmt.Errorf("analytics: shared CSV record shape changed")
	}
	context := make([]string, len(contextKeys))
	for i, key := range contextKeys {
		if len(records[i]) != 2 || records[i][0] != key {
			return nil, nil, fmt.Errorf("analytics: shared CSV context changed")
		}
		context[i] = records[i][1]
	}
	if !slices.Equal(records[len(context)], dataKeys) {
		return nil, nil, fmt.Errorf("analytics: shared CSV data header changed")
	}
	rows := records[len(context)+1:]
	for _, row := range rows {
		if len(row) != len(dataKeys) {
			return nil, nil, fmt.Errorf("analytics: shared CSV row width changed")
		}
	}
	return context, rows, nil
}
