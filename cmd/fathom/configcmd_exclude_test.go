package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCmdConfig_NamesTheExcludeGlobsUnion: `fathom config` shows the
// [decide] exclude globs as the union of every layer's (I61), so a
// repository's globs never hide the user's.
func TestCmdConfig_NamesTheExcludeGlobsUnion(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	writeUserConfig(t, "[decide]\nexclude = [\"secrets/**\"]\n")
	writeProjectConfig(t, projectDir, "[decide]\nexclude = [\"internal/vault/**\"]\n")
	var stdout, stderr bytes.Buffer
	if code := cmdConfig(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if want := "exclude:         internal/vault/**, secrets/** (every layer's, unioned)  [project"; !strings.Contains(stdout.String(), want) {
		t.Errorf("config text lacks %q:\n%s", want, stdout.String())
	}
	if strings.Count(stdout.String(), "\ndecide:") != 1 {
		t.Errorf("config text has %d decide headers, want 1:\n%s", strings.Count(stdout.String(), "\ndecide:"), stdout.String())
	}
}
