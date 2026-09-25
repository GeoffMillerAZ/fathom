package client

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSCIPIndexKey_StableAcrossRootsAndTempPaths pins scipIndexKey's
// identity rule (O116 residual): inside the service root a SCIP index is
// keyed by its root-relative path, so the same repository ingested from
// another checkout or a remapped root reconciles against the same
// namespace; outside the root it is keyed by base name, so an explicit
// --scip into a fresh temp directory each run does too.
func TestSCIPIndexKey_StableAcrossRootsAndTempPaths(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	for _, r := range []string{rootA, rootB} {
		if err := os.MkdirAll(filepath.Join(r, ".fathom", "scip"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(rootA, link); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, root, path, want string
	}{
		{"root index", rootA, filepath.Join(rootA, "index.scip"), "index.scip"},
		{"same repo, other checkout", rootB, filepath.Join(rootB, "index.scip"), "index.scip"},
		{"nested index", rootA, filepath.Join(rootA, ".fathom", "scip", "ts.scip"), ".fathom/scip/ts.scip"},
		{"root reached through a symlink", link, filepath.Join(rootA, ".fathom", "scip", "ts.scip"), ".fathom/scip/ts.scip"},
		{"outside the root", rootA, filepath.Join(t.TempDir(), "zod.scip"), "zod.scip"},
		{"sibling with a shared name prefix", rootA, rootA + "-other/index.scip", "index.scip"},
		{"no root recorded", "", filepath.Join(rootA, "index.scip"), "index.scip"},
	}
	for _, c := range cases {
		if got := scipIndexKey(c.root, c.path); got != c.want {
			t.Errorf("%s: scipIndexKey(%q, %q) = %q, want %q", c.name, c.root, c.path, got, c.want)
		}
	}
}
