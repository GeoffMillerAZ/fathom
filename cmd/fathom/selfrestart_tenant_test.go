package main

// selfrestart_tenant_test.go — the responsible-tenant self-restart rows
// (docs/design/responsible-tenant.md §8.2): RT-11 (adoption exits 75, a
// code launchd relaunches on under either plist schema) and RT-12 (stat-gated
// hashing, a cross-process cooldown in the lifecycle ledger, managed only).

import (
	"errors"
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
	if adoptionExitCode() != 75 {
		t.Fatalf("adoptionExitCode() = %d, want 75 (EX_TEMPFAIL)", adoptionExitCode())
	}
	// Schema 2 is the plist fathom actually writes (RT-13's plistBody).
	v2 := plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/s.sqlite", ":7581", "/logs", "/c.jsonl", "/usr/bin", nil)
	const v2KeepAlive = "<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>"
	if !strings.Contains(v2, v2KeepAlive) {
		t.Fatalf("rendered plist no longer carries %q; update this test to the schema it renders:\n%s", v2KeepAlive, v2)
	}
	// Schema 1 is what installed daemons still run until reconciled
	// (KeepAlive true): the render with the unconditional KeepAlive back.
	v1 := strings.Replace(v2, v2KeepAlive, "<key>KeepAlive</key><true/>", 1)

	for _, tc := range []struct {
		name  string
		plist string
		dict  bool
	}{{"schema 1", v1, false}, {"schema 2", v2, true}} {
		dict, relaunch, err := keepAliveRelaunches(tc.plist, adoptionExitCode())
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if dict != tc.dict {
			t.Errorf("%s: KeepAlive dict = %v, want %v", tc.name, dict, tc.dict)
		}
		if !relaunch {
			t.Errorf("%s: launchd would not relaunch after adoption exit %d — a deploy would leave the daemon down", tc.name, adoptionExitCode())
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
		waitHashes(t, &hashes, 1)
		time.Sleep(80 * time.Millisecond) // ~40 polls of an unchanged file
		if got := hashes.Load(); got != 1 {
			t.Fatalf("hashes of an unchanged binary over ~40 polls = %d, want 1 (the baseline)", got)
		}
		// One change, as `task deploy` makes it: written aside and renamed
		// into place. (os.WriteFile over the file is TWO observable changes
		// — the truncate, then the bytes — and a 2 ms poll under load sees
		// the empty file in between and rightly hashes it; see the
		// next subtest.)
		replaceBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbbbbbb")
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("never adopted; log:\n%s", out.String())
		}
		if got := exited.Load(); got != int32(adoptionExitCode()) {
			t.Errorf("exit = %d, want %d", got, adoptionExitCode())
		}
		if got := hashes.Load(); got != 2 {
			t.Errorf("hashes = %d, want 2 (baseline + the one change)", got)
		}
	})

	t.Run("each distinct stat is hashed once, a half-written file never adopted", func(t *testing.T) {
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
		waitHashes(t, &hashes, 1)
		// A copy caught mid-write: truncated in place, held there for many
		// polls. The empty file is a new stat, hashed once and not again.
		if err := os.Truncate(bin, 0); err != nil {
			t.Fatal(err)
		}
		waitHashes(t, &hashes, 2) // a loaded machine may take a while to poll
		time.Sleep(80 * time.Millisecond)
		if got := hashes.Load(); got != 2 {
			t.Fatalf("hashes after the truncate held for ~40 polls = %d, want 2 (baseline + the truncated stat, once)", got)
		}
		select {
		case <-done:
			t.Fatalf("adopted a half-written (empty) binary; log:\n%s", out.String())
		default:
		}
		fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbbbbbb", 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("never adopted; log:\n%s", out.String())
		}
		if got := exited.Load(); got != int32(adoptionExitCode()) {
			t.Errorf("exit = %d, want %d", got, adoptionExitCode())
		}
		// Baseline, the empty file, the finished file — and, if a poll landed
		// between fakeBinary's own truncate and write, one more for that
		// stat. Never a hash per poll.
		if got := hashes.Load(); got < 3 || got > 4 {
			t.Errorf("hashes = %d, want 3 (or 4 if a poll saw fakeBinary mid-write)", got)
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
		installed := func() []string { return []string{"com.example.fathom"} }
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
		if got, _ := selfRestartManaged(false, "com.example.fathom", func() []string { return nil }); got {
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

// TestWatchOwnBinaryRetriesAFailedHash: a hash error on a changed binary is
// logged and retried on the next poll, never recorded as "seen" — otherwise
// the verified new binary is never adopted and nothing says why (property 4,
// I38).
func TestWatchOwnBinaryRetriesAFailedHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)
	var calls atomic.Int32
	hash := func(p string) (string, error) {
		if calls.Add(1) == 2 { // the first hash after the change
			return "", errors.New("read interrupted")
		}
		return hashFile(p)
	}
	var exited atomic.Int32
	exited.Store(-1)
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{
			poll: 2 * time.Millisecond, exit: func(c int) { exited.Store(int32(c)) }, hash: hash,
		})
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	fakeBinary(t, bin, "fathom v0.1.0 @ bbbbbbbbbbbbbbbb", 0)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("a transient hash failure stranded the deploy (hashes=%d); log:\n%s", calls.Load(), out.String())
	}
	if !strings.Contains(out.String(), "read interrupted") {
		t.Errorf("the hash failure must be logged:\n%s", out.String())
	}
}

// TestWatchOwnBinaryReverifiesAfterATransientRefusal: a good binary whose
// first `version` run fails (a timeout under memory pressure, a slow first
// exec while syspolicyd scans it) is re-verified after the refusal backoff,
// not stranded until the next deploy.
func TestWatchOwnBinaryReverifiesAfterATransientRefusal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)
	var exited atomic.Int32
	exited.Store(-1)
	var out syncBuffer
	done := make(chan struct{})
	go func() {
		watchOwnBinary(bin, dlogOver(&out), selfRestartConfig{
			poll: 2 * time.Millisecond, cooldown: 20 * time.Millisecond,
			exit: func(c int) { exited.Store(int32(c)) },
		})
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	marker := filepath.Join(dir, "ran-once")
	body := "#!/bin/sh\nif [ ! -f '" + marker + "' ]; then touch '" + marker + "'; exit 1; fi\necho 'fathom v0.1.0 @ bbbbbbbbbbbb'\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("a good binary refused once was never re-verified; log:\n%s", out.String())
	}
	if got := exited.Load(); got != int32(adoptionExitCode()) {
		t.Errorf("exit = %d, want %d", got, adoptionExitCode())
	}
	if !strings.Contains(out.String(), "REFUSED") {
		t.Errorf("the first refusal must still be logged:\n%s", out.String())
	}
}

// TestWatchOwnBinaryUnreadableLedgerFallsBackToUptime: when the lifecycle
// ledger cannot be read, the cross-process cooldown is unknown, and the
// uncertainty resolves toward waiting — the watcher adopts only once this
// process has itself run for a full cooldown, which bounds adoptions to one
// per cooldown across processes without the ledger.
func TestWatchOwnBinaryUnreadableLedgerFallsBackToUptime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixtures are POSIX")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fathom")
	ledger := filepath.Join(dir, "ledger-is-a-dir")
	if err := os.MkdirAll(ledger, 0o700); err != nil {
		t.Fatal(err)
	}
	fakeBinary(t, bin, "fathom v0.1.0 @ aaaaaaaaaaaa", 0)
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	var clock atomic.Int64
	clock.Store(start.UnixNano())
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
	clock.Store(start.Add(time.Minute).UnixNano())
	select {
	case <-done:
		t.Fatalf("adopted with an unreadable ledger 1 min into this process; the cooldown is unknown:\n%s", out.String())
	case <-time.After(time.Second):
	}
	if !strings.Contains(out.String(), "cannot read the lifecycle ledger") {
		t.Errorf("the unreadable ledger must be named:\n%s", out.String())
	}
	clock.Store(start.Add(5 * time.Minute).UnixNano())
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("never adopted once this process had run a full cooldown; log:\n%s", out.String())
	}
}

// TestLastAdoptionSkipsOverlongLines: a ledger line longer than the scanner
// buffer is skipped like any other unparseable row, never an error that
// discards the rows after it.
func TestLastAdoptionSkipsOverlongLines(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "lifecycle.jsonl")
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	long := `{"row":"note","x":"` + strings.Repeat("y", 100*1024) + `"}` + "\n"
	if err := os.WriteFile(ledger, []byte(long), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendAdoptRow(ledger, at, "a", "b"); err != nil {
		t.Fatal(err)
	}
	last, ok, err := lastAdoption(ledger)
	if err != nil || !ok || !last.Equal(at) {
		t.Fatalf("lastAdoption = %v, %v, %v; want %v, true, nil", last, ok, err, at)
	}
}

// TestStartSelfRestartOnlyWhenManaged pins the serve-http wiring of RT-12:
// an unmanaged serve-http never starts the watcher (so a scratch daemon
// never hashes its binary or writes the operator's ledger) and says so; a
// managed one starts it with the production cooldown and the ledger under
// its own label — matched against every installed fathom agent, not only
// the alphabetically first.
func TestStartSelfRestartOnlyWhenManaged(t *testing.T) {
	installed := func() []string { return []string{"com.example.a-fathom", "com.example.fathom"} }
	exe := func() (string, error) { return "/opt/fathom", nil }
	for _, tc := range []struct {
		name     string
		flag     bool
		xpc      string
		disabled bool
		want     bool
		wantLog  string
		label    string
	}{
		{"shell-started", false, "application.com.mitchellh.ghostty.1", false, false, "unmanaged daemon", ""},
		{"no launchd", false, "", false, false, "unmanaged daemon", ""},
		{"second installed agent", false, "com.example.fathom", false, true, "", "com.example.fathom"},
		{"--managed", true, "", false, true, "", "com.example.a-fathom"},
		{"managed but pinned", true, "", true, false, "FATHOM_SELF_RESTART=off", ""},
	} {
		var out syncBuffer
		var got []string
		started := startSelfRestart(dlogOver(&out), selfRestartStart{
			managedFlag: tc.flag, xpcServiceName: tc.xpc, installedLabels: installed,
			disabled: tc.disabled, executable: exe,
			watch: func(path string, cfg selfRestartConfig) {
				got = append(got, path, cfg.ledger)
				if cfg.cooldown != selfRestartCooldown || cfg.poll != selfRestartPoll {
					t.Errorf("%s: watcher config %+v, want the production poll and cooldown", tc.name, cfg)
				}
			},
		})
		if started != tc.want || (len(got) > 0) != tc.want {
			t.Errorf("%s: started = %v (watch calls %v), want %v", tc.name, started, got, tc.want)
		}
		if tc.wantLog != "" && !strings.Contains(out.String(), tc.wantLog) {
			t.Errorf("%s: log %q lacks %q", tc.name, out.String(), tc.wantLog)
		}
		if tc.want && len(got) == 2 && !strings.Contains(got[1], filepath.Join("daemon", tc.label, "lifecycle.jsonl")) {
			t.Errorf("%s: ledger = %q, want one under label %s", tc.name, got[1], tc.label)
		}
	}
}

// replaceBinary installs a new fake binary the way `task deploy` does:
// written to a sibling path and renamed over the old one, so the watcher can
// only ever observe the old file or the whole new one.
func replaceBinary(t *testing.T, path, prints string) {
	t.Helper()
	tmp := path + ".new"
	fakeBinary(t, tmp, prints, 0)
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// waitHashes waits (up to 10 s) until the watcher has hashed at least n
// times, so an assertion about what happens AFTER a hash never races the
// watcher goroutine's first polls on a loaded machine.
func waitHashes(t *testing.T, hashes *atomic.Int32, n int32) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for hashes.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("watcher hashed %d times in 10s, want at least %d", hashes.Load(), n)
		}
		time.Sleep(time.Millisecond)
	}
}
