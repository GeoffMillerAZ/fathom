package connector_test

// local_test.go — tests for LocalConnector.
// Uses the fauxtown testdata fixture so that the LocalConnector is exercised
// against real on-disk files without any network or external secrets.

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fauxRoot returns the absolute path to the testdata/fauxtown directory
// regardless of the working directory the test runner uses.
func fauxRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "fauxtown")
}

func TestLocalConnector_Discover_PaymentService(t *testing.T) {
	root := fauxRoot(t)
	svc := core.ServiceRef{
		ID:   "payment-service",
		Path: filepath.Join(root, "payment-service"),
	}
	lc := connector.NewLocalConnector()

	ctx := context.Background()
	refs, err := lc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Discover returned no SourceRefs")
	}

	// All refs must carry the service ID.
	for _, r := range refs {
		if r.Service != svc.ID {
			t.Errorf("SourceRef.Service = %q, want %q", r.Service, svc.ID)
		}
	}

	// Paths must be sorted (deterministic ordering).
	for i := 1; i < len(refs); i++ {
		if refs[i].Path < refs[i-1].Path {
			t.Errorf("paths not sorted: refs[%d].Path=%q < refs[%d].Path=%q",
				i, refs[i].Path, i-1, refs[i-1].Path)
		}
	}

	// Both .go files in payment-service must appear.
	wantFiles := map[string]bool{
		"errors.go":  false,
		"payment.go": false,
	}
	for _, r := range refs {
		base := filepath.Base(r.Path)
		if _, ok := wantFiles[base]; ok {
			wantFiles[base] = true
		}
	}
	for name, found := range wantFiles {
		if !found {
			t.Errorf("expected file %q not in Discover results", name)
		}
	}
}

func TestLocalConnector_Discover_KindAssignment(t *testing.T) {
	root := fauxRoot(t)
	svc := core.ServiceRef{
		ID:   "payment-service",
		Path: filepath.Join(root, "payment-service"),
	}
	lc := connector.NewLocalConnector()
	ctx := context.Background()

	refs, err := lc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for _, r := range refs {
		if filepath.Ext(r.Path) == ".go" {
			if r.Kind != core.KindGoFunc {
				t.Errorf("path %q: Kind = %q, want KindGoFunc", r.Path, r.Kind)
			}
		}
	}
}

func TestLocalConnector_Fetch(t *testing.T) {
	root := fauxRoot(t)
	svc := core.ServiceRef{
		ID:   "payment-service",
		Path: filepath.Join(root, "payment-service"),
	}
	lc := connector.NewLocalConnector()
	ctx := context.Background()

	refs, err := lc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("no refs to Fetch")
	}

	for _, r := range refs {
		data, err := lc.Fetch(ctx, r)
		if err != nil {
			t.Errorf("Fetch(%q): %v", r.Path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Fetch(%q): empty bytes", r.Path)
		}
	}
}

func TestLocalConnector_Fetch_NonexistentRef(t *testing.T) {
	lc := connector.NewLocalConnector()
	ctx := context.Background()

	ref := core.SourceRef{
		Service: "nosuchsvc",
		Path:    "/no/such/file.go",
		Kind:    core.KindGoFunc,
	}
	_, err := lc.Fetch(ctx, ref)
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLocalConnector_Discover_SortedDeterministic(t *testing.T) {
	root := fauxRoot(t)
	svc := core.ServiceRef{
		ID:   "ledger-service",
		Path: filepath.Join(root, "ledger-service"),
	}
	lc := connector.NewLocalConnector()
	ctx := context.Background()

	first, err := lc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	second, err := lc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("non-deterministic length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Path != second[i].Path {
			t.Errorf("non-deterministic order at [%d]: %q vs %q", i, first[i].Path, second[i].Path)
		}
	}
}

func TestLocalConnector_Discover_IgnoresVendorAndDotGit(t *testing.T) {
	// The fauxtown fixture does not have vendor/ or .git/ dirs, so we use the
	// connectorContract helper to verify basic correctness and trust the skip
	// logic is covered by the negative fixture if one is added later.
	root := fauxRoot(t)
	svc := core.ServiceRef{
		ID:   "payment-service",
		Path: filepath.Join(root, "payment-service"),
	}
	lc := connector.NewLocalConnector()
	connectorContract(t, lc, svc, []string{"errors.go", "payment.go"})
}

// TestLocalConnector_Discover_SkipsToolStateDirs pins the full default skipDir
// set — including .claude, whose agent worktrees (.claude/worktrees/<agent>/)
// are full repo copies that polluted the fleet index when ingested. Tool-state
// dirs are the same class as .idea/.vscode and must never be discovered.
func TestLocalConnector_Discover_SkipsToolStateDirs(t *testing.T) {
	skips := []string{
		"vendor", ".git", "testdata", "generated",
		"node_modules", ".idea", ".vscode", ".claude",
	}
	for _, dir := range skips {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
			writeFile(t, filepath.Join(root, dir, "worktrees", "agent-stale", "copy.go"),
				"package keep\n")

			lc := connector.NewLocalConnector()
			refs, err := lc.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if len(refs) != 1 {
				t.Fatalf("Discover returned %d refs, want 1 (keep.go only): %+v", len(refs), refs)
			}
			if filepath.Base(refs[0].Path) != "keep.go" {
				t.Errorf("discovered %q, want keep.go", refs[0].Path)
			}
		})
	}
}

// ─── .fathomignore tests ────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalConnector_Discover_FathomIgnore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package main\n")
	writeFile(t, filepath.Join(root, "ref", "mirror.go"), "package mirror\n")
	writeFile(t, filepath.Join(root, "tmp", "scratch.md"), "# scratch\n")
	writeFile(t, filepath.Join(root, "docs", "keep.md"), "# keep\n")
	writeFile(t, filepath.Join(root, "skip.log"), "noise\n")
	writeFile(t, filepath.Join(root, connector.IgnoreFileName),
		"# repo-specific excludes\nref/\ntmp/\n*.log\n"+connector.IgnoreFileName+"\n")

	lc := connector.NewLocalConnector()
	refs, err := lc.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Discover emits repo-relative paths (ADR-017), so refs are used directly.
	got := map[string]bool{}
	for _, r := range refs {
		got[r.Path] = true
	}

	for _, want := range []string{"keep.go", "docs/keep.md"} {
		if !got[want] {
			t.Errorf("expected %s in discovered refs, got %v", want, got)
		}
	}
	for _, banned := range []string{"ref/mirror.go", "tmp/scratch.md", "skip.log", connector.IgnoreFileName} {
		if got[banned] {
			t.Errorf("%s should have been ignored, got %v", banned, got)
		}
	}
}

// A repo that adopted the tool under its former name keeps its excludes: the
// legacy .sextantignore is honoured when no .fathomignore exists.
func TestLocalConnector_Discover_LegacyIgnoreFallback(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package main\n")
	writeFile(t, filepath.Join(root, "ref", "mirror.go"), "package mirror\n")
	writeFile(t, filepath.Join(root, connector.LegacyIgnoreFileName),
		"ref/\n"+connector.LegacyIgnoreFileName+"\n")

	lc := connector.NewLocalConnector()
	refs, err := lc.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	got := map[string]bool{}
	for _, r := range refs {
		got[r.Path] = true
	}
	if !got["keep.go"] {
		t.Errorf("expected keep.go in discovered refs, got %v", got)
	}
	if got["ref/mirror.go"] {
		t.Errorf("legacy ignore file not honoured: ref/mirror.go should be excluded, got %v", got)
	}
}

// TestLocalConnector_Discover_SkipsSymlinkToDir is a regression test for an
// ingest abort on real-world trees: WalkDir does not follow symlinks, so a
// symlink pointing at a directory (e.g. a Nix/devbox profile link) surfaces as a
// non-dir entry. If Discover emitted it as a SourceRef, Fetch's os.ReadFile would
// fail with "is a directory" and — because the fusion fetch pass treats the first
// fetch error as fatal — abort the entire ingest over one stray link. Discover
// must skip non-regular files so a regular file beside the link still indexes.
func TestLocalConnector_Discover_SkipsSymlinkToDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package main\n")
	// A directory and a symlink that points at it (the .devbox/nix shape).
	target := filepath.Join(root, "profile")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "default-9-link")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	lc := connector.NewLocalConnector()
	ctx := context.Background()
	refs, err := lc.Discover(ctx, core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for _, r := range refs {
		if filepath.Base(r.Path) == "default-9-link" {
			t.Fatalf("symlink-to-dir was emitted as a SourceRef: %q", r.Path)
		}
		// Every emitted ref must Fetch without the "is a directory" abort.
		if _, err := lc.Fetch(ctx, r); err != nil {
			t.Errorf("Fetch(%q): %v", r.Path, err)
		}
	}

	// The regular file beside the link still indexes.
	var sawKeep bool
	for _, r := range refs {
		if filepath.Base(r.Path) == "keep.go" {
			sawKeep = true
		}
	}
	if !sawKeep {
		t.Error("keep.go should have been discovered alongside the skipped symlink")
	}
}

func TestLocalConnector_Discover_NoIgnoreFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")

	lc := connector.NewLocalConnector()
	refs, err := lc.Discover(context.Background(), core.ServiceRef{ID: "svc", Path: root})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d", len(refs))
	}
}
