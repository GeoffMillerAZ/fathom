package certify

import "testing"

func TestSelectionTimeSaved(t *testing.T) {
	sel := Selection{
		Tests:   []string{"pkg.TestBoth"},
		Skipped: []string{"pkg.TestParseOnly", "pkg.TestRenderOnly"},
	}
	durations := map[string]int64{
		"pkg.TestBoth":       500,
		"pkg.TestParseOnly":  200,
		"pkg.TestRenderOnly": 300,
	}
	got := selectionTimeSaved(sel, durations)
	if got.SelectedMs != 500 {
		t.Errorf("SelectedMs = %d, want 500", got.SelectedMs)
	}
	if got.SkippedMs != 500 {
		t.Errorf("SkippedMs = %d, want 500", got.SkippedMs)
	}
	if got.TotalMs != 1000 {
		t.Errorf("TotalMs = %d, want 1000", got.TotalMs)
	}
	if got.Percent != 50 {
		t.Errorf("Percent = %v, want 50", got.Percent)
	}
	if len(got.Missing) != 0 {
		t.Errorf("Missing = %+v, want none", got.Missing)
	}
}

func TestSelectionTimeSaved_MissingDurationNamedNeverFabricated(t *testing.T) {
	sel := Selection{
		Tests:   []string{"pkg.TestA"},
		Skipped: []string{"pkg.TestB"},
	}
	// pkg.TestB has no recorded duration (never attested, or attested
	// with no duration carried) — it must be NAMED, and must contribute
	// 0, never a guessed average.
	durations := map[string]int64{"pkg.TestA": 100}
	got := selectionTimeSaved(sel, durations)
	if got.SelectedMs != 100 || got.SkippedMs != 0 || got.TotalMs != 100 {
		t.Errorf("got = %+v, want SelectedMs=100 SkippedMs=0 TotalMs=100", got)
	}
	if len(got.Missing) != 1 || got.Missing[0] != "pkg.TestB" {
		t.Errorf("Missing = %+v, want [pkg.TestB]", got.Missing)
	}
}

func TestSelectionTimeSaved_EmptySelection(t *testing.T) {
	got := selectionTimeSaved(Selection{}, map[string]int64{"pkg.TestA": 100})
	if got.TotalMs != 0 || got.Percent != 0 {
		t.Errorf("got = %+v, want a zero TimeSaved for an empty selection", got)
	}
}

func TestSelectionTimeSaved_NothingSkippedIsHundredPercent(t *testing.T) {
	sel := Selection{Tests: []string{"pkg.TestA"}}
	got := selectionTimeSaved(sel, map[string]int64{"pkg.TestA": 100})
	if got.Percent != 0 {
		t.Errorf("Percent = %v, want 0 (nothing skipped, no time saved)", got.Percent)
	}
}
