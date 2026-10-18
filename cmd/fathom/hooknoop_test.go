package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// settingsHookCommands returns every hook command in a settings.json body.
func settingsHookCommands(t *testing.T, data []byte) []string {
	t.Helper()
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				out = append(out, h.Command)
			}
		}
	}
	return out
}

// runWithoutFathom runs command under sh with a PATH that has no fathom.
func runWithoutFathom(t *testing.T, command string) {
	t.Helper()
	empty := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", command)
	// The parent environment minus PATH, so FATHOM_HTTP/FATHOM_TEST still
	// reach the child; only PATH loses fathom.
	env := []string{"PATH=" + empty + ":/usr/bin:/bin"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil || so.Len() != 0 || se.Len() != 0 {
		t.Errorf("%q without fathom on PATH: err=%v stdout=%q stderr=%q, want exit 0 and no output", command, err, so.String(), se.String())
	}
}

// TestHookCommandsAreSilentNoOpWithoutFathom: a visitor who opens the repo
// without fathom installed sees no failing hooks, from the repo's own
// settings.json and from every command init generates.
func TestHookCommandsAreSilentNoOpWithoutFathom(t *testing.T) {
	if _, err := exec.LookPath("fathom"); err == nil {
		t.Log("fathom is on this PATH; the test strips it from the child PATH")
	}
	for _, hs := range fathomHookSpecs {
		runWithoutFathom(t, hookCommand("fathom", hs.sub))
	}
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range settingsHookCommands(t, data) {
		if strings.Contains(c, "cloud-session.sh") {
			continue // its own script exits 0 outside a cloud session
		}
		runWithoutFathom(t, c)
	}
}

// TestRepoSettingsHooksMatchInitTemplate pins the repo's own
// .claude/settings.json to the form init writes: every fathom hook command
// there is the guarded template, and every hookSpec has one.
func TestRepoSettingsHooksMatchInitTemplate(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	var wrapped []string // the repo's SessionStart row is wrapped by its cloud script, which no-ops locally
	for _, c := range settingsHookCommands(t, data) {
		if strings.Contains(c, "cloud-session.sh") {
			wrapped = append(wrapped, c)
			continue
		}
		if !strings.HasPrefix(c, "command -v fathom >/dev/null 2>&1 || exit 0; fathom ") {
			t.Errorf("settings.json hook %q is not the guarded form", c)
		}
		have[c] = true
	}
	for _, hs := range fathomHookSpecsAt("") {
		want := hookCommand("fathom", hs.sub)
		for _, w := range wrapped {
			if strings.HasSuffix(w, " fathom "+hs.sub) {
				have[want] = true
			}
		}
		if !have[want] {
			t.Errorf("settings.json lacks init's template command %q", want)
		}
	}
}

// TestInitThenDoctorShowsNoDrift: a fresh init (no --bin) writes the
// canonical bare-`fathom` form, so doctor finds no stale-path or drift.
func TestInitThenDoctorShowsNoDrift(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	srv := fakeInitDaemon(t, strings.ToLower(filepath.Base(dir)), 10)
	defer srv.Close()
	var out, errBuf bytes.Buffer
	if code := cmdInit([]string{"--path", dir, "--url", srv.URL}, &out, &errBuf); code != 0 {
		t.Fatalf("init exit %d: %s %s", code, out.String(), errBuf.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range settingsHookCommands(t, data) {
		if staleFathomPath.MatchString(c) {
			t.Errorf("init wrote an absolute binary path: %q", c)
		}
	}
	if d := analyzeWiring(dir); len(d.StaleCmds) != 0 || len(d.Missing) != 0 {
		t.Errorf("drift after fresh init: stale=%v missing=%v", d.StaleCmds, d.Missing)
	}
	if c := checkWiringDrift(dir, "claude"); c.Status != doctorPass {
		t.Errorf("wiring_drift = %v %s", c.Status, c.Reason)
	}
}
