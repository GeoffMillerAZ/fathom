package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestResolveCallLog_PrefersDaemonConfiguredLog pins OC-2: the daemon's
// configured log wins over config.toml and the legacy relative default; an
// explicit flag or FATHOM_CALL_LOG still wins for its own invocation; and
// every answer names the rule that chose it.
func TestResolveCallLog_PrefersDaemonConfiguredLog(t *testing.T) {
	dir := t.TempDir()
	daemonLog := filepath.Join(dir, "daemon", "mcp-calls.jsonl")
	configLog := filepath.Join(dir, "config", "mcp-calls.jsonl")
	legacy := "workspace/logs/mcp-calls.jsonl"

	// The survey's defect: a verb from another directory read the legacy
	// path while the daemon wrote elsewhere. With a daemon record, the
	// daemon's log wins over both config and legacy.
	got := console.ResolveCallLog(console.CallLogInputs{
		Daemon: daemonLog, DaemonNote: "recorded by serve-http pid 7 on 127.0.0.1:7581",
		Config: configLog, Legacy: legacy,
	})
	if got.Path != daemonLog || got.Rule != console.RuleDaemon {
		t.Fatalf("daemon + config + legacy resolved to %+v, want the daemon's log", got)
	}
	if !strings.Contains(got.Note, "daemon") || !strings.Contains(got.Note, "pid 7") {
		t.Errorf("note must name the daemon record: %q", got.Note)
	}

	// A daemon started with logging off is still the daemon's answer: rule
	// none, never a config or legacy log that daemon does not write.
	if got := console.ResolveCallLog(console.CallLogInputs{DaemonOff: true, DaemonNote: "pid 9", Config: configLog, Legacy: legacy}); got.Rule != console.RuleNone || got.Path != "" || !strings.Contains(got.Note, "logging off") {
		t.Errorf("logging-off daemon resolved to %+v, want rule none", got)
	}
	if got := console.ResolveCallLog(console.CallLogInputs{Flag: "/x.jsonl", DaemonOff: true}); got.Rule != console.RuleFlag {
		t.Errorf("an explicit flag still wins over a logging-off daemon: %+v", got)
	}

	// Without a daemon record: config, then legacy.
	if got := console.ResolveCallLog(console.CallLogInputs{Config: configLog, Legacy: legacy}); got.Path != configLog || got.Rule != console.RuleConfig {
		t.Errorf("config + legacy = %+v, want config", got)
	}
	never := func(string) bool { return false }
	if got := console.ResolveCallLog(console.CallLogInputs{Legacy: legacy, Exists: never}); got.Path != legacy || got.Rule != console.RuleLegacy || !strings.Contains(got.Note, "legacy") {
		t.Errorf("legacy only = %+v", got)
	}
	// A legacy path that does not exist gives way to a probed daemon
	// workspace log, still under the legacy rule and naming the probe.
	probe := filepath.Join(dir, "probe.jsonl")
	only := func(p string) bool { return p == probe }
	if got := console.ResolveCallLog(console.CallLogInputs{Legacy: legacy, LegacyProbes: []string{"", probe}, Exists: only}); got.Path != probe || got.Rule != console.RuleLegacy || !strings.Contains(got.Note, probe) {
		t.Errorf("legacy with probe = %+v", got)
	}

	// Explicit overrides win for their own invocation.
	if got := console.ResolveCallLog(console.CallLogInputs{Flag: "/x/flag.jsonl", Env: "/x/env.jsonl", Daemon: daemonLog}); got.Rule != console.RuleFlag || got.Path != "/x/flag.jsonl" {
		t.Errorf("flag = %+v", got)
	}
	if got := console.ResolveCallLog(console.CallLogInputs{Env: "/x/env.jsonl", Daemon: daemonLog}); got.Rule != console.RuleEnv {
		t.Errorf("env = %+v", got)
	}
	// J5's named-service rule sits below the daemon and above config.
	if got := console.ResolveCallLog(console.CallLogInputs{Daemon: daemonLog, Service: "/svc/log", Config: configLog}); got.Rule != console.RuleDaemon {
		t.Errorf("daemon must beat the service rule: %+v", got)
	}
	if got := console.ResolveCallLog(console.CallLogInputs{Service: "/svc/log", Config: configLog}); got.Rule != console.RuleService {
		t.Errorf("service must beat config: %+v", got)
	}
	if got := console.ResolveCallLog(console.CallLogInputs{}); got.Rule != console.RuleNone || got.Path != "" || got.Note == "" {
		t.Errorf("nothing configured = %+v, want rule none with a note", got)
	}

	// The daemon record round-trips through the path a verb derives from
	// its daemon URL — localhost and 127.0.0.1 name the same record.
	home := filepath.Join(dir, "home")
	written, err := console.DaemonRecordPath(home, "127.0.0.1:7581")
	if err != nil {
		t.Fatal(err)
	}
	rec := console.NewDaemonRecord("127.0.0.1:7581", 4242, daemonLog, "/db/fleet.sqlite", "abc123", time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC))
	if err := console.WriteDaemonRecord(written, rec); err != nil {
		t.Fatal(err)
	}
	addr, err := console.DaemonAddrFromURL("http://localhost:7581")
	if err != nil {
		t.Fatal(err)
	}
	read, err := console.DaemonRecordPath(home, addr)
	if err != nil || read != written {
		t.Fatalf("localhost URL resolved to %q (%v), want %q", read, err, written)
	}
	back, err := console.ReadDaemonRecord(read)
	if err != nil || back != rec {
		t.Fatalf("record round trip = %+v (%v), want %+v", back, err, rec)
	}
	if got := console.ResolveCallLog(console.CallLogInputs{Daemon: back.CallLog, DaemonNote: back.Note(), Legacy: legacy}); got.Path != daemonLog || !strings.Contains(got.Note, "4242") {
		t.Errorf("record-driven resolution = %+v", got)
	}
	if _, err := console.ReadDaemonRecord(filepath.Join(home, "missing.json")); !os.IsNotExist(err) {
		t.Errorf("a missing record must be an IsNotExist error, got %v", err)
	}
	if _, err := console.DaemonRecordPath(home, "no-port"); err == nil {
		t.Error("an address without a port must be refused")
	}
}
