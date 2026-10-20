package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorLegacyFilesNamesSupersededCopies(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("FATHOM_STATE_DIR", state)
	put := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if c := checkLegacyFiles(root); c.Status != doctorPass {
		t.Fatalf("clean repository: %+v", c)
	}
	put(filepath.Join(root, ".fathom", "owned.json"), "{}")
	put(filepath.Join(root, ".fathom", "leases.jsonl"), "")
	put(filepath.Join(root, ".fathom", "risk.cue"), "blast_radius_high_callers: 5\n")
	if c := checkLegacyFiles(root); c.Status != doctorPass {
		t.Fatalf("legacy copies with no new copy are still read, not stale: %+v", c)
	}
	put(filepath.Join(state, "owned.json"), "{}")
	put(filepath.Join(state, "leases.jsonl"), "")
	put(filepath.Join(root, ".fathom", "config.toml"), "[risk]\nblast_radius_high_callers = 9\n")
	c := checkLegacyFiles(root)
	if c.Status != doctorWarn {
		t.Fatalf("status = %v: %+v", c.Status, c)
	}
	for _, want := range []string{".fathom/owned.json", ".fathom/leases.jsonl", ".fathom/risk.cue"} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("reason does not name %s: %s", want, c.Reason)
		}
	}
	for _, f := range []string{"owned.json", "leases.jsonl", "risk.cue"} {
		if _, err := os.Stat(filepath.Join(root, ".fathom", f)); err != nil {
			t.Errorf("doctor deleted %s", f)
		}
	}
}
