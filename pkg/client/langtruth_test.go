package client_test

// langtruth_test.go — ADR-021 §1 (docs/plans/lang-truth.md) end-to-end
// coverage at the Client.Sources boundary: the shared connector language
// registry (LT-1) and content-based binary detection (LT-2) are both wired
// in here, live-computed on every call.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestSources_LanguageCoverage ingests a small polyglot corpus — including a
// previously-unmapped extension (.cue, one of the 14 the stale 10-language
// langFromPath table rendered as "other") and a genuinely unrecognised
// extension — and asserts Client.Sources tags every file with the shared
// registry's language, with an explicit "unclassified" bucket rather than a
// silent "other".
func TestSources_LanguageCoverage(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	files := map[string]string{
		"main.go":       "package main\nfunc main() {}\n",
		"schema.cue":    "package svc\nfoo: string\n",
		"config.toml":   "key = \"value\"\n",
		"run.sh":        "#!/bin/sh\necho hi\n",
		"notes.unknown": "no extractor for this one\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	st, err := store.Open("")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "langcov", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	got, err := c.Sources(ctx, "langcov")
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}

	langByPath := make(map[string]string, len(got))
	for _, f := range got {
		langByPath[f.Path] = f.Lang
	}

	want := map[string]string{
		"main.go":       "go",
		"schema.cue":    "cue",
		"config.toml":   "toml",
		"run.sh":        "sh",
		"notes.unknown": core.LangUnclassified,
	}
	for path, wantLang := range want {
		gotLang, ok := langByPath[path]
		if !ok {
			t.Errorf("%s: not present in Sources() output", path)
			continue
		}
		if gotLang != wantLang {
			t.Errorf("%s: Lang = %q, want %q", path, gotLang, wantLang)
		}
		if gotLang == "" {
			t.Errorf("%s: Lang is empty string — must be an explicit tag, never silent", path)
		}
	}
}

// TestSources_BinaryFileTaggedAndExcluded is the LT-3 regression: a
// NUL-bearing file is tagged core.LangBinary regardless of its extension
// (content wins over path), so downstream line-counting tools can exclude it.
func TestSources_BinaryFileTaggedAndExcluded(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Named like a source file the registry recognises (.go), but its
	// content is binary — the classic "someone committed a compiled
	// artifact" case (the grounding audit's 423K-line Mach-O binary).
	binary := append([]byte("\x00\x01\x02binary-ish-content"), make([]byte, 100)...)
	if err := os.WriteFile(filepath.Join(dir, "blob.go"), binary, 0o644); err != nil {
		t.Fatalf("write blob.go: %v", err)
	}
	realSrc := "package main\nfunc Real() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte(realSrc), 0o644); err != nil {
		t.Fatalf("write real.go: %v", err)
	}

	st, err := store.Open("")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "binsvc", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	got, err := c.Sources(ctx, "binsvc")
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	var sawBlob, sawReal bool
	for _, f := range got {
		switch f.Path {
		case "blob.go":
			sawBlob = true
			if f.Lang != core.LangBinary {
				t.Errorf("blob.go: Lang = %q, want %q (content-based, overriding its .go extension)", f.Lang, core.LangBinary)
			}
		case "real.go":
			sawReal = true
			if f.Lang != "go" {
				t.Errorf("real.go: Lang = %q, want %q", f.Lang, "go")
			}
		}
	}
	if !sawBlob {
		t.Error("blob.go missing from Sources() output")
	}
	if !sawReal {
		t.Error("real.go missing from Sources() output")
	}
}
