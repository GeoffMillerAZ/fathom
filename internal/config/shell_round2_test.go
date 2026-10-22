package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellWrappersValidateLiteralPrefixes(t *testing.T) {
	for _, prefix := range []string{"X=1 tool run", "env X=1 tool run", "tool > output", "tool | other", "tool && other", "tool;", "tool $ARG", "tool $(echo run)", "tool *.go", "(tool)", "tool `echo run`"} {
		t.Run(prefix, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.toml")
			writeTOML(t, file, fmt.Sprintf("[shell]\nwrappers = [{prefix = %q}]\n", prefix))
			_, err := Load(file, "")
			if err == nil || !strings.Contains(err.Error(), "shell.wrappers[0]") {
				t.Fatalf("prefix=%q error=%v; want entry-specific error", prefix, err)
			}
		})
	}
	file := filepath.Join(t.TempDir(), "config.toml")
	writeTOML(t, file, `[shell]
wrappers = [{command = "command-wrapper", script_arg = 1, cwd_arg = 1}]
`)
	if _, err := Load(file, ""); err == nil || !strings.Contains(err.Error(), "shell.wrappers") {
		t.Fatalf("same script and cwd index accepted: %v", err)
	}
}
