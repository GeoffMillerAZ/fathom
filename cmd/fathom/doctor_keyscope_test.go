package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorKeyScope_RepoKeySourceNeverRuns(t *testing.T) {
	for _, v := range []string{"TYPESAFE_API_KEY", "OPENROUTER_API_KEY", "OPENAI_API_KEY"} {
		t.Setenv(v, "")
	}
	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	marker := filepath.Join(t.TempDir(), "pwned")
	root := t.TempDir()
	cfg := filepath.Join(root, ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := `["/bin/sh", "-c", "touch ` + marker + `"]`
	body := "[decide]\nprovider = \"openai\"\napi_key_cmd = " + cmd + "\n\n[profiles.x]\nprovider = \"anthropic\"\napi_key_cmd = " + cmd + "\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	row := checkKeyScope(root, filepath.Join(home, "config.toml"))
	if row.Status != doctorWarn {
		t.Fatalf("key_source_scope = %v (%s), want a non-passing row", row.Status, row.Reason)
	}
	for _, want := range []string{cfg, "decide.api_key_cmd", "profiles.x.api_key_cmd"} {
		if !strings.Contains(row.Reason, want) {
			t.Errorf("Reason %q lacks %q", row.Reason, want)
		}
	}
	if strings.Contains(row.Reason, marker) {
		t.Errorf("Reason carries the command: %q", row.Reason)
	}

	// The decision_provider row, over the same repository.
	_ = checkDecisionProvider(root, nil)
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("doctor ran the repository's api_key_cmd: %s exists", marker)
	}
}

func TestDoctorKeyScope_CleanRepoPasses(t *testing.T) {
	home := t.TempDir()
	row := checkKeyScope(t.TempDir(), filepath.Join(home, "config.toml"))
	if row.Status != doctorPass {
		t.Fatalf("status = %v: %s", row.Status, row.Reason)
	}
}

// TestDoctorKeyScope_UserOnlyFieldsAndEscapingPathsNamed is SEC-2 (CF-15):
// the row names every user-only field a repository set and every repo-path
// value that leaves it, by file and field, never a user-only value.
func TestDoctorKeyScope_UserOnlyFieldsAndEscapingPathsNamed(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	cfg := filepath.Join(root, ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[profiles.x]\nprovider = \"codex-cli\"\nbin = \"./evil.sh\"\n\n[daemon]\nstore = \"/elsewhere/REPO_STORE.sqlite\"\nstate_dir = \"workspace\"\n\n[tests]\nresults_dir = \"../outside\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	row := checkKeyScope(root, filepath.Join(home, "config.toml"))
	if row.Status != doctorWarn {
		t.Fatalf("key_source_scope = %v (%s), want WARN", row.Status, row.Reason)
	}
	for _, want := range []string{cfg, "profiles.x.bin", "daemon.store", "tests.results_dir", "escapes the repository"} {
		if !strings.Contains(row.Reason, want) {
			t.Errorf("Reason %q lacks %q", row.Reason, want)
		}
	}
	for _, never := range []string{"evil.sh", "REPO_STORE", "state_dir"} {
		if strings.Contains(row.Reason, never) {
			t.Errorf("Reason carries %q: %q", never, row.Reason)
		}
	}
}
