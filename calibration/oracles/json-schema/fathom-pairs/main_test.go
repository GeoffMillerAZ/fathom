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
