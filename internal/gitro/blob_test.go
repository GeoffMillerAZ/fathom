package gitro

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// blobRepo builds a repository with two commits: first.go differs between
// them, the working tree carries an uncommitted third version, and the
// second commit adds a symlink, a submodule entry and a directory. It
// returns the root and both commit ids.
func blobRepo(t *testing.T) (root, first, second string) {
	t.Helper()
	root = t.TempDir()
	git(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "pkg", "first.go"), "package pkg\n\n// version one\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "one")
	first = strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))

	writeFile(t, filepath.Join(root, "pkg", "first.go"), "package pkg\n\n// version two\n")
	if err := os.Symlink("first.go", filepath.Join(root, "pkg", "link.go")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "update-index", "--add", "--cacheinfo", "160000,"+first+",vendored")
	git(t, root, "commit", "-q", "-m", "two")
	second = strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	git(t, root, "tag", "v2")

	writeFile(t, filepath.Join(root, "pkg", "first.go"), "package pkg\n\n// working tree\n")
	return root, first, second
}

// TestBlobAt_ReadsTheCommitNeverTheWorkingTree: each revision serves its
// own committed bytes and resolves to its commit; the working tree's
// uncommitted edit is never what comes back. A tag peels to its commit, and
// a service rooted in a subdirectory reads its paths relative to it.
func TestBlobAt_ReadsTheCommitNeverTheWorkingTree(t *testing.T) {
	root, first, second := blobRepo(t)
	ctx := context.Background()

	for _, c := range []struct {
		rev, want, commit string
	}{
		{first, "// version one", first},
		{second, "// version two", second},
		{"v2", "// version two", second},
		{first[:12], "// version one", first},
	} {
		data, commit, err := BlobAt(ctx, root, c.rev, "pkg/first.go", 0)
		if err != nil {
			t.Fatalf("BlobAt(%s): %v", c.rev, err)
		}
		if !strings.Contains(string(data), c.want) || strings.Contains(string(data), "working tree") {
			t.Errorf("BlobAt(%s) = %q, want the committed %q", c.rev, data, c.want)
		}
		if commit != c.commit {
			t.Errorf("BlobAt(%s) commit = %s, want %s", c.rev, commit, c.commit)
		}
	}

	data, _, err := BlobAt(ctx, filepath.Join(root, "pkg"), first, "first.go", 0)
	if err != nil || !strings.Contains(string(data), "// version one") {
		t.Errorf("subdirectory root: %q, %v, want version one", data, err)
	}
}

// TestBlobAt_RefusesByName: every refusal is its own named error, and none
// of them falls back to the working tree.
func TestBlobAt_RefusesByName(t *testing.T) {
	root, first, second := blobRepo(t)
	ctx := context.Background()
	notRepo := t.TempDir()
	writeFile(t, filepath.Join(notRepo, "pkg", "first.go"), "package pkg\n")

	for _, c := range []struct {
		name, root, rev, path string
		want                  error
		kind                  string
	}{
		{"empty revision", root, "", "pkg/first.go", ErrNoRevision, ""},
		{"option-shaped revision", root, "--output=/tmp/x", "pkg/first.go", ErrBadRevision, ""},
		{"tree-path revision", root, "HEAD:pkg", "first.go", ErrBadRevision, ""},
		{"unknown revision", root, strings.Repeat("ab", 20), "pkg/first.go", ErrUnknownRevision, ""},
		{"not a repository", notRepo, first, "pkg/first.go", ErrNotRepository, ""},
		{"absolute path", root, first, "/etc/passwd", ErrBadPath, ""},
		{"climbing path", root, first, "../x", ErrBadPath, ""},
		{"absent at revision", root, first, "pkg/link.go", ErrPathAbsent, ""},
		{"symlink", root, second, "pkg/link.go", ErrNotBlob, "symlink"},
		{"submodule", root, second, "vendored", ErrNotBlob, "submodule"},
		{"directory", root, second, "pkg", ErrNotBlob, "directory"},
		{"over the cap", root, second, "pkg/first.go", ErrBlobTooLarge, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			var max int64
			if c.want == ErrBlobTooLarge {
				max = 4
			}
			data, _, err := BlobAt(ctx, c.root, c.rev, c.path, max)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if data != nil {
				t.Errorf("data = %q, want none", data)
			}
			if c.kind != "" {
				var ee *EntryError
				if !errors.As(err, &ee) || ee.Kind != c.kind {
					t.Errorf("err = %v, want an EntryError of kind %q", err, c.kind)
				}
			}
		})
	}
}
