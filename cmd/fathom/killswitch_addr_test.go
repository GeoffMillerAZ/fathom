package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// TestSwitchWritersRecordAddrHooksMatch pins the cross-lane contract of
// RT-167 and RT-10 (design §2.3, §9): every writer of disabled.json and
// paused.json records the managed daemon's listen address as `addr`, the
// field the hooks' kill-switch stat matches. A switch for a managed daemon
// on a non-default address therefore silences hooks addressing THAT
// daemon, and never a scratch daemon on the default port.
func TestSwitchWritersRecordAddrHooksMatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".fathom", "daemon")
	ops := newFakeDaemonOps(stopT0)
	ops.env["CLAUDECODE"] = "1"
	g := &hookguard.Guard{HealthDir: filepath.Join(home, "health"), DaemonDir: dir}
	const managed = "http://127.0.0.1:7999"
	const other = "http://127.0.0.1:7575"

	var out, errBuf bytes.Buffer
	if rc := runDaemonStop([]string{"--url", managed}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("stop: rc=%d %s %s", rc, out.String(), errBuf.String())
	}
	if d := hookguard.BreakerAllows(g, managed); d.Verdict != hookguard.Disabled {
		t.Fatalf("a hook addressing the stopped daemon %s: %v, want disabled", managed, d.Verdict)
	}
	if d := hookguard.BreakerAllows(g, other); d.Verdict != hookguard.Allow {
		t.Fatalf("a hook addressing another daemon %s: %v, want allow — stop must record addr", other, d.Verdict)
	}
	if rc := runDaemonStart(nil, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("start: rc=%d %s", rc, errBuf.String())
	}

	if rc := runDaemonPause(true, []string{"--url", managed}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("pause: rc=%d %s", rc, errBuf.String())
	}
	if d := hookguard.BreakerAllows(g, managed); d.Verdict != hookguard.Paused {
		t.Fatalf("a hook addressing the paused daemon %s: %v, want paused", managed, d.Verdict)
	}
	if d := hookguard.BreakerAllows(g, other); d.Verdict != hookguard.Allow {
		t.Fatalf("a hook addressing another daemon %s under pause: %v, want allow — pause must record addr", other, d.Verdict)
	}
}
