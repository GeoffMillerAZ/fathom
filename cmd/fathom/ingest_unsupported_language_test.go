package main

// ingest_unsupported_language_test.go — O69 (wave P1): `fathom ingest` must
// disclose a known-but-unextracted source language by name and file count,
// both in the text summary and the --json envelope, rather than letting
// those files vanish silently into the generic no-extractor path — the
// exact gap the Long Code Arena bench run found in bumble-tech/appyx (a real
// Kotlin repo indexed with ~165 symbols and no clue anywhere that Kotlin
// itself was unindexed). Wave P2 (O81) closed the Kotlin gap itself
// (extractor.NewTSExtractor("kotlin")), so these fixtures now use Zig — a
// language that still has no extractor — to keep proving the disclosure
// mechanism in general; see TestIngestReportShowsKotlinSymbols below for the
// Kotlin closure proof.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const (
	zigSrcFixture    = "const std = @import(\"std\");\n\npub fn main() void {}\n"
	kotlinSrcFixture = "package com.example\n\nfun main() {}\n"
)

func TestIngestReportNamesUnsupportedLanguages(t *testing.T) {
	conn := &wedgedConnector{
		refs: []core.SourceRef{
			{Path: "main.go"},
			{Path: "App.zig"},
			{Path: "Widget.zig"},
		},
		content: map[string][]byte{
			"main.go":    []byte(honestyGoSrc),
			"App.zig":    []byte(zigSrcFixture),
			"Widget.zig": []byte(zigSrcFixture),
		},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, "unsupported languages (no extractor)") {
		t.Fatalf("stdout must disclose unsupported languages:\n%s", stdout)
	}
	if !strings.Contains(stdout, "zig: 2 file(s)") {
		t.Errorf("stdout must name zig with its file count:\n%s", stdout)
	}
}

func TestIngestJSONNamesUnsupportedLanguages(t *testing.T) {
	conn := &wedgedConnector{
		refs: []core.SourceRef{
			{Path: "main.go"},
			{Path: "App.zig"},
		},
		content: map[string][]byte{
			"main.go": []byte(honestyGoSrc),
			"App.zig": []byte(zigSrcFixture),
		},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	var payload struct {
		UnsupportedLanguages map[string]int `json:"unsupported_languages"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if got, want := payload.UnsupportedLanguages["zig"], 1; got != want {
		t.Errorf("unsupported_languages[zig] = %d, want %d\n%s", got, want, out.String())
	}
}

// TestIngestReportShowsKotlinSymbols is the wave-P2 closure proof for O69 at
// the CLI layer: a .kt file that used to fall into UnsupportedLanguages now
// extracts real symbols and is never listed as unsupported.
func TestIngestReportShowsKotlinSymbols(t *testing.T) {
	conn := &wedgedConnector{
		refs: []core.SourceRef{
			{Path: "main.go"},
			{Path: "App.kt"},
		},
		content: map[string][]byte{
			"main.go": []byte(honestyGoSrc),
			"App.kt":  []byte(kotlinSrcFixture),
		},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath, "--json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	var payload struct {
		Symbols              int            `json:"symbols"`
		UnsupportedLanguages map[string]int `json:"unsupported_languages"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.Symbols == 0 {
		t.Errorf("symbols = 0, want kotlin's main() extracted\n%s", out.String())
	}
	if _, ok := payload.UnsupportedLanguages["kotlin"]; ok {
		t.Errorf("unsupported_languages still lists kotlin (O69 gap should be closed): %v", payload.UnsupportedLanguages)
	}
}

func TestIngestReportOmitsUnsupportedLanguagesWhenNone(t *testing.T) {
	conn := &wedgedConnector{
		refs:    []core.SourceRef{{Path: "main.go"}},
		content: map[string][]byte{"main.go": []byte(honestyGoSrc)},
	}
	useConnectorForIngest(t, conn)
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")

	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "svc", "--path", t.TempDir(), "--store", storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%s stderr=%s", code, out.String(), errBuf.String())
	}
	if strings.Contains(out.String(), "unsupported languages") {
		t.Errorf("a repo with nothing unsupported must stay quiet:\n%s", out.String())
	}
}
