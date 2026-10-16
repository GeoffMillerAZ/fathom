package main

// resources.go — `fathom resources [--json] [--since 24h] [--id ID]`
// (docs/design/responsible-tenant.md §13.2, RT-48). One report of what the
// daemon takes from this machine, rendered from files — the resource ring,
// the lifecycle ledger, the switch files, the diagnostic captures — so it
// works when the daemon is down, wedged or crash-looping; the live
// /statusz tenancy block is used when the daemon answers. Every fact names
// its instrument (P22); a daemon that does not answer is named, with the
// ring's last sample and its age, never nothing (I38).
//
// Exit: 0 within budget; 1 over budget, crash-looping, stopped by the
// breaker, or scratch daemons leaked; 2 a file is unreadable or there is
// no sample at all to judge.

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// resourcesSchemaVersion versions `resources --json` (the Operate panel
// and doctor read it).
const resourcesSchemaVersion = 1

// resourcesEnv is every OS effect `resources` has; tests inject each.
type resourcesEnv struct {
	home   string
	now    func() time.Time
	fetch  func(baseURL string) (*StatuszReport, error)              // the live /statusz; an error = unreachable
	census func() ([]censusRow, error)                               // nil: not taken (the primer)
	hooks  func(baseURL string) hookguard.Decision                   // nil: not read
	budget func() (tenancy.Budget, []tenancy.Note, error)            // when the ring has no header
	labels func() []string                                           // installed launchd labels
	killSw func(dir string, now time.Time) tenancy.KillSwitchVerdict // nil: readKillSwitch
	size   func(path string) int64                                   // nil: dirSize
	// hookLat reads hook-latency.jsonl since a time (RT-135); nil: not read.
	hookLat func(since time.Time) []hookLatencyRow
}

// resourcesReport is `resources --json`.
type resourcesReport struct {
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`

	Daemon      resourcesDaemon    `json:"daemon"`
	Tenancy     *governorSnapshot  `json:"tenancy,omitempty"`
	Budget      resourcesBudget    `json:"budget"`
	Footprint   resourcesFootprint `json:"footprint"`
	Lifecycle   resourcesLifecycle `json:"lifecycle"`
	Quarantined []string           `json:"quarantined,omitempty"`
	Host        resourcesHost      `json:"host"`
	Triggers    resourcesTriggers  `json:"host_triggers"`
	Hooks       resourcesHooks     `json:"hooks"`
	Scratch     resourcesScratch   `json:"scratch"`
	Disk        resourcesDisk      `json:"disk"`
	Diag        *diagView          `json:"diag,omitempty"`
	// Ladder is the window's ladder transitions, oldest first (RT-121).
	Ladder []resourcesLadderStep `json:"ladder,omitempty"`
	// Days and Verdict are `--days N` only (RT-185): the per-UTC-day
	// summaries and RT-M1's reading over the last seven complete days.
	Days       []tenancy.DaySummary `json:"days,omitempty"`
	Verdict    *tenancy.RTM1Verdict `json:"verdict,omitempty"`
	Unreadable []string             `json:"unreadable,omitempty"`
	Exit       int                  `json:"exit"`
	Reasons    []string             `json:"reasons,omitempty"`

	quarantine []tenancy.Quarantine // the ledger's, for the primer's line
}

type resourcesDaemon struct {
	ID          string `json:"id"`
	StateDir    string `json:"state_dir"`
	Managed     bool   `json:"managed"`
	URL         string `json:"url"`
	Reachable   bool   `json:"reachable"`
	Unreachable string `json:"unreachable,omitempty"` // why, when not reachable
	PID         int    `json:"pid,omitempty"`
	Record      string `json:"record,omitempty"`
}

type resourcesBudget struct {
	Budget      int64  `json:"budget"`
	Shed        int64  `json:"shed_mark"`
	Refuse      int64  `json:"refuse_mark"`
	Exit        int64  `json:"exit_mark"`
	ExitEnabled bool   `json:"exit_enabled"`
	Source      string `json:"source"` // the instrument: live daemon, ring header, or config
}

type resourcesFootprint struct {
	Now        int64  `json:"now"` // phys_footprint of the daemon itself; -1 unknown
	Children   int64  `json:"children"`
	SampleAt   string `json:"sample_at,omitempty"`
	SampleAge  string `json:"sample_age,omitempty"`
	Stale      bool   `json:"stale,omitempty"` // a ring sample older than tenancySampleFresh: shown, never judged as now
	Window     string `json:"window"`
	Samples    int    `json:"samples"`
	Max        int64  `json:"max"` // footprint + children, the ladder's measure
	MaxAt      string `json:"max_at,omitempty"`
	MaxLevel   string `json:"max_level,omitempty"`
	Sparkline  string `json:"sparkline,omitempty"`
	Torn       int    `json:"torn_slots,omitempty"`
	Transition int    `json:"transitions"`
	Ring       string `json:"ring"`
	// Hourly is the window's peak footprint (self + children) per hour,
	// oldest first; -1 is an hour with no sample (never 0). The Operate
	// Tenancy panel draws it against the budget and its marks (RT-121).
	Hourly []int64 `json:"hourly,omitempty"`
}

// resourcesLadderStep is one ladder transition in the window (RT-121).
type resourcesLadderStep struct {
	At    string `json:"at"`
	Level string `json:"level"`
}

type resourcesLifecycle struct {
	Ledger          string `json:"ledger"`
	Uptime          string `json:"uptime,omitempty"`
	StartsRecorded  bool   `json:"starts_recorded"`        // the ledger holds start rows: starts and unclean exits are counted
	NotRecorded     string `json:"not_recorded,omitempty"` // when not: what this ledger cannot tell (I38)
	Starts24h       int    `json:"starts_24h"`
	Unclean24h      int    `json:"unclean_24h"`
	OverBudget24h   int    `json:"over_budget_24h"`
	CountedExits30m int    `json:"counted_exits_30m"`
	CrashLooping    bool   `json:"crash_looping"`
	Switch          string `json:"switch"` // run | stopped: … | paused: …
	SwitchSince     string `json:"switch_since,omitempty"`
	BreakerStopped  bool   `json:"breaker_stopped"`
	LastExit        string `json:"last_exit,omitempty"`
}

type resourcesHost struct {
	Pressure       string   `json:"pressure"`
	SwapUsed       int64    `json:"swap_used"`
	SwapTotal      int64    `json:"swap_total"`
	StoreFree      int64    `json:"store_volume_free"`
	TmpFree        int64    `json:"tmp_volume_free"`
	SwapVolumeFree int64    `json:"swap_volume_free"`
	Unknowns       []string `json:"unknowns,omitempty"`
}

// resourcesTriggers are the host-side rung triggers ([tenancy.host],
// resolved) host_pressure judges against — never literals.
type resourcesTriggers struct {
	ShedOnPressure    string `json:"shed_on_pressure"`   // warn | critical | off
	RefuseOnPressure  string `json:"refuse_on_pressure"` // warn | critical | off
	MinSwapVolumeFree int64  `json:"min_swap_volume_free"`
	Source            string `json:"source"`

	shed, refuse tenancy.PressureLevel
}

// triggersOf projects resolved host triggers.
func triggersOf(h tenancy.HostTriggers, source string) resourcesTriggers {
	name := func(p tenancy.PressureLevel) string {
		if p == tenancy.PressureUnknown {
			return "off"
		}
		return p.String()
	}
	return resourcesTriggers{
		ShedOnPressure: name(h.ShedOnPressure), RefuseOnPressure: name(h.RefuseOnPressure),
		MinSwapVolumeFree: h.MinSwapVolumeFree, Source: source, shed: h.ShedOnPressure, refuse: h.RefuseOnPressure,
	}
}

type resourcesHooks struct {
	URL     string `json:"url"`
	Breaker string `json:"breaker"` // allow | breaker_open | busy | half_open | disabled | paused | not read
	Until   string `json:"until,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Latency string `json:"latency"`
}

type resourcesScratch struct {
	Taken     bool     `json:"taken"`
	Kept      int      `json:"kept"` // unmanaged, not ok, and kept by reap (its record lives under another HOME)
	Error     string   `json:"error,omitempty"`
	Unmanaged int      `json:"unmanaged"`
	Leaked    int      `json:"leaked"`
	Stale     int      `json:"stale_records"`
	Rows      []string `json:"rows,omitempty"`
}

type resourcesDisk struct {
	FathomHome      int64  `json:"fathom_home_bytes"`
	CallLog         string `json:"call_log,omitempty"`
	CallLogBytes    int64  `json:"call_log_bytes"`
	WrittenByDaemon int64  `json:"written_by_daemon_bytes"` // cumulative rusage of the sampled process; -1 unknown
	// Judgment is RT-162's 24 h verdict (outside explicit ingest against
	// the daily budget) as the tenancy block carries it.
	Judgment string `json:"judgment"`
}

// resourcesDiskJudgment is the disk line's RT-162 verdict: the block's,
// or unknown when there is no block.
func resourcesDiskJudgment(blk *governorSnapshot) string {
	if blk == nil {
		return "24 h disk-write judgment unknown: no tenancy block"
	}
	if blk.DiskWrites == "" {
		return "24 h disk-write judgment unknown: the block carries none"
	}
	return blk.DiskWrites
}

// hookLatencyNote: no hook invocation recorded a wall time in the window
// (hook-latency.jsonl, RT-135); disclosed, never a zero.
const hookLatencyNote = "p99 not measured: no hook latency recorded in 24 h (hook-latency.jsonl)"

// hookLatencyLine is the hooks line's latency: each channel's p99 against
// its deadline over the last 24 h, or the note.
func hookLatencyLine(read func(since time.Time) []hookLatencyRow, now time.Time) string {
	if read == nil {
		return hookLatencyNote
	}
	rows := read(now.Add(-hookBudgetWindow))
	if len(rows) == 0 {
		return hookLatencyNote
	}
	return hookLatencySummary(rows)
}

// lifecycleNotRecorded discloses a ledger with no start row: the build
// that wrote it records over-budget exits (and operator resets, adoptions)
// but not starts, and an unclean exit is only named at the next start
// (RT-37) — so starts, unclean exits and a crash loop of panics or jetsam
// kills are not counted, never rendered as zero (I38).
const lifecycleNotRecorded = "starts and unclean exits not recorded (the ledger has no start rows: the build that wrote it predates start rows) — crash-loop detection sees over-budget exits only"

// countedExitReasons are the exits the breaker counts (§2.3).
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var countedExitReasons = map[string]bool{"over_budget": true, "unclean": true, "port_held": true, "panic": true}

// lifecycleEntry is the lifecycle ledger's common shape.
type lifecycleEntry struct {
	Row    string `json:"row"`
	At     string `json:"at"`
	PID    int    `json:"pid,omitempty"`
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
	Code   int    `json:"code,omitempty"`
}

// resolveResourcesTarget picks the daemon `resources` reports: --id, else
// the record serving url, else the managed daemon — a managed record,
// else an installed label (or the default) whose state directory exists,
// else the default label.
func resolveResourcesTarget(env resourcesEnv, id, url string) (resourcesDaemon, *console.DaemonRecord) {
	dir := daemonRecordsDir(env.home)
	var labels []string
	if env.labels != nil {
		labels = env.labels()
	}
	labels = append(labels, defaultDaemonLabel)
	recs, _ := listDaemonRecords(env.home)
	// fromRecord reports the daemon a record names. A --lifecycle-id
	// daemon (RT-158) keeps its ledger, ring and diagnostics under
	// <lifecycle-id>/, not beside its addr-<port>-<pid> record (the live
	// reading's F6: doctor read the empty per-process directory and said
	// the build had no RT-37).
	fromRecord := func(p string, rec console.DaemonRecord, id string) (resourcesDaemon, *console.DaemonRecord) {
		d := resourcesDaemon{ID: id, StateDir: filepath.Join(dir, id), Record: p, PID: rec.PID, Managed: rec.Managed}
		if rec.LifecycleID != "" && !rec.Managed {
			d.ID, d.StateDir = rec.LifecycleID, filepath.Join(dir, rec.LifecycleID)
		}
		return d, &rec
	}
	pick := func(id string) (resourcesDaemon, *console.DaemonRecord) {
		d := resourcesDaemon{ID: id, StateDir: filepath.Join(dir, id)}
		_, _, scratch := scratchRecordID(id)
		d.Managed = !scratch
		p := filepath.Join(d.StateDir, "record.json")
		if rec, err := console.ReadDaemonRecord(p); err == nil {
			d.Record, d.PID, d.Managed = p, rec.PID, rec.Managed || !scratch
			return d, &rec
		}
		// A --lifecycle-id directory holds no record: the live record that
		// names it does, and it is never the managed daemon's.
		for _, r := range recs {
			if r.Rec.LifecycleID == id && !r.Rec.Managed {
				dd, rec := fromRecord(filepath.Join(dir, r.Rec.ID, "record.json"), r.Rec, r.Rec.ID)
				return dd, rec
			}
		}
		return d, nil
	}
	if id != "" {
		return pick(id)
	}
	if url != "" {
		want := hookguard.NormalizeURL(url)
		for _, r := range recs {
			if r.Rec.ID != "" && r.Rec.Addr != "" && hookguard.NormalizeURL(r.Rec.Addr) == want {
				if r.Rec.LifecycleID != "" && !r.Rec.Managed {
					return fromRecord(filepath.Join(dir, r.Rec.ID, "record.json"), r.Rec, r.Rec.ID)
				}
				return pick(r.Rec.ID)
			}
		}
	}
	for _, r := range recs {
		if r.Rec.Managed && r.Rec.ID != "" {
			return pick(r.Rec.ID)
		}
	}
	for _, l := range labels {
		if fi, err := os.Stat(filepath.Join(dir, l)); err == nil && fi.IsDir() {
			return pick(l)
		}
	}
	return pick(labels[0])
}

// buildResources assembles the report.
func buildResources(env resourcesEnv, id, url string, since time.Duration) resourcesReport {
	now := env.now()
	rep := resourcesReport{SchemaVersion: resourcesSchemaVersion, GeneratedAt: now.UTC().Format(time.RFC3339)}
	d, rec := resolveResourcesTarget(env, id, url)
	switch {
	case url != "":
		d.URL = url
	case rec != nil && rec.Addr != "":
		d.URL = "http://" + rec.Addr
	default:
		d.URL = defaultFathomURL()
	}
	d.URL = strings.TrimRight(d.URL, "/")
	unreadable := func(what string, err error) {
		rep.Unreadable = append(rep.Unreadable, fmt.Sprintf("%s: %v", what, err))
	}

	// The live block, when the daemon answers.
	var live *governorSnapshot
	if env.fetch != nil {
		st, err := env.fetch(d.URL)
		switch {
		case err != nil:
			d.Unreachable = err.Error()
		case st.Tenancy == nil:
			d.Reachable = true
			d.Unreachable = "the daemon answered but its /statusz carries no tenancy block (a build before the governor)"
		default:
			d.Reachable, live = true, st.Tenancy
		}
	} else {
		d.Unreachable = "not asked"
	}
	rep.Daemon = d

	// Quarantine, from the ledger (the daemon's own set when live).
	ledger := filepath.Join(d.StateDir, "lifecycle.jsonl")
	rows, lerr := readLifecycleEntries(ledger)
	if lerr != nil && !errors.Is(lerr, os.ErrNotExist) {
		unreadable("lifecycle ledger "+ledger, lerr)
	}
	if f, err := os.Open(ledger); err == nil {
		if qs, _, qerr := tenancy.QuarantinedServices(f); qerr == nil {
			rep.quarantine = qs
			for _, q := range qs {
				rep.Quarantined = append(rep.Quarantined, q.Attention())
			}
		}
		_ = f.Close()
	}
	if live != nil {
		rep.Quarantined = live.Quarantined
	}

	// The ring.
	ringPath := filepath.Join(d.StateDir, tenancy.RingFileName)
	rep.Footprint.Ring = ringPath
	rep.Footprint.Now, rep.Footprint.Children = tenancy.Unknown, tenancy.Unknown
	rd, rerr := tenancy.ReadRingFile(ringPath)
	haveRing := rerr == nil
	switch {
	case rerr == nil:
	case errors.Is(rerr, os.ErrNotExist):
		rep.Footprint.Ring = ringPath + " (absent: this daemon has not run under the governor here)"
	default:
		unreadable("resource ring "+ringPath, rerr)
	}
	blk := live
	if blk == nil && haveRing {
		blk, _ = tenancyFromRing(rd, rep.Quarantined)
	}
	rep.Tenancy = blk

	// Budget: the daemon's own marks (live, else the ring header), else
	// what config resolves to on this machine today. The host triggers
	// are config's (the ring header does not carry them), else defaults.
	var cfgBudget *tenancy.Budget
	var cfgHow string
	var cfgErr error
	if env.budget != nil {
		if b, notes, err := env.budget(); err == nil {
			cfgBudget, cfgHow = &b, "config"
			for _, n := range notes {
				if n.Key == "budget" {
					cfgHow = n.Derivation
				}
			}
		} else {
			cfgErr = err
		}
	}
	if cfgBudget != nil {
		rep.Triggers = triggersOf(cfgBudget.Host, "[tenancy.host] resolved on this machine")
	} else {
		def, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{})
		rep.Triggers = triggersOf(def.Host, "defaults ([tenancy] config not read)")
	}
	switch {
	case live != nil && live.Budget > 0:
		rep.Budget = resourcesBudget{live.Budget, live.Shed, live.Refuse, live.Exit, live.ExitEnabled, "live daemon (/statusz)"}
	case haveRing && rd.HasMeta:
		m := rd.Meta
		rep.Budget = resourcesBudget{m.Budget, m.Shed, m.Refuse, m.Exit, m.ExitEnabled, "the daemon's ring header (the marks it ran under)"}
	case cfgBudget != nil:
		b := cfgBudget
		rep.Budget = resourcesBudget{b.Budget, b.Shed, b.Refuse, b.Exit, b.ExitEnabled, "resolved from config on this machine (" + cfgHow + ")"}
	case cfgErr != nil:
		unreadable("[tenancy] config", cfgErr)
	}

	// Footprint: now, and the window's history.
	if blk != nil {
		rep.Footprint.Now, rep.Footprint.Children, rep.Footprint.SampleAt = blk.Footprint, blk.Children, blk.SampleAt
		if at, err := time.Parse(time.RFC3339, blk.SampleAt); err == nil {
			rep.Footprint.SampleAge = now.Sub(at).Round(time.Second).String()
			rep.Footprint.Stale = blk.Source == "ring" && now.Sub(at) > tenancySampleFresh
		}
		rep.Host = resourcesHost{
			Pressure: blk.Pressure, SwapUsed: blk.SwapUsed, SwapTotal: blk.SwapTotal, Unknowns: blk.Unknowns,
			StoreFree: tenancy.Unknown, TmpFree: tenancy.Unknown, SwapVolumeFree: tenancy.Unknown,
		}
	} else {
		rep.Host = resourcesHost{
			Pressure: tenancy.PressureUnknown.String(), SwapUsed: tenancy.Unknown, SwapTotal: tenancy.Unknown,
			StoreFree: tenancy.Unknown, TmpFree: tenancy.Unknown, SwapVolumeFree: tenancy.Unknown,
		}
	}
	rep.Footprint.Window = fmtWindow(since)
	if haveRing {
		w := rd.Since(now.Add(-since))
		rep.Footprint.Samples, rep.Footprint.Torn = len(w.Records), rd.Torn
		for _, r := range w.Records {
			if r.Transition {
				rep.Footprint.Transition++
				rep.Ladder = append(rep.Ladder, resourcesLadderStep{At: r.At.UTC().Format(time.RFC3339), Level: r.Level.String()})
			}
		}
		rep.Footprint.Hourly = hourlyPeaks(w.Records, now, since)
		if pk, ok := w.Peak(); ok {
			rep.Footprint.Max, rep.Footprint.MaxAt, rep.Footprint.MaxLevel = pk.Combined(), pk.At.UTC().Format(time.RFC3339), pk.Level.String()
		}
		rep.Footprint.Sparkline = sparkline(w.Records, now, since, max(rep.Budget.Budget, rep.Footprint.Max))
		if last, ok := rd.Last(); ok {
			rep.Host.StoreFree, rep.Host.TmpFree, rep.Host.SwapVolumeFree = last.StoreFree, last.TmpFree, last.SwapVolumeFree
		}
	}

	// Lifecycle.
	rep.Lifecycle = summarizeLifecycle(rows, ledger, now)
	if haveRing && rd.HasMeta && d.Reachable {
		rep.Lifecycle.Uptime = now.Sub(rd.Meta.StartedAt).Round(time.Minute).String()
	}
	kill := env.killSw
	if kill == nil {
		kill = readKillSwitch
	}
	if d.Managed {
		v := kill(daemonRecordsDir(env.home), now)
		if !v.Record.Since.IsZero() {
			rep.Lifecycle.SwitchSince = v.Record.Since.UTC().Format(time.RFC3339)
		}
		switch v.Action {
		case tenancy.ActionExitStay, tenancy.ActionExitRetry:
			rep.Lifecycle.Switch = "stopped: " + v.Message
			rep.Lifecycle.BreakerStopped = v.Record.Reason == "breaker"
		case tenancy.ActionSafeMode:
			rep.Lifecycle.Switch = "paused: " + v.Message
		default:
			rep.Lifecycle.Switch = "run"
		}
	} else {
		rep.Lifecycle.Switch = "run (an unmanaged daemon ignores the switch files)"
	}

	// Hooks.
	rep.Hooks = resourcesHooks{URL: hookguard.NormalizeURL(d.URL), Breaker: "not read", Latency: hookLatencyLine(env.hookLat, env.now())}
	if env.hooks != nil {
		dec := env.hooks(d.URL)
		rep.Hooks.Breaker, rep.Hooks.Reason = string(dec.Verdict), dec.Reason
		if !dec.Until.IsZero() {
			rep.Hooks.Until = dec.Until.UTC().Format(time.RFC3339)
		}
	}

	// Scratch daemons.
	if env.census != nil {
		rep.Scratch.Taken = true
		rows, err := env.census()
		if err != nil {
			rep.Scratch.Error = err.Error()
		}
		for _, r := range rows {
			switch {
			case r.Record == "stale":
				rep.Scratch.Stale++
			case !r.Managed:
				rep.Scratch.Unmanaged++
				// Leaked is exactly reap's kill set, so its remedy clears it.
				switch d := reapDisposition(r, 0, 0); {
				case d.kill:
					rep.Scratch.Leaked++
					rep.Scratch.Rows = append(rep.Scratch.Rows, fmt.Sprintf("pid %d %s %s (age %s)", r.PID, firstNonEmpty(r.Addr, "-"), r.Verdict, shortDur(r.Age)))
				case r.Verdict != "ok":
					rep.Scratch.Kept++
				}
			}
		}
	}

	// Disk.
	size := env.size
	if size == nil {
		size = dirSize
	}
	rep.Disk.FathomHome = size(fathomhome.Under(env.home))
	rep.Disk.WrittenByDaemon = tenancy.Unknown
	rep.Disk.Judgment = resourcesDiskJudgment(blk)
	if blk != nil {
		rep.Disk.WrittenByDaemon = blk.DiskWritten
	}
	if rec != nil && rec.CallLog != "" {
		rep.Disk.CallLog = rec.CallLog
		if fi, err := os.Stat(rec.CallLog); err == nil {
			rep.Disk.CallLogBytes = fi.Size()
		}
	}

	// Diagnostics: the live daemon's newest capture, else the newest on disk.
	if live != nil && live.LastDiag != nil {
		rep.Diag = live.LastDiag
	} else {
		rep.Diag = newestDiag(filepath.Join(d.StateDir, "diag"))
	}

	rep.Exit, rep.Reasons = resourcesVerdict(&rep, blk != nil)
	return rep
}

// resourcesVerdict decides the exit code and names every reason.
func resourcesVerdict(rep *resourcesReport, haveSample bool) (int, []string) {
	var reasons []string
	if len(rep.Unreadable) > 0 {
		for _, u := range rep.Unreadable {
			reasons = append(reasons, "unreadable: "+u)
		}
	}
	if !haveSample {
		reasons = append(reasons, "no sample: the daemon did not answer and no resource ring was readable — nothing to judge the budget against")
	}
	if len(reasons) > 0 {
		return 2, reasons
	}
	if t := rep.Tenancy; t != nil && !rep.Footprint.Stale {
		combined := t.Footprint
		if t.Children > 0 {
			combined += t.Children
		}
		if rep.Budget.Budget > 0 && combined > rep.Budget.Budget {
			reasons = append(reasons, fmt.Sprintf("over budget: footprint + children %s > budget %s", tenancy.FormatBytes(combined), tenancy.FormatBytes(rep.Budget.Budget)))
		}
		if t.Level == tenancy.LevelRefuse.String() || t.Level == tenancy.LevelExit.String() {
			reasons = append(reasons, "ladder at "+t.Level)
		}
	}
	if rep.Lifecycle.CrashLooping {
		reasons = append(reasons, fmt.Sprintf("crash-looping: %d counted exits in 30 min", rep.Lifecycle.CountedExits30m))
	}
	if rep.Lifecycle.BreakerStopped {
		reasons = append(reasons, "stopped by the breaker")
	}
	if rep.Scratch.Leaked > 0 {
		reasons = append(reasons, fmt.Sprintf("%d scratch daemon(s) leaked — fathom daemon reap --apply", rep.Scratch.Leaked))
	}
	if len(reasons) > 0 {
		return 1, reasons
	}
	return 0, nil
}

// readLifecycleEntries reads the ledger's rows, skipping lines that do not parse
// (a torn last line). A missing ledger returns the not-exist error.
func readLifecycleEntries(path string) ([]lifecycleEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []lifecycleEntry
	rd := bufio.NewReaderSize(f, 64*1024)
	for {
		line, rerr := readLedgerLine(rd)
		if len(line) > 0 {
			var r lifecycleEntry
			if json.Unmarshal(line, &r) == nil && r.Row != "" {
				rows = append(rows, r)
			}
		}
		if rerr == io.EOF {
			return rows, nil
		}
		if rerr != nil {
			return rows, rerr
		}
	}
}

// summarizeLifecycle counts the ledger's starts and exits (§2.3).
func summarizeLifecycle(rows []lifecycleEntry, ledger string, now time.Time) resourcesLifecycle {
	l := resourcesLifecycle{Ledger: ledger}
	for _, r := range rows {
		at, err := time.Parse(time.RFC3339Nano, r.At)
		if err != nil {
			continue
		}
		age := now.Sub(at)
		switch r.Row {
		case "start":
			l.StartsRecorded = true
			if age <= 24*time.Hour {
				l.Starts24h++
			}
		case "exit":
			if age <= 24*time.Hour {
				switch r.Reason {
				case "unclean":
					l.Unclean24h++
				case "over_budget":
					l.OverBudget24h++
				}
			}
			if countedExitReasons[r.Reason] && age <= 30*time.Minute {
				l.CountedExits30m++
			}
			l.LastExit = fmt.Sprintf("%s %s (code %d)", at.UTC().Format(time.RFC3339), r.Reason, r.Code)
			if r.Detail != "" {
				l.LastExit += ": " + r.Detail
			}
		}
	}
	l.CrashLooping = l.CountedExits30m >= 3
	if !l.StartsRecorded {
		l.NotRecorded = lifecycleNotRecorded
	}
	return l
}

// lifecycleStartsText is "N starts/24h · N unclean" when the ledger
// records starts, else the disclosure's short form.
func lifecycleStartsText(l resourcesLifecycle) string {
	if !l.StartsRecorded {
		return "starts/unclean not recorded"
	}
	return fmt.Sprintf("%d starts/24h · %d unclean", l.Starts24h, l.Unclean24h)
}

// fmtWindow renders a window without trailing zero units (24h, 90m, 1h30m).
func fmtWindow(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// newestDiag is the newest capture under dir (<UTC ts>-<trigger>).
func newestDiag(dir string) *diagView {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	n := names[len(names)-1]
	v := &diagView{Path: filepath.Join(dir, n)}
	if ts, trig, ok := strings.Cut(n, "-"); ok {
		v.Trigger = trig
		if at, err := time.Parse("20060102T150405Z", ts); err == nil {
			v.At = at.UTC().Format(time.RFC3339)
		}
	}
	return v
}

// sparkline is one glyph per hour of the window: the hour's peak
// footprint + children against scale; '·' for an hour with no sample.
// hourlyPeaks is the peak combined footprint per hour of the window
// (at most 48 hours), oldest first; -1 for an hour with no sample.
func hourlyPeaks(recs []tenancy.RingRecord, now time.Time, window time.Duration) []int64 {
	buckets := int(window / time.Hour)
	if buckets < 1 {
		return nil
	}
	if buckets > 48 {
		buckets = 48
	}
	start := now.Add(-time.Duration(buckets) * time.Hour)
	peak := make([]int64, buckets)
	for i := range peak {
		peak[i] = -1
	}
	for _, r := range recs {
		i := int(r.At.Sub(start) / time.Hour)
		if i < 0 || i >= buckets {
			continue
		}
		if c := r.Combined(); c > peak[i] {
			peak[i] = c
		}
	}
	return peak
}

func sparkline(recs []tenancy.RingRecord, now time.Time, window time.Duration, scale int64) string {
	const glyphs = "▁▂▃▄▅▆▇█"
	g := []rune(glyphs)
	buckets := int(window / time.Hour)
	if buckets < 1 || scale <= 0 {
		return ""
	}
	if buckets > 48 {
		buckets = 48
	}
	start := now.Add(-time.Duration(buckets) * time.Hour)
	peak := make([]int64, buckets)
	for i := range peak {
		peak[i] = -1
	}
	for _, r := range recs {
		i := int(r.At.Sub(start) / time.Hour)
		if i < 0 || i >= buckets {
			continue
		}
		if c := r.Combined(); c > peak[i] {
			peak[i] = c
		}
	}
	var b strings.Builder
	for _, p := range peak {
		if p < 0 {
			b.WriteRune('·')
			continue
		}
		idx := int(p * int64(len(g)) / (scale + 1))
		if idx >= len(g) {
			idx = len(g) - 1
		}
		b.WriteRune(g[idx])
	}
	return b.String()
}

// renderResources writes the human report (§13.2).
func renderResources(w io.Writer, rep resourcesReport, now time.Time) {
	fb := func(n int64) string {
		if n < 0 {
			return "unknown"
		}
		return tenancy.FormatBytes(n)
	}
	loc := now.Location()
	when := func(s string) string {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.In(loc).Format("01-02 15:04")
		}
		return s
	}
	d := rep.Daemon
	kind := "unmanaged"
	if d.Managed {
		kind = "managed"
	}
	fmt.Fprintf(w, "fathom resources — %s (%s)\n", d.ID, kind)
	src := "no sample"
	if t := rep.Tenancy; t != nil {
		src = t.Source
	}
	if d.Reachable && src == "live" {
		fmt.Fprintf(w, "daemon     answering at %s (pid %d) · figures from its /statusz\n", d.URL, d.PID)
	} else {
		fmt.Fprintf(w, "daemon     NOT answering at %s (%s)", d.URL, firstNonEmpty(d.Unreachable, "no tenancy block"))
		if rep.Tenancy != nil {
			fmt.Fprintf(w, " — showing the ring's last sample, %s old", firstNonEmpty(rep.Footprint.SampleAge, "?"))
		}
		fmt.Fprintln(w)
	}
	b := rep.Budget
	level, mode := "unknown", "unknown"
	if t := rep.Tenancy; t != nil {
		level, mode = t.Level, t.Mode
		if t.Since != "" {
			level += " since " + when(t.Since)
		}
		if t.LadderOff {
			level += " (ladder off: footprint unknown)"
		}
	}
	exitMark := fb(b.Exit)
	if !b.ExitEnabled {
		exitMark += " (exit rung disabled)"
	}
	if b.Budget > 0 {
		fmt.Fprintf(w, "state      %s · mode %s · budget %s · shed %s · refuse %s · exit mark %s [%s]\n", level, mode, fb(b.Budget), fb(b.Shed), fb(b.Refuse), exitMark, b.Source)
	} else {
		fmt.Fprintf(w, "state      %s · mode %s · budget unknown\n", level, mode)
	}
	f := rep.Footprint
	ch := ""
	if f.Children >= 0 {
		ch = fmt.Sprintf(" (+%s children)", fb(f.Children))
	}
	fmt.Fprintf(w, "footprint  %s now%s [phys_footprint]", fb(f.Now), ch)
	if f.Samples > 0 {
		fmt.Fprintf(w, " · %s max %s (%s, %s) · %s", f.Window, fb(f.Max), when(f.MaxAt), f.MaxLevel, f.Sparkline)
	}
	fmt.Fprintf(w, " [ring: %d samples, %d transitions", f.Samples, f.Transition)
	if f.Torn > 0 {
		fmt.Fprintf(w, ", %d torn slots skipped", f.Torn)
	}
	fmt.Fprintln(w, "]")
	l := rep.Lifecycle
	up := ""
	if l.Uptime != "" {
		up = "up " + l.Uptime + " · "
	}
	fmt.Fprintf(w, "lifecycle  %s%s · %d over-budget exits/24h · %d counted exits/30m · switch %s · %d quarantined\n",
		up, lifecycleStartsText(l), l.OverBudget24h, l.CountedExits30m, l.Switch, len(rep.Quarantined))
	if l.NotRecorded != "" {
		fmt.Fprintf(w, "           %s\n", l.NotRecorded)
	}
	if l.LastExit != "" {
		fmt.Fprintf(w, "           last exit %s\n", l.LastExit)
	}
	for _, q := range rep.Quarantined {
		fmt.Fprintf(w, "quarantine %s\n", q)
	}
	h := rep.Host
	fmt.Fprintf(w, "host       pressure %s · swap %s/%s · store volume %s free · swap volume %s free\n", h.Pressure, fb(h.SwapUsed), fb(h.SwapTotal), fb(h.StoreFree), fb(h.SwapVolumeFree))
	if len(h.Unknowns) > 0 {
		fmt.Fprintf(w, "           unknown: %s\n", strings.Join(h.Unknowns, "; "))
	}
	if t := rep.Tenancy; t != nil {
		var parts []string
		for _, c := range tenancy.Classes {
			parts = append(parts, fmt.Sprintf("%s %d", c, t.InFlightByClass[c.String()]))
		}
		line := strings.Join(parts, " · ")
		if t.Oldest != nil {
			line += fmt.Sprintf(" · oldest %s (%s) %s", t.Oldest.Tool, t.Oldest.Class, time.Duration(t.Oldest.AgeSec*float64(time.Second)).Round(time.Second))
		}
		fmt.Fprintf(w, "in-flight  %s · abandoned by governor %d · refused: %s\n", line, t.Abandoned, t.Refusals)
	} else {
		fmt.Fprintln(w, "in-flight  unknown (no sample)")
	}
	hk := rep.Hooks
	hl := "breaker " + hk.Breaker
	if hk.Until != "" {
		hl += " until " + when(hk.Until)
	}
	fmt.Fprintf(w, "hooks      %s for %s · %s\n", hl, hk.URL, hk.Latency)
	s := rep.Scratch
	switch {
	case !s.Taken:
		fmt.Fprintln(w, "scratch    not taken")
	case s.Error != "":
		fmt.Fprintf(w, "scratch    census failed: %s\n", s.Error)
	default:
		fmt.Fprintf(w, "scratch    %d unmanaged daemon(s), %d leaked, %d stale record(s) [ps + daemon records]", s.Unmanaged, s.Leaked, s.Stale)
		if s.Kept > 0 {
			fmt.Fprintf(w, " · %d kept by reap (no record under this HOME, parent alive)", s.Kept)
		}
		fmt.Fprintln(w)
		for _, r := range s.Rows {
			fmt.Fprintf(w, "           %s\n", r)
		}
	}
	dk := rep.Disk
	cl := ""
	if dk.CallLog != "" {
		cl = fmt.Sprintf(" · call log %s", fb(dk.CallLogBytes))
	}
	fmt.Fprintf(w, "disk       ~/.fathom %s%s · written by the daemon %s since its start [rusage]\n", fb(dk.FathomHome), cl, fb(dk.WrittenByDaemon))
	fmt.Fprintf(w, "           %s\n", dk.Judgment)
	if rep.Diag != nil {
		fmt.Fprintf(w, "diag       last capture %s %s → %s\n", when(rep.Diag.At), rep.Diag.Trigger, rep.Diag.Path)
	} else {
		fmt.Fprintln(w, "diag       no capture")
	}
	for _, u := range rep.Unreadable {
		fmt.Fprintf(w, "UNREADABLE %s\n", u)
	}
	switch {
	case len(rep.Reasons) == 0 && rep.Footprint.Stale:
		fmt.Fprintf(w, "verdict    no current sample to judge the budget against (the ring's last sample is %s old: the daemon is not sampling) — nothing else flagged (exit 0)\n", rep.Footprint.SampleAge)
	case len(rep.Reasons) == 0:
		fmt.Fprintln(w, "verdict    within budget (exit 0)")
	default:
		fmt.Fprintf(w, "verdict    exit %d: %s\n", rep.Exit, strings.Join(rep.Reasons, "; "))
	}
}

// runResources is the verb over an injected environment.
func runResources(args []string, stdout, stderr io.Writer, env resourcesEnv) int {
	fs := newFlagSet("resources", stderr)
	jsonOut := fs.Bool("json", false, "emit the report as JSON (schema_version 1)")
	since := fs.Duration("since", 24*time.Hour, "history window read from the resource ring (the ring holds 24 h)")
	daysN := fs.Int("days", 0, "also print the last N (1..8) per-UTC-day summaries and the RT-M1 verdict over the last 7 complete days")
	id := fs.String("id", "", "daemon state directory id under ~/.fathom/daemon (default: the managed daemon)")
	url := fs.String("url", "", "the daemon's URL (default: from its record, else $FATHOM_HTTP or the default address)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *since <= 0 {
		fmt.Fprintln(stderr, "resources: --since must be positive")
		return 2
	}
	if daysSet(fs) && (*daysN < 1 || *daysN > tenancy.DaysKeep) {
		fmt.Fprintf(stderr, "resources: --days must be 1..%d\n", tenancy.DaysKeep)
		return 2
	}
	rep := buildResources(env, *id, *url, *since)
	if *daysN > 0 {
		attachDays(&rep, env.now(), *daysN)
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		renderResources(stdout, rep, env.now())
		renderDays(stdout, rep)
	}
	return rep.Exit
}

// daysSet reports whether --days was given (0 is then an error, not "off").
func daysSet(fs *flag.FlagSet) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == "days" })
	return set
}

// attachDays reads the day log beside the ring and merges the lifecycle
// ledger's exit rungs and unclean exits per day at read time (the ledger
// keeps two generations of 1 MiB, and an exit row is rare; a day the ledger
// does not reach back to is marked unknown, never zero — I38). The verdict
// is RT-M1's over the seven complete UTC days before today; the rows shown
// are the last n, today's partial row included.
func attachDays(rep *resourcesReport, now time.Time, n int) {
	dir := rep.Daemon.StateDir
	rows, err := tenancy.ReadDayLog(filepath.Join(dir, tenancy.DaysFileName))
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		rows = nil
	default:
		rep.Unreadable = append(rep.Unreadable, fmt.Sprintf("day summaries %s: %v", filepath.Join(dir, tenancy.DaysFileName), err))
		rep.Exit, rep.Reasons = 2, append(rep.Reasons, "unreadable: "+rep.Unreadable[len(rep.Unreadable)-1])
		rows = nil
	}
	// A generation that exists but cannot be read whole leaves every day's
	// exits unknown (I38): partial rows could read a day as exit-free. The
	// current generation's error is already in Unreadable (buildResources).
	ledger := filepath.Join(dir, "lifecycle.jsonl")
	old, oerr := readLifecycleEntries(ledger + ".1")
	cur, cerr := readLifecycleEntries(ledger)
	if oerr != nil && !errors.Is(oerr, os.ErrNotExist) {
		rep.Unreadable = append(rep.Unreadable, fmt.Sprintf("lifecycle ledger %s: %v", ledger+".1", oerr))
		rep.Exit, rep.Reasons = 2, append(rep.Reasons, "unreadable: "+rep.Unreadable[len(rep.Unreadable)-1])
	}
	var entries []lifecycleEntry
	if (oerr == nil || errors.Is(oerr, os.ErrNotExist)) && (cerr == nil || errors.Is(cerr, os.ErrNotExist)) {
		entries = append(old, cur...)
	}
	merged := mergeLedgerIntoDays(rows, entries, oerr == nil)
	v := tenancy.JudgeRTM1(merged, now)
	rep.Verdict = &v
	if len(merged) > n {
		merged = merged[len(merged)-n:]
	}
	rep.Days = tenancy.WithRefuseSplit(merged)
}

// mergeLedgerIntoDays raises each row's ExitRungs (the larger of the
// sampler's and the ledger's over-budget exits) and UncleanExits from the
// ledger, and sets ExitsKnown for the days the ledger reaches back to. A
// rotated ledger (rotated: the .1 generation exists) dropped the rows
// before its first, so the day holding that first row is known only when
// the row opens the day; an unrotated ledger holds every row ever written.
func mergeLedgerIntoDays(days []tenancy.DaySummary, ledger []lifecycleEntry, rotated bool) []tenancy.DaySummary {
	var first time.Time
	over, unclean := map[string]int{}, map[string]int{}
	for _, r := range ledger {
		at, err := time.Parse(time.RFC3339Nano, r.At)
		if err != nil {
			continue
		}
		if first.IsZero() || at.Before(first) {
			first = at
		}
		date := at.UTC().Format("2006-01-02")
		if r.Row == "exit" {
			switch r.Reason {
			case "over_budget":
				over[date]++
			case "unclean":
				unclean[date]++
			}
		}
	}
	out := make([]tenancy.DaySummary, len(days))
	copy(out, days)
	for i := range out {
		d := &out[i]
		start, err := time.Parse("2006-01-02", d.Date)
		if err != nil || first.IsZero() {
			continue
		}
		reach := start.Add(24 * time.Hour)
		if rotated {
			reach = start
		}
		d.ExitsKnown = !first.After(reach)
		d.ExitRungs = max(d.ExitRungs, over[d.Date])
		d.UncleanExits = max(d.UncleanExits, unclean[d.Date])
	}
	return out
}

// renderDays prints the `--days` table and RT-M1's one verdict line.
func renderDays(w io.Writer, rep resourcesReport) {
	if rep.Verdict == nil {
		return
	}
	fb := func(n int64) string {
		if n <= 0 {
			return "-"
		}
		return tenancy.FormatBytes(n)
	}
	fmt.Fprintln(w, "days       [resource day log; exits from the lifecycle ledger]")
	fmt.Fprintf(w, "           %-10s %7s %9s %9s %6s %6s %5s %12s %5s %4s %7s\n", "date (UTC)", "samples", "max fp", "max +kids", "shed m", "refuse", "self", "host-starved", "exit", "unclean", "ledger")
	for _, d := range rep.Days {
		led := "covers"
		if !d.ExitsKnown {
			led = "unknown"
		}
		fmt.Fprintf(w, "           %-10s %7d %9s %9s %6d %6d %5d %12d %5d %4d %7s\n", d.Date, d.Samples, fb(d.MaxFootprint), fb(d.MaxCombined), d.ShedMinutes, d.RefuseMinutes, d.SelfRefuseMinutes, d.HostStarvedMinutes, d.ExitRungs, d.UncleanExits, led)
		for _, rw := range d.RefuseWindows {
			fmt.Fprintf(w, "             refuse %s %d min: %s\n", rw.Start, rw.Minutes, rw.Trigger)
		}
	}
	fmt.Fprintf(w, "RT-M1 verdict  %s: %s\n", rep.Verdict.State, rep.Verdict.Detail)
}

// fetchStatuszReport GETs the daemon's /statusz JSON with a short deadline.
func fetchStatuszReport(baseURL string) (*StatuszReport, error) {
	hc := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/statusz?format=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /statusz: HTTP %d", resp.StatusCode)
	}
	var r StatuszReport
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&r); err != nil {
		return nil, fmt.Errorf("GET /statusz: %w", err)
	}
	return &r, nil
}

// liveResourcesEnv is the real machine.
func liveResourcesEnv() resourcesEnv {
	home, _ := os.UserHomeDir()
	return resourcesEnv{
		home:   home,
		now:    time.Now,
		fetch:  fetchStatuszReport,
		census: liveCensus,
		hooks:  func(u string) hookguard.Decision { return hookguard.BreakerAllows(&hookguard.Guard{}, u) },
		budget: resolveLocalBudget,
		labels: installedAgentLabels,
		hookLat: func(since time.Time) []hookLatencyRow {
			return readHookLatency(hookLatencyPath(), since)
		},
	}
}

// doctorResourcesReport is the resources projection doctor's tenancy
// rows read (RT-49, RT-50): the daemon at baseURL, its files, the census.
func doctorResourcesReport(baseURL string) resourcesReport {
	return buildResources(liveResourcesEnv(), "", baseURL, 24*time.Hour)
}

// resolveLocalBudget resolves the managed budget from this machine's
// config, the way serve-http does.
func resolveLocalBudget() (tenancy.Budget, []tenancy.Note, error) {
	loaded, err := config.Load(config.DefaultUserPath(), "")
	if err != nil {
		return tenancy.Budget{}, nil, err
	}
	cfg, _, err := config.ResolveTenancyConfig(loaded, os.Getenv)
	if err != nil {
		return tenancy.Budget{}, nil, err
	}
	b, notes := tenancy.ResolveBudget(cfg, tenancy.HostFacts{RAM: hostRAM(), NumCPU: runtime.NumCPU(), Managed: true})
	return b, notes, nil
}

// resourcesEnvNow is the environment `fathom resources` and the Operate
// Tenancy panel read (tests pin one for both, so the panel's section and
// the verb's JSON are compared over the same inputs).
var resourcesEnvNow = liveResourcesEnv

// cmdResources implements `fathom resources`.
func cmdResources(args []string, stdout, stderr io.Writer) int {
	return runResources(args, stdout, stderr, resourcesEnvNow())
}

// resourcesRunner runs `fathom resources` (registered below; EW-45).
type resourcesRunner struct{}

func (resourcesRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdResources(args, stdout, stderr)
}

func init() {
	declareVerb(verb{
		Name: "resources", Group: "operate", Order: 40, Run: resourcesRunner{},
		Summary: "What the daemon takes from this machine, from files (works when it is down): budget, footprint history, lifecycle, quarantine, host, in-flight, hooks, scratch daemons, disk, last capture; exit 0 within budget, 1 over/crash-looping/stopped/leaked, 2 unreadable [--json] [--since 24h] [--id ID]",
	})
}
