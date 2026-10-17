package config

// consent_test.go — KS-3 (docs/plans/decision-port.md, ADR-073 decision 5,
// I66): the decision port's consent and kill fields are user-only. A
// repository's .fathom/config.toml that sets one has it dropped and named
// (IgnoredKeySources: doctor's key_source_scope row and `fathom config`
// print it); the user layer's own value is honoured.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDecideConsentFieldsAreUserOnly(t *testing.T) {
	fields := []string{"decide.allow_repo_provider", "decide.deny_sites", "decide.enabled", "decide.session_sites"}
	for _, f := range fields {
		fs, ok := ScopeOf(f)
		if !ok || fs.Class != ScopeUserOnly {
			t.Errorf("ScopeOf(%s) = %+v, %v; want user-only", f, fs, ok)
		}
	}

	consentTOML := `[decide]
provider = "typesafe"
enabled = false
allow_repo_provider = true
session_sites = ["readout_mode"]
deny_sites = ["assess"]
`
	write := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("a repository's consent fields are dropped and named", func(t *testing.T) {
		dir := t.TempDir()
		proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
		write(t, proj, consentTOML)
		loaded, err := Load(filepath.Join(dir, "absent-user.toml"), proj)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		d := loaded.Config.Decide
		if d.Disabled || d.AllowRepoProvider || len(d.SessionSites) > 0 || len(d.DenySites) > 0 {
			t.Errorf("a repository set a consent field: Disabled=%v AllowRepoProvider=%v SessionSites=%v DenySites=%v", d.Disabled, d.AllowRepoProvider, d.SessionSites, d.DenySites)
		}
		var named []string
		for _, ig := range loaded.IgnoredKeySources {
			named = append(named, ig.Field)
			if ig.Path != proj {
				t.Errorf("IgnoredKeySource %s names %s, want the repository file %s", ig.Field, ig.Path, proj)
			}
		}
		for _, f := range fields {
			if !slices.Contains(named, f) {
				t.Errorf("IgnoredKeySources = %v, want it to name %s", named, f)
			}
		}
		joined := strings.Join(loaded.Warnings(), "\n")
		for _, f := range fields {
			if !strings.Contains(joined, f) {
				t.Errorf("Warnings() does not name %s:\n%s", f, joined)
			}
		}
		if d.ProviderSource != SourceProject || d.UserProvider != "" {
			t.Errorf("ProviderSource = %q UserProvider = %q, want project and none (the layer that named the provider is recorded)", d.ProviderSource, d.UserProvider)
		}
	})

	t.Run("the user layer's consent fields are honoured", func(t *testing.T) {
		dir := t.TempDir()
		user := filepath.Join(dir, "user.toml")
		write(t, user, consentTOML+"transport = \"openrouter\"\n")
		loaded, err := Load(user, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		d := loaded.Config.Decide
		if !d.Disabled || !d.AllowRepoProvider || !slices.Equal(d.SessionSites, []string{"readout_mode"}) || !slices.Equal(d.DenySites, []string{"assess"}) {
			t.Errorf("user layer not honoured: Disabled=%v AllowRepoProvider=%v SessionSites=%v DenySites=%v", d.Disabled, d.AllowRepoProvider, d.SessionSites, d.DenySites)
		}
		if d.ProviderSource != SourceUser || d.UserProvider != "typesafe" || d.TransportSource != SourceUser || d.UserTransport != "openrouter" {
			t.Errorf("layer record = provider %q/%q transport %q/%q, want the user layer's", d.ProviderSource, d.UserProvider, d.TransportSource, d.UserTransport)
		}
		if len(loaded.IgnoredKeySources) != 0 {
			t.Errorf("IgnoredKeySources = %+v, want none for the user's own file", loaded.IgnoredKeySources)
		}
		if loaded.Provenance["decide.enabled"].Source != SourceUser {
			t.Errorf("provenance of decide.enabled = %+v, want user", loaded.Provenance["decide.enabled"])
		}
	})

	t.Run("enabled defaults on and a missing block changes nothing", func(t *testing.T) {
		loaded, err := Load("", "")
		if err != nil {
			t.Fatal(err)
		}
		d := loaded.Config.Decide
		if d.Disabled || d.AllowRepoProvider || d.SessionSites != nil || d.DenySites != nil || d.ProviderSource != "" {
			t.Errorf("unconfigured Decide = %+v, want the defaults (enabled, no consent, no lists, no layer)", d)
		}
	})

	t.Run("a session site in any layer's sites is dropped and named", func(t *testing.T) {
		dir := t.TempDir()
		user := filepath.Join(dir, "user.toml")
		proj := filepath.Join(dir, "repo", ".fathom", "config.toml")
		write(t, user, "[decide]\nsites = [\"compile_prefilter\", \"compact_advice\"]\n")
		write(t, proj, "[decide]\nsites = [\"route\", \"readout_mode\"]\n")
		loaded, err := Load(user, proj)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		d := loaded.Config.Decide
		if !slices.Equal(d.Sites, []string{"route"}) || !slices.Equal(d.RepoSites, []string{"route"}) {
			t.Errorf("Sites = %v RepoSites = %v, want [route] (readout_mode dropped)", d.Sites, d.RepoSites)
		}
		var got []string
		for _, ds := range loaded.DroppedSessionSites {
			got = append(got, ds.Path+":"+ds.Field)
		}
		want := []string{user + ":decide.sites[compact_advice]", proj + ":decide.sites[readout_mode]"}
		if !slices.Equal(got, want) {
			t.Errorf("DroppedSessionSites = %v, want %v", got, want)
		}
		joined := strings.Join(loaded.Warnings(), "\n")
		if !strings.Contains(joined, "readout_mode") || !strings.Contains(joined, "session_sites") {
			t.Errorf("Warnings() does not name the dropped session site and where it belongs:\n%s", joined)
		}
	})
}
