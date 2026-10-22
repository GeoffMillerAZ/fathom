package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConfig_HooksDedupe(t *testing.T) {
	dir := t.TempDir()
	user, project := filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml")
	check := func(want bool) {
		t.Helper()
		loaded, err := Load(user, project)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Config.Hooks.Dedupe; got != want {
			t.Fatalf("Hooks.Dedupe = %v, want %v", got, want)
		}
	}
	check(true)
	writeTOML(t, user, "[hooks]\ndedupe = false\n")
	check(false)
	writeTOML(t, project, "[hooks]\ndedupe = true\n")
	check(true)
	loaded, _ := Load(user, project)
	if got := loaded.Provenance["hooks.dedupe"]; got.Source != SourceProject || got.Path != project {
		t.Errorf("provenance = %+v", got)
	}
	writeTOML(t, project, "[hooks]\ndedupe = \"no\"\n")
	if _, err := Load(user, project); err == nil || !strings.Contains(err.Error(), "dedupe") {
		t.Fatalf("non-bool dedupe accepted: %v", err)
	}
}
