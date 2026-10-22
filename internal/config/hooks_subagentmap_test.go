package config

import (
	"path/filepath"
	"testing"
)

func TestSubagentMapFactsModes(t *testing.T) {
	dir := t.TempDir()
	user, project := filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml")
	for _, tc := range []struct{ input, want string }{
		{"", ""}, {"false", ""}, {"true", "on"}, {`"observe"`, "observe"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			text := ""
			if tc.input != "" {
				text = "[hooks]\nsubagent_map_facts = " + tc.input + "\n"
			}
			writeTOML(t, project, text)
			loaded, err := Load(user, project)
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Config.Hooks.SubagentMapFacts; got != tc.want {
				t.Errorf("mode = %q, want %q", got, tc.want)
			}
		})
	}
	writeTOML(t, user, "[hooks]\nsubagent_map_facts = true\n")
	writeTOML(t, project, "[hooks]\nsubagent_map_facts = false\n")
	loaded, err := Load(user, project)
	if err != nil || loaded.Config.Hooks.SubagentMapFacts != "" {
		t.Fatalf("project false must override user true: %+v, %v", loaded, err)
	}
	if info := loaded.Provenance["hooks.subagent_map_facts"]; info.Source != SourceProject || info.Path != project {
		t.Errorf("provenance = %+v", info)
	}
	for _, bad := range []string{`"on"`, `"off"`, "1"} {
		writeTOML(t, project, "[hooks]\nsubagent_map_facts = "+bad+"\n")
		if _, err := Load(user, project); err == nil {
			t.Errorf("accepted invalid mode %s", bad)
		}
	}
}
