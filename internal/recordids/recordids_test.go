package recordids

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicatesInEveryFamily(t *testing.T) {
	for _, tc := range []struct{ family, path, text, id string }{
		{"adr", "ADR.md", "## ADR-001: first\n## ADR-001: second\n", "ADR-001"},
		{"invariant", "docs/intent/invariants.md", "| I1 | first |\n| I1 | second |\n", "I1"},
		{"future", "docs/intent/futures/f1.md", "## F1. first\n## F1. second\n", "F1"},
		{"incident", "docs/intent/incidents.md", "### INC-1 first\n- INC-1 closed\n", "INC-1"},
		{"plan", "docs/plans/one.md", "- [ ] first {id: P-1}\n- [x] second {id: P-1}\n", "P-1"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tc.path, tc.text)
			cfg := Defaults()
			var fs []Family
			for _, f := range cfg.Families {
				if f.Name == tc.family {
					fs = append(fs, f)
				}
			}
			cfg.Families = fs
			d, err := Check(root, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(d) != 1 || d[0].ID != tc.id || d[0].First.Line != 1 || d[0].Second.Line != 2 || !strings.Contains(d[0].String(), tc.path+":1") || !strings.Contains(d[0].String(), tc.path+":2") {
				t.Fatalf("%+v", d)
			}
		})
	}
}

func TestPlansArchiveAndGrandfather(t *testing.T) {
	root := t.TempDir()
	row := "- [ ] task {id: SHARED-1}\n"
	write(t, root, "docs/plans/a.md", row)
	write(t, root, "docs/plans/b.md", row)
	write(t, root, "docs/plans/archive/old.md", row+row)
	cfg := Defaults()
	cfg.Families = cfg.Families[len(cfg.Families)-1:]
	// Explicitly asking for archive must still exclude it.
	cfg.Families[0].Files = append(cfg.Families[0].Files, "docs/plans/archive/*.md")
	if d, err := Check(root, cfg); err != nil || len(d) != 0 {
		t.Fatalf("default per plan: %+v %v", d, err)
	}
	cfg.CrossPlan = true
	if d, err := Check(root, cfg); err != nil || len(d) != 1 {
		t.Fatalf("cross plan: %+v %v", d, err)
	}
	cfg.Grandfather = []Exception{{ID: "SHARED-1", Paths: []string{"docs/plans/a.md", "docs/plans/b.md"}}}
	if d, err := Check(root, cfg); err != nil || len(d) != 0 {
		t.Fatalf("grandfather: %+v %v", d, err)
	}
	write(t, root, "docs/plans/c.md", row)
	if d, err := Check(root, cfg); err != nil || len(d) == 0 {
		t.Fatalf("exception spread: %+v %v", d, err)
	}
	write(t, root, "docs/plans/a.md", row+row)
	if d, err := Check(root, cfg); err != nil || len(d) < 2 {
		t.Fatalf("same file still denied: %+v %v", d, err)
	}
}

func TestConfigurableFamilyAndDeterminism(t *testing.T) {
	root := t.TempDir()
	write(t, root, "records/a.md", "## Q1 title\n## Q1 another\n````\n## Q1 example\n````\n")
	cfg := Config{Families: []Family{{Name: "question", Files: []string{"records/*.md"}, Pattern: `^## (Q[0-9]+)\b`}}}
	a, err := Check(root, cfg)
	if err != nil || len(a) != 1 {
		t.Fatalf("%+v %v", a, err)
	}
	b, err := Check(root, cfg)
	if err != nil || len(b) != 1 || a[0] != b[0] {
		t.Fatal("non deterministic")
	}
	cfg.Families[0].Pattern = "["
	if _, err := Check(root, cfg); err == nil {
		t.Fatal("bad pattern accepted")
	}
	cfg.Families[0].Pattern = "no capture"
	if _, err := Check(root, cfg); err == nil {
		t.Fatal("no id capture")
	}
}

func TestRepositoryRecordIDs(t *testing.T) {
	cfg, err := Load("../../.fathom/record-ids.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Check("../..", cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, dup := range d {
		t.Error(dup.String())
	}
	cfg.CrossPlan = true
	d, err = Check("../..", cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, dup := range d {
		t.Error(dup.String())
	}
}

func TestMissingRootIsNotUnique(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "missing"), Defaults()); err == nil {
		t.Fatal("missing root reported unique")
	}
}

func TestDuplicateOutputIsSorted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "records.md", "## Q1 first\n## Q1 second\n")
	cfg := Config{Families: []Family{{Name: "z", Files: []string{"records.md"}, Pattern: `^## (Q[0-9]+)\b`}, {Name: "a", Files: []string{"records.md"}, Pattern: `^## (Q[0-9]+)\b`}}}
	d, err := Check(root, cfg)
	if err != nil || len(d) != 2 || d[0].Family != "a" || d[1].Family != "z" {
		t.Fatalf("not sorted: %+v %v", d, err)
	}
}

func TestIncidentBoldHeading(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/intent/incidents.md", "**INC-15 first**\n**INC-15 duplicate**\n")
	d, err := Check(root, Defaults())
	if err != nil || len(d) != 1 || d[0].ID != "INC-15" {
		t.Fatalf("bold incident duplicates: %+v %v", d, err)
	}
}

func TestGrandfatherRejectsStalePairs(t *testing.T) {
	for _, paths := range [][]string{{"docs/plans/a.md", "docs/plans/b.md"}, {"docs/plans/a.md", "docs/plans/b.md", "docs/plans/c.md"}} {
		root := t.TempDir()
		write(t, root, "docs/plans/a.md", "- [ ] one {id: P-1}\n")
		write(t, root, "docs/plans/b.md", "- [ ] another {id: P-2}\n")
		cfg := Defaults()
		cfg.CrossPlan = true
		cfg.Grandfather = []Exception{{ID: "P-1", Paths: paths}}
		if _, err := Check(root, cfg); err == nil || !strings.Contains(err.Error(), "stale exception") {
			t.Fatalf("stale pair accepted: %v", err)
		}
	}
}

func TestRepositoryEnforcesCrossPlanUniqueness(t *testing.T) {
	cfg, err := Load("../../.fathom/record-ids.json")
	if err != nil || !cfg.CrossPlan {
		t.Fatalf("cross-plan check disabled: %+v %v", cfg, err)
	}
}
