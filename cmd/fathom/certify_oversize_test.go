package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCertifySummaryCountsOversizeFiles(t *testing.T) {
	root := t.TempDir()
	for rel, src := range map[string]string{
		"a_generated.go": "// Code generated. DO NOT EDIT.\npackage cart\n" + strings.Repeat(" ", 3<<20),
		"cart_test.go":   "package cart\n// certifies: invariant I1\nfunc TestCart() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "store.sqlite")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	var out, warnings bytes.Buffer
	ingestConfiguredCertifications(context.Background(), path, "svc", root, &out, &warnings, false, false)
	if !strings.Contains(out.String(), "1 declared certification(s)") || !strings.Contains(out.String(), "oversize_skipped=1") {
		t.Fatalf("missing marker or oversize count: stdout=%q stderr=%q", out.String(), warnings.String())
	}
}
