package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLcov = `SF:src/widget.py
DA:1,1
DA:2,0
end_of_record
`

func writeTempLcov(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "coverage.info")
	if err := os.WriteFile(path, []byte(sampleLcov), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestCmdCoverageIngestBindsCommit(t *testing.T) {
	path := writeTempLcov(t)

	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{"--format", "lcov", "--commit", "deadbeef", "--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdCoverageIngest (with --commit) = %d, stderr: %s", code, stderr.String())
	}
	var report coverageIngestReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v (stdout: %s)", err, stdout.String())
	}
	if !report.Bound || report.Commit != "deadbeef" {
		t.Errorf("report = %+v, want Bound=true Commit=deadbeef", report)
	}
	if report.Blocks != 2 || report.Statements != 2 || report.Covered != 1 || report.Files != 1 {
		t.Errorf("report totals = %+v, want Blocks=2 Statements=2 Covered=1 Files=1", report)
	}
}

func TestCmdCoverageIngestUnboundWithoutCommit(t *testing.T) {
	path := writeTempLcov(t)

	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{"--format", "lcov", "--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdCoverageIngest (no --commit) = %d, stderr: %s", code, stderr.String())
	}
	var report coverageIngestReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Bound || report.Commit != "unbound" {
		t.Errorf("report = %+v, want Bound=false Commit=unbound", report)
	}
}

func TestCmdCoverageIngestUnboundNeverFails(t *testing.T) {
	// "never gating" (design doc §3 T3): an unbound report still exits 0,
	// human-readable output included.
	path := writeTempLcov(t)
	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{"--format", "lcov", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (never gating), stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "unbound") {
		t.Errorf("stdout = %q, want it to name \"unbound\"", stdout.String())
	}
}

func TestCmdCoverageIngestRequiresFormat(t *testing.T) {
	path := writeTempLcov(t)
	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{path}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit when --format is omitted")
	}
}

func TestCmdCoverageIngestRequiresFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{"--format", "lcov"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit when no file is named")
	}
}

func TestCmdCoverageIngestOversizeRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.info")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(coverageIngestMaxBytes + 1); err != nil {
		f.Close()
		t.Fatalf("truncate: %v", err)
	}
	f.Close()

	var stdout, stderr bytes.Buffer
	code := cmdCoverageIngest([]string{"--format", "lcov", path}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit for a report over the 256 MiB cap")
	}
	if !strings.Contains(stderr.String(), "cap") {
		t.Errorf("stderr = %q, want it to name the cap", stderr.String())
	}
}

func TestBoundedReader_CapEnforced(t *testing.T) {
	br := &boundedReader{r: strings.NewReader("0123456789"), max: 4}
	buf := make([]byte, 100)
	total := 0
	var lastErr error
	for {
		n, err := br.Read(buf[total:])
		total += n
		if err != nil {
			lastErr = err
			break
		}
	}
	if lastErr == nil {
		t.Fatal("expected boundedReader to error once past its cap")
	}
	if total > 4 {
		t.Fatalf("read %d bytes past a cap of 4", total)
	}
}
