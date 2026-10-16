package main

// doctor_retention.go — doctor's hook_budget and disk_retention rows
// (docs/design/responsible-tenant.md §11.3, §12.1; RT-135).
//
// hook_budget: each hook invocation's wall time against its deadline, per
// channel (hook-latency.jsonl, written by the hook guard's end, with the
// invocation's CPU time beside it where the platform reports it: EW-29), the
// minutes the shared breaker held hooks off, the edits served while it was
// open (the guard feature degrades silently then, so they are counted),
// and harness kills — an edit-hook invocation whose `seen` beat's id
// (hook_inv) never reached hook-latency.jsonl: it began and never
// finished (the hook guard writes the row on every return path).
//
// disk_retention: `fathom gc`'s dry run — reclaimable bytes by class and
// the largest classes — and the internal disk's free space.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// hookLatencyRow is one hook invocation's wall time, and its CPU time when
// the platform reports it (EW-29).
type hookLatencyRow struct {
	At       time.Time `json:"at"`
	Channel  string    `json:"channel"`
	MS       int64     `json:"ms"`
	BudgetMS int64     `json:"budget_ms,omitempty"` // the deadline; 0 = none
	// Inv is the invocation's id (pid-startnanos), also stamped on every
	// beat the invocation emitted: a `seen` whose id never reaches this
	// log is an invocation that never finished — a harness kill.
	Inv string `json:"inv,omitempty"`
	// CPUMS is the invocation's CPU time (user+sys, this process plus its
	// reaped children) between the guard's begin and end (EW-29), beside the
	// wall time MS: a slow hook with a small CPUMS waited, one with a
	// CPUMS near MS worked. A pointer with omitempty: absent when the CPU
	// could not be read at both ends (law 3) and in rows written before the
	// field existed, never a fabricated 0. It can exceed MS — the Go runtime
	// is multi-threaded.
	CPUMS *int64 `json:"cpu_ms,omitempty"`
}

// hookLatencyRotateBytes bounds hook-latency.jsonl (one generation kept).
const hookLatencyRotateBytes = 4 << 20

// hookLatencyPath sits beside the beat log.
func hookLatencyPath() string {
	p := beatLogPath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "hook-latency.jsonl")
}

// recordHookLatency appends one row. Best-effort, like every beat. cpu is
// the invocation's CPU time (EW-29), recorded only when cpuKnown: an
// unreadable CPU leaves cpu_ms absent, never 0.
func recordHookLatency(channel, inv string, wall, budget time.Duration, at time.Time, cpu time.Duration, cpuKnown bool) {
	path := hookLatencyPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = dlog.RotateIfNeeded(path, hookLatencyRotateBytes)
	row := hookLatencyRow{At: at.UTC(), Channel: channel, MS: wall.Milliseconds(), BudgetMS: budget.Milliseconds(), Inv: inv}
	if cpuKnown && cpu >= 0 {
		ms := cpu.Milliseconds()
		row.CPUMS = &ms
	}
	b, err := json.Marshal(row)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}

// readHookLatency reads both generations, rows at or after since.
func readHookLatency(path string, since time.Time) []hookLatencyRow {
	var out []hookLatencyRow
	for _, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var r hookLatencyRow
			if json.Unmarshal(sc.Bytes(), &r) == nil && !r.At.Before(since) {
				out = append(out, r)
			}
		}
		f.Close()
	}
	return out
}

// inHookSkip: doctor --brief (the Setup hook) prints FAIL rows only, and
// these rows never FAIL — their log and disk scans are not spent inside a
// hook's deadline.
const inHookSkip = "not run inside a hook (doctor --brief shows FAIL rows only; this row never FAILs) — run `fathom doctor`"

// hookBudgetWindow is what hook_budget judges.
const hookBudgetWindow = 24 * time.Hour

// checkHookBudget is the hook_budget row over the window's latency rows
// and beats.
func checkHookBudget(lat []hookLatencyRow, beats []heartbeat.Beat, now time.Time) doctorCheck {
	return checkHookBudgetCovered(lat, beats, time.Time{}, now)
}

// checkHookBudgetCovered is checkHookBudget over beats that reach back
// only to coveredFrom (zero: the whole window): the beat figures then name
// the span they cover, never "/24h" (I38).
func checkHookBudgetCovered(lat []hookLatencyRow, beats []heartbeat.Beat, coveredFrom, now time.Time) doctorCheck {
	const name = "hook_budget"
	since := now.Add(-hookBudgetWindow)
	var inWindow []hookLatencyRow
	for _, r := range lat {
		if !r.At.Before(since) {
			inWindow = append(inWindow, r)
		}
	}
	over := hookChannelsOverBudget(inWindow)
	// Breaker-open minutes, edits served open (beats), and harness kills:
	// an edit-hook invocation whose `seen` beat carries its id (hook_inv)
	// and whose id never reached the latency log — it began and never
	// finished. A silent exit after `seen` (obligations off, a relative
	// path, a deny) still finishes, so it is never a kill. Beats without
	// an id (an older binary) are not judged.
	finished := map[string]bool{}
	for _, r := range lat {
		if r.Inv != "" {
			finished[r.Inv] = true
		}
	}
	openMinutes := map[int64]bool{}
	editsOpen := 0
	kills := 0
	for _, b := range beats {
		if b.At.Before(since) {
			continue
		}
		switch {
		case b.Kind == "breaker_open":
			openMinutes[b.At.Unix()/60] = true
			if b.Channel == "edit-hook" {
				editsOpen++
			}
		case b.Channel == "edit-hook" && b.Kind == "seen":
			if inv := b.Meta["hook_inv"]; inv != "" && !finished[inv] && now.Sub(b.At) > 30*time.Second {
				kills++ // unfinished 30 s on: killed
			}
		}
	}
	lat24 := "no hook latency recorded in 24 h (hook-latency.jsonl)"
	if len(inWindow) > 0 {
		lat24 = hookLatencySummary(inWindow)
	}
	span := "/24h"
	if !coveredFrom.IsZero() && coveredFrom.After(since) {
		span = fmt.Sprintf(" in the %s the beat log read covers only (of 24h)", now.Sub(coveredFrom).Round(time.Minute))
	}
	reason := fmt.Sprintf("%s · breaker open %d min%s · %d edit(s) served while open (guards skipped) · %d harness kill(s) [hook-latency.jsonl, hooks.jsonl]", lat24, len(openMinutes), span, editsOpen, kills)
	switch {
	case len(over) > 0:
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason, Remedy: "hooks on " + strings.Join(over, ", ") + " run past their deadline at p99 — the harness kills them; `fathom resources` names the daemon's state and the in-flight work"}
	case kills > 0 || editsOpen > 0:
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason, Remedy: "hooks were killed or served edits unguarded while the breaker was open — `fathom resources` names why the daemon was slow or away"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// checkHookBudgetFromFiles reads the logs and judges them.
func checkHookBudgetFromFiles(now time.Time) doctorCheck {
	if activeHookGuard() != nil {
		return doctorCheck{Name: "hook_budget", Status: doctorSkipped, Reason: inHookSkip}
	}
	since := now.Add(-hookBudgetWindow)
	beats, coveredFrom := readBeatsSince(beatLogPath(), since, hookBudgetBeatMaxBytes)
	return checkHookBudgetCovered(readHookLatency(hookLatencyPath(), since), beats, coveredFrom, now)
}

// readBeatsSince reads the beat log's beats back to since, newest
// generation first — the live log, then its rotated generation <path>.1 —
// reading each from its end in doubling tails, never more than maxBytes in
// all. coveredFrom is zero when the beats read reach back to since;
// otherwise it is the oldest beat read (the retained history, or the byte
// bound, ended first), so the caller names the span it covers.
func readBeatsSince(path string, since time.Time, maxBytes int64) (beats []heartbeat.Beat, coveredFrom time.Time) {
	if path == "" {
		return nil, time.Time{}
	}
	left := maxBytes
	reached := false
	for _, g := range []string{path, path + ".1"} {
		if left <= 0 || reached {
			break
		}
		st, err := os.Stat(g)
		if err != nil {
			continue
		}
		var got []heartbeat.Beat
		var read int64
		for tail := min(int64(4<<20), left); ; tail = min(tail*2, left) {
			read = min(tail, st.Size())
			got, _ = readBeatLogTail(g, read)
			if read == st.Size() || tail == left || (len(got) > 0 && !got[0].At.After(since)) {
				break
			}
		}
		left -= read
		beats = append(got, beats...)
		if len(got) > 0 && !got[0].At.After(since) {
			reached = true
		}
	}
	if reached || len(beats) == 0 {
		return beats, time.Time{}
	}
	return beats, beats[0].At
}

// diskRetentionWarnBytes is the reclaimable total that warns.
const diskRetentionWarnBytes = 512 << 20

// checkDiskRetention is the disk_retention row over gc's dry run and the
// internal disk's free bytes (free < 0: unknown).
func checkDiskRetention(rep gcReport, free int64, freeVol string) doctorCheck {
	const name = "disk_retention"
	type cls struct {
		name  string
		bytes int64
	}
	var cs []cls
	var total int64
	for c, n := range rep.Reclaimable {
		cs = append(cs, cls{c, n})
		total += n
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].bytes != cs[j].bytes {
			return cs[i].bytes > cs[j].bytes
		}
		return cs[i].name < cs[j].name
	})
	var top []string
	for i, c := range cs {
		if i == 3 || c.bytes == 0 {
			break
		}
		top = append(top, fmt.Sprintf("%s %s", c.name, tenancy.FormatBytes(c.bytes)))
	}
	largest := "nothing reclaimable"
	if len(top) > 0 {
		largest = "largest: " + strings.Join(top, ", ")
	}
	freeText := "internal disk free unknown"
	if free >= 0 {
		freeText = fmt.Sprintf("%s free on %s", tenancy.FormatBytes(free), freeVol)
	}
	var reports []string
	for _, it := range rep.Items {
		if it.Action == "report" {
			reports = append(reports, it.Class+": "+it.Reason)
		}
	}
	reason := fmt.Sprintf("reclaimable %s (%s) · %s [fathom gc dry run, statfs]", tenancy.FormatBytes(total), largest, freeText)
	if len(reports) > 0 {
		reason += " · " + strings.Join(reports, "; ")
	}
	if len(rep.Errors) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason + " · unreadable: " + strings.Join(rep.Errors, "; "), Remedy: "fix the named paths' permissions, then `fathom gc`"}
	}
	if total > diskRetentionWarnBytes {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: reason, Remedy: "`fathom gc` shows the plan; `fathom gc --apply` reclaims it (evidence is archived, never deleted)"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// checkDiskRetentionFromDisk runs gc's dry run for root's workspace.
func checkDiskRetentionFromDisk(root string) doctorCheck {
	if activeHookGuard() != nil {
		return doctorCheck{Name: "disk_retention", Status: doctorSkipped, Reason: inHookSkip}
	}
	home, _ := os.UserHomeDir()
	ws := ""
	if top := gitToplevel(root); top != "" {
		ws = filepath.Join(top, "workspace")
	}
	rep := runGC(gcOptions{Home: home, TmpDir: os.TempDir(), Workspace: ws, CallLog: resolveConsoleCallLog("", "").Path})
	free, vol := int64(-1), home
	if vol == "" {
		vol = "/"
	}
	var st syscall.Statfs_t
	if syscall.Statfs(vol, &st) == nil {
		free = int64(st.Bavail) * int64(st.Bsize)
	}
	return checkDiskRetention(rep, free, vol)
}

// hookP99s is each channel's p99 wall time and largest deadline, sorted by
// channel.
func hookP99s(rows []hookLatencyRow) (chans []string, p99, budget map[string]int64, n map[string]int) {
	byChan := map[string][]int64{}
	budget, p99, n = map[string]int64{}, map[string]int64{}, map[string]int{}
	for _, r := range rows {
		byChan[r.Channel] = append(byChan[r.Channel], r.MS)
		if r.BudgetMS > budget[r.Channel] {
			budget[r.Channel] = r.BudgetMS
		}
	}
	for c, ms := range byChan {
		chans = append(chans, c)
		sort.Slice(ms, func(i, j int) bool { return ms[i] < ms[j] })
		p99[c] = ms[(len(ms)*99+99)/100-1]
		n[c] = len(ms)
	}
	sort.Strings(chans)
	return chans, p99, budget, n
}

// hookP50s is each channel's median wall and median CPU time (EW-29) over
// the rows that carry cpu_ms — the wall median is taken on those same rows,
// so the two compare like for like — and how many rows that is. A channel
// with no such row is absent from the maps.
func hookP50s(rows []hookLatencyRow) (wall, cpu map[string]int64, timed map[string]int) {
	wallBy, cpuBy := map[string][]int64{}, map[string][]int64{}
	for _, r := range rows {
		if r.CPUMS != nil {
			wallBy[r.Channel] = append(wallBy[r.Channel], r.MS)
			cpuBy[r.Channel] = append(cpuBy[r.Channel], *r.CPUMS)
		}
	}
	wall, cpu, timed = map[string]int64{}, map[string]int64{}, map[string]int{}
	median := func(v []int64) int64 {
		sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
		return v[(len(v)*50+99)/100-1]
	}
	for c, w := range wallBy {
		timed[c] = len(w)
		wall[c], cpu[c] = median(w), median(cpuBy[c])
	}
	return wall, cpu, timed
}

// hookLatencySummary renders "channel p99 1.9s/4.0s (n=…)" per channel, and
// beside n the median wall and CPU time where rows carry cpu_ms (EW-29) —
// "(n=…, p50 wall 940ms cpu 120ms)", or "… of 40 timed" when only some do.
func hookLatencySummary(rows []hookLatencyRow) string {
	chans, p99, budget, n := hookP99s(rows)
	p50Wall, p50CPU, timed := hookP50s(rows)
	parts := make([]string, 0, len(chans))
	for _, c := range chans {
		cpu := ""
		if k := timed[c]; k > 0 {
			cpu = fmt.Sprintf(", p50 wall %dms cpu %dms", p50Wall[c], p50CPU[c])
			if k != n[c] {
				cpu += fmt.Sprintf(" of %d timed", k)
			}
		}
		if budget[c] > 0 {
			parts = append(parts, fmt.Sprintf("%s p99 %.1fs/%.1fs (n=%d%s)", c, float64(p99[c])/1000, float64(budget[c])/1000, n[c], cpu))
		} else {
			parts = append(parts, fmt.Sprintf("%s p99 %.1fs (n=%d, no deadline%s)", c, float64(p99[c])/1000, n[c], cpu))
		}
	}
	return strings.Join(parts, " · ")
}

// hookChannelsOverBudget names the channels whose p99 exceeds their deadline.
func hookChannelsOverBudget(rows []hookLatencyRow) []string {
	chans, p99, budget, _ := hookP99s(rows)
	var over []string
	for _, c := range chans {
		if budget[c] > 0 && p99[c] > budget[c] {
			over = append(over, c)
		}
	}
	return over
}

// hookBudgetBeatMaxBytes bounds how many bytes of beats hook_budget reads
// to cover its window (both generations together).
var hookBudgetBeatMaxBytes int64 = 32 << 20
