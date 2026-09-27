package main

// doctor_footprint.go — `daemon_footprint` (RT-49, design §13.3). It
// judges the daemon's phys_footprint (plus its child processes, the
// ladder's measure) against the budget the daemon runs under, from the
// resources projection (resources.go): the live /statusz tenancy block
// when the daemon answers, else the resource ring's last sample with its
// age. It never SKIPs an unreachable daemon into silence (I38): a daemon
// that does not answer is a WARN that names what the files last saw.
// WARN, never FAIL, per doctor's honesty discipline (doctor.go): memory
// is an operator signal, not a wiring-broken verdict. The old remedy —
// "restart the daemon" — is gone: the governor bounds the daemon, and the
// remedy names the level, the class and the top tool, and `fathom
// resources`. (O108's heap-in-use reading was the wrong instrument: Go's
// heap is not what the machine pays.)

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// checkDaemonFootprint is doctor's daemon_footprint row over the
// resources report; metrics is the daemon's /metrics (nil when it did not
// answer), read only for the GC hint.
func checkDaemonFootprint(rep resourcesReport, metrics map[string]float64) doctorCheck {
	const name = "daemon_footprint"
	const see = "fathom resources"
	t := rep.Tenancy
	if t == nil {
		why := firstNonEmpty(rep.Daemon.Unreachable, "no tenancy block")
		switch {
		case len(rep.Unreadable) > 0:
			return doctorCheck{Name: name, Status: doctorWarn,
				Reason: fmt.Sprintf("the daemon at %s did not answer (%s) and its files are unreadable: %s", rep.Daemon.URL, why, strings.Join(rep.Unreadable, "; ")),
				Remedy: see}
		case rep.Daemon.Reachable:
			return doctorCheck{Name: name, Status: doctorWarn,
				Reason: fmt.Sprintf("the daemon at %s answers but reports no tenancy block (%s) — its binary predates the governor, so its footprint is unbounded and unmeasured", rep.Daemon.URL, why),
				Remedy: "update the daemon's binary the way it was installed (in a fathom checkout: task deploy); then " + see}
		default:
			return doctorCheck{Name: name, Status: doctorWarn,
				Reason: fmt.Sprintf("the daemon at %s did not answer (%s) and there is no resource ring at %s — no footprint instrument", rep.Daemon.URL, why, rep.Footprint.Ring),
				Remedy: see + " (and daemon_reachable above)"}
		}
	}
	fb := tenancy.FormatBytes
	var b strings.Builder
	if t.Footprint < 0 {
		b.WriteString("phys_footprint unknown")
	} else {
		fmt.Fprintf(&b, "phys_footprint %s", fb(t.Footprint))
		if t.Children >= 0 {
			fmt.Fprintf(&b, " (+%s children)", fb(t.Children))
		}
	}
	if rep.Budget.Budget > 0 {
		fmt.Fprintf(&b, " vs budget %s (shed %s, refuse %s)", fb(rep.Budget.Budget), fb(rep.Budget.Shed), fb(rep.Budget.Refuse))
	}
	fmt.Fprintf(&b, " · level %s · mode %s", t.Level, firstNonEmpty(t.Mode, "unknown"))
	if rep.Footprint.Max > 0 {
		fmt.Fprintf(&b, " · %s max %s", rep.Footprint.Window, fb(rep.Footprint.Max))
	}
	if limit, ok := metrics["go_memory_limit_bytes"]; ok {
		fmt.Fprintf(&b, " · GC hint %s", memoryLimitDisplay(limit))
	}
	remedy := see
	if t.Oldest != nil {
		remedy = fmt.Sprintf("level %s; top in-flight %s (%s) — %s", t.Level, t.Oldest.Tool, t.Oldest.Class, see)
	} else {
		remedy = fmt.Sprintf("level %s — %s", t.Level, see)
	}
	if t.Source != "live" {
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("the daemon at %s did not answer (%s); last ring sample %s old: %s", rep.Daemon.URL, firstNonEmpty(rep.Daemon.Unreachable, "no tenancy block"), firstNonEmpty(rep.Footprint.SampleAge, "?"), b.String()),
			Remedy: remedy}
	}
	if t.Footprint < 0 || t.LadderOff {
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: b.String() + " — footprint unknown: ladder off (admission still applies)", Remedy: remedy}
	}
	combined := t.Footprint
	if t.Children > 0 {
		combined += t.Children
	}
	if t.Level != tenancy.LevelNormal.String() || (rep.Budget.Shed > 0 && combined >= rep.Budget.Shed) {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: b.String() + " — at or above the shed mark", Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: b.String()}
}

// memoryLimitDisplay renders a go_memory_limit_bytes reading (already
// parsed as float64 by fetchMetricsText) for doctor's reason line — "off"
// for the runtime's own math.MaxInt64 sentinel (unlimitedMemoryLimit,
// memorylimit.go), else two-decimal GiB.
func memoryLimitDisplay(bytes float64) string {
	if unlimitedMemoryLimit(bytes) {
		return "off"
	}
	return fmt.Sprintf("%.2f GiB", bytes/(1<<30))
}

// fetchMetricsText GETs baseURL/metrics and parses its "key value\n" text
// exposition format into a float64 map. Unparseable or duplicate lines are
// skipped rather than erroring the whole read — /metrics is intentionally
// append-only across fathom's own history (new gauges land beside old
// ones), so a reader must tolerate lines it doesn't recognize.
func fetchMetricsText(hc *http.Client, baseURL string) (map[string]float64, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s/metrics: HTTP %d", baseURL, resp.StatusCode)
	}
	out := make(map[string]float64)
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		out[fields[0]] = v
	}
	return out, sc.Err()
}
