package tenancy

// daysummary.go — the per-UTC-day summary beside the resource ring (RT-184,
// design §13.1, §15.3). The ring holds 24 h, so six of RT-M1's seven days
// could only be bounded, never read. The governor's existing sampler tick
// rolls each minute sample and ladder transition into one row per UTC day
// and rewrites a small JSON file beside resources.ring, at most once a
// minute, atomically, keeping DaysKeep rows (a ninth day evicts the oldest).
// No goroutine, no table: the caller holds its own lock and passes its own
// clock's times. Exit rungs seen by the sampler are counted here; unclean
// exits and the ledger's over-budget exits are not the sampler's to see and
// are merged from lifecycle.jsonl at read time (cmd/fathom/resources.go).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Day-log geometry.
const (
	// DaysFileName is the day log's name beside RingFileName.
	DaysFileName = "resources.days"
	// DaysKeep is the rows kept: RT-M1's seven complete days plus today.
	DaysKeep = 8
	// DaysWriteEvery bounds the rewrite rate (RT-M12's disk budget).
	DaysWriteEvery = time.Minute

	daysVersion      = 1
	maxRefuseWindows = 64
	// A window extends over a sample at most this long after its last one
	// (a missed sample or a restart), else the next refuse starts a window.
	windowGap = 2 * time.Minute
	// DayCompleteSamples is the minute samples a day needs before the RT-M1
	// verdict counts it as read: 90% of 1440. A daemon down for a deploy
	// loses minutes honestly; a mostly-dark day is not evidence.
	DayCompleteSamples = 1296
	// RefuseMinutesPerDay is RT-M1's refuse-window ceiling.
	RefuseMinutesPerDay = 15
	// HostStarvedMinutesPerDay is RT-M1's separate availability bar for the
	// refuse minutes entered on a host trigger while fathom was under its
	// own shed mark (amended 2026-10-11).
	HostStarvedMinutesPerDay = 60
)

// DayMarks are the ladder marks in force when a sample is judged.
type DayMarks struct {
	Budget, Shed, Refuse, Exit int64
	ExitEnabled                bool
}

// RefuseWindow is one stretch at refuse or above.
type RefuseWindow struct {
	Start   string `json:"start"`
	End     string `json:"end"`
	Minutes int    `json:"minutes"`
	// Trigger is what entered the window: "memory", "host trigger" or
	// "disk floor" (the swap-volume floor), with the ladder's own reason.
	Trigger string `json:"trigger"`
	// PastShed is whether fathom's own footprint (self + children) was at or
	// past the shed mark when the window was entered. The trigger alone
	// cannot say: a host trigger can fire while fathom is also past shed.
	// Nil on windows logged before the field existed; those classify by
	// trigger (see SelfCaused).
	PastShed *bool `json:"past_shed,omitempty"`
}

// SelfCaused reports whether the window counts against RT-M1's 15 min/day
// bar: entered past fathom's own shed mark. A window with no PastShed record
// (logged before the field) is host-starved only when its trigger is a host
// one ("host trigger", "disk floor"); a memory, unknown or continued window
// is self-caused, the conservative reading.
func (w RefuseWindow) SelfCaused() bool {
	if w.PastShed != nil {
		return *w.PastShed
	}
	return !strings.HasPrefix(w.Trigger, "host trigger") && !strings.HasPrefix(w.Trigger, "disk floor")
}

// RefuseSplit divides the day's refuse minutes into self-caused and
// host-starved. Minutes with no window record (RefuseMinutes beyond the
// windows' sum) are self-caused.
func (r DaySummary) RefuseSplit() (self, host int) {
	sum := 0
	for _, w := range r.RefuseWindows {
		sum += w.Minutes
		if w.SelfCaused() {
			self += w.Minutes
		} else {
			host += w.Minutes
		}
	}
	if rest := r.RefuseMinutes - sum; rest > 0 {
		self += rest
	}
	return self, host
}

// DaySummary is one UTC day.
type DaySummary struct {
	Date    string `json:"date"` // UTC, 2006-01-02
	Samples int    `json:"samples"`
	// MaxFootprint is the largest phys_footprint; MaxCombined adds children.
	MaxFootprint int64 `json:"max_footprint"`
	MaxCombined  int64 `json:"max_combined"`
	// The marks in force at the day's last sample.
	Budget      int64 `json:"budget"`
	Shed        int64 `json:"shed_mark"`
	Refuse      int64 `json:"refuse_mark"`
	Exit        int64 `json:"exit_mark"`
	ExitEnabled bool  `json:"exit_enabled"`
	// OverBudgetOutsideRefuse counts samples below the refuse rung whose
	// footprint exceeded the budget then in force; OverExit counts samples
	// at any rung whose footprint exceeded the exit mark then in force.
	OverBudgetOutsideRefuse int `json:"over_budget_outside_refuse"`
	OverExit                int `json:"over_exit"`
	ShedMinutes             int `json:"shed_minutes"`
	RefuseMinutes           int `json:"refuse_minutes"`
	// SelfRefuseMinutes and HostStarvedMinutes split RefuseMinutes (RT-M1,
	// amended 2026-10-11); derived by the read side from the windows, never
	// persisted.
	SelfRefuseMinutes  int            `json:"self_refuse_minutes"`
	HostStarvedMinutes int            `json:"host_starved_minutes"`
	RefuseWindows      []RefuseWindow `json:"refuse_windows,omitempty"`
	// ExitRungs and UncleanExits: the sampler counts the exit rung it
	// stepped onto; the read side raises both from the lifecycle ledger.
	ExitRungs    int `json:"exit_rungs"`
	UncleanExits int `json:"unclean_exits"`
	// ExitsKnown is set by the read side when the ledger covers the day;
	// never persisted true.
	ExitsKnown bool `json:"exits_known"`
	// RungEntries are the day's rung entries by rung, trigger and the host
	// pressure at entry (RT-M13), rolled from the ladder's transitions.
	RungEntries []RungEntry `json:"rung_entries,omitempty"`
	// RungEntriesKnown is set on a row opened by a sampler that records
	// rung entries. A row logged before the field cannot say what entered
	// its rungs: RT-M13 reads it INCOMPLETE, never as zero entries (I38).
	RungEntriesKnown bool `json:"rung_entries_known,omitempty"`
	// ShedMinutesByTrigger splits the day's shed + refuse minutes by the
	// trigger that entered the rung the ladder was on (RungEntry's trigger
	// names; O1238). Its sum is ShedMinutes + RefuseMinutes on a row opened
	// by a sampler that records it; minutes beyond the sum (a row logged
	// before the field) are unattributed, never zero (I38).
	ShedMinutesByTrigger map[string]int `json:"shed_minutes_by_trigger,omitempty"`
}

// UnattributedShedMinutes is the day's shed + refuse minutes no trigger
// accounts for: those rolled before ShedMinutesByTrigger existed.
func (r DaySummary) UnattributedShedMinutes() int {
	sum := 0
	for _, n := range r.ShedMinutesByTrigger {
		sum += n
	}
	return max(r.ShedMinutes+r.RefuseMinutes-sum, 0)
}

// attributeShed counts n shed-or-above minutes against trigger t.
func (r *DaySummary) attributeShed(t RungTrigger, n int) {
	if n <= 0 {
		return
	}
	if r.ShedMinutesByTrigger == nil {
		r.ShedMinutesByTrigger = map[string]int{}
	}
	r.ShedMinutesByTrigger[t.String()] += n
}

type dayFile struct {
	Version int          `json:"version"`
	Days    []DaySummary `json:"days"`
}

// DayLog is the in-memory rows and their file. Not safe for concurrent use
// (the governor calls it under its own lock).
type DayLog struct {
	path      string
	rows      []DaySummary
	dirty     bool
	lastWrite time.Time
	// open: the newest refuse window is still running (the last sample was
	// at refuse); fresh: nothing observed yet this process.
	open, fresh bool
	trigger     string // the entering trigger of the transition just seen
	// pastShed: fathom's combined footprint was at or past the shed mark at
	// that transition; pastShedKnown: the transition carried a reading.
	pastShed, pastShedKnown bool
	// entered is the trigger that entered each rung on the ladder's last
	// rise to or past it (TriggerUnknown until a rise is seen this process;
	// the ladder starts at normal). A rise sets every rung it passes, so a
	// step-down returns to the trigger that entered the lower rung.
	entered [numLevels]RungTrigger
}

// enteredBy is the trigger that entered rung l.
func (d *DayLog) enteredBy(l Level) RungTrigger {
	if int(l) < len(d.entered) {
		return d.entered[l]
	}
	return TriggerUnknown
}

// OpenDayLog reads the rows at path. A missing, torn or foreign file
// starts empty: the log is a cache of recent history (the ring's rule).
func OpenDayLog(path string) (*DayLog, error) {
	d := &DayLog{path: path, fresh: true}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if rows, err := ReadDayLog(path); err == nil {
		d.rows = rows
		for i := range d.rows {
			d.rows[i].ExitsKnown = false
		}
	}
	return d, nil
}

// ReadDayLog reads the rows at path, oldest first. A missing file returns
// the not-exist error; an unreadable one a named error (I38).
func ReadDayLog(path string) ([]DaySummary, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f dayFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("day summaries unreadable: %w", err)
	}
	if f.Version != daysVersion {
		return nil, fmt.Errorf("day summaries unreadable: version %d, want %d", f.Version, daysVersion)
	}
	sort.SliceStable(f.Days, func(i, j int) bool { return f.Days[i].Date < f.Days[j].Date })
	return f.Days, nil
}

// Rows is a copy of the rows, oldest first.
func (d *DayLog) Rows() []DaySummary {
	out := make([]DaySummary, len(d.rows))
	copy(out, d.rows)
	return out
}

// RefuseTrigger names what entered a refuse rung from the ladder's cause.
func RefuseTrigger(c Cause) string {
	switch {
	case c.Kind == CauseMark:
		return "memory: " + c.Detail
	case c.Kind == CauseHost && strings.Contains(c.Detail, "swap volume"):
		return "disk floor: " + c.Detail
	case c.Kind == CauseHost:
		return "host trigger: " + c.Detail
	}
	return "unknown"
}

func (d *DayLog) day(at time.Time) *DaySummary {
	date := at.UTC().Format("2006-01-02")
	n := len(d.rows)
	if n > 0 && d.rows[n-1].Date == date {
		return &d.rows[n-1]
	}
	for i := range d.rows { // an out-of-order sample after a clock step
		if d.rows[i].Date == date {
			return &d.rows[i]
		}
	}
	d.rows = append(d.rows, DaySummary{Date: date, RungEntriesKnown: true})
	sort.SliceStable(d.rows, func(i, j int) bool { return d.rows[i].Date < d.rows[j].Date })
	if len(d.rows) > DaysKeep {
		d.rows = append([]DaySummary(nil), d.rows[len(d.rows)-DaysKeep:]...)
	}
	for i := range d.rows {
		if d.rows[i].Date == date {
			return &d.rows[i]
		}
	}
	// date is older than every kept row (a clock step back past DaysKeep
	// days): the sample is dropped into a row nothing keeps, never counted
	// into another day.
	return &DaySummary{Date: date}
}

// Observe rolls one ring record into its day. tr is the ladder transition
// when rec is a transition record. It returns true when the row should be
// flushed now (an exit rung: the process is about to end).
func (d *DayLog) Observe(rec RingRecord, m DayMarks, tr *Transition) (flushNow bool) {
	at := rec.At.UTC()
	row := d.day(at)
	d.dirty = true
	row.Budget, row.Shed, row.Refuse, row.Exit, row.ExitEnabled = m.Budget, m.Shed, m.Refuse, m.Exit, m.ExitEnabled
	if d.fresh {
		d.fresh = false
		d.open = d.continues(row, at)
	}
	if rec.Transition {
		if tr == nil {
			return false
		}
		if tr.To == LevelExit && tr.From != LevelExit {
			row.ExitRungs++
			flushNow = true
		}
		if tr.To > tr.From {
			row.addRungEntry(tr.To, tr.Cause.Trigger, rec.Pressure)
			for l := tr.From + 1; l <= tr.To && int(l) < len(d.entered); l++ {
				d.entered[l] = tr.Cause.Trigger
			}
		}
		if tr.To >= LevelRefuse && tr.From < LevelRefuse {
			d.trigger = RefuseTrigger(tr.Cause)
			d.pastShed = m.Shed > 0 && rec.Combined() >= m.Shed
			d.pastShedKnown = rec.Combined() >= 0 && m.Shed > 0
			d.refuseWindow(row, at, tr.To)
		}
		return flushNow
	}
	row.Samples++
	if rec.Footprint >= 0 {
		row.MaxFootprint = max(row.MaxFootprint, rec.Footprint)
		if c := rec.Combined(); c >= 0 {
			row.MaxCombined = max(row.MaxCombined, c)
		}
		if m.Exit > 0 && rec.Footprint > m.Exit {
			row.OverExit++
		}
		if rec.Level < LevelRefuse && m.Budget > 0 && rec.Footprint > m.Budget {
			row.OverBudgetOutsideRefuse++
		}
	}
	switch {
	case rec.Level >= LevelRefuse:
		d.refuseWindow(row, at, rec.Level)
	case rec.Level == LevelShed:
		row.ShedMinutes++
		row.attributeShed(d.enteredBy(LevelShed), 1)
		d.open = false
	default:
		d.open = false
	}
	return flushNow
}

// continues reports whether the day's newest window ended within windowGap
// of at (a restart inside a refuse window), here or on the previous day.
func (d *DayLog) continues(row *DaySummary, at time.Time) bool {
	if w := lastWindow(row); w != nil {
		return windowEnd(w).After(at.Add(-windowGap)) && !windowEnd(w).After(at)
	}
	return false
}

func lastWindow(row *DaySummary) *RefuseWindow {
	if n := len(row.RefuseWindows); n > 0 {
		return &row.RefuseWindows[n-1]
	}
	return nil
}

func windowEnd(w *RefuseWindow) time.Time {
	t, _ := time.Parse(time.RFC3339, w.End)
	return t
}

// refuseWindow rolls a sample at rung l into the refuse windows and counts
// the minutes it adds against the trigger that entered l.
func (d *DayLog) refuseWindow(row *DaySummary, at time.Time, l Level) {
	before := row.RefuseMinutes
	d.window(row, at)
	row.attributeShed(d.enteredBy(l), row.RefuseMinutes-before)
}

// window extends the open refuse window with a sample at at, or opens one.
func (d *DayLog) window(row *DaySummary, at time.Time) {
	stamp := at.Format(time.RFC3339)
	if d.open {
		if w := lastWindow(row); w != nil && !windowEnd(w).After(at) {
			w.End = stamp
			d.recount(row)
			return
		}
	}
	trig := d.trigger
	switch {
	case trig != "":
	case d.open:
		trig = "continued from the previous day"
	default:
		trig = "unknown (entered before this log saw it)"
	}
	var past *bool
	if d.pastShedKnown {
		b := d.pastShed
		past = &b
	}
	d.trigger, d.pastShed, d.pastShedKnown = "", false, false
	if len(row.RefuseWindows) >= maxRefuseWindows { // bounded: fold into the last
		w := lastWindow(row)
		w.End = stamp
		d.recount(row)
		d.open = true
		return
	}
	row.RefuseWindows = append(row.RefuseWindows, RefuseWindow{Start: stamp, End: stamp, Trigger: trig, PastShed: past})
	d.open = true
	d.recount(row)
}

// recount derives every window's minutes from its span, and the day's total.
func (d *DayLog) recount(row *DaySummary) {
	total := 0
	for i := range row.RefuseWindows {
		w := &row.RefuseWindows[i]
		s, _ := time.Parse(time.RFC3339, w.Start)
		w.Minutes = int(windowEnd(w).Sub(s)/time.Minute) + 1
		total += w.Minutes
	}
	row.RefuseMinutes = total
}

// Flush rewrites the file when rows changed and a minute has passed since
// the last write (or force): temp file then rename, so a reader never sees
// half a file.
func (d *DayLog) Flush(now time.Time, force bool) error {
	if !d.dirty || (!force && !d.lastWrite.IsZero() && now.Sub(d.lastWrite) < DaysWriteEvery) {
		return nil
	}
	raw, err := json.Marshal(dayFile{Version: daysVersion, Days: d.rows})
	if err != nil {
		return err
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, d.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	d.dirty, d.lastWrite = false, now
	return nil
}

// WithRefuseSplit returns the rows with SelfRefuseMinutes and
// HostStarvedMinutes derived from their windows (the read side's view).
func WithRefuseSplit(rows []DaySummary) []DaySummary {
	out := make([]DaySummary, len(rows))
	for i, r := range rows {
		r.SelfRefuseMinutes, r.HostStarvedMinutes = r.RefuseSplit()
		out[i] = r
	}
	return out
}

// RTM1Verdict is RT-M1's reading over the last seven complete UTC days.
type RTM1Verdict struct {
	State     string   `json:"state"` // PASS, FAIL or INCOMPLETE
	Criterion string   `json:"criterion,omitempty"`
	Day       string   `json:"day,omitempty"`
	Detail    string   `json:"detail"`
	Days      []string `json:"days"` // the seven judged, oldest first
}

// JudgeRTM1 reads RT-M1 over the seven complete UTC days before now's:
// every sample at or under the exit mark, at or under budget outside refuse,
// self-caused refuse minutes at most RefuseMinutesPerDay a day and host-starved
// ones at most HostStarvedMinutesPerDay (RT-187), no exit rung, no unclean
// exit. A violation is FAIL whatever else is missing; missing, thin or
// ledger-blind days with no violation are INCOMPLETE, never PASS.
func JudgeRTM1(rows []DaySummary, now time.Time) RTM1Verdict {
	today := now.UTC().Truncate(24 * time.Hour)
	v := RTM1Verdict{}
	byDate := map[string]DaySummary{}
	for _, r := range rows {
		byDate[r.Date] = r
	}
	var gaps []string
	for i := 7; i >= 1; i-- {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		v.Days = append(v.Days, date)
		r, ok := byDate[date]
		selfMin, hostMin := r.RefuseSplit()
		fail := func(crit, why string) {
			if v.State != "FAIL" {
				v.State, v.Criterion, v.Day, v.Detail = "FAIL", crit, date, date+": "+why
			}
		}
		switch {
		case !ok:
			gaps = append(gaps, date+" (no row)")
			continue
		case r.OverExit > 0:
			fail("max footprint <= exit mark", fmt.Sprintf("%d sample(s) over the exit mark (max footprint %s)", r.OverExit, FormatBytes(r.MaxFootprint)))
		case r.OverBudgetOutsideRefuse > 0:
			fail("max footprint <= budget outside refuse windows", fmt.Sprintf("%d sample(s) over budget below the refuse rung", r.OverBudgetOutsideRefuse))
		case selfMin > RefuseMinutesPerDay:
			fail("self-caused refuse windows <= 15 min/day", fmt.Sprintf("%d self-caused refuse minutes (entered past fathom's shed mark)", selfMin))
		case hostMin > HostStarvedMinutesPerDay:
			fail("host-starved refuse minutes <= 60 min/day", fmt.Sprintf("%d host-starved refuse minutes (host trigger, fathom under its shed mark)", hostMin))
		case r.ExitRungs > 0:
			fail("zero exit rungs", fmt.Sprintf("%d exit rung(s)", r.ExitRungs))
		case r.UncleanExits > 0:
			fail("zero unclean exits", fmt.Sprintf("%d unclean exit(s)", r.UncleanExits))
		}
		switch {
		case r.Samples < DayCompleteSamples:
			gaps = append(gaps, fmt.Sprintf("%s (%d of 1440 minute samples)", date, r.Samples))
		case !r.ExitsKnown:
			gaps = append(gaps, date+" (lifecycle ledger does not cover it: exit rungs and unclean exits unknown)")
		}
	}
	switch {
	case v.State == "FAIL":
	case len(gaps) > 0:
		v.State, v.Detail = "INCOMPLETE", "not yet readable: "+strings.Join(gaps, "; ")
	default:
		v.State, v.Detail = "PASS", "seven complete UTC days, every criterion held"
	}
	return v
}
