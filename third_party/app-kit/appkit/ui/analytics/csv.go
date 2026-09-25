package analytics

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// WriteCSV writes a validated selection as two-column context records, a blank
// record, then a fixed data header and its rows. Readers should allow variable
// field counts until the data header. Numeric cells use the same rounded values
// as RenderBars without display grouping; missing numeric cells stay empty.
// Text with formula/control prefixes receives a leading apostrophe. This is a
// spreadsheet-oriented text encoding, not a byte-preserving text round trip.
// Validation completes before the first write; IO errors are returned.
func WriteCSV(w io.Writer, d Dataset, now time.Time) error {
	if err := d.Validate(now); err != nil {
		return err
	}
	c := d.Contract
	roleLabels := d.roleLabels()
	wc := csv.NewWriter(w)
	context := [][2]string{
		{"chart_id", c.ID}, {"title", c.Title}, {"unit", c.Unit},
		{"grain", c.Grain}, {"aggregation", c.Aggregation},
		{"window_kind", string(c.Window.Kind)}, {"window", c.Window.label()},
		{"source_kind", string(c.Source.Kind)}, {"source", c.Source.Text}, {"source_href", c.Source.Href},
		{"source_draft", strconv.FormatBool(c.Source.Draft)},
		{"evaluated_at", now.Format(time.RFC3339Nano)},
		{"freshness", c.Freshness.label(now)}, {"completeness", string(c.Completeness)},
		{"coverage_note", c.CoverageNote}, {"uncertainty", c.Uncertainty},
		{"decimal_places", strconv.Itoa(c.Decimals)},
	}
	for _, row := range context {
		if err := wc.Write([]string{row[0], csvText(row[1])}); err != nil {
			return err
		}
	}
	if err := wc.Write(nil); err != nil {
		return err
	}
	if err := wc.Write([]string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}); err != nil {
		return err
	}
	for _, row := range d.Rows {
		value, low, high, state := "", "", "", "missing"
		if row.Value != nil {
			value, state = number(*row.Value, c.Decimals, false), "present"
			if row.Bounds != nil {
				low = number(row.Bounds.Low, c.Decimals, false)
				high = number(row.Bounds.High, c.Decimals, false)
			}
		}
		role := rowRole(row)
		if err := wc.Write([]string{csvText(row.ID), csvText(row.Label), value, csvText(c.Unit), state, csvText(row.MissingReason), low, high, csvText(role), csvText(roleLabels[role])}); err != nil {
			return err
		}
	}
	wc.Flush()
	return wc.Error()
}

func csvText(s string) string {
	start := strings.TrimLeftFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	})
	if strings.ContainsAny(s, "\t\r\n") || (start != "" && strings.ContainsRune("=+-@", rune(start[0]))) {
		return "'" + s
	}
	return s
}
