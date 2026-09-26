package console

// operate.go — OC-44 (vision 35 §3 System, design §3 "Operate"): the
// daemon and the traffic it answers, from the resolved call log.
//
// The survey: "the 7-day unscoped p95 is 25 s with an 11.7% error rate; no
// page shows latency or errors", and the tool bars were "dominated by
// plumbing". OperateView gives every surface its calls, p50, p95 and error
// rate per day — a day with no call-log rows at all is absent (the daemon
// was down, or logging was off), never a zero — and labels every tool by
// the caller classes that use it (callers.go). The console's own reads are
// logged but never counted.

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// DaemonFacts is what the serving process knows about itself.
type DaemonFacts struct {
	Version   string
	Commit    string
	Dirty     bool
	Addr      string
	StartedAt time.Time
	// RSSBytes is the resident set (phys footprint on macOS); 0 with
	// MemNote set when it could not be read.
	RSSBytes   uint64
	MemNote    string
	HeapBytes  uint64
	Goroutines int
}

// OperateInputs is what OperateView reads.
type OperateInputs struct {
	Daemon        DaemonFacts
	Calls         []CallRow
	CallLog       ResolvedCallLog
	CallLogReason string // non-empty: the log could not be read
	CallLogError  bool   // the log exists and failed (an error, not an absence)
	Now           time.Time
	Days          int // the per-day matrix's span (default 14)
	WindowDays    int // the tool and caller window (default 7)
	TopTools      int // tools listed (default 10)
}

// DaemonPanel is the daemon's own facts with their truth states.
type DaemonPanel struct {
	Version    string     `json:"version"`
	Commit     string     `json:"commit"`
	Dirty      bool       `json:"dirty"`
	Addr       string     `json:"addr,omitempty"`
	StartedAt  string     `json:"started_at,omitempty"`
	UptimeSec  Value[int] `json:"uptime_seconds"`
	MemoryMB   Value[int] `json:"memory_mb"`
	HeapMB     Value[int] `json:"heap_mb"`
	Goroutines Value[int] `json:"goroutines"`
}

// SurfaceDay is one surface's traffic on one day: calls (absent when the
// log holds no row at all that day), p50/p95 latency and error rate (nil
// when the surface answered nothing that day).
type SurfaceDay struct {
	Day       string     `json:"day"`
	Surface   string     `json:"surface"`
	Calls     Value[int] `json:"calls"`
	P50MS     *float64   `json:"p50_ms,omitempty"`
	P95MS     *float64   `json:"p95_ms,omitempty"`
	ErrorRate *float64   `json:"error_rate_percent,omitempty"`
}

// DayTotal is one day's calls across every surface.
type DayTotal struct {
	Day   string     `json:"day"`
	Calls Value[int] `json:"calls"`
}

// ClassTraffic is one caller class's traffic in the window.
type ClassTraffic struct {
	Class     CallerClass `json:"class"`
	Calls     int         `json:"calls"`
	Share     float64     `json:"share"`
	P50MS     float64     `json:"p50_ms"`
	P95MS     float64     `json:"p95_ms"`
	ErrorRate float64     `json:"error_rate_percent"`
}

// OperateModel is the Operate page's traffic projection.
type OperateModel struct {
	Daemon      DaemonPanel     `json:"daemon"`
	CallLog     ResolvedCallLog `json:"call_log"`
	Days        []string        `json:"days"`
	Surfaces    []string        `json:"surfaces"`
	Traffic     []SurfaceDay    `json:"traffic"` // day-major, surfaces in Surfaces order
	DayTotals   []DayTotal      `json:"day_totals"`
	WindowFrom  string          `json:"window_from"`
	WindowTo    string          `json:"window_to"`
	WindowCalls Value[int]      `json:"window_calls"`
	Classes     []ClassTraffic  `json:"classes"`
	Tools       []ToolLoad      `json:"tools"`
	ToolsCalled int             `json:"tools_called"`
}

// SurfaceLabel names a surface the way `metrics traffic` does.
func SurfaceLabel(s string) string {
	if s == "" {
		return "(unscoped)"
	}
	return s
}

// nearestRank is the nearest-rank percentile of a sorted slice, the rule
// `metrics traffic` uses (ceil(p·n), 1-indexed).
func nearestRank(sorted []float64, p float64) float64 {
	n := len(sorted)
	idx := int(math.Ceil(p*float64(n))) - 1
	return sorted[min(max(idx, 0), n-1)]
}

type latAcc struct {
	lat    []float64
	errors int
}

func (a *latAcc) stats() (p50, p95, errRate float64) {
	s := append([]float64(nil), a.lat...)
	sort.Float64s(s)
	return nearestRank(s, 0.50), nearestRank(s, 0.95), float64(a.errors) * 100 / float64(len(s))
}

func inAxis(axis map[string]int, day string) bool { _, ok := axis[day]; return ok }

// OperateView projects the daemon and its traffic.
func OperateView(in OperateInputs) OperateModel {
	if in.Days <= 0 {
		in.Days = 14
	}
	if in.WindowDays <= 0 {
		in.WindowDays = 7
	}
	if in.TopTools <= 0 {
		in.TopTools = 10
	}
	now := in.Now.UTC()
	m := OperateModel{CallLog: in.CallLog}

	// The daemon.
	d := in.Daemon
	m.Daemon = DaemonPanel{Version: d.Version, Commit: d.Commit, Dirty: d.Dirty, Addr: d.Addr}
	const self = "this daemon process"
	if d.StartedAt.IsZero() {
		m.Daemon.UptimeSec = Absent[int]("the process did not record its start").WithSource(self)
	} else {
		m.Daemon.StartedAt = d.StartedAt.UTC().Format(time.RFC3339)
		m.Daemon.UptimeSec = Measured(int(now.Sub(d.StartedAt).Seconds()), self)
	}
	if d.RSSBytes == 0 {
		m.Daemon.MemoryMB = Absent[int](nonEmpty(d.MemNote, "the resident set could not be read on this platform")).WithSource(self)
	} else {
		m.Daemon.MemoryMB = Present(int(d.RSSBytes>>20), self).WithDenominator("resident set (phys footprint on macOS), one reading at render")
	}
	if d.HeapBytes == 0 {
		m.Daemon.HeapMB = Absent[int]("the Go heap was not read").WithSource(self)
	} else {
		m.Daemon.HeapMB = Present(int(d.HeapBytes>>20), self).WithDenominator("Go heap in use")
	}
	m.Daemon.Goroutines = Measured(d.Goroutines, self)

	// The day axis: Days calendar days ending today, oldest first.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayIndex := map[string]int{}
	for i := in.Days - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		dayIndex[day] = len(m.Days)
		m.Days = append(m.Days, day)
	}
	windowFrom := now.AddDate(0, 0, -in.WindowDays)
	m.WindowFrom, m.WindowTo = windowFrom.Format(time.RFC3339), now.Format(time.RFC3339)
	source := in.CallLog.Path + " (rule " + string(in.CallLog.Rule) + ")"
	if in.CallLogReason != "" {
		reason := in.CallLogReason
		for _, day := range m.Days {
			v := Absent[int](reason)
			if in.CallLogError {
				v = Errored[int](reason)
			}
			m.DayTotals = append(m.DayTotals, DayTotal{Day: day, Calls: v.WithSource(source)})
		}
		if in.CallLogError {
			m.WindowCalls = Errored[int](reason).WithSource(source)
		} else {
			m.WindowCalls = Absent[int](reason).WithSource(source)
		}
		return m
	}

	cells := map[string]map[string]*latAcc{} // day → surface → latencies
	surfaces := map[string]bool{}
	classes := map[CallerClass]*latAcc{}
	var window []CallRow
	for _, r := range in.Calls {
		class := ClassifyCaller(r.Tool, r.Surface, r.Origin)
		if class == ClassConsole {
			continue
		}
		ts, ok := parseAt(r.TS)
		if !ok || ts.After(now) {
			continue
		}
		surface := SurfaceLabel(r.Surface)
		if day := ts.Format("2006-01-02"); inAxis(dayIndex, day) {
			if cells[day] == nil {
				cells[day] = map[string]*latAcc{}
			}
			a := cells[day][surface]
			if a == nil {
				a = &latAcc{}
				cells[day][surface] = a
			}
			a.lat = append(a.lat, r.LatencyMS)
			if r.Error != "" {
				a.errors++
			}
			surfaces[surface] = true
		}
		if ts.Before(windowFrom) {
			continue
		}
		window = append(window, r)
		a := classes[class]
		if a == nil {
			a = &latAcc{}
			classes[class] = a
		}
		a.lat = append(a.lat, r.LatencyMS)
		if r.Error != "" {
			a.errors++
		}
	}
	m.Surfaces = sortedKeys(surfaces)
	for _, day := range m.Days {
		total := 0
		for _, s := range m.Surfaces {
			if a := cells[day][s]; a != nil {
				total += len(a.lat)
			}
		}
		if total == 0 {
			reason := "no call-log rows this day — the daemon was down or not logging; absent, not zero"
			m.DayTotals = append(m.DayTotals, DayTotal{Day: day, Calls: Absent[int](reason).WithSource(source)})
			for _, s := range m.Surfaces {
				m.Traffic = append(m.Traffic, SurfaceDay{Day: day, Surface: s, Calls: Absent[int](reason).WithSource(source)})
			}
			continue
		}
		m.DayTotals = append(m.DayTotals, DayTotal{Day: day, Calls: Measured(total, source)})
		for _, s := range m.Surfaces {
			sd := SurfaceDay{Day: day, Surface: s}
			a := cells[day][s]
			if a == nil {
				sd.Calls = Measured(0, source).WithDenominator("the log has rows this day; none on this surface")
			} else {
				p50, p95, er := a.stats()
				sd.Calls = Measured(len(a.lat), source)
				sd.P50MS, sd.P95MS, sd.ErrorRate = &p50, &p95, &er
			}
			m.Traffic = append(m.Traffic, sd)
		}
	}
	m.WindowCalls = Measured(len(window), source).WithDenominator(fmt.Sprintf("usage calls in the last %d days (console reads excluded)", in.WindowDays))
	for _, c := range CallerClasses {
		a := classes[c]
		if a == nil {
			continue
		}
		p50, p95, er := a.stats()
		m.Classes = append(m.Classes, ClassTraffic{Class: c, Calls: len(a.lat), Share: float64(len(a.lat)) / float64(len(window)), P50MS: p50, P95MS: p95, ErrorRate: er})
	}
	loads := ToolLoads(window)
	m.ToolsCalled = len(loads)
	if len(loads) > in.TopTools {
		loads = loads[:in.TopTools]
	}
	m.Tools = loads
	return m
}
