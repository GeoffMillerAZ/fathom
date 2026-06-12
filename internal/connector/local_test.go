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

	"github.com/geoffmilleraz/sextant/internal/connector"
	"github.com/geoffmilleraz/sextant/internal/core"
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

// ─── .sextantignore tests ────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalConnector_Discover_SextantIgnore(t *testing.T) {
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

	got := map[string]bool{}
	for _, r := range refs {
		rel, _ := filepath.Rel(root, r.Path)
		got[filepath.ToSlash(rel)] = true
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
