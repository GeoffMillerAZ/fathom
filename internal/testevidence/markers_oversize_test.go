package testevidence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCertifyMarkersSkipOversizeGoFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a_generated.go"), "// Code generated. DO NOT EDIT.\npackage cart\n"+strings.Repeat(" ", 3<<20))
	mustWrite(t, filepath.Join(root, "cart_test.go"), "package cart\n// certifies: invariant I1\nfunc TestCart() {}\n")
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	report, err := IngestCertifyMarkers(context.Background(), st, "svc", "w1", root)
	if err != nil {
		t.Fatalf("oversize file aborted marker scan: %v", err)
	}
	decls, err := st.CertifyDeclarationsForService(context.Background(), "svc")
	if err != nil || len(decls) != 1 || report.Declared != 1 || report.FilesScanned != 1 || report.OversizeSkipped != 1 {
		t.Fatalf("marker lost: report=%+v decls=%+v err=%v", report, decls, err)
	}
}
