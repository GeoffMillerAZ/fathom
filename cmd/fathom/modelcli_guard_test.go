package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestModelCLIGuard_DoctorProbeReachesTheStub (TQ-22): the doctor's real
// `claude --version` probe — the path TestMain stubs out of claudeLoadEnv —
// resolves claude on the isolated PATH to the refusing stub: the stub log
// records the call and the real binary is never reached.
func TestModelCLIGuard_DoctorProbeReachesTheStub(t *testing.T) {
	p, err := exec.LookPath("claude")
	if err != nil {
		t.Fatalf("claude does not resolve on the isolated PATH: %v", err)
	}
	if !strings.Contains(p, "fathom-test-") {
		t.Fatalf("claude resolves to %s, not the testisolate stub", p)
	}
	if v := os.Getenv("FATHOM_CLAUDE_BIN"); v != "" {
		t.Fatalf("FATHOM_CLAUDE_BIN = %q, want unset", v)
	}
	if _, err := probeClaudeVersion(context.Background()); err == nil {
		t.Fatal("the probe succeeded: it reached a real claude")
	}
	raw, err := os.ReadFile(os.Getenv(testisolate.EnvStubLog))
	if err != nil {
		t.Fatalf("no stub log: %v", err)
	}
	if !strings.Contains(string(raw), "claude --version") {
		t.Fatalf("stub log %q does not record claude --version", raw)
	}
	if filepath.Dir(p) != filepath.Join(filepath.Dir(filepath.Dir(p)), "bin") {
		t.Fatalf("stub dir layout changed: %s", p)
	}
}

// TestModelCLIGuard_CodexResolvesToTheStub: the same for codex, the other
// CLI the compile path can start (codexcli.New with an empty Bin).
func TestModelCLIGuard_CodexResolvesToTheStub(t *testing.T) {
	p, err := exec.LookPath("codex")
	if err != nil || !strings.Contains(p, "fathom-test-") {
		t.Fatalf("codex resolves to %q (%v), want the testisolate stub", p, err)
	}
	if v := os.Getenv("FATHOM_CODEX_BIN"); v != "" {
		t.Fatalf("FATHOM_CODEX_BIN = %q, want unset", v)
	}
}
