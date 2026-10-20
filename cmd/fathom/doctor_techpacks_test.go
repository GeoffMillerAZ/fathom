package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const techPackTestHeader = `pack: {name: "NAME", version: 1, about: "test pack"}
guards: [{scopes: ["**/*.templ"], note: "n", max_rules: 1}]
generated: [{output: "{dir}/{name}_templ.go", source: ["{dir}/{name}.templ"], regenerate: "templ generate"}]
`

const techPackTestFixtures = `guards: [{guard: 0, match: ["ui/a.templ"], nomatch: ["ui/a.go"]}]
`

func writeTechFile(t *testing.T, base, rel, data string) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// techPackRepo pins two kits through local replaces and one module that is
// not on disk. kitok's guards have a fixture and its generated section has
// none (served guards, withheld generated); kitbare has no fixtures at all.
func techPackRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("FATHOM_HOME", filepath.Join(base, "home"))
	t.Setenv("GOMODCACHE", filepath.Join(base, "gomodcache"))
	t.Setenv("GOPATH", filepath.Join(base, "gopath-unused"))
	t.Setenv("CUE_CACHE_DIR", filepath.Join(base, "cuecache"))
	t.Setenv("GOPROXY", "off")
	repo := filepath.Join(base, "repo")
	writeTechFile(t, repo, "go.mod", "module example.test/app\n\ngo 1.22\n\nrequire (\n\texample.test/kitok v1.0.0\n\texample.test/kitbare v1.5.0\n\texample.test/absent v0.9.0\n)\n\nreplace (\n\texample.test/kitok => ../kitok\n\texample.test/kitbare => ../kitbare\n)\n")
	writeTechFile(t, base, "kitok/go.mod", "module example.test/kitok\n\ngo 1.22\n")
	writeTechFile(t, base, "kitok/fathom-pack/pack.cue", strings.Replace(techPackTestHeader, "NAME", "okpack", 1))
	writeTechFile(t, base, "kitok/fathom-pack/fixtures/all.cue", techPackTestFixtures)
	writeTechFile(t, base, "kitbare/go.mod", "module example.test/kitbare\n\ngo 1.22\n")
	writeTechFile(t, base, "kitbare/fathom-pack/pack.cue", strings.Replace(techPackTestHeader, "NAME", "barepack", 1))
	return repo
}

// TestDoctorTechPacksRow is FD-21's anchor: one row names each discovered
// pack with its pin and path, the sections it serves and withholds, and its
// exclusion by exclude_packs or by the capability; a pinned module absent
// from disk is named and never fetched; a repository with no pack gets a
// plain pass.
func TestDoctorTechPacksRow(t *testing.T) {
	repo := techPackRepo(t)
	c := checkTechPacks(repo)
	if c.Name != "tech_packs" || c.Status != doctorWarn {
		t.Fatalf("row = %+v, want tech_packs warn (withheld sections)", c)
	}
	for _, want := range []string{
		"2 tech pack(s)",
		"okpack@1 pin example.test/kitok@v1.0.0 (replace)",
		"served: guards, withheld: generated (no fixtures)",
		"barepack@1 pin example.test/kitbare@v1.5.0",
		"served: none, withheld: guards (no fixtures), generated (no fixtures)",
		"excluded: no",
		"pinned but not on disk, never fetched: example.test/absent@v0.9.0",
		filepath.Join("kitok", "fathom-pack"),
	} {
		if !strings.Contains(c.Reason, want) {
			t.Errorf("reason lacks %q:\n%s", want, c.Reason)
		}
	}
	if !strings.Contains(c.Remedy, "fathom rules fixtures") {
		t.Errorf("remedy = %q, want the fixtures verb named", c.Remedy)
	}

	// exclude_packs: named once, the other pack stays served.
	writeTechFile(t, repo, ".fathom/modules.cue", "exclude_packs: [\"barepack\"]\n")
	c = checkTechPacks(repo)
	if !strings.Contains(c.Reason, "excluded: yes, by exclude_packs") || !strings.Contains(c.Reason, "exclude_packs names barepack") {
		t.Errorf("exclusion not disclosed:\n%s", c.Reason)
	}
	if strings.Count(c.Reason, "excluded: no") != 1 {
		t.Errorf("only okpack is served:\n%s", c.Reason)
	}

	// The capability off: every pack is named as excluded by it.
	writeTechFile(t, repo, ".fathom/capabilities.cue", "capabilities: \"tech-packs\": on: false\n")
	c = checkTechPacks(repo)
	if strings.Count(c.Reason, "excluded: yes, by the tech-packs capability (project)") != 2 {
		t.Errorf("capability exclusion not named for both packs:\n%s", c.Reason)
	}

	// No pins, no pack: a plain pass.
	empty := t.TempDir()
	if c := checkTechPacks(empty); c.Status != doctorPass || !strings.Contains(c.Reason, "no tech packs discovered") {
		t.Errorf("empty repository row = %+v", c)
	}
}
