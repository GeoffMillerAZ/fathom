package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRL(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestConfigRiskLearningSectionsWinOverLegacyFiles (FD-31): [risk] and
// [learning] in config.toml carry the thresholds risk.cue and learning.cue
// carried. A repository section wins, per field, over the user default and
// shadows the legacy file beside it, which Load names; a legacy file with no
// section beside it is not shadowed.
func TestConfigRiskLearningSectionsWinOverLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "home", "config.toml")
	proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
	writeRL(t, user, "[risk]\nblast_radius_high_callers = 10\nzone_interest_medium_rate = 1.5\n\n[learning]\nretire_refutes = 4\n")
	writeRL(t, proj, "[risk]\nblast_radius_high_callers = 77\n\n[learning]\npromote_corroborations = 9\n")
	legacyRisk := filepath.Join(dir, "repo", ".fathom", "risk.cue")
	legacyLearn := filepath.Join(dir, "repo", ".fathom", "learning.cue")
	writeRL(t, legacyRisk, "blast_radius_high_callers: 5\n")
	writeRL(t, legacyLearn, "promote_corroborations: 2\n")

	l, err := Load(user, proj)
	if err != nil {
		t.Fatal(err)
	}
	c := l.Config
	if c.Risk.BlastRadiusHighCallers != 77 || c.Risk.ZoneInterestMediumRate != 1.5 {
		t.Fatalf("risk = %+v, want project 77 over user 10 and the user's 1.5", c.Risk)
	}
	if c.Learning.PromoteCorroborations != 9 || c.Learning.RetireRefutes != 4 {
		t.Fatalf("learning = %+v", c.Learning)
	}
	if got := l.Provenance["risk.blast_radius_high_callers"].Source; got != SourceProject {
		t.Errorf("risk.blast_radius_high_callers source = %v", got)
	}
	if got := l.Provenance["risk.zone_interest_medium_rate"].Source; got != SourceUser {
		t.Errorf("risk.zone_interest_medium_rate source = %v", got)
	}
	if len(l.ShadowedLegacy) != 2 || l.ShadowedLegacy[0] != legacyLearn || l.ShadowedLegacy[1] != legacyRisk {
		t.Fatalf("ShadowedLegacy = %v, want learning.cue and risk.cue (sorted)", l.ShadowedLegacy)
	}

	// No section in the repository: the legacy files are not shadowed, and
	// the user's defaults still load.
	writeRL(t, proj, "[console]\nconserve_max_files = 9\n")
	l, err = Load(user, proj)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.ShadowedLegacy) != 0 {
		t.Fatalf("ShadowedLegacy = %v with no repository section", l.ShadowedLegacy)
	}
	if l.Config.Risk.BlastRadiusHighCallers != 10 {
		t.Fatalf("user default not loaded: %+v", l.Config.Risk)
	}

	// A value the legacy schema refuses is refused here too.
	writeRL(t, proj, "[risk]\nblast_radius_high_callers = 0\n")
	if _, err := Load(user, proj); err == nil {
		t.Fatal("blast_radius_high_callers = 0 loaded")
	}
}
