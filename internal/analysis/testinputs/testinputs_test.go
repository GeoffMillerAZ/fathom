package testinputs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseLogResolvesPathsAgainstThePackageAndChdir(t *testing.T) {
	root := "/repo"
	log := strings.Join([]string{
		"# test log",
		"open ../../CHANGELOG.md",
		"stat testdata/golden.json",
		"getenv FATHOM_HOME",
		"open /repo/docs/plans",
		"open /tmp/fathom_tmp.d.abc/scratch.db",
		"chdir ../..",
		"open ADR.md",
		"getenv FATHOM_HOME",
		"open /usr/local/go/src/fmt/print.go",
		"",
	}, "\n")
	got, err := ParseLog(strings.NewReader(log), "/repo/internal/buildinfo", root)
	if err != nil {
		t.Fatal(err)
	}
	want := Observed{
		Files:   []string{"ADR.md", "CHANGELOG.md", "docs/plans", "internal/buildinfo/testdata/golden.json"},
		Env:     []string{"FATHOM_HOME"},
		Outside: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseLog:\n got %+v\nwant %+v", got, want)
	}
}

func TestParseLogAcceptsEitherSpellingOfASymlinkedRoot(t *testing.T) {
	log := "# test log\nopen /private/var/repo/README.md\nopen /var/repo/go.mod\n"
	got, err := ParseLog(strings.NewReader(log), "/var/repo/cmd/x", "/var/repo", "/private/var/repo")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "go.mod"}; !reflect.DeepEqual(got.Files, want) {
		t.Fatalf("files = %v, want %v", got.Files, want)
	}
}

func TestParseLogRefusesAFileThatIsNotATestlog(t *testing.T) {
	if _, err := ParseLog(strings.NewReader("open x\n"), "/repo/p", "/repo"); err == nil {
		t.Fatal("a log without the `# test log` header parsed; want an error")
	}
}

func TestParseLogCountsAnUnknownOpAndKeepsGoing(t *testing.T) {
	got, err := ParseLog(strings.NewReader("# test log\nmmap a\nopen b\n"), "/repo/p", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Unknown != 1 || !reflect.DeepEqual(got.Files, []string{"p/b"}) {
		t.Fatalf("got %+v, want 1 unknown op and file p/b", got)
	}
}

// writeMap lays out an observation directory the way scripts/test-inputs.sh
// writes it.
func writeMap(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadReadsPackageAndPerTestLogs(t *testing.T) {
	root := "/repo"
	dir := writeMap(t, map[string]string{
		"meta.json": `{"commit":"abc123","at":"2026-10-31T20:00:00Z","root":"/repo","packages":{` +
			`"internal/specdialect":{"exit":0},` +
			`"cmd/fathom":{"exit":0,"per_test":true,"tests":{"TestReleaseMatchesNewestChangelogHeading":0,"TestBroken":1}}}}`,
		"pkg/internal/specdialect/_package.log":                             "# test log\nopen /repo/docs/plans\nopen /repo/docs/plans/verbs.md\n",
		"pkg/cmd/fathom/_package.log":                                       "# test log\nopen ../../CHANGELOG.md\nopen ../../README.md\n",
		"pkg/cmd/fathom/_test.TestReleaseMatchesNewestChangelogHeading.log": "# test log\nopen ../../CHANGELOG.md\n",
	})
	m, err := Load(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Commit != "abc123" {
		t.Fatalf("commit = %q", m.Commit)
	}
	sd := m.Packages["internal/specdialect"]
	if want := []string{"docs/plans", "docs/plans/verbs.md"}; !reflect.DeepEqual(sd.Files, want) {
		t.Fatalf("specdialect files = %v, want %v", sd.Files, want)
	}
	cf := m.Packages["cmd/fathom"]
	if got := cf.Tests["TestReleaseMatchesNewestChangelogHeading"].Files; !reflect.DeepEqual(got, []string{"CHANGELOG.md"}) {
		t.Fatalf("per-test files = %v", got)
	}
	if _, ok := cf.Tests["TestBroken"]; ok {
		t.Fatal("a test whose isolated run failed has no observation of its own")
	}
	if !reflect.DeepEqual(cf.Unobserved, []string{"TestBroken"}) {
		t.Fatalf("unobserved = %v, want [TestBroken]", cf.Unobserved)
	}
}

func TestLoadWithoutMetaIsAnError(t *testing.T) {
	if _, err := Load(t.TempDir(), "/repo"); err == nil {
		t.Fatal("Load of a directory with no meta.json succeeded")
	}
}

func TestReadersMatchesOpenedFilesAndListedDirectories(t *testing.T) {
	m := Map{Packages: map[string]Package{
		"internal/specdialect": {Observed: Observed{Files: []string{"docs/plans", "docs/plans/verbs.md"}}},
		"cmd/fathom": {
			Observed: Observed{Files: []string{"CHANGELOG.md", "README.md"}},
			PerTest:  true,
			Tests: map[string]Observed{
				"TestReleaseMatchesNewestChangelogHeading": {Files: []string{"CHANGELOG.md"}},
				"TestReadmeLinks":                          {Files: []string{"README.md"}},
			},
			Unobserved: []string{"TestBroken"},
		},
		"internal/lint/tenancy": {Observed: Observed{Files: []string{"internal/mcp/server.go"}}},
	}}
	got := m.Readers([]Change{
		{Path: "CHANGELOG.md"},
		{Path: "docs/plans/verbs.md"},
		{Path: "docs/plans/new.md", Listing: true},
		{Path: "docs/plans/language-breadth.md"}, // an edit, not a listing change: the directory open does not see it
		{Path: "internal/mcp/server.go"},
	})
	want := []Hit{
		{Package: "cmd/fathom", Test: "TestBroken", File: "CHANGELOG.md", Via: "package"},
		{Package: "cmd/fathom", Test: "TestReleaseMatchesNewestChangelogHeading", File: "CHANGELOG.md", Via: "open"},
		{Package: "internal/lint/tenancy", File: "internal/mcp/server.go", Via: "open"},
		{Package: "internal/specdialect", File: "docs/plans/new.md", Via: "dir"},
		{Package: "internal/specdialect", File: "docs/plans/verbs.md", Via: "open"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Readers:\n got %+v\nwant %+v", got, want)
	}
}

func TestReadersOfARootLevelAddition(t *testing.T) {
	m := Map{Packages: map[string]Package{
		"internal/repofacts": {Observed: Observed{Files: []string{"."}}},
	}}
	got := m.Readers([]Change{{Path: "NEW.md", Listing: true}, {Path: "README.md"}})
	want := []Hit{{Package: "internal/repofacts", File: "NEW.md", Via: "dir"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// F2 (review of ADR-090): a file added in a new directory changes the
// listing of the first ancestor that existed, not of its own new parent.
func TestReadersNewDirectoryUsesTheDeepestExistingAncestor(t *testing.T) {
	m := Map{Packages: map[string]Package{
		"internal/walk": {Observed: Observed{Files: []string{"docs", "docs/plans"}}},
	}}
	got := m.Readers([]Change{{Path: "docs/plans/new/b.md", Listing: true, ListingDir: "docs/plans"}})
	want := []Hit{{Package: "internal/walk", File: "docs/plans/new/b.md", Via: "dir"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// F3: a read the package-wide log saw that no test's own log explains (a
// TestMain read, a test the list never named) selects the package whole.
func TestReadersUnexplainedPackageReadSelectsThePackageWhole(t *testing.T) {
	m := Map{Packages: map[string]Package{
		"cmd/x": {
			Observed: Observed{Files: []string{"go.mod", "testdata/fixture.json"}},
			PerTest:  true,
			Tests:    map[string]Observed{"TestA": {Files: []string{"go.mod"}}},
		},
		"cmd/empty": {Observed: Observed{Files: []string{"go.mod"}}, PerTest: true, Tests: map[string]Observed{}},
	}}
	got := m.Readers([]Change{{Path: "testdata/fixture.json"}, {Path: "go.mod"}})
	want := []Hit{
		{Package: "cmd/empty", File: "go.mod", Via: "package"},
		{Package: "cmd/x", File: "testdata/fixture.json", Via: "package"},
		{Package: "cmd/x", Test: "TestA", File: "go.mod", Via: "open"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

// F4: a missing or cut-short log never fails the map; its package is
// partial and runs whole for every change.
func TestLoadMarksAPackageWithABrokenLogPartial(t *testing.T) {
	dir := writeMap(t, map[string]string{
		"meta.json":              `{"commit":"c","root":"/repo","packages":{"a":{"exit":0},"b":{"exit":2},"c":{"exit":1}}}`,
		"pkg/a/_package.log":     "",
		"pkg/b/_package.log":     "# test log\nopen x\n",
		"pkg/c/_package.log":     "# test log\nopen ../docs/y.md\n",
		"pkg/c/unrelated.ignore": "",
	})
	m, err := Load(dir, "/repo")
	if err != nil {
		t.Fatalf("one bad log failed the whole map: %v", err)
	}
	if got := m.PartialPackages(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("partial = %v, want [a b] (an empty log, a panic's exit 2; c's failing tests, exit 1, flushed theirs)", got)
	}
	got := m.Readers([]Change{{Path: "README.md"}})
	want := []Hit{{Package: "a", File: "README.md", Via: "partial"}, {Package: "b", File: "README.md", Via: "partial"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// F5: an observation recorded in a worktree nested under the checkout that
// reads it resolves against the longer root.
func TestParseLogPrefersTheLongestRoot(t *testing.T) {
	log := "# test log\nopen ../../docs/plans/x.md\n"
	got, err := ParseLog(strings.NewReader(log), "/repo/.claude/worktrees/w/cmd/fathom", "/repo", "/repo/.claude/worktrees/w")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/plans/x.md"}; !reflect.DeepEqual(got.Files, want) {
		t.Fatalf("files = %v, want %v", got.Files, want)
	}
}

// F9: a read through a symlinked directory is also recorded under its
// target, the spelling git reports a change by.
func TestLoadAddsTheTargetSpellingOfASymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills", "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", ".claude", "skills"), filepath.Join(root, ".agents", "skills")); err != nil {
		t.Fatal(err)
	}
	dir := writeMap(t, map[string]string{
		"meta.json":          `{"commit":"c","root":"` + root + `","packages":{"p":{"exit":0}}}`,
		"pkg/p/_package.log": "# test log\nopen ../.agents/skills/s/SKILL.md\n",
	})
	m, err := Load(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Readers([]Change{{Path: ".claude/skills/s/SKILL.md"}}); len(got) != 1 || got[0].Package != "p" {
		t.Fatalf("a change to the target was not matched to the read through the link: %+v (files %v)", got, m.Packages["p"].Files)
	}
}
