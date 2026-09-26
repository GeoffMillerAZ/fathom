package main

// selfrestart_tenant_test.go — the responsible-tenant self-restart rows
// (docs/design/responsible-tenant.md §8.2): RT-11 (adoption exits 75, a
// code launchd relaunches on under either plist schema) and RT-12 (stat-gated
// hashing, a cross-process cooldown in the lifecycle ledger, managed only).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestSelfRestartExitCodeMatchesPlistSchema pins RT-11: the adoption exit
// code is non-zero (75), and launchd relaunches on it under the rendered
// schema-1 plist (KeepAlive true) AND under the schema-2 KeepAlive dict
// {SuccessfulExit: false}, where exit 0 — the old adoption code — would
// leave the daemon down after every deploy.
func TestSelfRestartExitCodeMatchesPlistSchema(t *testing.T) {
	if adoptionExitCode != 75 {
		t.Fatalf("adoptionExitCode = %d, want 75 (EX_TEMPFAIL)", adoptionExitCode)
	}
	v1 := plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/s.sqlite", ":7575", "/logs", "/c.jsonl", "/usr/bin", nil)
	const v1KeepAlive = "<key>KeepAlive</key><true/>"
	if !strings.Contains(v1, v1KeepAlive) {
		t.Fatalf("rendered plist no longer carries %q; update this test to the schema it renders:\n%s", v1KeepAlive, v1)
	}
	v2 := strings.Replace(v1, v1KeepAlive, "<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>", 1)

	for _, tc := range []struct {
		name  string
		plist string
		dict  bool
	}{{"schema 1", v1, false}, {"schema 2", v2, true}} {
		dict, relaunch, err := keepAliveRelaunches(tc.plist, adoptionExitCode)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if dict != tc.dict {
			t.Errorf("%s: KeepAlive dict = %v, want %v", tc.name, dict, tc.dict)
		}
		if !relaunch {
			t.Errorf("%s: launchd would not relaunch after adoption exit %d — a deploy would leave the daemon down", tc.name, adoptionExitCode)
		}
		// Exit 0 relaunches under KeepAlive true and stays down under the
		// dict: the adoption code must be non-zero exactly because of the dict.
		_, zeroRelaunch, _ := keepAliveRelaunches(tc.plist, 0)
		if zeroRelaunch == tc.dict {
			t.Errorf("%s: exit 0 relaunch = %v, want %v", tc.name, zeroRelaunch, !tc.dict)
		}
	}
}

// countingHash wraps hashFile and counts how often the watcher hashes.
func countingHash(n *atomic.Int32) func(string) (string, error) {
	return func(p string) (string, error) {
		n.Add(1)
		return hashFile(p)
	}
}

// TestWatchOwnBinaryHashesOnlyOnStatChange pins RT-12: an unchanged binary
// is never re-hashed (the incident binary hashed its 89 MB self every 20 s,
// ~4,320 times a day); a changed one is hashed once and adopted; a recent
// adoption recorded in the lifecycle ledger by ANOTHER process defers the
// adoption until the cooldown passes and the adoption is appended to the
// ledger; and the watcher runs only for a managed daemon.
func TestWatchOwnBinaryHashesOnlyOnStatChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}

	t.Run("unchanged binary is never re-hashed", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "fathom")
		fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)
		var hashes atomic.Int32
		var exited atomic.Int32
		exited.Store(-1)
		var out syncBuffer
		done := make(chan struct{})
		go func() {
			watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{
				poll: 2 * time.Millisecond, exit: func(c int) { exited.Store(int32(c)) },
				hash: countingHash(&hashes),
			})
			close(done)
		}()
		time.Sleep(80 * time.Millisecond) // ~40 polls of an unchanged file
		if got := hashes.Load(); got != 1 {
			t.Fatalf("hashes of an unchanged binary over ~40 polls = %d, want 1 (the baseline)", got)
		}
		fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbbbbbb", 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("never adopted; log:\n%s", out.String())
		}
		if got := exited.Load(); got != adoptionExitCode {
			t.Errorf("exit = %d, want %d", got, adoptionExitCode)
		}
		if got := hashes.Load(); got != 2 {
			t.Errorf("hashes = %d, want 2 (baseline + the one change)", got)
		}
	})

	t.Run("cross-process cooldown from the ledger", func(t *testing.T) {
		dir := t.TempDir()
		bin := filepath.Join(dir, "fathom")
		ledger := filepath.Join(dir, "daemon", "dev.fathom.serve-http", "lifecycle.jsonl")
		fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)
		now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
		var clock atomic.Int64
		clock.Store(now.UnixNano())
		// A predecessor process adopted one minute ago.
		if err := appendAdoptRow(ledger, now.Add(-time.Minute), "old", "prev"); err != nil {
			t.Fatal(err)
		}
		var exited atomic.Int32
		exited.Store(-1)
		var out syncBuffer
		done := make(chan struct{})
		go func() {
			watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{
				poll: 2 * time.Millisecond, cooldown: 5 * time.Minute, ledger: ledger,
				exit: func(c int) { exited.Store(int32(c)) },
				now:  func() time.Time { return time.Unix(0, clock.Load()) },
			})
			close(done)
		}()
		time.Sleep(20 * time.Millisecond)
		fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbbbbbb", 0)
		time.Sleep(80 * time.Millisecond)
		if exited.Load() != -1 {
			t.Fatalf("adopted 1 min after another process's adoption; cooldown is 5 min:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "cooldown") {
			t.Errorf("a deferred adoption must say why:\n%s", out.String())
		}
		clock.Store(now.Add(5 * time.Minute).UnixNano())
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("never adopted after the cooldown; log:\n%s", out.String())
		}
		last, ok, err := lastAdoption(ledger)
		if err != nil || !ok {
			t.Fatalf("lastAdoption = %v, %v, %v", last, ok, err)
		}
		if !last.Equal(now.Add(5 * time.Minute)) {
			t.Errorf("ledger's last adoption = %v, want %v (this adoption appended)", last, now.Add(5*time.Minute))
		}
	})

	t.Run("managed only", func(t *testing.T) {
		installed := func() string { return "com.example.fathom" }
		for _, tc := range []struct {
			flag bool
			xpc  string
			want bool
		}{
			{true, "", true},
			{false, "com.example.fathom", true},
			{false, "application.com.mitchellh.ghostty.1234", false},
			{false, "0", false},
			{false, "", false},
		} {
			got, _ := selfRestartManaged(tc.flag, tc.xpc, installed)
			if got != tc.want {
				t.Errorf("selfRestartManaged(%v, %q) = %v, want %v", tc.flag, tc.xpc, got, tc.want)
			}
		}
		if got, _ := selfRestartManaged(false, "com.example.fathom", func() string { return "" }); got {
			t.Error("managed with no installed fathom agent; an unrelated launchd job is not fathom's")
		}
	})
}

// TestBinaryStatChanged pins the stat gate's comparison itself.
func TestBinaryStatChanged(t *testing.T) {
	base := binaryStat{size: 10, mtimeNs: 100, ino: 7}
	if binaryStatChanged(base, base) {
		t.Error("identical stats reported changed")
	}
	for _, c := range []binaryStat{
		{size: 11, mtimeNs: 100, ino: 7},
		{size: 10, mtimeNs: 101, ino: 7},
		{size: 10, mtimeNs: 100, ino: 8},
	} {
		if !binaryStatChanged(base, c) {
			t.Errorf("%+v vs %+v reported unchanged", base, c)
		}
	}
	if _, err := statBinary(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("statBinary of a missing file returned no error")
	}
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := statBinary(f); err != nil || st.size != 1 || st.ino == 0 {
		t.Errorf("statBinary = %+v, %v", st, err)
	}
}
