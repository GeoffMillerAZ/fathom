package analytics

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
)

func readRoleCSV(t *testing.T, d Dataset) ([][]string, [][]string) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(out.String()))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, record := range records {
		if len(record) == 10 && reflect.DeepEqual(record, []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}) {
			return records[:i], records[i+1:]
		}
	}
	t.Fatalf("ten-column data header missing: %v", records)
	return nil, nil
}

func TestWriteCSVRetainsRoleMeaning(t *testing.T) {
	d := fixture()
	d.Rows[0].Role = "scheduled"
	d.Rows[1].Role = "recorded"
	d.Legend = []LegendItem{
		{Role: "recorded", Label: "Recorded result"},
		{Role: "scheduled", Label: "Scheduled plan"},
	}
	context, rows := readRoleCSV(t, d)
	if len(context) != 17 { // encoding/csv skips the blank separator line.
		t.Fatalf("context shape changed: %v", context)
	}
	if len(rows) != 2 || rows[0][8] != "scheduled" || rows[0][9] != "Scheduled plan" || rows[1][8] != "recorded" || rows[1][9] != "Recorded result" {
		t.Fatalf("roles were associated by order instead of identity: %v", rows)
	}
	wantFirstEight := [][]string{
		{"sept", "September 2026", "1250.5", "USD", "present", "", "", ""},
		{"oct", "October 2026", "0", "USD", "present", "", "", ""},
	}
	for i := range rows {
		if !reflect.DeepEqual(rows[i][:8], wantFirstEight[i]) {
			t.Fatalf("established row fields changed: got %v want %v", rows[i][:8], wantFirstEight[i])
		}
	}
}

func TestWriteCSVRoleDefaultsMissingAndUnmapped(t *testing.T) {
	t.Run("default accent", func(t *testing.T) {
		_, rows := readRoleCSV(t, fixture())
		if rows[0][8] != "accent" || rows[0][9] != "" {
			t.Fatalf("default role or absent label changed: %v", rows[0])
		}
	})

	t.Run("unmapped single role", func(t *testing.T) {
		d := fixture()
		for i := range d.Rows {
			d.Rows[i].Role = "danger"
		}
		_, rows := readRoleCSV(t, d)
		if rows[0][8] != "danger" || rows[0][9] != "" {
			t.Fatalf("unmapped role acquired a guessed label: %v", rows[0])
		}
	})

	t.Run("missing row", func(t *testing.T) {
		d := fixture()
		d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One source row was unavailable"
		d.Rows = append(d.Rows, Row{ID: "nov", Label: "November 2026", Role: "forecast", MissingReason: "Not observed"})
		d.Legend = []LegendItem{{Role: "forecast", Label: "Forecast series"}}
		_, rows := readRoleCSV(t, d)
		got := rows[2]
		wantFirstEight := []string{"nov", "November 2026", "", "USD", "missing", "Not observed", "", ""}
		if !reflect.DeepEqual(got[:8], wantFirstEight) || got[8] != "forecast" || got[9] != "Forecast series" {
			t.Fatalf("missing row lost state or series meaning: %v", got)
		}
	})
}

func TestWriteCSVNeutralizesRoleLabelOnce(t *testing.T) {
	d := fixture()
	d.Legend = []LegendItem{{Role: "accent", Label: `=HYPERLINK("https://example.invalid","<b>open</b>")`}}
	_, rows := readRoleCSV(t, d)
	want := `'=HYPERLINK("https://example.invalid","<b>open</b>")`
	if rows[0][9] != want || rows[1][9] != want {
		t.Fatalf("role label was not neutralized exactly once: %v", rows)
	}
}

func TestWriteCSVInvalidRoleInputWritesNothing(t *testing.T) {
	d := fixture()
	d.Rows[0].Role = "not a role"
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err == nil || out.Len() != 0 {
		t.Fatalf("invalid input wrote a partial export: err=%v output=%q", err, out.String())
	}
}
