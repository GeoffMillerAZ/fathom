package analyticscsv

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

func sectionFixture(t *testing.T) ([]byte, int) {
	return sectionFixtureRows(t, 1)
}

func sectionFixtureRows(t *testing.T, rowCount int) ([]byte, int) {
	t.Helper()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	d := analytics.Dataset{
		Contract: analytics.Contract{
			ID: "failures", Title: "Failed operations", Unit: "operations", Grain: "service",
			Aggregation: "count of failed operations", Decimals: 0,
			Window:       analytics.Window{Kind: analytics.RangeWindow, Start: now.Add(-time.Hour), End: now, TimeZone: "UTC"},
			Source:       vizframe.Derived("Synthetic unit-test ledger."),
			Freshness:    analytics.Freshness{ObservedAt: now.Add(-time.Minute), MaxAge: time.Hour},
			Completeness: analytics.Complete, Uncertainty: "Exact synthetic fixture count.",
		},
	}
	for i := 0; i < rowCount; i++ {
		value := float64(7 + i)
		d.Rows = append(d.Rows, analytics.Row{ID: fmt.Sprintf("api%d", i), Label: fmt.Sprintf("API %d", i), Value: &value})
	}
	var out bytes.Buffer
	if err := analytics.WriteCSV(&out, d, now); err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), len(d.Rows)
}

func TestKeysAreExactCopies(t *testing.T) {
	wantContext := []string{
		"chart_id", "title", "unit", "grain", "aggregation", "window_kind", "window",
		"source_kind", "source", "source_href", "source_draft", "evaluated_at",
		"freshness", "completeness", "coverage_note", "uncertainty", "decimal_places",
	}
	wantData := []string{"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label"}
	context, data := ContextKeys(), DataKeys()
	if !slices.Equal(context, wantContext) || !slices.Equal(data, wantData) {
		t.Fatalf("key inventory changed: context=%v data=%v", context, data)
	}
	context[0], data[0] = "mutated", "mutated"
	if ContextKeys()[0] != "chart_id" || DataKeys()[0] != "id" {
		t.Fatal("caller mutation changed the package key inventory")
	}
}

func TestSectionReadsAnalyticsWriteCSV(t *testing.T) {
	body, rows := sectionFixture(t)
	context, data, err := Section(body, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(context) != 17 || context[0] != "failures" || len(data) != 1 ||
		!slices.Equal(data[0], []string{"api0", "API 0", "7", "operations", "present", "", "", "", "accent", ""}) {
		t.Fatalf("unexpected section: context=%v rows=%v", context, data)
	}
}

func TestSectionAcceptsExpectedRowCountBounds(t *testing.T) {
	for _, count := range []int{0, analytics.MaxRows} {
		body, _ := sectionFixtureRows(t, count)
		_, rows, err := Section(body, count)
		if err != nil || len(rows) != count {
			t.Fatalf("row boundary %d rejected: rows=%d err=%v", count, len(rows), err)
		}
	}
}

func TestSectionRejectsLegacyEightColumnSchema(t *testing.T) {
	body, rows := sectionFixture(t)
	legacy := bytes.Replace(body, []byte(",role,role_label\n"), []byte("\n"), 1)
	legacy = bytes.Replace(legacy, []byte(",accent,\n"), []byte("\n"), 1)
	wantTail := "id,label,value,unit,state,missing_reason,lower,upper\napi0,API 0,7,operations,present,,,\n"
	if !bytes.HasSuffix(legacy, []byte(wantTail)) {
		t.Fatal("test must supply a complete, internally consistent legacy schema")
	}
	if _, _, err := Section(legacy, rows); err == nil {
		t.Fatal("legacy CSV must require an explicit migration, not silently lose role meaning")
	}
}

func TestSectionRejectsDriftAndMalformedInput(t *testing.T) {
	body, rows := sectionFixture(t)
	for name, test := range map[string]struct {
		body []byte
		rows int
	}{
		"malformed":    {[]byte("\"unterminated"), rows},
		"context":      {bytes.Replace(body, []byte("chart_id,"), []byte("renamed_id,"), 1), rows},
		"header":       {bytes.Replace(body, []byte("id,label,value,unit,state,missing_reason,lower,upper"), []byte("id,label,renamed,unit,state,missing_reason,lower,upper"), 1), rows},
		"count":        {body, rows + 1},
		"extra row":    {append(slices.Clone(body), []byte("extra,row\n")...), rows},
		"row width":    {bytes.Replace(body, []byte("present,,,,accent,\n"), []byte("present,,,accent,\n"), 1), rows},
		"negative":     {body, -1},
		"over maximum": {body, analytics.MaxRows + 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := Section(test.body, test.rows); err == nil {
				t.Fatal("invalid shared CSV shape was accepted")
			}
		})
	}
}
