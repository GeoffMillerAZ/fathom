package scenariobench

import (
	"path/filepath"
	"testing"
)

func TestPerTrialCellDir(t *testing.T) {
	if got, want := PerTrialCellDir("/cell", "", 1), filepath.Join("/cell", "trial-1"); got != want {
		t.Errorf("PerTrialCellDir(trial=1) = %q, want %q", got, want)
	}
	if got, want := PerTrialCellDir("/cell", "", 2), filepath.Join("/cell", "trial-2"); got != want {
		t.Errorf("PerTrialCellDir(trial=2) = %q, want %q", got, want)
	}
	// trial=0 (unset) behaves like trial 1, mirroring orOne's own
	// convention (sessions.go) for "no trial number given".
	if got, want := PerTrialCellDir("/cell", "", 0), filepath.Join("/cell", "trial-1"); got != want {
		t.Errorf("PerTrialCellDir(trial=0) = %q, want %q", got, want)
	}
	// An explicit attemptDir (wave-n1's own attempt layout) wins outright.
	if got, want := PerTrialCellDir("/cell", "/attempt/dir", 3), "/attempt/dir"; got != want {
		t.Errorf("PerTrialCellDir(attemptDir set) = %q, want %q", got, want)
	}
}
