package mcp_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func refGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// refCorpus builds a service corpus that is a git repository: pkg/foo.go
// at two commits (tag "v1" on the first), and at the second a symlink, a
// submodule entry, a .env file, a vendored file, a binary and a file over
// the read ceiling. The working tree then carries an uncommitted edit of
// pkg/foo.go and has the over-ceiling file and the binary removed.
func refCorpus(t *testing.T) (root, first, second string) {
	t.Helper()
	root = t.TempDir()
	refGit(t, root, "init", "-q")
	writeCorpusFile(t, root, "pkg/foo.go", "package pkg\n// committed one\n")
	refGit(t, root, "add", ".")
	refGit(t, root, "commit", "-q", "-m", "one")
	first = refGit(t, root, "rev-parse", "HEAD")
	refGit(t, root, "tag", "v1")

	writeCorpusFile(t, root, "pkg/foo.go", "package pkg\n// committed two\n// line three\n")
	writeCorpusFile(t, root, ".env", "SECRET=1\n")
	writeCorpusFile(t, root, "vendor/dep/dep.go", "package dep\n")
	writeCorpusFile(t, root, "bin.dat", "PK\x00\x00binary-ish-content")
	writeCorpusFile(t, root, "big.txt", strings.Repeat("x", 2*1024*1024+10))
	if err := os.Symlink("foo.go", filepath.Join(root, "pkg", "link.go")); err != nil {
		t.Fatal(err)
	}
	refGit(t, root, "add", "-f", ".")
	refGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
	refGit(t, root, "commit", "-q", "-m", "two")
	second = refGit(t, root, "rev-parse", "HEAD")

	writeCorpusFile(t, root, "pkg/foo.go", "package pkg\n// working tree edit\n")
	for _, f := range []string{"big.txt", "bin.dat"} {
		if err := os.Remove(filepath.Join(root, f)); err != nil {
			t.Fatal(err)
		}
	}
	return root, first, second
}

// TestReadSourceRefServesBlobBytes (WAE-3): read_source with `ref` serves
// the file's bytes at that commit through git, never the working tree,
// and names the commit it read; without `ref` it reads the working tree
// exactly as before. A file the working tree no longer has is still read
// at a ref that has it.
func TestReadSourceRefServesBlobBytes(t *testing.T) {
	root, first, second := refCorpus(t)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	for _, c := range []struct {
		ref, commit string
		want        []string
	}{
		{first, first, []string{"package pkg", "// committed one"}},
		{"v1", first, []string{"package pkg", "// committed one"}},
		{second, second, []string{"package pkg", "// committed two", "// line three"}},
		{"HEAD", second, []string{"package pkg", "// committed two", "// line three"}},
	} {
		p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "ref": c.ref})
		if got := lineTexts(t, p); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("ref %s: lines = %q, want %q (the blob at the commit, never the working tree)", c.ref, got, c.want)
		}
		if p["ref"] != c.ref || p["commit"] != c.commit {
			t.Errorf("ref %s: ref=%v commit=%v, want ref %s commit %s", c.ref, p["ref"], p["commit"], c.ref, c.commit)
		}
		if p["total_lines"] != float64(len(c.want)) {
			t.Errorf("ref %s: total_lines = %v, want %d", c.ref, p["total_lines"], len(c.want))
		}
	}

	// A range at a ref is sliced exactly as a working-tree read is.
	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go", "ref": second, "start_line": 2, "end_line": 2})
	if got := lineTexts(t, p); len(got) != 1 || got[0] != "// committed two" || p["start_line"] != float64(2) {
		t.Errorf("ranged ref read = %v (start %v), want line 2 of the commit", got, p["start_line"])
	}

	// Without ref: the working tree, and no ref/commit keys.
	p = readSourceCall(t, srv, map[string]any{"service": "svc", "file": "pkg/foo.go"})
	if got := lineTexts(t, p); len(got) != 2 || got[1] != "// working tree edit" {
		t.Errorf("default read = %q, want the working tree", got)
	}
	if _, ok := p["ref"]; ok {
		t.Errorf("default read carries ref: %v", p)
	}
	if _, ok := p["commit"]; ok {
		t.Errorf("default read carries commit: %v", p)
	}
}

// TestReadSourceRefFiltersNameTheirFilter: every filter a working-tree
// read applies holds at a ref, and the refusal names the filter that hit;
// a ref shaped like an option, a symlink entry and a submodule entry are
// refused at a ref, never followed.
func TestReadSourceRefFiltersNameTheirFilter(t *testing.T) {
	root, _, second := refCorpus(t)
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	for _, c := range []struct {
		name, file, ref, want string
	}{
		{"option-shaped ref", "pkg/foo.go", "--output=/tmp/x", "may not start with '-'"},
		{"absolute path", "/etc/passwd", second, "absolute"},
		{"traversal", "../secret.txt", second, "escapes"},
		{"env file", ".env", second, ".env-pattern"},
		{"skipped dir", "vendor/dep/dep.go", second, "always excluded"},
		{"binary", "bin.dat", second, "binary"},
		{"over the ceiling", "big.txt", second, "read ceiling"},
		{"symlink", "pkg/link.go", second, "symlink"},
		{"submodule", "sub", second, "submodule"},
		{"absent at ref", "pkg/none.go", second, "not found at ref"},
		{"unknown ref", "pkg/foo.go", strings.Repeat("ef", 20), "names no commit"},
	} {
		t.Run(c.name, func(t *testing.T) {
			errObj := readSourceCallError(t, srv, map[string]any{"service": "svc", "file": c.file, "ref": c.ref})
			msg, _ := errObj["message"].(string)
			if !strings.Contains(msg, c.want) {
				t.Errorf("message = %q, want it to name %q", msg, c.want)
			}
		})
	}
}
