package config

// readouts_test.go — RO-15 (docs/plans/readouts.md, ADR-071): the
// [readouts] keys are the user's own switches. A repository's
// .fathom/config.toml that sets one has it dropped and named, like [decide]
// enabled (I66).

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadoutsFieldsAreUserOnly(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	proj := filepath.Join(dir, "repo", ".fathom", "config.toml")

	t.Run("default is off", func(t *testing.T) {
		loaded, err := Load("", "")
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Config.Readouts.Mode; got != ReadoutsOff {
			t.Errorf("default mode = %q, want %q", got, ReadoutsOff)
		}
	})

	t.Run("user layer sets them", func(t *testing.T) {
		writeTOML(t, user, "[readouts]\nmode = \"lean\"\nroots = [\"/w/a\", \"/w/b\"]\n[readouts.rules.ls-long]\nmode = \"observe\"\n")
		loaded, err := Load(user, "")
		if err != nil {
			t.Fatal(err)
		}
		want := Readouts{Mode: "lean", Roots: []string{"/w/a", "/w/b"}, Rules: map[string]ReadoutRule{"ls-long": {Mode: "observe"}}}
		if !reflect.DeepEqual(loaded.Config.Readouts, want) {
			t.Errorf("Readouts = %+v, want %+v", loaded.Config.Readouts, want)
		}
		if got := loaded.Provenance["readouts.mode"].Source; got != SourceUser {
			t.Errorf("provenance readouts.mode = %v", got)
		}
	})

	t.Run("repository layer is dropped and named", func(t *testing.T) {
		writeTOML(t, user, "[readouts]\nmode = \"observe\"\n[readouts.rules.ls-long]\nmode = \"lean\"\n")
		writeTOML(t, proj, "[readouts]\nmode = \"rich\"\nroots = [\"/x\"]\n[readouts.rules.ls-long]\nmode = \"rich\"\n[readouts.rules.other]\nmode = \"lean\"\n[decide]\nmodel = \"m\"\n")
		loaded, err := Load(user, proj)
		if err != nil {
			t.Fatal(err)
		}
		want := Readouts{Mode: "observe", Rules: map[string]ReadoutRule{"ls-long": {Mode: "lean"}}}
		if !reflect.DeepEqual(loaded.Config.Readouts, want) {
			t.Errorf("Readouts = %+v, want the user's alone %+v", loaded.Config.Readouts, want)
		}
		var fields []string
		for _, ig := range loaded.IgnoredKeySources {
			fields = append(fields, ig.Field)
		}
		wantFields := []string{"readouts.mode", "readouts.roots", "readouts.rules.ls-long.mode", "readouts.rules.other.mode"}
		if !reflect.DeepEqual(fields, wantFields) {
			t.Errorf("IgnoredKeySources fields = %v, want %v", fields, wantFields)
		}
		if loaded.Config.Decide.Model != "m" {
			t.Errorf("a behaviour key beside them must still load: %+v", loaded.Config.Decide)
		}
	})

	t.Run("repository alone leaves the default off", func(t *testing.T) {
		writeTOML(t, proj, "[readouts]\nmode = \"rich\"\n")
		loaded, err := Load("", proj)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Config.Readouts.Mode != ReadoutsOff || len(loaded.IgnoredKeySources) != 1 {
			t.Errorf("Readouts = %+v, ignored = %+v", loaded.Config.Readouts, loaded.IgnoredKeySources)
		}
	})
}
