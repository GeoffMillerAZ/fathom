package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func copyTypedFixture(t *testing.T) string {
	t.Helper()
	src := typedFixtureDir(t)
	dst := t.TempDir()
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func allowedExcept(t *testing.T, dir, skip string) map[string]bool {
	t.Helper()
	names, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	allowed := map[string]bool{}
	for _, n := range names {
		if filepath.Base(n) != skip {
			allowed[filepath.Base(n)] = true
		}
	}
	return allowed
}

// A typed edge into a file excluded by a declared policy (.fathomignore)
// is policy-excluded, not a discovery gap; with no policy it stays a gap.
func TestTypedEdgesPolicyExcludedIsNotADiscoveryGap(t *testing.T) {
	dir := copyTypedFixture(t)
	if err := os.WriteFile(filepath.Join(dir, ".fathomignore"), []byte("adapter.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := fusion.EnrichTypedEdges(context.Background(), store.NewMemoryStore(), "pol", dir, allowedExcept(t, dir, "adapter.go"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.DiscoveryGaps != 0 || len(rep.DiscoveryGapFiles) != 0 {
		t.Errorf("DiscoveryGaps = %d %v, want 0", rep.DiscoveryGaps, rep.DiscoveryGapFiles)
	}
	if rep.PolicyExcluded != 1 {
		t.Errorf("PolicyExcluded = %d, want 1", rep.PolicyExcluded)
	}
	if rep.EdgesDroppedOutOfScope == 0 {
		t.Error("EdgesDroppedOutOfScope = 0, the drops are still counted")
	}
}

func TestTypedEdgesUndeclaredExclusionStaysAGap(t *testing.T) {
	dir := copyTypedFixture(t)
	rep, err := fusion.EnrichTypedEdges(context.Background(), store.NewMemoryStore(), "pol", dir, allowedExcept(t, dir, "adapter.go"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.DiscoveryGaps != 1 || rep.PolicyExcluded != 0 {
		t.Errorf("DiscoveryGaps = %d, PolicyExcluded = %d, want 1, 0", rep.DiscoveryGaps, rep.PolicyExcluded)
	}
}
