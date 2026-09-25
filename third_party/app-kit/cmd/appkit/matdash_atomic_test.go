package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMatdashBuildPreservesExistingIndexOnInvalidSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	out := filepath.Join(dir, "components.sqlite")
	if err := os.MkdirAll(filepath.Join(src, "components"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "components", "Example.tsx"), []byte(`export default function Example(){return <div className="p-4">Example</div>}`), 0644); err != nil {
		t.Fatal(err)
	}
	want := []byte("existing usable index is preserved byte-for-byte")
	if err := os.WriteFile(out, want, 0644); err != nil {
		t.Fatal(err)
	}
	if err := dataMatdashBuild(context.Background(), []string{"-src", src, "-out", out}); err == nil {
		t.Fatal("missing source manifest must refuse build")
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("failed build damaged prior index")
	}
	stages, err := filepath.Glob(filepath.Join(dir, ".matdash-*.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 0 {
		t.Fatalf("staging files leaked: %v", stages)
	}
}
