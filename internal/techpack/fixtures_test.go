package techpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPack = `pack: {name: "demo", version: 1, about: "demo pack"}
guards: [{scopes: ["**/*.templ"], note: "n", max_rules: 1}]
generated: [{output: "{dir}/{name}_templ.go", source: ["{dir}/{name}.templ"], regenerate: "templ generate"}]
formats: [{dialect: "templ", globs: ["**/*.templ"]}]
`

const testFixtures = `guards: [{guard: 0, match: ["ui/a.templ"], nomatch: ["ui/a.go"]}]
generated: [{output: "ui/a_templ.go", source: ["ui/a.templ"]}]
formats: [{path: "ui/a.templ", dialect: "templ"}]
`

func writePack(t *testing.T, pack, fixtures string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pack.cue"), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	if fixtures != "" {
		if err := os.MkdirAll(filepath.Join(dir, "fixtures"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fixtures", "all.cue"), []byte(fixtures), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunFixturesWithholdsUnfixturedSections(t *testing.T) {
	r, err := RunFixtures(writePack(t, testPack, ""))
	if err != nil {
		t.Fatal(err)
	}
	if r.OK() || len(r.Sections) != 3 {
		t.Fatalf("want 3 withheld sections, got %+v", r)
	}
	got := strings.Join(r.Lines(), "\n")
	for _, w := range []string{"withheld: guards (no fixtures)", "withheld: generated (no fixtures)", "withheld: formats (no fixtures)"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in\n%s", w, got)
		}
	}
}

func TestRunFixturesPassesAndFails(t *testing.T) {
	r, err := RunFixtures(writePack(t, testPack, testFixtures))
	if err != nil || !r.OK() {
		t.Fatalf("want pass, err=%v r=%+v", err, r)
	}
	for _, s := range r.Sections {
		if s.Run != 1 || s.Passed != 1 || s.Withheld {
			t.Errorf("section %+v", s)
		}
	}
	bad := strings.NewReplacer(`nomatch: ["ui/a.go"]`, `nomatch: ["ui/b.templ"]`,
		`source: ["ui/a.templ"]`, `source: ["ui/other.templ"]`,
		`dialect: "templ"}]`, `dialect: "go"}]`).Replace(testFixtures)
	r, err = RunFixtures(writePack(t, testPack, bad))
	if err != nil || r.OK() {
		t.Fatalf("want fail, err=%v", err)
	}
	for _, s := range r.Sections {
		if !s.Withheld || s.Failed < 1 || len(s.Failures) < 1 {
			t.Errorf("section %+v", s)
		}
	}
	if !strings.Contains(strings.Join(r.Lines(), "\n"), "ui/b.templ") {
		t.Errorf("failing fixture not named: %v", r.Lines())
	}
}

func TestRunFixturesInvalidPack(t *testing.T) {
	if _, err := RunFixtures(t.TempDir()); err == nil {
		t.Error("missing pack.cue must error")
	}
	if _, err := RunFixtures(writePack(t, `guards: []`, "")); err == nil {
		t.Error("headerless pack must error")
	}
}

func TestRunFixturesDeterministic(t *testing.T) {
	dir := writePack(t, testPack, testFixtures)
	a, _ := RunFixtures(dir)
	b, _ := RunFixtures(dir)
	if strings.Join(a.Lines(), "|") != strings.Join(b.Lines(), "|") {
		t.Error("nondeterministic")
	}
}
