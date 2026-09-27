package tenancy

// diskwrite.go — RT-162 (docs/design/responsible-tenant.md §12.4). The
// audit's macOS resource report showed 8.6 GB dirtied in 1,395 s against a
// ~99 KB/s limit, and nothing in fathom could see it. The contract: the
// managed daemon writes at most 2 GiB a day outside explicit ingest. The
// sampler reads the process's cumulative bytes written (rusage
// ri_diskio_byteswritten); DiskWriteRing keeps it per minute for 24 h in a
// fixed-size ring, split by whether an explicit ingest was in flight; and
// DiskWriteVerdict judges the day. A breach is disclosed, not enforced: the
// writers are the call log, the WAL and ingest, each bounded by its own
// mechanism. RT-47's persisted resource ring carries the same per-minute
// reading across restarts once it lands.

import (
	"fmt"
	"time"
)

// DiskWriteBudgetPerDay is §12.4's contract.
const DiskWriteBudgetPerDay = 2 * GiB

// diskWriteSlots is one slot a minute for 24 h.
const diskWriteSlots = 24 * 60

type diskMinute struct {
	minute          int64 // Unix minute this slot holds; 0 = empty
	outside, ingest int64 // bytes written in that minute
}

// DiskWriteRing is a fixed-size per-minute ring of bytes written. Not safe
// for concurrent use (the governor's tick owns it).
type DiskWriteRing struct {
	slots    [diskWriteSlots]diskMinute
	last     int64 // the last known cumulative reading
	haveLast bool
	known    bool // at least one known reading was added
}

// Add records a cumulative bytes-written reading taken at `at`; explicit
// says an explicit ingest was in flight. The delta since the previous
// reading lands in at's minute: a first reading counts everything the
// process wrote since it started, and a reading below the previous one (a
// restart reset the counter) counts as new writes. Unknown is skipped.
func (r *DiskWriteRing) Add(at time.Time, cumulative int64, explicit bool) {
	if cumulative < 0 {
		return
	}
	delta := cumulative
	if r.haveLast && cumulative >= r.last {
		delta = cumulative - r.last
	}
	r.last, r.haveLast, r.known = cumulative, true, true
	m := at.Unix() / 60
	s := &r.slots[m%diskWriteSlots]
	if s.minute != m {
		*s = diskMinute{minute: m}
	}
	if explicit {
		s.ingest += delta
	} else {
		s.outside += delta
	}
}

// Window sums the minutes in (now−24h, now] and says how far back the
// ring's readings reach.
func (r *DiskWriteRing) Window(now time.Time) (outside, ingest int64, covered time.Duration) {
	cur := now.Unix() / 60
	oldest := cur
	for i := range r.slots {
		s := &r.slots[i]
		if s.minute == 0 || s.minute > cur || s.minute <= cur-diskWriteSlots {
			continue
		}
		outside += s.outside
		ingest += s.ingest
		if s.minute < oldest {
			oldest = s.minute
		}
	}
	return outside, ingest, time.Duration(cur-oldest+1) * time.Minute
}

// DiskWriteJudgment is DiskWriteVerdict's answer.
type DiskWriteJudgment struct {
	Outside int64         // bytes written in the window outside explicit ingest
	Ingest  int64         // bytes written while an explicit ingest was in flight
	Budget  int64         // the per-day budget judged against
	Covered time.Duration // how much of the 24 h the readings cover
	Over    bool
	Unknown bool // no reading could be taken: never rendered as 0
	Line    string
}

// DiskWriteVerdict judges the last 24 h of r at now against budget.
func DiskWriteVerdict(r *DiskWriteRing, now time.Time, budget int64) DiskWriteJudgment {
	j := DiskWriteJudgment{Budget: budget}
	if r == nil || !r.known {
		j.Unknown = true
		j.Line = "disk written unknown — the process's bytes-written reading was not taken (rusage unavailable)"
		return j
	}
	j.Outside, j.Ingest, j.Covered = r.Window(now)
	span := "24 h"
	if j.Covered < 24*time.Hour {
		span = j.Covered.String() + " (since the first reading)"
	}
	j.Line = fmt.Sprintf("written %s/%s in the last %s outside explicit ingest (+%s during explicit ingest)",
		formatGiBRatio(j.Outside, budget), FormatBytes(budget), span, FormatBytes(j.Ingest))
	if j.Outside > budget {
		j.Over = true
		j.Line += fmt.Sprintf(" — over the %s/day budget; disclosed, not enforced: the writers are the call log, the WAL and ingest — fathom resources", FormatBytes(budget))
	} else if j.Covered < 24*time.Hour {
		// The disk-write ring is in memory (not persisted): a young
		// daemon's readings are not a day (review finding 4).
		j.Line += " — within budget so far"
	} else {
		j.Line += " — within budget"
	}
	return j
}

// formatGiBRatio renders n as the numerator of "n/budget GiB".
func formatGiBRatio(n, budget int64) string {
	if budget%GiB == 0 && budget >= GiB {
		s := fmt.Sprintf("%.2f", float64(n)/float64(GiB))
		for len(s) > 1 && s[len(s)-1] == '0' {
			s = s[:len(s)-1]
		}
		if s[len(s)-1] == '.' {
			s = s[:len(s)-1]
		}
		return s
	}
	return FormatBytes(n)
}
