package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

type doctorChannel struct {
	Channel              string   `json:"channel"`
	State                string   `json:"state"`
	Reason               string   `json:"reason,omitempty"`
	Injections           *int     `json:"injections"`
	TokensPerDayEstimate *float64 `json:"tokens_per_day_estimate"`
	LastSkipReason       string   `json:"last_skip_reason,omitempty"`
	Coverage             string   `json:"coverage"`
	CostGroup            string   `json:"cost_group,omitempty"`
}

// Existing payload-cost groups cannot distinguish pre-edit and pre-shell
// guards or startup from a cwd primer. Name shared costs; never sum them.
func doctorChannelCostGroup(cap harness.Capability) string {
	switch cap {
	case harness.CapPrime, harness.CapCwdPrime:
		return "session-brief"
	case harness.CapObligationsPostEdit:
		return "post-edit"
	case harness.CapGuardPreEdit, harness.CapGuardPreShell:
		return "pre-edit-guard"
	case harness.CapObligationsPostShell:
		return "post-bash"
	}
	return string(cap)
}
func doctorChannelLogName(cap harness.Capability) string {
	switch cap {
	case harness.CapPrime, harness.CapSubagentPrime, harness.CapToolFailureAdvice, harness.CapCwdPrime:
		return "session-hook"
	case harness.CapObligationsPostEdit, harness.CapGuardPreEdit, harness.CapGuardPreShell, harness.CapObligationsPostShell:
		return "edit-hook"
	case harness.CapStopVerify:
		return "stop-hook"
	case harness.CapPromptObligations:
		return "prompt-hook"
	case harness.CapTestRunLine:
		return "test-run-hook"
	}
	return string(cap)
}

// Pure projection with an injected clock. No readiness registry/stamp producer
// exists in this checkout: derive observations from the bounded hook-log tail.
func doctorChannelRows(d harness.Descriptor, cfg config.Config, wired map[harness.Capability]bool, beats []heartbeat.Beat, now time.Time, logOK bool, coverage string) []doctorChannel {
	bindings := map[harness.Capability][]harness.EventBinding{}
	for cap, v := range d.Vehicles {
		if v == harness.VehicleHookInject {
			bindings[cap] = nil
		}
	}
	for _, b := range d.Hooks {
		if b.Vehicle == harness.VehicleHookInject {
			bindings[b.Capability] = append(bindings[b.Capability], b)
		}
	}
	keys := make([]harness.Capability, 0, len(bindings))
	for k := range bindings {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	rows := make([]doctorChannel, 0, len(keys))
	since := now.Add(-7 * 24 * time.Hour)
	for _, cap := range keys {
		r := doctorChannel{Channel: string(cap), State: "on but unused", Coverage: coverage, CostGroup: doctorChannelCostGroup(cap)}
		enabled := len(bindings[cap]) == 0
		for _, b := range bindings[cap] {
			if b.OptIn == "" {
				enabled = true
			} else if b.OptIn == harness.OptInReadouts {
				enabled = enabled || hookSeamOn(cfg, "readouts")
			} else {
				key := strings.TrimPrefix(b.OptIn, "hooks.")
				enabled = enabled || hookSeamOn(cfg, key) || key == "test_run" && cfg.Hooks.TestRun
			}
		}

		injections, tokens := 0, 0
		tokensKnown := true
		var lastPayload, lastSkip time.Time
		notReady := ""
		for _, bt := range beats {
			if bt.At.Before(since) || bt.At.After(now) {
				continue
			}
			if h := bt.Meta["harness"]; h != "" && h != d.ID {
				continue
			}
			tagged := bt.Meta["capability"] == string(cap) || bt.Meta["channel"] == string(cap)
			if bt.Channel == "payload-cost" && (bt.Kind == r.CostGroup || tagged) {
				injections++
				n, err := strconv.Atoi(bt.Meta["tokens_estimate"])
				if err == nil && n >= 0 {
					tokens += n
				} else {
					tokensKnown = false
				}
				if bt.At.After(lastPayload) {
					lastPayload = bt.At
				}
			}
			// Current producers use answered/skip reasons; ADR-080 producers use
			// skipped:not-ready:<reason>. Both are observations, never fresh probes.
			if !tagged && bt.Channel != string(cap) && bt.Channel != doctorChannelLogName(cap) {
				continue
			}
			reason := bt.Meta["reason"]
			if strings.HasPrefix(bt.Kind, "skipped:") {
				reason = bt.Kind
			}
			if reason == "" {
				continue
			}
			if bt.At.Before(lastSkip) {
				continue
			}
			lastSkip = bt.At
			r.LastSkipReason = reason
			notReady = ""
			if strings.Contains(reason, "not-ready:") {
				notReady = strings.SplitN(reason, "not-ready:", 2)[1]
			}
		}
		// A missing log and channels without a payload-cost producer are absent
		// metrics, not measured zero. Shared groups carry a named denominator.
		knownGroup := r.CostGroup == "session-brief" || r.CostGroup == "post-edit" || r.CostGroup == "pre-edit-guard" || r.CostGroup == "post-bash"
		if logOK && (knownGroup || injections > 0) {
			r.Injections = &injections
			rate := float64(tokens) / 7
			if tokensKnown {
				r.TokensPerDayEstimate = &rate
			}
		}
		switch {
		case !enabled:
			r.State = "off by choice"
			r.Reason = "disabled by configuration or FATHOM_CHANNELS"
		case !wired[cap]:
			r.State = "on but not ready"
			r.Reason = "no matching hook registration for this harness"
		case notReady != "" && !lastSkip.Before(lastPayload):
			r.State = "on but not ready"
			r.Reason = notReady
		case injections > 0:
			r.State = "on and ready"
			r.Reason = "registered; payload-cost rows observed in the last 7 days"
		default:
			r.Reason = "registered; no payload-cost rows observed (use is unmeasured for channels without cost rows)"
		}
		rows = append(rows, r)
	}
	return rows
}

func doctorChannels(root, label string, now time.Time) []doctorChannel {
	d, ok := harness.Lookup(doctorHarnessID(label))
	if !ok {
		return nil
	}
	cfg, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	opts := []string{}
	for _, b := range d.Hooks {
		if b.OptIn != "" {
			opts = append(opts, b.OptIn)
		}
	}
	specs := hookSpecsFor(d.ID, opts...)
	ctx, cancel := doctorProbeContext(context.Background())
	defer cancel()
	present, notes, _, _ := harnessHookEntriesByScope(root, d, specs, ctx)
	wired := map[harness.Capability]bool{}
	for _, b := range d.Hooks {
		for _, s := range specs {
			if s.event == b.Harness && s.matcher == b.Matcher && s.marker == b.Marker && len(present[hookSpecKeyOf(s)]) > 0 {
				wired[b.Capability] = true
			}
		}
	}
	path := beatLogPath()
	logOK := false
	if st, e := os.Stat(path); e == nil && st.Mode().IsRegular() {
		if f, e := os.Open(path); e == nil {
			logOK = true
			f.Close()
		}
	}
	var beats []heartbeat.Beat
	var coveredFrom time.Time
	if logOK {
		beats, coveredFrom = readBeatsSince(path, now.Add(-7*24*time.Hour), 1<<20)
	}

	coverage := "last 7 days; bounded 1 MiB tail; hook-log scope is all repositories (untagged rows may span harnesses); tokens are estimates, uptake unmeasured"
	if !coveredFrom.IsZero() {
		coverage += "; retained coverage starts " + coveredFrom.UTC().Format(time.RFC3339) + " (counts/rates are lower bounds)"
	}
	if !logOK {
		coverage = "hook log unavailable; injections and cost unknown"
	}
	rows := doctorChannelRows(d, cfg.Config, wired, beats, now, logOK, coverage)
	for i := range rows {
		if os.Getenv("FATHOM_CHANNELS") == "off" {
			rows[i].State = "off by choice"
			rows[i].Reason = "FATHOM_CHANNELS=off in doctor environment"
			continue
		}
		if err != nil {
			rows[i].State = "on but not ready"
			rows[i].Reason = "config unreadable: " + err.Error()
		}
		if len(notes) > 0 && rows[i].State == "on but not ready" {
			rows[i].Reason += "; " + strings.Join(notes, "; ")
		}
	}
	return rows
}

func printDoctorChannels(w io.Writer, rows []doctorChannel) {
	fmt.Fprintln(w, "\ninjecting channels — last 7 days (bounded hook-log tail; all repositories; untagged rows may span harnesses; shared cost groups must not be summed)")
	fmt.Fprintf(w, "%-25s %-18s %-10s %-16s %-20s %s\n", "CHANNEL", "STATE", "INJECTIONS", "~TOKENS/DAY", "COST GROUP", "LAST SKIP / REASON")
	for _, r := range rows {
		count, rate := "unknown", "unknown"
		if r.Injections != nil {
			count = strconv.Itoa(*r.Injections)
		}
		if r.TokensPerDayEstimate != nil {
			rate = fmt.Sprintf("%.1f", *r.TokensPerDayEstimate)
		}
		reason := r.Reason
		if r.LastSkipReason != "" {
			reason = r.LastSkipReason + "; " + reason
		}
		fmt.Fprintf(w, "%-25s %-18s %-10s %-16s %-20s %s\n", r.Channel, r.State, count, rate, r.CostGroup, reason)
	}
	if len(rows) > 0 {
		fmt.Fprintln(w, rows[0].Coverage)
	}
}
