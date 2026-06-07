package main

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitRunner returns a runner that emits a newline-separated list of paths.
func fakeGitRunner(paths []string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		return []byte(strings.Join(paths, "\n") + "\n"), nil
	}
}

// fauxPaymentPathForTest resolves the absolute path to the payment-service
// fixture regardless of working directory.
func fauxPaymentPathForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file),
		"../../internal/connector/testdata/fauxtown/payment-service")
}

// TestIngestDiffMissingService verifies that omitting --service returns 1.
func TestIngestDiffMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"ingest-diff"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// TestIngestDiffNoBase performs a full ingest (empty base) via the faux
// connector to verify end-to-end wiring without real git.
func TestIngestDiffNoBase(t *testing.T) {
	store := filepath.Join(t.TempDir(), "diff.duckdb")
	paymentPath := fauxPaymentPathForTest(t)

	var out, errBuf bytes.Buffer
	code := run([]string{
		"ingest-diff",
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--mode", "faux",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("ingest-diff exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
	if out.String() == "" {
		t.Fatal("ingest-diff produced no output")
	}
}

// TestIngestDiffWithBase verifies that a fake git runner drives per-file ingest.
func TestIngestDiffWithBase(t *testing.T) {
	store := filepath.Join(t.TempDir(), "diff.duckdb")
	paymentPath := fauxPaymentPathForTest(t)

	// Provide two changed files that actually exist in the fixture.
	changedFiles := []string{
		filepath.Join(paymentPath, "payment.go"),
		filepath.Join(paymentPath, "errors.go"),
	}

	var out, errBuf bytes.Buffer
	code := runIngestDiff([]string{
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--base", "HEAD~1",
	}, fakeGitRunner(changedFiles), &out, &errBuf)
	if code != 0 {
		t.Fatalf("runIngestDiff exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
	combined := out.String()
	if !strings.Contains(combined, "changed") && !strings.Contains(combined, "ingest") {
		t.Errorf("output should describe changed files; got: %s", combined)
	}
}

// TestIngestDiffWithBaseNoFiles verifies that an empty diff (no changed files)
// prints a summary and exits 0.
func TestIngestDiffWithBaseNoFiles(t *testing.T) {
	store := filepath.Join(t.TempDir(), "diff.duckdb")
	paymentPath := fauxPaymentPathForTest(t)

	var out, errBuf bytes.Buffer
	code := runIngestDiff([]string{
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--base", "HEAD~1",
	}, fakeGitRunner(nil), &out, &errBuf)
	if code != 0 {
		t.Fatalf("runIngestDiff exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
}
