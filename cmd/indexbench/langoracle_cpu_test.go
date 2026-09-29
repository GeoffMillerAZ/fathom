package main

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

// stubFileClocks swaps the clocks measureSubject times a file with: every
// wall reading advances wallStep, every CPU reading advances cpuStep (0
// cpuStep with cpuOK false is a platform with no CPU reading). It restores
// the real clocks when the test ends.
func stubFileClocks(t *testing.T, wallStep, cpuStep time.Duration, cpuOK bool) {
	t.Helper()
	prev := fileClocks
	var wall time.Time = time.Unix(1_700_000_000, 0)
	var cpu time.Duration
	fileClocks.wall = func() time.Time { wall = wall.Add(wallStep); return wall }
	fileClocks.cpu = func() time.Duration {
		if !cpuOK {
			return 0
		}
		cpu += cpuStep
		return cpu
	}
	t.Cleanup(func() { fileClocks = prev })
}

// TestMeasureSubjectMaxFileIsCPUTime (EW-3): max_file_ms, the tenancy
// floor's slowest-file number, is the file's CPU time — the same basis the
// throughput floor already reads (owner decision D1) — so a machine busy
// with other workers cannot fail a reading (TestCmdLangParity) by making one
// file wait. Wall clock is the fallback only where the platform has no CPU
// reading, and the reading says so through throughput_basis.
func TestMeasureSubjectMaxFileIsCPUTime(t *testing.T) {
	subject := tsFixtureSubject(t)
	measure := func(t *testing.T) (maxFileMS int64, basis string, extractMS float64) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "typescript.json")
		var stdout, stderr bytes.Buffer
		args := []string{"-dialect", "typescript", "-subject", subject, "-ctags", filepath.Join(t.TempDir(), "absent"), "-out", out}
		if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		r := readReading(t, out)
		return r.Tenancy.MaxFileMS, r.Tenancy.ThroughputBasis, r.Subjects[0].Tenancy.ExtractMS
	}

	t.Run("a file that waited 5s of wall clock and ran 3ms of CPU costs 3ms", func(t *testing.T) {
		stubFileClocks(t, 5*time.Second, 3*time.Millisecond, true)
		got, basis, wallMS := measure(t)
		if got != 3 {
			t.Fatalf("max_file_ms = %d, want 3 (the file's CPU time); a wall reading would say 5000", got)
		}
		if basis != "cpu" {
			t.Errorf("throughput_basis = %q, want cpu", basis)
		}
		if wallMS < 15000 {
			t.Errorf("extract_ms = %v: the wall total must still be recorded beside the CPU figures", wallMS)
		}
	})

	t.Run("no CPU reading falls back to wall clock and the reading says wall", func(t *testing.T) {
		stubFileClocks(t, 5*time.Second, 0, false)
		got, basis, _ := measure(t)
		if got != 5000 {
			t.Fatalf("max_file_ms = %d, want 5000 (wall fallback: this platform has no CPU reading)", got)
		}
		if basis != "wall" {
			t.Errorf("throughput_basis = %q, want wall", basis)
		}
	})
}
