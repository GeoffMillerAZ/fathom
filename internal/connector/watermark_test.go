package connector_test

// watermark_test.go — the Watermark SPI (ADR-017: freshness is domain-neutral).
// Non-git corpora get a content-addressed manifest watermark that is stable
// across directory moves (the portability property) and changes iff content
// changes. Repo-relative Fetch resolution is exercised alongside, since the
// manifest is computed through Discover+Fetch.

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// gitProofTempDir returns t.TempDir() after pinning GIT_CEILING_DIRECTORIES to
// its parent, so git's upward discovery can never escape into an enclosing
// repository — the non-git premise must hold even when TMPDIR points inside a
// git work tree (the Taskfile routes TMPDIR to workspace/tmp/gobuild).
func gitProofTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	parent := filepath.Dir(dir)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	return dir
}

func TestLocalConnector_Watermark_ManifestStableAcrossMove(t *testing.T) {
	ctx := context.Background()
	lc := connector.NewLocalConnector()

	rootA := gitProofTempDir(t)
	writeFile(t, filepath.Join(rootA, "runbook.md"), "# restart the daemon\n")
	writeFile(t, filepath.Join(rootA, "findings", "f1.md"), "severity: high\n")

	wmA, err := lc.Watermark(ctx, core.ServiceRef{ID: "mod", Path: rootA})
	if err != nil {
		t.Fatalf("Watermark A: %v", err)
	}
	if !strings.HasPrefix(wmA, "manifest:") {
		t.Fatalf("non-git watermark = %q, want manifest: prefix", wmA)
	}

	// Same content at a different location (a moved/copied corpus) must
	// produce the identical watermark — location never leaks into freshness.
	rootB := t.TempDir()
	writeFile(t, filepath.Join(rootB, "runbook.md"), "# restart the daemon\n")
	writeFile(t, filepath.Join(rootB, "findings", "f1.md"), "severity: high\n")

	wmB, err := lc.Watermark(ctx, core.ServiceRef{ID: "mod", Path: rootB})
	if err != nil {
		t.Fatalf("Watermark B: %v", err)
	}
	if wmA != wmB {
		t.Errorf("watermark changed across a pure move: %q vs %q", wmA, wmB)
	}

	// Content change ⇒ watermark change.
	writeFile(t, filepath.Join(rootB, "runbook.md"), "# kickstart the daemon\n")
	wmC, err := lc.Watermark(ctx, core.ServiceRef{ID: "mod", Path: rootB})
	if err != nil {
		t.Fatalf("Watermark C: %v", err)
	}
	if wmC == wmB {
		t.Error("watermark did not change after content edit")
	}
}

func TestLocalConnector_Fetch_ResolvesRelativeRefs(t *testing.T) {
	ctx := context.Background()
	lc := connector.NewLocalConnector()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pkg", "a.go"), "package a\n")

	refs, err := lc.Discover(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("Discover returned %d refs, want 1", len(refs))
	}
	if refs[0].Path != "pkg/a.go" {
		t.Fatalf("ref.Path = %q, want repo-relative %q", refs[0].Path, "pkg/a.go")
	}
	data, err := lc.Fetch(ctx, refs[0])
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(data) != "package a\n" {
		t.Errorf("Fetch content = %q", data)
	}
}

func TestLocalConnector_Fetch_LegacyAbsoluteRef(t *testing.T) {
	// Stores written before ADR-017 carry absolute anchors; Fetch must keep
	// reading them without a preceding Discover.
	root := t.TempDir()
	abs := filepath.Join(root, "old.go")
	writeFile(t, abs, "package old\n")

	lc := connector.NewLocalConnector()
	data, err := lc.Fetch(context.Background(), core.SourceRef{Service: "svc", Path: abs})
	if err != nil {
		t.Fatalf("Fetch(absolute): %v", err)
	}
	if string(data) != "package old\n" {
		t.Errorf("Fetch content = %q", data)
	}
}

func TestLocalConnector_Watermark_GitRepoUsesHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "."},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	wm, err := connector.NewLocalConnector().Watermark(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Watermark: %v", err)
	}
	if strings.HasPrefix(wm, "manifest:") || len(wm) != 40 {
		t.Errorf("git-backed watermark = %q, want bare 40-char HEAD SHA", wm)
	}
}
