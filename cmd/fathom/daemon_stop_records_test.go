package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// TestFindManagedDaemonReadsSchema2Record: RT-166 moved daemon records to
// ~/.fathom/daemon/<id>/record.json, so `daemon stop` (RT-4) and the
// signing read (RT-148) must find the managed pid there — not only in the
// legacy ~/.fathom/daemons/<host>-<port>.json — keeping the pid-reuse guard.
func TestFindManagedDaemonReadsSchema2Record(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	started := stopT0.Add(-time.Hour)
	rec := console.DaemonRecord{
		SchemaVersion: console.DaemonRecordSchemaVersion, Addr: "127.0.0.1:7581", PID: 4242,
		StartedAt: started.UTC().Format(time.RFC3339), ID: defaultDaemonLabel, Managed: true,
	}
	p, err := daemonRecordPath(home, defaultDaemonLabel)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(rec)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	ops := newFakeDaemonOps(stopT0)
	ops.procs[4242] = procInfo{PID: 4242, Start: started, Command: "/opt/fathom serve-http --managed"}
	if pid, notes := findManagedDaemon(ops, "127.0.0.1:7581"); pid != 4242 {
		t.Fatalf("findManagedDaemon = %d (%v), want the schema-2 record's pid 4242", pid, notes)
	}
	// The pid-reuse guard still applies to a schema-2 record.
	ops.procs[4242] = procInfo{PID: 4242, Start: started, Command: "/usr/bin/vim"}
	if pid, _ := findManagedDaemon(ops, "127.0.0.1:7581"); pid != 0 {
		t.Fatalf("a reused pid from a schema-2 record was returned: %d", pid)
	}
}
