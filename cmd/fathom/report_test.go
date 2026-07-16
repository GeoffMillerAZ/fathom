package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reportFixtureSrc contains one AWS-key-shaped literal (the same example key
// internal/analysis/security's own tests use) so security_rollup's secret
// scanner has exactly one deterministic finding to bridge.
const reportFixtureSrc = `package infra

const accessKey = "AKIAIOSFODNN7EXAMPLE"
`

func TestReport_SecurityRollup_WritesFindingDocument(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "infra.go"), []byte(reportFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.duckdb")
	outDir := filepath.Join(t.TempDir(), "findings")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "wa-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	var out, errBuf bytes.Buffer
	code := run([]string{
		"report", "--store", storePath, "--service", "wa-svc",
		"--analyzer", "security_rollup", "--out", outDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("report exit = %d, stderr: %s", code, errBuf.String())
	}

	outPath := filepath.Join(outDir, "wa-svc-security_rollup.md")
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	doc := string(body)

	if !strings.Contains(doc, "security_rollup findings — wa-svc") {
		t.Errorf("doc missing header: %s", doc)
	}
	if !strings.Contains(doc, "SEC-001: Hardcoded secrets") {
		t.Errorf("doc missing the secrets section: %s", doc)
	}
	if !strings.Contains(doc, "infra.go:3") {
		t.Errorf("doc missing the file:line anchor: %s", doc)
	}
	if strings.Contains(doc, "SEC-002") || strings.Contains(doc, "SEC-003") {
		t.Errorf("doc should omit categories with zero findings: %s", doc)
	}

	// Determinism: re-running against an unchanged index reproduces the
	// identical bytes — the findings compiler's digest gate depends on this.
	var out2, errBuf2 bytes.Buffer
	code2 := run([]string{
		"report", "--store", storePath, "--service", "wa-svc",
		"--analyzer", "security_rollup", "--out", outDir,
	}, &out2, &errBuf2)
	if code2 != 0 {
		t.Fatalf("second report exit = %d, stderr: %s", code2, errBuf2.String())
	}
	body2, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s (2nd run): %v", outPath, err)
	}
	if !bytes.Equal(body, body2) {
		t.Errorf("report output not byte-identical across runs:\nfirst:\n%s\nsecond:\n%s", body, body2)
	}
}

func TestReport_Errors(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.duckdb")

	cases := []struct {
		name string
		args []string
	}{
		{"missing store", []string{"report", "--service", "svc"}},
		{"missing service", []string{"report", "--store", storePath}},
		{"unsupported analyzer", []string{"report", "--store", storePath, "--service", "svc", "--analyzer", "dead_code"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			if code := run(tc.args, &out, &errBuf); code == 0 {
				t.Errorf("exit = 0, want non-zero; stderr: %s", errBuf.String())
			}
		})
	}
}
