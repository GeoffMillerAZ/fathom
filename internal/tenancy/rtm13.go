package tenancy

// rtm13.go — RT-M13's reading (RT-189, design §2.1, §13.1): the daemon
// spends at most 2% of a week in shed or above, and never enters a rung on
// a host trigger at normal pressure. The day log already rolls each ladder
// transition on the sampler tick; each rise is counted here by rung, the
// trigger that fired and the host pressure at entry, as additive fields of
// the day row (no goroutine, no table).

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	// RTM13ShedShare is RT-M13's ceiling on shed + refuse minutes: 2% of
	// the week's 10,080 minutes, 201.6.
	RTM13ShedShare = 0.02
	// maxRungEntries bounds a day's entry rows (rungs × triggers ×
	// pressures is 3 × 5 × 4 = 60 at most).
	maxRungEntries = 64
)

// RungEntry counts one day's entries into one rung by one trigger at one
// host pressure.
type RungEntry struct {
	Rung     string `json:"rung"`     // shed | refuse | exit
	Trigger  string `json:"trigger"`  // own_mark | pressure | swap_growth | swap_volume_floor | unknown
	Pressure string `json:"pressure"` // the host pressure at entry: normal | warn | critical | unknown
	Count    int    `json:"count"`
}

// hostTrigger reports whether the entry was a host trigger, not fathom's
// own mark.
func (e RungEntry) hostTrigger() bool {
	return e.Trigger == TriggerPressure.String() || e.Trigger == TriggerSwapGrowth.String() || e.Trigger == TriggerSwapVolume.String()
}

func rungEntryKey(e RungEntry) [3]int {
	rank := func(s string, names []string) int {
		if i := slices.Index(names, s); i >= 0 {
			return i
		}
		return len(names)
	}
	return [3]int{
		rank(e.Rung, []string{LevelShed.String(), LevelRefuse.String(), LevelExit.String()}),
		rank(e.Trigger, []string{TriggerOwnMark.String(), TriggerPressure.String(), TriggerSwapGrowth.String(), TriggerSwapVolume.String()}),
		rank(e.Pressure, []string{PressureNormal.String(), PressureWarn.String(), PressureCritical.String()}),
	}
}

// addRungEntry counts one rise into rung l, keeping the entries sorted by
// rung, trigger and pressure.
func (r *DaySummary) addRungEntry(l Level, t RungTrigger, p PressureLevel) {
	e := RungEntry{Rung: l.String(), Trigger: t.String(), Pressure: p.String(), Count: 1}
	for i := range r.RungEntries {
		if x := &r.RungEntries[i]; x.Rung == e.Rung && x.Trigger == e.Trigger && x.Pressure == e.Pressure {
			x.Count++
			return
		}
	}
	if len(r.RungEntries) >= maxRungEntries {
		return
	}
	r.RungEntries = append(r.RungEntries, e)
	slices.SortStableFunc(r.RungEntries, func(a, b RungEntry) int {
		ka, kb := rungEntryKey(a), rungEntryKey(b)
		return slices.Compare(ka[:], kb[:])
	})
}

// RTM13Verdict is RT-M13's reading over the last seven complete UTC days,
// in RTM1Verdict's state grammar, with the week's two measures.
type RTM13Verdict struct {
	State     string   `json:"state"` // PASS, FAIL or INCOMPLETE
	Criterion string   `json:"criterion,omitempty"`
	Day       string   `json:"day,omitempty"`
	Detail    string   `json:"detail"`
	Days      []string `json:"days"` // the seven judged, oldest first
	// ShedRefuseMinutes is the shed + refuse minutes of the rows read;
	// HostEntriesAtNormal the host-trigger rung entries at normal pressure
	// on the days that record entries.
	ShedRefuseMinutes   int `json:"shed_refuse_minutes"`
	HostEntriesAtNormal int `json:"host_entries_at_normal"`
}

// JudgeRTM13 reads RT-M13 over the seven complete UTC days before now's:
// shed + refuse minutes at most 2% of the week, and zero host-trigger rung
// entries at normal pressure. A violation is FAIL whatever else is missing
// (more days only add minutes); a missing or thin day, a day logged before
// rung entries were recorded, or an entry whose trigger or pressure is
// unknown leaves it INCOMPLETE, named, never PASS.
func JudgeRTM13(rows []DaySummary, now time.Time) RTM13Verdict {
	today := now.UTC().Truncate(24 * time.Hour)
	limit := RTM13ShedShare * 7 * 24 * 60
	v := RTM13Verdict{}
	var gaps []string
	fail := func(crit, date, why string) {
		if v.State != "FAIL" {
			v.State, v.Criterion, v.Day, v.Detail = "FAIL", crit, date, date+": "+why
		}
	}
	for i := 7; i >= 1; i-- {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		v.Days = append(v.Days, date)
		j := slices.IndexFunc(rows, func(r DaySummary) bool { return r.Date == date })
		if j < 0 {
			gaps = append(gaps, date+" (no row)")
			continue
		}
		r := rows[j]
		v.ShedRefuseMinutes += r.ShedMinutes + r.RefuseMinutes
		atNormal, unknown := 0, 0
		for _, e := range r.RungEntries {
			switch {
			case e.Trigger == TriggerUnknown.String() || (e.hostTrigger() && e.Pressure == PressureUnknown.String()):
				unknown += e.Count
			case e.hostTrigger() && e.Pressure == PressureNormal.String():
				atNormal += e.Count
			}
		}
		v.HostEntriesAtNormal += atNormal
		if atNormal > 0 {
			fail("zero host-trigger rung entries at normal pressure", date, fmt.Sprintf("%d host-trigger rung entr%s at normal pressure", atNormal, plural(atNormal, "y", "ies")))
		}
		if float64(v.ShedRefuseMinutes) > limit {
			fail(fmt.Sprintf("shed + refuse minutes <= %.1f a week (2%%)", limit), date, fmt.Sprintf("the week's shed + refuse minutes reach %d by this day", v.ShedRefuseMinutes))
		}
		switch {
		case r.Samples < DayCompleteSamples:
			gaps = append(gaps, fmt.Sprintf("%s (%d of 1440 minute samples)", date, r.Samples))
		case !r.RungEntriesKnown:
			gaps = append(gaps, date+" (logged before rung entries were recorded: what entered its rungs is unknown)")
		case unknown > 0:
			gaps = append(gaps, fmt.Sprintf("%s (%d rung entr%s with an unknown trigger or pressure)", date, unknown, plural(unknown, "y", "ies")))
		}
	}
	switch {
	case v.State == "FAIL":
	case len(gaps) > 0:
		v.State, v.Detail = "INCOMPLETE", "not yet readable: "+strings.Join(gaps, "; ")
	default:
		v.State, v.Detail = "PASS", fmt.Sprintf("seven complete UTC days: %d shed + refuse minutes <= %.1f, no host-trigger rung entry at normal pressure", v.ShedRefuseMinutes, limit)
	}
	return v
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
