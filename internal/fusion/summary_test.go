package fusion_test

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

// TestServiceSummary_WrittenAtIngestCommit is RT-76 (§6): an ingest's
// final commit writes the watermark, file_stats and service_summary
// together — every discovered file with its size, mtime, inode and hash,
// oversize and unsupported files named and counted, the symbol count equal
// to what the ingest stored — and, into a fresh store, the call index.
func TestServiceSummary_WrittenAtIngestCommit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(rel string, data []byte) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a/a.go", []byte("package a\n\nfunc A() { B() }\n"))
	write("a/b.go", []byte("package a\n\nfunc B() {}\n"))
	write("app/Main.swift", []byte("print(1)\n"))
	write("huge.go", append([]byte("package a\n// "), bytes.Repeat([]byte("x"), 3<<20)...))

	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithGitHead(func(string) string { return "head1" })
	p.WithGitLog(func(string) string { return "" })
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}

	meta, ok, _ := st.GetServiceMeta(ctx, "svc")
	if !ok || meta.IndexedCommit != "head1" {
		t.Fatalf("meta = %+v", meta)
	}
	sum, ok, err := st.ServiceSummary(ctx, "svc")
	if err != nil || !ok {
		t.Fatalf("no summary after ingest: ok=%v err=%v", ok, err)
	}
	obs, _ := st.QueryObservations(ctx, "svc", "")
	if sum.Watermark != store.WatermarkOf(meta) || sum.Source != store.SummaryFromIngest || sum.SymbolCount != len(obs) || sum.SymbolCount == 0 {
		t.Fatalf("summary = %+v (observations %d, watermark %s)", sum, len(obs), store.WatermarkOf(meta))
	}
	if sum.FileCount != 4 || sum.UnsupportedLangs["swift"] != 1 || sum.ExcludedCounts["oversize"] != 1 || sum.Bytes < 3<<20 || !sum.HasFileStats {
		t.Fatalf("summary facts = %+v", sum)
	}
	files, ok, _ := st.FileStats(ctx, "svc")
	if !ok || len(files) != 4 {
		t.Fatalf("file_stats = %+v", files)
	}
	byPath := map[string]store.FileStat{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	a := byPath["a/a.go"]
	if a.Size == 0 || a.MtimeNS == 0 || a.Ino == 0 || a.ContentHash == "" || a.Excluded != "" {
		t.Fatalf("a/a.go stat row = %+v", a)
	}
	if h := byPath["huge.go"]; h.Excluded != "oversize" || h.Size < 3<<20 || h.ContentHash != "" {
		t.Fatalf("huge.go row = %+v", h)
	}
	if sw := byPath["app/Main.swift"]; sw.Lang != store.UnsupportedLangPrefix+"swift" {
		t.Fatalf("swift row = %+v", sw)
	}
	ds, _ := st.DerivedState(ctx, "svc")
	if !ds.SummaryCurrent || !ds.CallsIndexed || !ds.HasFileStats {
		t.Fatalf("derived state = %+v", ds)
	}
	if rows, _ := st.CallersOfKeys(ctx, []string{"B"}); len(rows) != 1 || rows[0].Symbol != "A" {
		t.Fatalf("callers of B = %+v", rows)
	}

	// A no-op re-ingest rewrites the summary at the new watermark.
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	meta2, _, _ := st.GetServiceMeta(ctx, "svc")
	if sum2, _, _ := st.ServiceSummary(ctx, "svc"); sum2.Watermark != store.WatermarkOf(meta2) || sum2.SymbolCount != sum.SymbolCount {
		t.Fatalf("re-ingest summary = %+v", sum2)
	}
}
