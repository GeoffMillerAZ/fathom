package fusion_test

// makefile_ingest_test.go — LB-Q-makefile: a Makefile, a GNUmakefile and a *.mk fragment reach the
// makefile dialect through discovery and dispatch (the registry is built from the dialect manifests),
// and nothing else claims them.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_MakefilesReachTheMakefileDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Makefile":         "CC = gcc\n\nall: prog\n\nprog: main.o\n\t$(CC) -o $@ $^\n",
		"sub/GNUmakefile":  "test: build\n\techo test\n",
		"rules/pattern.mk": "%.o: %.c\n\t$(CC) -c $<\n",
		"notes/Makefile.x": "not: claimed\n",
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "makefile-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SourceKind{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o.SourceKind
	}
	for key, kind := range map[string]core.SourceKind{
		"Makefile#CC":          "MakeVariable",
		"Makefile#all":         "MakeTarget",
		"Makefile#prog":        "MakeTarget",
		"sub/GNUmakefile#test": "MakeTarget",
		"rules/pattern.mk#%.o": "MakePatternRule",
	} {
		if got[key] != kind {
			t.Errorf("%s: kind %q, want %q (observations %v)", key, got[key], kind, got)
		}
	}
	for key := range got {
		if filepath.Dir(key) == "notes" || key == "notes/Makefile.x#not" {
			t.Errorf("notes/Makefile.x is not a makefile the dialect claims, but produced %s", key)
		}
	}
}
