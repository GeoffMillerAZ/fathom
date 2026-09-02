package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckStanzaLoaded pins PC-25: SKIP (never FAIL) with the reason when
// no InstructionsLoaded beat names this repo's CLAUDE.md/AGENTS.md; PASS
// naming the file, the load reason and the latest time once one does; a
// beat about ANOTHER repo's CLAUDE.md never counts (the beat log is
// user-global — the attribution failure doctor once had with edit-hook
// beats must not recur here).
func TestCheckStanzaLoaded(t *testing.T) {
	beatsPath := filepath.Join(t.TempDir(), "hooks.jsonl")
	t.Setenv("FATHOM_HOOKS_LOG", beatsPath)
	root := t.TempDir()

	// No log at all.
	c := checkStanzaLoaded(root)
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, "no InstructionsLoaded beat") || !strings.Contains(c.Reason, "hook not wired here") {
		t.Errorf("no log: %+v", c)
	}
	if c.Remedy == "" || !strings.Contains(c.Remedy, "fathom init") {
		t.Errorf("skip must name the remedy: %+v", c)
	}

	// Wired at project scope, but no session has loaded anything yet: the
	// reason dates the wiring with a question mark.
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks":{"InstructionsLoaded":[{"hooks":[{"type":"command","command":"fathom session-hook --event instructions-loaded","timeout":5}]}]}}`
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkStanzaLoaded(root)
	if c.Status != doctorSkipped || !strings.Contains(c.Reason, "hook wired at ") || !strings.HasSuffix(strings.SplitN(c.Reason, " — ", 2)[0], "?)") {
		t.Errorf("wired, no beat: %+v", c)
	}

	// Beats: another repo's CLAUDE.md (must not count), then this repo's.
	other := t.TempDir()
	lines := []string{
		`{"channel":"instructions-loaded","kind":"loaded","at":"2026-09-02T10:00:00Z","meta":{"file_path":"` + filepath.Join(other, "CLAUDE.md") + `","load_reason":"session_start","memory_type":"Project"}}`,
		`{"channel":"session-hook","kind":"fired","at":"2026-09-02T10:00:01Z"}`,
	}
	if err := os.WriteFile(beatsPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkStanzaLoaded(root)
	if c.Status != doctorSkipped {
		t.Errorf("another repo's beat counted for this one: %+v", c)
	}

	lines = append(
		lines,
		`{"channel":"instructions-loaded","kind":"loaded","at":"2026-09-02T11:00:00Z","meta":{"file_path":"`+filepath.Join(root, "AGENTS.md")+`","load_reason":"nested_traversal"}}`,
		`{"channel":"instructions-loaded","kind":"loaded","at":"2026-09-02T12:00:00Z","meta":{"file_path":"`+filepath.Join(root, "CLAUDE.md")+`","load_reason":"session_start","memory_type":"Project"}}`,
		`{"channel":"instructions-loaded","kind":"loaded","at":"2026-09-02T09:00:00Z","meta":{"file_path":"`+filepath.Join(root, "CLAUDE.md")+`","load_reason":"compact"}}`,
	)
	if err := os.WriteFile(beatsPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = checkStanzaLoaded(root)
	if c.Status != doctorPass {
		t.Fatalf("beat present: %+v", c)
	}
	for _, want := range []string{"CLAUDE.md loaded by a session", "session_start, Project", "latest 2026-09-02T12:00:00Z", "3 beat(s)"} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("pass reason missing %q: %s", want, c.Reason)
		}
	}
	if c.Remedy != "" {
		t.Errorf("pass carries a remedy: %q", c.Remedy)
	}
	if again := checkStanzaLoaded(root); again != c {
		t.Errorf("not deterministic:\n%+v\n%+v", c, again)
	}

	// A torn tail never blanks the history.
	if err := os.WriteFile(beatsPath, []byte(strings.Join(lines, "\n")+"\n{\"channel\":\"instru"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c = checkStanzaLoaded(root); c.Status != doctorPass {
		t.Errorf("torn tail: %+v", c)
	}
}

// TestDoctorCarriesStanzaLoaded: the row is in the checklist, right after
// stanza_present, for every harness.
func TestDoctorCarriesStanzaLoaded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_HOOKS_LOG", filepath.Join(t.TempDir(), "hooks.jsonl"))
	checks := runDoctorChecks(t.TempDir(), "svc", "claude", "http://127.0.0.1:1")
	prev := ""
	found := false
	for _, c := range checks {
		if c.Name == "stanza_loaded" {
			found = true
			if prev != "stanza_present" {
				t.Errorf("stanza_loaded follows %q, want stanza_present", prev)
			}
			if c.Status != doctorSkipped {
				t.Errorf("fresh repo must SKIP, never FAIL: %+v", c)
			}
		}
		prev = c.Name
	}
	if !found {
		t.Error("stanza_loaded row missing from doctor")
	}
}
