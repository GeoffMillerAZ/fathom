package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestConfig_HooksTestRunDefaultsOff pins TR-9 (docs/plans/test-run-hook.md,
// ADR-056 decision 6): `[hooks] test_run` is the opt-in switch for the
// test-run hook and is OFF until a layer sets it — absent everywhere reads
// false with no provenance; a project layer's true wins over a user
// layer's false; a non-bool is refused at load, naming the key.
func TestConfig_HooksTestRunDefaultsOff(t *testing.T) {
	t.Run("absent is off", func(t *testing.T) {
		dir := t.TempDir()
		loaded, err := Load(filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.Config.Hooks.TestRun {
			t.Error("Hooks.TestRun = true with no layer setting it, want false (default off until the bench)")
		}
		if _, ok := loaded.Provenance["hooks.test_run"]; ok {
			t.Error("hooks.test_run has provenance with no layer setting it")
		}
	})

	t.Run("an empty [hooks] table stays off", func(t *testing.T) {
		dir := t.TempDir()
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, "[hooks]\n")
		loaded, err := Load("", projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.Config.Hooks.TestRun {
			t.Error("an empty [hooks] table turned test_run on")
		}
	})

	t.Run("project on wins over user off", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, userPath, "[hooks]\ntest_run = false\n")
		writeTOML(t, projectPath, "[hooks]\ntest_run = true\n")
		loaded, err := Load(userPath, projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !loaded.Config.Hooks.TestRun {
			t.Error("Hooks.TestRun = false, want the project layer's true")
		}
		if info := loaded.Provenance["hooks.test_run"]; info.Source != SourceProject || info.Path != projectPath {
			t.Errorf("provenance = %+v, want the project layer", info)
		}
	})

	t.Run("a non-bool is refused", func(t *testing.T) {
		dir := t.TempDir()
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, "[hooks]\ntest_run = \"yes\"\n")
		_, err := Load("", projectPath)
		if err == nil || !strings.Contains(err.Error(), "test_run") {
			t.Fatalf("Load = %v, want an error naming test_run", err)
		}
	})

	t.Run("an unknown hooks key is refused", func(t *testing.T) {
		dir := t.TempDir()
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, "[hooks]\ntest_runs = true\n")
		if _, err := Load("", projectPath); err == nil {
			t.Fatal("Load accepted [hooks] test_runs, a key the schema does not declare")
		}
	})
}

// TestConfig_TestsCommands pins TR-1's config half: `[tests] commands` is a
// repository's own list of declared test commands, beside the ones read
// from its Taskfile, Makefile and package.json. The layer that sets it
// replaces the whole list (every slice field's discipline).
func TestConfig_TestsCommands(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.toml")
	projectPath := filepath.Join(dir, "project.toml")
	writeTOML(t, userPath, "[tests]\ncommands = [\"make check\"]\n")
	writeTOML(t, projectPath, "[tests]\ncommands = [\"go test\", \"bin/itest\"]\n")
	loaded, err := Load(userPath, projectPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"go test", "bin/itest"}; !reflect.DeepEqual(loaded.Config.Tests.Commands, want) {
		t.Errorf("Tests.Commands = %q, want %q", loaded.Config.Tests.Commands, want)
	}
	if info := loaded.Provenance["tests.commands"]; info.Source != SourceProject {
		t.Errorf("provenance = %+v, want the project layer", info)
	}

	bad := filepath.Join(dir, "bad.toml")
	writeTOML(t, bad, "[tests]\ncommands = \"go test\"\n")
	if _, err := Load("", bad); err == nil {
		t.Error("Load accepted a string where [tests] commands is a list")
	}
}

// TestConfig_HooksPinCheckDefaultsRecord pins PR-6 (docs/plans/served-not-kept.md,
// ADR-057 decision 4): `[hooks] pin_check` is "off", "record" or
// "feedback", and reads "record" with no provenance when no layer sets it
// (record mode until PR-M1 passes); a project layer wins over a user layer;
// any other value is refused at load, naming the key; an unknown [hooks] key
// is still refused.
func TestConfig_HooksPinCheckDefaultsRecord(t *testing.T) {
	t.Run("absent is record", func(t *testing.T) {
		dir := t.TempDir()
		loaded, err := Load(filepath.Join(dir, "user.toml"), filepath.Join(dir, "project.toml"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := loaded.Config.Hooks.PinCheck; got != PinCheckRecord {
			t.Errorf("Hooks.PinCheck = %q with no layer setting it, want %q", got, PinCheckRecord)
		}
		if _, ok := loaded.Provenance["hooks.pin_check"]; ok {
			t.Error("hooks.pin_check has provenance with no layer setting it")
		}
	})

	t.Run("a [hooks] table without the key stays record", func(t *testing.T) {
		dir := t.TempDir()
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, "[hooks]\ntest_run = true\n")
		loaded, err := Load("", projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := loaded.Config.Hooks.PinCheck; got != PinCheckRecord {
			t.Errorf("Hooks.PinCheck = %q, want %q", got, PinCheckRecord)
		}
	})

	t.Run("project wins over user", func(t *testing.T) {
		dir := t.TempDir()
		userPath := filepath.Join(dir, "user.toml")
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, userPath, "[hooks]\npin_check = \"off\"\n")
		writeTOML(t, projectPath, "[hooks]\npin_check = \"feedback\"\n")
		loaded, err := Load(userPath, projectPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := loaded.Config.Hooks.PinCheck; got != PinCheckFeedback {
			t.Errorf("Hooks.PinCheck = %q, want the project layer's %q", got, PinCheckFeedback)
		}
		if info := loaded.Provenance["hooks.pin_check"]; info.Source != SourceProject || info.Path != projectPath {
			t.Errorf("provenance = %+v, want the project layer", info)
		}
		userOnly, err := Load(userPath, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := userOnly.Config.Hooks.PinCheck; got != PinCheckOff {
			t.Errorf("Hooks.PinCheck = %q, want the user layer's %q", got, PinCheckOff)
		}
	})

	for _, bad := range []string{`"warn"`, `true`, `""`} {
		t.Run("refuses "+bad, func(t *testing.T) {
			dir := t.TempDir()
			projectPath := filepath.Join(dir, "project.toml")
			writeTOML(t, projectPath, "[hooks]\npin_check = "+bad+"\n")
			_, err := Load("", projectPath)
			if err == nil || !strings.Contains(err.Error(), "pin_check") {
				t.Fatalf("Load = %v, want an error naming pin_check", err)
			}
		})
	}

	t.Run("an unknown hooks key is still refused", func(t *testing.T) {
		dir := t.TempDir()
		projectPath := filepath.Join(dir, "project.toml")
		writeTOML(t, projectPath, "[hooks]\npin_checks = \"off\"\n")
		if _, err := Load("", projectPath); err == nil {
			t.Fatal("Load accepted [hooks] pin_checks, a key the schema does not declare")
		}
	})
}
