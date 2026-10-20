package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestConfig_HooksTestsToRunDefaultOff pins RO-43's key: `[hooks]
// tests_to_run` is off until a layer sets it, a repository may set it (the
// behaviour class), and a non-bool is refused naming the key.
func TestConfig_HooksTestsToRunDefaultOff(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Config.Hooks.TestsToRun {
		t.Error("tests_to_run is on with no layer setting it")
	}
	if _, ok := loaded.Provenance["hooks.tests_to_run"]; ok {
		t.Error("hooks.tests_to_run has provenance with no layer setting it")
	}
	if !RepoMaySet("hooks.tests_to_run") {
		t.Error("a repository may not set hooks.tests_to_run, want the behaviour class")
	}

	on := filepath.Join(dir, "on.toml")
	writeTOML(t, on, "[hooks]\ntests_to_run = true\n")
	loaded, err = Load("", on)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.Config.Hooks.TestsToRun {
		t.Error("tests_to_run = true did not turn it on")
	}
	if info := loaded.Provenance["hooks.tests_to_run"]; info.Source != SourceProject || info.Path != on {
		t.Errorf("provenance = %+v, want the project layer", info)
	}

	bad := filepath.Join(dir, "bad.toml")
	writeTOML(t, bad, "[hooks]\ntests_to_run = \"on\"\n")
	if _, err := Load("", bad); err == nil || !strings.Contains(err.Error(), "tests_to_run") {
		t.Errorf("Load(tests_to_run = \"on\") = %v, want an error naming tests_to_run", err)
	}
}
