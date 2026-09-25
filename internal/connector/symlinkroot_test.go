package connector_test

// symlinkroot_test.go — O39: a service root whose LAST path component is a
// symlink (a checkout reached through a link, or a root recorded in its
// linked form) must be walked like the directory it points at.
// filepath.WalkDir Lstat's its root, so a symlinked root read as a
// non-directory and Discover returned zero files with a nil error: ingest
// through it indexed nothing (exit 0), and `references` — the one tool
// that re-reads sources at query time — found zero occurrences of symbols
// verify_plan answered from the index.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestLocalConnector_Discover_SymlinkedRootMatchesItsTarget(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	for rel, body := range map[string]string{
		"go.mod":       "module example.com/x\n",
		"pkg/a.go":     "package pkg\n\nfunc A() {}\n",
		"pkg/b.go":     "package pkg\n\nfunc B() { A() }\n",
		"docs/note.md": "# note\n",
	} {
		full := filepath.Join(real, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	ctx := context.Background()
	lc := connector.NewLocalConnector()
	want, err := lc.Discover(ctx, core.ServiceRef{ID: "real", Path: real})
	if err != nil {
		t.Fatalf("Discover(real): %v", err)
	}
	if len(want) != 4 {
		t.Fatalf("Discover(real) = %d refs, want 4: %+v", len(want), want)
	}

	for _, root := range []string{link, link + string(filepath.Separator)} {
		got, err := lc.Discover(ctx, core.ServiceRef{ID: "real", Path: root})
		if err != nil {
			t.Fatalf("Discover(%s): %v", root, err)
		}
		var gotPaths, wantPaths []string
		for _, r := range got {
			gotPaths = append(gotPaths, r.Path)
		}
		for _, r := range want {
			wantPaths = append(wantPaths, r.Path)
		}
		if !reflect.DeepEqual(gotPaths, wantPaths) {
			t.Errorf("Discover(%s) paths = %v, want the target's own %v (repo-relative, never through the link's name)", root, gotPaths, wantPaths)
			continue
		}
		for _, r := range got {
			if data, err := lc.Fetch(ctx, r); err != nil || len(data) == 0 {
				t.Errorf("Fetch(%s) through %s: %v (%d bytes)", r.Path, root, err, len(data))
			}
		}
	}
}
