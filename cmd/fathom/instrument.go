package main

import (
	"fmt"
	"io"
	"os"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// standingExpectations are the Gap-1 liveness pairs (docs/vision/04 Gap 1):
// sessions without edits within a week, or edits the channel never answered
// within a day, are the four-dark-days failure made visible.
var standingExpectations = []heartbeat.Expectation{
	{IfChannel: "session-hook.fired", ThenChannel: "edit-hook.seen", Within: 7 * 24 * time.Hour},
	{IfChannel: "edit-hook.seen", ThenChannel: "edit-hook.answered", Within: 24 * time.Hour},
}

// cmdInstrument implements "instrument": the channel-health report over the
// hooks beat log — injection coverage (answered/seen), per-day rollup, and
// liveness alarms. Advisory data only: exit 0 always (a broken instrument
// must never gate anything — it reports, humans decide).
func cmdInstrument(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("instrument", stderr)
	logPath := fs.String("log", beatLogPath(), "hooks beat log (JSONL)")
	window := fs.Duration("window", 7*24*time.Hour, "coverage window, measured back from now")
	if err := fs.Parse(args); err != nil {
		return 0
	}
	f, err := os.Open(*logPath)
	if err != nil {
		fmt.Fprintf(stdout, "no beat log at %s — channels uninstrumented or never fired\n", *logPath)
		return 0
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true)
	if err != nil || len(beats) == 0 {
		fmt.Fprintf(stdout, "beat log unreadable or empty (%v)\n", err)
		return 0
	}
	ref := time.Now().UTC()
	reportInstrument(stdout, beats, ref, *window)
	return 0
}

// reportInstrument renders the coverage/rollup/liveness report — pure over
// its inputs so tests inject beats and a fixed reference time.
func reportInstrument(w io.Writer, beats []heartbeat.Beat, ref time.Time, window time.Duration) {
	since := ref.Add(-window)
	seen, answered := 0, 0
	for _, b := range beats {
		if b.At.Before(since) || b.At.After(ref) || b.Channel != "edit-hook" {
			continue
		}
		switch b.Kind {
		case "seen":
			seen++
		case "answered":
			answered++
		}
	}
	fmt.Fprintf(w, "injection coverage (%s window): ", window)
	if seen == 0 {
		fmt.Fprintln(w, "no edits seen — denominator empty")
	} else {
		fmt.Fprintf(w, "%d/%d = %.0f%% (gate: ≥90%%)\n", answered, seen, 100*float64(answered)/float64(seen))
	}

	fmt.Fprintln(w, "\nper-day rollup:")
	for _, dc := range heartbeat.Rollup(beats) {
		fmt.Fprintf(w, "  %s  %-14s %-9s %d\n", dc.Day, dc.Channel, dc.Kind, dc.N)
	}

	alarms := heartbeat.CheckLiveness(beats, standingExpectations, ref)
	if len(alarms) == 0 {
		fmt.Fprintln(w, "\nliveness: all standing expectations met")
		return
	}
	fmt.Fprintln(w, "\n⚠ liveness alarms:")
	for _, a := range alarms {
		fmt.Fprintf(w, "  %s\n", a.Detail)
	}
}
