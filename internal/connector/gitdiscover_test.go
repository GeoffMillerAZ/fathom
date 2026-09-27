package connector_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// gitRepo builds a work tree: committed files (vendor/ and testdata/
// among them), untracked files, and .gitignore'd ones.
func gitRepo(t *testing.T, connectorTOML string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	files := map[string]string{
		"main.go":             "package main\n",
		"pkg/a.go":            "package pkg\n",
		"vendor/dep/dep.go":   "package dep\n",
		"testdata/fix/fix.go": "package fix\n",
		".gitignore":          "tmpout/\ngen/\n*.log\n",
		"tmpout/out.go":       "package out\n",
		"gen/keep.go":         "package gen\n",
		"trace.log":           "log\n",
		"untracked/new.go":    "package untracked\n",
		".cache/c.go":         "package c\n",
		".fathom/config.toml": connectorTOML,
	}
	for p, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "main.go", "pkg/a.go", "vendor", "testdata", ".gitignore", ".fathom")
	git("commit", "-q", "-m", "init")
	return root
}

// TestDiscover_HonoursGitignoreAndKeepsSkipDir is RT-68 (§4.2): with
// `[connector] respect_gitignore = true`, discovery lists a git work tree
// through `git ls-files -co --exclude-standard`, so gitignored files never
// enter the corpus — while skipDir and the default cache list still apply
// to what git returns (a COMMITTED vendor/ or testdata/ stays out),
// `include` re-includes a gitignored directory, and every exclusion is
// counted. Without the opt-in (the default until RT-M11), the walk is
// unchanged.
func TestDiscover_HonoursGitignoreAndKeepsSkipDir(t *testing.T) {
	ctx := context.Background()
	root := gitRepo(t, "[connector]\nrespect_gitignore = true\ninclude = [\"gen\"]\n")
	c := connector.NewLocalConnector()
	refs, err := c.Discover(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.Path)
	}
	want := []string{".fathom/config.toml", ".gitignore", "gen/keep.go", "main.go", "pkg/a.go", "untracked/new.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("git discovery = %v\nwant %v", got, want)
	}
	ex := c.Exclusions("svc")
	if !ex.Git || ex.GitIgnored < 2 || ex.SkippedDirs != 2 || !reflect.DeepEqual(ex.DefaultIgnored, []string{".cache"}) {
		t.Fatalf("exclusions = %+v, want git, ≥2 gitignored (tmpout/, trace.log), vendor+testdata skipped, .cache default-ignored", ex)
	}

	// Default: the walk, exactly as before (it sees tmpout/ and trace.log,
	// which it has no way to know are ignored).
	off := gitRepo(t, "[connector]\ninclude = [\"gen\"]\n")
	walked, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: "svc2", Path: off})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, r := range walked {
		paths = append(paths, r.Path)
	}
	has := map[string]bool{}
	for _, p := range paths {
		has[p] = true
	}
	if !has["tmpout/out.go"] || !has["trace.log"] || has["vendor/dep/dep.go"] || has[".cache/c.go"] {
		t.Fatalf("default walk changed: %v", paths)
	}
}

// TestExcludedFromCorpus_GitignoredUnderRespectGitignore: the refresh path
// never indexes a file git discovery would leave out.
func TestExcludedFromCorpus_GitignoredUnderRespectGitignore(t *testing.T) {
	root := gitRepo(t, "[connector]\nrespect_gitignore = true\ninclude = [\"gen\"]\n")
	for rel, want := range map[string]bool{"tmpout/out.go": true, "trace.log": true, "gen/keep.go": false, "untracked/new.go": false, "main.go": false} {
		_, excluded, err := connector.ExcludedFromCorpus(root, "svc", rel)
		if err != nil || excluded != want {
			t.Errorf("ExcludedFromCorpus(%s) = %v, %v; want %v", rel, excluded, err, want)
		}
	}
}

// runGit runs git in dir with a fixed identity.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func gitDiscoverPaths(t *testing.T, root string) []string {
	t.Helper()
	refs, err := connector.NewLocalConnector().Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, r.Path)
	}
	return got
}

// TestDiscover_GitignoredCorpusAndNestedRepos (review finding, RT-68):
// git discovery never empties a corpus. A corpus that an enclosing
// repository's .gitignore excludes as a whole is walked (that repository's
// rules do not govern it — and an empty discovery would make the ingest
// purge every stored row), and the refresh path agrees; a nested
// repository, which git lists as one directory entry, is listed through
// its own git rather than silently dropped.
func TestDiscover_GitignoredCorpusAndNestedRepos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	parent := t.TempDir()
	writeFiles(t, parent, map[string]string{
		".gitignore":              "svc/\n",
		"top.go":                  "package top\n",
		"svc/.fathom/config.toml": "[connector]\nrespect_gitignore = true\n",
		"svc/pkg/a.go":            "package pkg\n",
	})
	runGit(t, parent, "init", "-q")
	runGit(t, parent, "add", ".gitignore", "top.go")
	runGit(t, parent, "commit", "-q", "-m", "init")
	svc := filepath.Join(parent, "svc")
	got := gitDiscoverPaths(t, svc)
	want := []string{".fathom/config.toml", "pkg/a.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery of a corpus its enclosing repo ignores = %v, want %v", got, want)
	}
	if _, excluded, err := connector.ExcludedFromCorpus(svc, "svc", "pkg/a.go"); err != nil || excluded {
		t.Fatalf("refresh of pkg/a.go in an enclosing-ignored corpus: excluded=%v err=%v", excluded, err)
	}

	// A nested repository inside a respect_gitignore corpus.
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		".fathom/config.toml": "[connector]\nrespect_gitignore = true\n",
		"pkg/a.go":            "package pkg\n",
		"nested/b.go":         "package nested\n",
		"nested/.gitignore":   "out/\n",
		"nested/out/x.go":     "package out\n",
	})
	runGit(t, filepath.Join(root, "nested"), "init", "-q")
	runGit(t, filepath.Join(root, "nested"), "add", "b.go", ".gitignore")
	runGit(t, filepath.Join(root, "nested"), "commit", "-q", "-m", "init")
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", ".fathom", "pkg")
	runGit(t, root, "commit", "-q", "-m", "init")
	got = gitDiscoverPaths(t, root)
	want = []string{".fathom/config.toml", "nested/.gitignore", "nested/b.go", "pkg/a.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovery with a nested repository = %v, want %v", got, want)
	}
}
