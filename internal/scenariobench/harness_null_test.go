package scenariobench

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewNullHarness(t *testing.T) {
	h := NewNullHarness()
	if h.Name() != "null" || h.Version() != "null-1" {
		t.Fatalf("harness identity = %q/%q, want null/null-1", h.Name(), h.Version())
	}
	worktree, rawDir := t.TempDir(), t.TempDir()
	path := filepath.Join(worktree, "source.go")
	const original = "package fixture\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, role := range []string{"single", "planner", "worker"} {
		req := SessionRequest{
			Worktree: worktree, RawDir: rawDir,
			Prompt: "Edit source.go and answer with the changed file.",
			Index:  i, Role: role, RowID: "BX-fixture",
		}
		result, err := h.RunSession(req)
		if err != nil {
			t.Fatal(err)
		}
		want := SessionResult{Index: req.Index, Role: req.Role, RowID: req.RowID}
		if !reflect.DeepEqual(result, want) {
			t.Errorf("%s session = %+v, want identity only: %+v", role, result, want)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatalf("source changed: %q, err=%v", data, err)
	}
	entries, err := os.ReadDir(worktree)
	if err != nil || len(entries) != 1 {
		t.Errorf("worktree entries = %v, err=%v, want only source.go", entries, err)
	}
	entries, err = os.ReadDir(rawDir)
	if err != nil || len(entries) != 0 {
		t.Errorf("raw entries = %v, err=%v, want none", entries, err)
	}
}
