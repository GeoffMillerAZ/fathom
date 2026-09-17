package main

// ingest_typed_disclosure_test.go — wave P3 defect 1 / O82
// (.fathom/observations.md): the CLI TEXT path (`fathom ingest`, no --json)
// must disclose the typed-edge pass's outcome as plainly as --json does —
// a Go tree whose typed pass produced nothing to trust must print it and
// exit non-zero, never a silent "symbols: N" with no typed-edges line at
// all (the pre-O82 "non-zero packages/failures only" print condition).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdIngest_TypedEdgesDisclosedInText(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/typedtextprobe\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Hello() { helper() }\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "typedtextprobe", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "typed-edges:") {
		t.Errorf("a Go-source ingest must print a typed-edges line, got:\n%s", stdout.String())
	}
}

// TestCmdIngest_TypedEdgesFailureExitsNonZero: a Go tree whose typed pass
// fails must exit 1 in text mode too — the ok:false-grade disclosure is not
// a --json-only behavior.
func TestCmdIngest_TypedEdgesFailureExitsNonZero(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/typedtextbroken\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package broken\n\nfunc Broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "typedtextbroken", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "typed-edge pass failed") {
		t.Errorf("stderr must name the typed-edge failure plainly, got: %s", stderr.String())
	}
}
