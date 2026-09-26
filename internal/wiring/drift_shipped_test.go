package wiring

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A skill file whose bytes are exactly what this binary ships is fathom's
// own content: the baseline in .fathom/owned.json (gitignored, per
// checkout) lags whenever the shipped skill changes in git, and reading
// that lag as drift blocked every push after a skill edit (config-audit's
// wiring_current, 2026-09-25). Content that matches neither the recorded
// digest nor the shipped rendering is still drift.
func TestDriftSkills_ShippedContentIsNotDrift(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, ".claude", "skills", "fathom-custodian", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(skillPath, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const old = "<!-- fathom:owned v1 -->\n# custodian, as shipped yesterday\n"
	const shipped = "<!-- fathom:owned v1 -->\n# custodian, as this binary ships it\n"
	label := ".claude/skills/fathom-custodian/SKILL.md"
	skills := []SkillFile{{Path: skillPath, Label: label, Shipped: []byte(shipped)}}

	write(old)
	if _, err := RecordBaselines(root, nil, skills, "2026-09-24", nil, false); err != nil {
		t.Fatal(err)
	}

	// The shipped skill changed in git; the checkout now holds it.
	write(shipped)
	ledger, _ := LoadLedger(root)
	if rep := DriftSkills(ledger, skills, nil); len(rep.Drifts) != 0 || rep.Checked != 1 {
		t.Fatalf("shipped content read as drift: %+v", rep)
	}

	// Recording settles the lagging baseline without --rebaseline: the file
	// is what `fathom init --skills` would have written.
	recorded, err := RecordBaselines(root, nil, skills, "2026-09-25", nil, false)
	if err != nil || strings.Join(recorded, ";") != "skill "+label {
		t.Fatalf("RecordBaselines = %v, %v; want the shipped skill re-recorded", recorded, err)
	}
	ledger, _ = LoadLedger(root)
	if k, ok := ownedKey(ledger, label, skillKeyID); !ok || k.Digest != digest(shipped) || k.Wrote != "2026-09-25" {
		t.Fatalf("ledger = %+v, %v; want the shipped digest recorded on 2026-09-25", k, ok)
	}

	// Anything else is still drift, reported by kind and path, and is not
	// re-recorded without the operator's --rebaseline.
	write("<!-- fathom:owned v1 -->\n---\nhooks: [x]\n---\n# custodian\n")
	if rep := DriftSkills(ledger, skills, nil); len(rep.Drifts) != 1 {
		t.Fatalf("a non-shipped edit must drift: %+v", rep)
	}
	if recorded, _ := RecordBaselines(root, nil, skills, "2026-09-26", nil, false); len(recorded) != 0 {
		t.Fatalf("a drifted skill was re-recorded without --rebaseline: %v", recorded)
	}
}
