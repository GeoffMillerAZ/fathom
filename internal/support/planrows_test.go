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
