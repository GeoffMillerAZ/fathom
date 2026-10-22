package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

func TestShellWrappersLoad(t *testing.T) {
	dir := t.TempDir()
	user, repo := filepath.Join(dir, "user.toml"), filepath.Join(dir, "repo.toml")
	writeTOML(t, user, "[shell]\nwrappers = [{ prefix = 'uv run' }]\n")
	writeTOML(t, repo, "[shell]\nwrappers = [{ command = 'command-wrapper', script_arg = 2, cwd_arg = 1 }]\n")
	l, err := Load(user, repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []shellcmd.Wrapper{{Prefix: "uv run"}, {Command: "command-wrapper", ScriptArg: 2, CwdArg: 1}}
	if !reflect.DeepEqual(l.Config.Shell.Wrappers, want) || len(l.IgnoredKeySources) != 0 {
		t.Fatalf("wrappers=%+v ignored=%+v", l.Config.Shell.Wrappers, l.IgnoredKeySources)
	}
	writeTOML(t, repo, "[shell]\nwrappers = []\n")
	l, err = Load(user, repo)
	if err != nil || !reflect.DeepEqual(l.Config.Shell.Wrappers, want[:1]) {
		t.Fatalf("empty repo layer removed user declarations: %+v, %v", l.Config.Shell, err)
	}
}

func TestShellWrappersRejectInvalidForms(t *testing.T) {
	for _, entry := range []string{
		`{}`, `{ prefix = "" }`, `{ prefix = "  " }`,
		`{ command = "command-wrapper" }`, `{ command = "command-wrapper", script_arg = 0 }`,
		`{ command = "command-wrapper", script_arg = 1, cwd_arg = 0 }`,
		`{ prefix = "uv run", command = "command-wrapper", script_arg = 1 }`,
		`{ prefix = "uv run", cwd_arg = 1 }`,
		`{ command = "command-wrapper", script_arg = 1.5 }`,
	} {
		t.Run(entry, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.toml")
			writeTOML(t, p, "[shell]\nwrappers = ["+entry+"]\n")
			if _, err := Load(p, ""); err == nil {
				t.Fatal("invalid wrapper accepted")
			}
		})
	}
}
