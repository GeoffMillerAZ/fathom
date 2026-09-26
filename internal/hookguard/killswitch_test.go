package hookguard

import (
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
	for _, u := range []string{"http://localhost:7575", "http://127.0.0.1:7575/", "127.0.0.1:7575"} {
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
	if d := BreakerAllows(g, "http://127.0.0.1:7575"); d.Verdict != Allow {
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
