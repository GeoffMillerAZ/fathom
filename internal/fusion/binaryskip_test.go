package fusion_test

// binaryskip_test.go — ADR-021 §1 / LT-2 regression: a file whose content
// sniffs as binary (NUL byte in the first 8000 bytes) is excluded from
// extraction entirely, even if its path happens to match a registered
// extractor's extension. The fingerprint is still recorded so the file isn't
// re-sniffed on every ingest.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_BinaryFileExcludedFromExtraction(t *testing.T) {
	dir := t.TempDir()

	// A "source" file that matches a registered extractor's extension (.go)
	// but is actually binary content (NUL bytes) — e.g. a misnamed compiled
	// artifact. It must contribute zero symbols.
	binaryContent := append([]byte("MZ\x00\x00fake binary payload"), bytes.Repeat([]byte{0x00, 0x01}, 100)...)
	if err := os.WriteFile(filepath.Join(dir, "blob.go"), binaryContent, 0o644); err != nil {
		t.Fatalf("write blob.go: %v", err)
	}
	// A real, extractable Go file alongside it, so the ingest has at least
	// one legitimate symbol to sanity-check against.
	goSrc := "package main\n\nfunc Real() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatalf("write real.go: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "binary-skip-svc", Path: dir}
	p := fusion.New(conn, st, fusion.Config{Concurrency: 1})

	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	for _, o := range obs {
		if o.SourceRef == "blob.go" {
			t.Errorf("binary file blob.go produced an observation: %+v", o)
		}
	}
	var sawReal bool
	for _, o := range obs {
		if o.SourceRef == "real.go" && o.Symbol == "Real" {
			sawReal = true
		}
	}
	if !sawReal {
		t.Error("the real, non-binary Go file's symbol was not extracted")
	}

	// The fingerprint is still recorded for the binary file (so it isn't
	// re-sniffed forever), but its content hash is what's stored.
	if _, found, err := st.Fingerprint(ctx, svc.ID, "blob.go"); err != nil || !found {
		t.Errorf("Fingerprint(blob.go): found=%v err=%v, want found=true", found, err)
	}
}
