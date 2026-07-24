package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fauxTownRoot returns the absolute path to
// internal/connector/testdata/fauxtown, resolved relative to this source
// file so the test is working-directory independent.
func fauxTownRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../../internal/connector/testdata/fauxtown"
	}
	return filepath.Join(filepath.Dir(file),
		"../../internal/connector/testdata/fauxtown")
}

// ─── flag validation ─────────────────────────────────────────────────────────

func TestPlatformIngestMissingRoot(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"platform-ingest", "--store-dir", t.TempDir()}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --root is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr message")
	}
}

func TestPlatformIngestMissingStoreDir(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"platform-ingest", "--root", t.TempDir()}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --store-dir is missing")
	}
	if errBuf.String() == "" {
		t.Fatal("expected non-empty stderr message")
	}
}

func TestPlatformIngestRootNotExist(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-ingest",
		"--root", "/no/such/path/xyz",
		"--store-dir", t.TempDir(),
	}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --root does not exist")
	}
}

// ─── end-to-end: fauxtown has payment-service + ledger-service ───────────────

func TestPlatformIngestFauxtown(t *testing.T) {
	root := fauxTownRoot()
	// Use a non-existent subpath to exercise store-dir auto-creation.
	storeDir := filepath.Join(t.TempDir(), "fleet")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-ingest",
		"--root", root,
		"--store-dir", storeDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("platform-ingest exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	// Both service store files must exist (ADR-020: platform-ingest now
	// defaults to the SQLite backend).
	for _, svc := range []string{"payment-service", "ledger-service"} {
		dbPath := filepath.Join(storeDir, svc+".sqlite")
		if _, err := os.Stat(dbPath); err != nil {
			t.Errorf("expected store file %s to exist: %v", dbPath, err)
		}
	}

	// Output must mention both services.
	combined := out.String()
	for _, svc := range []string{"payment-service", "ledger-service"} {
		if !strings.Contains(combined, svc) {
			t.Errorf("output does not mention %q;\nstdout:\n%s", svc, combined)
		}
	}

	// Output must include a totals line.
	if !strings.Contains(combined, "total") && !strings.Contains(combined, "Total") {
		t.Errorf("output does not mention totals;\nstdout:\n%s", combined)
	}
}

// ─── empty root: no subdirs → zero services, still exits 0 ──────────────────

func TestPlatformIngestEmptyRoot(t *testing.T) {
	root := t.TempDir()
	storeDir := t.TempDir()

	var out, errBuf bytes.Buffer
	code := run([]string{
		"platform-ingest",
		"--root", root,
		"--store-dir", storeDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("platform-ingest on empty root exited %d; stderr=%q", code, errBuf.String())
	}
	// No store files should have been created.
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty storeDir for empty root; got %v", entries)
	}
}
