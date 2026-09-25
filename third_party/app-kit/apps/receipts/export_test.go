package receipts

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var evidenceHeader = []string{
	"series", "measure", "selection_period", "selection_compare", "selection_channel", "selection_stale", "selection_product", "product_channel",
	"context_chart_id", "context_title", "context_unit", "context_grain", "context_aggregation", "context_window_kind", "context_window",
	"context_source_kind", "context_source", "context_source_href", "context_source_draft", "context_evaluated_at", "context_freshness",
	"context_completeness", "context_coverage_note", "context_uncertainty", "context_decimal_places",
	"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label",
}

func readEvidenceCSV(t *testing.T, body []byte) []map[string]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 || !reflect.DeepEqual(records[0], evidenceHeader) || len(records[0]) != 35 {
		t.Fatalf("evidence header changed: %v", records)
	}
	seenColumns := map[string]bool{}
	for _, name := range records[0] {
		if seenColumns[name] {
			t.Fatalf("duplicate evidence column %q", name)
		}
		seenColumns[name] = true
	}
	rows := make([]map[string]string, 0, len(records)-1)
	seenRows := map[string]bool{}
	for index, record := range records[1:] {
		if len(record) != len(evidenceHeader) {
			t.Fatalf("row %d width=%d, want %d: %v", index, len(record), len(evidenceHeader), record)
		}
		row := map[string]string{}
		for column, name := range evidenceHeader {
			row[name] = record[column]
		}
		key := row["series"] + "/" + row["id"] + "/" + row["measure"]
		if seenRows[key] {
			t.Fatalf("duplicate evidence row %q", key)
		}
		seenRows[key] = true
		rows = append(rows, row)
	}
	return rows
}

func expectedEvidenceProducts(selected selection) []string {
	if selected.DetailID != "" {
		return []string{selected.DetailID}
	}
	switch selected.Channel {
	case "all":
		return []string{"atlas", "beacon", "cedar"}
	case "app":
		return []string{"atlas", "cedar"}
	case "mobile":
		return []string{"beacon"}
	default:
		return nil
	}
}

func evidenceRow(t *testing.T, rows []map[string]string, series, product, measure string) map[string]string {
	t.Helper()
	for _, row := range rows {
		if row["series"] == series && row["id"] == product && row["measure"] == measure {
			return row
		}
	}
	t.Fatalf("missing %s/%s/%s evidence row", series, product, measure)
	return nil
}

func expectedEvidenceFact(t *testing.T, expected expectedData, period, product string) fact {
	t.Helper()
	for _, item := range expected.Facts[period] {
		if item.Product == product {
			return item
		}
	}
	t.Fatalf("literal CUE oracle has no %s/%s fact", period, product)
	return fact{}
}

func expectedEvidenceAmount(item fact, measure string) (*int64, string) {
	var amount *int64
	switch measure {
	case "receipts":
		amount = item.ReceiptsCents
	case "refunds":
		amount = item.RefundsCents
	case "net":
		if item.ReceiptsCents != nil && item.RefundsCents != nil {
			net := *item.ReceiptsCents - *item.RefundsCents
			amount = &net
		}
	}
	if amount == nil {
		return nil, item.MissingReason
	}
	return amount, ""
}

func expectedEvidenceValue(amount *int64) string {
	if amount == nil {
		return ""
	}
	return strconv.FormatFloat(float64(*amount)/100, 'f', -1, 64)
}

func expectedEvidenceCompleteness(t *testing.T, expected expectedData, period string, products []string, measure string) string {
	t.Helper()
	if period == "2024-11" {
		return "partial"
	}
	for _, product := range products {
		amount, _ := expectedEvidenceAmount(expectedEvidenceFact(t, expected, period, product), measure)
		if amount == nil {
			return "partial"
		}
	}
	return "complete"
}

func TestEvidenceCSVAll36OverviewAndDetailSelections(t *testing.T) {
	input, expected := loadCUE(t)
	before, _ := json.Marshal(input)
	count := 0
	for _, period := range []string{"2024-09", "2024-10", "2024-11"} {
		for _, channel := range []string{"all", "app", "mobile"} {
			for _, compare := range []string{"none", "previous"} {
				for _, stale := range []bool{false, true} {
					var overviewRows []map[string]string
					for _, detail := range []bool{false, true} {
						selected := selection{Period: period, Channel: channel, Compare: compare, Stale: stale}
						if detail {
							selected.DetailID = map[string]string{"all": "atlas", "app": "cedar", "mobile": "beacon"}[channel]
						}
						body, err := buildEvidenceCSV(input, selected)
						if err != nil {
							t.Fatalf("build %+v: %v", selected, err)
						}
						rows := readEvidenceCSV(t, body)
						if !detail {
							overviewRows = rows
						}
						products := expectedEvidenceProducts(selected)
						series := []string{"current"}
						if compare == "previous" {
							series = append(series, "previous")
						}
						if len(rows) != len(series)*len(products)*3 {
							t.Fatalf("selection %+v rows=%d", selected, len(rows))
						}
						index := 0
						productChannels := map[string]string{"atlas": "app", "beacon": "mobile", "cedar": "app"}
						for _, side := range series {
							factPeriod := period
							if side == "previous" {
								factPeriod = map[string]string{"2024-09": "2024-08", "2024-10": "2024-09", "2024-11": "2024-10"}[period]
							}
							for _, product := range products {
								literal := expectedEvidenceFact(t, expected, factPeriod, product)
								for _, measure := range []string{"receipts", "refunds", "net"} {
									row := rows[index]
									if row["series"] != side || row["id"] != product || row["measure"] != measure ||
										row["selection_period"] != period || row["selection_compare"] != compare || row["selection_channel"] != channel ||
										row["selection_stale"] != map[bool]string{false: "false", true: "true"}[stale] || row["selection_product"] != selected.DetailID ||
										row["product_channel"] != productChannels[product] {
										t.Fatalf("selection/order row %d changed: %+v", index, row)
									}
									amount, reason := expectedEvidenceAmount(literal, measure)
									state := "present"
									if amount == nil {
										state = "missing"
									}
									wantCompleteness := expectedEvidenceCompleteness(t, expected, factPeriod, products, measure)
									if row["value"] != expectedEvidenceValue(amount) || row["state"] != state || row["missing_reason"] != reason ||
										row["context_completeness"] != wantCompleteness || (row["context_coverage_note"] == "") != (wantCompleteness == "complete") {
										t.Fatalf("literal CUE oracle mismatch for %+v %s/%s/%s: %+v", selected, side, product, measure, row)
									}
									if detail {
										overview := evidenceRow(t, overviewRows, side, product, measure)
										for _, key := range []string{
											"value", "state", "missing_reason", "unit", "context_unit", "context_window_kind", "context_window",
											"context_source_kind", "context_source", "context_source_href", "context_source_draft",
											"context_evaluated_at", "context_freshness", "context_decimal_places",
										} {
											if row[key] != overview[key] {
												t.Fatalf("detail changed overview field %s for %s/%s/%s: %q versus %q", key, side, product, measure, row[key], overview[key])
											}
										}
									}
									index++
								}
							}
						}
						count++
					}
				}
			}
		}
	}
	if count != 72 {
		t.Fatalf("selection denominator=%d, want 36 overview + 36 detail", count)
	}
	after, _ := json.Marshal(input)
	if !bytes.Equal(before, after) {
		t.Fatal("evidence export mutated its source fixture")
	}
}

func TestEvidenceCSVLiteralValuesMissingAndMeasureCompleteness(t *testing.T) {
	input, _ := loadCUE(t)
	september, err := buildEvidenceCSV(input, selection{Period: "2024-09", Channel: "all", Compare: "previous"})
	if err != nil {
		t.Fatal(err)
	}
	rows := readEvidenceCSV(t, september)
	for _, want := range []struct{ series, product, measure, value string }{
		{"current", "atlas", "receipts", "1000"}, {"current", "atlas", "refunds", "200"}, {"current", "atlas", "net", "800"},
		{"current", "cedar", "net", "-150"}, {"previous", "beacon", "refunds", "0"}, {"previous", "cedar", "net", "0"},
	} {
		if got := evidenceRow(t, rows, want.series, want.product, want.measure)["value"]; got != want.value {
			t.Fatalf("%+v value=%q", want, got)
		}
	}

	october, err := buildEvidenceCSV(input, selection{Period: "2024-10", Channel: "all", Compare: "none"})
	if err != nil {
		t.Fatal(err)
	}
	rows = readEvidenceCSV(t, october)
	for _, measure := range []string{"receipts", "refunds", "net"} {
		atlas := evidenceRow(t, rows, "current", "atlas", measure)
		if atlas["value"] != "0" || atlas["state"] != "present" {
			t.Fatalf("Atlas zero lost for %s: %v", measure, atlas)
		}
	}
	cedarReceipt := evidenceRow(t, rows, "current", "cedar", "receipts")
	cedarRefund := evidenceRow(t, rows, "current", "cedar", "refunds")
	if cedarReceipt["value"] != "50" || cedarReceipt["state"] != "present" || cedarReceipt["missing_reason"] != "" ||
		cedarRefund["value"] != "" || cedarRefund["state"] != "missing" || cedarRefund["missing_reason"] == "" {
		t.Fatalf("known receipt was erased by missing refund: receipt=%v refund=%v", cedarReceipt, cedarRefund)
	}
	if evidenceRow(t, rows, "current", "beacon", "receipts")["state"] != "missing" {
		t.Fatal("unknown receipt was filled as zero")
	}
	for _, measure := range []string{"receipts", "refunds", "net"} {
		row := evidenceRow(t, rows, "current", "atlas", measure)
		if row["context_completeness"] != "partial" || row["context_coverage_note"] == "" {
			t.Fatalf("October overview %s completeness=%v", measure, row)
		}
	}

	detail, err := buildEvidenceCSV(input, selection{Period: "2024-10", Channel: "app", Compare: "none", DetailID: "cedar"})
	if err != nil {
		t.Fatal(err)
	}
	rows = readEvidenceCSV(t, detail)
	if evidenceRow(t, rows, "current", "cedar", "receipts")["context_completeness"] != "complete" ||
		evidenceRow(t, rows, "current", "cedar", "refunds")["context_completeness"] != "partial" {
		t.Fatal("detail did not evaluate raw measure completeness after scoping")
	}
}

func TestEvidenceCSVPartialStaleContextAndSharedEncoding(t *testing.T) {
	input, _ := loadCUE(t)
	input.Products[0].Label = "\t=Atlas"
	atlas := sourceFactByID(snapshotByPeriod(&input, "2024-11"), "atlas")
	*atlas.ReceiptsCents, *atlas.RefundsCents = 1, 0
	cedar := sourceFactByID(snapshotByPeriod(&input, "2024-11"), "cedar")
	*cedar.ReceiptsCents, *cedar.RefundsCents = 0, 1
	body, err := buildEvidenceCSV(input, selection{Period: "2024-11", Channel: "all", Compare: "none", Stale: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := readEvidenceCSV(t, body)
	for _, measure := range []string{"receipts", "refunds", "net"} {
		row := evidenceRow(t, rows, "current", "atlas", measure)
		if row["label"] != "'\t=Atlas" || strings.HasPrefix(row["label"], "''") || row["context_unit"] != "USD" || row["context_decimal_places"] != "2" ||
			row["context_completeness"] != "partial" || !strings.Contains(row["context_window"], "2024-11-21T00:00:00Z") ||
			!strings.HasPrefix(row["context_freshness"], "Stale:") || row["context_source"] != projectionSourceText {
			t.Fatalf("shared context/encoding changed for %s: %v", measure, row)
		}
	}
	if evidenceRow(t, rows, "current", "atlas", "receipts")["value"] != "0.01" ||
		evidenceRow(t, rows, "current", "cedar", "net")["value"] != "-0.01" {
		t.Fatal("cent precision or signed value changed")
	}

	input, _ = loadCUE(t)
	beacon := sourceFactByID(snapshotByPeriod(&input, "2024-10"), "beacon")
	beacon.MissingReason = "\t=missing source"
	body, err = buildEvidenceCSV(input, selection{Period: "2024-10", Channel: "mobile", Compare: "none"})
	if err != nil {
		t.Fatal(err)
	}
	rows = readEvidenceCSV(t, body)
	for _, measure := range []string{"receipts", "refunds", "net"} {
		reason := evidenceRow(t, rows, "current", "beacon", measure)["missing_reason"]
		if reason != "'\t=missing source" || strings.HasPrefix(reason, "''") {
			t.Fatalf("missing reason was not escaped exactly once for %s: %q", measure, reason)
		}
	}
}

func TestEvidenceCSVInvalidIsNilAndOutputIsDeterministic(t *testing.T) {
	input, _ := loadCUE(t)
	selected := selection{Period: "2024-09", Channel: "all", Compare: "previous"}
	first, err := buildEvidenceCSV(input, selected)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildEvidenceCSV(input, selected)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("evidence output is not deterministic: %v", err)
	}
	invalid, _ := loadCUE(t)
	invalid.Products[0].Label = ""
	if body, err := buildEvidenceCSV(invalid, selected); err == nil || body != nil {
		t.Fatalf("invalid fixture returned body=%q err=%v", body, err)
	}
	valid, _ := loadCUE(t)
	if body, err := buildEvidenceCSV(valid, selection{Period: "2024-09", Channel: "desktop", Compare: "none"}); err == nil || body != nil {
		t.Fatalf("invalid selection returned body=%q err=%v", body, err)
	}
}
