package fusion_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// An arbitrary connector has no authoritative policy receipt. It continues
// to ingest, but cannot publish a complete source reference projection.
type unknownPolicyConnector struct{ connector.Connector }

func TestIngestSourceRefProjectionExactBytesAndBackfill(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "refs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithStoreStats()
	p.WithGitHead(func(string) string { return "same-head" })
	p.WithGitLog(func(string) string { return "" })
	svc := core.ServiceRef{ID: "svc", Path: root}
	old := time.Now().Add(-time.Hour)
	write := func(name string, b []byte) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	var zipped bytes.Buffer
	gz := gzip.NewWriter(&zipped)
	_, _ = gz.Write([]byte("[ZIP-9]"))
	_ = gz.Close()
	inputs := map[string][]byte{
		"a.go":         []byte("package a\n// [A-1]\nfunc A() {}"),
		"late/z.json":  []byte(`{"key":"[LATE-7]"}`),
		"plain.txt":    []byte("no references"),
		"generated.go": []byte("// Code generated; DO NOT EDIT. [GEN-3]\npackage a"),
		"binary.bin":   append([]byte{0, 1, 2}, []byte("[BIN-4]")...),
		"archive.gz":   zipped.Bytes(),
	}
	for name, b := range inputs {
		write(name, b)
	}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	check := func(wantComplete bool) {
		t.Helper()
		got, ok, err := st.ReadSourceRefProjection(ctx, "svc")
		if err != nil || !ok {
			t.Fatalf("projection ok=%v err=%v", ok, err)
		}
		if got.Complete != wantComplete || len(got.Rows) != len(inputs) {
			t.Fatalf("projection %+v", got)
		}
		for _, row := range got.Rows {
			want := sdd.ExtractRefs(string(inputs[row.Path]))
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(row.Refs, want) {
				t.Errorf("%s refs=%v want=%v", row.Path, row.Refs, want)
			}
			if row.SourceHash == "" {
				t.Errorf("%s missing hash", row.Path)
			}
		}
	}
	check(true)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.StatSkipped == 0 {
		t.Fatal("expected stat reuse")
	}
	check(true)
	// Losing a previous seal forces reference backfill even when every stat
	// and symbol fingerprint would otherwise be skipped.
	if err := st.InvalidateSourceRefProjection(ctx, "svc"); err != nil {
		t.Fatal(err)
	}
	rep, err = p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.StatSkipped != 0 {
		t.Fatalf("backfill skipped %d files", rep.StatSkipped)
	}
	check(true)
	// Same HEAD is intentionally a new generation only after another ingest.
	write("late/z.json", []byte(`{"key":"[NEW-8]"}`))
	inputs["late/z.json"] = []byte(`{"key":"[NEW-8]"}`)
	oldSnapshot, ok, err := st.ReadSourceRefProjection(ctx, "svc")
	if err != nil || !ok || oldSnapshot.Scope != store.SourceRefScopeIngestGeneration || oldSnapshot.CurrentWorktreeFreshness != "unknown" {
		t.Fatalf("dirty same-HEAD snapshot scope: %+v ok=%v err=%v", oldSnapshot, ok, err)
	}
	if err := os.Remove(filepath.Join(root, "plain.txt")); err != nil {
		t.Fatal(err)
	}
	delete(inputs, "plain.txt")
	write("new.txt", []byte("[ADD-9]"))
	inputs["new.txt"] = []byte("[ADD-9]")
	if err := os.Rename(filepath.Join(root, "a.go"), filepath.Join(root, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	inputs["renamed.go"] = inputs["a.go"]
	delete(inputs, "a.go")
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	check(true)
}

func TestIngestSourceRefProjectionUnknownConnectorRefusesSeal(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("[A-1]"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "refs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(unknownPolicyConnector{connector.NewLocalConnector()}, st, fusion.Config{Concurrency: 1})
	p.WithGitHead(func(string) string { return "head" })
	p.WithGitLog(func(string) string { return "" })
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: root}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := st.ReadSourceRefProjection(ctx, "svc"); err != nil || ok {
		t.Fatalf("unknown policy sealed: ok=%v err=%v", ok, err)
	}
}
