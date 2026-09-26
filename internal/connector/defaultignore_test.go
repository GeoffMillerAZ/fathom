package connector_test

// defaultignore_test.go — RT-145 (docs/design/responsible-tenant.md §4.2):
// discovery skips the default cache directories on top of skipDir, a
// `[connector] include` in the corpus's own .fathom/config.toml re-includes
// one, and every exclusion is counted and named. App-kit's .cache/ and
// compute/.tools held 43% of its 5.3 GiB tree.

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func discoverPaths(t *testing.T, c *connector.LocalConnector, root string) []string {
	t.Helper()
	refs, err := c.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range refs {
		out = append(out, r.Path)
	}
	return out
}

func TestDiscover_DefaultCacheIgnoresKeepSkipDir(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"src/ok.go",
		".cache/fp/x.js",
		"compute/.tools/bin/y.go",
		"target/debug/z.rs",
		"dist/a.js",
		"build/lib/b.py",
		"pkg/__pycache__/c.py",
		".venv/lib/site.py",
		".gradle/d.java",
		"web/.next/e.js",
		".turbo/f.js",
		"internal/coverage/g.go",
		"vendor/h.go",
	} {
		writeFile(t, filepath.Join(root, p), "x\n")
	}

	c := connector.NewLocalConnector()
	if got, want := discoverPaths(t, c, root), []string{"src/ok.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover = %v, want %v (default cache dirs and skipDir both pruned)", got, want)
	}
	ex := c.Exclusions("svc")
	wantDirs := []string{".cache", ".gradle", ".turbo", ".venv", "build", "compute/.tools", "dist", "internal/coverage", "pkg/__pycache__", "target", "web/.next"}
	if !reflect.DeepEqual(ex.DefaultIgnored, wantDirs) {
		t.Fatalf("Exclusions.DefaultIgnored = %v, want %v", ex.DefaultIgnored, wantDirs)
	}
	if ex.SkippedDirs != 1 { // vendor
		t.Fatalf("Exclusions.SkippedDirs = %d, want 1 (vendor)", ex.SkippedDirs)
	}

	// `[connector] include` re-includes by directory name or by
	// repo-relative path; it never re-includes a skipDir name.
	writeFile(t, filepath.Join(root, ".fathom", "config.toml"), "[connector]\ninclude = [\"coverage\", \"compute/.tools\", \"vendor\"]\n")
	got := discoverPaths(t, c, root)
	want := []string{".fathom/config.toml", "compute/.tools/bin/y.go", "internal/coverage/g.go", "src/ok.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover with include = %v, want %v", got, want)
	}
	if ex := c.Exclusions("svc"); len(ex.DefaultIgnored) != len(wantDirs)-2 {
		t.Fatalf("after include, DefaultIgnored = %v; the two re-included dirs must drop out", ex.DefaultIgnored)
	}
}
