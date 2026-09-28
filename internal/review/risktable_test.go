package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRiskTable_MatchesDesignDocNumbers(t *testing.T) {
	table := DefaultRiskTable()
	if got := table.blastRadiusHighCallers(); got != 50 {
		t.Errorf("blastRadiusHighCallers() = %d, want 50 (design doc §2)", got)
	}
	if got := table.zoneInterestMediumRate(); got != 2.0 {
		t.Errorf("zoneInterestMediumRate() = %v, want 2.0 (design doc §2)", got)
	}
}

func TestLoadRiskTable_MissingFileReturnsDefault(t *testing.T) {
	table, err := LoadRiskTable(filepath.Join(t.TempDir(), "does-not-exist.cue"))
	if err != nil {
		t.Fatalf("LoadRiskTable(missing) error: %v", err)
	}
	if table != DefaultRiskTable() {
		t.Errorf("LoadRiskTable(missing) = %+v, want DefaultRiskTable()", table)
	}
}

func TestLoadRiskTable_PartialOverrideKeepsOtherDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "risk.cue")
	if err := os.WriteFile(path, []byte("zone_interest_medium_rate: 3.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table, err := LoadRiskTable(path)
	if err != nil {
		t.Fatalf("LoadRiskTable: %v", err)
	}
	if table.zoneInterestMediumRate() != 3.0 {
		t.Errorf("zoneInterestMediumRate() = %v, want 3.0 (from file)", table.zoneInterestMediumRate())
	}
	if table.blastRadiusHighCallers() != 50 {
		t.Errorf("blastRadiusHighCallers() = %d, want 50 (untouched field keeps the binary default)", table.blastRadiusHighCallers())
	}
}

func TestLoadRiskTable_FullOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "risk.cue")
	if err := os.WriteFile(path, []byte("blast_radius_high_callers: 75\nzone_interest_medium_rate: 1.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table, err := LoadRiskTable(path)
	if err != nil {
		t.Fatalf("LoadRiskTable: %v", err)
	}
	if table.blastRadiusHighCallers() != 75 {
		t.Errorf("blastRadiusHighCallers() = %d, want 75", table.blastRadiusHighCallers())
	}
	if table.zoneInterestMediumRate() != 1.5 {
		t.Errorf("zoneInterestMediumRate() = %v, want 1.5", table.zoneInterestMediumRate())
	}
}

func TestLoadRiskTable_MalformedFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "risk.cue")
	if err := os.WriteFile(path, []byte("this is not valid cue {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRiskTable(path); err == nil {
		t.Fatal("LoadRiskTable(malformed) returned no error")
	}
}
