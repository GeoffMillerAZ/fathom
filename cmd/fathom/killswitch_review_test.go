package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// installAgent writes a fathom serve-http plist with label into home's
// LaunchAgents (the file name and the Label key agree, as launchd needs).
func installAgent(t *testing.T, home, label string) {
	t.Helper()
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(v1PlistFixture, "com.example.mine", label)
	if err := os.WriteFile(filepath.Join(agents, label+".plist"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A deliberate operator stop is not a fault: with the kill switch set,
// every "how do I get the daemon back" line (status, doctor, the session
// primer) names `fathom daemon start` and why it is down — never a
// launchctl kickstart, which only produces another exit 0 and wedges when
// run from an agent shell.
func TestRestartHintNamesDaemonStartWhenKillSwitchSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installAgent(t, home, "com.example.mine")
	// No switch: the platform's own way back stands — the installed agent's
	// kickstart under launchd; off macOS (CS-8: the Linux cloud VMs) no
	// launchd agent is ever read, and the hint is the foreground start line.
	wantNoSwitch := "launchctl kickstart"
	if runtime.GOOS != "darwin" {
		wantNoSwitch = "no launchd on this platform"
	}
	if hint := daemonRestartHint(""); !strings.Contains(hint, wantNoSwitch) || strings.HasPrefix(hint, "fathom daemon start") {
		t.Fatalf("no switch: the %q hint stands, got %q", wantNoSwitch, hint)
	}
	writeSwitch(t, home, tenancy.DisabledFile, tenancy.MarshalSwitch(tenancy.SwitchRecord{By: "fathom daemon stop", Reason: "laptop too hot", Since: stopT0}))
	hint := daemonRestartHint("")
	if !strings.HasPrefix(hint, "fathom daemon start") || strings.Contains(hint, "kickstart") || !strings.Contains(hint, "laptop too hot") {
		t.Fatalf("a set kill switch must be named with its reason and `fathom daemon start`: %q", hint)
	}
	if msg := buildSessionContext("http://127.0.0.1:1", t.TempDir()); !strings.Contains(msg, "fathom daemon start") || strings.Contains(msg, "kickstart") {
		t.Fatalf("the session primer must not hand an agent a kickstart for a deliberate stop: %q", msg)
	}

	ops := newFakeDaemonOps(stopT0)
	var out, errBuf bytes.Buffer
	runDaemonStatus([]string{"--url", "http://127.0.0.1:1"}, &out, &errBuf, ops)
	if !strings.Contains(out.String(), "remedy:        fathom daemon start") || strings.Contains(out.String(), "kickstart") {
		t.Fatalf("status must name `fathom daemon start` for a deliberate stop:\n%s", out.String())
	}

	// A timed stop still in force: the same, not a kickstart.
	writeSwitch(t, home, tenancy.DisabledFile, tenancy.MarshalSwitch(tenancy.SwitchRecord{Reason: "meeting", Until: time.Now().Add(time.Hour)}))
	if hint := daemonRestartHint(""); !strings.HasPrefix(hint, "fathom daemon start") {
		t.Fatalf("a timed stop in force: %q", hint)
	}
}

// Under a schema-1 plist (no --managed) launchd's XPC_SERVICE_NAME is the
// only thing that tells a relaunch it is managed. The label is the
// adopter's (`daemon install --label`, ADR-011): it need not contain
// "fathom". Managed means XPC_SERVICE_NAME equals the label of an
// installed fathom serve-http agent (design §10.1) — not a substring guess.
func TestManagedLabelIsTheInstalledAgentsLabel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd labels are darwin-only")
	}
	home := killSwitchHome(t)
	writeSwitch(t, home, tenancy.DisabledFile, nil)
	var errBuf bytes.Buffer
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "XPC_SERVICE_NAME" {
				return v
			}
			return ""
		}
	}

	if _, stop := checkKillSwitch(nil, env("com.acme.codeintel"), time.Now(), &errBuf); stop {
		t.Fatal("no agent installed under that label: not managed")
	}
	installAgent(t, home, "com.acme.codeintel")
	if code, stop := checkKillSwitch(nil, env("com.acme.codeintel"), time.Now(), &errBuf); !stop || code != 0 {
		t.Fatal("a launchd relaunch under the installed agent's label (no 'fathom' in it) must obey the switch")
	}
	if _, stop := checkKillSwitch(nil, env("org.example.fathom-other"), time.Now(), &errBuf); stop {
		t.Fatal("a label merely containing 'fathom', with no installed agent, is not the managed daemon")
	}

	// And stop's relaunch claim holds for that label: the v1 plist is
	// named, and a relaunch under it is managed (checked above).
	ops := newFakeDaemonOps(stopT0)
	lines := strings.Join(relaunchClause(ops, "", tenancy.SwitchRecord{}), "\n")
	if !strings.Contains(lines, "each start exits in < 50 ms") || !strings.Contains(lines, "com.acme.codeintel") {
		t.Fatalf("relaunch clause: %s", lines)
	}
}

// relaunchClause claims "each start exits in < 50 ms" only when a relaunch
// under the plist's Label will be recognised as managed. An agent named
// explicitly (`stop --label`) whose plist names no fathom binary is not
// recognised by the relaunched process, so the claim would be false.
func TestRelaunchClauseOnlyClaimsFastExitForARecognisedLabel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plists are darwin-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(strings.ReplaceAll(v1PlistFixture, "com.example.mine", "com.acme.codeintel"), "fathom", "codeintel")
	if err := os.WriteFile(filepath.Join(agents, "com.acme.codeintel.plist"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ops := newFakeDaemonOps(stopT0)
	lines := strings.Join(relaunchClause(ops, "com.acme.codeintel", tenancy.SwitchRecord{}), "\n")
	if strings.Contains(lines, "< 50 ms") || !strings.Contains(lines, "NOT see the kill switch") || !strings.Contains(lines, "launchctl bootout gui/501/com.acme.codeintel") {
		t.Fatalf("an unrecognised relaunch label must be named, not claimed fast: %s", lines)
	}
}

// I38: a switch file that exists but cannot be stat'ed or read is present
// with its reason unknown — never absent. Only "no such file" (or a
// non-directory in the path, where none can exist) is absent.
func TestKillSwitchUnreadableCountsAsPresent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	home := killSwitchHome(t)
	writeSwitch(t, home, tenancy.DisabledFile, tenancy.MarshalSwitch(tenancy.SwitchRecord{Reason: "hot"}))
	dir := filepath.Join(home, ".fathom", "daemon")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	var errBuf bytes.Buffer
	code, stop := checkKillSwitch([]string{"--managed"}, os.Getenv, time.Now(), &errBuf)
	if !stop || code != 0 {
		t.Fatalf("an unreadable switch dir must hold the switch (stop=%v code=%d)", stop, code)
	}
	if !strings.Contains(errBuf.String(), "unreadable") {
		t.Fatalf("the exit must say the switch was unreadable: %q", errBuf.String())
	}
	_ = os.Chmod(dir, 0o755)

	// ~/.fathom/daemon is a regular file: no switch can exist under it.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stop := checkKillSwitch([]string{"--managed"}, os.Getenv, time.Now(), &errBuf); stop {
		t.Fatal("ENOTDIR is absent, not a switch")
	}
}
