package hookcache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

func TestBustIdent_RemovesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fathom-ident-cache")
	orig := hookcache.IdentDir
	hookcache.IdentDir = dir
	t.Cleanup(func() { hookcache.IdentDir = orig })

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entry.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	hookcache.BustIdent()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err = %v", dir, err)
	}
}

func TestBustIdent_AbsentDirectory_NoError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	orig := hookcache.IdentDir
	hookcache.IdentDir = dir
	t.Cleanup(func() { hookcache.IdentDir = orig })

	hookcache.BustIdent() // must not panic on an absent directory
}
