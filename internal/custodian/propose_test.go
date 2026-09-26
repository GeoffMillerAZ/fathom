package custodian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposeRelocatesOnlyUnambiguousMoves: a dead pointer whose basename
// names exactly one repo file gets an edit (file-relative for a markdown
// link, root-relative for a backtick path, fragment kept); none (gone, not
// moved) and two (ambiguous) are named with why, never guessed; a
// violation no diff can mend gets its remedy; holding concerns get
// nothing.
func TestProposeRelocatesOnlyUnambiguousMoves(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "design", "guide.md"), "# guide\n")
	writeTestFile(t, filepath.Join(root, "docs", "a", "notes.md"), "x\n")
	writeTestFile(t, filepath.Join(root, "docs", "b", "notes.md"), "x\n")
	writeTestFile(t, filepath.Join(root, "sub", "CLAUDE.md"), strings.Join([]string{
		"See [the guide](../docs/guide.md#setup) and `docs/guide.md`.", // 1: moved → docs/design/guide.md
		"Old notes: `docs/notes.md`.",                                  // 2: ambiguous
		"Gone: `docs/vanished.md`.",                                    // 3: gone
		"cp bin/fathom somewhere",                                      // 4: forbidden
		"",
	}, "\n"))
	concerns := []Concern{
		{ID: "pointers", Want: "pointers resolve", Check: Check{Kind: KindPointersResolve}},
		{ID: "no_cp", Want: "no raw cp", Check: Check{Kind: KindForbiddenPattern, Pattern: `cp bin/fathom`}},
		{ID: "thin", Want: "thin", Check: Check{Kind: KindMaxLines, File: "sub/CLAUDE.md", Lines: 100}},
	}
	findings := Audit(root, concerns)
	props, err := Propose(root, findings)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 2 {
		t.Fatalf("one proposal per VIOLATED concern (the holding max_lines gets none), got %d: %+v", len(props), props)
	}
	p := props[0]
	if len(p.Edits) != 2 {
		t.Fatalf("both pointers on line 1 relocate, got %+v", p.Edits)
	}
	if len(p.Unmended) != 2 || !strings.Contains(p.Unmended[0], "2 files named notes.md") || !strings.Contains(p.Unmended[1], "gone, not moved") {
		t.Fatalf("ambiguous and gone pointers are named, not guessed: %q", p.Unmended)
	}
	if p.Remedy == "" || props[1].Remedy == "" || len(props[1].Edits) != 0 {
		t.Fatalf("what no diff mends carries a remedy: %+v / %+v", p, props[1])
	}

	patch, err := Patch(root, p.Edits)
	if err != nil {
		t.Fatal(err)
	}
	want := "diff --git a/sub/CLAUDE.md b/sub/CLAUDE.md\n--- a/sub/CLAUDE.md\n+++ b/sub/CLAUDE.md\n@@ -1,4 +1,4 @@\n" +
		"-See [the guide](../docs/guide.md#setup) and `docs/guide.md`.\n" +
		"+See [the guide](../docs/design/guide.md#setup) and `docs/design/guide.md`.\n" +
		" Old notes: `docs/notes.md`.\n Gone: `docs/vanished.md`.\n cp bin/fathom somewhere\n"
	if patch != want {
		t.Fatalf("patch =\n%s\nwant\n%s", patch, want)
	}
}

// TestPatchUnifiedDiffShape: hunks merge when their context touches, a
// file without a trailing newline says so, and a stale edit is refused.
func TestPatchUnifiedDiffShape(t *testing.T) {
	root := t.TempDir()
	lines := []string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"}
	writeTestFile(t, filepath.Join(root, "f.md"), strings.Join(lines, "\n")) // no trailing newline
	patch, err := Patch(root, []Edit{
		{File: "f.md", Line: 2, Old: "l2", New: "L2"},
		{File: "f.md", Line: 6, Old: "l6", New: "L6"},
		{File: "f.md", Line: 15, Old: "l15", New: "L15"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "diff --git a/f.md b/f.md\n--- a/f.md\n+++ b/f.md\n" +
		"@@ -1,9 +1,9 @@\n l1\n-l2\n+L2\n l3\n l4\n l5\n-l6\n+L6\n l7\n l8\n l9\n" +
		"@@ -12,4 +12,4 @@\n l12\n l13\n l14\n-l15\n\\ No newline at end of file\n+L15\n\\ No newline at end of file\n"
	if patch != want {
		t.Fatalf("patch =\n%s\nwant\n%s", patch, want)
	}
	if _, err := Patch(root, []Edit{{File: "f.md", Line: 3, Old: "not l3", New: "x"}}); err == nil {
		t.Fatal("an edit whose Old no longer matches must be refused")
	}
	if data, _ := os.ReadFile(filepath.Join(root, "f.md")); string(data) != strings.Join(lines, "\n") {
		t.Fatal("Patch must never write")
	}
}
