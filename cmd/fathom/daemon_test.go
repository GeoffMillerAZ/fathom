package main

import (
	"bytes"
	"encoding/xml"
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
	if err := os.WriteFile(plist, []byte(plistBody("com.example.mine", "/opt/fathom", "/tmp/s.sqlite", ":7581", "/tmp", "/tmp/c.jsonl", "/usr/bin", nil)), 0o644); err != nil {
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
		[]byte(plistBody("com.existing.fathom-http", "/opt/fathom", "/tmp/s.sqlite", ":7581", "/tmp", "/tmp/c.jsonl", "/usr/bin", nil)), 0o644); err != nil {
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
	body := plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/home/u/.fathom/db/fathom.sqlite", ":7581", "/home/u/.fathom/logs", "/home/u/.fathom/logs/mcp-calls.jsonl", "/usr/local/go/bin:/usr/bin", nil)
	for _, want := range []string{
		"<string>dev.fathom.serve-http</string>",
		"<string>/usr/local/bin/fathomd</string>", // the launcher beside the binary (launcher.go)
		"<string>serve-http</string>",
		"<string>/home/u/.fathom/db/fathom.sqlite</string>",
		"<string>:7581</string>",
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

// BP-37 (docs/plans/bench-pyramid.md): the plist must carry the Go
// toolchain's environment, not only PATH.
//
// On 2026-09-12 the installed daemon had PATH and nothing else. launchd
// does not add a login shell's environment, so GOPATH reached the daemon
// UNSET, go resolved its default to the literal "$HOME/go" and refused
// every command, and every daemon-side ingest of a Go service fell back to
// syntax-only edges while reporting a healthy index. These keys are the fix
// at the source; internal/fusion.TypedEdgeEnv is the belt for daemons an
// older binary installed.
func TestPlistCarriesGoToolchainEnv(t *testing.T) {
	body := plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/tmp/s.sqlite", ":7581", "/tmp", "/tmp/c.jsonl", "/usr/local/go/bin:/usr/bin",
		[]string{"GOPATH=/home/u/go", "GOCACHE=/home/u/Library/Caches/go-build", "GOMODCACHE=/home/u/go/pkg/mod"})
	for _, want := range []string{
		"<key>GOPATH</key><string>/home/u/go</string>",
		"<key>GOCACHE</key><string>/home/u/Library/Caches/go-build</string>",
		"<key>GOMODCACHE</key><string>/home/u/go/pkg/mod</string>",
		"<key>PATH</key><string>/usr/local/go/bin:/usr/bin</string>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("plist missing %q:\n%s", want, body)
		}
	}
	// The plist must stay valid XML with the extra keys in it.
	var doc any
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("plist is not well-formed XML with the Go env keys: %v", err)
	}
	// An unresolvable variable is omitted, never written empty — an empty
	// GOPATH is exactly the state this row prevents.
	partial := plistBody("l", "/b", "/s", ":1", "/l", "/c", "/usr/bin", []string{"GOPATH=/home/u/go", "GOCACHE="})
	if strings.Contains(partial, "<key>GOCACHE</key>") {
		t.Fatalf("an empty value was written into the plist environment:\n%s", partial)
	}
}

// daemonGoEnv resolves what the installer writes: absolute values only, in
// the pinned key order.
func TestDaemonGoEnvIsAbsolute(t *testing.T) {
	t.Setenv("GOPATH", "relative/path") // the daemon's exact broken state
	got := daemonGoEnv()
	if len(got) == 0 {
		t.Fatal("daemonGoEnv resolved nothing — the installer would write a PATH-only plist, which is the defect")
	}
	seen := map[string]bool{}
	for _, kv := range got {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(v, "/") || strings.Contains(v, "$") {
			t.Fatalf("daemonGoEnv produced a non-absolute or unexpanded entry: %q", kv)
		}
		seen[k] = true
	}
	if !seen["GOPATH"] {
		t.Fatalf("daemonGoEnv did not resolve GOPATH from a relative one: %v", got)
	}
}
