package hookguard

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKillSwitchGovernsOnlyManagedURL(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := newTestGuard(t, clk)
	if err := os.MkdirAll(g.DaemonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(g.DaemonDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// An empty disabled.json disables, reason unknown (§9).
	write("disabled.json", "")
	// The managed address is named symbolically: a test never carries the
	// operator's live daemon address as a literal (tenancy lint test-live-url).
	mu, err := url.Parse(DefaultManagedURL)
	if err != nil {
		t.Fatal(err)
	}
	port := mu.Port()
	for _, u := range []string{"http://localhost:" + port, "http://127.0.0.1:" + port + "/", "127.0.0.1:" + port} {
		if d := BreakerAllows(g, u); d.Verdict != Disabled || d.Reason != "reason unknown" {
			t.Fatalf("%s under empty disabled.json: %+v", u, d)
		}
	}
	// A scratch daemon on another port is never silenced by it.
	if d := BreakerAllows(g, "http://127.0.0.1:7840"); d.Verdict != Allow {
		t.Fatalf("unmanaged URL: %v, want allow", d.Verdict)
	}
	// The switch may name the managed address.
	write("disabled.json", `{"reason":"breaker","addr":"127.0.0.1:7600"}`)
	if d := BreakerAllows(g, "http://localhost:7600"); d.Verdict != Disabled || d.Reason != "breaker" {
		t.Fatalf("named addr: %+v", d)
	}
	if d := BreakerAllows(g, DefaultManagedURL); d.Verdict != Allow {
		t.Fatalf("default port when the switch names another addr: %v", d.Verdict)
	}
	// A timed stop that has lapsed no longer silences.
	write("disabled.json", `{"reason":"for 2h","until":"2026-09-26T11:00:00Z"}`)
	if d := BreakerAllows(g, DefaultManagedURL); d.Verdict != Allow {
		t.Fatalf("lapsed --for: %v, want allow", d.Verdict)
	}
	if err := os.Remove(filepath.Join(g.DaemonDir, "disabled.json")); err != nil {
		t.Fatal(err)
	}
	write("paused.json", `{"reason":"operator"}`)
	if d := BreakerAllows(g, DefaultManagedURL); d.Verdict != Paused {
		t.Fatalf("paused.json: %v, want paused", d.Verdict)
	}
}

// A switch the hook cannot stat for any reason but "no such file" still
// holds, reason unknown — the daemon's reader (cmd/fathom readSwitchFile,
// tenancy.SwitchFile.ReadErr) treats it the same way (I38).
func TestKillSwitchUnstatableCountsAsPresent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	clk := &fakeClock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := newTestGuard(t, clk)
	if err := os.MkdirAll(g.DaemonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.DaemonDir, "disabled.json"), []byte(`{"reason":"operator"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(g.DaemonDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(g.DaemonDir, 0o755) })
	if d := BreakerAllows(g, DefaultManagedURL); d.Verdict != Disabled {
		t.Fatalf("an unstatable disabled.json: %v, want disabled (reason unknown)", d.Verdict)
	}
	// No daemon dir at all is simply no switch.
	g2 := newTestGuard(t, clk)
	if d := BreakerAllows(g2, DefaultManagedURL); d.Verdict == Disabled || d.Verdict == Paused {
		t.Fatalf("absent switch files: %v, want no kill-switch verdict", d.Verdict)
	}
}
