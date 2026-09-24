package main

// certify_manifest_test.go — TC-12's own test:
// TestCmdCertifyManifest_RegeneratedAndCheck. Reuses
// setUpCertifyGapsFixture (certify_gaps_test.go): one live certificate,
// one declared-but-not-live gap and one corpus-only gap.
import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdCertifyManifest_RegeneratedAndCheck(t *testing.T) {
	_, storePath, service := setUpCertifyGapsFixture(t)
	out := filepath.Join(t.TempDir(), "certifications.md")

	t.Run("generates a Markdown artifact with a generated-header and one table per kind", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", out}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("certify manifest exit = %d, stderr: %s", code, stderr.String())
		}
		body, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read generated file: %v", err)
		}
		content := string(body)
		if !strings.HasPrefix(content, certifyManifestHeader) {
			t.Fatalf("generated file must open with the generated-header, got:\n%s", content)
		}
		if !strings.Contains(content, "## incident") || !strings.Contains(content, "## invariant") {
			t.Fatalf("expected one table per kind (incident, invariant), got:\n%s", content)
		}
		if !strings.Contains(content, "I1") || !strings.Contains(content, "I2") || !strings.Contains(content, "INC-1") {
			t.Fatalf("expected every target listed, got:\n%s", content)
		}
	})

	t.Run("--check passes when the committed file is current", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", out, "--check"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("certify manifest --check exit = %d, stderr: %s", code, stderr.String())
		}
	})

	t.Run("--check fails when a hand edit makes the committed file differ", func(t *testing.T) {
		if err := os.WriteFile(out, []byte("hand-edited nonsense\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", out, "--check"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("certify manifest --check exit = %d, want 1 (stale), stderr: %s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "stale") {
			t.Errorf("expected a 'stale' message, got: %s", stderr.String())
		}
	})

	t.Run("--check fails outright when the file does not exist", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.md")
		var stdout, stderr bytes.Buffer
		code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", missing, "--check"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("certify manifest --check on a missing file exit = %d, want 1", code)
		}
	})

	t.Run("regenerating is idempotent (deterministic output)", func(t *testing.T) {
		var stdout1, stderr1 bytes.Buffer
		if code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", out}, &stdout1, &stderr1); code != 0 {
			t.Fatalf("certify manifest exit = %d, stderr: %s", code, stderr1.String())
		}
		first, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var stdout2, stderr2 bytes.Buffer
		if code := cmdCertifyManifest([]string{"--service", service, "--store", storePath, "--out", out}, &stdout2, &stderr2); code != 0 {
			t.Fatalf("certify manifest exit = %d, stderr: %s", code, stderr2.String())
		}
		second, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if string(first) != string(second) {
			t.Fatalf("regenerating from an unchanged store must be byte-identical:\n--- first ---\n%s\n--- second ---\n%s", first, second)
		}
	})
}
