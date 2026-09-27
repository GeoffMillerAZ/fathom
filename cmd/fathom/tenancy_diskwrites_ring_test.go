package main

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestRingProjectionDisclosesWhatTheRingDoesNotKeep (RT-52 with RT-162
// integrated): the ring keeps the process's bytes-written counter, not the
// sampled children's nor the 24 h judgment. Rebuilt from the ring, the
// block says so — children disk written unknown, the judgment named as
// not kept — and never renders either as zero or as within budget (I38).
func TestRingProjectionDisclosesWhatTheRingDoesNotKeep(t *testing.T) {
	at := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	rd := tenancy.RingRead{Records: []tenancy.RingRecord{{At: at, Footprint: 100 << 20, Children: 0, DiskWritten: 4096}}}
	s, ok := tenancyFromRing(rd, nil)
	if !ok {
		t.Fatal("ring holds a record")
	}
	if s.ChildDiskWritten != tenancy.Unknown {
		t.Fatalf("children disk written must be unknown from the ring, got %d", s.ChildDiskWritten)
	}
	if !strings.Contains(strings.Join(s.Unknowns, ";"), "children disk written unknown") {
		t.Fatalf("unknowns must name the children's disk reading: %v", s.Unknowns)
	}
	if s.DiskWritesOver || !strings.Contains(s.DiskWrites, "not kept in the ring") {
		t.Fatalf("the 24 h judgment must be named as not kept: %q over=%v", s.DiskWrites, s.DiskWritesOver)
	}

	// `resources` carries the block's judgment line, live or ring.
	rep := resourcesReport{Tenancy: s}
	rep.Disk.Judgment = resourcesDiskJudgment(s)
	if rep.Disk.Judgment != s.DiskWrites {
		t.Fatalf("resources disk judgment %q, want the block's %q", rep.Disk.Judgment, s.DiskWrites)
	}
	if got := resourcesDiskJudgment(nil); !strings.Contains(got, "unknown") {
		t.Fatalf("no block: %q", got)
	}
}
