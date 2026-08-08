package metrics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuildDailyRollup_FlattensAllFourReports(t *testing.T) {
	date := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 7, 30, 23, 0, 0, 0, time.UTC)

	spend := SpendReport{
		TotalCostUSD: 1.50,
		Rows:         []SpendRow{{Group: "obligations", ClaimsEmitted: 10}},
	}
	traffic := TrafficReport{
		TotalCalls: 100,
		Rows:       []TrafficRow{{Group: "agent", Calls: 100, ErrorRate: floatPtr(2)}},
	}
	channels := ChannelsReport{InjectionCoverageRate: floatPtr(0.91), ObligationsServed: 42}
	unproven, superseded := 5, 3
	interventions := InterventionsReport{Rows: []InterventionRow{
		{Name: "unproven_done_claims_refused", Count: &unproven},
		{Name: "review_findings_superseded", Count: &superseded},
	}}

	r := BuildDailyRollup(date, spend, traffic, channels, interventions, now)
	if r.Date != "2026-07-30" {
		t.Errorf("Date = %q, want 2026-07-30", r.Date)
	}
	if r.SpendTotalUSD != 1.50 {
		t.Errorf("SpendTotalUSD = %v, want 1.50", r.SpendTotalUSD)
	}
	if r.SpendCostPerClaimUSD == nil || *r.SpendCostPerClaimUSD != 0.15 {
		t.Errorf("SpendCostPerClaimUSD = %v, want 0.15", r.SpendCostPerClaimUSD)
	}
	if r.TrafficTotalCalls != 100 {
		t.Errorf("TrafficTotalCalls = %d, want 100", r.TrafficTotalCalls)
	}
	if r.TrafficErrorRate == nil || *r.TrafficErrorRate != 2 {
		t.Errorf("TrafficErrorRate = %v, want 2", r.TrafficErrorRate)
	}
	if r.ChannelsInjectionCoverageRate == nil || *r.ChannelsInjectionCoverageRate != 0.91 {
		t.Errorf("ChannelsInjectionCoverageRate = %v, want 0.91", r.ChannelsInjectionCoverageRate)
	}
	if r.ChannelsObligationsServed != 42 {
		t.Errorf("ChannelsObligationsServed = %d, want 42", r.ChannelsObligationsServed)
	}
	if r.InterventionsUnprovenDoneRefusals == nil || *r.InterventionsUnprovenDoneRefusals != 5 {
		t.Errorf("InterventionsUnprovenDoneRefusals = %v, want 5", r.InterventionsUnprovenDoneRefusals)
	}
	if r.InterventionsReviewSuperseded == nil || *r.InterventionsReviewSuperseded != 3 {
		t.Errorf("InterventionsReviewSuperseded = %v, want 3", r.InterventionsReviewSuperseded)
	}
}

func floatPtr(f float64) *float64 { return &f }

func TestWriteDailyRollup_ThenReadDailyRollups_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	r := DailyRollup{Date: "2026-07-30", GeneratedAt: "2026-07-30T23:00:00Z", SpendTotalUSD: 1.5}
	if err := WriteDailyRollup(dir, r); err != nil {
		t.Fatalf("WriteDailyRollup: %v", err)
	}
	got, err := ReadDailyRollups(dir, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReadDailyRollups: %v", err)
	}
	if len(got) != 1 || got[0].Date != "2026-07-30" || got[0].SpendTotalUSD != 1.5 {
		t.Fatalf("got %+v, want the written rollup back", got)
	}
}

func TestWriteDailyRollup_OverwritesSameDateIdempotently(t *testing.T) {
	dir := t.TempDir()
	if err := WriteDailyRollup(dir, DailyRollup{Date: "2026-07-30", SpendTotalUSD: 1.0}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteDailyRollup(dir, DailyRollup{Date: "2026-07-30", SpendTotalUSD: 2.0}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("files = %v, want exactly 1 (overwritten, not appended)", entries)
	}
	got, err := ReadDailyRollups(dir, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil || len(got) != 1 || got[0].SpendTotalUSD != 2.0 {
		t.Fatalf("got %+v, err=%v, want the SECOND write's value", got, err)
	}
}

func TestReadDailyRollups_MissingDatesSkippedNotZeroed(t *testing.T) {
	dir := t.TempDir()
	if err := WriteDailyRollup(dir, DailyRollup{Date: "2026-07-30", SpendTotalUSD: 1.0}); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Day 29 and 31 never had a rollup written.
	got, err := ReadDailyRollups(dir, time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReadDailyRollups: %v", err)
	}
	if len(got) != 1 || got[0].Date != "2026-07-30" {
		t.Fatalf("got %+v, want exactly the 1 real rollup, missing dates SKIPPED not zeroed", got)
	}
}

func TestReadDailyRollups_EmptyDir_NoError(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadDailyRollups(dir, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReadDailyRollups: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}

// TestRollupVersionStamp pins IO-13 (vision 14 §3): rollups carry the
// producing binary + schema version like every other persistent record.
func TestRollupVersionStamp(t *testing.T) {
	day := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	r := BuildDailyRollup(day, SpendReport{}, TrafficReport{}, ChannelsReport{}, InterventionsReport{}, day)
	if r.SchemaVersion != RollupSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", r.SchemaVersion, RollupSchemaVersion)
	}
	if r.FathomVersion == "" {
		t.Error("FathomVersion must never be empty")
	}
}
