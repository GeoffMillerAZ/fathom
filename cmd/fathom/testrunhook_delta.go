package main

// testrunhook_delta.go — RO-23 (docs/plans/readouts.md phase 2): under
// [readouts] mode = rich a failing run's line carries the test delta
// (internal/readout RunDelta) against the session's previous test-run rows,
// read from the hook log's last 256 KiB (the tail RO-39 and RO-40 read).
// Local reads only: no daemon call, no git, no table. A miss is silent and
// logged on the run's row (`delta`), never a wrong line.

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

// testRunDeltaClause returns the delta clause for this failing run and
// records its outcome on rec. complete is false when the output was piped
// or truncated, so a package it does not show may still have run.
func testRunDeltaClause(rec *testRunRecord, run testrun.Run, command string, complete bool) string {
	prev, ok := sessionTestRunRows(beatLogPath(), rec.Session, rec.AgentID)
	if !ok {
		rec.Delta = "unread"
		return ""
	}
	clause, outcome := readout.RunDelta(prev, testRunNowOf(rec, run, command, complete))
	rec.Delta = outcome
	return clause
}

// sessionTestRunRows reads the session's test-run rows that agentID ran
// ("" the main agent) from the hook log at path, in log order. A log that
// does not exist yet holds no rows; ok is false when it cannot be read.
func sessionTestRunRows(path, session, agentID string) ([]readout.TestRunRow, bool) {
	if session == "" {
		return nil, false
	}
	beats, ok := readBeatLogTail(path, sessionRunsTailBytes)
	if !ok {
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return nil, true
		}
		return nil, false
	}
	sort.SliceStable(beats, func(i, j int) bool { return beats[i].At.Before(beats[j].At) })
	var rows []readout.TestRunRow
	for _, b := range beats {
		if b.Channel != testRunChannel || b.Meta["session"] != session || b.Meta["agent_id"] != agentID {
			continue
		}
		var failing []string
		if f := b.Meta["failing"]; f != "" {
			failing = strings.Split(f, ",")
		}
		rows = append(rows, readout.TestRunRow{
			Invocation: b.Meta["invocation"], Result: b.Meta["result"], Failing: failing,
			Capped:  len(failing) >= testRunMaxLoggedIDs,
			Partial: b.Meta["piped"] == "true" || b.Meta["truncated"] == "true",
			Head:    b.Meta["head"], Tree: b.Meta["tree"], Scope: b.Meta["scope"],
		})
	}
	return rows, true
}

// testRunNowOf is this run as the delta reads it: its leaf failures, the
// packages it shows a result for, and whether its scope's filters are all
// the command carries.
func testRunNowOf(rec *testRunRecord, run testrun.Run, command string, complete bool) readout.TestRunNow {
	now := readout.TestRunNow{
		Invocation: rec.Invocation, Complete: complete && run.Complete,
		Head: rec.Head, Tree: rec.Tree, Scope: rec.Scope,
	}
	for _, f := range run.LeafFailures() {
		now.Failing = append(now.Failing, f.ID())
	}
	panicked := map[string]bool{}
	for _, p := range run.Panics {
		panicked[p.Package] = true
	}
	built := map[string]bool{}
	for _, b := range run.BuildErrors {
		built[b.Package] = true
	}
	for _, p := range run.Packages {
		status := string(p.Status)
		if built[p.Package] {
			status = string(testrun.StatusBuildFailed)
		}
		now.Packages = append(now.Packages, readout.TestRunPackage{Name: p.Package, Status: status, Panicked: panicked[p.Package]})
	}
	_, unreadable := lexShellSegments(command)
	now.FiltersKnown = !unreadable
	for _, w := range strings.Fields(command) {
		name, val, _ := strings.Cut(strings.TrimLeft(w, "-"), "=")
		switch {
		case w == "--" || w == "-args" || w == "--args":
			now.FiltersKnown = false // the rest reaches the test binary or the task unread
		case strings.HasPrefix(w, "-") && name == "failfast":
			if b, err := strconv.ParseBool(val); val == "" || (err == nil && b) {
				now.FailFast = true
			}
		}
	}
	return now
}
