package tenancy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var dayMarks = DayMarks{Budget: 4 * GiB, Shed: 3 * GiB, Refuse: 4 * GiB, Exit: 8 * GiB, ExitEnabled: true}

func minuteRec(at time.Time, lv Level, fp int64) RingRecord {
	return RingRecord{At: at, Level: lv, Footprint: fp, Children: 100 * MiB}
}

func diskFloorCause() Cause {
	return Cause{Kind: CauseHost, Detail: "swap volume free 3.0 GiB < 5.0 GiB"}
}

func openDays(t *testing.T) (*DayLog, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), DaysFileName)
	d, err := OpenDayLog(p)
	if err != nil {
		t.Fatal(err)
	}
	return d, p
}

// TestDayLog_RollsAcrossUTCMidnight: samples before and after 00:00 UTC land
// in two rows, each with its own count, max and marks (RT-184).
func TestDayLog_RollsAcrossUTCMidnight(t *testing.T) {
	d, _ := openDays(t)
	base := time.Date(2026, 10, 1, 23, 58, 0, 0, time.UTC)
	for i, fp := range []int64{1 * GiB, 2 * GiB, 1500 * MiB, 900 * MiB} {
		d.Observe(minuteRec(base.Add(time.Duration(i)*time.Minute), LevelNormal, fp), dayMarks, nil)
	}
	rows := d.Rows()
	if len(rows) != 2 || rows[0].Date != "2026-10-01" || rows[1].Date != "2026-10-02" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Samples != 2 || rows[0].MaxFootprint != 2*GiB || rows[0].MaxCombined != 2*GiB+100*MiB {
		t.Errorf("day 1 = %+v", rows[0])
	}
	if rows[1].Samples != 2 || rows[1].MaxFootprint != 1500*MiB || rows[1].Budget != 4*GiB || rows[1].Exit != 8*GiB {
		t.Errorf("day 2 = %+v", rows[1])
	}
}

// TestDayLog_RestartMergesIntoTheSameDay: a daemon restart mid-day reopens the
// file and keeps adding to that day's row.
func TestDayLog_RestartMergesIntoTheSameDay(t *testing.T) {
	d, p := openDays(t)
	at := time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC)
	d.Observe(minuteRec(at, LevelShed, 3500*MiB), dayMarks, nil)
	if err := d.Flush(at, true); err != nil {
		t.Fatal(err)
	}
	d2, err := OpenDayLog(p)
	if err != nil {
		t.Fatal(err)
	}
	d2.Observe(minuteRec(at.Add(5*time.Minute), LevelNormal, 1*GiB), dayMarks, nil)
	rows := d2.Rows()
	if len(rows) != 1 || rows[0].Samples != 2 || rows[0].ShedMinutes != 1 || rows[0].MaxFootprint != 3500*MiB {
		t.Fatalf("restart did not merge: %+v", rows)
	}
}

// TestDayLog_KeepsEightRows: a ninth day evicts the oldest.
func TestDayLog_KeepsEightRows(t *testing.T) {
	d, p := openDays(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		d.Observe(minuteRec(base.AddDate(0, 0, i), LevelNormal, GiB), dayMarks, nil)
	}
	rows := d.Rows()
	if len(rows) != DaysKeep || rows[0].Date != "2026-10-03" || rows[DaysKeep-1].Date != "2026-10-10" {
		t.Fatalf("rows = %v", dates(rows))
	}
	if err := d.Flush(base, true); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDayLog(p)
	if err != nil || len(got) != DaysKeep {
		t.Fatalf("read back %d rows, err %v", len(got), err)
	}
}

// TestDayLog_SampleOlderThanTheKeptRowsIsDropped: after a clock step back
// past the eight kept days, a sample for an evicted date is not counted into
// the newest day (it used to land on the last row).
func TestDayLog_SampleOlderThanTheKeptRowsIsDropped(t *testing.T) {
	d, _ := openDays(t)
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < DaysKeep; i++ {
		d.Observe(minuteRec(base.AddDate(0, 0, i), LevelNormal, GiB), dayMarks, nil)
	}
	d.Observe(minuteRec(base.AddDate(0, 0, -20), LevelNormal, 7*GiB), dayMarks, nil)
	rows := d.Rows()
	if len(rows) != DaysKeep || rows[0].Date != "2026-10-01" || rows[DaysKeep-1].Date != "2026-10-08" {
		t.Fatalf("rows = %v", dates(rows))
	}
	if last := rows[DaysKeep-1]; last.Samples != 1 || last.MaxFootprint != GiB {
		t.Errorf("an evicted day's sample landed on %s: %+v", last.Date, last)
	}
}

func dates(rows []DaySummary) []string {
	var o []string
	for _, r := range rows {
		o = append(o, r.Date)
	}
	return o
}

// TestDayLog_RefuseWindowRecordsItsTrigger: the disk-floor refuse of the
// 10-03 reading is a window whose trigger says so, not "memory".
func TestDayLog_RefuseWindowRecordsItsTrigger(t *testing.T) {
	d, _ := openDays(t)
	at := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	tr := Transition{From: LevelNormal, To: LevelRefuse, At: at, Changed: true, Cause: diskFloorCause()}
	d.Observe(RingRecord{At: at, Transition: true, Level: LevelRefuse, Footprint: GiB}, dayMarks, &tr)
	for i := 1; i <= 6; i++ {
		d.Observe(minuteRec(at.Add(time.Duration(i)*time.Minute), LevelRefuse, GiB), dayMarks, nil)
	}
	d.Observe(minuteRec(at.Add(7*time.Minute), LevelNormal, GiB), dayMarks, nil)
	// a second, memory-triggered window
	at2 := at.Add(time.Hour)
	tr2 := Transition{From: LevelShed, To: LevelRefuse, At: at2, Changed: true, Cause: Cause{Kind: CauseMark, Detail: "footprint 4.1 GiB >= refuse mark"}}
	d.Observe(RingRecord{At: at2, Transition: true, Level: LevelRefuse, Footprint: 4200 * MiB}, dayMarks, &tr2)
	d.Observe(minuteRec(at2.Add(time.Minute), LevelRefuse, 4200*MiB), dayMarks, nil)
	r := d.Rows()[0]
	if len(r.RefuseWindows) != 2 {
		t.Fatalf("windows = %+v", r.RefuseWindows)
	}
	w0, w1 := r.RefuseWindows[0], r.RefuseWindows[1]
	if w0.Minutes != 7 || !strings.HasPrefix(w0.Trigger, "disk floor") || w0.Start != "2026-10-03T04:00:00Z" {
		t.Errorf("w0 = %+v", w0)
	}
	if w1.Minutes != 2 || !strings.HasPrefix(w1.Trigger, "memory") {
		t.Errorf("w1 = %+v", w1)
	}
	if r.RefuseMinutes != 9 {
		t.Errorf("refuse minutes = %d, want 9", r.RefuseMinutes)
	}
	if r.OverBudgetOutsideRefuse != 0 || r.OverExit != 0 {
		t.Errorf("a refuse-window sample over budget counted outside refuse: %+v", r)
	}
}

// TestDayLog_ExitRungAndOverBudgetCounts: an exit transition is an exit rung
// (forced flush); a sample over budget below refuse is counted outside refuse.
func TestDayLog_ExitRungAndOverBudgetCounts(t *testing.T) {
	d, _ := openDays(t)
	at := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	d.Observe(minuteRec(at, LevelShed, 5*GiB), dayMarks, nil)
	d.Observe(minuteRec(at.Add(time.Minute), LevelNormal, 9*GiB), dayMarks, nil)
	tr := Transition{From: LevelRefuse, To: LevelExit, At: at, Changed: true}
	if !d.Observe(RingRecord{At: at, Transition: true, Level: LevelExit, Footprint: 9 * GiB}, dayMarks, &tr) {
		t.Error("an exit-rung transition must ask for an immediate flush")
	}
	r := d.Rows()[0]
	if r.OverBudgetOutsideRefuse != 2 || r.OverExit != 1 || r.ExitRungs != 1 {
		t.Errorf("row = %+v", r)
	}
}

// TestDayLog_CorruptFileStartsEmpty: the file is a cache; a torn one is
// replaced, never an error that stops the sampler.
func TestDayLog_CorruptFileStartsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), DaysFileName)
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := OpenDayLog(p)
	if err != nil || len(d.Rows()) != 0 {
		t.Fatalf("OpenDayLog = %v, %d rows", err, len(d.Rows()))
	}
	if _, err := ReadDayLog(p); err == nil {
		t.Error("ReadDayLog must name an unreadable file (I38), not return no rows")
	}
}

// TestDayLog_FlushRateAndAtomic: at most one write a minute unless forced; no
// temp file is left behind.
func TestDayLog_FlushRateAndAtomic(t *testing.T) {
	d, p := openDays(t)
	at := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	d.Observe(minuteRec(at, LevelNormal, GiB), dayMarks, nil)
	if err := d.Flush(at, false); err != nil {
		t.Fatal(err)
	}
	d.Observe(minuteRec(at.Add(10*time.Second), LevelNormal, 2*GiB), dayMarks, nil)
	if err := d.Flush(at.Add(10*time.Second), false); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadDayLog(p)
	if got[0].MaxFootprint != GiB {
		t.Errorf("a second write inside a minute landed: %+v", got[0])
	}
	if err := d.Flush(at.Add(61*time.Second), false); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadDayLog(p)
	if got[0].MaxFootprint != 2*GiB {
		t.Errorf("the write after a minute did not land: %+v", got[0])
	}
	if ents, _ := os.ReadDir(filepath.Dir(p)); len(ents) != 1 {
		t.Errorf("leftover files: %v", ents)
	}
}

func goodDay(date string) DaySummary {
	return DaySummary{Date: date, Samples: 1440, MaxFootprint: 3 * GiB, MaxCombined: 3 * GiB, Budget: 4 * GiB, Exit: 8 * GiB, ExitsKnown: true}
}

func sevenDays(mut func(i int, d *DaySummary)) []DaySummary {
	var rows []DaySummary
	for i := 0; i < 8; i++ {
		d := goodDay(time.Date(2026, 10, 2+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
		if mut != nil {
			mut(i, &d)
		}
		rows = append(rows, d)
	}
	return rows // 10-02 .. 10-09
}

// TestJudgeRTM1 pins the verdict rules over the last 7 complete UTC days.
func TestJudgeRTM1(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC) // complete days: 10-02 .. 10-08
	cases := []struct {
		name   string
		rows   []DaySummary
		state  string
		inText string
	}{
		{"pass", sevenDays(nil), "PASS", ""},
		{"refuse ceiling is 15 min a day, 15 passes", sevenDays(func(i int, d *DaySummary) {
			if i == 2 {
				d.RefuseMinutes = 15
			}
		}), "PASS", ""},
		{"16 refuse minutes fails and names the day", sevenDays(func(i int, d *DaySummary) {
			if i == 2 {
				d.RefuseMinutes = 16
			}
		}), "FAIL", "2026-10-04"},
		{"over the exit mark", sevenDays(func(i int, d *DaySummary) {
			if i == 0 {
				d.OverExit = 1
			}
		}), "FAIL", "exit mark"},
		{"over budget outside refuse", sevenDays(func(i int, d *DaySummary) {
			if i == 6 {
				d.OverBudgetOutsideRefuse = 3
			}
		}), "FAIL", "budget"},
		{"an exit rung", sevenDays(func(i int, d *DaySummary) {
			if i == 1 {
				d.ExitRungs = 1
			}
		}), "FAIL", "exit rung"},
		{"an unclean exit", sevenDays(func(i int, d *DaySummary) {
			if i == 3 {
				d.UncleanExits = 1
			}
		}), "FAIL", "unclean"},
		{"a missing day is incomplete", append(sevenDays(nil)[:2], sevenDays(nil)[3:]...), "INCOMPLETE", "2026-10-04"},
		{"a thin day is incomplete", sevenDays(func(i int, d *DaySummary) {
			if i == 4 {
				d.Samples = 600
			}
		}), "INCOMPLETE", "2026-10-06"},
		{"ledger coverage unknown is incomplete", sevenDays(func(i int, d *DaySummary) {
			if i == 5 {
				d.ExitsKnown = false
			}
		}), "INCOMPLETE", "2026-10-07"},
		{"a violation beats a gap", append(sevenDays(func(i int, d *DaySummary) { d.UncleanExits = 1 })[:2], sevenDays(nil)[3:]...), "FAIL", "unclean"},
		{"no rows", nil, "INCOMPLETE", "2026-10-02"},
	}
	for _, c := range cases {
		v := JudgeRTM1(c.rows, now)
		if v.State != c.state || !strings.Contains(v.Detail, c.inText) {
			t.Errorf("%s: %+v, want %s containing %q", c.name, v, c.state, c.inText)
		}
	}
	// today's partial row never counts as a complete day
	rows := sevenDays(nil)
	rows[7].OverExit = 5
	if v := JudgeRTM1(rows, now); v.State != "PASS" {
		t.Errorf("today's partial row judged: %+v", v)
	}
}

// splitDay sets a day's windows: one per (trigger, minutes, pastShed).
type splitWin struct {
	trigger  string
	minutes  int
	pastShed *bool
}

func withWindows(ws ...splitWin) func(i int, d *DaySummary) {
	return func(i int, d *DaySummary) {
		if i != 2 {
			return
		}
		d.RefuseMinutes = 0
		for _, w := range ws {
			d.RefuseWindows = append(d.RefuseWindows, RefuseWindow{Start: "2026-10-04T01:00:00Z", End: "2026-10-04T01:00:00Z", Minutes: w.minutes, Trigger: w.trigger, PastShed: w.pastShed})
			d.RefuseMinutes += w.minutes
		}
	}
}

func boolp(b bool) *bool { return &b }

// TestJudgeRTM1_SplitsSelfCausedFromHostStarved pins the amended RT-M1 clause
// (RT-187): refuse minutes entered past fathom's own shed mark count against
// the 15 min/day bar; minutes entered on a host trigger while under it are
// host-starved, against a separate 60 min/day availability bar. Windows logged
// before the past_shed field classify by trigger: memory, unknown and
// continued windows are self-caused, "host trigger" and "disk floor" are
// host-starved.
func TestJudgeRTM1_SplitsSelfCausedFromHostStarved(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	legacyFloor := "disk floor: swap volume free 3.0 GiB < 5.0 GiB"
	t.Run("the 10-09 shape: 27 host-floor minutes under the shed mark pass the refuse clause", func(t *testing.T) {
		rows := sevenDays(withWindows(splitWin{legacyFloor, 27, nil}))
		v := JudgeRTM1(rows, now)
		if v.State != "PASS" {
			t.Fatalf("verdict %+v", v)
		}
		self, host := rows[2].RefuseSplit()
		if self != 0 || host != 27 {
			t.Errorf("split = %d self, %d host, want 0, 27", self, host)
		}
	})
	t.Run("20 self-caused minutes fail the refuse clause", func(t *testing.T) {
		v := JudgeRTM1(sevenDays(withWindows(splitWin{"memory: footprint 4.1 GiB >= refuse mark", 20, nil})), now)
		if v.State != "FAIL" || !strings.Contains(v.Criterion, "15 min/day") || v.Day != "2026-10-04" {
			t.Fatalf("verdict %+v", v)
		}
	})
	t.Run("a host trigger entered past the shed mark is self-caused", func(t *testing.T) {
		v := JudgeRTM1(sevenDays(withWindows(splitWin{"host trigger: memory pressure critical", 20, boolp(true)})), now)
		if v.State != "FAIL" || !strings.Contains(v.Criterion, "15 min/day") {
			t.Fatalf("verdict %+v", v)
		}
	})
	t.Run("a memory-labelled window recorded under the shed mark is host-starved by the field", func(t *testing.T) {
		rows := sevenDays(withWindows(splitWin{"host trigger: memory pressure critical", 30, boolp(false)}))
		if v := JudgeRTM1(rows, now); v.State != "PASS" {
			t.Fatalf("verdict %+v", v)
		}
	})
	t.Run("61 host-starved minutes fail availability", func(t *testing.T) {
		v := JudgeRTM1(sevenDays(withWindows(splitWin{legacyFloor, 61, nil})), now)
		if v.State != "FAIL" || !strings.Contains(v.Criterion, "host-starved") || !strings.Contains(v.Detail, "61") || v.Day != "2026-10-04" {
			t.Fatalf("verdict %+v", v)
		}
	})
	t.Run("60 host-starved minutes pass", func(t *testing.T) {
		if v := JudgeRTM1(sevenDays(withWindows(splitWin{legacyFloor, 60, nil})), now); v.State != "PASS" {
			t.Fatalf("verdict %+v", v)
		}
	})
	t.Run("minutes with no window record count as self-caused", func(t *testing.T) {
		self, host := DaySummary{RefuseMinutes: 16}.RefuseSplit()
		if self != 16 || host != 0 {
			t.Errorf("split = %d, %d", self, host)
		}
	})
}

// TestDayLog_RecordsPastShedAtEntry: the window records whether fathom's own
// combined footprint was at or past the shed mark when it was entered.
func TestDayLog_RecordsPastShedAtEntry(t *testing.T) {
	d, _ := openDays(t)
	at := time.Date(2026, 10, 3, 4, 0, 0, 0, time.UTC)
	host := Transition{From: LevelNormal, To: LevelRefuse, At: at, Changed: true, Cause: diskFloorCause()}
	d.Observe(RingRecord{At: at, Transition: true, Level: LevelRefuse, Footprint: GiB, Children: 100 * MiB}, dayMarks, &host)
	d.Observe(minuteRec(at.Add(5*time.Minute), LevelNormal, GiB), dayMarks, nil)
	at2 := at.Add(time.Hour)
	host2 := Transition{From: LevelShed, To: LevelRefuse, At: at2, Changed: true, Cause: diskFloorCause()}
	d.Observe(RingRecord{At: at2, Transition: true, Level: LevelRefuse, Footprint: 3 * GiB, Children: 100 * MiB}, dayMarks, &host2)
	w := d.Rows()[0].RefuseWindows
	if len(w) != 2 || w[0].PastShed == nil || *w[0].PastShed || w[1].PastShed == nil || !*w[1].PastShed {
		t.Fatalf("windows = %+v", w)
	}
}
