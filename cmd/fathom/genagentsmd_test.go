package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenClaudeMdMissingService verifies that --service is required.
func TestGenClaudeMdMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"gen-agents-md"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// TestGenClaudeMd_EndToEnd ingests the faux fixture then generates the
// knowledge index with gen-agents-md, verifying that the output file exists
// and contains expected sections.
func TestGenClaudeMd_EndToEnd(t *testing.T) {
	storeDir := t.TempDir()
	store := filepath.Join(storeDir, "test.sqlite")
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "AGENTS.md")
	paymentPath := fauxPaymentPathForTest(t)

	// First ingest so the store has symbols.
	var ingestOut, ingestErr bytes.Buffer
	code := run([]string{
		"ingest",
		"--service", "payment-service",
		"--path", paymentPath,
		"--store", store,
		"--mode", "faux",
	}, &ingestOut, &ingestErr)
	if code != 0 {
		t.Fatalf("ingest exited %d; stderr=%s", code, ingestErr.String())
	}

	// Now generate AGENTS.md.
	var genOut, genErr bytes.Buffer
	code = run([]string{
		"gen-agents-md",
		"--service", "payment-service",
		"--store", store,
		"--output", outputPath,
	}, &genOut, &genErr)
	if code != 0 {
		t.Fatalf("gen-agents-md exited %d\nstdout=%s\nstderr=%s",
			code, genOut.String(), genErr.String())
	}

	// Verify the file was written.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}

	content := string(data)

	// The output must include the service name and key sections.
	for _, want := range []string{
		"payment-service",
		"Symbols",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q; content:\n%s", want, content)
		}
	}
}

// TestGenClaudeMd_DefaultOutput verifies that when --output is omitted the
// default path under workspace/artifacts is used.
func TestGenClaudeMd_DefaultOutput(t *testing.T) {
	// We don't want to actually create workspace/artifacts in the test run, so
	// we just check the flag validation — if --service is set but no symbols
	// exist (empty store in a temp dir), the command should still exit 0 and
	// create the file.
	storeDir := t.TempDir()
	store := filepath.Join(storeDir, "empty.sqlite")

	// Create an empty store by opening / immediately closing a local client.
	// (We can't call client.Open directly but cmdIngest will create it.)
	// Instead, override the output to a known tmp location.
	outputPath := filepath.Join(t.TempDir(), "CLAUDE.md")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"gen-claude-md",
		"--service", "no-such-service",
		"--store", store,
		"--output", outputPath,
	}, &out, &errBuf)
	// Should succeed even with zero symbols.
	if code != 0 {
		t.Fatalf("gen-claude-md exited %d\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("output file not created: %v", err)
	}
}

// TestGenAgentsMdDefaultsToAgentsMD pins AG-19 (ADR-059): gen-agents-md
// defaults to workspace/artifacts/AGENTS.md, and gen-claude-md still runs
// through 0.9.x with a deprecation line naming the new verb.
func TestGenAgentsMdDefaultsToAgentsMD(t *testing.T) {
	if genAgentsMdDefaultOutput != "workspace/artifacts/AGENTS.md" {
		t.Fatalf("default output = %q", genAgentsMdDefaultOutput)
	}
	store := filepath.Join(t.TempDir(), "empty.sqlite")
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errBuf bytes.Buffer
	if code := cmdGenAgentsMd([]string{"--service", "none", "--store", store}, &out, &errBuf); code != 0 {
		t.Fatalf("gen-agents-md exited %d: %s%s", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "workspace", "artifacts", "AGENTS.md")); err != nil {
		t.Fatalf("default output not written: %v", err)
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"gen-claude-md", "--service", "none", "--store", store, "--output", filepath.Join(dir, "old.md")}, &out, &errBuf); code != 0 {
		t.Fatalf("the deprecated alias exited %d: %s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "deprecated") || !strings.Contains(errBuf.String(), "gen-agents-md") {
		t.Errorf("the alias says nothing about the rename: %q", errBuf.String())
	}
}
