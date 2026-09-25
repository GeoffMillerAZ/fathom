package main

import (
	"bytes"
	"encoding/csv"
	"testing"
)

// Compile and execute the exact embedded consumer during ordinary root tests.
// Its typed imports alone cannot catch a breaking wire-schema migration.
func TestSyntheticConsumerContract(t *testing.T) {
	defer func() {
		if problem := recover(); problem != nil {
			t.Fatalf("standalone consumer contract failed: %v", problem)
		}
	}()
	main()
}

func TestCSVContractRefusesSchemaAndMeaningDrift(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([][]string) [][]string
	}{
		{"legacy eight columns", func(rows [][]string) [][]string {
			for i := range rows {
				rows[i] = rows[i][:8]
			}
			return rows
		}},
		{"renamed header", func(rows [][]string) [][]string { rows[0][9] = "meaning"; return rows }},
		{"reordered header", func(rows [][]string) [][]string {
			rows[0][8], rows[0][9] = rows[0][9], rows[0][8]
			return rows
		}},
		{"wrong role", func(rows [][]string) [][]string { rows[1][8] = "positive"; return rows }},
		{"lost meaning", func(rows [][]string) [][]string { rows[1][9] = ""; return rows }},
		{"missing becomes zero", func(rows [][]string) [][]string { rows[3][2] = "0"; return rows }},
		{"lost row", func(rows [][]string) [][]string { return rows[:3] }},
		{"extra row", func(rows [][]string) [][]string { return append(rows, rows[1]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := [][]string{
				{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"},
				{"measured", `Measured <script>alert("x")</script>`, "12.3", "items", "present", "", "", "", "accent", "Scheduled <work>"},
				{"zero", "Measured zero", "0", "items", "present", "", "", "", "accent", "Scheduled <work>"},
				{"unknown", "Unknown value", "", "items", "missing", "not observed in synthetic fixture", "", "", "accent", "Scheduled <work>"},
			}
			encode := func(rows [][]string) []byte {
				var b bytes.Buffer
				writer := csv.NewWriter(&b)
				if err := writer.WriteAll(rows); err != nil {
					t.Fatal(err)
				}
				return b.Bytes()
			}
			assertCSV(encode(rows))
			defer func() {
				if recover() == nil {
					t.Fatal("consumer accepted corrupted schema, rows or meaning")
				}
			}()
			assertCSV(encode(tc.mutate(rows)))
		})
	}
}
