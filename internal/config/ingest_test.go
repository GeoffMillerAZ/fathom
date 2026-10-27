package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIngestGoNetworkIsUserOnly (ADR-083): `[ingest] go_network = true`
// reopens the module network for the go commands fathom runs on its own,
// so only the user's ~/.fathom/config.toml may set it. A repository's copy
// is dropped and named; the user's copy is read, and UserGoNetwork reads
// nothing else.
func TestIngestGoNetworkIsUserOnly(t *testing.T) {
	if RepoMaySet("ingest.go_network") {
		t.Fatal("ingest.go_network: a repository may set it, want user-only")
	}
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	project := filepath.Join(dir, "project.toml")
	if err := os.WriteFile(project, []byte("[ingest]\ngo_network = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Load(user, project)
	if err != nil {
		t.Fatal(err)
	}
	if l.Config.Ingest.GoNetwork {
		t.Fatal("a repository's [ingest] go_network = true was merged")
	}
	named := false
	for _, ig := range l.IgnoredKeySources {
		named = named || ig.Field == "ingest.go_network"
	}
	if !named {
		t.Fatalf("the dropped ingest.go_network is not named: %+v", l.IgnoredKeySources)
	}

	if err := os.WriteFile(user, []byte("[ingest]\ngo_network = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err = Load(user, ""); err != nil || !l.Config.Ingest.GoNetwork {
		t.Fatalf("the user's go_network = true: GoNetwork=%v err=%v, want true", l.Config.Ingest.GoNetwork, err)
	}
	if got := l.Provenance["ingest.go_network"]; got.Source != SourceUser {
		t.Fatalf("provenance of ingest.go_network = %+v, want the user layer", got)
	}

	home := t.TempDir()
	t.Setenv("FATHOM_HOME", home)
	if UserGoNetwork() {
		t.Fatal("UserGoNetwork with no user config = true, want false (offline)")
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[ingest]\ngo_network = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !UserGoNetwork() {
		t.Fatal("UserGoNetwork with the user's go_network = true = false")
	}
}
