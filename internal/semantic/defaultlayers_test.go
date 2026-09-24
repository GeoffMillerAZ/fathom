package semantic

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFile is a small test helper: creates path (and its parent dirs)
// under dir with the given content.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", full, err)
	}
}

// TestDefaultLayersFor_CorpusPresence is CV-41 (docs/plans/
// refactor-conservation.md, docs/design/refactor-conservation.md §6
// "Activation"): DefaultLayersFor(root) always includes "obligations" (the
// v1 tracer bullet, on for every fleet repo per its own catalog entry,
// docs/layers.md), plus "adr-rules" and "spec-bindings" whenever their own
// corpus globs (read from the layer catalog's layer.cue, not hardcoded
// here) match at least one file under root.
func TestDefaultLayersFor_CorpusPresence(t *testing.T) {
	t.Run("neither corpus present: obligations alone", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "README.md", "nothing relevant here")
		got := DefaultLayersFor(root)
		want := []string{LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v", got, want)
		}
	})

	t.Run("adr-rules corpus present via ADR.md: adr-rules joins the set", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "ADR.md", "# Decisions\n")
		got := DefaultLayersFor(root)
		want := []string{"adr-rules", LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v", got, want)
		}
	})

	t.Run("adr-rules corpus present via docs/design/*.md: adr-rules joins the set", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "docs/design/refactor-conservation.md", "# Design\n")
		got := DefaultLayersFor(root)
		want := []string{"adr-rules", LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v", got, want)
		}
	})

	t.Run("spec-bindings corpus present via docs/plans/*.md: spec-bindings joins the set", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "docs/plans/refactor-conservation.md", "- [ ] a row\n")
		got := DefaultLayersFor(root)
		want := []string{LayerObligations, "spec-bindings"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v", got, want)
		}
	})

	t.Run("spec-bindings corpus excludes README.md: the exclude glob is honored, not just the include", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "docs/plans/README.md", "the grammar, not a gated task\n")
		got := DefaultLayersFor(root)
		want := []string{LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v (docs/plans/README.md is excluded from spec-bindings' corpus)", got, want)
		}
	})

	t.Run("both corpora present: the full default set, sorted", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "ADR.md", "# Decisions\n")
		writeFile(t, root, "docs/plans/refactor-conservation.md", "- [ ] a row\n")
		got := DefaultLayersFor(root)
		want := []string{"adr-rules", LayerObligations, "spec-bindings"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor = %v, want %v", got, want)
		}
	})

	t.Run("empty root: obligations alone, never a guess at an unreadable corpus", func(t *testing.T) {
		got := DefaultLayersFor("")
		want := []string{LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor(\"\") = %v, want %v", got, want)
		}
	})

	t.Run("nonexistent root: obligations alone, no panic", func(t *testing.T) {
		got := DefaultLayersFor(filepath.Join(t.TempDir(), "does-not-exist"))
		want := []string{LayerObligations}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultLayersFor(nonexistent) = %v, want %v", got, want)
		}
	})
}
