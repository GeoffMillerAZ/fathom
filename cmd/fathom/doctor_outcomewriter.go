package main

// doctor_outcomewriter.go — GL-24 (docs/plans/governance-learning.md): the
// `rule_outcome_writer` row. INC-27: the rule-outcome ledger wrote nothing
// for nine days while the edit hook kept serving rules, and nothing said so
// (JF-1 and GL-10 read [x] on unit tests whose daemon served a claim first).
// A writer is proven by its output: this row names the ledger when the hook
// served claim-bearing answers in the last 72 hours and the ledger holds no
// row from them.
//
// A read, not a job: the hooks beat log (bounded, newest first,
// readBeatsSince) and the ledger's tail, judged against an injected clock.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// ruleOutcomeWriterWindow is how long a writer may be silent while the hook
// serves claims before doctor names it.
const ruleOutcomeWriterWindow = 72 * time.Hour

// ruleOutcomeLedgerTailBytes bounds the ledger read: its newest rows only.
var ruleOutcomeLedgerTailBytes int64 = 1 << 20

// ruleOutcomeLedgerReading is what the ledger's tail says about the window.
type ruleOutcomeLedgerReading struct {
	path     string
	exists   bool
	err      error
	newest   time.Time // the newest row's ts; zero when there is none
	inWindow int       // rows at or after the window's start
}

// readRuleOutcomeLedgerTail reads the newest ruleOutcomeLedgerTailBytes of
// the ledger and counts the rows since since. A missing ledger is no rows,
// not an error; one that exists and cannot be read is the error (I38).
func readRuleOutcomeLedgerTail(path string, since time.Time) ruleOutcomeLedgerReading {
	r := ruleOutcomeLedgerReading{path: path}
	if path == "" {
		return r
	}
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			r.err = err
		}
		return r
	}
	defer f.Close()
	r.exists = true
	st, err := f.Stat()
	if err != nil {
		r.err = err
		return r
	}
	dropFirst := false
	if st.Size() > ruleOutcomeLedgerTailBytes {
		if _, err := f.Seek(-ruleOutcomeLedgerTailBytes, io.SeekEnd); err != nil {
			r.err = err
			return r
		}
		dropFirst = true // the first line is cut mid-row
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		if dropFirst {
			dropFirst = false
			continue
		}
		var row struct {
			TS string `json:"ts"`
		}
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.TS)
		if err != nil {
			continue
		}
		if at.After(r.newest) {
			r.newest = at
		}
		if !at.Before(since) {
			r.inWindow++
		}
	}
	if err := sc.Err(); err != nil {
		r.err = err
	}
	return r
}

// checkRuleOutcomeWriterFromFiles reads the ledger and, when it holds no row
// in the window, the beat log, and judges them. Inside a hook it is skipped:
// the beat read is not spent inside a hook's deadline.
func checkRuleOutcomeWriterFromFiles(now time.Time) doctorCheck {
	if activeHookGuard() != nil {
		return doctorCheck{Name: "rule_outcome_writer", Status: doctorSkipped, Reason: "not run inside a hook (it reads up to 32 MiB of the beat log) — run `fathom doctor`"}
	}
	since := now.Add(-ruleOutcomeWriterWindow)
	ledger := readRuleOutcomeLedgerTail(ruleOutcomeLogPath(), since)
	if ledger.err == nil && ledger.inWindow > 0 {
		return checkRuleOutcomeWriter(ledger, nil, time.Time{}, now) // live: no beat read needed
	}
	beats, coveredFrom := readBeatsSince(beatLogPath(), since, hookBudgetBeatMaxBytes)
	return checkRuleOutcomeWriter(ledger, beats, coveredFrom, now)
}

// checkRuleOutcomeWriter judges the ledger against the edit hook's served
// answers in the window: post and post-bash answers that emitted rules
// (never a doubt line, which serves no rule body). An answer's "claims"
// count says how many served rules carried a claim id, so how many rows it
// should have written; an answer without the count is from a build before
// GL-24 and cannot say.
func checkRuleOutcomeWriter(ledger ruleOutcomeLedgerReading, beats []heartbeat.Beat, coveredFrom, now time.Time) doctorCheck {
	const name = "rule_outcome_writer"
	since := now.Add(-ruleOutcomeWriterWindow)
	if ledger.err != nil {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("the rule-outcome log %s exists but could not be read (%v) — rows recorded there are invisible to this check, which is not the same as none", ledger.path, ledger.err),
			Remedy: "make " + ledger.path + " readable, then re-run doctor",
		}
	}
	if ledger.inWindow > 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s: %d row(s) in the last 72 h, newest %s", ledger.path, ledger.inWindow, ledger.newest.UTC().Format(time.RFC3339))}
	}
	claimAnswers, claimsServed, uncounted := 0, 0, 0
	var newestServed time.Time
	for _, b := range beats {
		if b.Channel != "edit-hook" || b.Kind != "answered" || b.At.Before(since) || b.Meta["emitted"] != "true" || b.Meta["doubt"] == "true" {
			continue
		}
		if p := b.Meta["phase"]; p != "post" && p != "post-bash" {
			continue
		}
		c, ok := b.Meta["claims"]
		if !ok {
			uncounted++
			continue
		}
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			claimAnswers++
			claimsServed += n
			if b.At.After(newestServed) {
				newestServed = b.At
			}
		}
	}
	last := "it holds no row"
	if !ledger.exists {
		last = "it does not exist: no row was ever written"
	} else if !ledger.newest.IsZero() {
		last = "its newest row is " + ledger.newest.UTC().Format(time.RFC3339)
	}
	span := ""
	if !coveredFrom.IsZero() && coveredFrom.After(since) {
		span = fmt.Sprintf(" (the beat log read covers only the last %s)", now.Sub(coveredFrom).Round(time.Minute))
	}
	switch {
	case claimAnswers > 0:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("the rule-outcome log %s wrote no row in 72 h while the edit hook served %d answer(s) carrying %d claim(s), the newest at %s%s; %s",
				ledger.path, claimAnswers, claimsServed, newestServed.UTC().Format(time.RFC3339), span, last),
			Remedy: "edit a governed file in an indexed repository and re-read " + ledger.path + "; no new row means the writer is broken: `fathom observe` it",
		}
	case uncounted > 0:
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("the rule-outcome log %s wrote no row in 72 h; the edit hook served %d answer(s) from a build that does not count their claims, so whether any should have written a row is unknown%s; %s", ledger.path, uncounted, span, last),
			Remedy: "update the hook binary (`fathom --version`), then re-run doctor after an edit",
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("no answer in the last 72 h served a claim, so the rule-outcome log %s had nothing to write%s; %s", ledger.path, span, last)}
}
