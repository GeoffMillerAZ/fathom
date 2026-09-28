// learningthresholds_test.go — GL-12 (docs/plans/governance-learning.md,
// design doc §6): the dual-mode load contract (missing file → binary
// defaults; a partial file overrides only the fields it names) mirroring
// internal/review's risktable_test.go precedent for schema/risk.
package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLearningThresholds(t *testing.T) {
	d := DefaultLearningThresholds()
	if d.PromoteCorroborations != 3 {
		t.Errorf("PromoteCorroborations = %d, want 3 (design doc §6)", d.PromoteCorroborations)
	}
	if d.PromoteMinSessions != 2 {
		t.Errorf("PromoteMinSessions = %d, want 2", d.PromoteMinSessions)
	}
	if d.PromoteWindowDays != 30 {
		t.Errorf("PromoteWindowDays = %d, want 30", d.PromoteWindowDays)
	}
	if d.RetireRefutes != 3 {
		t.Errorf("RetireRefutes = %d, want 3", d.RetireRefutes)
	}
	if d.RetireNeverTriggeredDays != 180 {
		t.Errorf("RetireNeverTriggeredDays = %d, want 180", d.RetireNeverTriggeredDays)
	}
}

func TestLoadLearningThresholds_MissingFileReturnsDefaults(t *testing.T) {
	got, err := LoadLearningThresholds(filepath.Join(t.TempDir(), "nope.cue"))
	if err != nil {
		t.Fatalf("LoadLearningThresholds: %v", err)
	}
	if got != DefaultLearningThresholds() {
		t.Errorf("got %+v, want defaults %+v", got, DefaultLearningThresholds())
	}
}

func TestLoadLearningThresholds_PartialOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.cue")
	if err := os.WriteFile(path, []byte("retire_refutes: 5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLearningThresholds(path)
	if err != nil {
		t.Fatalf("LoadLearningThresholds: %v", err)
	}
	want := DefaultLearningThresholds()
	want.RetireRefutes = 5
	if got != want {
		t.Errorf("got %+v, want %+v (only retire_refutes overridden)", got, want)
	}
}

func TestLoadLearningThresholds_EveryFieldOverridden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.cue")
	body := `
promote_corroborations: 7
promote_min_sessions: 4
promote_window_days: 21
retire_refutes: 9
retire_never_triggered_days: 90
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLearningThresholds(path)
	if err != nil {
		t.Fatalf("LoadLearningThresholds: %v", err)
	}
	want := LearningThresholds{
		PromoteCorroborations:    7,
		PromoteMinSessions:       4,
		PromoteWindowDays:        21,
		RetireRefutes:            9,
		RetireNeverTriggeredDays: 90,
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadLearningThresholds_MalformedFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.cue")
	if err := os.WriteFile(path, []byte("retire_refutes: \"not-an-int\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLearningThresholds(path); err == nil {
		t.Fatal("want an error for a non-int retire_refutes, got nil")
	}
}
