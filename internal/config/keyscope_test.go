package config

// keyscope_test.go — SEC-1 (ADR-042 amendment 2026-10-09, DP-40): the key
// source fields are the USER's. A repository's .fathom/config.toml that
// sets api_key_cmd, api_key_file or api_key_env (in [decide] or a profile)
// is dropped at Load with a warning naming file + field, and its command is
// never run.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pwnedMarker(t *testing.T) (marker, cmdLine string) {
	t.Helper()
	marker = filepath.Join(t.TempDir(), "pwned")
	return marker, `["/bin/sh", "-c", "touch ` + marker + `"]`
}

func assertNoMarker(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("a repository's api_key_cmd ran: %s exists", marker)
	}
}

func TestLoad_RepoKeySourcesAreDroppedAndNamed(t *testing.T) {
	marker, cmd := pwnedMarker(t)
	dir := t.TempDir()
	proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
	writeTOML(t, proj, `
[decide]
provider    = "openai"
api_key_cmd = `+cmd+`
api_key_file = "~/keys/x"
api_key_env  = "REPO_CHOSEN"

[profiles.x]
provider    = "anthropic"
api_key_cmd = `+cmd+`
api_key_env = "REPO_CHOSEN"
`)
	loaded, err := Load("", proj)
	if err != nil {
		t.Fatal(err)
	}
	d := loaded.Config.Decide
	if d.Provider != "openai" {
		t.Errorf("provider = %q, want the repo layer to still choose the provider", d.Provider)
	}
	if len(d.APIKeyCmd) != 0 || d.APIKeyFile != "" || d.APIKeyEnv != "" {
		t.Errorf("Decide key fields survived from the repo layer: %+v", d)
	}
	if src := d.KeySource(); !src.Default {
		t.Errorf("KeySource = %+v, want the transport's default variable", src)
	}
	p := loaded.Config.Profiles["x"]
	if len(p.APIKeyCmd) != 0 || p.APIKeyEnv != "" || p.Provider != "anthropic" {
		t.Errorf("profile = %+v, want key fields dropped, provider kept", p)
	}
	for _, k := range []string{"decide.api_key_cmd", "decide.api_key_file", "decide.api_key_env", "profiles.x.api_key_cmd", "profiles.x.api_key_env"} {
		if _, ok := loaded.Provenance[k]; ok {
			t.Errorf("provenance[%s] set for a dropped field", k)
		}
	}
	got := map[string]bool{}
	for _, ig := range loaded.IgnoredKeySources {
		if ig.Path != proj {
			t.Errorf("ignored path = %q, want %q", ig.Path, proj)
		}
		got[ig.Field] = true
	}
	for _, f := range []string{"decide.api_key_cmd", "decide.api_key_file", "decide.api_key_env", "profiles.x.api_key_cmd", "profiles.x.api_key_env"} {
		if !got[f] {
			t.Errorf("IgnoredKeySources missing %s: %+v", f, loaded.IgnoredKeySources)
		}
	}
	for _, w := range loaded.Warnings() {
		if strings.Contains(w, "REPO_CHOSEN") || strings.Contains(w, marker) || strings.Contains(w, "~/keys/x") {
			t.Errorf("warning carries a value: %q", w)
		}
		if !strings.Contains(w, proj) {
			t.Errorf("warning does not name the file: %q", w)
		}
	}
	if len(loaded.Warnings()) != 5 {
		t.Errorf("warnings = %v, want 5", loaded.Warnings())
	}

	// The real entry points: neither resolver runs the repo's command.
	if _, err := d.ResolveAPIKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ResolveAPIKey(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertNoMarker(t, marker)
}

func TestLoad_UserKeySourcesStillHonored(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
	writeTOML(t, user, "[decide]\nprovider = \"openai\"\napi_key_env = \"MY_USER_KEY\"\n[profiles.x]\nprovider = \"anthropic\"\napi_key_env = \"MY_USER_KEY\"\n")
	writeTOML(t, proj, "[decide]\nprovider = \"openai\"\napi_key_env = \"REPO_CHOSEN\"\nmodel = \"m\"\n")
	loaded, err := Load(user, proj)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Config.Decide.APIKeyEnv != "MY_USER_KEY" || loaded.Config.Decide.Model != "m" {
		t.Errorf("Decide = %+v, want the user's key source and the repo's model", loaded.Config.Decide)
	}
	if loaded.Config.Decide.KeyTransport != DecideTransportOpenAI {
		t.Errorf("KeyTransport = %q", loaded.Config.Decide.KeyTransport)
	}
	if loaded.Config.Profiles["x"].APIKeyEnv != "MY_USER_KEY" {
		t.Errorf("profile = %+v", loaded.Config.Profiles["x"])
	}
	if len(loaded.IgnoredKeySources) != 1 || loaded.IgnoredKeySources[0].Field != "decide.api_key_env" {
		t.Errorf("IgnoredKeySources = %+v", loaded.IgnoredKeySources)
	}
}

func TestResolveAPIKey_UserCmdRunsOncePerProcess(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	script := `echo x >> ` + counter + `; echo the-key`
	argv := []string{"/bin/sh", "-c", script}
	d := Decide{Provider: "openai", APIKeyCmd: argv}
	p := Profile{APIKeyCmd: argv}
	for i := 0; i < 3; i++ {
		k, err := d.ResolveAPIKey(context.Background())
		if err != nil || k != "the-key" {
			t.Fatalf("decide key = %q, %v", k, err)
		}
		k, err = p.ResolveAPIKey(context.Background())
		if err != nil || k != "the-key" {
			t.Fatalf("profile key = %q, %v", k, err)
		}
		if _, err := d.KeySource().Probe(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(counter)
	if n := strings.Count(string(b), "x"); n != 1 {
		t.Fatalf("api_key_cmd ran %d times, want exactly once", n)
	}
}
