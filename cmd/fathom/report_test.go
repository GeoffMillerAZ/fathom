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
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
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

// TestReport_SecurityRollup_InsideCorpus_NoSelfScanFeedback is the review
// blocker: --out inside the repo root is the INTENDED deployment (the doc
// lands in the corpus dir so the findings layer compiles it), and there the
// report's own output must be excluded from the scan. Without the exclusion,
// run 2 re-discovers the run-1 doc via Sources() (which walks the live tree),
// the shell-out scanner matches its own `subprocess.call(` snippet embedded
// in the doc, and every run mutates the doc — shifting line numbers, breaking
// the findings layer's digest gate, and flipping compiled claims stale on
// every refresh.
func TestReport_SecurityRollup_InsideCorpus_NoSelfScanFeedback(t *testing.T) {
	root := t.TempDir()
	// A python fixture whose finding MESSAGE embeds the matched snippet
	// ("shell-out pattern: subprocess.call(") — the exact self-scan vector.
	pySrc := "import subprocess\n\nsubprocess.call([\"ls\"])\n"
	if err := os.WriteFile(filepath.Join(root, "tools.py"), []byte(pySrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	outDir := filepath.Join(root, "findings") // INSIDE the corpus root

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "wa-loop", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	reportArgs := []string{
		"report", "--store", storePath, "--service", "wa-loop",
		"--analyzer", "security_rollup", "--out", outDir,
	}
	var out1, err1 bytes.Buffer
	if code := run(reportArgs, &out1, &err1); code != 0 {
		t.Fatalf("first report exit = %d, stderr: %s", code, err1.String())
	}
	outPath := filepath.Join(outDir, "wa-loop-security_rollup.md")
	body1, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	if !strings.Contains(string(body1), "tools.py:3") {
		t.Fatalf("first doc missing the fixture shell-out finding: %s", body1)
	}

	// Second run: Sources() now sees the run-1 doc on disk. The doc must not
	// scan itself — output stays byte-identical.
	var out2, err2 bytes.Buffer
	if code := run(reportArgs, &out2, &err2); code != 0 {
		t.Fatalf("second report exit = %d, stderr: %s", code, err2.String())
	}
	body2, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s (2nd run): %v", outPath, err)
	}
	if !bytes.Equal(body1, body2) {
		t.Errorf("self-scan feedback loop: report inside the corpus is not byte-identical across runs:\nfirst:\n%s\nsecond:\n%s", body1, body2)
	}
	if strings.Contains(string(body2), "findings/wa-loop-security_rollup.md") {
		t.Errorf("doc contains findings anchored in itself: %s", body2)
	}
}

// TestReport_SecurityRollup_RedactsSecretValues: the secret-assignment
// scanner's Message embeds the whole matched source line — verbatim
// credentials must never be exfiltrated into a committable finding document.
// The bridge renders secret-category findings as path:line + rule only.
func TestReport_SecurityRollup_RedactsSecretValues(t *testing.T) {
	root := t.TempDir()
	const secretValue = "supersecret9hunter2"
	pySrc := "password = \"" + secretValue + "\"\n"
	if err := os.WriteFile(filepath.Join(root, "config.py"), []byte(pySrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	outDir := filepath.Join(t.TempDir(), "findings")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "wa-redact", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}
	var out, errBuf bytes.Buffer
	code := run([]string{
		"report", "--store", storePath, "--service", "wa-redact",
		"--analyzer", "security_rollup", "--out", outDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("report exit = %d, stderr: %s", code, errBuf.String())
	}
	body, err := os.ReadFile(filepath.Join(outDir, "wa-redact-security_rollup.md"))
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	doc := string(body)
	if !strings.Contains(doc, "config.py:1") || !strings.Contains(doc, "secret-assignment") {
		t.Fatalf("doc missing the redacted finding anchor (path:line + rule): %s", doc)
	}
	if strings.Contains(doc, secretValue) {
		t.Errorf("doc exfiltrates the secret value verbatim: %s", doc)
	}
}

func TestReport_Errors(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

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
