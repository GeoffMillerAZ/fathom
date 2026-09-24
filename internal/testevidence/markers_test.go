package testevidence

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngestCertifyMarkers_DeclaresAndWarns(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "cart", "cart_test.go"), `package cart

// certifies: invariant I35 (asserts)
func TestAdd(t *T) {}

// certifies: not-a-kind I99
func TestBroken(t *T) {}
`)
	// A vendored file's marker must never be scanned.
	mustWrite(t, filepath.Join(root, "vendor", "pkg", "pkg_test.go"), `package pkg

// certifies: invariant SHOULD-NOT-APPEAR
func TestVendored(t *T) {}
`)

	report, err := IngestCertifyMarkers(ctx, st, "cart-svc", "wm-1", root)
	if err != nil {
		t.Fatalf("IngestCertifyMarkers: %v", err)
	}
	if report.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1 (vendor/ pruned)", report.FilesScanned)
	}
	if report.Declared != 1 {
		t.Errorf("Declared = %d, want 1", report.Declared)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want 1", report.Warnings)
	}

	decls, err := st.CertifyDeclarationsForService(ctx, "cart-svc")
	if err != nil {
		t.Fatalf("CertifyDeclarationsForService: %v", err)
	}
	if len(decls) != 1 {
		t.Fatalf("stored declarations = %d, want 1", len(decls))
	}
	if decls[0].Kind != "invariant" || decls[0].TargetID != "I35" || decls[0].Strength != "asserts" {
		t.Errorf("decl = %+v", decls[0])
	}
	if decls[0].Watermark != "wm-1" {
		t.Errorf("Watermark = %q, want wm-1", decls[0].Watermark)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
