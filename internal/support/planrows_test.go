package support

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanRowMarks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "# P\n\n- [x] done {id: A-1, oracle: lexical, path: x, pattern: y}\n- [ ] open {id: A-2, oracle: lexical, path: x, pattern: y}\n- [-] skipped {id: A-3, disposition: deferred, reason: \"r\", trigger: \"t\"}\n- [-] refused {id: A-4, disposition: refused, reason: \"r\"}\n"
	if err := os.WriteFile(filepath.Join(dir, "p.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := PlanRowMarks(root, "p.md", "missing.md")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]RowMark{"A-1": RowMarkClaimed, "A-2": RowMarkOpen, "A-3": RowMarkDispositioned, "A-4": RowMarkRefused}
	for id, m := range want {
		if got[id] != m {
			t.Errorf("%s = %q, want %q", id, got[id], m)
		}
	}
}

func TestPlanRowMarksReadsLanguageBreadthSiblings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	row := func(box, id string) string {
		return "- [" + box + "] r {id: " + id + ", oracle: lexical, path: x, pattern: y}\n"
	}
	write("language-breadth.md", "# LB\n\n"+row("x", "LB-F1")+row(" ", "LB-DUP"))
	write("language-breadth.queue.json", `{"LB-NOTAROW": 1}`)
	before, err := PlanRowMarks(root, "language-breadth.md")
	if err != nil {
		t.Fatal(err)
	}
	write("language-breadth.b.md", "# LB b\n\n"+row("x", "LB-F90")+row("x", "LB-DUP"))
	write("language-breadth.a.md", "# LB a\n\n"+row(" ", "LB-F91"))
	write("other.md", "# o\n\n"+row("x", "OTHER-1"))
	got, err := PlanRowMarks(root, "language-breadth.md")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]RowMark{"LB-F1": RowMarkClaimed, "LB-DUP": RowMarkOpen, "LB-F90": RowMarkClaimed, "LB-F91": RowMarkOpen}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for id, m := range want {
		if got[id] != m {
			t.Errorf("%s = %q, want %q", id, got[id], m)
		}
	}
	dups, err := PlanRowDuplicates(root, "language-breadth.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 1 || dups[0] != "LB-DUP (language-breadth.b.md)" {
		t.Errorf("dups = %v", dups)
	}
	// No sibling: byte-identical to the main plan alone.
	if err := os.Remove(filepath.Join(dir, "language-breadth.a.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "language-breadth.b.md")); err != nil {
		t.Fatal(err)
	}
	after, err := PlanRowMarks(root, "language-breadth.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) || after["LB-F1"] != before["LB-F1"] || after["LB-DUP"] != before["LB-DUP"] {
		t.Errorf("no-sibling output changed: %v vs %v", after, before)
	}
}
