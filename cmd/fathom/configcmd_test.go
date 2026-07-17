package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withIsolatedConfigHome sets HOME to a fresh temp dir and chdirs into a
// second fresh temp dir (the "project"), so config.DefaultUserPath() and
// config.DefaultProjectPath() resolve into locations this test fully
// controls — no ambient ~/.fathom/config.toml or repo-root .fathom/
// config.toml can leak in. Returns (userDir, projectDir); write
// "<dir>/.fathom/config.toml" under either to seed a layer.
func withIsolatedConfigHome(t *testing.T) (userDir, projectDir string) {
	t.Helper()
	clearProfileEnv(t)

	userDir = t.TempDir()
	t.Setenv("HOME", userDir)

	projectDir = t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	return userDir, projectDir
}

func writeProjectConfig(t *testing.T, projectDir, content string) {
	t.Helper()
	dir := filepath.Join(projectDir, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCmdConfig_NoFiles_TextOutput(t *testing.T) {
	withIsolatedConfigHome(t)

	var stdout, stderr bytes.Buffer
	code := cmdConfig(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "not found") {
		t.Errorf("output = %s, want it to note the absent config files", stdout.String())
	}
	if !strings.Contains(stdout.String(), "anthropic") {
		t.Errorf("output = %s, want the resolved (default) provider", stdout.String())
	}
}

func TestCmdConfig_Explain_AnnotatesSource(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeProjectConfig(t, projectDir, `
default_profile = "work"

[profiles.work]
provider = "bedrock"
region = "us-gov-west-1"
`)
	var stdout, stderr bytes.Buffer
	code := cmdConfig([]string{"--explain"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bedrock") {
		t.Errorf("output = %s, want the resolved provider", out)
	}
	if !strings.Contains(out, "project:") {
		t.Errorf("output = %s, want a [project:<path>] provenance annotation", out)
	}
}

func TestCmdConfig_JSON(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeProjectConfig(t, projectDir, `
default_profile = "work"

[profiles.work]
provider = "openai"
model = "gpt-4o"
`)
	var stdout, stderr bytes.Buffer
	code := cmdConfig([]string{"--json", "--explain"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if payload["default_profile"] != "work" {
		t.Errorf("default_profile = %v", payload["default_profile"])
	}
	profile, _ := payload["profile"].(map[string]any)
	if profile["provider"] != "openai" || profile["model"] != "gpt-4o" {
		t.Errorf("profile = %v", profile)
	}
	explain, ok := payload["explain"].(map[string]any)
	if !ok {
		t.Fatal("explain missing from JSON output")
	}
	if explain["provider"] == nil {
		t.Error("explain.provider missing")
	}
}

func TestCmdConfig_UnknownProfile_Errors(t *testing.T) {
	withIsolatedConfigHome(t)
	var stdout, stderr bytes.Buffer
	code := cmdConfig([]string{"--profile", "does-not-exist"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a nonzero exit for an unknown --profile")
	}
	if !strings.Contains(stderr.String(), "does-not-exist") {
		t.Errorf("stderr = %s, want it to name the bad profile", stderr.String())
	}
}

func TestCmdConfig_EnvOverlayVisibleInExplain(t *testing.T) {
	withIsolatedConfigHome(t)
	t.Setenv("FATHOM_LLM_PROVIDER", "codex-cli")

	var stdout, stderr bytes.Buffer
	code := cmdConfig([]string{"--explain"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "codex-cli") {
		t.Errorf("output = %s, want the env-overridden provider", out)
	}
	if !strings.Contains(out, "env:FATHOM_LLM_PROVIDER") {
		t.Errorf("output = %s, want the env provenance naming FATHOM_LLM_PROVIDER", out)
	}
}
