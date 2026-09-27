package config

// The wave-B live reading's F8: with the working directory at $HOME, the
// "project" config is ~/.fathom/config.toml itself. It is the user layer,
// read once — never also named an ignored repository [tenancy].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserConfigAsProjectIsNotAnIgnoredRepository(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("[tenancy]\nbudget = \"2GiB\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The same file reached by another spelling (a symlinked home).
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	for _, proj := range []string{p, filepath.Join(link, ".fathom", "config.toml")} {
		loaded, err := Load(p, proj)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.ProjectTenancyIgnored {
			t.Fatalf("project %s is the user config; its [tenancy] was named an ignored repository table", proj)
		}
		_, notes, err := ResolveTenancyConfigNotes(loaded, func(string) string { return "" })
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range notes {
			if n.Kind == NoteProjectIgnored || strings.Contains(n.Text, "ignored") {
				t.Fatalf("note %q for the user's own config", n.Text)
			}
		}
	}
}
