package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-47 wiring (§13.1): the governor writes the ring once a minute and on
// every ladder transition, with the in-flight work, the mode and the
// abandoned count beside the sample.
func TestGovernorWritesRingEveryMinuteAndTransitions(t *testing.T) {
	r := newGovRig(t)
	path := filepath.Join(t.TempDir(), tenancy.RingFileName)
	ring, err := tenancy.OpenRing(path)
	if err != nil {
		t.Fatal(err)
	}
	r.g.ring = ring
	r.g.safeMode = func() bool { return true }
	_, end := r.g.inflight.Begin(t.Context(), tenancy.ClassTree, "references", "fathom", "agent")
	defer end()

	r.ticks(90) // 3 minutes at 2 s
	r.host.set(3200*tenancy.MiB, 0)
	r.ticks(2) // → shed: one transition record
	if err := ring.Close(); err != nil {
		t.Fatal(err)
	}
	rd, err := tenancy.ReadRingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var mins, trans int
	for _, rec := range rd.Records {
		if rec.Transition {
			trans++
			if rec.Level != tenancy.LevelShed || rec.Footprint != 3200*tenancy.MiB {
				t.Fatalf("transition record %+v, want shed at 3200 MiB", rec)
			}
		} else {
			mins++
		}
	}
	if trans != 1 {
		t.Fatalf("%d transition records, want 1", trans)
	}
	if mins < 3 || mins > 4 {
		t.Fatalf("%d minute records over ~3 minutes, want 3-4 (one a minute)", mins)
	}
	last, _ := rd.Last()
	if !last.SafeMode || last.InFlight[tenancy.ClassTree] != 1 || last.OldestTool != "references" || last.OldestClass != tenancy.ClassTree {
		t.Fatalf("ring record lost mode or in-flight work: %+v", last)
	}
	if last.OldestAge < 3*time.Minute {
		t.Fatalf("oldest in-flight age %s, want ≥ 3m", last.OldestAge)
	}
}

// A scratch daemon's ring goes with its record (its id is unique per
// process); a managed daemon's ring stays — it is how a stopped daemon
// stays visible (§13.1).
func TestScratchRingRemovedWithRecordManagedKept(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"addr-7940-4242", "dev.fathom.serve-http"} {
		dir := filepath.Join(root, id)
		ring, err := tenancy.OpenRing(filepath.Join(dir, tenancy.RingFileName))
		if err != nil {
			t.Fatal(err)
		}
		_ = ring.Close()
		if err := os.WriteFile(filepath.Join(dir, "record.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		removeDaemonRecord(filepath.Join(dir, "record.json"))
	}
	if _, err := os.Stat(filepath.Join(root, "addr-7940-4242")); !os.IsNotExist(err) {
		t.Fatalf("scratch daemon dir survives its record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "dev.fathom.serve-http", tenancy.RingFileName)); err != nil {
		t.Fatalf("managed ring removed with its record: %v", err)
	}
}
