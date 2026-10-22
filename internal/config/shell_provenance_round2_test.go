package config

import (
	"path/filepath"
	"testing"
)

func TestShellWrapperProvenanceKeepsEachContributor(t *testing.T) {
	dir := t.TempDir()
	user, project := filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml")
	writeTOML(t, user, "[shell]\nwrappers = [{prefix = 'uv run'}]\n")
	for _, body := range []string{"[shell]\nwrappers = [{prefix = 'bundle exec'}]\n", "[shell]\nwrappers = []\n"} {
		writeTOML(t, project, body)
		l, err := Load(user, project)
		if err != nil {
			t.Fatal(err)
		}
		if got := l.Provenance["shell.wrappers[0]"]; got.Source != SourceUser || got.Path != user {
			t.Errorf("user entry provenance=%+v", got)
		}
		if len(l.Config.Shell.Wrappers) == 2 {
			if got := l.Provenance["shell.wrappers[1]"]; got.Source != SourceProject || got.Path != project {
				t.Errorf("project entry provenance=%+v", got)
			}
		} else if got := l.Provenance["shell.wrappers"]; got.Source != SourceUser {
			t.Errorf("empty project list took user provenance: %+v", got)
		}
	}
}
