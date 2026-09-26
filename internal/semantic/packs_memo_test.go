package semantic

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestLoadUnionMemoizedPerDigest is RT-83 (docs/plans/responsible-tenant.md,
// design §4.3): the obligations hot path resolved the pack union through
// loadUnion on every call, and every call built a fresh CUE context and
// recompiled #Layer plus every pack. The union is now compiled once per
// (root, digest of root/.fathom/layers) — a repeat call, or eight
// concurrent ones, builds nothing; editing a project pack or resolving a
// different root builds exactly once more.
func TestLoadUnionMemoizedPerDigest(t *testing.T) {
	resetUnionMemoForTest()
	root := t.TempDir()
	writeFixturePack(t, filepath.Join(root, ".fathom", "layers", "custom"), "custom", false)

	before := unionBuildCount()
	if _, err := LoadLayerIn(root, "custom"); err != nil {
		t.Fatalf("LoadLayerIn: %v", err)
	}
	if got := unionBuildCount() - before; got != 1 {
		t.Fatalf("first call built the union %d times, want 1", got)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := LoadLayerIn(root, LayerObligations); err != nil {
				t.Errorf("LoadLayerIn: %v", err)
			}
			if _, err := ListLayersIn(root); err != nil {
				t.Errorf("ListLayersIn: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := unionBuildCount() - before; got != 1 {
		t.Fatalf("repeat and concurrent calls at the same digest built the union %d times in all, want 1", got)
	}

	// A project pack edit moves the digest: exactly one rebuild, and the
	// rebuilt union serves the edit.
	cue := filepath.Join(root, ".fathom", "layers", "custom", "layer.cue")
	data, err := os.ReadFile(cue)
	if err != nil {
		t.Fatal(err)
	}
	edited := append([]byte(nil), data...)
	edited = append(edited, []byte("\nrole: \"claims\"\n")...)
	if err := os.WriteFile(cue, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	// Belt and braces for coarse-mtime filesystems: the size also moved.
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(cue, future, future)
	def, err := LoadLayerIn(root, "custom")
	if err != nil {
		t.Fatalf("LoadLayerIn after edit: %v", err)
	}
	if def.Role != "claims" {
		t.Errorf("after edit Role = %q, want the edited pack's claims", def.Role)
	}
	if got := unionBuildCount() - before; got != 2 {
		t.Fatalf("after a pack edit the union was built %d times in all, want 2", got)
	}

	// A different root is its own key.
	if _, err := LoadLayerIn(t.TempDir(), LayerObligations); err != nil {
		t.Fatalf("LoadLayerIn other root: %v", err)
	}
	if got := unionBuildCount() - before; got != 3 {
		t.Fatalf("a second root built the union %d times in all, want 3", got)
	}
	if _, err := LoadLayerIn(root, "custom"); err != nil {
		t.Fatal(err)
	}
	if got := unionBuildCount() - before; got != 3 {
		t.Fatalf("returning to the first root rebuilt the union (%d builds), want the memo to hold both roots", got)
	}
}
