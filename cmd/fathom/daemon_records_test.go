package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// resetServingCallLogForTest undoes the process-global serving call log a
// recordServingDaemonAs call sets, so an in-process record never leaks
// into another test's call-log resolution.
func resetServingCallLogForTest(t *testing.T) {
	t.Cleanup(func() {
		servingCallLog.Lock()
		servingCallLog.set, servingCallLog.path, servingCallLog.note = false, "", ""
		servingCallLog.Unlock()
	})
}

// TestDaemonRecordUnderDaemonDirWithLegacyRead (RT-166, §10.2): serve-http
// records itself at ~/.fathom/daemon/<id>/record.json (schema 2, id = the
// label when managed, else addr-<port>-<pid>); a legacy
// ~/.fathom/daemons/<host>-<port>.json still reads for one release, a new
// record for the same address wins over it and prunes it.
func TestDaemonRecordUnderDaemonDirWithLegacyRead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetServingCallLogForTest(t)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

	if got := daemonID(daemonMeta{Managed: true, Label: "dev.fathom.serve-http"}, "127.0.0.1:7581", 99); got != "dev.fathom.serve-http" {
		t.Fatalf("managed id = %q, want the label", got)
	}
	if got := daemonID(daemonMeta{}, "localhost:7853", 1234); got != "addr-7853-1234" {
		t.Fatalf("unmanaged id = %q, want addr-7853-1234", got)
	}
	if _, err := daemonRecordPath(home, "../escape"); err == nil {
		t.Fatal("daemonRecordPath accepted an id that escapes the daemon dir")
	}

	// Legacy only: still read.
	legacyPath, _ := console.DaemonRecordPath(home, "127.0.0.1:7853")
	if err := console.WriteDaemonRecord(legacyPath, console.DaemonRecord{SchemaVersion: 1, Addr: "127.0.0.1:7853", PID: 7, StartedAt: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	rec, ok := readDaemonRecordFor("http://localhost:7853")
	if !ok || rec.PID != 7 {
		t.Fatalf("legacy record not read: %+v ok=%v", rec, ok)
	}

	// A new record for the same address: written under daemon/<id>/, wins,
	// and prunes the legacy file.
	meta := daemonMeta{PPID: 1, IdleExit: 20 * time.Minute, TTL: 8 * time.Hour}
	written, path, err := recordServingDaemonAs(meta, "127.0.0.1:7853", filepath.Join(home, "calls.jsonl"), "", "c0ffee", now)
	if err != nil {
		t.Fatal(err)
	}
	wantPath, _ := daemonRecordPath(home, daemonID(meta, "127.0.0.1:7853", os.Getpid()))
	if path != wantPath || filepath.Base(path) != "record.json" || filepath.Base(filepath.Dir(filepath.Dir(path))) != "daemon" {
		t.Fatalf("record at %s, want %s (~/.fathom/daemon/<id>/record.json)", path, wantPath)
	}
	back, err := console.ReadDaemonRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != 2 || back.Managed || back.IdleExit != "20m0s" || back.TTL != "8h0m0s" || back.ID != written.ID || back.StateRoot != filepath.Dir(path) {
		t.Fatalf("schema-2 fields: %+v", back)
	}
	rec, ok = readDaemonRecordFor("http://127.0.0.1:7853")
	if !ok || rec.PID != os.Getpid() {
		t.Fatalf("the new record does not win: %+v ok=%v", rec, ok)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy record for the same address not pruned (err=%v)", err)
	}

	// Removal on clean exit: the record, and its directory only when
	// nothing else (a lifecycle ledger) lives there.
	removeDaemonRecord(path)
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("empty record dir not removed on clean exit (err=%v)", err)
	}
	_, path2, err := recordServingDaemonAs(meta, "127.0.0.1:7853", "", "", "c0ffee", now)
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(filepath.Dir(path2), "lifecycle.jsonl")
	if err := os.WriteFile(ledger, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeDaemonRecord(path2)
	if _, err := os.Stat(path2); !os.IsNotExist(err) {
		t.Fatalf("record not removed (err=%v)", err)
	}
	if _, err := os.Stat(ledger); err != nil {
		t.Fatalf("removing the record took the ledger with it: %v", err)
	}
}
