package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestGovernorRollsDaySummariesOnTheSamplerTick (RT-184): the existing tick
// folds each minute sample and transition into the day log beside the ring,
// across UTC midnight, with the refuse window's entering trigger.
func TestGovernorRollsDaySummariesOnTheSamplerTick(t *testing.T) {
	r := newGovRig(t)
	path := filepath.Join(t.TempDir(), tenancy.DaysFileName)
	days, err := tenancy.OpenDayLog(path)
	if err != nil {
		t.Fatal(err)
	}
	r.g.days = days
	r.now = time.Date(2026, 10, 1, 23, 57, 0, 0, time.UTC)

	r.host.set(2*tenancy.GiB, 0)
	r.ticks(60) // two minutes
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(100) // past midnight, at refuse
	r.host.set(tenancy.GiB, 0)
	r.ticks(120)

	rows := days.Rows()
	if len(rows) != 2 || rows[0].Date != "2026-10-01" || rows[1].Date != "2026-10-02" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Samples < 2 || rows[0].MaxFootprint < 2*tenancy.GiB {
		t.Errorf("day 1 = %+v", rows[0])
	}
	var found bool
	for _, row := range rows {
		for _, w := range row.RefuseWindows {
			if strings.HasPrefix(w.Trigger, "memory") || strings.HasPrefix(w.Trigger, "continued") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no refuse window with a memory trigger: %+v", rows)
	}
	if err := days.Flush(r.now, true); err != nil {
		t.Fatal(err)
	}
	if got, err := tenancy.ReadDayLog(path); err != nil || len(got) != 2 {
		t.Fatalf("persisted %d rows, err %v", len(got), err)
	}
}
