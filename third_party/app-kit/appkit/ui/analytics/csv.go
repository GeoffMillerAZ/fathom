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
	roleLabels := d.roleLabels()
	wc := csv.NewWriter(w)
	for _, row := range csvContext(d.Contract, now) {
		if err := wc.Write([]string{row[0], csvText(row[1])}); err != nil {
			return err
		}
	}
	if err := wc.Write(nil); err != nil {
		return err
	}
	if err := wc.Write(csvDataHeader()); err != nil {
		return err
	}
	for _, row := range d.Rows {
		if err := wc.Write(csvDataRow(row, d.Contract, roleLabels)); err != nil {
			return err
		}
	}
	wc.Flush()
	return wc.Error()
}

// WriteTableCSV is an experimental rectangular alternative to WriteCSV for
// spreadsheet import: one header, then one row per selected observation. The
// seventeen context columns (prefixed context_) precede the same ten data columns;
// context is repeated on every row. Values, order, rounding and formula-safe text
// encoding are identical to WriteCSV. No app-owned comparison fields are inferred.
//
// Empty selections emit only the header, with no fabricated observation. Use
// WriteCSV when the context of an empty selection must travel with the export.
// Validation completes before the first write; IO errors are returned. Like
// WriteCSV, this is presentation evidence, not a lossless source-data format.
func WriteTableCSV(w io.Writer, d Dataset, now time.Time) error {
	if err := d.Validate(now); err != nil {
		return err
	}
	context := csvContext(d.Contract, now)
	header := make([]string, 0, len(context)+10)
	values := make([]string, 0, len(context))
	for _, field := range context {
		header = append(header, "context_"+field[0])
		values = append(values, csvText(field[1]))
	}
	header = append(header, csvDataHeader()...)
	wc := csv.NewWriter(w)
	if err := wc.Write(header); err != nil {
		return err
	}
	roleLabels := d.roleLabels()
	for _, row := range d.Rows {
		cells := make([]string, 0, len(header))
		cells = append(cells, values...)
		cells = append(cells, csvDataRow(row, d.Contract, roleLabels)...)
		if err := wc.Write(cells); err != nil {
			return err
		}
	}
	wc.Flush()
	return wc.Error()
}

func csvContext(c Contract, now time.Time) [][2]string {
	return [][2]string{
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
}

func csvDataHeader() []string {
	return []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}
}

func csvDataRow(row Row, c Contract, roleLabels map[string]string) []string {
	value, low, high, state := "", "", "", "missing"
	if row.Value != nil {
		value, state = number(*row.Value, c.Decimals, false), "present"
		if row.Bounds != nil {
			low = number(row.Bounds.Low, c.Decimals, false)
			high = number(row.Bounds.High, c.Decimals, false)
		}
	}
	role := rowRole(row)
	return []string{csvText(row.ID), csvText(row.Label), value, csvText(c.Unit), state, csvText(row.MissingReason), low, high, csvText(role), csvText(roleLabels[role])}
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
