package conserve

// coverage_test.go — CV-68: TestLoadCoverage_MapsModulePathsAndMergesBlocks
// pins LoadCoverage's own two jobs (design §3.5): mapping a profile's
// import-path file keys onto repo-relative paths via each go.mod's own
// module line (a repo may hold several), and merging duplicate blocks —
// the same file:line.col span reported by more than one per-package
// section — by MAXIMUM count.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a small helper: write content to root/relPath, creating
// parent directories as needed.
func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", full, err)
	}
}

func TestLoadCoverage_MapsModulePathsAndMergesBlocks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/outer\n\ngo 1.22\n")
	writeFile(t, root, "vendored/nested/go.mod", "module example.com/nested\n\ngo 1.22\n")

	// Profile: two sections both instrumenting outer's pkg/a.go (the
	// -coverpkg=./... shape, one section per test binary) with DIFFERENT
	// counts for the identical span — max-merge must keep 3, not 1 or the
	// sum. A third block in the SAME file is untouched by either section
	// (count 0 both times — merges to 0, still "seen"). The nested
	// module's own file is mapped through ITS OWN module path, not
	// outer's. A file key matching neither module is skipped.
	profile := "mode: set\n" +
		"example.com/outer/pkg/a.go:3.10,5.2 2 1\n" +
		"example.com/outer/pkg/a.go:7.10,9.2 1 0\n" +
		"example.com/outer/pkg/a.go:3.10,5.2 2 3\n" + // duplicate span, higher count
		"example.com/nested/x.go:1.1,2.1 1 5\n" +
		"example.com/unrelated/z.go:1.1,2.1 1 9\n"
	profPath := filepath.Join(root, "coverage.out")
	if err := os.WriteFile(profPath, []byte(profile), 0o644); err != nil {
		t.Fatalf("WriteFile profile: %v", err)
	}

	prof, skipped, err := LoadCoverage(context.Background(), root, profPath)
	if err != nil {
		t.Fatalf("LoadCoverage: %v", err)
	}
	if prof.Mode != "set" {
		t.Errorf("Mode = %q, want set", prof.Mode)
	}

	blocks, ok := prof.Blocks["pkg/a.go"]
	if !ok {
		t.Fatalf("Blocks has no entry for pkg/a.go; got %+v", prof.Blocks)
	}
	if len(blocks) != 2 {
		t.Fatalf("pkg/a.go blocks = %+v, want exactly 2 (the duplicate span merged)", blocks)
	}
	// Sorted by start line: the 3..5 span first, then 7..9.
	if blocks[0].StartLine != 3 || blocks[0].Count != 3 {
		t.Errorf("blocks[0] = %+v, want StartLine=3 Count=3 (max of 1 and 3)", blocks[0])
	}
	if blocks[1].StartLine != 7 || blocks[1].Count != 0 {
		t.Errorf("blocks[1] = %+v, want StartLine=7 Count=0", blocks[1])
	}

	nested, ok := prof.Blocks["vendored/nested/x.go"]
	if !ok || len(nested) != 1 || nested[0].Count != 5 {
		t.Errorf("vendored/nested/x.go blocks = %+v (ok=%v), want one block Count=5 mapped via its OWN module", nested, ok)
	}

	if len(skipped) != 1 || skipped[0] != "example.com/unrelated/z.go" {
		t.Errorf("skipped = %v, want [example.com/unrelated/z.go]", skipped)
	}
}

func TestLoadCoverage_MissingFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/outer\n\ngo 1.22\n")
	if _, _, err := LoadCoverage(context.Background(), root, filepath.Join(root, "does-not-exist.out")); err == nil {
		t.Fatal("want an error for a missing profile file")
	}
}

// TestLoadCoverage_SkipsTestdataAndDotUnderscoreModules pins that
// findModules never descends into a directory the `go` tool itself
// ignores for package discovery (testdata/, .-prefixed, _-prefixed —
// `go help packages`): a fixture go.mod declaring the SAME module path
// as the real one, planted under testdata/ or an _-prefixed directory,
// must never claim the real module's own blocks.
func TestLoadCoverage_SkipsTestdataAndDotUnderscoreModules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/outer\n\ngo 1.22\n")
	writeFile(t, root, "internal/x/testdata/fixture/go.mod", "module example.com/outer\n\ngo 1.22\n")
	writeFile(t, root, "_ignored/agent/go.mod", "module example.com/outer\n\ngo 1.22\n")
	writeFile(t, root, ".hidden/worktree/go.mod", "module example.com/outer\n\ngo 1.22\n")

	profile := "mode: set\nexample.com/outer/pkg/a.go:3.10,5.2 2 7\n"
	profPath := filepath.Join(root, "coverage.out")
	if err := os.WriteFile(profPath, []byte(profile), 0o644); err != nil {
		t.Fatalf("WriteFile profile: %v", err)
	}

	prof, skipped, err := LoadCoverage(context.Background(), root, profPath)
	if err != nil {
		t.Fatalf("LoadCoverage: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}
	blocks, ok := prof.Blocks["pkg/a.go"]
	if !ok || len(blocks) != 1 || blocks[0].Count != 7 {
		t.Fatalf("Blocks[pkg/a.go] = %+v (ok=%v), want exactly one block Count=7 mapped to the REAL pkg/a.go, never a testdata/_/. fixture", blocks, ok)
	}
	for f := range prof.Blocks {
		if f != "pkg/a.go" {
			t.Errorf("unexpected mapped file %q — a testdata/_/. fixture module claimed a block", f)
		}
	}
}

// TestLoadCoverage_IdenticalModulePathPrefersNearestDir pins the tie-
// break design §3.5's "the module path of the nearest go.mod" requires
// when two REAL (non-ignored) go.mod files declare the identical module
// path — a nested full clone outside any dot/underscore/testdata
// directory, e.g. under a project's own scratch convention. The
// shallower (nearer root) module must win, deterministically, never an
// unstable sort's arbitrary pick.
func TestLoadCoverage_IdenticalModulePathPrefersNearestDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/outer\n\ngo 1.22\n")
	writeFile(t, root, "scratch/clone/go.mod", "module example.com/outer\n\ngo 1.22\n")

	profile := "mode: set\nexample.com/outer/pkg/a.go:3.10,5.2 2 4\n"
	profPath := filepath.Join(root, "coverage.out")
	if err := os.WriteFile(profPath, []byte(profile), 0o644); err != nil {
		t.Fatalf("WriteFile profile: %v", err)
	}

	prof, _, err := LoadCoverage(context.Background(), root, profPath)
	if err != nil {
		t.Fatalf("LoadCoverage: %v", err)
	}
	if _, ok := prof.Blocks["pkg/a.go"]; !ok {
		t.Fatalf("Blocks has no entry for pkg/a.go (the nearer, root module); got %+v", prof.Blocks)
	}
	if _, ok := prof.Blocks["scratch/clone/pkg/a.go"]; ok {
		t.Errorf("block wrongly mapped to the deeper nested clone scratch/clone/pkg/a.go; got %+v", prof.Blocks)
	}
}

// TestModuleDeclFrom_StripsCommentAndQuotes pins that a valid go.mod
// `module` line carrying a trailing `//` comment or surrounding quotes —
// both valid syntax `go list -m` renders as the bare path — extracts the
// SAME module path a plain line would, rather than a string no profile
// key ever matches (silently mapping zero blocks).
func TestModuleDeclFrom_StripsCommentAndQuotes(t *testing.T) {
	cases := []string{
		"module example.com/x\n",
		"module example.com/x // fixture module\n",
		"module \"example.com/x\"\n",
		"module example.com/x   //trailing, no space\n",
	}
	for _, c := range cases {
		got, ok := moduleDeclFrom([]byte(c))
		if !ok || got != "example.com/x" {
			t.Errorf("moduleDeclFrom(%q) = (%q, %v), want (\"example.com/x\", true)", c, got, ok)
		}
	}
}

func TestLoadCoverageMeta_RoundTrip(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "coverage.meta.json", `{"schema":"fathom.coverage/v1","commit":"abc123","dirty":false,"race":true,"profile":"coverage.out","passed":true}`)
	m, err := LoadCoverageMeta(filepath.Join(root, "coverage.meta.json"))
	if err != nil {
		t.Fatalf("LoadCoverageMeta: %v", err)
	}
	want := CoverageMeta{Schema: CoverageSchemaV1, Commit: "abc123", Dirty: false, Race: true, Profile: "coverage.out", Passed: true}
	if m != want {
		t.Errorf("LoadCoverageMeta = %+v, want %+v", m, want)
	}
}

func TestLoadCoverageMeta_MissingFile(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadCoverageMeta(filepath.Join(root, "nope.json")); err == nil {
		t.Fatal("want an error for a missing meta file")
	}
}
