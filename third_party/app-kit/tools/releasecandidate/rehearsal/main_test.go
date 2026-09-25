package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedFixtureIsCompleteAndFrozen(t *testing.T) {
	files, sum, err := embeddedFixture()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || len(sum) != 64 {
		t.Fatalf("fixture identity incomplete: files=%d sum=%q", len(files), sum)
	}
	if !strings.Contains(string(files["go.mod.txt"]), "__APPKIT_VERSION__") ||
		!strings.Contains(string(files["input.css"]), "source(none)") ||
		!strings.Contains(string(files["main.go"]), "analytics.RenderBars") {
		t.Fatal("embedded fixture lost its pin, scanner guard, or public API exercise")
	}
	source, err := os.ReadFile("fixture/main.go")
	if err != nil || !bytes.Equal(source, files["main.go"]) {
		t.Fatalf("embedded consumer differs from the root-tested source: %v", err)
	}
	if _, err := fixtureFS.ReadFile("fixture/main_test.go"); err == nil {
		t.Fatal("fixture tests must not become embedded consumer inputs")
	}
}

func TestReceiptRejectsTrailingData(t *testing.T) {
	for _, suffix := range []string{"{}", "null", "garbage"} {
		if _, err := decodeCandidateReceipt([]byte(`{"module":"test"}` + suffix)); err == nil {
			t.Fatalf("accepted trailing %q", suffix)
		}
	}
	if _, err := decodeCandidateReceipt([]byte("{\"module\":\"test\"}\n \t")); err != nil {
		t.Fatal(err)
	}
}

func TestCSSRulesCannotPassOnRelatedClassNames(t *testing.T) {
	if hasCSSRule([]byte(`.text-ink-muted{color:red}.border-line-strong {color:blue}`), ".text-ink") ||
		hasCSSRule([]byte(`.border-line-strong {color:blue}`), ".border-line") {
		t.Fatal("related class name passed exact selector check")
	}
	for _, css := range []string{`.text-ink{color:red}`, ".text-ink {\ncolor:red\n}"} {
		if !hasCSSRule([]byte(css), ".text-ink") {
			t.Fatalf("exact selector refused: %q", css)
		}
	}
}

func TestReceiptVersionMustBindTree(t *testing.T) {
	receipt := candidateReceipt{TreeSHA256: strings.Repeat("a", 64), Version: "v0.0.0-rehearsal.h" + strings.Repeat("b", 64)}
	if err := validateReceiptIdentity(receipt); err == nil {
		t.Fatal("mismatched version/tree identity passed")
	}
	receipt.Version = "v0.0.0-rehearsal.h" + receipt.TreeSHA256
	if err := validateReceiptIdentity(receipt); err != nil {
		t.Fatalf("matching version/tree identity failed: %v", err)
	}
}

func TestZIPInventoryRejectsInventedContentHash(t *testing.T) {
	tree := strings.Repeat("a", 64)
	version := "v0.0.0-rehearsal.h" + tree
	zipPath := filepath.Join(t.TempDir(), "candidate.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create(candidateModule + "@" + version + "/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("module " + candidateModule + "\n")
	if _, err := entry.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	receipt := candidateReceipt{Version: version, Files: []fileRecord{{Path: "go.mod", Bytes: int64(len(body)), SHA256: strings.Repeat("0", 64)}}}
	if err := verifyZIPInventory(zipPath, receipt); err == nil || !strings.Contains(err.Error(), "content disagrees") {
		t.Fatalf("invented receipt content hash was not refused: %v", err)
	}
}
