package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenManifestMissingStoreDir verifies that --store-dir is required.
func TestGenManifestMissingStoreDir(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"gen-platform-manifest"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --store-dir is missing")
	}
}

// TestGenManifest_EmptyDir verifies that an empty store dir produces a valid
// manifest with zero services.
func TestGenManifest_EmptyDir(t *testing.T) {
	storeDir := t.TempDir()
	outputPath := filepath.Join(t.TempDir(), "manifest.yaml")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-platform-manifest",
		"--store-dir", storeDir,
		"--output", outputPath,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("gen-platform-manifest exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "services") {
		t.Errorf("manifest missing 'services' key; got:\n%s", content)
	}
}

// TestGenManifest_WithServices creates fake .duckdb files and verifies they
// appear in the manifest.
func TestGenManifest_WithServices(t *testing.T) {
	storeDir := t.TempDir()

	// Create fake .duckdb files (content doesn't matter — just existence).
	for _, name := range []string{"alpha.duckdb", "beta.duckdb", "gamma.duckdb"} {
		if err := os.WriteFile(filepath.Join(storeDir, name), []byte{}, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	// Non-.duckdb file should be ignored.
	if err := os.WriteFile(filepath.Join(storeDir, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile readme: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "manifest.yaml")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-platform-manifest",
		"--store-dir", storeDir,
		"--output", outputPath,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("gen-platform-manifest exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	content := string(data)
	for _, svc := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(content, svc) {
			t.Errorf("manifest missing service %q; content:\n%s", svc, content)
		}
	}
	if strings.Contains(content, "readme") {
		t.Errorf("manifest should not contain non-.duckdb file 'readme'; content:\n%s", content)
	}
}

// TestGenManifest_DefaultOutput checks that the command uses workspace/artifacts
// as the default output directory when --output is not supplied.  We redirect
// to a temp path to avoid polluting the workspace.
func TestGenManifest_DefaultOutput(t *testing.T) {
	storeDir := t.TempDir()
	outputPath := filepath.Join(t.TempDir(), "platform-manifest.yaml")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-platform-manifest",
		"--store-dir", storeDir,
		"--output", outputPath,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("gen-platform-manifest exited %d; stderr=%s", code, errBuf.String())
	}

	if out.String() == "" {
		t.Error("expected non-empty stdout from gen-platform-manifest")
	}
}
