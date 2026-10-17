package custodian

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

// TestProposeNeverEditsInsideFathomMarkers is law 15 / I13 for propose: a
// dead pointer or script inside fathom's own marker regions — the stanza
// (a digest-bearing v3 region `fleet reconcile` refuses once hand-edited) — is never
// offered a diff. It is listed unmended with the regenerating remedy; the
// same dead pointer outside the markers is still mended.
func TestProposeNeverEditsInsideFathomMarkers(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "vision", "archive", "27-strengths-bench.md"), "x\n")
	writeTestFile(t, filepath.Join(root, "docs", "intent", "archive", "scope.md"), "x\n")
	writeTestFile(t, filepath.Join(root, "scripts", "ci", "check.sh"), "#!/bin/sh\n")
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), strings.Join([]string{
		"# router", // 1
		"See `docs/vision/27-strengths-bench.md`.",  // 2: mendable
		"<!-- fathom:begin v3 sha256:abc -->",       // 3
		"Cite `docs/vision/27-strengths-bench.md`.", // 4: fathom's
		"Gate: `bash scripts/check.sh`.",            // 5: fathom's
		"<!-- fathom:end -->",                       // 6
		"",
	}, "\n"))
	fs := Audit(root, []Concern{
		{ID: "pointers_hold", Want: "pointers resolve", Check: Check{Kind: KindPointersResolve}},
		{ID: "cmds", Want: "commands resolve", Check: Check{Kind: KindCommandsResolve}},
	})
	props, err := Propose(root, fs)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 2 {
		t.Fatalf("both concerns are violated: %+v", props)
	}
	ptr, cmd := props[0], props[1]
	if len(ptr.Edits) != 1 || ptr.Edits[0].File != "CLAUDE.md" || ptr.Edits[0].Line != 2 {
		t.Fatalf("only the pointer outside the markers is mended: %+v", ptr.Edits)
	}
	if len(ptr.Unmended) != 1 ||
		!strings.HasPrefix(ptr.Unmended[0], "CLAUDE.md:4 → docs/vision/27-strengths-bench.md: inside fathom's stanza region (CLAUDE.md:3)") ||
		!strings.Contains(ptr.Unmended[0], "fathom fleet reconcile --repo "+root+" --apply") {
		t.Fatalf("the pointers inside fathom's markers are named with their remedy: %q", ptr.Unmended)
	}
	if !strings.Contains(ptr.Remedy, "never hand-edit") {
		t.Errorf("the remedy covers the marker regions: %q", ptr.Remedy)
	}
	if len(cmd.Edits) != 0 || len(cmd.Unmended) != 1 || !strings.Contains(cmd.Unmended[0], "inside fathom's stanza region") {
		t.Fatalf("a script inside the stanza is not mended either: %+v", cmd)
	}
	patch, err := Patch(root, append(ptr.Edits, cmd.Edits...))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patch, "fathom:begin") && strings.Contains(patch, "+Cite") {
		t.Fatalf("the patch must not touch the stanza body:\n%s", patch)
	}
}

// TestProposeIndexFollowsTheWorkingTree: relocation candidates are what
// is on disk, on git's authority — an ordinary unstaged `mv` is a move
// (not "ambiguous" with the path it left), a deleted-unstaged target is
// gone (not relocated onto itself), and a non-ASCII name is matched
// (git quotes it without -z).
func TestProposeIndexFollowsTheWorkingTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "guide.md"), "# guide\n")
	writeTestFile(t, filepath.Join(root, "docs", "old.md"), "# old\n")
	writeTestFile(t, filepath.Join(root, "docs", "design", "café.md"), "# café\n")
	writeTestFile(t, filepath.Join(root, "CLAUDE.md"), "See [the guide](docs/guide.md), [old](docs/old.md) and [café](docs/café.md).\n")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "init")
	// An ordinary move and a delete, neither staged.
	if err := os.MkdirAll(filepath.Join(root, "docs", "guides"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "docs", "guide.md"), filepath.Join(root, "docs", "guides", "guide.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "docs", "old.md")); err != nil {
		t.Fatal(err)
	}

	props, err := Propose(root, Audit(root, []Concern{{ID: "p", Want: "resolve", Check: Check{Kind: KindPointersResolve}}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 {
		t.Fatalf("one violated concern: %+v", props)
	}
	var whys []string
	for _, e := range props[0].Edits {
		whys = append(whys, e.Why)
	}
	want := []string{
		"docs/guide.md moved to docs/guides/guide.md (the only file named guide.md)",
		"docs/café.md moved to docs/design/café.md (the only file named café.md)",
	}
	if !reflect.DeepEqual(whys, want) {
		t.Fatalf("edits %q, want %q (unmended: %q)", whys, want, props[0].Unmended)
	}
	if len(props[0].Unmended) != 1 || !strings.Contains(props[0].Unmended[0], "no file named old.md anywhere in the repo") {
		t.Fatalf("a deleted target is gone, not relocated onto itself: %q", props[0].Unmended)
	}
}
