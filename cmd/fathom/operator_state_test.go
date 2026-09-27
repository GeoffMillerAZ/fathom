package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// writeTestPlist writes a launchd agent for a fathom serve-http under
// home/Library/LaunchAgents, as `daemon install` renders it.
func writeTestPlist(t *testing.T, home, label, store, addr, callLog string) {
	t.Helper()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := plistBody(label, "/opt/fathom/bin/fathom", store, addr, filepath.Join(home, "logs"), callLog, "/usr/bin", nil)
	if err := os.WriteFile(filepath.Join(dir, label+".plist"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestOperatorStateFromPlistAndPasswdHome (RT-22 review, I52): the
// refusal knows the managed daemon's store and call log from the installed
// plist as well as its record, and from the account's own home when HOME
// is swapped (stranger-check, scenariobench) — the throwaway HOME has no
// record, so before, a scratch daemon there wrote the operator's call log
// unrefused. Under FATHOM_TEST=1 the account's home is never read.
func TestOperatorStateFromPlistAndPasswdHome(t *testing.T) {
	realHome := t.TempDir()
	scratchHome := t.TempDir()
	opStore := filepath.Join(realHome, "db", "fleet co.sqlite")
	opLog := filepath.Join(realHome, "logs", "mcp-calls.jsonl")
	writeTestPlist(t, realHome, "dev.fathom.serve-http", opStore, "127.0.0.1:7600", opLog)

	args := installedDaemonArgs(realHome)
	if got, _ := cmdArgValue(args, "store"); got != opStore {
		t.Fatalf("plist --store = %q, want %q (args %v)", got, opStore, args)
	}

	if got := operatorHomes(scratchHome, realHome, ""); !reflect.DeepEqual(got, []string{scratchHome, realHome}) {
		t.Fatalf("operatorHomes = %v", got)
	}
	if got := operatorHomes(scratchHome, realHome, "1"); !reflect.DeepEqual(got, []string{scratchHome}) {
		t.Fatalf("operatorHomes under FATHOM_TEST=1 = %v, want the isolated HOME only", got)
	}
	if got := operatorHomes(realHome, realHome, ""); !reflect.DeepEqual(got, []string{realHome}) {
		t.Fatalf("operatorHomes with HOME = the account's home = %v", got)
	}

	homes := operatorHomes(scratchHome, realHome, "")
	warns, err := checkOperatorState(opStore, "", homes, managedAddr)
	if !errors.Is(err, errOperatorState) || len(warns) != 0 {
		t.Fatalf("the plist's store under the account's home not refused from a swapped HOME: %v %v", err, warns)
	}
	if _, err := checkOperatorState(filepath.Join(scratchHome, "s.sqlite"), opLog, homes, managedAddr); !errors.Is(err, errOperatorState) {
		t.Fatalf("the plist's call log not refused: %v", err)
	}
	if _, err := checkOperatorState(filepath.Join(scratchHome, "s.sqlite"), filepath.Join(scratchHome, "c.jsonl"), homes, managedAddr); err != nil {
		t.Fatalf("scratch paths refused: %v", err)
	}
}

// TestOperatorStateUnreadableManagedRecordRefuses (RT-22 review, I38): an
// unreadable managed record (a newer schema, a torn write) is named, and
// because it may hold the operator's paths the check fails closed; an
// unreadable scratch record is only named.
func TestOperatorStateUnreadableManagedRecordRefuses(t *testing.T) {
	home := t.TempDir()
	scratch := filepath.Join(home, ".fathom", "daemon", "addr-7001-1", "record.json")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scratch, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	warns, err := checkOperatorState(filepath.Join(home, "s.sqlite"), "", []string{home}, managedAddr)
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], scratch) {
		t.Fatalf("an unreadable scratch record: err %v warns %v, want one warning naming it", err, warns)
	}

	managed := filepath.Join(home, ".fathom", "daemon", "dev.fathom.serve-http", "record.json")
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managed, []byte(`{"schema_version":9,"addr":"`+managedAddr+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = checkOperatorState(filepath.Join(home, "s.sqlite"), "", []string{home}, managedAddr)
	if !errors.Is(err, errOperatorState) || !strings.Contains(err.Error(), managed) || !strings.Contains(err.Error(), "--allow-operator-state") {
		t.Fatalf("an unreadable managed record did not refuse, naming it: %v", err)
	}
	_ = os.Remove(managed)

	legacy, _ := console.DaemonRecordPath(home, managedAddr)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkOperatorState(filepath.Join(home, "s.sqlite"), "", []string{home}, "localhost:"+managedPort); !errors.Is(err, errOperatorState) {
		t.Fatalf("an unreadable legacy record at the managed address did not refuse: %v", err)
	}
}

// TestManagedDaemonAddrFromInstalledPlist (RT-23 review): the census
// takes the managed address from the installed plist before config and
// the 7575 default, so a managed daemon installed with --addr elsewhere,
// still on a schema-1 record, is not read as an orphan and reaped.
func TestManagedDaemonAddrFromInstalledPlist(t *testing.T) {
	home := t.TempDir()
	if got := managedDaemonAddrIn(home, ""); got != managedAddr {
		t.Fatalf("no plist, no config: %q", got)
	}
	if got := managedDaemonAddrIn(home, "127.0.0.1:7700"); got != "127.0.0.1:7700" {
		t.Fatalf("config addr: %q", got)
	}
	writeTestPlist(t, home, "dev.fathom.serve-http", "/s.sqlite", "127.0.0.1:7600", "/c.jsonl")
	if got := managedDaemonAddrIn(home, "127.0.0.1:7700"); got != "127.0.0.1:7600" {
		t.Fatalf("plist addr: %q, want 127.0.0.1:7600", got)
	}
}
