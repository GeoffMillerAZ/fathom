package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestGitServiceNeverTakesManifestPath is RT-110 (design §6, ADR-017
// amended): a service whose stored watermark is a git SHA gets its HEAD from
// the ref files, and when HEAD cannot be read the answer is freshness
// unknown — never the content manifest, which reads the whole corpus. A
// genuine non-git corpus still gets its content manifest, recomputed only
// when its stat manifest moves (a repeat read inside the TTL reads nothing).
func TestGitServiceNeverTakesManifestPath(t *testing.T) {
	ctx := context.Background()
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const sha = "0123456789abcdef0123456789abcdef01234567"
	ms := store.NewMemoryStore()
	c := New(connector.NewLocalConnector(), ms)
	t.Cleanup(func() { _ = c.Close() })

	// A git work tree whose HEAD names a ref that exists nowhere: the files
	// cannot answer, the git fallback fails — freshness unknown.
	broken := t.TempDir()
	write(filepath.Join(broken, ".git", "HEAD"), "ref: refs/heads/gone\n")
	write(filepath.Join(broken, "a.go"), "package a\n")
	_ = ms.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "gitsvc", Path: broken, IndexedCommit: sha})
	before := connector.ManifestComputations()
	wm, err := c.ServiceWatermark(ctx, "gitsvc")
	if wm != "" || !errors.Is(err, gitro.ErrFreshnessUnknown) {
		t.Fatalf("unreadable HEAD: %q, %v; want \"\" and freshness unknown", wm, err)
	}
	if n := connector.ManifestComputations() - before; n != 0 {
		t.Fatalf("a git-SHA service computed %d content manifest(s); it must never take that path", n)
	}

	// Readable: the SHA from the ref file.
	ok := t.TempDir()
	write(filepath.Join(ok, ".git", "HEAD"), "ref: refs/heads/main\n")
	write(filepath.Join(ok, ".git", "refs", "heads", "main"), sha+"\n")
	_ = ms.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "gitok", Path: ok, IndexedCommit: "fedcba9876543210fedcba9876543210fedcba98"})
	if wm, err := c.ServiceWatermark(ctx, "gitok"); err != nil || wm != sha {
		t.Fatalf("readable HEAD: %q, %v; want %s", wm, err, sha)
	}

	// A non-git corpus: the content manifest, computed once and then
	// answered from the stat-gated memo.
	docs := t.TempDir()
	write(filepath.Join(docs, "a.md"), "# a\n")
	write(filepath.Join(docs, "b.md"), "# b\n")
	_ = ms.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "docs", Path: docs, IndexedCommit: connector.ManifestWatermarkPrefix + "x"})
	before = connector.ManifestComputations()
	first, err := c.ServiceWatermark(ctx, "docs")
	if err != nil || len(first) < len(connector.ManifestWatermarkPrefix)+10 {
		t.Fatalf("non-git: %q, %v", first, err)
	}
	second, _ := c.ServiceWatermark(ctx, "docs")
	if second != first {
		t.Fatalf("repeat read changed the watermark: %q -> %q", first, second)
	}
	if n := connector.ManifestComputations() - before; n != 1 {
		t.Fatalf("two reads of an unchanged non-git corpus computed %d content manifests; want 1", n)
	}
	want, _ := connector.ManifestWatermark(ctx, connector.NewLocalConnector(), core.ServiceRef{ID: "docs", Path: docs})
	if first != want {
		t.Fatalf("stat-gated manifest %q differs from the content manifest %q an ingest records", first, want)
	}
}
