package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The defect this closes: every "daemon is down" message named ONE launchd
// label, `com.geoffmilleraz.fathom-http`, which exists on one machine. The
// SessionStart hook carried it into every session of every adopter, where
// `launchctl kickstart` fails with "Could not find service".
//
// A hint must name the label ACTUALLY installed, or tell the reader how to
// install one — never a third party's service name.
func TestDaemonRestartHintNeverNamesAnUninstalledService(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	hint := daemonRestartHint("")
	if strings.Contains(hint, "com.geoffmilleraz") {
		t.Fatalf("a machine with no fathom agent must not be told to kickstart one: %q", hint)
	}
	if runtime.GOOS == "darwin" {
		if !strings.Contains(hint, "fathom daemon install") {
			t.Fatalf("on darwin the remedy is to install the agent: %q", hint)
		}
	} else if !strings.Contains(hint, "serve-http") {
		t.Fatalf("off darwin the remedy is to run it under a supervisor: %q", hint)
	}

	if runtime.GOOS != "darwin" {
		t.Skip("launchd discovery is darwin-only")
	}

	// With an agent installed, the hint names THAT label.
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(agents, "com.example.mine.plist")
	if err := os.WriteFile(plist, []byte(plistBody("com.example.mine", "/opt/fathom", "/tmp/s.sqlite", ":7575", "/tmp", "/tmp/c.jsonl", "/usr/bin")), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := labelInstalled(); got != "com.example.mine" {
		t.Fatalf("labelInstalled = %q, want the installed agent", got)
	}
	if hint := daemonRestartHint(""); !strings.Contains(hint, "com.example.mine") {
		t.Fatalf("the hint must name the installed label: %q", hint)
	}
}

// A plist that is not fathom's must never be adopted as fathom's agent —
// kickstarting an unrelated service is worse than reporting none.
func TestLabelInstalledIgnoresForeignAgents(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd discovery is darwin-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := `<?xml version="1.0"?><plist version="1.0"><dict>
  <key>Label</key><string>com.other.thing</string>
  <key>ProgramArguments</key><array><string>/usr/bin/true</string></array>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(agents, "com.other.thing.plist"), []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := labelInstalled(); got != "" {
		t.Fatalf("a non-fathom agent must not be claimed as fathom's: %q", got)
	}
}

// install refuses to stack a second daemon on the same port behind the
// user's back, and says how to keep or replace the one that is there.
func TestDaemonInstallRefusesToStackASecondAgent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "com.existing.fathom-http.plist"),
		[]byte(plistBody("com.existing.fathom-http", "/opt/fathom", "/tmp/s.sqlite", ":7575", "/tmp", "/tmp/c.jsonl", "/usr/bin")), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errBuf bytes.Buffer
	rc := cmdDaemonInstall([]string{"--label", "dev.fathom.serve-http"}, &out, &errBuf)
	if rc == 0 {
		t.Fatal("installing a second agent on the same port must not silently succeed")
	}
	msg := errBuf.String()
	if !strings.Contains(msg, "com.existing.fathom-http") {
		t.Fatalf("the refusal must name the agent already there: %s", msg)
	}
	for _, want := range []string{"keep it", "replace it"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the refusal must offer both ways forward (%q): %s", want, msg)
		}
	}
}

// The plist launchd gets must carry the resolved paths, not placeholders —
// this was previously a doc snippet the adopter hand-edited.
func TestPlistCarriesResolvedPaths(t *testing.T) {
	body := plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/home/u/.fathom/db/fathom.sqlite", ":7575", "/home/u/.fathom/logs", "/home/u/.fathom/logs/mcp-calls.jsonl", "/usr/local/go/bin:/usr/bin")
	for _, want := range []string{
		"<string>dev.fathom.serve-http</string>",
		"<string>/usr/local/bin/fathom</string>",
		"<string>serve-http</string>",
		"<string>/home/u/.fathom/db/fathom.sqlite</string>",
		"<string>:7575</string>",
		"/home/u/.fathom/logs/fathom-http.log",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("plist missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "com.example") {
		t.Fatal("the plist must not carry the doc's placeholder label")
	}
}
