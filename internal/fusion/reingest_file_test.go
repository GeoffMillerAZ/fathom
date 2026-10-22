package fusion_test

// reingest_file_test.go — SU-5 (docs/plans/substrate.md §A, ADR-041
// decisions 3 and 4).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestReingestFile_ReplacesOneFile is SU-5's headline proof: after a
// file's content changes on disk, ReingestFile replaces its observations
// (old symbol gone, new symbol present) and fingerprint WITHOUT touching
// any other file's observations.
func TestReingestFile_ReplacesOneFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	other := filepath.Join(dir, "other.go")

	original := "package main\n\nfunc Original() int { return 1 }\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatalf("write target.go: %v", err)
	}
	if err := os.WriteFile(other, []byte("package main\n\nfunc Untouched() int { return 2 }\n"), 0o644); err != nil {
		t.Fatalf("write other.go: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "reingest-svc", Path: dir}
	p := fusion.New(conn, st, fusion.Config{Concurrency: 1})

	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}

	before, err := st.QueryObservations(ctx, "reingest-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations (before): %v", err)
	}
	if !hasSymbol(before, "Original") {
		t.Fatalf("expected Original among initial observations: %+v", before)
	}

	// Edit target.go on disk: Original() is gone, Changed() takes its
	// place — exactly the "one file changed under an agent's edit" shape
	// the pre-edit hook (SU-6) will call this for.
	edited := "package main\n\nfunc Changed() int { return 3 }\n"
	if err := os.WriteFile(target, []byte(edited), 0o644); err != nil {
		t.Fatalf("rewrite target.go: %v", err)
	}

	report, err := p.ReingestFile(ctx, st, "reingest-svc", dir, "target.go")
	if err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	if !report.Refreshed {
		t.Fatalf("report.Refreshed = false, want true: %+v", report)
	}
	if report.Symbols != 1 {
		t.Errorf("report.Symbols = %d, want 1: %+v", report.Symbols, report)
	}
	if !report.TypedEdgesStale {
		t.Errorf("report.TypedEdgesStale = false for a .go file, want true: %+v", report)
	}

	after, err := st.QueryObservations(ctx, "reingest-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations (after): %v", err)
	}
	if hasSymbol(after, "Original") {
		t.Errorf("Original still present after ReingestFile replaced target.go: %+v", after)
	}
	if !hasSymbol(after, "Changed") {
		t.Errorf("Changed missing after ReingestFile: %+v", after)
	}
	if !hasSymbol(after, "Untouched") {
		t.Errorf("Untouched (a DIFFERENT file) disappeared after ReingestFile touched only target.go: %+v", after)
	}

	// The fingerprint must reflect the NEW content, not the original.
	newHash := fusion.FingerprintHash([]byte(edited))
	gotHash, found, err := st.Fingerprint(ctx, "reingest-svc", "target.go")
	if err != nil || !found {
		t.Fatalf("Fingerprint after ReingestFile: found=%v err=%v", found, err)
	}
	if gotHash != newHash {
		t.Errorf("fingerprint = %q, want %q (the new content's hash)", gotHash, newHash)
	}
}

// TestReingestFile_BinaryFileSkipsExtraction: a binary file is
// fingerprinted but never handed to an extractor — Refreshed stays false.
func TestReingestFile_BinaryFileSkipsExtraction(t *testing.T) {
	dir := t.TempDir()
	blob := filepath.Join(dir, "blob.go")
	content := append([]byte("MZ\x00\x00binary"), 0x00, 0x01, 0x00, 0x02)
	if err := os.WriteFile(blob, content, 0o644); err != nil {
		t.Fatalf("write blob.go: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, fusion.Config{Concurrency: 1})
	ctx := context.Background()

	report, err := p.ReingestFile(ctx, st, "svc", dir, "blob.go")
	if err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	if report.Refreshed {
		t.Errorf("report.Refreshed = true for a binary file, want false: %+v", report)
	}
	if _, found, _ := st.Fingerprint(ctx, "svc", "blob.go"); !found {
		t.Error("binary file was not fingerprinted")
	}
}

func hasSymbol(obs []core.Observation, symbol string) bool {
	for _, o := range obs {
		if o.Symbol == symbol {
			return true
		}
	}
	return false
}

// RT-145 review: /refresh-file sees the corpus ingest sees. A file under a
// default-ignored cache directory, an always-skipped directory or a
// .fathomignore'd path is not indexed by the pre-edit hook only to be
// purged as an orphan by the next ingest; a corpus that re-includes the
// directory with `[connector] include` refreshes it as usual.
func TestReingestFile_HonoursCorpusExclusions(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc Main() {}\n")
	write(".fathomignore", "scratch/\n")
	for _, rel := range []string{"dist/gen.go", "vendor/v/v.go", "scratch/s.go", "web/build/b.go"} {
		write(rel, "package x\n\nfunc Hidden() {}\n")
	}
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1, DisableTypedEdges: true})
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "excl", Path: dir}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"dist/gen.go", "vendor/v/v.go", "scratch/s.go", "web/build/b.go"} {
		rep, err := p.ReingestFile(ctx, st, "excl", dir, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if rep.Refreshed || rep.Skipped == "" {
			t.Fatalf("%s: report %+v, want a named skip (outside the corpus)", rel, rep)
		}
	}
	obs, _ := st.QueryObservations(ctx, "excl", "")
	if hasSymbol(obs, "Hidden") {
		t.Fatal("a refresh indexed a file the corpus excludes")
	}

	// Re-included: refreshed like any corpus file.
	write(".fathom/config.toml", "[connector]\ninclude = [\"dist\"]\n")
	rep, err := p.ReingestFile(ctx, st, "excl", dir, "dist/gen.go")
	if err != nil || !rep.Refreshed {
		t.Fatalf("re-included dist/gen.go: %+v, %v; want refreshed", rep, err)
	}
}

// TestReingestFileKeepsEpochReceiptSoEditsReadStale: a one-file refresh moves
// the fingerprint, so the file_stats receipt must follow or the next edit
// reads "unknown" (no epoch receipt) instead of "stale" until a full ingest.
func TestReingestFileKeepsEpochReceiptSoEditsReadStale(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package main\n\nfunc One() int { return 1 }\n")
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "receipt-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	write("package main\n\nfunc Two() int { return 2 }\n")
	if _, err := p.ReingestFile(ctx, st, svc.ID, dir, "target.go"); err != nil {
		t.Fatalf("ReingestFile: %v", err)
	}
	indexed, found, err := st.Fingerprint(ctx, svc.ID, "target.go")
	if err != nil || !found {
		t.Fatalf("Fingerprint: found=%v err=%v", found, err)
	}
	write("package main\n\nfunc Three() int { return 3 }\n")
	if now := fusion.FingerprintHash([]byte("package main\n\nfunc Three() int { return 3 }\n")); now == indexed {
		t.Fatal("edit did not change the fingerprint")
	}
	c := client.New(connector.NewLocalConnector(), st)
	epoch, err := c.SourceFingerprintEpoch(ctx, svc.ID, "target.go", indexed)
	if err != nil {
		t.Fatalf("SourceFingerprintEpoch: %v", err)
	}
	if epoch != fusion.ExtractorEpoch {
		t.Fatalf("epoch receipt = %q, want %q (an edit would read unknown, not stale)", epoch, fusion.ExtractorEpoch)
	}
}
