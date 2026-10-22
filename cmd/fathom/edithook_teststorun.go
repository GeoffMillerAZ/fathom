package main

// edithook_teststorun.go — RO-43 (docs/plans/readouts.md; design
// docs/design/readouts.md section 20, H4): after an Edit, Write or
// MultiEdit of a Go file, the PostToolUse edit hook adds one line naming at
// most 5 tests that reach the edited file (the shared reach, reach.go, the
// one the pre-edit blast trailer counts), `and N more` past the cap, the
// runner's own filter as text fathom never runs, and whether the session
// has a recorded test run of them at this tree (RO-35's row keys). Silent
// when no test reaches; a daemon that does not answer in time reads
// `tests: not computed (deadline)`. Inside I35's 320-token ceiling: the
// line is advisory and drops first. Off by default behind
// `[hooks] tests_to_run`; any error is silence or the named miss, never a
// failed hook.

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/readout"
)

// hookOptInTestsToRun is the line's [hooks] key.
const hookOptInTestsToRun = "hooks.tests_to_run"

// testsToRunChannel is the hook-log channel of the line's one row per
// computed call: counts, the named test keys, the tree verdict and ms;
// never file text.
const testsToRunChannel = "tests-to-run"

// testsToRunWait is the line's own deadline. RO-M13 read the reach through
// a warm daemon at a p95 of 37 ms; a cold daemon (8 to 10 s building its
// call graph) or an absent one is the named miss.
var testsToRunWait = 300 * time.Millisecond

// testsToRunReserve is what the edit hook keeps under its deadline after the
// line: the encode, the beats and the write.
const testsToRunReserve = 200 * time.Millisecond

// testsToRunMissLine is the line for a reach that did not answer.
func testsToRunMissLine(reason string) string {
	return "tests: not computed (" + reason + ")"
}

// testsToRunLine is H4's line for an edit of filePath in session sessionID,
// through the daemon at baseURL, with the row it logs. Both are empty when
// the line is off at the file's repository or the file is no Go source; the
// row alone (kind silent) when no test reaches.
func testsToRunLine(baseURL, filePath, sessionID string) (string, map[string]string) {
	if !strings.HasSuffix(filePath, ".go") {
		return "", nil
	}
	root := gitToplevelByStat(filepath.Dir(filePath))
	if root == "" {
		return "", nil
	}
	cfg, ok := hooksConfigAt(root, hookOptInTestsToRun)
	if !ok || !cfg.Hooks.TestsToRun {
		return "", nil
	}
	rel, ok := relativeToRoot(root, filePath)
	if !ok {
		return "", nil
	}
	rel = filepath.ToSlash(rel)
	start := time.Now()
	wait := testsToRunWait
	if h := hookCallTimeout(testsToRunReserve, testsToRunWait); h < wait {
		wait = h
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	row := map[string]string{}
	if cfg.Hooks.Observes(hookOptInTestsToRun) {
		row["mode"] = "observe"
	}
	line := ""
	svc, err := reachService(ctx, baseURL, root)
	var r blastReach
	if err == nil {
		r, err = fetchBlastReach(ctx, baseURL, svc, []string{rel})
	}
	if err != nil {
		reason := daemonMissReason(ctx, err)
		line, row["kind"], row["missed"] = testsToRunMissLine(reason), "missed", reason
		if reason == daemonJoinMissNotIndexed && svc == "" {
			line, row["missed"] = "", "not-indexed"
		}
	} else {
		shown, more := pickTestsToRun(r.Tests, path.Dir(rel))
		row["reached"] = strconv.Itoa(len(shown) + more)
		if len(shown) == 0 {
			row["kind"] = "silent"
		} else {
			ran := testsRanAtTree(ctx, root, sessionID, cfg.Hooks.TestRun, shown)
			line, row["kind"] = testsToRunText(shown, more, ran), "line"
			row["named"] = strconv.Itoa(len(shown))
			row["ran"] = strings.TrimPrefix(ran, "at this tree: ")
			keys := make([]string, len(shown))
			for i, t := range shown {
				keys[i] = t.dir + "." + t.name
			}
			row["tests"] = strings.Join(keys, ",")
		}
	}
	row["ms"] = strconv.FormatInt(time.Since(start).Milliseconds(), 10)
	return line, row
}

// daemonMissReason names why a daemon call did not answer: the deadline,
// a root or file the index does not hold, or the daemon itself.
func daemonMissReason(ctx context.Context, err error) string {
	switch {
	case deadlinePassed(ctx) || errors.Is(err, context.DeadlineExceeded):
		return daemonJoinMissDeadline
	case errors.Is(err, errNoService) || strings.Contains(err.Error(), "not indexed"):
		return daemonJoinMissNotIndexed
	}
	return daemonJoinMissDaemon
}

// testsRanAtTree is the line's tree clause for the shown tests: whether a
// test-run row of the session at this HEAD and tree ran each in its scope
// (RO-35's keys; the latest such row decides a test), from the hook log's
// tail. A missing run is said only when the tail reaches the session's last
// edit (I38); else the clause says what it could not check.
func testsRanAtTree(ctx context.Context, root, sessionID string, testRunOn bool, shown []testRef) string {
	const lead = "at this tree: "
	if !testRunOn {
		return lead + "not checked (test_run off)"
	}
	runs := readSessionRuns(beatLogPath(), sessionID)
	if !runs.Read {
		return lead + "not checked (log)"
	}
	ran, unsure, failed := 0, false, false
	keyed := slices.ContainsFunc(runs.Rows, func(r sessionRunRow) bool { return r.Head != "" && r.Tree != "" })
	var cur testRunRecord
	if keyed {
		// Only a row that carries a tree can match one: with none, the
		// session's git state is never read.
		fillTestRunRowKeys(ctx, &cur, root, sessionID, "", "")
		if cur.Head == "" || cur.Tree == "" {
			return lead + "not checked (deadline)"
		}
	}
	for _, t := range shown {
		var last *sessionRunRow
		for i := range runs.Rows {
			r := &runs.Rows[i]
			if r.Head == "" || r.Tree == "" || r.Head != cur.Head || r.Tree != cur.Tree {
				continue
			}
			admits, ok := readout.ScopeAdmits(r.Scope, t.dir, t.name)
			if !ok {
				unsure = true
				continue
			}
			if admits {
				last = r
			}
		}
		if last != nil {
			ran++
			failed = failed || last.Result != "pass"
		}
	}
	switch {
	case ran == len(shown) && !failed:
		return lead + "ran, passed"
	case ran == len(shown):
		return lead + "ran, not all passed"
	case ran > 0:
		return lead + fmt.Sprintf("%d of %d ran", ran, len(shown))
	case unsure:
		return lead + "not checked (scope)"
	}
	var start time.Time
	if runs.LastEdit.IsZero() {
		start, _ = sessionStartFor(root, sessionID)
	}
	if !runs.covers(start) {
		return lead + "not checked (window)"
	}
	return lead + "not run"
}

// withTestsToRun adds the tests-to-run line to the post-edit payload text
// when it is on and fits I35's ceiling, and logs the line's row.
func withTestsToRun(baseURL, filePath, sessionID, text string, ok bool) (string, bool) {
	line, row := testsToRunLine(baseURL, filePath, sessionID)
	if row == nil {
		return text, ok
	}
	kind := row["kind"]
	delete(row, "kind")
	emitted := false
	if line != "" && row["mode"] != "observe" {
		joined := line
		if ok && text != "" {
			joined = text + "\n" + line
		}
		if cost.Estimate(joined) <= specLineTokenBudget {
			text, ok, emitted = joined, true, true
		} else {
			row["dropped"] = "budget"
		}
	}
	row["emitted"] = boolStr(emitted)
	emitBeat(testsToRunChannel, kind, firingMeta(row, sessionID, filePath))
	return text, ok
}
