package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountUsesFilePairUnitAndDoesNotGrantTruth(t *testing.T) {
	root := t.TempDir()
	inputs := map[string]string{
		"one.schema.json":  `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://example.invalid/one","properties":{"a":{"$ref":"two.schema.json"},"b":{"$ref":"two.schema.json"}}}`,
		"two.schema.json":  `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`,
		"zero.schema.json": `{"$schema":"https://json-schema.org/draft/2020-12/schema"}`,
	}
	for name, text := range inputs {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	n, err := count(root, []string{"one.schema.json", "two.schema.json", "zero.schema.json"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pairs=%d, want one despite two sites and identifier", n)
	}
}

func TestCountRefusesUnmeasuredOrDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "value.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, files := range [][]string{{"../outside.json"}, {"value.json", "value.json"}, {"missing.json"}} {
		if _, err := count(root, files); err == nil {
			t.Fatal("invalid membership accepted")
		}
	}
}

// TestCountAcceptsARootReachedThroughASymlink (issue 120): a symlinked
// parent of root is not a symlinked input file, so a root spelled through a
// symlink (a symlinked TMPDIR) still counts; an input that is itself a
// symlink is still refused.
func TestCountAcceptsARootReachedThroughASymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "one.schema.json"), []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","properties":{"a":{"$ref":"two.schema.json"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "two.schema.json"), []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := count(link, []string{"one.schema.json", "two.schema.json"})
	if err != nil || n != 1 {
		t.Fatalf("count(root through a symlink) = %d, %v; want 1, nil", n, err)
	}
	if err := os.Symlink(filepath.Join(real, "two.schema.json"), filepath.Join(real, "alias.schema.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := count(link, []string{"alias.schema.json"}); err == nil || err.Error() != "symlink input" {
		t.Fatalf("a symlinked input file must still be refused, got %v", err)
	}
}
