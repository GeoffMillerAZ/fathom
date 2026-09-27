package tenancy

import (
	"strings"
	"testing"
	"time"
)

// TestDiskWriteBudgetDisclosed pins RT-162 (design §12.4): the sampler's
// cumulative bytes written (process rusage) are kept per minute in a
// fixed-size 24 h ring, split by whether an explicit ingest was in flight,
// and DiskWriteVerdict judges the last 24 h outside explicit ingest against
// 2 GiB/day — disclosed, never enforced, and never a zero when the reading
// is unknown (I38).
func TestDiskWriteBudgetDisclosed(t *testing.T) {
	t0 := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	var r DiskWriteRing
	if got := len(r.slots); got != 24*60 {
		t.Fatalf("ring slots = %d, want 1440 (one a minute for 24 h, fixed size)", got)
	}

	v := DiskWriteVerdict(&r, t0, DiskWriteBudgetPerDay)
	if !v.Unknown || !strings.Contains(v.Line, "unknown") {
		t.Fatalf("an empty ring = %+v, want unknown, never 0 written", v)
	}
	r.Add(t0, Unknown, false)
	if v := DiskWriteVerdict(&r, t0, DiskWriteBudgetPerDay); !v.Unknown {
		t.Fatalf("only unknown readings = %+v, want unknown", v)
	}

	r.Add(t0.Add(time.Minute), 512*MiB, false)         // since process start
	r.Add(t0.Add(2*time.Minute), 1*GiB, false)         // +512 MiB outside
	r.Add(t0.Add(3*time.Minute), 6*GiB, true)          // +5 GiB during explicit ingest
	r.Add(t0.Add(4*time.Minute), 6*GiB+512*MiB, false) // +512 MiB outside
	v = DiskWriteVerdict(&r, t0.Add(5*time.Minute), DiskWriteBudgetPerDay)
	if v.Unknown || v.Outside != 1536*MiB || v.Ingest != 5*GiB || v.Over {
		t.Fatalf("verdict = %+v, want 1.5 GiB outside, 5 GiB during ingest, within budget", v)
	}
	if !strings.Contains(v.Line, "1.5/2 GiB") || !strings.Contains(v.Line, "outside explicit ingest") || !strings.Contains(v.Line, "5 GiB during explicit ingest") {
		t.Errorf("line = %q", v.Line)
	}
	// Review finding 4: five minutes of readings are not a day — the line
	// names the window and says "so far", never "within budget" for 24 h.
	if v.Covered >= 24*time.Hour || !strings.Contains(v.Line, "since the first reading") || !strings.Contains(v.Line, "within budget so far") {
		t.Errorf("a 5-minute ring's line = %q (covered %s), want its window named and \"within budget so far\"", v.Line, v.Covered)
	}

	// A restart resets the cumulative counter: the new value is new writes.
	r.Add(t0.Add(6*time.Minute), 768*MiB, false)
	v = DiskWriteVerdict(&r, t0.Add(7*time.Minute), DiskWriteBudgetPerDay)
	if v.Outside != 2304*MiB || !v.Over {
		t.Fatalf("after a counter reset = %+v, want 2.25 GiB outside and over", v)
	}
	if !strings.Contains(v.Line, "over the 2 GiB/day budget") || !strings.Contains(v.Line, "disclosed, not enforced") {
		t.Errorf("over-budget line = %q", v.Line)
	}

	// 24 h later the old minutes have left the window.
	later := t0.Add(24*time.Hour + 10*time.Minute)
	r.Add(later, 768*MiB+100*MiB, false)
	v = DiskWriteVerdict(&r, later, DiskWriteBudgetPerDay)
	if v.Outside != 100*MiB || v.Ingest != 0 || v.Over {
		t.Fatalf("a day later = %+v, want only the last minute's 100 MiB", v)
	}
}
